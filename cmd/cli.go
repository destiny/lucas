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
	"github.com/spf13/cobra"
	"lucas/cmd/cli"
	"lucas/internal/logger"
)

var (
	debugFlag bool
	testFlag  bool
)

var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Start the interactive CLI interface",
	Long: `Launch the interactive Terminal User Interface (TUI) for Lucas.
This provides a menu-driven interface for accessing various tools and utilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging based on debug or test flag
		if debugFlag || testFlag {
			logger.SetSilentMode(false) // Enable logging output
			if debugFlag {
				logger.SetLevel("debug")
			}
		} else {
			logger.SetSilentMode(true) // Keep logging silent
		}

		log := logger.New()
		log.Info().
			Bool("debug", debugFlag).
			Bool("test", testFlag).
			Msg("Starting Lucas CLI interface")

		if err := cli.StartTUI(debugFlag, testFlag); err != nil {
			log.Error().Err(err).Msg("Failed to start TUI")
			return err
		}

		return nil
	},
}

func init() {
	cliCmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable debug logging for HTTP requests")
	cliCmd.Flags().BoolVar(&testFlag, "test", false, "Enable test mode (simulate device responses without HTTP calls)")
}
