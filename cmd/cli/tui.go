package cli

import (
	"github.com/charmbracelet/bubbletea"
)

// Main TUI model that routes between screens
type model struct {
	currentScreen screen
	width         int
	height        int
	quitting      bool

	// Screen models
	setupModel  SetupModel
	remoteModel RemoteModel
}

func initialModel() model {
	return model{
		currentScreen: screenDeviceSetup,
		setupModel:    NewSetupModel(),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit handling
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.currentScreen == screenDeviceSetup {
				m.quitting = true
				return m, tea.Quit
			}
			// In remote screen, 'q' goes back to setup
			m.currentScreen = screenDeviceSetup
			m.setupModel = NewSetupModel()
			return m, nil
		}

		// Route messages to appropriate screen
		switch m.currentScreen {
		case screenDeviceSetup:
			var cmd tea.Cmd
			m.setupModel, cmd = m.setupModel.Update(msg)
			
			// Check if connection was successful
			if m.setupModel.IsConnected() {
				m.remoteModel = NewRemoteModel(m.setupModel.GetDevice(), m.setupModel.GetDeviceInfo())
				m.currentScreen = screenRemoteControl
			}
			
			return m, cmd

		case screenRemoteControl:
			var cmd tea.Cmd
			m.remoteModel, cmd = m.remoteModel.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return successStyle.Render("Thanks for using Lucas CLI!") + "\n"
	}

	// Route view rendering to appropriate screen
	switch m.currentScreen {
	case screenDeviceSetup:
		return m.setupModel.View()
	case screenRemoteControl:
		return m.remoteModel.View()
	default:
		return "Unknown screen"
	}
}

func StartTUI() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
