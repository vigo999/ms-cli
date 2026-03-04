package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileStore 基于文件的会话存储
type FileStore struct {
	mu       sync.RWMutex
	basePath string
}

// NewFileStore 创建新的文件存储
func NewFileStore(basePath string) (*FileStore, error) {
	if basePath == "" {
		basePath = ".ms-cli/sessions"
	}

	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	return &FileStore{
		basePath: basePath,
	}, nil
}

// Save 保存会话
func (fs *FileStore) Save(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// 更新修改时间
	session.UpdatedAt = time.Now()

	// 序列化为 JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// 写入文件
	filepath := fs.getFilePath(session.ID)
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load 加载会话
func (fs *FileStore) Load(id ID) (*Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filepath := fs.getFilePath(id)
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// Delete 删除会话
func (fs *FileStore) Delete(id ID) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.deleteNoLock(id)
}

// deleteNoLock 删除会话文件（调用者必须持有 fs.mu）。
func (fs *FileStore) deleteNoLock(id ID) error {
	filepath := fs.getFilePath(id)
	if err := os.Remove(filepath); err != nil {
		if os.IsNotExist(err) {
			return nil // 已经不存在，不算错误
		}
		return fmt.Errorf("delete session file: %w", err)
	}
	return nil
}

// List 列出所有会话
func (fs *FileStore) List() ([]Info, error) {
	return fs.ListFiltered(Filter{})
}

// ListFiltered 根据过滤器列出会话
func (fs *FileStore) ListFiltered(filter Filter) ([]Info, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, fmt.Errorf("read session directory: %w", err)
	}

	var infos []Info
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 .json 文件
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// 从文件名提取 ID
		id := ID(strings.TrimSuffix(entry.Name(), ".json"))
		session, err := fs.Load(id)
		if err != nil {
			continue // 跳过损坏的文件
		}

		// 应用过滤器
		if !fs.matchesFilter(session, filter) {
			continue
		}

		infos = append(infos, session.ToInfo())
	}

	// 按更新时间排序（最新的在前）
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[j].UpdatedAt.After(infos[i].UpdatedAt) {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	return infos, nil
}

// matchesFilter 检查会话是否匹配过滤器
func (fs *FileStore) matchesFilter(session *Session, filter Filter) bool {
	// 归档状态过滤
	if filter.Archived != nil && session.Archived != *filter.Archived {
		return false
	}

	// 工作目录过滤
	if filter.WorkDir != "" && session.WorkDir != filter.WorkDir {
		return false
	}

	// 名称前缀过滤
	if filter.NamePrefix != "" && !strings.HasPrefix(session.Name, filter.NamePrefix) {
		return false
	}

	// 标签过滤
	if len(filter.Tags) > 0 {
		hasTag := false
		for _, tag := range filter.Tags {
			for _, sessionTag := range session.Metadata.Tags {
				if tag == sessionTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	return true
}

// Exists 检查会话是否存在
func (fs *FileStore) Exists(id ID) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filepath := fs.getFilePath(id)
	_, err := os.Stat(filepath)
	return err == nil
}

// Close 关闭存储
func (fs *FileStore) Close() error {
	// 文件存储无需特殊关闭操作
	return nil
}

// getFilePath 获取会话文件路径
func (fs *FileStore) getFilePath(id ID) string {
	return filepath.Join(fs.basePath, string(id)+".json")
}

// GetSessionPath 获取会话存储路径
func (fs *FileStore) GetSessionPath(id ID) string {
	return fs.getFilePath(id)
}

// Export 导出会话到指定路径
func (fs *FileStore) Export(id ID, exportPath string) error {
	session, err := fs.Load(id)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	return os.WriteFile(exportPath, data, 0644)
}

// Import 从指定路径导入会话
func (fs *FileStore) Import(importPath string) (*Session, error) {
	data, err := os.ReadFile(importPath)
	if err != nil {
		return nil, fmt.Errorf("read import file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	// 重新生成 ID 以避免冲突
	session.ID = generateID()
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	if err := fs.Save(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

// CleanupOldSessions 清理过期会话
func (fs *FileStore) CleanupOldSessions(maxAge time.Duration) (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 删除过期的文件
		if info.ModTime().Before(cutoff) {
			id := ID(strings.TrimSuffix(entry.Name(), ".json"))
			if err := fs.deleteNoLock(id); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}
