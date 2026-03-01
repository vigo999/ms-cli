package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Viewport wraps the bubbles viewport for scrollable chat content.
type Viewport struct {
	Model viewport.Model
	lines []string
}

// AtBottom reports whether the viewport is scrolled to the bottom.
func (v Viewport) AtBottom() bool {
	return v.Model.AtBottom()
}

// NewViewport creates a viewport with the given dimensions.
func NewViewport(width, height int) Viewport {
	vp := viewport.New(width, height)
	return Viewport{Model: vp}
}

// SetSize updates the viewport dimensions.
func (v Viewport) SetSize(width, height int) Viewport {
	v.Model.Width = width
	v.Model.Height = height
	v.Model.SetContent(strings.Join(v.lines, "\n"))
	return v
}

// Append adds a line. Only auto-scrolls if already at bottom.
func (v Viewport) Append(line string) Viewport {
	follow := v.AtBottom()
	newLines := make([]string, len(v.lines), len(v.lines)+1)
	copy(newLines, v.lines)
	newLines = append(newLines, line)
	v.lines = newLines
	v.Model.SetContent(strings.Join(v.lines, "\n"))
	if follow {
		v.Model.GotoBottom()
	}
	return v
}

// SetContent replaces all content. If followBottom is true, scrolls to bottom.
func (v Viewport) SetContent(content string) Viewport {
	follow := v.AtBottom()
	v.lines = strings.Split(content, "\n")
	v.Model.SetContent(content)
	if follow {
		v.Model.GotoBottom()
	}
	return v
}

// Clear resets the viewport.
func (v Viewport) Clear() Viewport {
	v.lines = nil
	v.Model.SetContent("")
	return v
}

// Update handles scroll keys.
func (v Viewport) Update(msg tea.Msg) (Viewport, tea.Cmd) {
	m, cmd := v.Model.Update(msg)
	v.Model = m
	return v, cmd
}

// View renders the viewport.
func (v Viewport) View() string {
	return v.Model.View()
}
