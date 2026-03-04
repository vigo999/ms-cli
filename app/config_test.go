package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mscli.yaml")
	content := `
model:
  default_provider: openai
  default_model: gpt-4o-mini
providers:
  openai:
    endpoint: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
  openrouter:
    endpoint: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MSCLI_MODEL_PROVIDER", "openrouter")
	t.Setenv("MSCLI_MODEL_NAME", "deepseek/deepseek-r1")
	t.Setenv("MSCLI_MODEL_ENDPOINT", "https://custom.openrouter/v1")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	model := cfg.ResolveModel("", "")
	if model.Provider != "openrouter" {
		t.Fatalf("provider=%s want openrouter", model.Provider)
	}
	if model.Name != "deepseek/deepseek-r1" {
		t.Fatalf("model=%s want deepseek/deepseek-r1", model.Name)
	}
	if model.Endpoint != "https://custom.openrouter/v1" {
		t.Fatalf("endpoint=%s want https://custom.openrouter/v1", model.Endpoint)
	}
}

func TestConfigMaxStepsZeroMeansUnlimited(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Engine.MaxSteps != 0 {
		t.Fatalf("default max_steps=%d want 0 (unlimited)", cfg.Engine.MaxSteps)
	}

	cfg.Engine.MaxSteps = 0
	cfg.applySafeDefaults()
	if cfg.Engine.MaxSteps != 0 {
		t.Fatalf("max_steps should stay 0 after defaults, got %d", cfg.Engine.MaxSteps)
	}
}

func TestLoadConfig_DefaultApprovalBlockEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mscli.yaml")
	content := `
model:
  default_provider: openai
  default_model: gpt-4o-mini
providers:
  openai:
    endpoint: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
  openrouter:
    endpoint: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !cfg.Permissions.RequireApprovalBlock {
		t.Fatalf("require_approval_block should default to true")
	}
	if cfg.Session.PersistAPIKeys {
		t.Fatalf("persist_api_keys should default to false")
	}
}

func TestBudgetMaxTokensM_DefaultIsOneMillion(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Budget.MaxTokenLimit() != 1_000_000 {
		t.Fatalf("default max token limit=%d want 1000000", cfg.Budget.MaxTokenLimit())
	}
}

func TestBudgetMaxTokens_BackwardCompatibility(t *testing.T) {
	cfg := defaultConfig()
	cfg.Budget.MaxTokensM = 0
	cfg.Budget.MaxTokens = 120000
	if cfg.Budget.MaxTokenLimit() != 120000 {
		t.Fatalf("legacy max_tokens should still work, got %d", cfg.Budget.MaxTokenLimit())
	}
}

func TestResolveContextWindowAndBudget(t *testing.T) {
	cfg := defaultConfig()
	if got := cfg.ResolveContextWindow("openrouter", "deepseek/deepseek-r1"); got != 163000 {
		t.Fatalf("default model window=%d want 163000", got)
	}
	if got := cfg.ResolveContextBudget("openrouter", "deepseek/deepseek-r1"); got != 158904 {
		t.Fatalf("default auto budget=%d want 158904", got)
	}

	cfg.Context.ModelWindows["openrouter/deepseek/deepseek-r1"] = 200000
	if got := cfg.ResolveContextWindow("openrouter", "deepseek/deepseek-r1"); got != 200000 {
		t.Fatalf("override model window=%d want 200000", got)
	}
	if got := cfg.ResolveContextBudget("openrouter", "deepseek/deepseek-r1"); got != 195904 {
		t.Fatalf("override auto budget=%d want 195904", got)
	}
}

func TestResolveContextBudget_ExplicitMaxTokens(t *testing.T) {
	cfg := defaultConfig()
	cfg.Context.MaxTokens = 10000
	if got := cfg.ResolveContextBudget("openrouter", "deepseek/deepseek-r1"); got != 10000 {
		t.Fatalf("explicit context.max_tokens should win, got %d", got)
	}

	cfg.Context.MaxTokens = 300000
	if got := cfg.ResolveContextBudget("openrouter", "deepseek/deepseek-r1"); got != 163000 {
		t.Fatalf("budget should be capped by window, got %d", got)
	}
}

func TestLoadConfig_OpenAIBaseURLFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mscli.yaml")
	content := `
model:
  default_provider: openai
  default_model: gpt-4o-mini
providers:
  openai:
    base_url: https://openai-proxy.example.com/v1
    api_key_env: OPENAI_API_KEY
  openrouter:
    endpoint: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	model := cfg.ResolveModel("openai", "gpt-4o-mini")
	if model.Endpoint != "https://openai-proxy.example.com/v1" {
		t.Fatalf("endpoint=%s want https://openai-proxy.example.com/v1", model.Endpoint)
	}
}

func TestLoadConfig_OpenAIBaseURLFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mscli.yaml")
	content := `
model:
  default_provider: openai
  default_model: gpt-4o-mini
providers:
  openai:
    endpoint: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
  openrouter:
    endpoint: https://openrouter.ai/api/v1
    api_key_env: OPENROUTER_API_KEY
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_BASE_URL", "https://openai-env.example.com/v1")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	model := cfg.ResolveModel("openai", "gpt-4o-mini")
	if model.Endpoint != "https://openai-env.example.com/v1" {
		t.Fatalf("endpoint=%s want https://openai-env.example.com/v1", model.Endpoint)
	}
}
