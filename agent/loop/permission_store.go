package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FilePermissionStore 基于文件的权限存储
type FilePermissionStore struct {
	mu       sync.RWMutex
	filepath string
	decisions []PermissionDecision
}

// NewFilePermissionStore 创建文件权限存储
func NewFilePermissionStore(path string) (*FilePermissionStore, error) {
	if path == "" {
		path = ".ms-cli/permissions.json"
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	store := &FilePermissionStore{
		filepath:  path,
		decisions: make([]PermissionDecision, 0),
	}

	// 尝试加载已有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load permissions: %w", err)
	}

	return store, nil
}

// SaveDecision 保存权限决策
func (s *FilePermissionStore) SaveDecision(decision PermissionDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在相同的决策
	for i, d := range s.decisions {
		if d.Tool == decision.Tool && d.Action == decision.Action && d.Path == decision.Path {
			s.decisions[i] = decision
			return s.save()
		}
	}

	// 添加新决策
	s.decisions = append(s.decisions, decision)
	return s.save()
}

// LoadDecisions 加载所有权限决策
func (s *FilePermissionStore) LoadDecisions() ([]PermissionDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PermissionDecision, len(s.decisions))
	copy(result, s.decisions)
	return result, nil
}

// ClearDecisions 清除所有权限决策
func (s *FilePermissionStore) ClearDecisions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.decisions = make([]PermissionDecision, 0)
	return s.save()
}

// GetDecisionForTool 获取指定工具的决策
func (s *FilePermissionStore) GetDecisionForTool(tool string) *PermissionDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, d := range s.decisions {
		if d.Tool == tool && d.Action == "" && d.Path == "" {
			return &d
		}
	}
	return nil
}

// GetDecisionForCommand 获取指定命令的决策
func (s *FilePermissionStore) GetDecisionForCommand(command string) *PermissionDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cmd := extractCommandName(command)
	for _, d := range s.decisions {
		if d.Tool == "shell" {
			storedCmd := extractCommandName(d.Action)
			if storedCmd == cmd {
				return &d
			}
		}
	}
	return nil
}

// RemoveExpiredDecisions 移除过期决策
func (s *FilePermissionStore) RemoveExpiredDecisions(maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	valid := make([]PermissionDecision, 0, len(s.decisions))

	for _, d := range s.decisions {
		if d.Timestamp.After(cutoff) {
			valid = append(valid, d)
		}
	}

	s.decisions = valid
	return s.save()
}

// save 保存到文件（必须持有锁）
func (s *FilePermissionStore) save() error {
	data, err := json.MarshalIndent(s.decisions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}

	return os.WriteFile(s.filepath, data, 0644)
}

// load 从文件加载（必须持有锁）
func (s *FilePermissionStore) load() error {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.decisions)
}

// GetFilePath 获取存储文件路径
func (s *FilePermissionStore) GetFilePath() string {
	return s.filepath
}

// MemoryPermissionStore 内存权限存储
type MemoryPermissionStore struct {
	mu        sync.RWMutex
	decisions []PermissionDecision
}

// NewMemoryPermissionStore 创建内存权限存储
func NewMemoryPermissionStore() *MemoryPermissionStore {
	return &MemoryPermissionStore{
		decisions: make([]PermissionDecision, 0),
	}
}

// SaveDecision 保存权限决策
func (s *MemoryPermissionStore) SaveDecision(decision PermissionDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新或添加
	for i, d := range s.decisions {
		if d.Tool == decision.Tool && d.Action == decision.Action && d.Path == decision.Path {
			s.decisions[i] = decision
			return nil
		}
	}

	s.decisions = append(s.decisions, decision)
	return nil
}

// LoadDecisions 加载所有权限决策
func (s *MemoryPermissionStore) LoadDecisions() ([]PermissionDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PermissionDecision, len(s.decisions))
	copy(result, s.decisions)
	return result, nil
}

// ClearDecisions 清除所有权限决策
func (s *MemoryPermissionStore) ClearDecisions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.decisions = make([]PermissionDecision, 0)
	return nil
}

// PermissionStoreConfig 权限存储配置
type PermissionStoreConfig struct {
	Type      string        // "file" or "memory"
	Path      string        // for file store
	MaxAge    time.Duration // 决策最大有效期
}

// DefaultPermissionStoreConfig 返回默认配置
func DefaultPermissionStoreConfig() PermissionStoreConfig {
	return PermissionStoreConfig{
		Type:      "file",
		Path:      ".ms-cli/permissions.json",
		MaxAge:    7 * 24 * time.Hour, // 7 days
	}
}

// NewPermissionStore 创建权限存储
func NewPermissionStore(cfg PermissionStoreConfig) (PermissionStore, error) {
	switch cfg.Type {
	case "file":
		return NewFilePermissionStore(cfg.Path)
	case "memory":
		return NewMemoryPermissionStore(), nil
	default:
		return nil, fmt.Errorf("unknown store type: %s", cfg.Type)
	}
}

// PermissionStats 权限统计
type PermissionStats struct {
	TotalDecisions   int
	ByLevel          map[PermissionLevel]int
	ByTool           map[string]int
	OldestDecision   time.Time
	NewestDecision   time.Time
}

// GetStats 获取权限决策统计
func (s *FilePermissionStore) GetStats() PermissionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := PermissionStats{
		TotalDecisions: len(s.decisions),
		ByLevel:        make(map[PermissionLevel]int),
		ByTool:         make(map[string]int),
	}

	if len(s.decisions) == 0 {
		return stats
	}

	stats.OldestDecision = s.decisions[0].Timestamp
	stats.NewestDecision = s.decisions[0].Timestamp

	for _, d := range s.decisions {
		stats.ByLevel[d.Level]++
		stats.ByTool[d.Tool]++

		if d.Timestamp.Before(stats.OldestDecision) {
			stats.OldestDecision = d.Timestamp
		}
		if d.Timestamp.After(stats.NewestDecision) {
			stats.NewestDecision = d.Timestamp
		}
	}

	return stats
}

// ExportToFile 导出权限决策到文件
func (s *FilePermissionStore) ExportToFile(exportPath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.decisions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(exportPath, data, 0644)
}

// ImportFromFile 从文件导入权限决策
func (s *FilePermissionStore) ImportFromFile(importPath string) error {
	data, err := os.ReadFile(importPath)
	if err != nil {
		return err
	}

	var imported []PermissionDecision
	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 合并导入的决策
	for _, newDecision := range imported {
		found := false
		for i, existing := range s.decisions {
			if existing.Tool == newDecision.Tool &&
				existing.Action == newDecision.Action &&
				existing.Path == newDecision.Path {
				// 更新现有决策
				s.decisions[i] = newDecision
				found = true
				break
			}
		}
		if !found {
			s.decisions = append(s.decisions, newDecision)
		}
	}

	return s.save()
}
