package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/model"
)

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Italic(true)

	// expanded tool block
	toolBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	toolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	toolContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				PaddingLeft(2)

	// collapsed tool (dim, single line)
	collapsedIconStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	collapsedNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	collapsedSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true)

	// error tool block
	errorBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	errorHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	errorContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("203")).
				PaddingLeft(2)

	// edit/write diff styles
	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114"))

	diffRemoveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	diffNeutralStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				PaddingLeft(2)
)

// RenderMessages converts messages into styled text for the viewport.
// stats is used to show summary when thinking is done.
func RenderMessages(state model.State, spinnerView string) string {
	var parts []string
	messages := state.Messages
	stats := state.Stats
	isThinking := state.IsThinking

	for _, m := range messages {
		switch m.Kind {
		case model.MsgUser:
			parts = append(parts, renderUserMsg(m.Content))
		case model.MsgAgent:
			parts = append(parts, renderAgentMsg(m.Content))
		case model.MsgThinking:
			if isThinking {
				// Show animated thinking indicator
				parts = append(parts, renderThinking(spinnerView))
			} else {
				// Show "Done" with summary
				parts = append(parts, renderDone(stats))
			}
		case model.MsgTool:
			parts = append(parts, renderTool(m))
		}
	}

	return strings.Join(parts, "\n\n")
}

func renderUserMsg(content string) string {
	return fmt.Sprintf("  %s %s", userStyle.Render(">"), userStyle.Render(content))
}

func renderAgentMsg(content string) string {
	lines := strings.Split(content, "\n")
	styled := make([]string, len(lines))
	for i, line := range lines {
		styled[i] = "  " + agentStyle.Render(line)
	}
	return strings.Join(styled, "\n")
}

func renderThinking(thinkingView string) string {
	// Animated thinking indicator with Braille spinner
	// thinkingView already contains the spinner and text from ThinkingSpinner.View()
	return "  " + thinkingView
}

// renderDone shows completed task summary without animation
func renderDone(stats model.TaskStats) string {
	var parts []string
	
	// Always show Done
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114")) // green
	parts = append(parts, doneStyle.Render("✓ Done"))
	
	// Show simple execution summary if there's anything to report
	summaryParts := []string{}
	if stats.Commands > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d commands", stats.Commands))
	}
	if stats.FilesRead > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d files read", stats.FilesRead))
	}
	if stats.FilesEdited > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d files modified", stats.FilesEdited))
	}
	if stats.Searches > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d searches", stats.Searches))
	}
	if stats.Errors > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d errors", stats.Errors))
	}
	
	if len(summaryParts) > 0 {
		summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		parts = append(parts, summaryStyle.Render("("+strings.Join(summaryParts, ", ")+")"))
	}
	
	return "  " + strings.Join(parts, " ")
}

func renderTool(m model.Message) string {
	switch m.Display {
	case model.DisplayCollapsed:
		return renderCollapsedTool(m)
	case model.DisplayError:
		return renderErrorTool(m)
	default:
		return renderExpandedTool(m)
	}
}

// --- Collapsed: single dim line ---
// "  ▸ Read model/layer3.go — 42 lines"
// "  ▸ Grep "allocTensor" — 5 matches"
func renderCollapsedTool(m model.Message) string {
	summary := ""
	if m.Summary != "" {
		summary = " — " + collapsedSummaryStyle.Render(m.Summary)
	}
	return fmt.Sprintf("  %s %s%s",
		collapsedIconStyle.Render("▸"),
		collapsedNameStyle.Render(m.ToolName+" "+m.Content),
		summary,
	)
}

// --- Expanded: full output with header + body ---
func renderExpandedTool(m model.Message) string {
	// edit/write get diff rendering
	if m.ToolName == "Edit" || m.ToolName == "Write" {
		return renderDiffTool(m)
	}

	header := fmt.Sprintf("  %s %s %s",
		toolBorderStyle.Render("▸"),
		toolHeaderStyle.Render(m.ToolName),
		toolBorderStyle.Render(strings.Repeat("─", 50)),
	)

	lines := strings.Split(m.Content, "\n")
	styled := make([]string, len(lines))
	for i, line := range lines {
		styled[i] = toolContentStyle.Render(line)
	}
	body := strings.Join(styled, "\n")

	return header + "\n" + body
}

// --- Diff: edit/write with +/- coloring ---
func renderDiffTool(m model.Message) string {
	header := fmt.Sprintf("  %s %s %s",
		toolBorderStyle.Render("▸"),
		toolHeaderStyle.Render(m.ToolName),
		toolBorderStyle.Render(strings.Repeat("─", 50)),
	)

	lines := strings.Split(m.Content, "\n")
	styled := make([]string, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "+"):
			styled[i] = "  " + diffAddStyle.Render(line)
		case strings.HasPrefix(trimmed, "-"):
			styled[i] = "  " + diffRemoveStyle.Render(line)
		default:
			styled[i] = diffNeutralStyle.Render(line)
		}
	}
	body := strings.Join(styled, "\n")

	return header + "\n" + body
}

// --- Error: red highlighted block ---
func renderErrorTool(m model.Message) string {
	header := fmt.Sprintf("  %s %s %s",
		errorBorderStyle.Render("✗"),
		errorHeaderStyle.Render(m.ToolName+" failed"),
		errorBorderStyle.Render(strings.Repeat("─", 44)),
	)

	lines := strings.Split(m.Content, "\n")
	styled := make([]string, len(lines))
	for i, line := range lines {
		styled[i] = errorContentStyle.Render(line)
	}
	body := strings.Join(styled, "\n")

	return header + "\n" + body
}
