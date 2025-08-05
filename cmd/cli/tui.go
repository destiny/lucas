package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#7D56F4"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))
)

type model struct {
	choices  []string
	cursor   int
	selected map[int]struct{}
	quitting bool
}

func initialModel() model {
	return model{
		choices: []string{
			"Interactive File Manager",
			"System Information",
			"Process Monitor",
			"Log Viewer",
			"Configuration Editor",
			"Exit",
		},
		selected: make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			if m.choices[m.cursor] == "Exit" {
				m.quitting = true
				return m, tea.Quit
			}
			
			// Toggle selection
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Thanks for using Lucas CLI!\n"
	}

	s := strings.Builder{}
	s.WriteString(titleStyle.Render("Lucas CLI - Interactive Mode"))
	s.WriteString("\n\n")

	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "✓"
		}

		style := itemStyle
		if m.cursor == i {
			style = selectedItemStyle
		}

		s.WriteString(style.Render(fmt.Sprintf("%s [%s] %s", cursor, checked, choice)))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("Press q to quit, ↑/↓ to move, enter/space to select"))
	s.WriteString("\n")

	return s.String()
}

func StartTUI() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}