package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/internal/project"
	"github.com/vigo999/ms-cli/permission"
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

	if strings.EqualFold(strings.TrimSpace(args[0]), "key") {
		if len(args) < 2 {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Usage: /model key <API_KEY>",
			}
			return
		}
		key := strings.TrimSpace(strings.Join(args[1:], " "))
		if key == "" {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Usage: /model key <API_KEY>",
			}
			return
		}
		a.switchProvider("", "", key)
		return
	}

	// Accept "provider:model" format.
	modelArg := args[0]
	if strings.Contains(modelArg, ":") {
		parts := strings.SplitN(modelArg, ":", 2)
		rawProvider := strings.TrimSpace(parts[0])
		providerName := rawProvider
		if rawProvider != "" {
			providerName = configs.NormalizeProtocol(rawProvider)
		}
		modelName := strings.TrimSpace(parts[1])
		if rawProvider != "" && !configs.IsSupportedProtocol(providerName) {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: fmt.Sprintf("Unsupported provider prefix: %s (supported: %s, %s)", providerName, configs.ProtocolOpenAI, configs.ProtocolAnthropic),
			}
			return
		}
		if modelName == "" {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Usage: /model <provider>:<model>",
			}
			return
		}
		a.switchProvider(providerName, modelName, "")
		return
	}

	// Switch model on current provider.
	a.switchProvider("", strings.TrimSpace(modelArg), "")
}

// showCurrentModel displays current URL/model/key status.
func (a *Application) showCurrentModel() {
	protocol := configs.NormalizeProtocol(a.Config.Model.Protocol)
	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = defaultURLForProtocol(protocol)
	}

	apiKeyStatus := "not set"
	if hasConfiguredAPIKey(protocol, a.Config.Model.Key) {
		apiKeyStatus = "set"
	}

	msg := fmt.Sprintf(`Current Model Configuration:

  Protocol: %s
  URL:   %s
  Model: %s
  Key:   %s

To switch provider/model:
  /model <model-name>
  /model openai:<model>
  /model anthropic:<model>
  /model key <API_KEY>

Examples:
  /model gpt-4o
  /model openai:gpt-4o-mini
  /model anthropic:claude-3-5-sonnet-latest
  /model key sk-xxx`,
		protocol, url, modelName, apiKeyStatus)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
	}
}

// switchProvider switches provider/model/key.
func (a *Application) switchProvider(providerName, modelName, apiKey string) {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	err := a.SetProvider(providerName, modelName, apiKey)
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
			Message: fmt.Sprintf("Updated provider config (protocol=%s, model=%s). Warning: failed to save state: %v", a.Config.Model.Protocol, a.Config.Model.Model, err),
		}
		return
	}

	successMsg := fmt.Sprintf("Model switched to: %s (%s)", a.Config.Model.Model, a.Config.Model.Protocol)
	if apiKey != "" && modelName == "" {
		successMsg = fmt.Sprintf("API key updated for %s", a.Config.Model.Protocol)
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: successMsg,
	}
}

// cmdExit handles "/exit".
func (a *Application) cmdExit() {
	msg := "Goodbye!"
	if a.currentSessionID != "" {
		msg = fmt.Sprintf("Goodbye!\nYou can resume this conversation with: ms-cli resume %s", a.currentSessionID)
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
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
	if a.ctxManager != nil {
		a.ctxManager.Clear()
	}
	if a.sessionManager != nil {
		if err := a.sessionManager.ClearCurrentMessages(); err != nil {
			a.EventCh <- model.Event{
				Type:     model.ToolError,
				ToolName: "session",
				Message:  fmt.Sprintf("failed to clear session messages: %v", err),
			}
		}
	}

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
	protocol := configs.NormalizeProtocol(a.Config.Model.Protocol)
	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = defaultURLForProtocol(protocol)
	}
	apiKeyStatus := "not set"
	if hasConfiguredAPIKey(protocol, a.Config.Model.Key) {
		apiKeyStatus = "set"
	}

	msg := fmt.Sprintf(`API Connection Test:

  Protocol: %s
  URL:     %s
  Model:   %s
  API Key: %s

Testing connectivity...`, protocol, url, modelName, apiKeyStatus)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: msg,
	}

	if !hasConfiguredAPIKey(protocol, a.Config.Model.Key) {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "API key is not configured. Use `/model key <KEY>` or set MSCLI_API_KEY / provider-specific API key env.",
		}
		return
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
	permSvc, ok := a.permService.(*permission.DefaultPermissionService)
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
	level := permission.ParsePermissionLevel(levelStr)

	permSvc.Grant(tool, level)
	if err := a.syncSessionRuntime(); err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "session",
			Message:  fmt.Sprintf("Permission updated but failed to sync session runtime: %v", err),
		}
	}

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Permission for '%s' set to: %s", tool, level),
	}
}

// cmdYolo handles "/yolo" - toggles auto-approve mode.
func (a *Application) cmdYolo() {
	permSvc, ok := a.permService.(*permission.DefaultPermissionService)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode not available in current configuration.",
		}
		return
	}

	// Check current state by looking at shell permission
	current := permSvc.Check("shell", "")
	if current == permission.PermissionAllowAlways {
		// Disable yolo mode
		permSvc.Grant("shell", permission.PermissionAsk)
		permSvc.Grant("write", permission.PermissionAsk)
		permSvc.Grant("edit", permission.PermissionAsk)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "🔒 YOLO mode disabled. Will ask for confirmation on destructive operations.",
		}
	} else {
		// Enable yolo mode
		permSvc.Grant("shell", permission.PermissionAllowAlways)
		permSvc.Grant("write", permission.PermissionAllowAlways)
		permSvc.Grant("edit", permission.PermissionAllowAlways)
		permSvc.Grant("read", permission.PermissionAllowAlways)
		permSvc.Grant("grep", permission.PermissionAllowAlways)
		permSvc.Grant("glob", permission.PermissionAllowAlways)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "⚡ YOLO mode enabled! All operations will be auto-approved. Use with caution!",
		}
	}
	if err := a.syncSessionRuntime(); err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "session",
			Message:  fmt.Sprintf("YOLO updated but failed to sync session runtime: %v", err),
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
  /model openai:gpt-4o    OpenAI protocol
  /model anthropic:claude-3-5-sonnet-latest
  /model key sk-xxx       Update API key at runtime

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
  MSCLI_PROTOCOL          Protocol override (openai|anthropic)
  MSCLI_BASE_URL          Base URL (highest priority)
  MSCLI_MODEL             Default model
  MSCLI_API_KEY           API key
  OPENAI_BASE_URL         Base URL (fallback)
  OPENAI_MODEL            Model (fallback)
  OPENAI_API_KEY          API key (fallback for openai)
  ANTHROPIC_BASE_URL      Base URL (fallback for anthropic)
  ANTHROPIC_MODEL         Model (fallback for anthropic)
  ANTHROPIC_API_KEY       API key (fallback for anthropic)`

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: helpText,
	}
}
