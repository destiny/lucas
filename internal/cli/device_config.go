package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"lucas/internal/hub"
)

// deviceConfigField represents the different fields in device configuration
type deviceConfigField int

const (
	deviceConfigFieldID deviceConfigField = iota
	deviceConfigFieldType
	deviceConfigFieldModel
	deviceConfigFieldAddress
	deviceConfigFieldCredential
	deviceConfigFieldCapabilities
	deviceConfigFieldSave
	deviceConfigFieldCancel
	deviceConfigFieldTest
)

// DeviceConfigModel handles the device configuration screen
type DeviceConfigModel struct {
	configManager  *ConfigManager
	devices        []hub.DeviceConfig
	selectedDevice int
	editingDevice  *hub.DeviceConfig
	focusedField   deviceConfigField
	editMode       bool
	addMode        bool
	testMode       bool
	errorMessage   string
	successMessage string
	width          int
	height         int

	// Input field cursors and states
	idCursor           int
	typeCursor         int
	modelCursor        int
	addressCursor      int
	credentialCursor   int
	capabilitiesCursor int

	// Available device types
	deviceTypes []string
	typeIndex   int
}

// NewDeviceConfigModel creates a new device configuration model
func NewDeviceConfigModel(configPath string) DeviceConfigModel {
	configManager := NewConfigManager(configPath)

	model := DeviceConfigModel{
		configManager: configManager,
		deviceTypes:   configManager.GetSupportedDeviceTypes(),
		devices:       []hub.DeviceConfig{},
	}

	// Load devices
	model.loadDevices()

	return model
}

// loadDevices loads devices from configuration
func (m *DeviceConfigModel) loadDevices() {
	devices, err := m.configManager.ListDevices()
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to load devices: %v", err)
		return
	}
	m.devices = devices
}

// Init initializes the device config model
func (m DeviceConfigModel) Init() tea.Cmd {
	return nil
}

// Update handles device configuration screen messages
func (m DeviceConfigModel) Update(msg tea.Msg) (DeviceConfigModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Clear messages on any key press
		m.errorMessage = ""
		m.successMessage = ""

		switch msg.String() {
		case "ctrl+c", "q":
			if m.editMode || m.addMode {
				return m.exitEditMode(), nil
			}
			// Return to main menu (this would be handled by parent)
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "tab", "shift+tab":
			return m.handleTabNavigation(msg.String() == "shift+tab")

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

		case "a":
			if !m.editMode && !m.addMode {
				return m.startAddMode()
			}
			return m.handleTextInput("a")

		case "e":
			if !m.editMode && !m.addMode && len(m.devices) > 0 {
				return m.startEditMode()
			}
			return m.handleTextInput("e")

		case "d":
			if !m.editMode && !m.addMode && len(m.devices) > 0 {
				return m.deleteDevice()
			}
			return m.handleTextInput("d")

		case "t":
			if !m.editMode && !m.addMode && len(m.devices) > 0 {
				return m.testDevice()
			}
			return m.handleTextInput("t")

		case "r":
			if !m.editMode && !m.addMode {
				m.loadDevices()
				m.successMessage = "Devices reloaded"
				return m, nil
			}
			return m.handleTextInput("r")

		default:
			if m.editMode || m.addMode {
				return m.handleTextInput(msg.String())
			}
		}
	}

	return m, nil
}

// View renders the device configuration screen
func (m DeviceConfigModel) View() string {
	if m.editMode || m.addMode {
		return m.renderEditView()
	}
	return m.renderListView()
}

// renderListView renders the device list view
func (m DeviceConfigModel) renderListView() string {
	var sections []string

	// Header
	sections = append(sections, titleStyle.Render("Device Configuration Manager"))

	// Device list
	if len(m.devices) == 0 {
		sections = append(sections,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render("No devices configured"))
	} else {
		sections = append(sections, subtitleStyle.Render("Configured Devices:"))

		for i, device := range m.devices {
			cursor := "  "
			if i == m.selectedDevice {
				cursor = "> "
			}

			style := lipgloss.NewStyle()
			if i == m.selectedDevice {
				style = style.Foreground(lipgloss.Color("#FF79C6")).Bold(true)
			}

			deviceInfo := fmt.Sprintf("%s%s (%s) - %s", cursor, device.ID, device.Type, device.Address)
			sections = append(sections, style.Render(deviceInfo))
		}
	}

	// Status messages
	if m.errorMessage != "" {
		sections = append(sections, errorStyle.Render("Error: "+m.errorMessage))
	}
	if m.successMessage != "" {
		sections = append(sections, successStyle.Render("✓ "+m.successMessage))
	}

	// Help text
	var helpItems []string
	if len(m.devices) > 0 {
		helpItems = []string{
			"↑/↓: Navigate",
			"a: Add device",
			"e: Edit device",
			"d: Delete device",
			"t: Test device",
			"r: Reload",
			"q: Back",
		}
	} else {
		helpItems = []string{
			"a: Add device",
			"r: Reload",
			"q: Back",
		}
	}

	sections = append(sections, "")
	sections = append(sections, helpStyle.Render(strings.Join(helpItems, " • ")))

	return strings.Join(sections, "\n")
}

// renderEditView renders the device edit/add view
func (m DeviceConfigModel) renderEditView() string {
	var sections []string

	// Header
	title := "Add Device"
	if m.editMode {
		title = "Edit Device"
	}
	sections = append(sections, titleStyle.Render(title))

	if m.editingDevice == nil {
		sections = append(sections, errorStyle.Render("No device being edited"))
		return strings.Join(sections, "\n")
	}

	// Device ID field
	sections = append(sections, subtitleStyle.Render("Device ID:"))
	idStyle := inputStyle
	showIDCursor := m.focusedField == deviceConfigFieldID
	if showIDCursor {
		idStyle = inputFocusedStyle
	}
	idText := renderTextWithCursor(m.editingDevice.ID, m.idCursor, showIDCursor)
	sections = append(sections, idStyle.Render(idText))

	// Device Type field
	sections = append(sections, subtitleStyle.Render("Device Type:"))
	if m.focusedField == deviceConfigFieldType {
		// Show dropdown-like selection for types
		for i, deviceType := range m.deviceTypes {
			cursor := "  "
			if deviceType == m.editingDevice.Type {
				cursor = "> "
			}
			style := lipgloss.NewStyle()
			if i == m.typeIndex {
				style = style.Foreground(lipgloss.Color("#FF79C6"))
			}
			sections = append(sections, style.Render(cursor+deviceType))
		}
	} else {
		typeStyle := inputStyle
		sections = append(sections, typeStyle.Render(m.editingDevice.Type))
	}

	// Model field
	sections = append(sections, subtitleStyle.Render("Model:"))
	modelStyle := inputStyle
	showModelCursor := m.focusedField == deviceConfigFieldModel
	if showModelCursor {
		modelStyle = inputFocusedStyle
	}
	modelText := renderTextWithCursor(m.editingDevice.Model, m.modelCursor, showModelCursor)
	sections = append(sections, modelStyle.Render(modelText))

	// Address field
	sections = append(sections, subtitleStyle.Render("Address:"))
	addressStyle := inputStyle
	showAddressCursor := m.focusedField == deviceConfigFieldAddress
	if showAddressCursor {
		addressStyle = inputFocusedStyle
	}
	addressText := renderTextWithCursor(m.editingDevice.Address, m.addressCursor, showAddressCursor)
	sections = append(sections, addressStyle.Render(addressText))

	// Credential field
	sections = append(sections, subtitleStyle.Render("Credential:"))
	credStyle := inputStyle
	showCredCursor := m.focusedField == deviceConfigFieldCredential
	if showCredCursor {
		credStyle = inputFocusedStyle
	}
	credText := renderTextWithCursor(m.editingDevice.Credential, m.credentialCursor, showCredCursor)
	sections = append(sections, credStyle.Render(credText))

	// Capabilities field
	sections = append(sections, subtitleStyle.Render("Capabilities:"))
	capStyle := inputStyle
	showCapCursor := m.focusedField == deviceConfigFieldCapabilities
	if showCapCursor {
		capStyle = inputFocusedStyle
	}
	capText := strings.Join(m.editingDevice.Capabilities, ", ")
	capTextWithCursor := renderTextWithCursor(capText, m.capabilitiesCursor, showCapCursor)
	sections = append(sections, capStyle.Render(capTextWithCursor))

	// Action buttons
	sections = append(sections, "")

	saveStyle := buttonStyle
	if m.focusedField == deviceConfigFieldSave {
		saveStyle = buttonActiveStyle
	}
	sections = append(sections, saveStyle.Render("Save"))

	testStyle := buttonStyle
	if m.focusedField == deviceConfigFieldTest {
		testStyle = buttonActiveStyle
	}
	sections = append(sections, testStyle.Render("Test Connection"))

	cancelStyle := buttonStyle
	if m.focusedField == deviceConfigFieldCancel {
		cancelStyle = buttonActiveStyle
	}
	sections = append(sections, cancelStyle.Render("Cancel"))

	// Status messages
	if m.errorMessage != "" {
		sections = append(sections, errorStyle.Render("Error: "+m.errorMessage))
	}
	if m.successMessage != "" {
		sections = append(sections, successStyle.Render("✓ "+m.successMessage))
	}

	// Help text
	sections = append(sections, "")
	sections = append(sections, helpStyle.Render("Tab: Next field • Enter: Action • Ctrl+C: Cancel"))

	return strings.Join(sections, "\n")
}

// Event handlers
func (m DeviceConfigModel) handleEnter() (DeviceConfigModel, tea.Cmd) {
	if m.editMode || m.addMode {
		switch m.focusedField {
		case deviceConfigFieldSave:
			return m.saveDevice()
		case deviceConfigFieldTest:
			return m.testDevice()
		case deviceConfigFieldCancel:
			return m.exitEditMode(), nil
		}
	} else {
		// In list mode, enter starts edit
		if len(m.devices) > 0 {
			return m.startEditMode()
		}
	}
	return m, nil
}

func (m DeviceConfigModel) handleTabNavigation(reverse bool) (DeviceConfigModel, tea.Cmd) {
	if !m.editMode && !m.addMode {
		return m, nil
	}

	fields := []deviceConfigField{
		deviceConfigFieldID,
		deviceConfigFieldType,
		deviceConfigFieldModel,
		deviceConfigFieldAddress,
		deviceConfigFieldCredential,
		deviceConfigFieldCapabilities,
		deviceConfigFieldSave,
		deviceConfigFieldTest,
		deviceConfigFieldCancel,
	}

	currentIndex := 0
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
	m.syncCursors()
	return m, nil
}

func (m DeviceConfigModel) handleUp() DeviceConfigModel {
	if m.editMode || m.addMode {
		if m.focusedField == deviceConfigFieldType {
			if m.typeIndex > 0 {
				m.typeIndex--
				m.editingDevice.Type = m.deviceTypes[m.typeIndex]
			}
		}
	} else {
		if m.selectedDevice > 0 {
			m.selectedDevice--
		}
	}
	return m
}

func (m DeviceConfigModel) handleDown() DeviceConfigModel {
	if m.editMode || m.addMode {
		if m.focusedField == deviceConfigFieldType {
			if m.typeIndex < len(m.deviceTypes)-1 {
				m.typeIndex++
				m.editingDevice.Type = m.deviceTypes[m.typeIndex]
			}
		}
	} else {
		if m.selectedDevice < len(m.devices)-1 {
			m.selectedDevice++
		}
	}
	return m
}

func (m DeviceConfigModel) handleLeft() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.moveCursorLeft()
	}
	return m
}

func (m DeviceConfigModel) handleRight() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.moveCursorRight()
	}
	return m
}

func (m DeviceConfigModel) handleBackspace() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.deleteCharLeft()
	}
	return m
}

func (m DeviceConfigModel) handleDelete() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.deleteCharRight()
	}
	return m
}

func (m DeviceConfigModel) handleHome() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.moveCursorHome()
	}
	return m
}

func (m DeviceConfigModel) handleEnd() DeviceConfigModel {
	if m.editMode || m.addMode {
		m.moveCursorEnd()
	}
	return m
}

func (m DeviceConfigModel) handleTextInput(input string) (DeviceConfigModel, tea.Cmd) {
	if !m.editMode && !m.addMode {
		return m, nil
	}

	// Filter non-printable characters
	printableInput := ""
	for _, r := range input {
		if r >= 32 && r < 127 {
			printableInput += string(r)
		}
	}

	if printableInput == "" {
		return m, nil
	}

	m.insertText(printableInput)
	return m, nil
}

// Device operations
func (m DeviceConfigModel) startAddMode() (DeviceConfigModel, tea.Cmd) {
	template := m.configManager.CreateDeviceTemplate("bravia")
	m.editingDevice = &template
	m.addMode = true
	m.focusedField = deviceConfigFieldID
	m.syncCursors()
	return m, nil
}

func (m DeviceConfigModel) startEditMode() (DeviceConfigModel, tea.Cmd) {
	if len(m.devices) == 0 {
		return m, nil
	}

	// Copy the selected device for editing
	device := m.devices[m.selectedDevice]
	m.editingDevice = &device
	m.editMode = true
	m.focusedField = deviceConfigFieldID
	m.syncCursors()
	return m, nil
}

func (m DeviceConfigModel) exitEditMode() DeviceConfigModel {
	m.editMode = false
	m.addMode = false
	m.editingDevice = nil
	m.focusedField = deviceConfigFieldID
	return m
}

func (m DeviceConfigModel) saveDevice() (DeviceConfigModel, tea.Cmd) {
	if m.editingDevice == nil {
		return m, nil
	}

	// Validate device
	if m.editingDevice.ID == "" {
		m.errorMessage = "Device ID is required"
		return m, nil
	}

	if m.editingDevice.Type == "" {
		m.errorMessage = "Device type is required"
		return m, nil
	}

	if m.editingDevice.Address == "" {
		m.errorMessage = "Device address is required"
		return m, nil
	}

	// Parse capabilities from comma-separated string
	if m.focusedField == deviceConfigFieldCapabilities {
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		caps := strings.Split(capText, ",")
		var cleanCaps []string
		for _, cap := range caps {
			cap = strings.TrimSpace(cap)
			if cap != "" {
				cleanCaps = append(cleanCaps, cap)
			}
		}
		m.editingDevice.Capabilities = cleanCaps
	}

	var err error
	if m.addMode {
		err = m.configManager.AddDevice(*m.editingDevice)
	} else {
		originalID := m.devices[m.selectedDevice].ID
		err = m.configManager.UpdateDevice(originalID, *m.editingDevice)
	}

	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	// Reload devices and exit edit mode
	m.loadDevices()
	m.successMessage = "Device saved successfully"
	return m.exitEditMode(), nil
}

func (m DeviceConfigModel) deleteDevice() (DeviceConfigModel, tea.Cmd) {
	if len(m.devices) == 0 {
		return m, nil
	}

	device := m.devices[m.selectedDevice]
	if err := m.configManager.RemoveDevice(device.ID); err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	m.loadDevices()
	if m.selectedDevice >= len(m.devices) {
		m.selectedDevice = len(m.devices) - 1
	}
	if m.selectedDevice < 0 {
		m.selectedDevice = 0
	}

	m.successMessage = fmt.Sprintf("Device '%s' deleted", device.ID)
	return m, nil
}

func (m DeviceConfigModel) testDevice() (DeviceConfigModel, tea.Cmd) {
	// For now, just show a placeholder message
	// In a full implementation, this would test the actual device connection
	m.successMessage = "Device test not yet implemented"
	return m, nil
}

// Helper methods for cursor and text management
func (m *DeviceConfigModel) syncCursors() {
	if m.editingDevice == nil {
		return
	}

	switch m.focusedField {
	case deviceConfigFieldID:
		m.idCursor = len(m.editingDevice.ID)
	case deviceConfigFieldModel:
		m.modelCursor = len(m.editingDevice.Model)
	case deviceConfigFieldAddress:
		m.addressCursor = len(m.editingDevice.Address)
	case deviceConfigFieldCredential:
		m.credentialCursor = len(m.editingDevice.Credential)
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		m.capabilitiesCursor = len(capText)
	}
}

func (m *DeviceConfigModel) moveCursorLeft() {
	switch m.focusedField {
	case deviceConfigFieldID:
		if m.idCursor > 0 {
			m.idCursor--
		}
	case deviceConfigFieldModel:
		if m.modelCursor > 0 {
			m.modelCursor--
		}
	case deviceConfigFieldAddress:
		if m.addressCursor > 0 {
			m.addressCursor--
		}
	case deviceConfigFieldCredential:
		if m.credentialCursor > 0 {
			m.credentialCursor--
		}
	case deviceConfigFieldCapabilities:
		if m.capabilitiesCursor > 0 {
			m.capabilitiesCursor--
		}
	}
}

func (m *DeviceConfigModel) moveCursorRight() {
	switch m.focusedField {
	case deviceConfigFieldID:
		if m.idCursor < len(m.editingDevice.ID) {
			m.idCursor++
		}
	case deviceConfigFieldModel:
		if m.modelCursor < len(m.editingDevice.Model) {
			m.modelCursor++
		}
	case deviceConfigFieldAddress:
		if m.addressCursor < len(m.editingDevice.Address) {
			m.addressCursor++
		}
	case deviceConfigFieldCredential:
		if m.credentialCursor < len(m.editingDevice.Credential) {
			m.credentialCursor++
		}
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		if m.capabilitiesCursor < len(capText) {
			m.capabilitiesCursor++
		}
	}
}

func (m *DeviceConfigModel) moveCursorHome() {
	switch m.focusedField {
	case deviceConfigFieldID:
		m.idCursor = 0
	case deviceConfigFieldModel:
		m.modelCursor = 0
	case deviceConfigFieldAddress:
		m.addressCursor = 0
	case deviceConfigFieldCredential:
		m.credentialCursor = 0
	case deviceConfigFieldCapabilities:
		m.capabilitiesCursor = 0
	}
}

func (m *DeviceConfigModel) moveCursorEnd() {
	switch m.focusedField {
	case deviceConfigFieldID:
		m.idCursor = len(m.editingDevice.ID)
	case deviceConfigFieldModel:
		m.modelCursor = len(m.editingDevice.Model)
	case deviceConfigFieldAddress:
		m.addressCursor = len(m.editingDevice.Address)
	case deviceConfigFieldCredential:
		m.credentialCursor = len(m.editingDevice.Credential)
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		m.capabilitiesCursor = len(capText)
	}
}

func (m *DeviceConfigModel) insertText(text string) {
	switch m.focusedField {
	case deviceConfigFieldID:
		m.editingDevice.ID = insertText(m.editingDevice.ID, m.idCursor, text)
		m.idCursor += len(text)
	case deviceConfigFieldModel:
		m.editingDevice.Model = insertText(m.editingDevice.Model, m.modelCursor, text)
		m.modelCursor += len(text)
	case deviceConfigFieldAddress:
		m.editingDevice.Address = insertText(m.editingDevice.Address, m.addressCursor, text)
		m.addressCursor += len(text)
	case deviceConfigFieldCredential:
		m.editingDevice.Credential = insertText(m.editingDevice.Credential, m.credentialCursor, text)
		m.credentialCursor += len(text)
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		newCapText := insertText(capText, m.capabilitiesCursor, text)
		caps := strings.Split(newCapText, ",")
		var cleanCaps []string
		for _, cap := range caps {
			cap = strings.TrimSpace(cap)
			if cap != "" {
				cleanCaps = append(cleanCaps, cap)
			}
		}
		m.editingDevice.Capabilities = cleanCaps
		m.capabilitiesCursor += len(text)
	}
}

func (m *DeviceConfigModel) deleteCharLeft() {
	switch m.focusedField {
	case deviceConfigFieldID:
		if m.idCursor > 0 {
			m.editingDevice.ID = deleteCharAt(m.editingDevice.ID, m.idCursor-1)
			m.idCursor--
		}
	case deviceConfigFieldModel:
		if m.modelCursor > 0 {
			m.editingDevice.Model = deleteCharAt(m.editingDevice.Model, m.modelCursor-1)
			m.modelCursor--
		}
	case deviceConfigFieldAddress:
		if m.addressCursor > 0 {
			m.editingDevice.Address = deleteCharAt(m.editingDevice.Address, m.addressCursor-1)
			m.addressCursor--
		}
	case deviceConfigFieldCredential:
		if m.credentialCursor > 0 {
			m.editingDevice.Credential = deleteCharAt(m.editingDevice.Credential, m.credentialCursor-1)
			m.credentialCursor--
		}
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		if m.capabilitiesCursor > 0 {
			newCapText := deleteCharAt(capText, m.capabilitiesCursor-1)
			caps := strings.Split(newCapText, ",")
			var cleanCaps []string
			for _, cap := range caps {
				cap = strings.TrimSpace(cap)
				if cap != "" {
					cleanCaps = append(cleanCaps, cap)
				}
			}
			m.editingDevice.Capabilities = cleanCaps
			m.capabilitiesCursor--
		}
	}
}

func (m *DeviceConfigModel) deleteCharRight() {
	switch m.focusedField {
	case deviceConfigFieldID:
		if m.idCursor < len(m.editingDevice.ID) {
			m.editingDevice.ID = deleteCharAt(m.editingDevice.ID, m.idCursor)
		}
	case deviceConfigFieldModel:
		if m.modelCursor < len(m.editingDevice.Model) {
			m.editingDevice.Model = deleteCharAt(m.editingDevice.Model, m.modelCursor)
		}
	case deviceConfigFieldAddress:
		if m.addressCursor < len(m.editingDevice.Address) {
			m.editingDevice.Address = deleteCharAt(m.editingDevice.Address, m.addressCursor)
		}
	case deviceConfigFieldCredential:
		if m.credentialCursor < len(m.editingDevice.Credential) {
			m.editingDevice.Credential = deleteCharAt(m.editingDevice.Credential, m.credentialCursor)
		}
	case deviceConfigFieldCapabilities:
		capText := strings.Join(m.editingDevice.Capabilities, ", ")
		if m.capabilitiesCursor < len(capText) {
			newCapText := deleteCharAt(capText, m.capabilitiesCursor)
			caps := strings.Split(newCapText, ",")
			var cleanCaps []string
			for _, cap := range caps {
				cap = strings.TrimSpace(cap)
				if cap != "" {
					cleanCaps = append(cleanCaps, cap)
				}
			}
			m.editingDevice.Capabilities = cleanCaps
		}
	}
}

// Helper functions (these should be moved to shared utilities)
func insertText(text string, position int, insert string) string {
	if position < 0 {
		position = 0
	}
	if position > len(text) {
		position = len(text)
	}

	return text[:position] + insert + text[position:]
}

func deleteCharAt(text string, position int) string {
	if position < 0 || position >= len(text) {
		return text
	}

	return text[:position] + text[position+1:]
}

func renderTextWithCursor(text string, cursor int, showCursor bool) string {
	if !showCursor {
		return text
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(text) {
		cursor = len(text)
	}

	if cursor == len(text) {
		return text + "█"
	}

	return text[:cursor] + "█" + text[cursor+1:]
}

// Styles (should be shared with other CLI components)
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(0, 1)

	inputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FF79C6")).
				Padding(0, 1)

	buttonStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(0, 1).
			MarginTop(1)

	buttonActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#50FA7B")).
				Background(lipgloss.Color("#282A36")).
				Padding(0, 1).
				MarginTop(1).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))
)
