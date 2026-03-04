// Package configs provides state persistence for user preferences.
package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// State holds user preferences that persist across sessions.
type State struct {
	Model        string `yaml:"model,omitempty"`
	Key          string `yaml:"key,omitempty"`
	LegacyAPIKey string `yaml:"api_key,omitempty"` // Backward compatibility.
}

// StateManager manages persistent state.
type StateManager struct {
	mu       sync.RWMutex
	state    State
	filePath string
}

// NewStateManager creates a new state manager.
// If workDir is empty, uses current directory.
func NewStateManager(workDir string) *StateManager {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &StateManager{
		filePath: filepath.Join(workDir, ".mscli", "state.yaml"),
	}
}

// Load loads state from disk.
func (m *StateManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file yet, that's ok
			return nil
		}
		return fmt.Errorf("read state file: %w", err)
	}

	if err := yaml.Unmarshal(data, &m.state); err != nil {
		return fmt.Errorf("parse state file: %w", err)
	}

	return nil
}

// Save saves state to disk.
func (m *StateManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := yaml.Marshal(m.state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

// Get returns a copy of the current state.
func (m *StateManager) Get() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// SetModel sets the model.
func (m *StateManager) SetModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Model = model
}

// SetKey sets the API key.
func (m *StateManager) SetKey(apiKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Key = apiKey
}

// ApplyToConfig applies saved state to config.
func (m *StateManager) ApplyToConfig(cfg *Config) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state.Model != "" {
		cfg.Model.Model = m.state.Model
	}
	if m.state.Key != "" {
		cfg.Model.Key = m.state.Key
	} else if m.state.LegacyAPIKey != "" {
		cfg.Model.Key = m.state.LegacyAPIKey
	}
}

// SaveFromConfig saves current config to state.
func (m *StateManager) SaveFromConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Model = cfg.Model.Model
	m.state.Key = cfg.Model.Key
}
