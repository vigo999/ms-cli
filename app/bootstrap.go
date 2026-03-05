package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/integrations/domain"
	"github.com/vigo999/ms-cli/tools/fs"
	"github.com/vigo999/ms-cli/tools/shell"
	"github.com/vigo999/ms-cli/trace"
	"github.com/vigo999/ms-cli/ui/model"
)

// Bootstrap wires top-level dependencies.
func Bootstrap(demo bool) (*Application, error) {
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	workDir, _ = filepath.Abs(workDir)

	cfg, err := LoadConfig(defaultConfigPath)
	if err != nil {
		return nil, err
	}

	sessionPath := ResolveSessionStatePath(workDir)
	sessionState, err := LoadPersistentState(sessionPath)
	if err != nil {
		return nil, err
	}

	sessionModel := cfg.ResolveModel("", "")
	if !hasModelEnvOverride() && strings.TrimSpace(sessionState.Model.Provider) != "" {
		sessionModel = cfg.ResolveModel(sessionState.Model.Provider, sessionState.Model.Name)
	}

	openAIKey := strings.TrimSpace(os.Getenv(cfg.Providers.OpenAI.APIKeyEnv))
	if openAIKey == "" && cfg.Session.PersistAPIKeys {
		openAIKey = strings.TrimSpace(sessionState.APIKeys.OpenAI)
	}
	openRouterKey := strings.TrimSpace(os.Getenv(cfg.Providers.OpenRouter.APIKeyEnv))
	if openRouterKey == "" && cfg.Session.PersistAPIKeys {
		openRouterKey = strings.TrimSpace(sessionState.APIKeys.OpenRouter)
	}

	modelFactory := domain.NewFactory(domain.FactoryConfig{
		Providers: map[string]domain.ProviderConfig{
			"openai": {
				Endpoint:  cfg.Providers.OpenAI.Endpoint,
				BaseURL:   cfg.Providers.OpenAI.BaseURL,
				APIKeyEnv: cfg.Providers.OpenAI.APIKeyEnv,
				APIKey:    openAIKey,
			},
			"openrouter": {
				Endpoint:  cfg.Providers.OpenRouter.Endpoint,
				BaseURL:   cfg.Providers.OpenRouter.BaseURL,
				APIKeyEnv: cfg.Providers.OpenRouter.APIKeyEnv,
				APIKey:    openRouterKey,
			},
		},
	})

	var writer trace.Writer = trace.NewNoopWriter()
	if cfg.Trace.Enabled {
		traceWriter, traceErr := trace.NewJSONLWriter(cfg.ResolveTracePath(workDir))
		if traceErr != nil {
			return nil, traceErr
		}
		writer = traceWriter
	}

	permissionSvc := loop.NewPermissionManager(cfg.Permissions.SkipRequests, cfg.Permissions.AllowedTools)

	engine := loop.NewEngine(loop.Config{
		FS:                    fs.NewTool(workDir),
		Shell:                 shell.NewTool(workDir, cfg.ShellTimeout()),
		ModelFactory:          modelFactory,
		Permission:            permissionSvc,
		Trace:                 writer,
		DefaultMaxStep:        cfg.Engine.MaxSteps,
		MaxOutputLines:        cfg.Engine.MaxOutputLines,
		ContextMaxTokens:      cfg.ResolveContextBudget(sessionModel.Provider, sessionModel.Name),
		ContextCompactionRate: cfg.Context.CompactionRatio,
		ContextMaxEntries:     cfg.Memory.MaxItems,
		MaxRepeatedShell:      cfg.Engine.MaxRepeatedShell,
		MaxWallTimeSec:        cfg.Engine.MaxWallTimeSec,
		MaxTotalTokens:        cfg.Budget.MaxTokenLimit(),
		MaxTotalCostUSD:       cfg.Budget.MaxCost,
		RequireApprovalBlock:  cfg.Permissions.RequireApprovalBlock,
	})

	sessionState.Model = sessionModel
	if cfg.Session.PersistAPIKeys {
		if openAIKey != "" {
			sessionState.APIKeys.OpenAI = openAIKey
		}
		if openRouterKey != "" {
			sessionState.APIKeys.OpenRouter = openRouterKey
		}
	} else {
		sessionState.APIKeys = PersistedAPIKeys{}
	}
	if saveErr := SavePersistentState(sessionPath, sessionState); saveErr != nil {
		return nil, saveErr
	}

	return &Application{
		Engine:       engine,
		Permission:   permissionSvc,
		EventCh:      make(chan model.Event, 64),
		Demo:         demo,
		WorkDir:      workDir,
		RepoURL:      "github.com/vigo999/ms-cli",
		Config:       cfg,
		SessionModel: sessionModel,
		SessionPath:  sessionPath,
		SessionState: sessionState,
	}, nil
}
