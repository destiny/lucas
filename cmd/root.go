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
			logger.SetLevel("debug")
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	
	// Add subcommands
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(hubCmd)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}