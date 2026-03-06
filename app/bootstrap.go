package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/agent/session"
	"github.com/vigo999/ms-cli/configs"
	"github.com/vigo999/ms-cli/executor"
	"github.com/vigo999/ms-cli/integrations/llm"
	anthropic "github.com/vigo999/ms-cli/integrations/llm/anthropic"
	openai "github.com/vigo999/ms-cli/integrations/llm/openai"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/fs"
	"github.com/vigo999/ms-cli/tools/shell"
	"github.com/vigo999/ms-cli/ui/model"
)

// BootstrapConfig holds bootstrap configuration.
type BootstrapConfig struct {
	Demo            bool
	ConfigPath      string
	URL             string // Override API URL from config
	Model           string // Override model from config
	Key             string // Override API key from config
	ResumeSessionID string
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

	// Session manager is always initialized.
	sessionStorePath := filepath.Join(workDir, ".mscli", "sessions")
	store, err := session.NewFileStore(sessionStorePath)
	if err != nil {
		return nil, fmt.Errorf("init session store: %w", err)
	}
	sessionCfg := session.DefaultConfig()
	sessionCfg.StorePath = sessionStorePath
	sessionCfg.AutoSave = true
	sessionManager := session.NewManager(store, sessionCfg)

	var (
		currentSession *session.Session
		initialUI      []model.Message
	)
	if strings.TrimSpace(cfg.ResumeSessionID) != "" {
		currentSession, err = sessionManager.Load(session.ID(cfg.ResumeSessionID))
		if err != nil {
			return nil, fmt.Errorf("resume session %q: %w", cfg.ResumeSessionID, err)
		}
		applyModelSnapshot(config, currentSession.Runtime.Model)
		initialUI = sessionMessagesToUI(currentSession.Messages)
	} else {
		currentSession, err = sessionManager.CreateAndSetCurrent("", workDir)
		if err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
	}

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
	config.Model.Protocol = configs.NormalizeProtocol(config.Model.Protocol)
	if strings.TrimSpace(config.Model.URL) == "" {
		config.Model.URL = defaultURLForProtocol(config.Model.Protocol)
	}

	// In demo mode, use stub engine
	if cfg.Demo {
		loop.SetExecutorRun(executor.Run)
		engine := loop.NewEngine(loop.EngineConfig{}, nil, nil)
		app := &Application{
			Engine:            engine,
			EventCh:           make(chan model.Event, 64),
			Demo:              true,
			WorkDir:           workDir,
			RepoURL:           "github.com/vigo999/ms-cli",
			Config:            config,
			stateManager:      stateManager,
			sessionManager:    sessionManager,
			currentSessionID:  currentSession.ID,
			initialUIMessages: initialUI,
		}
		return app, nil
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
	if strings.TrimSpace(cfg.ResumeSessionID) != "" {
		ctxManager.ReplaceMessages(currentSession.Messages)
	}

	// Initialize per-session trajectory writer (fixed path by session ID).
	traceWriter, err := session.NewSessionTraceWriter(sessionStorePath, currentSession.ID)
	if err != nil {
		return nil, fmt.Errorf("init trace writer: %w", err)
	}

	// Initialize permission service and apply session snapshot if needed.
	permService := permission.NewDefaultPermissionService(config.Permissions)
	if strings.TrimSpace(cfg.ResumeSessionID) != "" {
		applyPermissionSnapshot(permService, currentSession.Runtime.Permission)
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

	app := &Application{
		Engine:            engine,
		EventCh:           make(chan model.Event, 64),
		Demo:              false,
		WorkDir:           workDir,
		RepoURL:           "github.com/vigo999/ms-cli",
		Config:            config,
		toolRegistry:      toolRegistry,
		ctxManager:        ctxManager,
		permService:       permService,
		stateManager:      stateManager,
		traceWriter:       traceWriter,
		sessionManager:    sessionManager,
		currentSessionID:  currentSession.ID,
		initialUIMessages: initialUI,
	}
	app.attachEngineHooks(engine)

	if err := sessionManager.SetCurrentTracePath(traceWriter.Path()); err != nil {
		return nil, fmt.Errorf("set session trace path: %w", err)
	}
	if err := app.syncSessionRuntime(); err != nil {
		return nil, fmt.Errorf("sync session runtime: %w", err)
	}

	return app, nil
}

func applyModelSnapshot(cfg *configs.Config, snap session.ModelSnapshot) {
	if snap.Protocol != "" {
		cfg.Model.Protocol = configs.NormalizeProtocol(snap.Protocol)
	}
	if snap.URL != "" {
		cfg.Model.URL = snap.URL
	}
	if snap.Model != "" {
		cfg.Model.Model = snap.Model
	}
	if snap.Temperature != 0 {
		cfg.Model.Temperature = snap.Temperature
	}
	if snap.TimeoutSec > 0 {
		cfg.Model.TimeoutSec = snap.TimeoutSec
	}
	if snap.MaxTokens > 0 {
		cfg.Model.MaxTokens = snap.MaxTokens
	}
}

func applyPermissionSnapshot(svc *permission.DefaultPermissionService, snap session.PermissionSnapshot) {
	for tool, level := range snap.ToolPolicies {
		svc.Grant(tool, permission.ParsePermissionLevel(level))
	}
	for cmd, level := range snap.CommandPolicies {
		svc.GrantCommand(cmd, permission.ParsePermissionLevel(level))
	}
	for _, item := range snap.PathPolicies {
		svc.GrantPath(item.Pattern, permission.ParsePermissionLevel(item.Level))
	}
}

func sessionMessagesToUI(messages []llm.Message) []model.Message {
	out := make([]model.Message, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			out = append(out, model.Message{Kind: model.MsgUser, Content: msg.Content})
		case "assistant":
			out = append(out, model.Message{Kind: model.MsgAgent, Content: msg.Content})
		case "tool":
			out = append(out, model.Message{
				Kind:     model.MsgTool,
				ToolName: "Tool",
				Display:  model.DisplayExpanded,
				Content:  msg.Content,
			})
		}
	}
	return out
}

// initProvider initializes the LLM provider.
func initProvider(cfg configs.ModelConfig) (llm.Provider, error) {
	protocol := configs.NormalizeProtocol(cfg.Protocol)
	if !configs.IsSupportedProtocol(protocol) {
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}

	key := strings.TrimSpace(cfg.Key)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("MSCLI_API_KEY"))
	}
	if key == "" {
		switch protocol {
		case configs.ProtocolAnthropic:
			key = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		default:
			key = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
	}

	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		url = defaultURLForProtocol(protocol)
	}

	if key == "" {
		return newUnconfiguredProvider(protocol, cfg.Model), nil
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	switch protocol {
	case configs.ProtocolAnthropic:
		client, err := anthropic.NewClient(anthropic.Config{
			Key:       key,
			URL:       url,
			Model:     cfg.Model,
			MaxTokens: cfg.MaxTokens,
			Timeout:   timeout,
		})
		if err != nil {
			return nil, err
		}
		return client, nil
	default:
		client, err := openai.NewClient(openai.Config{
			Key:     key,
			URL:     url,
			Model:   cfg.Model,
			Timeout: timeout,
		})
		if err != nil {
			return nil, err
		}
		return client, nil
	}
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
