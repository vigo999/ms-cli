package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/internal/project"
	"github.com/vigo999/ms-cli/ui/model"
)

// handleCommand dispatches slash commands.
func (a *Application) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/roadmap":
		a.cmdRoadmap(parts[1:])
	case "/weekly":
		a.cmdWeekly(parts[1:])
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Unknown command: %s", parts[0]),
		}
	}
}

// cmdRoadmap handles "/roadmap status [path]".
func (a *Application) cmdRoadmap(args []string) {
	if len(args) == 0 || args[0] != "status" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /roadmap status [path] (default: roadmap.yaml)",
		}
		return
	}

	path := "roadmap.yaml"
	if len(args) > 1 {
		path = args[1]
	}

	a.EventCh <- model.Event{Type: model.AgentThinking}

	rm, err := project.LoadRoadmapFromFile(path)
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "roadmap",
			Message:  err.Error(),
		}
		return
	}

	status, err := project.ComputeRoadmapStatus(rm, time.Now())
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "roadmap",
			Message:  err.Error(),
		}
		return
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: string(data),
	}
}

// cmdWeekly handles "/weekly status [path]".
func (a *Application) cmdWeekly(args []string) {
	if len(args) == 0 || args[0] != "status" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /weekly status [path] (default: weekly.md)",
		}
		return
	}

	path := "weekly.md"
	if len(args) > 1 {
		path = args[1]
	}

	a.EventCh <- model.Event{Type: model.AgentThinking}

	wu, err := project.LoadWeeklyUpdateFromFile(path)
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "weekly",
			Message:  err.Error(),
		}
		return
	}

	data, _ := json.MarshalIndent(wu, "", "  ")
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: string(data),
	}
}
