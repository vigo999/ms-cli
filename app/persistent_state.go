package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultSessionStatePath = ".mscli/session.yaml"
)

type PersistedAPIKeys struct {
	OpenAI     string `yaml:"openai,omitempty"`
	OpenRouter string `yaml:"openrouter,omitempty"`
}

type PersistentState struct {
	Version   int              `yaml:"version"`
	Model     SessionModel     `yaml:"model"`
	APIKeys   PersistedAPIKeys `yaml:"api_keys,omitempty"`
	UpdatedAt string           `yaml:"updated_at,omitempty"`
}

func defaultPersistentState() PersistentState {
	return PersistentState{
		Version: 1,
	}
}

func ResolveSessionStatePath(workDir string) string {
	return filepath.Join(workDir, defaultSessionStatePath)
}

func LoadPersistentState(path string) (PersistentState, error) {
	st := defaultPersistentState()
	p := strings.TrimSpace(path)
	if p == "" {
		return st, errors.New("session state path is required")
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return st, err
	}

	if err := yaml.Unmarshal(data, &st); err != nil {
		return st, err
	}
	if st.Version <= 0 {
		st.Version = 1
	}
	return st, nil
}

func SavePersistentState(path string, st PersistentState) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return errors.New("session state path is required")
	}
	if st.Version <= 0 {
		st.Version = 1
	}
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func hasModelEnvOverride() bool {
	return strings.TrimSpace(os.Getenv("MSCLI_MODEL_PROVIDER")) != "" ||
		strings.TrimSpace(os.Getenv("MSCLI_MODEL_NAME")) != "" ||
		strings.TrimSpace(os.Getenv("MSCLI_MODEL_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")) != "" ||
		strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")) != ""
}
