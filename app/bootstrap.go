package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/executor"
	"github.com/vigo999/ms-cli/integrations/llm"
	openai "github.com/vigo999/ms-cli/integrations/llm/openai"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/fs"
	"github.com/vigo999/ms-cli/tools/shell"
	"github.com/vigo999/ms-cli/trace"
	"github.com/vigo999/ms-cli/ui/model"
)

// BootstrapConfig holds bootstrap configuration.
type BootstrapConfig struct {
	Demo       bool
	ConfigPath string
	URL        string // Override API URL from config
	Model      string // Override model from config
	Key        string // Override API key from config
}

// Bootstrap wires top-level dependencies.
func Bootstrap(cfg BootstrapConfig) (*Application, error) {
	// Find config file if not specified
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = configs.FindConfigFile()
	}

	// Load configuration
	config, err := configs.LoadWithEnv(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}
	workDir, _ = filepath.Abs(workDir)

	// Load saved state and apply to config (before command-line overrides)
	stateManager := configs.NewStateManager(workDir)
	if err := stateManager.Load(); err != nil {
		// Log but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to load state: %v\n", err)
	}
	stateManager.ApplyToConfig(config)

	// Keep ENV precedence even when state exists.
	configs.ApplyEnvOverrides(config)

	// Apply command-line overrides (highest priority)
	if cfg.URL != "" {
		config.Model.URL = cfg.URL
	}
	if cfg.Model != "" {
		config.Model.Model = cfg.Model
	}
	if cfg.Key != "" {
		config.Model.Key = cfg.Key
	}

	// In demo mode, use stub engine
	if cfg.Demo {
		loop.SetExecutorRun(executor.Run)
		engine := loop.NewEngine(loop.EngineConfig{}, nil, nil)
		return &Application{
			Engine:  engine,
			EventCh: make(chan model.Event, 64),
			Demo:    true,
			WorkDir: workDir,
			RepoURL: "github.com/vigo999/ms-cli",
			Config:  config,
		}, nil
	}

	// Initialize LLM provider
	provider, err := initProvider(config.Model)
	if err != nil {
		return nil, fmt.Errorf("init provider: %w", err)
	}

	// Initialize tool registry
	toolRegistry := initTools(config, workDir)

	// Initialize context manager
	ctxManager := context.NewManager(context.ManagerConfig{
		MaxTokens:           config.Context.MaxTokens,
		ReserveTokens:       config.Context.ReserveTokens,
		CompactionThreshold: config.Context.CompactionThreshold,
		MaxHistoryRounds:    config.Context.MaxHistoryRounds,
	})

	// Initialize per-session trajectory writer.
	traceWriter, err := trace.NewTimestampWriter(filepath.Join(workDir, ".cache"))
	if err != nil {
		return nil, fmt.Errorf("init trace writer: %w", err)
	}

	// Initialize engine
	// MaxIterations = 0 means no limit (user can interrupt with Ctrl+C)
	engineCfg := loop.EngineConfig{
		MaxIterations:  0, // Unlimited iterations
		MaxTokens:      config.Budget.MaxTokens,
		Temperature:    float32(config.Model.Temperature),
		TimeoutPerTurn: time.Duration(config.Model.TimeoutSec) * time.Second,
	}
	engine := loop.NewEngine(engineCfg, provider, toolRegistry)
	engine.SetContextManager(ctxManager)
	engine.SetTraceWriter(traceWriter)

	// Initialize permission service (default allow for now)
	permService := loop.NewDefaultPermissionService(config.Permissions)
	engine.SetPermissionService(permService)

	return &Application{
		Engine:       engine,
		EventCh:      make(chan model.Event, 64),
		Demo:         false,
		WorkDir:      workDir,
		RepoURL:      "github.com/vigo999/ms-cli",
		Config:       config,
		toolRegistry: toolRegistry,
		ctxManager:   ctxManager,
		permService:  permService,
		stateManager: stateManager,
		traceWriter:  traceWriter,
	}, nil
}

// initProvider initializes the LLM provider.
func initProvider(cfg configs.ModelConfig) (llm.Provider, error) {
	key := strings.TrimSpace(cfg.Key)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("MSCLI_API_KEY"))
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if key == "" {
		return nil, fmt.Errorf("API key not found (set MSCLI_API_KEY/OPENAI_API_KEY or key in config)")
	}

	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		url = "https://api.openai.com/v1"
	}

	client, err := openai.NewClient(openai.Config{
		Key:     key,
		URL:     url,
		Model:   cfg.Model,
		Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// initTools initializes the tool registry.
func initTools(cfg *configs.Config, workDir string) *tools.Registry {
	registry := tools.NewRegistry()

	// Register file tools
	registry.MustRegister(fs.NewReadTool(workDir))
	registry.MustRegister(fs.NewWriteTool(workDir))
	registry.MustRegister(fs.NewEditTool(workDir))
	registry.MustRegister(fs.NewGrepTool(workDir))
	registry.MustRegister(fs.NewGlobTool(workDir))

	// Register shell tool
	shellRunner := shell.NewRunner(shell.Config{
		WorkDir:        workDir,
		Timeout:        time.Duration(cfg.Execution.TimeoutSec) * time.Second,
		AllowedCmds:    cfg.Permissions.AllowedTools,
		BlockedCmds:    cfg.Permissions.BlockedTools,
		RequireConfirm: []string{"rm", "mv", "cp"},
	})
	registry.MustRegister(shell.NewShellTool(shellRunner))

	return registry
}
