package panels

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"mscli/ui/model"
)

var (
	selectorBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)

	activeItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	inactiveItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	newTaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)

// TaskSelector tracks the cursor in the task pool overlay.
type TaskSelector struct {
	Cursor int
}

// NewTaskSelector returns a selector with cursor at the active task.
func NewTaskSelector(activeTask int) TaskSelector {
	return TaskSelector{Cursor: activeTask}
}

// Up moves the cursor up.
func (ts TaskSelector) Up() TaskSelector {
	c := ts.Cursor - 1
	if c < 0 {
		c = 0
	}
	return TaskSelector{Cursor: c}
}

// Down moves the cursor down. max is len(tasks) (includes "+new task" row).
func (ts TaskSelector) Down(max int) TaskSelector {
	c := ts.Cursor + 1
	if c > max {
		c = max
	}
	return TaskSelector{Cursor: c}
}

// RenderTaskSelector renders the task pool dropdown overlay.
func RenderTaskSelector(tasks []model.TaskInfo, activeTask int, sel TaskSelector) string {
	lines := make([]string, 0, len(tasks)+1)

	for i, t := range tasks {
		bullet := "○"
		style := inactiveItemStyle
		if i == activeTask {
			bullet = "●"
		}
		if i == sel.Cursor {
			style = activeItemStyle
		}
		lines = append(lines, style.Render(fmt.Sprintf("  %s %s", bullet, t.Name)))
	}

	newLine := newTaskStyle
	if sel.Cursor == len(tasks) {
		newLine = activeItemStyle
	}
	lines = append(lines, newLine.Render("  + new task"))

	content := ""
	for i, l := range lines {
		if i > 0 {
			content += "\n"
		}
		content += l
	}

	return selectorBorderStyle.Render(content)
}
