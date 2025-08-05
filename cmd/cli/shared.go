package cli

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Screen types
type screen int

const (
	screenDeviceSetup screen = iota
	screenRemoteControl
)

// Common styles
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

	remoteButtonStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Margin(0, 1).
		Background(lipgloss.Color("#44475A")).
		Foreground(lipgloss.Color("#F8F8F2"))

	remoteButtonActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Margin(0, 1).
		Background(lipgloss.Color("#FF79C6")).
		Foreground(lipgloss.Color("#FAFAFA"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))
)

// Remote button types
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

// Action history entry
type actionHistoryEntry struct {
	Timestamp time.Time
	Action    string
	Success   bool
	Response  string
	Error     string
}

// Utility functions

// insertText inserts text at the specified position in a string
func insertText(text string, pos int, insert string) string {
	if pos < 0 {
		pos = 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	return text[:pos] + insert + text[pos:]
}

// deleteCharAt deletes the character at the specified position
func deleteCharAt(text string, pos int) string {
	if pos < 0 || pos >= len(text) {
		return text
	}
	return text[:pos] + text[pos+1:]
}

// renderTextWithCursor renders text with a cursor indicator at the specified position
func renderTextWithCursor(text string, cursorPos int, showCursor bool) string {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}