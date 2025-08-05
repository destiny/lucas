package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lucas/internal/device"
	"lucas/internal/logger"
)

// RemoteModel handles the remote control screen
type RemoteModel struct {
	// Connected device
	device     device.Device
	deviceInfo device.DeviceInfo

	// Remote control state
	selectedButton  remoteButton
	lastButtonPress time.Time

	// Response and history
	lastResponse  *device.ActionResponse
	actionHistory []actionHistoryEntry
}

// NewRemoteModel creates a new remote control screen model
func NewRemoteModel(dev device.Device, info device.DeviceInfo) RemoteModel {
	return RemoteModel{
		device:        dev,
		deviceInfo:    info,
		actionHistory: []actionHistoryEntry{},
	}
}

// Update handles remote control screen messages
func (m RemoteModel) Update(msg tea.Msg) (RemoteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Navigation keys
		case "up":
			return m.handleRemoteButton(buttonUp)
		case "down":
			return m.handleRemoteButton(buttonDown)
		case "left":
			return m.handleRemoteButton(buttonLeft)
		case "right":
			return m.handleRemoteButton(buttonRight)
		case "enter":
			return m.handleRemoteButton(buttonOK)

		// Power and volume
		case "p":
			return m.handleRemoteButton(buttonPower)
		case "+", "=":
			return m.handleRemoteButton(buttonVolumeUp)
		case "-":
			return m.handleRemoteButton(buttonVolumeDown)
		case "m":
			return m.handleRemoteButton(buttonMute)

		// Channel controls
		case "ctrl+up":
			return m.handleRemoteButton(buttonChannelUp)
		case "ctrl+down":
			return m.handleRemoteButton(buttonChannelDown)

		// Number keys
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			return m.handleNumberKey(msg.String())

		// Function keys
		case "h":
			return m.handleRemoteButton(buttonHome)
		case "ctrl+m":
			return m.handleRemoteButton(buttonMenu)
		case "backspace":
			return m.handleRemoteButton(buttonBack)
		case "i":
			return m.handleRemoteButton(buttonInput)

		// HDMI shortcuts
		case "f1":
			return m.handleRemoteButton(buttonHDMI1)
		case "f2":
			return m.handleRemoteButton(buttonHDMI2)
		case "f3":
			return m.handleRemoteButton(buttonHDMI3)
		case "f4":
			return m.handleRemoteButton(buttonHDMI4)
		}
	}

	return m, nil
}

// View renders the remote control screen
func (m RemoteModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Lucas CLI - TV Remote Control"))
	b.WriteString("\n\n")

	// Device Info
	b.WriteString(successStyle.Render("📺 " + m.deviceInfo.Model + " Connected"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Address: %s", m.deviceInfo.Address))
	b.WriteString("\n\n")

	// Remote Control Layout
	b.WriteString(m.renderRemoteLayout())

	// Status and Last Action
	if m.lastResponse != nil {
		b.WriteString("\n")
		if m.lastResponse.Success {
			b.WriteString(successStyle.Render("✓ " + fmt.Sprintf("%v", m.lastResponse.Data)))
		} else {
			b.WriteString(errorStyle.Render("✗ " + m.lastResponse.Error))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Arrows: Navigate • Enter: OK • P: Power • +/-: Volume • M: Mute • 0-9: Numbers • H: Home • I: Input • F1-F4: HDMI • q: Disconnect"))

	return b.String()
}

// renderRemoteLayout creates the visual remote control layout
func (m RemoteModel) renderRemoteLayout() string {
	var b strings.Builder

	// Get button style based on recent press
	getButtonStyle := func(btn remoteButton) lipgloss.Style {
		base := remoteButtonStyle
		if m.selectedButton == btn && time.Since(m.lastButtonPress) < 200*time.Millisecond {
			base = remoteButtonActiveStyle
		}
		return base
	}

	// Power button (top center)
	b.WriteString("                    ")
	b.WriteString(getButtonStyle(buttonPower).Render(" PWR "))
	b.WriteString("\n\n")

	// Volume and Channel controls with Navigation pad
	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonVolumeUp).Render("VOL+"))
	b.WriteString("           ")
	b.WriteString(getButtonStyle(buttonChannelUp).Render("CH+"))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonVolumeDown).Render("VOL-"))
	b.WriteString("      ")
	b.WriteString(getButtonStyle(buttonUp).Render(" ↑ "))
	b.WriteString("      ")
	b.WriteString(getButtonStyle(buttonChannelDown).Render("CH-"))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonMute).Render("MUTE"))
	b.WriteString("   ")
	b.WriteString(getButtonStyle(buttonLeft).Render(" ← "))
	b.WriteString(getButtonStyle(buttonOK).Render(" OK "))
	b.WriteString(getButtonStyle(buttonRight).Render(" → "))
	b.WriteString("\n")

	b.WriteString("                 ")
	b.WriteString(getButtonStyle(buttonDown).Render(" ↓ "))
	b.WriteString("\n\n")

	// Number pad
	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonNum1).Render(" 1 "))
	b.WriteString(getButtonStyle(buttonNum2).Render(" 2 "))
	b.WriteString(getButtonStyle(buttonNum3).Render(" 3 "))
	b.WriteString("     ")
	b.WriteString(getButtonStyle(buttonHome).Render("HOME"))
	b.WriteString(getButtonStyle(buttonMenu).Render("MENU"))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonNum4).Render(" 4 "))
	b.WriteString(getButtonStyle(buttonNum5).Render(" 5 "))
	b.WriteString(getButtonStyle(buttonNum6).Render(" 6 "))
	b.WriteString("     ")
	b.WriteString(getButtonStyle(buttonBack).Render("BACK"))
	b.WriteString(getButtonStyle(buttonInput).Render("INPUT"))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(getButtonStyle(buttonNum7).Render(" 7 "))
	b.WriteString(getButtonStyle(buttonNum8).Render(" 8 "))
	b.WriteString(getButtonStyle(buttonNum9).Render(" 9 "))
	b.WriteString("\n")

	b.WriteString("        ")
	b.WriteString(getButtonStyle(buttonNum0).Render(" 0 "))
	b.WriteString("\n\n")

	// HDMI shortcuts
	b.WriteString("              ")
	b.WriteString(getButtonStyle(buttonHDMI1).Render("HDMI1"))
	b.WriteString(getButtonStyle(buttonHDMI2).Render("HDMI2"))
	b.WriteString("\n")
	b.WriteString("              ")
	b.WriteString(getButtonStyle(buttonHDMI3).Render("HDMI3"))
	b.WriteString(getButtonStyle(buttonHDMI4).Render("HDMI4"))
	b.WriteString("\n")

	return b.String()
}

// handleRemoteButton executes a remote control action
func (m RemoteModel) handleRemoteButton(button remoteButton) (RemoteModel, tea.Cmd) {
	if m.device == nil {
		return m, nil
	}

	// Map button to device action
	var actionType device.ActionType = device.ActionTypeRemote
	var actionName string

	switch button {
	case buttonPower:
		actionName = "power"
	case buttonVolumeUp:
		actionName = "volume_up"
	case buttonVolumeDown:
		actionName = "volume_down"
	case buttonMute:
		actionName = "mute"
	case buttonChannelUp:
		actionName = "channel_up"
	case buttonChannelDown:
		actionName = "channel_down"
	case buttonUp:
		actionName = "up"
	case buttonDown:
		actionName = "down"
	case buttonLeft:
		actionName = "left"
	case buttonRight:
		actionName = "right"
	case buttonOK:
		actionName = "confirm"
	case buttonHome:
		actionName = "home"
	case buttonMenu:
		actionName = "menu"
	case buttonBack:
		actionName = "back"
	case buttonInput:
		actionName = "input"
	case buttonHDMI1:
		actionName = "hdmi1"
	case buttonHDMI2:
		actionName = "hdmi2"
	case buttonHDMI3:
		actionName = "hdmi3"
	case buttonHDMI4:
		actionName = "hdmi4"
	case buttonNum0:
		actionName = "num_0"
	case buttonNum1:
		actionName = "num_1"
	case buttonNum2:
		actionName = "num_2"
	case buttonNum3:
		actionName = "num_3"
	case buttonNum4:
		actionName = "num_4"
	case buttonNum5:
		actionName = "num_5"
	case buttonNum6:
		actionName = "num_6"
	case buttonNum7:
		actionName = "num_7"
	case buttonNum8:
		actionName = "num_8"
	case buttonNum9:
		actionName = "num_9"
	default:
		return m, nil
	}

	// Execute the action
	request := device.ActionRequest{
		Type:   actionType,
		Action: actionName,
	}

	actionJSON, err := json.Marshal(request)
	if err != nil {
		m.lastResponse = &device.ActionResponse{
			Success: false,
			Error:   err.Error(),
		}
		return m, nil
	}

	response, err := m.device.Process(actionJSON)
	if err != nil {
		response = &device.ActionResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	m.lastResponse = response
	m.selectedButton = button
	m.lastButtonPress = time.Now()

	// Add to history
	entry := actionHistoryEntry{
		Timestamp: time.Now(),
		Action:    string(actionJSON),
		Success:   response.Success,
	}

	if response.Success {
		if data, err := json.MarshalIndent(response.Data, "", "  "); err == nil {
			entry.Response = string(data)
		} else {
			entry.Response = fmt.Sprintf("%v", response.Data)
		}
	} else {
		entry.Error = response.Error
	}

	m.actionHistory = append([]actionHistoryEntry{entry}, m.actionHistory...)
	if len(m.actionHistory) > 50 {
		m.actionHistory = m.actionHistory[:50]
	}

	log := logger.New()
	log.Info().
		Str("action", string(actionJSON)).
		Bool("success", response.Success).
		Msg("Remote button pressed")

	return m, nil
}

// handleNumberKey handles number key presses
func (m RemoteModel) handleNumberKey(key string) (RemoteModel, tea.Cmd) {
	var button remoteButton
	switch key {
	case "0":
		button = buttonNum0
	case "1":
		button = buttonNum1
	case "2":
		button = buttonNum2
	case "3":
		button = buttonNum3
	case "4":
		button = buttonNum4
	case "5":
		button = buttonNum5
	case "6":
		button = buttonNum6
	case "7":
		button = buttonNum7
	case "8":
		button = buttonNum8
	case "9":
		button = buttonNum9
	default:
		return m, nil
	}

	return m.handleRemoteButton(button)
}

// GetActionHistory returns the action history
func (m RemoteModel) GetActionHistory() []actionHistoryEntry {
	return m.actionHistory
}

// GetLastResponse returns the last response
func (m RemoteModel) GetLastResponse() *device.ActionResponse {
	return m.lastResponse
}