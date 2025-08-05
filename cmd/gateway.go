package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"lucas/internal/gateway"
	"lucas/internal/logger"
)

var (
	gatewayDBPath     string
	gatewayKeysPath   string
	gatewayZMQAddr    string
	gatewayAPIAddr    string
	gatewayDebugFlag  bool
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the Lucas Gateway daemon",
	Long: `Lucas Gateway is a daemon service that manages multiple hubs and provides REST API access.
It handles hub registration, device management, and secure communication with hubs via ZMQ.
The gateway provides a central point for managing distributed IoT devices across multiple locations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging
		if gatewayDebugFlag {
			logger.SetSilentMode(false)
			logger.SetLevel("debug")
		} else {
			logger.SetSilentMode(false)
			logger.SetLevel("info")
		}

		log := logger.New()
		log.Info().
			Str("db_path", gatewayDBPath).
			Str("keys_path", gatewayKeysPath).
			Str("zmq_address", gatewayZMQAddr).
			Str("api_address", gatewayAPIAddr).
			Bool("debug", gatewayDebugFlag).
			Msg("Starting Lucas Gateway daemon")

		// Initialize database
		database, err := gateway.NewDatabase(gatewayDBPath)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize database")
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		// Load or generate keys
		keys, err := gateway.LoadOrGenerateGatewayKeys(gatewayKeysPath)
		if err != nil {
			log.Error().Err(err).Msg("Failed to load gateway keys")
			return fmt.Errorf("failed to load gateway keys: %w", err)
		}

		log.Info().
			Str("public_key", keys.GetServerPublicKey()).
			Msg("Gateway keys loaded")

		// Initialize ZMQ server
		zmqServer := gateway.NewZMQServer(gatewayZMQAddr, keys, database)

		// Initialize API server
		apiServer := gateway.NewAPIServer(database, zmqServer)

		// Start services
		var wg sync.WaitGroup
		errChan := make(chan error, 2)

		// Start ZMQ server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := zmqServer.Start(); err != nil {
				errChan <- fmt.Errorf("ZMQ server error: %w", err)
			}
		}()

		// Start API server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := apiServer.Start(gatewayAPIAddr); err != nil {
				errChan <- fmt.Errorf("API server error: %w", err)
			}
		}()

		// Handle graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigChan:
			log.Info().
				Str("signal", sig.String()).
				Msg("Received shutdown signal")
		case err := <-errChan:
			log.Error().Err(err).Msg("Service error")
			return err
		}

		// Shutdown services
		log.Info().Msg("Shutting down gateway services")
		
		if err := zmqServer.Stop(); err != nil {
			log.Error().Err(err).Msg("Error stopping ZMQ server")
		}
		
		if err := apiServer.Stop(); err != nil {
			log.Error().Err(err).Msg("Error stopping API server")
		}

		log.Info().Msg("Gateway daemon stopped")
		return nil
	},
}

var gatewayKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage gateway cryptographic keys",
	Long:  `Generate, view, and manage CurveZMQ keys for the gateway.`,
}

var gatewayKeysGenerateCmd = &cobra.Command{
	Use:   "generate [keys-file]",
	Short: "Generate new gateway keys",
	Long:  `Generate a new CurveZMQ keypair for the gateway server.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keysPath := gatewayKeysPath
		if len(args) > 0 {
			keysPath = args[0]
		}

		// Check if keys already exist
		if _, err := os.Stat(keysPath); err == nil {
			cmd.Printf("Keys already exist at: %s\n", keysPath)
			cmd.Print("Do you want to overwrite them? [y/N]: ")
			
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				cmd.Println("Key generation cancelled")
				return nil
			}
		}

		// Generate new keys
		keys, err := gateway.CreateDefaultGatewayKeys()
		if err != nil {
			return fmt.Errorf("failed to generate keys: %w", err)
		}

		// Save keys
		if err := gateway.SaveGatewayKeys(keys, keysPath); err != nil {
			return fmt.Errorf("failed to save keys: %w", err)
		}

		cmd.Printf("Gateway keys generated and saved to: %s\n", keysPath)
		cmd.Printf("Public key: %s\n", keys.GetServerPublicKey())
		cmd.Println("IMPORTANT: Keep the private key secure and never share it!")

		return nil
	},
}

var gatewayKeysShowCmd = &cobra.Command{
	Use:   "show [keys-file]",
	Short: "Show gateway public key",
	Long:  `Display the gateway's public key information.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keysPath := gatewayKeysPath
		if len(args) > 0 {
			keysPath = args[0]
		}

		keys, err := gateway.LoadGatewayKeys(keysPath)
		if err != nil {
			return fmt.Errorf("failed to load keys: %w", err)
		}

		keyInfo := keys.GetKeyInfo()
		securityInfo := keys.GetSecurityInfo()

		cmd.Printf("Gateway Key Information:\n")
		cmd.Printf("Public Key: %s\n", keyInfo.PublicKey)
		cmd.Printf("Key Type: %s\n", keyInfo.KeyType)
		cmd.Printf("Algorithm: %s\n", securityInfo.Algorithm)
		cmd.Printf("Curve: %s\n", securityInfo.Curve)
		cmd.Printf("Key Strength: %s\n", securityInfo.KeyStrength)

		return nil
	},
}

var gatewayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check gateway daemon status",
	Long:  `Check the status of the running gateway daemon via HTTP API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This would make an HTTP request to the gateway API
		cmd.Printf("Gateway status checking via API: %s\n", gatewayAPIAddr)
		cmd.Println("Not yet implemented - would check /api/v1/gateway/status")
		return nil
	},
}

var gatewayInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gateway with default configuration",
	Long:  `Initialize the gateway by creating default database and generating keys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Initializing gateway...\n")

		// Generate keys if they don't exist
		if _, err := os.Stat(gatewayKeysPath); os.IsNotExist(err) {
			cmd.Printf("Generating gateway keys: %s\n", gatewayKeysPath)
			keys, err := gateway.CreateDefaultGatewayKeys()
			if err != nil {
				return fmt.Errorf("failed to generate keys: %w", err)
			}

			if err := gateway.SaveGatewayKeys(keys, gatewayKeysPath); err != nil {
				return fmt.Errorf("failed to save keys: %w", err)
			}

			cmd.Printf("✓ Keys generated: %s\n", keys.GetServerPublicKey())
		} else {
			cmd.Printf("✓ Keys already exist: %s\n", gatewayKeysPath)
		}

		// Initialize database
		cmd.Printf("Initializing database: %s\n", gatewayDBPath)
		database, err := gateway.NewDatabase(gatewayDBPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		// Create default user
		defaultUser, err := database.CreateUser("admin", "admin@example.com")
		if err != nil {
			// User might already exist
			cmd.Printf("⚠ Default user creation: %v\n", err)
		} else {
			cmd.Printf("✓ Default user created: %s (API Key: %s)\n", defaultUser.Username, defaultUser.APIKey)
		}

		cmd.Printf("\n✅ Gateway initialization complete!\n")
		cmd.Printf("Start the gateway with: lucas gateway\n")
		cmd.Printf("ZMQ Address: %s\n", gatewayZMQAddr)
		cmd.Printf("API Address: %s\n", gatewayAPIAddr)

		return nil
	},
}

func init() {
	// Main gateway command flags
	gatewayCmd.Flags().StringVar(&gatewayDBPath, "db", "gateway.db", "Path to SQLite database file")
	gatewayCmd.Flags().StringVar(&gatewayKeysPath, "keys", "gateway_keys.json", "Path to gateway keys file")
	gatewayCmd.Flags().StringVar(&gatewayZMQAddr, "zmq-addr", "tcp://*:5555", "ZMQ server bind address")
	gatewayCmd.Flags().StringVar(&gatewayAPIAddr, "api-addr", ":8080", "HTTP API server address")
	gatewayCmd.Flags().BoolVarP(&gatewayDebugFlag, "debug", "d", false, "Enable debug logging")

	// Add subcommands
	gatewayCmd.AddCommand(gatewayKeysCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
	gatewayCmd.AddCommand(gatewayInitCmd)

	// Keys subcommands
	gatewayKeysCmd.AddCommand(gatewayKeysGenerateCmd)
	gatewayKeysCmd.AddCommand(gatewayKeysShowCmd)

	// Keys command flags
	gatewayKeysGenerateCmd.Flags().StringVar(&gatewayKeysPath, "keys", "gateway_keys.json", "Path for generated keys file")
	gatewayKeysShowCmd.Flags().StringVar(&gatewayKeysPath, "keys", "gateway_keys.json", "Path to keys file")

	// Add to root command
	rootCmd.AddCommand(gatewayCmd)
}