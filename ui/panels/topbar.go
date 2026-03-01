package panels

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/model"
)

var (
	topBarStyle = lipgloss.NewStyle().
			Padding(0, 1)

	brandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	bannerLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	bannerValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	bannerDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// RenderTopBar renders the top status bar.
// When showBanner is true, a second line with workdir + repo is shown.
func RenderTopBar(s model.State, width int) string {
	sep := sepStyle.Render("│")

	// Line 1: brand + model info (always shown)
	left := brandStyle.Render(s.Version)
	right := strings.Join([]string{
		infoStyle.Render("model:"),
		infoStyle.Render(s.Model.Name),
		sep,
		infoStyle.Render(fmt.Sprintf("ctx: %s/%s", formatTokens(s.Model.CtxUsed), formatTokens(s.Model.CtxMax))),
		sep,
		infoStyle.Render(fmt.Sprintf("tokens: %s", formatTokens(s.Model.TokensUsed))),
	}, " ")

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	pad := lipgloss.NewStyle().Width(gap).Render("")
	line1 := topBarStyle.Render(left + pad + right)

	divider := dividerStyle.Render(repeatChar("━", width))

	// Line 2: workdir + repo
	left2 := bannerLabelStyle.Render("cwd:") + " " + bannerValueStyle.Render(shortenPath(s.WorkDir))
	right2 := bannerDimStyle.Render(s.RepoURL)

	gap2 := width - lipgloss.Width(left2) - lipgloss.Width(right2) - 2
	if gap2 < 1 {
		gap2 = 1
	}
	pad2 := lipgloss.NewStyle().Width(gap2).Render("")
	line2 := topBarStyle.Render(left2 + pad2 + right2)

	return line1 + "\n" + line2 + "\n" + divider
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func repeatChar(ch string, n int) string {
	return strings.Repeat(ch, n)
}
