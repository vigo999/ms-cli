package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TextInput wraps the bubbles text input for the chat prompt.
type TextInput struct {
	Model textinput.Model
}

// NewTextInput creates a focused text input with "> " prompt.
func NewTextInput() TextInput {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 2000
	return TextInput{Model: ti}
}

// Value returns the current input text.
func (t TextInput) Value() string {
	return t.Model.Value()
}

// Reset clears the input.
func (t TextInput) Reset() TextInput {
	t.Model.Reset()
	return t
}

// Focus gives the input focus.
func (t TextInput) Focus() (TextInput, tea.Cmd) {
	cmd := t.Model.Focus()
	return t, cmd
}

// Update handles key events.
func (t TextInput) Update(msg tea.Msg) (TextInput, tea.Cmd) {
	m, cmd := t.Model.Update(msg)
	return TextInput{Model: m}, cmd
}

// View renders the input.
func (t TextInput) View() string {
	return t.Model.View()
}
