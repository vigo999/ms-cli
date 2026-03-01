package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// Spinner wraps the bubbles spinner for the thinking indicator.
type Spinner struct {
	Model spinner.Model
}

// NewSpinner creates a spinner with the dot style.
func NewSpinner() Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return Spinner{Model: s}
}

// Tick returns the spinner tick command.
func (s Spinner) Tick() tea.Msg {
	return s.Model.Tick()
}

// Update advances the spinner animation.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	m, cmd := s.Model.Update(msg)
	return Spinner{Model: m}, cmd
}

// View renders the spinner.
func (s Spinner) View() string {
	return s.Model.View()
}
