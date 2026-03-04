package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/tools"
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
	toolRegistry *tools.Registry
	ctxManager   *context.Manager
	permService  loop.PermissionService
	stateManager *configs.StateManager
	traceWriter  trace.Writer
}

// SetProvider creates a new LLM provider and reinitializes the engine with the new provider.
func (a *Application) SetProvider(providerName, modelName, apiKey string) error {
	// Update config
	if providerName != "" {
		a.Config.Model.Provider = providerName
	}
	if modelName != "" {
		a.Config.Model.Model = modelName
	}
	if apiKey != "" {
		a.Config.Model.APIKey = apiKey
	}

	// Validate provider
	if a.Config.Model.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	// Fix endpoint if it doesn't match the provider
	a.fixEndpointForProvider()

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
	newEngine.SetPermissionService(a.permService)
	newEngine.SetTraceWriter(a.traceWriter)

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

// fixEndpointForProvider ensures the endpoint matches the selected provider.
func (a *Application) fixEndpointForProvider() {
	endpoint := a.Config.Model.Endpoint
	provider := a.Config.Model.Provider

	switch provider {
	case "openai":
		// If endpoint is empty or contains openrouter, reset to OpenAI default
		if endpoint == "" || strings.Contains(endpoint, "openrouter.ai") {
			a.Config.Model.Endpoint = "https://api.openai.com/v1"
		}
	case "openrouter":
		// If endpoint is empty or contains openai, reset to OpenRouter default
		if endpoint == "" || strings.Contains(endpoint, "openai.com") {
			a.Config.Model.Endpoint = "https://openrouter.ai/api/v1"
		}
	}
}
