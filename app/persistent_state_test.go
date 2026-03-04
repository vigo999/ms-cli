package main

import (
	"path/filepath"
	"testing"
)

func TestLoadSavePersistentState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.yaml")
	in := PersistentState{
		Version: 1,
		Model: SessionModel{
			Provider: "openrouter",
			Name:     "deepseek/deepseek-r1",
			Endpoint: "https://openrouter.ai/api/v1",
		},
		APIKeys: PersistedAPIKeys{
			OpenRouter: "test-key",
		},
	}

	if err := SavePersistentState(path, in); err != nil {
		t.Fatalf("SavePersistentState failed: %v", err)
	}

	out, err := LoadPersistentState(path)
	if err != nil {
		t.Fatalf("LoadPersistentState failed: %v", err)
	}
	if out.Model.Provider != in.Model.Provider || out.Model.Name != in.Model.Name {
		t.Fatalf("model mismatch: got=%+v want=%+v", out.Model, in.Model)
	}
	if out.APIKeys.OpenRouter != in.APIKeys.OpenRouter {
		t.Fatalf("api key mismatch")
	}
}

func TestHasModelEnvOverride_Endpoint(t *testing.T) {
	t.Setenv("MSCLI_MODEL_PROVIDER", "")
	t.Setenv("MSCLI_MODEL_NAME", "")
	t.Setenv("MSCLI_MODEL_ENDPOINT", "https://example.com/v1")
	if !hasModelEnvOverride() {
		t.Fatalf("endpoint override should be detected")
	}
}

func TestHasModelEnvOverride_OpenAIBaseURL(t *testing.T) {
	t.Setenv("MSCLI_MODEL_PROVIDER", "")
	t.Setenv("MSCLI_MODEL_NAME", "")
	t.Setenv("MSCLI_MODEL_ENDPOINT", "")
	t.Setenv("OPENAI_BASE_URL", "https://openai-base.example.com/v1")
	if !hasModelEnvOverride() {
		t.Fatalf("OPENAI_BASE_URL override should be detected")
	}
}
