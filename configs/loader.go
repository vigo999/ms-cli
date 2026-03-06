package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	return cfg, nil
}

// LoadWithEnv loads configuration from file and applies environment variable overrides.
func LoadWithEnv(path string) (*Config, error) {
	// Auto-discover config file when no explicit path is provided.
	if strings.TrimSpace(path) == "" {
		path = FindConfigFile()
	}

	cfg := DefaultConfig()
	if path != "" {
		loaded, err := LoadFromFile(path)
		if err != nil {
			// If file doesn't exist, continue with defaults.
			if !os.IsNotExist(err) {
				return nil, err
			}
		} else {
			cfg = loaded
		}
	}

	// ENV > YAML > default
	ApplyEnvOverrides(cfg)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// FindConfigFile searches for config file in standard locations.
func FindConfigFile() string {
	// Check environment variable
	if path := os.Getenv("MSCLI_CONFIG"); path != "" {
		return path
	}

	// Check current directory
	if _, err := os.Stat("mscli.yaml"); err == nil {
		return "mscli.yaml"
	}
	if _, err := os.Stat("configs/mscli.yaml"); err == nil {
		return "configs/mscli.yaml"
	}

	// Check config directories
	home, err := os.UserHomeDir()
	if err == nil {
		paths := []string{
			filepath.Join(home, ".config", "mscli", "config.yaml"),
			filepath.Join(home, ".mscli", "config.yaml"),
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	return ""
}

// ApplyEnvOverrides applies environment variable overrides to the config.
// Precedence:
// 1) protocol selection: MSCLI_PROTOCOL > auto-detect(OpenAI triplet > Anthropic triplet) > YAML/default
// 2) model fields: provider-specific env (selected protocol) > MSCLI_MODEL/MSCLI_BASE_URL/MSCLI_API_KEY
func ApplyEnvOverrides(cfg *Config) {
	baseProtocol := NormalizeProtocol(cfg.Model.Protocol)
	protocol := baseProtocol

	if v := strings.TrimSpace(os.Getenv("MSCLI_PROTOCOL")); v != "" {
		if candidate := NormalizeProtocol(v); IsSupportedProtocol(candidate) {
			protocol = candidate
		}
	} else {
		switch {
		case hasEnvTriplet("OPENAI_BASE_URL", "OPENAI_MODEL", "OPENAI_API_KEY"):
			protocol = ProtocolOpenAI
		case hasEnvTriplet("ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL", "ANTHROPIC_API_KEY"):
			protocol = ProtocolAnthropic
		}
	}
	cfg.Model.Protocol = protocol
	if protocol != baseProtocol {
		currentURL := strings.TrimSpace(cfg.Model.URL)
		if currentURL == "" || currentURL == defaultURLForProtocol(baseProtocol) {
			cfg.Model.URL = defaultURLForProtocol(protocol)
		}
	}

	switch protocol {
	case ProtocolAnthropic:
		if v := os.Getenv("ANTHROPIC_MODEL"); v != "" {
			cfg.Model.Model = v
		}
		if v := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); v != "" {
			cfg.Model.Key = v
		}
		if v := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL")); v != "" {
			cfg.Model.URL = v
		}
	default:
		if v := os.Getenv("OPENAI_MODEL"); v != "" {
			cfg.Model.Model = v
		}
		if v := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); v != "" {
			cfg.Model.Key = v
		}
		if v := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); v != "" {
			cfg.Model.URL = v
		}
	}

	// MSCLI_* always has top priority.
	if v := os.Getenv("MSCLI_MODEL"); v != "" {
		cfg.Model.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("MSCLI_API_KEY")); v != "" {
		cfg.Model.Key = v
	}
	if v := strings.TrimSpace(os.Getenv("MSCLI_BASE_URL")); v != "" {
		cfg.Model.URL = v
	}
	if v := os.Getenv("MSCLI_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Model.Temperature = f
		}
	}
	if v := os.Getenv("MSCLI_MAX_TOKENS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Model.MaxTokens = i
		}
	}
	if v := os.Getenv("MSCLI_TIMEOUT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Model.TimeoutSec = i
		}
	}

	// Budget settings
	if v := os.Getenv("MSCLI_BUDGET_TOKENS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Budget.MaxTokens = i
		}
	}
	if v := os.Getenv("MSCLI_BUDGET_COST"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Budget.MaxCostUSD = f
		}
	}

	// UI settings
	if v := os.Getenv("MSCLI_UI_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.UI.Enabled = b
		}
	}

	// Permissions
	if v := os.Getenv("MSCLI_PERMISSIONS_SKIP"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Permissions.SkipRequests = b
		}
	}
	if v := os.Getenv("MSCLI_PERMISSIONS_DEFAULT"); v != "" {
		cfg.Permissions.DefaultLevel = v
	}

	// Context settings
	if v := os.Getenv("MSCLI_CONTEXT_MAX"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Context.MaxTokens = i
		}
	}
	if v := os.Getenv("MSCLI_CONTEXT_RESERVE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Context.ReserveTokens = i
		}
	}

	// Memory settings
	if v := os.Getenv("MSCLI_MEMORY_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Memory.Enabled = b
		}
	}
	if v := os.Getenv("MSCLI_MEMORY_PATH"); v != "" {
		cfg.Memory.StorePath = v
	}
}

func hasEnvTriplet(urlKey, modelKey, apiKey string) bool {
	return strings.TrimSpace(os.Getenv(urlKey)) != "" &&
		strings.TrimSpace(os.Getenv(modelKey)) != "" &&
		strings.TrimSpace(os.Getenv(apiKey)) != ""
}

func defaultURLForProtocol(protocol string) string {
	switch NormalizeProtocol(protocol) {
	case ProtocolAnthropic:
		return "https://api.anthropic.com/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

// SaveToFile saves the configuration to a YAML file.
func SaveToFile(cfg *Config, path string) error {
	if path == "" {
		return fmt.Errorf("config path is required")
	}

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// StringSliceEnv splits an environment variable by comma.
func StringSliceEnv(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}
