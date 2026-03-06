package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/ui/model"
)

const Version = "ms-cli v0.2.0"

// Application is the top-level composition container.
type Application struct {
	Engine            *loop.Engine
	EventCh           chan model.Event
	Demo              bool
	WorkDir           string
	RepoURL           string
	Config            *configs.Config
	toolRegistry      *tools.Registry
	ctxManager        *context.Manager
	permService       permission.PermissionService
	stateManager      *configs.StateManager
	traceWriter       traceWriter
	sessionManager    *session.Manager
	currentSessionID  session.ID
	initialUIMessages []model.Message
}

type traceWriter interface {
	Write(eventType string, payload any) error
	Path() string
}

// SetProvider updates provider/model/key and reinitializes the engine.
func (a *Application) SetProvider(providerName, modelName, apiKey string) error {
	currentProtocol := configs.NormalizeProtocol(a.Config.Model.Protocol)
	targetProtocol := currentProtocol
	if strings.TrimSpace(providerName) != "" {
		targetProtocol = configs.NormalizeProtocol(providerName)
		if !configs.IsSupportedProtocol(targetProtocol) {
			return fmt.Errorf("unsupported provider: %s (supported: %s, %s)", providerName, configs.ProtocolOpenAI, configs.ProtocolAnthropic)
		}
	}

	// If switching provider and URL is default/empty, move to new provider default endpoint.
	if targetProtocol != currentProtocol {
		currentURL := strings.TrimSpace(a.Config.Model.URL)
		if currentURL == "" || currentURL == defaultURLForProtocol(currentProtocol) {
			a.Config.Model.URL = defaultURLForProtocol(targetProtocol)
		}
	}

	// Update config
	a.Config.Model.Protocol = targetProtocol
	if modelName != "" {
		a.Config.Model.Model = modelName
	}
	if apiKey != "" {
		a.Config.Model.Key = apiKey
	}

	// Initialize new provider
	provider, err := initProvider(a.Config.Model)
	if err != nil {
		return fmt.Errorf("init provider: %w", err)
	}

	// Create new engine with the new provider but keep other settings
	engineCfg := loop.EngineConfig{
		MaxIterations:  10,
		MaxTokens:      a.Config.Budget.MaxTokens,
		Temperature:    float32(a.Config.Model.Temperature),
		TimeoutPerTurn: time.Duration(a.Config.Model.TimeoutSec) * time.Second,
	}
	newEngine := loop.NewEngine(engineCfg, provider, a.toolRegistry)
	a.attachEngineHooks(newEngine)

	// Replace the engine
	a.Engine = newEngine

	// Save state to disk
	if a.stateManager != nil {
		a.stateManager.SaveFromConfig(a.Config)
		if err := a.stateManager.Save(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	if err := a.syncSessionRuntime(); err != nil {
		return err
	}

	return nil
}

func (a *Application) attachEngineHooks(engine *loop.Engine) {
	if engine == nil {
		return
	}
	engine.SetContextManager(a.ctxManager)
	engine.SetPermissionService(a.permService)
	engine.SetTraceWriter(a.traceWriter)
	engine.SetMessageSink(func(msg llm.Message) error {
		if a.sessionManager == nil {
			return nil
		}
		return a.sessionManager.AddMessageToCurrent(msg)
	})
}

// SaveState saves current configuration to persistent state.
func (a *Application) SaveState() error {
	if a.stateManager == nil {
		return nil
	}
	a.stateManager.SaveFromConfig(a.Config)
	return a.stateManager.Save()
}

func (a *Application) syncSessionRuntime() error {
	if a.sessionManager == nil || a.Config == nil {
		return nil
	}

	snapshot := session.RuntimeSnapshot{
		Model: session.ModelSnapshot{
			Protocol:    a.Config.Model.Protocol,
			URL:         a.Config.Model.URL,
			Model:       a.Config.Model.Model,
			Temperature: a.Config.Model.Temperature,
			TimeoutSec:  a.Config.Model.TimeoutSec,
			MaxTokens:   a.Config.Model.MaxTokens,
		},
		Permission: collectPermissionSnapshot(a.permService),
		TracePath:  currentTracePath(a.traceWriter),
	}
	return a.sessionManager.UpdateCurrentRuntime(snapshot)
}

func collectPermissionSnapshot(ps permission.PermissionService) session.PermissionSnapshot {
	snapshot := session.PermissionSnapshot{
		ToolPolicies:    make(map[string]string),
		CommandPolicies: make(map[string]string),
		PathPolicies:    make([]session.PathPolicySnapshot, 0),
	}

	def, ok := ps.(*permission.DefaultPermissionService)
	if !ok {
		return snapshot
	}

	for tool, level := range def.GetPolicies() {
		snapshot.ToolPolicies[tool] = level.String()
	}
	for cmd, level := range def.GetCommandPolicies() {
		snapshot.CommandPolicies[cmd] = level.String()
	}
	for _, item := range def.GetPathPolicies() {
		snapshot.PathPolicies = append(snapshot.PathPolicies, session.PathPolicySnapshot{
			Pattern: item.Pattern,
			Level:   item.Level.String(),
		})
	}
	return snapshot
}

func currentTracePath(w traceWriter) string {
	if w == nil {
		return ""
	}
	return w.Path()
}
