package configs

import "testing"

func TestApplyEnvOverridesMSCLIProtocolOverridesAutoDetect(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://openai.example/v1")
	t.Setenv("OPENAI_MODEL", "gpt-openai")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("ANTHROPIC_BASE_URL", "https://anthropic.example/v1")
	t.Setenv("ANTHROPIC_MODEL", "claude-x")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("MSCLI_PROTOCOL", "anthropic")

	cfg := DefaultConfig()
	ApplyEnvOverrides(cfg)

	if cfg.Model.Protocol != ProtocolAnthropic {
		t.Fatalf("protocol = %s, want %s", cfg.Model.Protocol, ProtocolAnthropic)
	}
	if cfg.Model.Model != "claude-x" {
		t.Fatalf("model = %s, want claude-x", cfg.Model.Model)
	}
	if cfg.Model.Key != "anthropic-key" {
		t.Fatalf("key = %s, want anthropic-key", cfg.Model.Key)
	}
	if cfg.Model.URL != "https://anthropic.example/v1" {
		t.Fatalf("url = %s, want https://anthropic.example/v1", cfg.Model.URL)
	}
}

func TestApplyEnvOverridesAutoDetectPrefersOpenAIWhenBothComplete(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://openai.example/v1")
	t.Setenv("OPENAI_MODEL", "gpt-openai")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("ANTHROPIC_BASE_URL", "https://anthropic.example/v1")
	t.Setenv("ANTHROPIC_MODEL", "claude-x")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")

	cfg := DefaultConfig()
	ApplyEnvOverrides(cfg)

	if cfg.Model.Protocol != ProtocolOpenAI {
		t.Fatalf("protocol = %s, want %s", cfg.Model.Protocol, ProtocolOpenAI)
	}
	if cfg.Model.Model != "gpt-openai" {
		t.Fatalf("model = %s, want gpt-openai", cfg.Model.Model)
	}
}

func TestApplyEnvOverridesAutoDetectAnthropicWhenOpenAIIncomplete(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "gpt-openai")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("ANTHROPIC_BASE_URL", "https://anthropic.example/v1")
	t.Setenv("ANTHROPIC_MODEL", "claude-x")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")

	cfg := DefaultConfig()
	ApplyEnvOverrides(cfg)

	if cfg.Model.Protocol != ProtocolAnthropic {
		t.Fatalf("protocol = %s, want %s", cfg.Model.Protocol, ProtocolAnthropic)
	}
	if cfg.Model.Model != "claude-x" {
		t.Fatalf("model = %s, want claude-x", cfg.Model.Model)
	}
}

func TestApplyEnvOverridesMSCLIFieldsOverrideProviderFields(t *testing.T) {
	t.Setenv("MSCLI_PROTOCOL", "anthropic")
	t.Setenv("ANTHROPIC_BASE_URL", "https://anthropic.example/v1")
	t.Setenv("ANTHROPIC_MODEL", "claude-x")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("MSCLI_BASE_URL", "https://mscli.example/v1")
	t.Setenv("MSCLI_MODEL", "mscli-model")
	t.Setenv("MSCLI_API_KEY", "mscli-key")

	cfg := DefaultConfig()
	ApplyEnvOverrides(cfg)

	if cfg.Model.Protocol != ProtocolAnthropic {
		t.Fatalf("protocol = %s, want %s", cfg.Model.Protocol, ProtocolAnthropic)
	}
	if cfg.Model.URL != "https://mscli.example/v1" {
		t.Fatalf("url = %s, want https://mscli.example/v1", cfg.Model.URL)
	}
	if cfg.Model.Model != "mscli-model" {
		t.Fatalf("model = %s, want mscli-model", cfg.Model.Model)
	}
	if cfg.Model.Key != "mscli-key" {
		t.Fatalf("key = %s, want mscli-key", cfg.Model.Key)
	}
}

func TestApplyEnvOverridesProtocolSwitchesDefaultURL(t *testing.T) {
	t.Setenv("MSCLI_PROTOCOL", "anthropic")

	cfg := DefaultConfig()
	ApplyEnvOverrides(cfg)

	if cfg.Model.Protocol != ProtocolAnthropic {
		t.Fatalf("protocol = %s, want %s", cfg.Model.Protocol, ProtocolAnthropic)
	}
	if cfg.Model.URL != "https://api.anthropic.com/v1" {
		t.Fatalf("url = %s, want anthropic default", cfg.Model.URL)
	}
}
