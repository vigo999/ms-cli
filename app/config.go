package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "configs/mscli.yaml"
)

// Config is the runtime configuration loaded from configs/mscli.yaml.
type Config struct {
	Model       ModelConfig       `yaml:"model"`
	Providers   ProvidersConfig   `yaml:"providers"`
	Budget      BudgetConfig      `yaml:"budget"`
	Permissions PermissionsConfig `yaml:"permissions"`
	Context     ContextConfig     `yaml:"context"`
	Memory      MemoryConfig      `yaml:"memory"`
	Engine      EngineConfig      `yaml:"engine"`
	Session     SessionConfig     `yaml:"session"`
	Trace       TraceConfig       `yaml:"trace"`
}

type ModelConfig struct {
	Provider        string `yaml:"provider"`
	Endpoint        string `yaml:"endpoint"`
	DefaultProvider string `yaml:"default_provider"`
	DefaultModel    string `yaml:"default_model"`
}

type ProvidersConfig struct {
	OpenAI     ProviderConfig `yaml:"openai"`
	OpenRouter ProviderConfig `yaml:"openrouter"`
}

type ProviderConfig struct {
	Endpoint  string `yaml:"endpoint"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}

type BudgetConfig struct {
	// MaxTokensM is the preferred token budget unit (millions of tokens).
	MaxTokensM float64 `yaml:"max_tokens_m"`
	// MaxTokens keeps backward compatibility with legacy absolute-token configs.
	MaxTokens int     `yaml:"max_tokens,omitempty"`
	MaxCost   float64 `yaml:"max_cost_usd"`
}

type PermissionsConfig struct {
	SkipRequests         bool     `yaml:"skip_requests"`
	AllowedTools         []string `yaml:"allowed_tools"`
	RequireApprovalBlock bool     `yaml:"require_approval_block"`
}

type ContextConfig struct {
	// MaxTokens is the planner-context budget. 0 means auto (derived from model window).
	MaxTokens int `yaml:"max_tokens"`
	// ModelWindowTokens controls top-bar context window and auto budget baseline.
	ModelWindowTokens int `yaml:"model_window_tokens"`
	// BudgetMarginTokens reserves tokens for system prompt / output when auto budgeting.
	BudgetMarginTokens int `yaml:"budget_margin_tokens"`
	// ModelWindows allows per-model overrides, key format: "<provider>/<model>".
	ModelWindows    map[string]int `yaml:"model_windows"`
	CompactionRatio float64        `yaml:"compaction_threshold"`
}

type MemoryConfig struct {
	MaxItems int `yaml:"max_items"`
	MaxBytes int `yaml:"max_bytes"`
	TTLHours int `yaml:"ttl_hours"`
}

type EngineConfig struct {
	MaxSteps         int `yaml:"max_steps"`
	ShellTimeout     int `yaml:"shell_timeout_sec"`
	MaxOutputLines   int `yaml:"max_output_lines"`
	MaxWallTimeSec   int `yaml:"max_wall_time_sec"`
	MaxRepeatedShell int `yaml:"max_repeated_shell"`
}

type SessionConfig struct {
	PersistAPIKeys bool `yaml:"persist_api_keys"`
}

type TraceConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func defaultConfig() Config {
	return Config{
		Model: ModelConfig{
			Provider:        "openai",
			Endpoint:        "https://api.openai.com/v1",
			DefaultProvider: "openai",
			DefaultModel:    "gpt-4o-mini",
		},
		Providers: ProvidersConfig{
			OpenAI: ProviderConfig{
				BaseURL:   "https://api.openai.com/v1",
				APIKeyEnv: "OPENAI_API_KEY",
			},
			OpenRouter: ProviderConfig{
				Endpoint:  "https://openrouter.ai/api/v1",
				APIKeyEnv: "OPENROUTER_API_KEY",
			},
		},
		Budget: BudgetConfig{
			MaxTokensM: 1,
			MaxCost:    10,
		},
		Permissions: PermissionsConfig{
			SkipRequests:         false,
			AllowedTools:         []string{},
			RequireApprovalBlock: true,
		},
		Context: ContextConfig{
			MaxTokens:          0,
			ModelWindowTokens:  163000,
			BudgetMarginTokens: 4096,
			ModelWindows:       map[string]int{},
			CompactionRatio:    0.85,
		},
		Memory: MemoryConfig{
			MaxItems: 200,
			MaxBytes: 2 * 1024 * 1024,
			TTLHours: 168,
		},
		Engine: EngineConfig{
			MaxSteps:         0, // 0 means unlimited; stop via Ctrl+C
			ShellTimeout:     120,
			MaxOutputLines:   200,
			MaxWallTimeSec:   1800,
			MaxRepeatedShell: 2,
		},
		Session: SessionConfig{
			PersistAPIKeys: false,
		},
		Trace: TraceConfig{
			Enabled: true,
			Path:    ".cache/mscli/trace.jsonl",
		},
	}
}

// LoadConfig reads ms-cli runtime config and applies environment overrides.
func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	if strings.TrimSpace(path) == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg.applyBackwardCompatibility()
	cfg.applyEnvOverrides()
	cfg.applySafeDefaults()

	return cfg, nil
}

func (c *Config) applyBackwardCompatibility() {
	if c.Model.DefaultProvider == "" && c.Model.Provider != "" {
		c.Model.DefaultProvider = c.Model.Provider
	}
	if c.Model.DefaultProvider == "" {
		c.Model.DefaultProvider = "openai"
	}
	if c.Model.Endpoint != "" {
		switch strings.ToLower(c.Model.DefaultProvider) {
		case "openrouter":
			if c.Providers.OpenRouter.Endpoint == "" {
				c.Providers.OpenRouter.Endpoint = c.Model.Endpoint
			}
		default:
			if c.Providers.OpenAI.Endpoint == "" && c.Providers.OpenAI.BaseURL == "" {
				c.Providers.OpenAI.Endpoint = c.Model.Endpoint
			}
		}
	}
}

func (c *Config) applyEnvOverrides() {
	if v := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); v != "" {
		c.Providers.OpenAI.BaseURL = v
		c.Providers.OpenAI.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("MSCLI_MODEL_PROVIDER")); v != "" {
		c.Model.DefaultProvider = strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("MSCLI_MODEL_NAME")); v != "" {
		c.Model.DefaultModel = v
	}
	if v := strings.TrimSpace(os.Getenv("MSCLI_MODEL_ENDPOINT")); v != "" {
		switch strings.ToLower(c.Model.DefaultProvider) {
		case "openrouter":
			c.Providers.OpenRouter.Endpoint = v
		default:
			c.Providers.OpenAI.Endpoint = v
		}
	}
}

func (c *Config) applySafeDefaults() {
	if c.Providers.OpenAI.Endpoint == "" && c.Providers.OpenAI.BaseURL != "" {
		c.Providers.OpenAI.Endpoint = c.Providers.OpenAI.BaseURL
	}
	if c.Context.MaxTokens < 0 {
		c.Context.MaxTokens = 0
	}
	if c.Context.ModelWindowTokens <= 0 {
		c.Context.ModelWindowTokens = 163000
	}
	if c.Context.BudgetMarginTokens < 0 {
		c.Context.BudgetMarginTokens = 0
	}
	if c.Context.ModelWindows == nil {
		c.Context.ModelWindows = map[string]int{}
	}

	if c.Budget.MaxTokens < 0 {
		c.Budget.MaxTokens = 0
	}
	if c.Budget.MaxTokensM < 0 {
		c.Budget.MaxTokensM = 0
	}
	// Default to 1M tokens when no budget is provided.
	if c.Budget.MaxTokens <= 0 && c.Budget.MaxTokensM <= 0 {
		c.Budget.MaxTokensM = 1
	}

	if c.Providers.OpenAI.Endpoint == "" {
		c.Providers.OpenAI.Endpoint = "https://api.openai.com/v1"
	}
	if c.Providers.OpenAI.BaseURL == "" {
		c.Providers.OpenAI.BaseURL = c.Providers.OpenAI.Endpoint
	}
	if c.Providers.OpenAI.APIKeyEnv == "" {
		c.Providers.OpenAI.APIKeyEnv = "OPENAI_API_KEY"
	}
	if c.Providers.OpenRouter.Endpoint == "" {
		c.Providers.OpenRouter.Endpoint = "https://openrouter.ai/api/v1"
	}
	if c.Providers.OpenRouter.APIKeyEnv == "" {
		c.Providers.OpenRouter.APIKeyEnv = "OPENROUTER_API_KEY"
	}
	if c.Model.DefaultProvider == "" {
		c.Model.DefaultProvider = "openai"
	}
	if c.Model.DefaultModel == "" {
		c.Model.DefaultModel = "gpt-4o-mini"
	}
	// MaxSteps: 0 means unlimited; only negative values are normalized.
	if c.Engine.MaxSteps < 0 {
		c.Engine.MaxSteps = 0
	}
	if c.Engine.ShellTimeout <= 0 {
		c.Engine.ShellTimeout = 120
	}
	if c.Engine.MaxOutputLines <= 0 {
		c.Engine.MaxOutputLines = 200
	}
	if c.Engine.MaxWallTimeSec <= 0 {
		c.Engine.MaxWallTimeSec = 1800
	}
	if c.Engine.MaxRepeatedShell <= 0 {
		c.Engine.MaxRepeatedShell = 2
	}
	if c.Trace.Path == "" {
		c.Trace.Path = ".cache/mscli/trace.jsonl"
	}
}

func (b BudgetConfig) MaxTokenLimit() int {
	if b.MaxTokensM > 0 {
		return int(b.MaxTokensM * 1_000_000)
	}
	if b.MaxTokens > 0 {
		return b.MaxTokens
	}
	return 0
}

func (c Config) ResolveModel(provider, modelName string) SessionModel {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = strings.ToLower(c.Model.DefaultProvider)
	}
	m := strings.TrimSpace(modelName)
	if m == "" {
		m = c.Model.DefaultModel
	}

	switch p {
	case "openrouter":
		return SessionModel{Provider: "openrouter", Name: m, Endpoint: c.Providers.OpenRouter.Endpoint}
	default:
		return SessionModel{Provider: "openai", Name: m, Endpoint: c.Providers.OpenAI.Endpoint}
	}
}

func (c Config) ResolveContextWindow(provider, modelName string) int {
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.ToLower(strings.TrimSpace(modelName))
	if p != "" && m != "" {
		key := p + "/" + m
		if v, ok := c.Context.ModelWindows[key]; ok && v > 0 {
			return v
		}
	}
	return c.Context.ModelWindowTokens
}

func (c Config) ResolveContextBudget(provider, modelName string) int {
	window := c.ResolveContextWindow(provider, modelName)
	if c.Context.MaxTokens > 0 {
		if window > 0 && c.Context.MaxTokens > window {
			return window
		}
		return c.Context.MaxTokens
	}
	margin := c.Context.BudgetMarginTokens
	if margin < 0 {
		margin = 0
	}
	budget := window - margin
	if budget <= 0 {
		return window
	}
	return budget
}

func (c Config) ShellTimeout() time.Duration {
	return time.Duration(c.Engine.ShellTimeout) * time.Second
}

func (c Config) ResolveTracePath(workDir string) string {
	p := strings.TrimSpace(c.Trace.Path)
	if p == "" {
		p = ".cache/mscli/trace.jsonl"
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(workDir, p)
}
