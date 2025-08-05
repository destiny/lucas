package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"lucas/internal/bravia"
	"lucas/internal/logger"
)

var (
	braviaHost       string
	braviaCredential string
	braviaDebug      bool
)

var braviaCmd = &cobra.Command{
	Use:   "bravia",
	Short: "Control Sony Bravia TV",
	Long: `Control Sony Bravia TV using IRCC remote commands and JSON API.
Supports remote control operations and system status queries.`,
}

var braviaRemoteCmd = &cobra.Command{
	Use:   "remote [code]",
	Short: "Send remote control command",
	Long: `Send remote control command to Sony Bravia TV.
Available codes: power, volume-up, volume-down, mute, home, etc.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging based on debug flag
		if braviaDebug {
			logger.SetSilentMode(false) // Enable logging output
			logger.SetLevel("debug")
		}
		
		client := bravia.NewBraviaClient(braviaHost, braviaCredential, braviaDebug)
		
		// Map string commands to remote codes
		codeMap := map[string]bravia.BraviaRemoteCode{
			"power":       bravia.PowerButton,
			"power-on":    bravia.PowerOn,
			"power-off":   bravia.PowerOff,
			"volume-up":   bravia.VolumeUp,
			"volume-down": bravia.VolumeDown,
			"mute":        bravia.Mute,
			"channel-up":  bravia.ChannelUp,
			"channel-down": bravia.ChannelDown,
			"up":          bravia.Up,
			"down":        bravia.Down,
			"left":        bravia.Left,
			"right":       bravia.Right,
			"confirm":     bravia.Confirm,
			"home":        bravia.Home,
			"menu":        bravia.Menu,
			"back":        bravia.Back,
			"input":       bravia.Input,
			"hdmi1":       bravia.HDMI1,
			"hdmi2":       bravia.HDMI2,
			"hdmi3":       bravia.HDMI3,
			"hdmi4":       bravia.HDMI4,
			"play":        bravia.Play,
			"pause":       bravia.Pause,
			"stop":        bravia.Stop,
		}

		code, exists := codeMap[args[0]]
		if !exists {
			return fmt.Errorf("unknown remote code: %s", args[0])
		}

		log.Info().
			Str("host", braviaHost).
			Str("code", args[0]).
			Msg("Sending remote control command")

		err := client.RemoteRequest(code)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send remote command")
			return err
		}

		log.Info().Msg("Remote command sent successfully")
		return nil
	},
}

var braviaControlCmd = &cobra.Command{
	Use:   "control [method]",
	Short: "Send control API command",
	Long: `Send control API command to Sony Bravia TV.
Available methods: power-status, volume-info, playing-content, etc.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set up logging based on debug flag
		if braviaDebug {
			logger.SetSilentMode(false) // Enable logging output
			logger.SetLevel("debug")
		}
		
		client := bravia.NewBraviaClient(braviaHost, braviaCredential, braviaDebug)

		// Map string commands to API methods and endpoints
		type apiCommand struct {
			endpoint bravia.BraviaEndpoint
			method   bravia.BraviaMethod
		}

		commandMap := map[string]apiCommand{
			"power-status": {bravia.SystemEndpoint, bravia.GetPowerStatus},
			"system-info":  {bravia.SystemEndpoint, bravia.GetSystemInformation},
			"volume-info":  {bravia.AudioEndpoint, bravia.GetVolumeInformation},
			"playing-content": {bravia.AVContentEndpoint, bravia.GetPlayingContentInfo},
			"app-list":     {bravia.AppControlEndpoint, bravia.GetApplicationList},
			"content-list": {bravia.AVContentEndpoint, bravia.GetContentList},
		}

		apiCmd, exists := commandMap[args[0]]
		if !exists {
			return fmt.Errorf("unknown control method: %s", args[0])
		}

		log.Info().
			Str("host", braviaHost).
			Str("method", args[0]).
			Msg("Sending control API command")

		payload := bravia.CreatePayload(1, apiCmd.method, nil)
		resp, err := client.ControlRequest(apiCmd.endpoint, payload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send control command")
			return err
		}
		defer resp.Body.Close()

		// Read and display response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// Pretty print JSON response
		var result interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			prettyJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(prettyJSON))
		} else {
			fmt.Println(string(body))
		}

		return nil
	},
}

var braviaListCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List available commands or codes",
	Long:  `List available remote codes or control methods.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "remote", "codes":
			fmt.Println("Available remote control codes:")
			fmt.Println("  power, power-on, power-off")
			fmt.Println("  volume-up, volume-down, mute")
			fmt.Println("  channel-up, channel-down")
			fmt.Println("  up, down, left, right, confirm")
			fmt.Println("  home, menu, back, input")
			fmt.Println("  hdmi1, hdmi2, hdmi3, hdmi4")
			fmt.Println("  play, pause, stop")
		case "control", "methods":
			fmt.Println("Available control API methods:")
			fmt.Println("  power-status    - Get TV power status")
			fmt.Println("  system-info     - Get system information")
			fmt.Println("  volume-info     - Get volume information")
			fmt.Println("  playing-content - Get currently playing content")
			fmt.Println("  app-list        - Get installed applications")
			fmt.Println("  content-list    - Get available content")
		default:
			return fmt.Errorf("unknown list type: %s (use 'remote' or 'control')", args[0])
		}
		return nil
	},
}

func init() {
	// Add persistent flags for bravia commands
	braviaCmd.PersistentFlags().StringVarP(&braviaHost, "host", "H", "", "Bravia TV host address")
	braviaCmd.PersistentFlags().StringVarP(&braviaCredential, "credential", "c", "", "Authentication credential (PSK)")
	braviaCmd.PersistentFlags().BoolVarP(&braviaDebug, "debug", "d", false, "Enable debug logging")
	
	// Mark flags as required for remote and control commands (not list)
	braviaRemoteCmd.MarkFlagRequired("host")
	braviaRemoteCmd.MarkFlagRequired("credential")
	braviaControlCmd.MarkFlagRequired("host")
	braviaControlCmd.MarkFlagRequired("credential")

	// Add subcommands
	braviaCmd.AddCommand(braviaRemoteCmd)
	braviaCmd.AddCommand(braviaControlCmd)
	braviaCmd.AddCommand(braviaListCmd)

	// Add to root command
	rootCmd.AddCommand(braviaCmd)
}