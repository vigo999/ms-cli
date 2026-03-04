// Demo of the ThinkingSpinner animation
package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/ui/components"
)

type model struct {
	thinking components.ThinkingSpinner
	message  string
	done     bool
}

func initialModel() model {
	return model{
		thinking: components.NewThinkingSpinner(),
		message:  "",
		done:     false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.thinking.Tick(),
		simulateWork(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case components.TickMsg:
		var cmd tea.Cmd
		m.thinking, cmd = m.thinking.Update(msg)
		return m, cmd

	case workDoneMsg:
		m.done = true
		m.message = "✓ Done! Thinking completed."
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.done {
		return m.message + "\n\nPress 'q' to quit.\n"
	}
	return m.thinking.View() + "\n\nPress 'q' to quit.\n"
}

type workDoneMsg struct{}

func simulateWork() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return workDoneMsg{}
	})
}

func main() {
	fmt.Println("ThinkingSpinner Demo")
	fmt.Println("====================")
	fmt.Println()

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
