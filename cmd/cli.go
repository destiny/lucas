package cmd

import (
	"github.com/spf13/cobra"
	"lucas/cmd/cli"
)

var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Start the interactive CLI interface",
	Long: `Launch the interactive Terminal User Interface (TUI) for Lucas.
This provides a menu-driven interface for accessing various tools and utilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info().Msg("Starting Lucas CLI interface")
		
		if err := cli.StartTUI(); err != nil {
			log.Error().Err(err).Msg("Failed to start TUI")
			return err
		}
		
		return nil
	},
}