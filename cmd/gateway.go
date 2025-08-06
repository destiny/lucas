package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"lucas/internal/gateway"
	"lucas/internal/logger"
)

var (
	gatewayConfigPath string
	gatewayDBPath     string
	gatewayKeysPath   string
	gatewayZMQAddr    string
	gatewayAPIAddr    string
	gatewayDebugFlag  bool
	gatewayVerboseStatus bool
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the Lucas Gateway daemon",
	Long: `Lucas Gateway is a daemon service that manages multiple hubs and provides REST API access.
It handles hub registration, device management, and secure communication with hubs via ZMQ.
The gateway provides a central point for managing distributed IoT devices across multiple locations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		config, err := loadGatewayConfiguration()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Set up logging based on configuration
		setupLogging(config)

		log := logger.New()
		log.Info().
			Str("config_file", gatewayConfigPath).
			Str("db_path", config.Database.Path).
			Str("keys_path", config.Keys.File).
			Str("zmq_address", config.Server.ZMQ.Address).
			Str("api_address", config.Server.API.Address).
			Str("log_level", config.Logging.Level).
			Msg("Starting Lucas Gateway daemon")

		// Initialize database
		database, err := gateway.NewDatabase(config.Database.Path)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize database")
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		// Load or generate keys
		keys, err := gateway.LoadOrGenerateGatewayKeys(config.Keys.File)
		if err != nil {
			log.Error().Err(err).Msg("Failed to load gateway keys")
			return fmt.Errorf("failed to load gateway keys: %w", err)
		}

		log.Info().
			Str("public_key", keys.GetServerPublicKey()).
			Msg("Gateway keys loaded")

		// Initialize ZMQ server
		zmqServer := gateway.NewZMQServer(config.Server.ZMQ.Address, keys, database)

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
			if err := apiServer.Start(config.Server.API.Address); err != nil {
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
		return checkGatewayStatus(cmd)
	},
}

var gatewayInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gateway with default configuration",
	Long:  `Initialize the gateway by creating default configuration file, database and generating keys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Initializing gateway...\n")

		// Use provided config path or default
		configPath := gatewayConfigPath
		if configPath == "" {
			configPath = "gateway.yml"
		}

		// Create default configuration if it doesn't exist
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			cmd.Printf("Creating default configuration: %s\n", configPath)
			config := gateway.NewDefaultGatewayConfig()
			
			if err := gateway.SaveGatewayConfig(config, configPath); err != nil {
				return fmt.Errorf("failed to save config file: %w", err)
			}
			
			cmd.Printf("✓ Configuration file created: %s\n", configPath)
		} else {
			cmd.Printf("✓ Configuration file already exists: %s\n", configPath)
		}

		// Load configuration to use for initialization
		config, err := gateway.LoadGatewayConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Apply CLI overrides for init
		if gatewayDBPath != "" {
			config.Database.Path = gatewayDBPath
		}
		if gatewayKeysPath != "" {
			config.Keys.File = gatewayKeysPath
		}

		// Generate keys if they don't exist
		if _, err := os.Stat(config.Keys.File); os.IsNotExist(err) {
			cmd.Printf("Generating gateway keys: %s\n", config.Keys.File)
			keys, err := gateway.CreateDefaultGatewayKeys()
			if err != nil {
				return fmt.Errorf("failed to generate keys: %w", err)
			}

			if err := gateway.SaveGatewayKeys(keys, config.Keys.File); err != nil {
				return fmt.Errorf("failed to save keys: %w", err)
			}

			cmd.Printf("✓ Keys generated: %s\n", keys.GetServerPublicKey())
		} else {
			cmd.Printf("✓ Keys already exist: %s\n", config.Keys.File)
		}

		// Initialize database
		cmd.Printf("Initializing database: %s\n", config.Database.Path)
		database, err := gateway.NewDatabase(config.Database.Path)
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
		cmd.Printf("Configuration: %s\n", configPath)
		cmd.Printf("Start the gateway with: lucas gateway -c %s\n", configPath)
		cmd.Printf("ZMQ Address: %s\n", config.Server.ZMQ.Address)
		cmd.Printf("API Address: %s\n", config.Server.API.Address)
		cmd.Printf("Health endpoint: http://localhost%s/api/v1/health\n", config.Server.API.Address)

		return nil
	},
}

// loadGatewayConfiguration loads configuration from file and applies CLI flag overrides
func loadGatewayConfiguration() (*gateway.GatewayConfig, error) {
	var config *gateway.GatewayConfig
	var err error

	// Try to load configuration file
	if gatewayConfigPath != "" {
		if _, statErr := os.Stat(gatewayConfigPath); statErr == nil {
			config, err = gateway.LoadGatewayConfig(gatewayConfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load config file: %w", err)
			}
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("failed to check config file: %w", statErr)
		}
	}

	// If no config file or config file doesn't exist, use defaults
	if config == nil {
		config = gateway.NewDefaultGatewayConfig()
	}

	// Apply CLI flag overrides (if flags were explicitly set)
	if gatewayDBPath != "" {
		config.Database.Path = gatewayDBPath
	}
	if gatewayKeysPath != "" {
		config.Keys.File = gatewayKeysPath
	}
	if gatewayZMQAddr != "" {
		config.Server.ZMQ.Address = gatewayZMQAddr
	}
	if gatewayAPIAddr != "" {
		config.Server.API.Address = gatewayAPIAddr
	}
	if gatewayDebugFlag {
		config.Logging.Level = "debug"
	}

	return config, nil
}

// setupLogging configures the logger based on configuration
func setupLogging(config *gateway.GatewayConfig) {
	logger.SetSilentMode(false)
	logger.SetLevel(config.Logging.Level)
	
	// Additional logging setup could be done here based on config.Logging.Format, etc.
}

// GatewayStatusResponse represents the structure returned by /api/v1/gateway/status
type GatewayStatusResponse struct {
	Status         string                 `json:"status"`
	ActiveHubs     int                    `json:"active_hubs"`
	HubConnections map[string]interface{} `json:"hub_connections"`
	Uptime         string                 `json:"uptime"`
	Version        string                 `json:"version"`
	Timestamp      string                 `json:"timestamp"`
}

// GatewayHealthResponse represents the structure returned by /api/v1/health
type GatewayHealthResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
	Timestamp  string            `json:"timestamp"`
}

// checkGatewayStatus checks the status of the running gateway daemon
func checkGatewayStatus(cmd *cobra.Command) error {
	// Load configuration to determine API address
	config, configPath, err := loadGatewayConfigurationForStatus()
	if err != nil {
		cmd.Printf("⚠ Warning: Could not load configuration: %v\n", err)
		cmd.Printf("Using default settings\n\n")
		config = gateway.NewDefaultGatewayConfig()
		configPath = "gateway.yml (default)"
	}

	apiAddr := config.Server.API.Address
	if !strings.HasPrefix(apiAddr, "http://") && !strings.HasPrefix(apiAddr, "https://") {
		apiAddr = "http://localhost" + apiAddr
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try to get gateway status
	statusURL := apiAddr + "/api/v1/gateway/status"
	healthURL := apiAddr + "/api/v1/health"
	
	statusResp, statusErr := makeHTTPRequest(client, statusURL)
	healthResp, healthErr := makeHTTPRequest(client, healthURL)

	if gatewayVerboseStatus {
		return displayVerboseStatus(cmd, config, configPath, statusResp, healthResp, statusErr, healthErr)
	} else {
		return displayCompactStatus(cmd, config, configPath, statusResp, healthResp, statusErr, healthErr)
	}
}

// loadGatewayConfigurationForStatus loads configuration for status checking with error handling
func loadGatewayConfigurationForStatus() (*gateway.GatewayConfig, string, error) {
	var config *gateway.GatewayConfig
	var err error
	var configPath string = gatewayConfigPath

	// Use default config path if not specified
	if configPath == "" {
		configPath = "gateway.yml"
	}

	// Try to load configuration file
	if _, statErr := os.Stat(configPath); statErr == nil {
		config, err = gateway.LoadGatewayConfig(configPath)
		if err != nil {
			return nil, configPath, fmt.Errorf("failed to load config file: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return nil, configPath, fmt.Errorf("failed to check config file: %w", statErr)
	} else {
		// Config file doesn't exist, use defaults
		config = gateway.NewDefaultGatewayConfig()
		configPath = "gateway.yml (not found, using defaults)"
	}

	// Apply CLI flag overrides
	if gatewayAPIAddr != "" {
		config.Server.API.Address = gatewayAPIAddr
		configPath += " (with CLI overrides)"
	}

	return config, configPath, nil
}

// makeHTTPRequest makes an HTTP GET request and returns the response body
func makeHTTPRequest(client *http.Client, url string) (map[string]interface{}, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// displayCompactStatus displays a user-friendly compact status
func displayCompactStatus(cmd *cobra.Command, config *gateway.GatewayConfig, configPath string, statusResp, healthResp map[string]interface{}, statusErr, healthErr error) error {
	// Determine overall status
	isOnline := statusErr == nil && healthErr == nil
	
	if isOnline {
		cmd.Printf("Gateway Status: ✓ RUNNING\n")
	} else {
		cmd.Printf("Gateway Status: ✗ OFFLINE\n")
		if statusErr != nil {
			cmd.Printf("Connection Error: %v\n", statusErr)
		}
		return nil
	}

	// Display basic information
	apiAddr := config.Server.API.Address
	if !strings.HasPrefix(apiAddr, "http://") && !strings.HasPrefix(apiAddr, "https://") {
		apiAddr = "localhost" + apiAddr
	}
	cmd.Printf("API Address: %s\n", apiAddr)
	cmd.Printf("ZMQ Address: %s\n", config.Server.ZMQ.Address)
	cmd.Printf("Configuration: %s\n", configPath)

	// Display status details if available
	if statusResp != nil {
		if activeHubs, ok := statusResp["active_hubs"].(float64); ok {
			cmd.Printf("Active Hubs: %.0f\n", activeHubs)
		}
		if version, ok := statusResp["version"].(string); ok {
			cmd.Printf("Version: %s\n", version)
		}
		if uptime, ok := statusResp["uptime"].(string); ok && uptime != "N/A" {
			cmd.Printf("Uptime: %s\n", uptime)
		}
	}

	// Display health information
	if healthResp != nil {
		if components, ok := healthResp["components"].(map[string]interface{}); ok {
			for component, status := range components {
				if statusStr, ok := status.(string); ok {
					icon := "✓"
					if statusStr != "healthy" {
						icon = "✗"
					}
					cmd.Printf("%s: %s %s\n", strings.Title(component), icon, strings.Title(statusStr))
				}
			}
		}
	}

	return nil
}

// displayVerboseStatus displays detailed JSON status information
func displayVerboseStatus(cmd *cobra.Command, config *gateway.GatewayConfig, configPath string, statusResp, healthResp map[string]interface{}, statusErr, healthErr error) error {
	result := map[string]interface{}{
		"online": statusErr == nil && healthErr == nil,
		"config": map[string]interface{}{
			"file":        configPath,
			"api_address": config.Server.API.Address,
			"zmq_address": config.Server.ZMQ.Address,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if statusErr != nil {
		result["status_error"] = statusErr.Error()
	} else {
		result["status"] = statusResp
	}

	if healthErr != nil {
		result["health_error"] = healthErr.Error()
	} else {
		result["health"] = healthResp
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func init() {
	// Main gateway command flags
	gatewayCmd.Flags().StringVarP(&gatewayConfigPath, "config", "c", "gateway.yml", "Path to configuration file")
	gatewayCmd.Flags().StringVar(&gatewayDBPath, "db", "", "Path to SQLite database file (overrides config)")
	gatewayCmd.Flags().StringVar(&gatewayKeysPath, "keys", "", "Path to gateway keys file (overrides config)")
	gatewayCmd.Flags().StringVar(&gatewayZMQAddr, "zmq-addr", "", "ZMQ server bind address (overrides config)")
	gatewayCmd.Flags().StringVar(&gatewayAPIAddr, "api-addr", "", "HTTP API server address (overrides config)")
	gatewayCmd.Flags().BoolVarP(&gatewayDebugFlag, "debug", "d", false, "Enable debug logging (overrides config)")

	// Add subcommands
	gatewayCmd.AddCommand(gatewayKeysCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
	gatewayCmd.AddCommand(gatewayInitCmd)

	// Status command flags
	gatewayStatusCmd.Flags().BoolVarP(&gatewayVerboseStatus, "verbose", "v", false, "Show detailed status information in JSON format")
	gatewayStatusCmd.Flags().StringVarP(&gatewayConfigPath, "config", "c", "gateway.yml", "Path to configuration file")
	gatewayStatusCmd.Flags().StringVar(&gatewayAPIAddr, "api-addr", "", "API server address to check (overrides config)")

	// Keys subcommands
	gatewayKeysCmd.AddCommand(gatewayKeysGenerateCmd)
	gatewayKeysCmd.AddCommand(gatewayKeysShowCmd)

	// Keys command flags (these still use the old defaults for backward compatibility)
	gatewayKeysGenerateCmd.Flags().StringVar(&gatewayKeysPath, "keys", "gateway_keys.yml", "Path for generated keys file")
	gatewayKeysShowCmd.Flags().StringVar(&gatewayKeysPath, "keys", "gateway_keys.yml", "Path to keys file")

	// Add to root command
	rootCmd.AddCommand(gatewayCmd)
}