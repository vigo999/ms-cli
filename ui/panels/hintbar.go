package panels

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	hintDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	hintTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(1)

	hintKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	hintDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	hintSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

type hint struct {
	key  string
	desc string
}

var hints = []hint{
	{"/", "commands"},
	{"ctrl+l", "model"},
	{"ctrl+j", "newline"},
	{"ctrl+c", "quit"},
	{"ctrl+g", "more"},
}

// RenderHintBar renders the bottom keybinding hint bar.
func RenderHintBar(width int) string {
	divider := hintDividerStyle.Render(repeatChar("━", width))

	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = hintKeyStyle.Render(h.key) + " " + hintDescStyle.Render(h.desc)
	}

	sep := hintSepStyle.Render(" • ")
	line := hintTextStyle.Render("")
	for i, p := range parts {
		if i > 0 {
			line += sep
		}
		line += p
	}

	return divider + "\n" + line
}
