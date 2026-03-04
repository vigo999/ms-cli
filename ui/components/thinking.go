package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// thinkingSpinnerFrames are Braille characters for a smooth spinning animation
var thinkingSpinnerFrames = []string{
	"⣷", "⣯", "⣟", "⣿", "⣻", "⣽", "⣾", "⣷",
}

var thinkingStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("205")).
	Italic(true)

var thinkingSpinnerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("212"))

// ThinkingSpinner shows a "⣻ Thinking..." animated indicator
type ThinkingSpinner struct {
	frame int
	text  string
}

// NewThinkingSpinner creates a new thinking spinner with default text.
func NewThinkingSpinner() ThinkingSpinner {
	return ThinkingSpinner{
		frame: 0,
		text:  "Thinking...",
	}
}

// NewThinkingSpinnerWithText creates a spinner with custom text.
func NewThinkingSpinnerWithText(text string) ThinkingSpinner {
	return ThinkingSpinner{
		frame: 0,
		text:  text,
	}
}

// SetText updates the spinner text.
func (t *ThinkingSpinner) SetText(text string) {
	t.text = text
}

// TickMsg is the message sent on each animation tick.
type TickMsg struct {
	Time time.Time
}

// Tick returns a command that ticks the spinner.
func (t ThinkingSpinner) Tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// Update advances the spinner animation.
func (t ThinkingSpinner) Update(msg tea.Msg) (ThinkingSpinner, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		t.frame = (t.frame + 1) % len(thinkingSpinnerFrames)
		return t, t.Tick()
	default:
		return t, nil
	}
}

// View renders the thinking spinner.
func (t ThinkingSpinner) View() string {
	frame := thinkingSpinnerFrames[t.frame]
	return fmt.Sprintf("%s %s", 
		thinkingSpinnerStyle.Render(frame),
		thinkingStyle.Render(t.text))
}

// IsThinking returns true if the spinner is active.
func (t ThinkingSpinner) IsThinking() bool {
	return true
}

// Frame returns the current frame index (for testing).
func (t ThinkingSpinner) Frame() int {
	return t.frame
}

// Reset resets the spinner to the first frame.
func (t *ThinkingSpinner) Reset() {
	t.frame = 0
}
