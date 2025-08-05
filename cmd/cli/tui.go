package cli

import (
	"encoding/json"
	"fmt"
	"lucas/internal/device"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lucas/internal/bravia"
	"lucas/internal/logger"
)

var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Bold(true)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Width(50)

	inputFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF79C6")).
		Padding(0, 1).
		Width(50)

	buttonStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FAFAFA")).
		Padding(0, 2).
		Margin(0, 1)

	buttonActiveStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#FF79C6")).
		Foreground(lipgloss.Color("#FAFAFA")).
		Padding(0, 2).
		Margin(0, 1)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	codeStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#282A36")).
		Foreground(lipgloss.Color("#F8F8F2")).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#44475A"))
)

type screen int

const (
	screenDeviceSetup screen = iota
	screenConnected
	screenActionInput
	screenActionHistory
)

type inputField int

const (
	fieldDeviceType inputField = iota
	fieldHostAddress
	fieldCredential
	fieldConnect
	fieldActionType
	fieldAction
	fieldParameters
	fieldExecute
)

type remoteButton int

const (
	buttonPower remoteButton = iota
	buttonVolumeUp
	buttonVolumeDown
	buttonMute
	buttonChannelUp
	buttonChannelDown
	buttonUp
	buttonDown
	buttonLeft
	buttonRight
	buttonOK
	buttonHome
	buttonMenu
	buttonBack
	buttonInput
	buttonNum0
	buttonNum1
	buttonNum2
	buttonNum3
	buttonNum4
	buttonNum5
	buttonNum6
	buttonNum7
	buttonNum8
	buttonNum9
	buttonHDMI1
	buttonHDMI2
	buttonHDMI3
	buttonHDMI4
)

type actionHistoryEntry struct {
	Timestamp time.Time
	Action    string
	Success   bool
	Response  string
	Error     string
}

type model struct {
	screen       screen
	focusedField inputField

	// Device setup fields
	deviceTypes     []string
	selectedDevice  int
	hostAddress     string
	credential      string
	connecting      bool
	connectionError string

	// Connected device
	device     device.Device
	deviceInfo device.DeviceInfo

	// Action input fields
	actionTypes        []string
	selectedActionType int
	availableActions   []string
	selectedAction     int
	actionParameters   string
	actionInput        string
	executing          bool

	// Response and history
	lastResponse    *device.ActionResponse
	actionHistory   []actionHistoryEntry
	historySelected int

	// Input cursor positions
	hostAddressCursor int
	credentialCursor  int
	parametersCursor  int

	// Remote control state
	selectedButton  remoteButton
	lastButtonPress time.Time

	// UI state
	width    int
	height   int
	quitting bool
}

func initialModel() model {
	return model{
		screen:             screenDeviceSetup,
		focusedField:       fieldDeviceType,
		deviceTypes:        []string{"Sony Bravia TV"},
		selectedDevice:     0,
		hostAddress:        "",
		credential:         "",
		actionTypes:        []string{"remote", "control"},
		selectedActionType: 0,
		availableActions:   []string{},
		actionHistory:      []actionHistoryEntry{},
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
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "q":
			if m.screen == screenDeviceSetup {
				m.quitting = true
				return m, tea.Quit
			}
			// In other screens, 'q' goes back to device setup
			m.screen = screenDeviceSetup
			m.device = nil
			m.focusedField = fieldDeviceType
			return m, nil

		case "tab", "shift+tab":
			return m.handleTabNavigation(msg.String() == "shift+tab"), nil

		case "enter":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonOK)
			}
			return m.handleEnter()

		case "up", "k":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonUp)
			}
			return m.handleUp(), nil

		case "down", "j":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonDown)
			}
			return m.handleDown(), nil

		case "left", "h":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonLeft)
			}
			return m.handleLeft(), nil

		case "right", "l":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonRight)
			}
			return m.handleRight(), nil

		case "backspace":
			return m.handleBackspace(), nil

		case "delete":
			return m.handleDelete(), nil

		case "home":
			return m.handleHome(), nil

		case "end":
			return m.handleEnd(), nil

		case "ctrl+a":
			return m.handleSelectAll(), nil

		case "ctrl+v":
			return m.handlePaste(), nil

		// Remote control specific keys
		case "p":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonPower)
			}
		case "+", "=":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonVolumeUp)
			}
		case "-":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonVolumeDown)
			}
		case "m":
			if m.screen == screenConnected {
				return m.handleRemoteButton(buttonMute)
			}
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.screen == screenConnected {
				return m.handleNumberKey(msg.String())
			}

		default:
			return m.handleTextInput(msg.String()), nil
		}
	}

	return m, nil
}

func (m model) handleTabNavigation(reverse bool) model {
	fields := m.getAvailableFields()
	if len(fields) == 0 {
		return m
	}

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
	(&m).syncCursorPosition()
	return m
}

func (m model) getAvailableFields() []inputField {
	switch m.screen {
	case screenDeviceSetup:
		return []inputField{fieldDeviceType, fieldHostAddress, fieldCredential, fieldConnect}
	case screenActionInput:
		return []inputField{fieldActionType, fieldAction, fieldParameters, fieldExecute}
	case screenActionHistory:
		return []inputField{fieldExecute}
	default:
		return []inputField{}
	}
}

func (m model) handleEnter() (model, tea.Cmd) {
	switch m.screen {
	case screenDeviceSetup:
		if m.focusedField == fieldConnect {
			return m.handleConnect()
		}
	case screenActionInput:
		if m.focusedField == fieldExecute {
			return m.handleExecuteAction()
		}
	}
	return m, nil
}

func (m model) handleConnect() (model, tea.Cmd) {
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
	if !m.isValidHostAddress(m.hostAddress) {
		m.connectionError = "Invalid host address format (expected IP:port)"
		return m, nil
	}

	m.connecting = true
	m.connectionError = ""

	// Create device connection
	device := bravia.NewBraviaRemote(m.hostAddress, m.credential, false) // TODO: Add debug flag support

	// Test connection by getting device info
	deviceInfo := device.GetDeviceInfo()

	// Connection successful
	m.device = device
	m.deviceInfo = deviceInfo
	m.screen = screenConnected
	m.connecting = false

	log := logger.New()
	log.Info().
		Str("device_type", deviceInfo.Type).
		Str("device_model", deviceInfo.Model).
		Str("address", deviceInfo.Address).
		Msg("Device connected successfully")

	return m, nil
}

func (m model) handleExecuteAction() (model, tea.Cmd) {
	if m.executing || m.device == nil {
		return m, nil
	}

	// Build action JSON
	actionJSON, err := m.buildActionJSON()
	if err != nil {
		m.lastResponse = &device.ActionResponse{
			Success: false,
			Error:   err.Error(),
		}
		return m, nil
	}

	m.executing = true

	// Execute action
	response, err := m.device.Process(actionJSON)
	if err != nil {
		response = &device.ActionResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	m.lastResponse = response
	m.executing = false

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
		Msg("Action executed")

	return m, nil
}

func (m model) buildActionJSON() ([]byte, error) {
	actionType := m.actionTypes[m.selectedActionType]

	var action string
	if m.selectedAction < len(m.availableActions) {
		action = m.availableActions[m.selectedAction]
	} else {
		return nil, fmt.Errorf("no action selected")
	}

	request := device.ActionRequest{
		Type:   device.ActionType(actionType),
		Action: action,
	}

	// Parse parameters if provided
	if strings.TrimSpace(m.actionParameters) != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(m.actionParameters), &params); err != nil {
			return nil, fmt.Errorf("invalid parameters JSON: %w", err)
		}
		request.Parameters = params
	}

	return json.Marshal(request)
}

func (m model) handleUp() model {
	switch m.focusedField {
	case fieldDeviceType:
		if m.selectedDevice > 0 {
			m.selectedDevice--
		}
	case fieldActionType:
		if m.selectedActionType > 0 {
			m.selectedActionType--
			m.updateAvailableActions()
		}
	case fieldAction:
		if m.selectedAction > 0 {
			m.selectedAction--
		}
	}
	return m
}

func (m model) handleDown() model {
	switch m.focusedField {
	case fieldDeviceType:
		if m.selectedDevice < len(m.deviceTypes)-1 {
			m.selectedDevice++
		}
	case fieldActionType:
		if m.selectedActionType < len(m.actionTypes)-1 {
			m.selectedActionType++
			m.updateAvailableActions()
		}
	case fieldAction:
		if m.selectedAction < len(m.availableActions)-1 {
			m.selectedAction++
		}
	}
	return m
}

func (m model) handleLeft() model {
	switch m.focusedField {
	case fieldHostAddress:
		if m.hostAddressCursor > 0 {
			m.hostAddressCursor--
		}
	case fieldCredential:
		if m.credentialCursor > 0 {
			m.credentialCursor--
		}
	case fieldParameters:
		if m.parametersCursor > 0 {
			m.parametersCursor--
		}
	}
	return m
}

func (m model) handleRight() model {
	switch m.focusedField {
	case fieldHostAddress:
		if m.hostAddressCursor < len(m.hostAddress) {
			m.hostAddressCursor++
		}
	case fieldCredential:
		if m.credentialCursor < len(m.credential) {
			m.credentialCursor++
		}
	case fieldParameters:
		if m.parametersCursor < len(m.actionParameters) {
			m.parametersCursor++
		}
	}
	return m
}

func (m model) handleBackspace() model {
	switch m.focusedField {
	case fieldHostAddress:
		if m.hostAddressCursor > 0 && len(m.hostAddress) > 0 {
			m.hostAddress = m.deleteCharAt(m.hostAddress, m.hostAddressCursor-1)
			m.hostAddressCursor--
		}
	case fieldCredential:
		if m.credentialCursor > 0 && len(m.credential) > 0 {
			m.credential = m.deleteCharAt(m.credential, m.credentialCursor-1)
			m.credentialCursor--
		}
	case fieldParameters:
		if m.parametersCursor > 0 && len(m.actionParameters) > 0 {
			m.actionParameters = m.deleteCharAt(m.actionParameters, m.parametersCursor-1)
			m.parametersCursor--
		}
	}
	return m
}

func (m model) handleTextInput(input string) model {
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
	case fieldHostAddress:
		m.hostAddress = m.insertText(m.hostAddress, m.hostAddressCursor, printableInput)
		m.hostAddressCursor += len(printableInput)
	case fieldCredential:
		m.credential = m.insertText(m.credential, m.credentialCursor, printableInput)
		m.credentialCursor += len(printableInput)
	case fieldParameters:
		m.actionParameters = m.insertText(m.actionParameters, m.parametersCursor, printableInput)
		m.parametersCursor += len(printableInput)
	}
	return m
}

// insertText inserts text at the specified position in a string
func (m model) insertText(text string, pos int, insert string) string {
	if pos < 0 {
		pos = 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	return text[:pos] + insert + text[pos:]
}

// deleteCharAt deletes the character at the specified position
func (m model) deleteCharAt(text string, pos int) string {
	if pos < 0 || pos >= len(text) {
		return text
	}
	return text[:pos] + text[pos+1:]
}

// handleDelete deletes the character at the cursor position (forward delete)
func (m model) handleDelete() model {
	switch m.focusedField {
	case fieldHostAddress:
		if m.hostAddressCursor < len(m.hostAddress) {
			m.hostAddress = m.deleteCharAt(m.hostAddress, m.hostAddressCursor)
		}
	case fieldCredential:
		if m.credentialCursor < len(m.credential) {
			m.credential = m.deleteCharAt(m.credential, m.credentialCursor)
		}
	case fieldParameters:
		if m.parametersCursor < len(m.actionParameters) {
			m.actionParameters = m.deleteCharAt(m.actionParameters, m.parametersCursor)
		}
	}
	return m
}

// handleHome moves cursor to beginning of current field
func (m model) handleHome() model {
	switch m.focusedField {
	case fieldHostAddress:
		m.hostAddressCursor = 0
	case fieldCredential:
		m.credentialCursor = 0
	case fieldParameters:
		m.parametersCursor = 0
	}
	return m
}

// handleEnd moves cursor to end of current field
func (m model) handleEnd() model {
	switch m.focusedField {
	case fieldHostAddress:
		m.hostAddressCursor = len(m.hostAddress)
	case fieldCredential:
		m.credentialCursor = len(m.credential)
	case fieldParameters:
		m.parametersCursor = len(m.actionParameters)
	}
	return m
}

// handleSelectAll selects all text in current field (moves cursor to end)
func (m model) handleSelectAll() model {
	return m.handleEnd()
}

// handlePaste handles clipboard paste operation (Ctrl+V)
// Note: This is a placeholder implementation - real clipboard access would require additional dependencies
func (m model) handlePaste() model {
	// For now, we'll implement a simple paste of common example values
	// In a real implementation, this would access the system clipboard

	var pasteText string
	switch m.focusedField {
	case fieldHostAddress:
		// Provide a common example for host address
		if m.hostAddress == "" {
			pasteText = "192.168.1.100:80"
		}
	case fieldCredential:
		// Don't auto-paste credentials for security
		return m
	case fieldParameters:
		// Provide a common JSON example
		if m.actionParameters == "" {
			pasteText = `{"volume": 50}`
		}
	}

	if pasteText != "" {
		// Insert the paste text at cursor position
		switch m.focusedField {
		case fieldHostAddress:
			m.hostAddress = m.insertText(m.hostAddress, m.hostAddressCursor, pasteText)
			m.hostAddressCursor += len(pasteText)
		case fieldParameters:
			m.actionParameters = m.insertText(m.actionParameters, m.parametersCursor, pasteText)
			m.parametersCursor += len(pasteText)
		}
	}

	return m
}

// syncCursorPosition ensures cursor positions are within bounds when switching fields
func (m *model) syncCursorPosition() {
	switch m.focusedField {
	case fieldHostAddress:
		if m.hostAddressCursor < 0 {
			m.hostAddressCursor = 0
		}
		if m.hostAddressCursor > len(m.hostAddress) {
			m.hostAddressCursor = len(m.hostAddress)
		}
	case fieldCredential:
		if m.credentialCursor < 0 {
			m.credentialCursor = 0
		}
		if m.credentialCursor > len(m.credential) {
			m.credentialCursor = len(m.credential)
		}
	case fieldParameters:
		if m.parametersCursor < 0 {
			m.parametersCursor = 0
		}
		if m.parametersCursor > len(m.actionParameters) {
			m.parametersCursor = len(m.actionParameters)
		}
	}
}

// renderTextWithCursor renders text with a cursor indicator at the specified position
func (m model) renderTextWithCursor(text string, cursorPos int, showCursor bool) string {
	if !showCursor || cursorPos < 0 {
		return text
	}

	if cursorPos > len(text) {
		cursorPos = len(text)
	}

	if cursorPos == len(text) {
		// Cursor at end - add a visible cursor character
		return text + "│"
	} else {
		// Cursor in middle - highlight the character at cursor position
		before := text[:cursorPos]
		atCursor := string(text[cursorPos])
		after := text[cursorPos+1:]

		// Highlight the character under cursor
		highlightedChar := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF79C6")).
			Foreground(lipgloss.Color("#FAFAFA")).
			Render(atCursor)

		return before + highlightedChar + after
	}
}

func (m model) updateAvailableActions() {
	actionType := m.actionTypes[m.selectedActionType]

	if actionType == "remote" {
		m.availableActions = bravia.AvailableRemoteActions
	} else if actionType == "control" {
		m.availableActions = bravia.AvailableControlActions
	}

	if m.selectedAction >= len(m.availableActions) {
		m.selectedAction = 0
	}
}

func (m model) isValidHostAddress(address string) bool {
	// Check for IP:port format
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}

	// Validate IP
	if net.ParseIP(host) == nil {
		// Try as hostname
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+$`, host)
		if !matched {
			return false
		}
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return false
	}

	return true
}

func (m model) View() string {
	if m.quitting {
		return successStyle.Render("Thanks for using Lucas CLI!") + "\n"
	}

	switch m.screen {
	case screenDeviceSetup:
		return m.renderDeviceSetup()
	case screenConnected:
		return m.renderConnected()
	case screenActionInput:
		return m.renderActionInput()
	case screenActionHistory:
		return m.renderActionHistory()
	default:
		return "Unknown screen"
	}
}

func (m model) renderDeviceSetup() string {
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
		if m.focusedField == fieldDeviceType && i == m.selectedDevice {
			style = style.Foreground(lipgloss.Color("#FF79C6"))
		}

		b.WriteString(style.Render(cursor + deviceType))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Host Address Input
	b.WriteString(subtitleStyle.Render("Host Address (IP:Port):"))
	b.WriteString("\n")
	hostStyle := inputStyle
	showCursor := m.focusedField == fieldHostAddress
	if showCursor {
		hostStyle = inputFocusedStyle
	}
	hostText := m.renderTextWithCursor(m.hostAddress, m.hostAddressCursor, showCursor)
	b.WriteString(hostStyle.Render(hostText))
	b.WriteString("\n\n")

	// Credential Input
	b.WriteString(subtitleStyle.Render("Credential (PSK):"))
	b.WriteString("\n")
	credStyle := inputStyle
	showCredCursor := m.focusedField == fieldCredential
	if showCredCursor {
		credStyle = inputFocusedStyle
	}
	maskedCredential := strings.Repeat("*", len(m.credential))
	credText := m.renderTextWithCursor(maskedCredential, m.credentialCursor, showCredCursor)
	b.WriteString(credStyle.Render(credText))
	b.WriteString("\n\n")

	// Connect Button
	connectStyle := buttonStyle
	if m.focusedField == fieldConnect {
		connectStyle = buttonActiveStyle
	}

	connectText := "Connect"
	if m.connecting {
		connectText = "Connecting..."
	}
	b.WriteString(connectStyle.Render(connectText))
	b.WriteString("\n\n")

	// Connection Error
	if m.connectionError != "" {
		b.WriteString(errorStyle.Render("Error: " + m.connectionError))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Tab: Next field • Enter: Connect • ←/→: Move cursor • Home/End: Start/End • Ctrl+V: Paste • Ctrl+C: Quit"))

	return b.String()
}

func (m model) renderConnected() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Lucas CLI - TV Remote Control"))
	b.WriteString("\n\n")

	// Device Info
	b.WriteString(successStyle.Render("📺 " + m.deviceInfo.Model + " Connected"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Address: %s", m.deviceInfo.Address))
	b.WriteString("\n\n")

	// Remote Control Layout
	b.WriteString(m.renderRemoteControl())

	// Status and Help
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
	b.WriteString(helpStyle.Render("Arrow keys: Navigate • Enter: OK • P: Power • +/-: Volume • M: Mute • 0-9: Numbers • q: Disconnect"))

	return b.String()
}

func (m model) renderRemoteControl() string {
	var b strings.Builder

	// Create button styles
	getButtonStyle := func(btn remoteButton) lipgloss.Style {
		base := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Margin(0, 1)

		if m.selectedButton == btn && time.Since(m.lastButtonPress) < 200*time.Millisecond {
			// Highlight pressed button briefly
			return base.Background(lipgloss.Color("#FF79C6")).
				Foreground(lipgloss.Color("#FAFAFA"))
		}

		return base.Background(lipgloss.Color("#44475A")).
			Foreground(lipgloss.Color("#F8F8F2"))
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
func (m model) handleRemoteButton(button remoteButton) (model, tea.Cmd) {
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

// handleNumberKey executes a number button press
func (m model) handleNumberKey(key string) (model, tea.Cmd) {
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

func (m model) renderActionInput() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Lucas CLI - Send Action"))
	b.WriteString("\n\n")

	// Action Type Selection
	b.WriteString(subtitleStyle.Render("Action Type:"))
	b.WriteString("\n")
	for i, actionType := range m.actionTypes {
		cursor := "  "
		if i == m.selectedActionType {
			cursor = "> "
		}

		style := lipgloss.NewStyle()
		if m.focusedField == fieldActionType && i == m.selectedActionType {
			style = style.Foreground(lipgloss.Color("#FF79C6"))
		}

		b.WriteString(style.Render(cursor + actionType))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Action Selection
	if len(m.availableActions) == 0 {
		m.updateAvailableActions()
	}

	b.WriteString(subtitleStyle.Render("Action:"))
	b.WriteString("\n")
	for i, action := range m.availableActions {
		if i > 10 { // Limit display
			b.WriteString("  ... and more")
			break
		}

		cursor := "  "
		if i == m.selectedAction {
			cursor = "> "
		}

		style := lipgloss.NewStyle()
		if m.focusedField == fieldAction && i == m.selectedAction {
			style = style.Foreground(lipgloss.Color("#FF79C6"))
		}

		b.WriteString(style.Render(cursor + action))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Parameters Input
	b.WriteString(subtitleStyle.Render("Parameters (JSON):"))
	b.WriteString("\n")
	paramStyle := inputStyle
	showParamCursor := m.focusedField == fieldParameters
	if showParamCursor {
		paramStyle = inputFocusedStyle
	}
	paramText := m.renderTextWithCursor(m.actionParameters, m.parametersCursor, showParamCursor)
	b.WriteString(paramStyle.Render(paramText))
	b.WriteString("\n\n")

	// Execute Button
	executeStyle := buttonStyle
	if m.focusedField == fieldExecute {
		executeStyle = buttonActiveStyle
	}

	executeText := "Execute Action"
	if m.executing {
		executeText = "Executing..."
	}
	b.WriteString(executeStyle.Render(executeText))
	b.WriteString("\n\n")

	// Last Response
	if m.lastResponse != nil {
		b.WriteString(subtitleStyle.Render("Last Response:"))
		b.WriteString("\n")

		if m.lastResponse.Success {
			b.WriteString(successStyle.Render("✓ Success"))
			b.WriteString("\n")
			if m.lastResponse.Data != nil {
				data, _ := json.MarshalIndent(m.lastResponse.Data, "", "  ")
				b.WriteString(codeStyle.Render(string(data)))
			}
		} else {
			b.WriteString(errorStyle.Render("✗ Error: " + m.lastResponse.Error))
		}
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("↑/↓: Navigate • Tab: Next field • Enter: Execute • ←/→: Move cursor • Home/End: Start/End • Ctrl+V: Paste • q: Back • Ctrl+C: Quit"))

	return b.String()
}

func (m model) renderActionHistory() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Lucas CLI - Action History"))
	b.WriteString("\n\n")

	if len(m.actionHistory) == 0 {
		b.WriteString("No actions executed yet.")
		b.WriteString("\n\n")
	} else {
		for i, entry := range m.actionHistory {
			if i >= 10 { // Limit display
				break
			}

			timestamp := entry.Timestamp.Format("15:04:05")
			status := "✓"
			statusStyle := successStyle
			if !entry.Success {
				status = "✗"
				statusStyle = errorStyle
			}

			b.WriteString(fmt.Sprintf("%s %s %s",
				timestamp,
				statusStyle.Render(status),
				entry.Action))
			b.WriteString("\n")

			if entry.Success && entry.Response != "" {
				b.WriteString("  Response: " + entry.Response[:min(100, len(entry.Response))])
				if len(entry.Response) > 100 {
					b.WriteString("...")
				}
			} else if !entry.Success && entry.Error != "" {
				b.WriteString("  Error: " + entry.Error)
			}
			b.WriteString("\n\n")
		}
	}

	// Help
	b.WriteString(helpStyle.Render("q: Back • Ctrl+C: Quit"))

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func StartTUI() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
