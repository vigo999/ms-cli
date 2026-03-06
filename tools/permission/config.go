package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 权限配置
type Config struct {
	// 规则集
	Ruleset *Ruleset `json:"ruleset"`

	// 缓存配置
	Cache CacheConfig `json:"cache,omitempty"`

	// 超时配置
	Timeout TimeoutConfig `json:"timeout,omitempty"`

	// 存储路径
	StoragePath string `json:"storagePath,omitempty"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 是否启用缓存
	Enabled bool `json:"enabled,omitempty"`

	// 缓存大小限制
	MaxSize int `json:"maxSize,omitempty"`

	// 缓存过期时间（秒）
	TTL int `json:"ttl,omitempty"`

	// 清理间隔（秒）
	CleanupInterval int `json:"cleanupInterval,omitempty"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	// 权限确认超时（秒）
	PermissionConfirm int `json:"permissionConfirm,omitempty"`

	// 评估超时（毫秒）
	Evaluation int `json:"evaluation,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Ruleset: NewRuleset(),
		Cache: CacheConfig{
			Enabled:         true,
			MaxSize:         1000,
			TTL:             3600,
			CleanupInterval: 300,
		},
		Timeout: TimeoutConfig{
			PermissionConfirm: 60,
			Evaluation:        100,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在返回默认配置
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file failed: %w", err)
	}

	// 设置默认值
	if config.Ruleset == nil {
		config.Ruleset = NewRuleset()
	}

	return &config, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory failed: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file failed: %w", err)
	}

	return nil
}

// AddRule 添加规则
func (c *Config) AddRule(rule Rule) *Config {
	c.Ruleset.AddRule(rule)
	return c
}

// SetDefaultAction 设置默认决策
func (c *Config) SetDefaultAction(action Action) *Config {
	c.Ruleset.SetDefaultAction(action)
	return c
}

// WithStoragePath 设置存储路径
func (c *Config) WithStoragePath(path string) *Config {
	c.StoragePath = path
	return c
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Ruleset == nil {
		return fmt.Errorf("ruleset is required")
	}

	for i, rule := range c.Ruleset.Rules {
		if rule.Permission == "" {
			return fmt.Errorf("rule %d: permission is required", i)
		}
		if rule.Action != ActionAllow && rule.Action != ActionDeny && rule.Action != ActionAsk {
			return fmt.Errorf("rule %d: invalid action %s", i, rule.Action)
		}
	}

	return nil
}
