package cli

import (
	"lucas/internal"
	"net"
	"regexp"
	"strconv"
	"strings"

	"lucas/internal/bravia"
	"lucas/internal/device"
	"lucas/internal/logger"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Setup screen input fields
type setupField int

const (
	setupFieldDeviceType setupField = iota
	setupFieldHostAddress
	setupFieldCredential
	setupFieldConnect
	setupFieldConfigureDevices
)

// SetupModel handles the device setup screen
type SetupModel struct {
	// Navigation
	focusedField setupField

	// Device selection
	deviceTypes    []string
	selectedDevice int

	// Input fields
	hostAddress string
	credential  string

	// Cursor positions
	hostAddressCursor int
	credentialCursor  int

	// Connection state
	connecting      bool
	connectionError string

	// Connected device (when setup complete)
	device     device.Device
	deviceInfo device.DeviceInfo

	// Flags
	debugMode bool
	testMode  bool

	// Configuration
	configPath string
}

// NewSetupModel creates a new setup screen model
func NewSetupModel() SetupModel {
	return NewSetupModelWithFlags(false, false)
}

// NewSetupModelWithFlags creates a new setup screen model with flags
func NewSetupModelWithFlags(debug, test bool) SetupModel {
	return SetupModel{
		focusedField:   setupFieldDeviceType,
		deviceTypes:    []string{"Sony Bravia TV"},
		selectedDevice: 0,
		hostAddress:    "",
		credential:     "",
		debugMode:      debug,
		testMode:       test,
		configPath:     "hub.yml", // Default config path
	}
}

// NewSetupModelWithConfig creates a new setup screen model with config path
func NewSetupModelWithConfig(debug, test bool, configPath string) SetupModel {
	model := NewSetupModelWithFlags(debug, test)
	model.configPath = configPath
	return model
}

// Update handles setup screen messages
func (m SetupModel) Update(msg tea.Msg) (SetupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			return m.handleTabNavigation(msg.String() == "shift+tab"), nil

		case "enter":
			if m.focusedField == setupFieldConnect {
				return m.handleConnect()
			}
			if m.focusedField == setupFieldConfigureDevices {
				return m.handleConfigureDevices()
			}
			return m, nil

		case "up":
			return m.handleUp(), nil

		case "down":
			return m.handleDown(), nil

		case "left":
			return m.handleLeft(), nil

		case "right":
			return m.handleRight(), nil

		case "backspace":
			return m.handleBackspace(), nil

		case "delete":
			return m.handleDelete(), nil

		case "home":
			return m.handleHome(), nil

		case "end":
			return m.handleEnd(), nil

		case "ctrl+v":
			return m.handlePaste(), nil

		default:
			return m.handleTextInput(msg.String()), nil
		}
	}

	return m, nil
}

// View renders the setup screen
func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Lucas CLI - Device Setup"))
	b.WriteString("\n\n")

	// Device Type Selection
	b.WriteString(subtitleStyle.Render("Device Type:"))
	b.WriteString("\n")
	for i, deviceType := range m.deviceTypes {
		cursor := "  "
		if i == m.selectedDevice {
			cursor = "> "
		}

		style := lipgloss.NewStyle()
		if m.focusedField == setupFieldDeviceType && i == m.selectedDevice {
			style = style.Foreground(lipgloss.Color("#FF79C6"))
		}

		b.WriteString(style.Render(cursor + deviceType))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Host Address Input
	b.WriteString(subtitleStyle.Render("Host Address (IP or IP:Port):"))
	b.WriteString("\n")
	hostStyle := inputStyle
	showCursor := m.focusedField == setupFieldHostAddress
	if showCursor {
		hostStyle = inputFocusedStyle
	}
	hostText := renderTextWithCursor(m.hostAddress, m.hostAddressCursor, showCursor)
	b.WriteString(hostStyle.Render(hostText))
	b.WriteString("\n\n")

	// Credential Input
	b.WriteString(subtitleStyle.Render("Credential (PSK):"))
	b.WriteString("\n")
	credStyle := inputStyle
	showCredCursor := m.focusedField == setupFieldCredential
	if showCredCursor {
		credStyle = inputFocusedStyle
	}
	//maskedCredential := strings.Repeat("*", len(m.credential))
	credText := renderTextWithCursor(m.credential, m.credentialCursor, showCredCursor)
	b.WriteString(credStyle.Render(credText))
	b.WriteString("\n\n")

	// Connect Button
	connectStyle := buttonStyle
	if m.focusedField == setupFieldConnect {
		connectStyle = buttonActiveStyle
	}

	connectText := "Connect"
	if m.connecting {
		connectText = "Connecting..."
	}
	b.WriteString(connectStyle.Render(connectText))
	b.WriteString("\n\n")

	// Configure Devices Button
	configStyle := buttonStyle
	if m.focusedField == setupFieldConfigureDevices {
		configStyle = buttonActiveStyle
	}
	b.WriteString(configStyle.Render("Configure Devices"))
	b.WriteString("\n\n")

	// Connection Error
	if m.connectionError != "" {
		b.WriteString(errorStyle.Render("Error: " + m.connectionError))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Tab: Next field • Enter: Action • ←/→: Move cursor • Home/End: Start/End • Ctrl+V: Paste • q: Quit"))

	return b.String()
}

// handleTabNavigation moves between input fields
func (m SetupModel) handleTabNavigation(reverse bool) SetupModel {
	fields := []setupField{setupFieldDeviceType, setupFieldHostAddress, setupFieldCredential, setupFieldConnect, setupFieldConfigureDevices}

	currentIndex := -1
	for i, field := range fields {
		if field == m.focusedField {
			currentIndex = i
			break
		}
	}

	if reverse {
		currentIndex--
		if currentIndex < 0 {
			currentIndex = len(fields) - 1
		}
	} else {
		currentIndex++
		if currentIndex >= len(fields) {
			currentIndex = 0
		}
	}

	m.focusedField = fields[currentIndex]
	m.syncCursorPosition()
	return m
}

// handleConnect attempts to connect to the device
func (m SetupModel) handleConnect() (SetupModel, tea.Cmd) {
	if m.connecting {
		return m, nil
	}

	// Validate inputs
	if m.hostAddress == "" {
		m.connectionError = "Host address is required"
		return m, nil
	}

	if m.credential == "" {
		m.connectionError = "Credential is required"
		return m, nil
	}

	// Validate host address format
	if !m.IsValidHostAddress(m.hostAddress) {
		m.connectionError = "Invalid host address format"
		return m, nil
	}

	m.connecting = true
	m.connectionError = ""

	// Use address as provided by user (no normalization)
	// Let the HTTP client handle default ports naturally

	// Create device connection with debug and test flags
	device := bravia.NewBraviaRemote(m.hostAddress, m.credential, internal.NewModeOptions(internal.WithDebug(m.debugMode), internal.WithTest(m.testMode)))

	// Test connection by getting device info
	deviceInfo := device.GetDeviceInfo()

	// Connection successful
	m.device = device
	m.deviceInfo = deviceInfo
	m.connecting = false

	log := logger.New()
	log.Info().
		Str("device_type", deviceInfo.Type).
		Str("device_model", deviceInfo.Model).
		Str("address", m.hostAddress).
		Msg("Device connected successfully")

	return m, nil
}

// handleUp handles up arrow key
func (m SetupModel) handleUp() SetupModel {
	if m.focusedField == setupFieldDeviceType {
		if m.selectedDevice > 0 {
			m.selectedDevice--
		}
	}
	return m
}

// handleDown handles down arrow key
func (m SetupModel) handleDown() SetupModel {
	if m.focusedField == setupFieldDeviceType {
		if m.selectedDevice < len(m.deviceTypes)-1 {
			m.selectedDevice++
		}
	}
	return m
}

// handleLeft handles left arrow key
func (m SetupModel) handleLeft() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddressCursor > 0 {
			m.hostAddressCursor--
		}
	case setupFieldCredential:
		if m.credentialCursor > 0 {
			m.credentialCursor--
		}
	}
	return m
}

// handleRight handles right arrow key
func (m SetupModel) handleRight() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddressCursor < len(m.hostAddress) {
			m.hostAddressCursor++
		}
	case setupFieldCredential:
		if m.credentialCursor < len(m.credential) {
			m.credentialCursor++
		}
	}
	return m
}

// handleBackspace handles backspace key
func (m SetupModel) handleBackspace() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddressCursor > 0 && len(m.hostAddress) > 0 {
			m.hostAddress = deleteCharAt(m.hostAddress, m.hostAddressCursor-1)
			m.hostAddressCursor--
		}
	case setupFieldCredential:
		if m.credentialCursor > 0 && len(m.credential) > 0 {
			m.credential = deleteCharAt(m.credential, m.credentialCursor-1)
			m.credentialCursor--
		}
	}
	return m
}

// handleDelete handles delete key
func (m SetupModel) handleDelete() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddressCursor < len(m.hostAddress) {
			m.hostAddress = deleteCharAt(m.hostAddress, m.hostAddressCursor)
		}
	case setupFieldCredential:
		if m.credentialCursor < len(m.credential) {
			m.credential = deleteCharAt(m.credential, m.credentialCursor)
		}
	}
	return m
}

// handleHome handles home key
func (m SetupModel) handleHome() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		m.hostAddressCursor = 0
	case setupFieldCredential:
		m.credentialCursor = 0
	}
	return m
}

// handleEnd handles end key
func (m SetupModel) handleEnd() SetupModel {
	switch m.focusedField {
	case setupFieldHostAddress:
		m.hostAddressCursor = len(m.hostAddress)
	case setupFieldCredential:
		m.credentialCursor = len(m.credential)
	}
	return m
}

// handlePaste handles paste operation
func (m SetupModel) handlePaste() SetupModel {
	var pasteText string
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddress == "" {
			pasteText = "192.168.1.100" // Simple IP without port
		}
	case setupFieldCredential:
		// Don't auto-paste credentials for security
		return m
	}

	if pasteText != "" && m.focusedField == setupFieldHostAddress {
		m.hostAddress = insertText(m.hostAddress, m.hostAddressCursor, pasteText)
		m.hostAddressCursor += len(pasteText)
	}

	return m
}

// handleTextInput handles character input
func (m SetupModel) handleTextInput(input string) SetupModel {
	// Filter out non-printable characters and control sequences
	if len(input) == 0 || input == "\x00" {
		return m
	}

	// Allow printable characters including spaces and punctuation
	printableInput := ""
	for _, r := range input {
		if r >= 32 && r < 127 || r > 127 { // ASCII printable + Unicode
			printableInput += string(r)
		}
	}

	if len(printableInput) == 0 {
		return m
	}

	switch m.focusedField {
	case setupFieldHostAddress:
		m.hostAddress = insertText(m.hostAddress, m.hostAddressCursor, printableInput)
		m.hostAddressCursor += len(printableInput)
	case setupFieldCredential:
		m.credential = insertText(m.credential, m.credentialCursor, printableInput)
		m.credentialCursor += len(printableInput)
	}
	return m
}

// syncCursorPosition ensures cursor positions are within bounds
func (m *SetupModel) syncCursorPosition() {
	switch m.focusedField {
	case setupFieldHostAddress:
		if m.hostAddressCursor < 0 {
			m.hostAddressCursor = 0
		}
		if m.hostAddressCursor > len(m.hostAddress) {
			m.hostAddressCursor = len(m.hostAddress)
		}
	case setupFieldCredential:
		if m.credentialCursor < 0 {
			m.credentialCursor = 0
		}
		if m.credentialCursor > len(m.credential) {
			m.credentialCursor = len(m.credential)
		}
	}
}

// IsValidHostAddress validates the host address format (with optional port)
func (m SetupModel) IsValidHostAddress(address string) bool {
	// Try to split host:port first
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		// If split fails, treat the whole address as host (no port specified)
		host = address
		portStr = ""
	}

	// Validate host (IP or hostname)
	if net.ParseIP(host) == nil {
		// Try as hostname
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+$`, host)
		if !matched {
			return false
		}
	}

	// Validate port if provided
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}

	return true
}

// IsConnected returns true if device is connected
func (m SetupModel) IsConnected() bool {
	return m.device != nil
}

// GetDevice returns the connected device
func (m SetupModel) GetDevice() device.Device {
	return m.device
}

// GetDeviceInfo returns the device info
func (m SetupModel) GetDeviceInfo() device.DeviceInfo {
	return m.deviceInfo
}

// GetDebugMode returns the debug mode flag
func (m SetupModel) GetDebugMode() bool {
	return m.debugMode
}

// GetTestMode returns the test mode flag
func (m SetupModel) GetTestMode() bool {
	return m.testMode
}

// handleConfigureDevices launches the device configuration screen
func (m SetupModel) handleConfigureDevices() (SetupModel, tea.Cmd) {
	// For now, we'll show a placeholder message
	// In a full implementation, the parent application would handle switching
	// to the device configuration screen using the config path: m.configPath

	log := logger.New()
	log.Info().
		Str("config_path", m.configPath).
		Msg("Device configuration requested - would launch device config screen")

	return m, nil
}
