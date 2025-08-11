// Copyright 2025 Arion Yau
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"lucas/internal/logger"
)

var (
	verbose bool
	log     = logger.New()
)

var rootCmd = &cobra.Command{
	Use:   "lucas",
	Short: "Lucas - A powerful CLI tool with interactive features",
	Long: `Lucas is a command-line application that provides various tools and utilities.
It includes interactive TUI components and hub functionality for managing workflows.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			logger.SetSilentMode(false) // Enable logging output for verbose mode
			logger.SetLevel("debug")
		} else {
			logger.SetSilentMode(true) // Keep logging silent by default
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	
	// Initialize gateway command configuration
	initGatewayCmd()
	
	// Add subcommands
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(hubCmd)
	rootCmd.AddCommand(gatewayCmd)
}

// initGatewayCmd initializes the gateway command and its subcommands
func initGatewayCmd() {
	// This is moved from gateway.go init() function
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
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}