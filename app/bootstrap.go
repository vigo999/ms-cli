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
	"github.com/vigo999/ms-cli/integrations/llm"
	openai "github.com/vigo999/ms-cli/integrations/llm/openai"
	permEngine "github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/tools/registry"
	"github.com/vigo999/ms-cli/tools/registry/builtin"
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

	// Create permission engine (new system)
	permConfig := buildPermissionConfig(config.Permissions, workDir)
	permEngine, err := permEngine.NewEngine(permConfig)
	if err != nil {
		return nil, fmt.Errorf("init permission engine: %w", err)
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
	engine.SetPermissionEngine(permEngine)

	return &Application{
		Engine:       engine,
		EventCh:      make(chan model.Event, 64),
		Demo:         false,
		WorkDir:      workDir,
		RepoURL:      "github.com/vigo999/ms-cli",
		Config:       config,
		toolRegistry: toolRegistry,
		ctxManager:   ctxManager,
		permEngine:   permEngine,
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
func initTools(cfg *configs.Config, workDir string) registry.Registry {
	reg := registry.NewRegistry()

	// Get all builtin tools and register them
	builtinTools := builtin.GetAllTools()
	for _, tool := range builtinTools {
		reg.Register(tool)
	}

	return reg
}

// buildPermissionConfig converts legacy PermissionsConfig to new permission Config
func buildPermissionConfig(cfg configs.PermissionsConfig, workDir string) *permEngine.Config {
	ruleset := permEngine.NewRuleset()

	// Set default action
	switch cfg.DefaultLevel {
	case "deny":
		ruleset.SetDefaultAction(permEngine.ActionDeny)
	case "allow", "allow_always":
		ruleset.SetDefaultAction(permEngine.ActionAllow)
	default:
		ruleset.SetDefaultAction(permEngine.ActionAsk)
	}

	// Add tool-specific rules from ToolPolicies
	for tool, level := range cfg.ToolPolicies {
		action := permEngine.ActionAsk
		switch level {
		case "deny":
			action = permEngine.ActionDeny
		case "allow", "allow_always":
			action = permEngine.ActionAllow
		case "ask":
			action = permEngine.ActionAsk
		}
		// 使用权限名称（如 file:write）而非工具名称（如 write）
		permName := builtin.GetToolPermissionName(tool)
		if permName == tool+":execute" {
			// GetToolPermissionName 对未知工具返回 :execute，我们改为 :* 以保持向后兼容
			permName = tool + ":*"
		}
		ruleset.AddRule(permEngine.Rule{
			ID:         "tool-" + tool,
			Permission: permName,
			Action:     action,
			Enabled:    true,
		})
	}

	// Add allowed tools as allow rules
	for _, tool := range cfg.AllowedTools {
		permName := builtin.GetToolPermissionName(tool)
		if permName == tool+":execute" {
			permName = tool + ":*"
		}
		ruleset.AddRule(permEngine.Rule{
			ID:         "allowed-" + tool,
			Permission: permName,
			Action:     permEngine.ActionAllow,
			Enabled:    true,
		})
	}

	// Add blocked tools as deny rules
	for _, tool := range cfg.BlockedTools {
		permName := builtin.GetToolPermissionName(tool)
		if permName == tool+":execute" {
			permName = tool + ":*"
		}
		ruleset.AddRule(permEngine.Rule{
			ID:         "blocked-" + tool,
			Permission: permName,
			Action:     permEngine.ActionDeny,
			Enabled:    true,
		})
	}

	// Add dangerous tool defaults (write, edit, shell should ask by default)
	dangerousTools := []string{"write", "edit", "shell", "bash"}
	for _, tool := range dangerousTools {
		// Only add if not already specified
		if _, ok := cfg.ToolPolicies[tool]; !ok {
			permName := builtin.GetToolPermissionName(tool)
			if permName == tool+":execute" {
				permName = tool + ":*"
			}
			ruleset.AddRule(permEngine.Rule{
				ID:         "dangerous-" + tool,
				Permission: permName,
				Action:     permEngine.ActionAsk,
				Enabled:    true,
				Priority:   -1, // Lower priority than explicit rules
			})
		}
	}

	// Add default allow rules for read-only operations
	// file:read covers glob, read, and other read-only file operations
	ruleset.AddRule(permEngine.Rule{
		ID:          "default-file-read",
		Permission:  "file:read",
		Action:      permEngine.ActionAllow,
		Enabled:     true,
		Priority:    -2, // Lower priority than explicit rules
		Description: "Allow all file read operations by default",
	})

	config := permEngine.DefaultConfig()
	config.Ruleset = ruleset
	return config
}
