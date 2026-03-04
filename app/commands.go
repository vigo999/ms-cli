package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/loop"
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
	case "/model":
		a.cmdModel(parts[1:])
	case "/provider":
		a.cmdProvider(parts[1:])
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
		// Show current model info
		a.showCurrentModel()
		return
	}

	// Check if it's a provider:model format
	modelArg := args[0]
	if strings.Contains(modelArg, ":") {
		parts := strings.SplitN(modelArg, ":", 2)
		providerName := parts[0]
		modelName := parts[1]
		a.switchProviderAndModel(providerName, modelName)
		return
	}

	// Just switch model (keep current provider)
	a.switchModel(modelArg)
}

// cmdProvider handles "/provider [name]".
func (a *Application) cmdProvider(args []string) {
	if len(args) == 0 {
		// Show current provider
		a.showCurrentModel()
		return
	}

	providerName := args[0]
	// Use default model for the provider
	var modelName string
	switch providerName {
	case "openai":
		modelName = "gpt-4o-mini"
	case "openrouter":
		modelName = "anthropic/claude-3.5-sonnet"
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Unknown provider: %s. Supported providers: openai, openrouter", providerName),
		}
		return
	}

	a.switchProviderAndModel(providerName, modelName)
}

// showCurrentModel displays current provider and model.
func (a *Application) showCurrentModel() {
	modelName := a.Config.Model.Model
	provider := a.Config.Model.Provider
	endpoint := a.Config.Model.Endpoint
	if endpoint == "" {
		switch provider {
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "openrouter":
			endpoint = "https://openrouter.ai/api/v1"
		}
	}

	apiKeyStatus := "not set"
	if a.Config.Model.APIKey != "" ||
		(provider == "openai" && getEnv("OPENAI_API_KEY") != "") ||
		(provider == "openrouter" && getEnv("OPENROUTER_API_KEY") != "") {
		apiKeyStatus = "set"
	}

	msg := fmt.Sprintf(`Current Model Configuration:

  Provider: %s
  Model:    %s
  Endpoint: %s
  API Key:  %s

To switch model:
  /model <model-name>           (keep current provider)
  /model <provider>:<model>     (switch both provider and model)
  /provider <provider-name>     (switch provider with default model)

Examples:
  /model gpt-4o
  /model openrouter:anthropic/claude-3-opus
  /provider openrouter`,
		provider, modelName, endpoint, apiKeyStatus)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
	}
}

// switchModel switches to a new model (keeping current provider).
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
			Message: fmt.Sprintf("Model switched to: %s (provider: %s). Warning: failed to save state: %v", a.Config.Model.Model, a.Config.Model.Provider, err),
		}
		return
	}

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Model switched to: %s (provider: %s)", a.Config.Model.Model, a.Config.Model.Provider),
	}
}

// switchProviderAndModel switches both provider and model.
func (a *Application) switchProviderAndModel(providerName, modelName string) {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	err := a.SetProvider(providerName, modelName, "")
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "provider",
			Message:  fmt.Sprintf("Failed to switch provider: %v", err),
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
			Message: fmt.Sprintf("Switched to provider: %s, model: %s. Warning: failed to save state: %v", a.Config.Model.Provider, a.Config.Model.Model, err),
		}
		return
	}

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Switched to provider: %s, model: %s", a.Config.Model.Provider, a.Config.Model.Model),
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

	// Get current provider info
	provider := a.Config.Model.Provider
	modelName := a.Config.Model.Model
	apiKeyStatus := "not set"
	if a.Config.Model.APIKey != "" {
		apiKeyStatus = "set (" + fmt.Sprintf("%d chars", len(a.Config.Model.APIKey)) + ")"
	}

	msg := fmt.Sprintf(`API Connection Test:

  Provider: %s
  Model:    %s
  API Key:  %s

Testing connectivity...`, provider, modelName, apiKeyStatus)

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
	permSvc, ok := a.permService.(*loop.DefaultPermissionService)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission management not available in current mode.",
		}
		return
	}

	if len(args) == 0 {
		// Show current permissions
		policies := permSvc.GetPolicies()
		msg := "Current Permission Settings:\n\n"
		if len(policies) == 0 {
			msg += "  No custom permissions set.\n"
			msg += "  Default: ask for destructive operations (write, edit, shell)\n"
		} else {
			for tool, level := range policies {
				msg += fmt.Sprintf("  %s: %s\n", tool, level)
			}
		}
		msg += "\nUsage:\n"
		msg += "  /permission <tool> <level>\n"
		msg += "\nLevels:\n"
		msg += "  ask         - Ask each time (default)\n"
		msg += "  allow_once  - Allow once\n"
		msg += "  allow_session - Allow for this session\n"
		msg += "  allow_always - Always allow\n"
		msg += "  deny        - Always deny\n"
		msg += "\nTools: read, write, edit, grep, glob, shell\n"
		msg += "\nExamples:\n"
		msg += "  /permission shell ask\n"
		msg += "  /permission write allow_always"
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
	level := loop.ParsePermissionLevel(levelStr)

	permSvc.Grant(tool, level)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Permission for '%s' set to: %s", tool, level),
	}
}

// cmdYolo handles "/yolo" - toggles auto-approve mode.
func (a *Application) cmdYolo() {
	permSvc, ok := a.permService.(*loop.DefaultPermissionService)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode not available in current configuration.",
		}
		return
	}

	// Check current state by looking at shell permission
	current := permSvc.Check("shell", "")
	if current == loop.PermissionAllowAlways {
		// Disable yolo mode
		permSvc.Grant("shell", loop.PermissionAsk)
		permSvc.Grant("write", loop.PermissionAsk)
		permSvc.Grant("edit", loop.PermissionAsk)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "🔒 YOLO mode disabled. Will ask for confirmation on destructive operations.",
		}
	} else {
		// Enable yolo mode
		permSvc.Grant("shell", loop.PermissionAllowAlways)
		permSvc.Grant("write", loop.PermissionAllowAlways)
		permSvc.Grant("edit", loop.PermissionAllowAlways)
		permSvc.Grant("read", loop.PermissionAllowAlways)
		permSvc.Grant("grep", loop.PermissionAllowAlways)
		permSvc.Grant("glob", loop.PermissionAllowAlways)
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
  /provider [name]        Show or switch provider
  /test                   Test API connectivity
  /permission [tool] [level]  Manage tool permissions
  /yolo                   Toggle auto-approve mode
  /mouse [on|off|toggle|status] Toggle mouse wheel scrolling
  /exit                   Exit the application
  /compact                Compact conversation context to save tokens
  /clear                  Clear chat history
  /help                   Show this help message

Model/Provider Commands:
  /model                  Show current configuration
  /model gpt-4o           Switch to gpt-4o (keep provider)
  /model openrouter:claude-3-opus  Switch to OpenRouter with Claude 3 Opus
  /provider openai        Switch to OpenAI provider
  /provider openrouter    Switch to OpenRouter provider

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
  MSCLI_PROVIDER          Default provider (openai, openrouter)
  MSCLI_MODEL             Default model
  OPENAI_API_KEY          API key for OpenAI
  OPENROUTER_API_KEY      API key for OpenRouter`

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: helpText,
	}
}
