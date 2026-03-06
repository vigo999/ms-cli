package main

import (
	"fmt"
	"time"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/configs"
	permEngine "github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/tools/registry"
	"github.com/vigo999/ms-cli/trace"
	"github.com/vigo999/ms-cli/ui/model"
)

const Version = "ms-cli v0.2.0"

// Application is the top-level composition container.
type Application struct {
	Engine       *loop.Engine
	EventCh      chan model.Event
	Demo         bool
	WorkDir      string
	RepoURL      string
	Config       *configs.Config
	toolRegistry registry.Registry
	ctxManager   *context.Manager
	permEngine   permEngine.Engine
	stateManager *configs.StateManager
	traceWriter  trace.Writer
}

// SetProvider updates model/key and reinitializes the engine.
// providerName is kept for command compatibility and only accepts "openai".
func (a *Application) SetProvider(providerName, modelName, apiKey string) error {
	if providerName != "" && providerName != "openai" {
		return fmt.Errorf("unsupported provider: %s (only openai-compatible is supported)", providerName)
	}

	// Update config
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
	newEngine.SetContextManager(a.ctxManager)
	newEngine.SetTraceWriter(a.traceWriter)
	newEngine.SetPermissionEngine(a.permEngine)

	// Replace the engine
	a.Engine = newEngine

	// Save state to disk
	if a.stateManager != nil {
		a.stateManager.SaveFromConfig(a.Config)
		if err := a.stateManager.Save(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	return nil
}

// SaveState saves current configuration to persistent state.
func (a *Application) SaveState() error {
	if a.stateManager == nil {
		return nil
	}
	a.stateManager.SaveFromConfig(a.Config)
	return a.stateManager.Save()
}
