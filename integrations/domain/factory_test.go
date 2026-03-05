package domain

import (
	"os"
	"testing"
)

func TestHTTPFactory_ProvidersSorted(t *testing.T) {
	f := NewFactory(FactoryConfig{})
	ps := f.Providers()
	if len(ps) < 2 {
		t.Fatalf("expected built-in providers, got %d", len(ps))
	}
	if ps[0].Name > ps[1].Name {
		t.Fatalf("providers should be sorted: %+v", ps)
	}
}

func TestHTTPFactory_ClientForWithStaticKey(t *testing.T) {
	f := NewFactory(FactoryConfig{
		Providers: map[string]ProviderConfig{
			"openai": {
				Endpoint:  "https://api.openai.com/v1",
				APIKeyEnv: "OPENAI_API_KEY",
				APIKey:    "test-key",
			},
		},
	})
	cli, err := f.ClientFor(ModelSpec{Provider: "openai", Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatalf("ClientFor failed: %v", err)
	}
	if cli == nil {
		t.Fatalf("expected non-nil client")
	}
}

func TestHTTPFactory_ClientForMissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_ = os.Unsetenv("OPENAI_API_KEY")

	f := NewFactory(FactoryConfig{
		Providers: map[string]ProviderConfig{
			"openai": {
				Endpoint:  "https://api.openai.com/v1",
				APIKeyEnv: "OPENAI_API_KEY",
			},
		},
	})
	_, err := f.ClientFor(ModelSpec{Provider: "openai", Model: "gpt-4o-mini"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestHTTPFactory_ClientForWithBaseURLOnly(t *testing.T) {
	f := NewFactory(FactoryConfig{
		Providers: map[string]ProviderConfig{
			"openrouter": {
				BaseURL:   "https://openrouter.ai/api/v1",
				APIKeyEnv: "OPENROUTER_API_KEY",
				APIKey:    "test-key",
			},
		},
	})
	cli, err := f.ClientFor(ModelSpec{Provider: "openrouter", Model: "deepseek/deepseek-r1"})
	if err != nil {
		t.Fatalf("ClientFor failed: %v", err)
	}
	if cli == nil {
		t.Fatalf("expected non-nil client")
	}
}
