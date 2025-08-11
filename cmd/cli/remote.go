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

// LogEntry represents a log entry for display
type LogEntry struct {
	Timestamp time.Time
	Level     string // INF, DBG, ERR
	Message   string
	Action    string // button name or action
}

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

	// Flags
	debugMode bool
	testMode  bool

	// Screen dimensions for responsive layout
	width  int
	height int

	// Log display
	logBuffer   []LogEntry
	maxLogLines int
}

// NewRemoteModelWithFlags creates a new remote control screen model with flags
func NewRemoteModelWithFlags(dev device.Device, info device.DeviceInfo, debug, test bool) RemoteModel {
	return RemoteModel{
		device:        dev,
		deviceInfo:    info,
		actionHistory: []actionHistoryEntry{},
		debugMode:     debug,
		testMode:      test,
		logBuffer:     []LogEntry{},
		maxLogLines:   6, // Show last 6 log entries
	}
}

// Update handles remote control screen messages
func (m RemoteModel) Update(msg tea.Msg) (RemoteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

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
	var sections []string

	// Header
	sections = append(sections, titleStyle.Render("Lucas CLI - TV Remote Control"))

	// Device Info (compact single line)
	deviceInfo := successStyle.Render("📺 " + m.deviceInfo.Model)
	if m.testMode {
		deviceInfo += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Render("(Test)")
	}
	sections = append(sections, deviceInfo)

	// Remote Control Layout
	sections = append(sections, m.renderHorizontalRemoteLayout())

	// Status (if recent action)
	if m.lastResponse != nil {
		sections = append(sections, m.renderStatusBar())
	}

	// Fixed 3-line Log Display (if debug or test mode)
	if m.debugMode || m.testMode {
		logDisplay := m.renderLogDisplay()
		if logDisplay != "" {
			sections = append(sections, logDisplay)
		}
	}

	// Help Text
	sections = append(sections, m.renderHelpText())

	return strings.Join(sections, "\n\n")
}

// renderHorizontalRemoteLayout creates a horizontal remote control layout
func (m RemoteModel) renderHorizontalRemoteLayout() string {
	getButtonStyle := func(btn remoteButton) lipgloss.Style {
		base := remoteButtonStyle
		if m.selectedButton == btn && time.Since(m.lastButtonPress) < 200*time.Millisecond {
			base = remoteButtonActiveStyle
		}
		return base
	}

	// Left column: Power & Navigation (all buttons 6 chars wide)
	navColumn := lipgloss.JoinVertical(lipgloss.Center,
		getButtonStyle(buttonPower).Render(" PWR  "),
		"",
		getButtonStyle(buttonUp).Render("  ↑   "),
		lipgloss.JoinHorizontal(lipgloss.Center,
			getButtonStyle(buttonLeft).Render("  ←   "),
			getButtonStyle(buttonOK).Render(" OK   "),
			getButtonStyle(buttonRight).Render("  →   ")),
		getButtonStyle(buttonDown).Render("  ↓   "),
	)

	// Middle column: Volume & Channel (all buttons 6 chars wide)
	volumeColumn := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Render("Volume & Channel:"),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonVolumeUp).Render("VOL + "),
			"  ",
			getButtonStyle(buttonChannelUp).Render("CH +  ")),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonVolumeDown).Render("VOL - "),
			"  ",
			getButtonStyle(buttonChannelDown).Render("CH -  ")),
		getButtonStyle(buttonMute).Render("MUTE  "),
	)

	// Right column: Control Functions (all buttons 6 chars wide)
	functionColumn := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C")).Render("Functions:"),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonHome).Render("HOME  "),
			" ",
			getButtonStyle(buttonMenu).Render("MENU  ")),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonBack).Render("BACK  "),
			" ",
			getButtonStyle(buttonInput).Render("INPUT ")),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Render("HDMI:"),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonHDMI1).Render("HDMI1 "),
			" ",
			getButtonStyle(buttonHDMI2).Render("HDMI2 ")),
		lipgloss.JoinHorizontal(lipgloss.Left,
			getButtonStyle(buttonHDMI3).Render("HDMI3 "),
			" ",
			getButtonStyle(buttonHDMI4).Render("HDMI4 ")),
	)

	// Add column headers
	navHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Render("Power & Navigation:")

	navColumnWithHeader := lipgloss.JoinVertical(lipgloss.Center,
		navHeader,
		navColumn,
	)

	// Join columns horizontally with spacing
	return lipgloss.JoinHorizontal(lipgloss.Top,
		navColumnWithHeader,
		strings.Repeat(" ", 6),
		volumeColumn,
		strings.Repeat(" ", 6),
		functionColumn,
	)
}

// renderStatusBar creates the status bar with last action result
func (m RemoteModel) renderStatusBar() string {
	if m.lastResponse == nil {
		return ""
	}

	var status string
	if m.lastResponse.Success {
		status = successStyle.Render("✓ Action successful")
		if m.lastResponse.Data != nil {
			status += fmt.Sprintf(": %v", m.lastResponse.Data)
		}
	} else {
		status = errorStyle.Render("✗ " + m.lastResponse.Error)
	}

	return status
}

// renderLogDisplay creates a simple 3-line log display area
func (m RemoteModel) renderLogDisplay() string {
	if len(m.logBuffer) == 0 {
		return ""
	}

	// Always show exactly 3 lines
	maxLines := 3

	// Get last 3 log entries
	start := 0
	if len(m.logBuffer) > maxLines {
		start = len(m.logBuffer) - maxLines
	}

	var logLines []string

	// Log header with auto-scroll indicator
	hasMoreLogs := len(m.logBuffer) > maxLines
	autoScrollIcon := ""
	if hasMoreLogs {
		autoScrollIcon = " ↓" // Shows auto-scroll is active
	}

	header := fmt.Sprintf("─── LOGS%s ───", autoScrollIcon)
	logLines = append(logLines, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render(header))

	// Show exactly 3 log lines (pad with empty if needed)
	for i := 0; i < maxLines; i++ {
		if start+i < len(m.logBuffer) {
			entry := m.logBuffer[start+i]
			timestamp := entry.Timestamp.Format("15:04:05")

			var levelStyle lipgloss.Style
			switch entry.Level {
			case "ERR":
				levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
			case "DBG":
				levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
			default: // INF
				levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))
			}

			logLine := fmt.Sprintf("%s [%s] %s",
				timestamp,
				levelStyle.Render(entry.Level),
				entry.Message)

			// Truncate long lines to fit
			if len(logLine) > 70 {
				logLine = logLine[:67] + "..."
			}

			logLines = append(logLines, logLine)
		} else {
			// Empty line to maintain 3-line height
			logLines = append(logLines, "")
		}
	}

	return strings.Join(logLines, "\n")
}

// addLogEntry adds a new log entry to the buffer
func (m *RemoteModel) addLogEntry(level, message, action string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Action:    action,
	}

	m.logBuffer = append(m.logBuffer, entry)

	// Keep only the most recent entries
	if len(m.logBuffer) > 20 { // Keep more in buffer than we display
		m.logBuffer = m.logBuffer[1:]
	}
}

// renderHelpText creates the help text at the bottom
func (m RemoteModel) renderHelpText() string {
	help := "Arrows: Navigate • Enter: OK • P: Power • +/-: Volume • M: Mute • 0-9: Numbers"
	if m.width > 100 {
		help += " • H: Home • I: Input • F1-F4: HDMI • q: Disconnect"
	} else {
		help += " • q: Disconnect"
	}

	return "\n" + helpStyle.Render(help)
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

	// Add log entry for display (if debug or test mode)
	if m.debugMode || m.testMode {
		var logLevel string
		var logMessage string

		if response.Success {
			logLevel = "INF"
			if m.testMode {
				logMessage = fmt.Sprintf("Test mode: %s action simulated", actionName)
			} else {
				logMessage = fmt.Sprintf("%s action completed successfully", actionName)
			}
		} else {
			logLevel = "ERR"
			logMessage = fmt.Sprintf("%s failed: %s", actionName, response.Error)
		}

		m.addLogEntry(logLevel, logMessage, actionName)
	}

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
