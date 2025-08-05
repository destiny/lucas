package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"lucas/internal/hub"
	"lucas/internal/logger"
)

var (
	hubConfigPath string
	hubDebugFlag  bool
	hubTestFlag   bool
)

var hubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Start the Lucas Hub daemon",
	Long: `Lucas Hub is a daemon service that connects to a gateway via ZMQ with CurveZMQ encryption.
It manages devices from a configuration file and processes commands received from the gateway.
The hub provides secure, centralized device control and management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging based on debug or test flag
		if hubDebugFlag || hubTestFlag {
			logger.SetSilentMode(false) // Enable logging output
			if hubDebugFlag {
				logger.SetLevel("debug")
			} else {
				logger.SetLevel("info")
			}
		} else {
			logger.SetSilentMode(false) // Enable info level logging for daemon
			logger.SetLevel("info")
		}

		log := logger.New()
		log.Info().
			Str("config_path", hubConfigPath).
			Bool("debug", hubDebugFlag).
			Bool("test", hubTestFlag).
			Msg("Starting Lucas Hub daemon")

		// Check if config file exists
		if _, err := os.Stat(hubConfigPath); os.IsNotExist(err) {
			// Create default config
			defaultConfig := hub.NewDefaultConfig()
			if err := hub.SaveConfig(defaultConfig, hubConfigPath); err != nil {
				log.Error().Err(err).Msg("Failed to create default config file")
				return fmt.Errorf("failed to create default config file: %w", err)
			}
			log.Info().
				Str("config_path", hubConfigPath).
				Msg("Created default configuration file. Please edit it with your settings.")
			return nil
		}

		// Create and start daemon
		daemon, err := hub.NewDaemon(hubConfigPath, hubDebugFlag, hubTestFlag)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create hub daemon")
			return fmt.Errorf("failed to create hub daemon: %w", err)
		}

		// Start daemon (blocks until shutdown)
		if err := daemon.Start(); err != nil {
			log.Error().Err(err).Msg("Hub daemon stopped with error")
			return fmt.Errorf("hub daemon error: %w", err)
		}

		return nil
	},
}

var hubStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check hub daemon status",
	Long:  `Check the status of the running hub daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement status checking via IPC or API
		cmd.Println("Hub status checking not yet implemented.")
		cmd.Println("This would connect to a running hub daemon to check its status.")
		return nil
	},
}

var hubConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage hub configuration",
	Long:  `Generate or validate hub configuration files.`,
}

var hubConfigGenerateCmd = &cobra.Command{
	Use:   "generate [config-file]",
	Short: "Generate default configuration file",
	Long:  `Generate a default configuration file with example settings.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := hubConfigPath
		if len(args) > 0 {
			configPath = args[0]
		}

		// Create default config
		defaultConfig := hub.NewDefaultConfig()
		if err := hub.SaveConfig(defaultConfig, configPath); err != nil {
			return fmt.Errorf("failed to save default config: %w", err)
		}

		cmd.Printf("Default configuration saved to: %s\n", configPath)
		cmd.Println("Please edit the file with your actual gateway and device settings.")
		return nil
	},
}

var hubConfigValidateCmd = &cobra.Command{
	Use:   "validate [config-file]",
	Short: "Validate configuration file",
	Long:  `Validate a hub configuration file for syntax and required fields.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := hubConfigPath
		if len(args) > 0 {
			configPath = args[0]
		}

		// Load and validate config
		config, err := hub.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		cmd.Printf("Configuration file is valid: %s\n", configPath)
		cmd.Printf("Gateway endpoint: %s\n", config.Gateway.Endpoint)
		cmd.Printf("Configured devices: %d\n", len(config.Devices))
		
		for _, device := range config.Devices {
			cmd.Printf("  - %s (%s) at %s\n", device.ID, device.Type, device.Address)
		}

		return nil
	},
}

func init() {
	// Main hub command flags
	hubCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to hub configuration file")
	hubCmd.Flags().BoolVarP(&hubDebugFlag, "debug", "d", false, "Enable debug logging")
	hubCmd.Flags().BoolVar(&hubTestFlag, "test", false, "Enable test mode (simulate device responses)")

	// Add subcommands
	hubCmd.AddCommand(hubStatusCmd)
	hubCmd.AddCommand(hubConfigCmd)
	hubConfigCmd.AddCommand(hubConfigGenerateCmd)
	hubConfigCmd.AddCommand(hubConfigValidateCmd)

	// Config subcommand flags
	hubConfigGenerateCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path for generated configuration file")
	hubConfigValidateCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to configuration file to validate")
}