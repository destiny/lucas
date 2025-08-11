package cmd

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"lucas/internal/hub"
	"lucas/internal/logger"
)

var (
	hubConfigPath   string
	hubDebugFlag    bool
	hubTestFlag     bool
	hubGatewayURL   string
	hubVerboseFlag  bool
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
			// Create config with real generated keys
			log.Info().Msg("No configuration found, creating new hub configuration with keys...")
			
			config, err := hub.NewConfigWithKeys("", "")
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate hub configuration with keys")
				return fmt.Errorf("failed to generate hub configuration: %w", err)
			}
			
			if err := hub.SaveConfig(config, hubConfigPath); err != nil {
				log.Error().Err(err).Msg("Failed to save configuration file")
				return fmt.Errorf("failed to save configuration file: %w", err)
			}
			
			log.Info().
				Str("config_path", hubConfigPath).
				Str("hub_public_key", config.Hub.PublicKey).
				Msg("Created configuration file with generated keys")
			
			cmd.Printf("✅ Hub configuration created: %s\n", hubConfigPath)
			cmd.Printf("✅ Hub keys generated automatically\n")
			cmd.Printf("🔑 Product Key: %s\n", config.Hub.ProductKey)
			cmd.Printf("🔑 Public Key: %s\n", config.Hub.PublicKey)
			cmd.Printf("\n")
			
			// Offer interactive registration
			registrationSuccess := false
			if promptForRegistration() {
				gatewayURL := promptGatewayURL()
				if gatewayURL != "" {
					performInteractiveRegistration(config, gatewayURL)
					registrationSuccess = true
					cmd.Printf("\n🚀 Registration complete! Starting hub daemon...\n")
				} else {
					cmd.Printf("⚠ Registration cancelled\n")
					showManualSteps(cmd, hubConfigPath)
					return nil
				}
			} else {
				showManualSteps(cmd, hubConfigPath)
				return nil
			}
			
			// If registration was successful, reload the config with gateway info and continue
			if registrationSuccess {
				// Reload config to get updated gateway information
				config, err = hub.LoadConfig(hubConfigPath)
				if err != nil {
					log.Error().Err(err).Msg("Failed to reload configuration after registration")
					return fmt.Errorf("failed to reload configuration after registration: %w", err)
				}
			}
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

var hubInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hub with configuration and keys",
	Long: `Initialize the Lucas Hub by creating configuration file, generating keys,
and optionally discovering and registering with a gateway.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return initializeHub(cmd)
	},
}

var hubKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage hub cryptographic keys",
	Long:  `Generate, view, and manage CurveZMQ keys for the hub.`,
}

var hubKeysGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate new hub keys",
	Long:  `Generate a new CurveZMQ keypair for the hub and store in hub.yml config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var config *hub.Config
		configExists := false

		// Check if config already exists
		if _, err := os.Stat(hubConfigPath); err == nil {
			configExists = true
			// Load existing config
			config, err = hub.LoadConfig(hubConfigPath)
			if err != nil {
				return fmt.Errorf("failed to load existing config: %w", err)
			}

			// Check if keys already exist and aren't placeholders
			if config.HasValidHubKeys() {
				cmd.Printf("Hub keys already exist in: %s\n", hubConfigPath)
				cmd.Print("Do you want to overwrite them? [y/N]: ")
				
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					cmd.Println("Key generation cancelled")
					return nil
				}
			}
		} else {
			// Create new config with defaults
			config = hub.NewDefaultConfig()
		}

		// Generate new keypair
		hubKeys, err := hub.GenerateHubKeyPair()
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}

		// Update config with new keys
		config.Hub.PublicKey = hubKeys.PublicKey
		config.Hub.PrivateKey = hubKeys.PrivateKey

		// Save updated config to file
		if err := hub.SaveConfig(config, hubConfigPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		if configExists {
			cmd.Printf("Hub keys updated in: %s\n", hubConfigPath)
		} else {
			cmd.Printf("Hub configuration created with keys: %s\n", hubConfigPath)
		}
		cmd.Printf("Public key: %s\n", hubKeys.PublicKey)
		cmd.Println("IMPORTANT: Keep the private key secure and never share it!")

		// Optional: Register with gateway if URL provided
		if hubGatewayURL != "" {
			cmd.Printf("Registering new keys with gateway...\n")
			discovery := hub.NewGatewayDiscovery()
			
			// Use hub ID from configuration
			hubID := config.Hub.ID
			
			err = discovery.RegisterWithGateway(hubGatewayURL, hubID, hubKeys.PublicKey, config.Hub.ProductKey)
			if err != nil {
				cmd.Printf("⚠ Gateway registration failed: %v\n", err)
			} else {
				cmd.Printf("✓ Registered with gateway successfully\n")
			}
		}

		return nil
	},
}

var hubKeysShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show hub public key",
	Long:  `Display the hub's public key information from hub.yml config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		config, err := hub.LoadConfig(hubConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		cmd.Printf("Hub Key Information:\n")
		cmd.Printf("Config File: %s\n", hubConfigPath)
		cmd.Printf("Public Key: %s\n", config.Hub.PublicKey)
		cmd.Printf("Key Type: curve25519\n")
		cmd.Printf("Algorithm: CurveZMQ\n")
		cmd.Printf("Key Strength: 256-bit\n")
		
		if config.HasValidGatewayKey() {
			cmd.Printf("Gateway Key: %s\n", config.Gateway.PublicKey)
			cmd.Printf("Gateway Endpoint: %s\n", config.Gateway.Endpoint)
		} else {
			cmd.Printf("Gateway Key: Not configured\n")
		}

		// Validate key status
		if !config.HasValidHubKeys() {
			cmd.Printf("\n⚠ Warning: Hub keys are placeholders. Run 'lucas hub keys generate' to create real keys.\n")
		}

		return nil
	},
}

var hubRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register hub with gateway",
	Long: `Register the hub with a gateway by exchanging public keys.
This is automatically done during 'hub init' if a gateway is found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return registerWithGateway(cmd)
	},
}

// initializeHub handles the comprehensive hub initialization process
func initializeHub(cmd *cobra.Command) error {
	cmd.Printf("Initializing Lucas Hub...\n")

	// Check if configuration already exists
	configExists := false
	var existingConfig *hub.Config
	if _, err := os.Stat(hubConfigPath); err == nil {
		configExists = true
		cmd.Printf("✓ Configuration file already exists: %s\n", hubConfigPath)
		
		// Load existing config to check if keys are placeholders
		existingConfig, err = hub.LoadConfig(hubConfigPath)
		if err != nil {
			cmd.Printf("⚠ Warning: Failed to load existing config: %v\n", err)
		} else if !existingConfig.HasValidHubKeys() {
			cmd.Printf("⚠ Configuration has placeholder keys, will generate real keys\n")
			configExists = false // Treat as new config to generate keys
		}
	}

	var config *hub.Config
	var gatewayInfo *hub.GatewayInfo
	var err error

	if !configExists {
		// Try to discover gateway
		cmd.Printf("Discovering gateway...\n")
		discovery := hub.NewGatewayDiscovery()
		
		if hubGatewayURL != "" {
			// Use specific gateway URL
			gatewayInfo, err = discovery.GetGatewayInfo(hubGatewayURL)
			if err != nil {
				cmd.Printf("⚠ Could not connect to specified gateway: %v\n", err)
			} else {
				cmd.Printf("✓ Found gateway at %s\n", hubGatewayURL)
			}
		} else {
			// Try auto-discovery
			gatewayInfo, err = discovery.DiscoverGateway()
			if err != nil {
				cmd.Printf("⚠ Gateway not found at default locations: %v\n", err)
			} else {
				cmd.Printf("✓ Discovered gateway at %s\n", gatewayInfo.APIEndpoint)
			}
		}

		// Create configuration with keys
		gatewayEndpoint := ""
		gatewayPublicKey := ""
		
		if gatewayInfo != nil {
			gatewayEndpoint = gatewayInfo.ZMQEndpoint
			gatewayPublicKey = gatewayInfo.PublicKey
		}

		config, err = hub.NewConfigWithKeys(gatewayEndpoint, gatewayPublicKey)
		if err != nil {
			return fmt.Errorf("failed to create configuration: %w", err)
		}

		// Save configuration
		if err := hub.SaveConfig(config, hubConfigPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		cmd.Printf("✓ Configuration created: %s\n", hubConfigPath)
		cmd.Printf("✓ Hub keys generated\n")
	} else {
		// Load existing configuration with valid keys
		config = existingConfig
	}

	// Try to register with gateway if available
	if gatewayInfo != nil && config.HasValidHubKeys() {
		cmd.Printf("Registering with gateway...\n")
		
		discovery := hub.NewGatewayDiscovery()
		err = discovery.RegisterWithGateway(gatewayInfo.APIEndpoint, config.Hub.ID, config.Hub.PublicKey, config.Hub.ProductKey)
		if err != nil {
			cmd.Printf("⚠ Gateway registration failed: %v\n", err)
		} else {
			cmd.Printf("✓ Registered with gateway successfully\n")
		}
	}

	// Display summary
	cmd.Printf("\n✅ Hub initialization complete!\n")
	cmd.Printf("Configuration: %s\n", hubConfigPath)
	cmd.Printf("Hub ID: %s\n", config.Hub.ID)
	
	if config.HasValidHubKeys() {
		cmd.Printf("Hub Public Key: %s\n", config.Hub.PublicKey)
	}
	
	if config.HasValidGatewayKey() {
		cmd.Printf("Gateway: %s\n", config.Gateway.Endpoint)
		cmd.Printf("Status: Connected\n")
	} else {
		cmd.Printf("Gateway: Not configured\n")
		cmd.Printf("Status: Offline mode\n")
		cmd.Printf("\nTo connect to gateway later:\n")
		cmd.Printf("1. Start gateway: lucas gateway\n")
		cmd.Printf("2. Register hub: lucas hub register --gateway-url http://gateway:8080\n")
	}
	
	cmd.Printf("\nStart hub with: lucas hub\n")

	return nil
}

// registerWithGateway handles manual gateway registration
func registerWithGateway(cmd *cobra.Command) error {
	// Load configuration to get hub keys
	config, err := hub.LoadConfig(hubConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if !config.HasValidHubKeys() {
		return fmt.Errorf("hub keys not found, run 'lucas hub init' first")
	}

	// Determine gateway URL
	gatewayURL := hubGatewayURL
	if gatewayURL == "" && config.HasValidGatewayKey() {
		// Try to derive API URL from ZMQ endpoint
		zmqEndpoint := config.Gateway.Endpoint
		if strings.Contains(zmqEndpoint, ":5555") {
			gatewayURL = strings.Replace(zmqEndpoint, ":5555", ":8080", 1)
			gatewayURL = strings.Replace(gatewayURL, "tcp://", "http://", 1)
		}
	}

	if gatewayURL == "" {
		return fmt.Errorf("gateway URL not specified, use --gateway-url flag")
	}

	cmd.Printf("Registering with gateway at %s...\n", gatewayURL)

	// Use hub ID from configuration
	hubID := config.Hub.ID

	// Register with gateway
	discovery := hub.NewGatewayDiscovery()
	err = discovery.RegisterWithGateway(gatewayURL, hubID, config.Hub.PublicKey, config.Hub.ProductKey)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Get gateway info and update configuration
	gatewayInfo, err := discovery.GetGatewayInfo(gatewayURL)
	if err != nil {
		cmd.Printf("⚠ Could not retrieve gateway info: %v\n", err)
	} else {
		// Update configuration with gateway info
		config.UpdateGatewayInfo(gatewayInfo.ZMQEndpoint, gatewayInfo.PublicKey)
		
		// Save updated configuration
		if err := hub.SaveConfig(config, hubConfigPath); err != nil {
			cmd.Printf("⚠ Could not update configuration: %v\n", err)
		} else {
			cmd.Printf("✓ Configuration updated with gateway information\n")
		}
	}

	cmd.Printf("✅ Registration successful!\n")
	cmd.Printf("Hub ID: %s\n", hubID)
	cmd.Printf("Gateway: %s\n", gatewayURL)

	return nil
}

func init() {
	// Main hub command flags
	hubCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to hub configuration file")
	hubCmd.Flags().BoolVarP(&hubDebugFlag, "debug", "d", false, "Enable debug logging")
	hubCmd.Flags().BoolVar(&hubTestFlag, "test", false, "Enable test mode (simulate device responses)")

	// Add subcommands
	hubCmd.AddCommand(hubStatusCmd)
	hubCmd.AddCommand(hubConfigCmd)
	hubCmd.AddCommand(hubInitCmd)
	hubCmd.AddCommand(hubKeysCmd)
	hubCmd.AddCommand(hubRegisterCmd)
	
	// Config subcommands
	hubConfigCmd.AddCommand(hubConfigGenerateCmd)
	hubConfigCmd.AddCommand(hubConfigValidateCmd)
	
	// Keys subcommands
	hubKeysCmd.AddCommand(hubKeysGenerateCmd)
	hubKeysCmd.AddCommand(hubKeysShowCmd)

	// Init command flags
	hubInitCmd.Flags().StringVar(&hubGatewayURL, "gateway-url", "", "Gateway URL for registration (e.g., http://gateway:8080)")
	hubInitCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path for generated configuration file")

	// Register command flags
	hubRegisterCmd.Flags().StringVar(&hubGatewayURL, "gateway-url", "", "Gateway URL for registration (required)")
	hubRegisterCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to hub configuration file")

	// Keys command flags
	hubKeysGenerateCmd.Flags().StringVar(&hubGatewayURL, "gateway-url", "", "Gateway URL to register new keys")
	hubKeysGenerateCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to hub configuration file")
	hubKeysShowCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to hub configuration file")

	// Config subcommand flags  
	hubConfigGenerateCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path for generated configuration file")
	hubConfigValidateCmd.Flags().StringVarP(&hubConfigPath, "config", "c", "hub.yml", "Path to configuration file to validate")

	// Add to root command
	rootCmd.AddCommand(hubCmd)
}

// promptForRegistration asks user if they want to register with gateway now
func promptForRegistration() bool {
	fmt.Print("📡 Register with gateway now? [y/N]: ")
	
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// promptGatewayURL asks user for gateway URL with validation
func promptGatewayURL() string {
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("🌐 Gateway URL (http://localhost:8080): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return ""
		}
		
		input = strings.TrimSpace(input)
		
		// Use default if empty
		if input == "" {
			input = "http://localhost:8080"
		}
		
		// Validate URL format
		if _, err := url.Parse(input); err != nil {
			fmt.Printf("⚠ Invalid URL format. Please try again.\n")
			continue
		}
		
		// Ensure http/https prefix
		if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
			input = "http://" + input
		}
		
		return input
	}
}

// performInteractiveRegistration handles the interactive registration flow
func performInteractiveRegistration(config *hub.Config, gatewayURL string) {
	fmt.Printf("🔍 Discovering gateway at %s...\n", gatewayURL)
	
	// Use hub ID from configuration file (don't generate a new one)
	hubID := config.Hub.ID
	
	// Discovery and registration
	discovery := hub.NewGatewayDiscovery()
	
	// Check if gateway is reachable
	gatewayInfo, err := discovery.GetGatewayInfo(gatewayURL)
	if err != nil {
		fmt.Printf("⚠ Could not connect to gateway: %v\n", err)
		fmt.Println("💡 Make sure gateway is running and try: lucas hub register --gateway-url " + gatewayURL)
		return
	}
	
	fmt.Printf("✅ Gateway found at %s\n", gatewayInfo.APIEndpoint)
	fmt.Printf("📝 Registering hub...\n")
	
	// Register with gateway
	err = discovery.RegisterWithGateway(gatewayInfo.APIEndpoint, hubID, config.Hub.PublicKey, config.Hub.ProductKey)
	if err != nil {
		fmt.Printf("⚠ Registration failed: %v\n", err)
		fmt.Println("💡 You can try again with: lucas hub register --gateway-url " + gatewayURL)
		return
	}
	
	fmt.Printf("✅ Hub registered successfully!\n")
	fmt.Printf("🆔 Hub ID: %s\n", hubID)
	
	// Update config with gateway information
	config.UpdateGatewayInfo(gatewayInfo.ZMQEndpoint, gatewayInfo.PublicKey)
	
	// Save updated config
	if err := hub.SaveConfig(config, hubConfigPath); err != nil {
		fmt.Printf("⚠ Warning: Could not update config with gateway info: %v\n", err)
	} else {
		fmt.Printf("📋 Configuration updated with gateway information\n")
	}
}

// showManualSteps displays manual setup instructions
func showManualSteps(cmd *cobra.Command, configPath string) {
	cmd.Printf("\nNext steps:\n")
	cmd.Printf("1. Edit %s to configure your devices\n", configPath)
	cmd.Printf("2. Register with gateway: lucas hub init --gateway-url http://gateway:8080\n")
	cmd.Printf("3. Start hub: lucas hub --config %s\n", configPath)
}