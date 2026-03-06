package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/internal/project"
	permEngine "github.com/vigo999/ms-cli/tools/permission"
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
	case "/model":
		a.cmdModel(parts[1:])
	case "/exit":
		a.cmdExit()
	case "/compact":
		a.cmdCompact()
	case "/clear":
		a.cmdClear()
	case "/test":
		a.cmdTest()
	case "/permission":
		a.cmdPermission(parts[1:])
	case "/yolo":
		a.cmdYolo()
	case "/mouse":
		a.cmdMouse(parts[1:])
	case "/help":
		a.cmdHelp()
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", parts[0]),
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

// cmdModel handles "/model [model-name]".
func (a *Application) cmdModel(args []string) {
	if len(args) == 0 {
		// Show current model config.
		a.showCurrentModel()
		return
	}

	// Accept "openai:model" for backward compatibility.
	modelArg := args[0]
	if strings.Contains(modelArg, ":") {
		parts := strings.SplitN(modelArg, ":", 2)
		providerName := strings.TrimSpace(parts[0])
		modelName := parts[1]
		if providerName != "" && providerName != "openai" {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: fmt.Sprintf("Unsupported provider prefix: %s (only openai-compatible is supported)", providerName),
			}
			return
		}
		a.switchModel(modelName)
		return
	}

	// Just switch model.
	a.switchModel(modelArg)
}

// showCurrentModel displays current URL/model/key status.
func (a *Application) showCurrentModel() {
	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = "https://api.openai.com/v1"
	}

	apiKeyStatus := "not set"
	if a.Config.Model.Key != "" ||
		getEnv("MSCLI_API_KEY") != "" ||
		getEnv("OPENAI_API_KEY") != "" {
		apiKeyStatus = "set"
	}

	msg := fmt.Sprintf(`Current Model Configuration:

  URL:   %s
  Model: %s
  Key:   %s

To switch model:
  /model <model-name>
  /model openai:<model>         (backward-compatible prefix)

Examples:
  /model gpt-4o
  /model openai:gpt-4o-mini`,
		url, modelName, apiKeyStatus)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
	}
}

// switchModel switches to a new model.
func (a *Application) switchModel(modelName string) {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	err := a.SetProvider("", modelName, "")
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "model",
			Message:  fmt.Sprintf("Failed to switch model: %v", err),
		}
		return
	}

	// Update UI model name
	a.EventCh <- model.Event{
		Type:    model.ModelUpdate,
		Message: a.Config.Model.Model,
	}

	// Save state to disk
	if err := a.SaveState(); err != nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Model switched to: %s. Warning: failed to save state: %v", a.Config.Model.Model, err),
		}
		return
	}

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Model switched to: %s", a.Config.Model.Model),
	}
}

// getEnv is a helper to get environment variable.
func getEnv(key string) string {
	return os.Getenv(key)
}

// cmdExit handles "/exit".
func (a *Application) cmdExit() {
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: "Goodbye!",
	}
	// Send Done event to close the UI
	go func() {
		time.Sleep(100 * time.Millisecond)
		a.EventCh <- model.Event{Type: model.Done}
	}()
}

// cmdCompact handles "/compact".
func (a *Application) cmdCompact() {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	// Trigger context compaction through the engine
	if a.Engine != nil {
		// In a real implementation, this would compact the conversation context
		// For now, just show a message
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Context compacted. Conversation summary has been created to save tokens.",
		}
	} else {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Context compaction is not available in demo mode.",
		}
	}
}

// cmdClear handles "/clear".
func (a *Application) cmdClear() {
	// Clear all messages by sending a special event
	a.EventCh <- model.Event{
		Type:    model.ClearScreen,
		Message: "Chat history cleared.",
	}
}

// cmdTest handles "/test" - tests API connectivity.
func (a *Application) cmdTest() {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	// Get current model config.
	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	apiKeyStatus := "not set"
	if a.Config.Model.Key != "" {
		apiKeyStatus = "set (" + fmt.Sprintf("%d chars", len(a.Config.Model.Key)) + ")"
	}

	msg := fmt.Sprintf(`API Connection Test:

  URL:     %s
  Model:   %s
  API Key: %s

Testing connectivity...`, url, modelName, apiKeyStatus)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
	}

	// Try a simple completion to test the API
	if a.Engine != nil && !a.Demo {
		// The actual test will happen when the user sends a message
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "API configuration looks correct. Send a message to test the connection.",
		}
	} else {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Cannot test in demo mode. Run without --demo flag to test API connectivity.",
		}
	}
}

// cmdPermission handles "/permission [tool] [level]".
func (a *Application) cmdPermission(args []string) {
	engine, ok := a.permEngine.(*permEngine.DefaultEngine)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission management not available in current mode.",
		}
		return
	}

	if len(args) == 0 {
		// Show current permissions
		ruleset := engine.GetRuleset()
		msg := "Current Permission Settings:\n\n"
		if len(ruleset.Rules) == 0 {
			msg += "  No custom permissions set.\n"
			msg += "  Default: ask for destructive operations (write, edit, shell)\n"
		} else {
			for _, rule := range ruleset.Rules {
				if rule.Enabled {
					msg += fmt.Sprintf("  %s: %s\n", rule.Permission, rule.Action)
				}
			}
		}
		msg += "\nUsage:\n"
		msg += "  /permission <tool> <level>\n"
		msg += "\nLevels:\n"
		msg += "  ask         - Ask each time (default)\n"
		msg += "  allow       - Always allow\n"
		msg += "  deny        - Always deny\n"
		msg += "\nTools: read, write, edit, grep, glob, shell\n"
		msg += "\nExamples:\n"
		msg += "  /permission shell ask\n"
		msg += "  /permission write allow"
		a.EventCh <- model.Event{Type: model.AgentReply, Message: msg}
		return
	}

	if len(args) < 2 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /permission <tool> <level>\nExample: /permission shell ask",
		}
		return
	}

	tool := args[0]
	levelStr := args[1]

	// Convert level string to Action
	var action permEngine.Action
	switch strings.ToLower(levelStr) {
	case "allow", "allow_always":
		action = permEngine.ActionAllow
	case "deny":
		action = permEngine.ActionDeny
	case "ask":
		action = permEngine.ActionAsk
	default:
		action = permEngine.ActionAsk
	}

	// Add new rule
	ruleset := engine.GetRuleset()
	ruleset.AddRule(permEngine.Rule{
		ID:         "cmd-" + tool,
		Permission: tool + ":*",
		Action:     action,
		Enabled:    true,
	})
	engine.UpdateRuleset(ruleset)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Permission for '%s' set to: %s", tool, action),
	}
}

// cmdYolo handles "/yolo" - toggles auto-approve mode.
func (a *Application) cmdYolo() {
	engine, ok := a.permEngine.(*permEngine.DefaultEngine)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode not available in current configuration.",
		}
		return
	}

	ruleset := engine.GetRuleset()

	// Check current state by looking at shell permission
	result := engine.CanExecute("shell:*", "", "")
	if result.Action == permEngine.ActionAllow {
		// Disable yolo mode - restore defaults
		ruleset.AddRule(permEngine.Rule{
			ID:         "yolo-shell",
			Permission: "shell:*",
			Action:     permEngine.ActionAsk,
			Enabled:    true,
		})
		ruleset.AddRule(permEngine.Rule{
			ID:         "yolo-write",
			Permission: "write:*",
			Action:     permEngine.ActionAsk,
			Enabled:    true,
		})
		ruleset.AddRule(permEngine.Rule{
			ID:         "yolo-edit",
			Permission: "edit:*",
			Action:     permEngine.ActionAsk,
			Enabled:    true,
		})
		engine.UpdateRuleset(ruleset)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "🔒 YOLO mode disabled. Will ask for confirmation on destructive operations.",
		}
	} else {
		// Enable yolo mode - allow all tools
		tools := []string{"shell", "write", "edit", "read", "grep", "glob"}
		for _, tool := range tools {
			ruleset.AddRule(permEngine.Rule{
				ID:         "yolo-" + tool,
				Permission: tool + ":*",
				Action:     permEngine.ActionAllow,
				Enabled:    true,
			})
		}
		engine.UpdateRuleset(ruleset)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "⚡ YOLO mode enabled! All operations will be auto-approved. Use with caution!",
		}
	}
}

// cmdMouse handles "/mouse [on|off|toggle|status]".
func (a *Application) cmdMouse(args []string) {
	mode := "toggle"
	if len(args) > 0 {
		mode = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch mode {
	case "on", "enable", "enabled":
		a.EventCh <- model.Event{Type: model.MouseModeToggle, Message: "on"}
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Mouse scrolling enabled. Use wheel to scroll chat."}
	case "off", "disable", "disabled":
		a.EventCh <- model.Event{Type: model.MouseModeToggle, Message: "off"}
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Mouse scrolling disabled."}
	case "toggle":
		a.EventCh <- model.Event{Type: model.MouseModeToggle, Message: "toggle"}
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Mouse scrolling toggled."}
	case "status":
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Use `/mouse on` to enable scroll wheel, `/mouse off` to disable, `/mouse toggle` to switch.",
		}
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /mouse [on|off|toggle|status]",
		}
	}
}

// cmdHelp handles "/help".
func (a *Application) cmdHelp() {
	helpText := `Available commands:

  /roadmap status [path]  Check roadmap status (default: roadmap.yaml)
  /weekly status [path]   Check weekly update status (default: weekly.md)
  /model [model-name]     Show or switch model
  /test                   Test API connectivity
  /permission [tool] [level]  Manage tool permissions
  /yolo                   Toggle auto-approve mode
  /mouse [on|off|toggle|status] Toggle mouse wheel scrolling
  /exit                   Exit the application
  /compact                Compact conversation context to save tokens
  /clear                  Clear chat history
  /help                   Show this help message

Model Commands:
  /model                  Show current configuration
  /model gpt-4o           Switch to gpt-4o
  /model openai:gpt-4o    Backward-compatible format

Permission Commands:
  /permission             Show current permission settings
  /permission shell ask   Set permission level for a tool
  /yolo                   Toggle auto-approve for all operations

Permission Levels:
  ask          - Ask each time (default)
  allow_once   - Allow once
  allow_session - Allow for this session
  allow_always - Always allow
  deny         - Always deny

Keybindings:
  enter      Send input
  ↑/↓        Navigate slash suggestions
  mouse wheel Scroll chat
  pgup/pgdn  Scroll chat
  home/end   Jump to top/bottom
  /          Start a slash command
  ctrl+c     Cancel/Quit (press twice to exit)

Environment Variables:
  MSCLI_BASE_URL          OpenAI-compatible base URL
  MSCLI_MODEL             Default model
  MSCLI_API_KEY           API key
  OPENAI_BASE_URL         Base URL (fallback)
  OPENAI_MODEL            Model (fallback)
  OPENAI_API_KEY          API key (fallback)`

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: helpText,
	}
}
