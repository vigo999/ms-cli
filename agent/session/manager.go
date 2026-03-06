package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Manager 会话管理器
type Manager struct {
	mu       sync.RWMutex
	store    Store
	config   Config
	current  *Session
	sessions map[ID]*Session // 缓存
}

// NewManager 创建新的会话管理器
func NewManager(store Store, cfg Config) *Manager {
	if store == nil {
		panic("store cannot be nil")
	}

	return &Manager{
		store:    store,
		config:   cfg,
		sessions: make(map[ID]*Session),
	}
}

// Create 创建新会话
func (m *Manager) Create(name, workDir string) (*Session, error) {
	if name == "" {
		name = fmt.Sprintf("Session_%d", time.Now().Unix())
	}

	session := New(name, workDir)

	m.mu.Lock()
	defer m.mu.Unlock()
	session.ID = m.nextAvailableIDLocked(session.ID)

	if err := m.store.Save(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	m.sessions[session.ID] = session
	return session, nil
}

func (m *Manager) nextAvailableIDLocked(base ID) ID {
	candidate := base
	suffix := 2
	for m.idExistsLocked(candidate) {
		candidate = ID(fmt.Sprintf("%s-%d", base, suffix))
		suffix++
	}
	return candidate
}

func (m *Manager) idExistsLocked(id ID) bool {
	if _, ok := m.sessions[id]; ok {
		return true
	}
	return m.store.Exists(id)
}

// CreateFromMessages 从消息创建会话
func (m *Manager) CreateFromMessages(name, workDir string, messages []llm.Message) (*Session, error) {
	session, err := m.Create(name, workDir)
	if err != nil {
		return nil, err
	}

	for _, msg := range messages {
		session.AddMessage(msg)
	}

	if m.config.AutoSave {
		m.Save(session.ID)
	}

	return session, nil
}

// Load 加载会话
func (m *Manager) Load(id ID) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先查缓存
	if session, ok := m.sessions[id]; ok {
		m.current = session
		return session, nil
	}

	// 从存储加载
	session, err := m.store.Load(id)
	if err != nil {
		return nil, err
	}

	m.sessions[id] = session
	m.current = session
	return session, nil
}

// Save 保存会话
func (m *Manager) Save(id ID) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not in cache: %s", id)
	}

	return m.store.Save(session)
}

// SaveCurrent 保存当前会话
func (m *Manager) SaveCurrent() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	return m.store.Save(m.current)
}

// Delete 删除会话
func (m *Manager) Delete(id ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果是当前会话，清除当前会话
	if m.current != nil && m.current.ID == id {
		m.current = nil
	}

	// 从缓存删除
	delete(m.sessions, id)

	// 从存储删除
	return m.store.Delete(id)
}

// Current 获取当前会话
func (m *Manager) Current() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// SetCurrent 设置当前会话
func (m *Manager) SetCurrent(id ID) error {
	session, err := m.Load(id)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.current = session
	m.mu.Unlock()

	return nil
}

// CreateAndSetCurrent 创建并设置为当前会话
func (m *Manager) CreateAndSetCurrent(name, workDir string) (*Session, error) {
	session, err := m.Create(name, workDir)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.current = session
	m.mu.Unlock()

	return session, nil
}

// List 列出所有会话
func (m *Manager) List() ([]Info, error) {
	return m.store.List()
}

// ListActive 列出活动会话（未归档）
func (m *Manager) ListActive() ([]Info, error) {
	archived := false
	return m.store.ListFiltered(Filter{Archived: &archived})
}

// ListArchived 列出归档会话
func (m *Manager) ListArchived() ([]Info, error) {
	archived := true
	return m.store.ListFiltered(Filter{Archived: &archived})
}

// Archive 归档会话
func (m *Manager) Archive(id ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		var err error
		session, err = m.store.Load(id)
		if err != nil {
			return err
		}
		m.sessions[id] = session
	}

	session.Archive()
	return m.store.Save(session)
}

// Unarchive 取消归档
func (m *Manager) Unarchive(id ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		var err error
		session, err = m.store.Load(id)
		if err != nil {
			return err
		}
		m.sessions[id] = session
	}

	session.Unarchive()
	return m.store.Save(session)
}

// Rename 重命名会话
func (m *Manager) Rename(id ID, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		var err error
		session, err = m.store.Load(id)
		if err != nil {
			return err
		}
		m.sessions[id] = session
	}

	session.Name = newName
	session.UpdatedAt = time.Now()
	return m.store.Save(session)
}

// AddTag 为会话添加标签
func (m *Manager) AddTag(id ID, tag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		var err error
		session, err = m.store.Load(id)
		if err != nil {
			return err
		}
		m.sessions[id] = session
	}

	// 检查标签是否已存在
	for _, t := range session.Metadata.Tags {
		if t == tag {
			return nil
		}
	}

	session.Metadata.Tags = append(session.Metadata.Tags, tag)
	session.UpdatedAt = time.Now()
	return m.store.Save(session)
}

// RemoveTag 为会话移除标签
func (m *Manager) RemoveTag(id ID, tag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		var err error
		session, err = m.store.Load(id)
		if err != nil {
			return err
		}
		m.sessions[id] = session
	}

	// 查找并移除标签
	for i, t := range session.Metadata.Tags {
		if t == tag {
			session.Metadata.Tags = append(session.Metadata.Tags[:i], session.Metadata.Tags[i+1:]...)
			session.UpdatedAt = time.Now()
			return m.store.Save(session)
		}
	}

	return nil
}

// GetMessages 获取会话消息
func (m *Manager) GetMessages(id ID) ([]llm.Message, error) {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if ok {
		return session.Messages, nil
	}

	loaded, err := m.store.Load(id)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	// Double-check after acquiring write lock to avoid duplicate map writes.
	if session, ok = m.sessions[id]; !ok {
		m.sessions[id] = loaded
		session = loaded
	}
	m.mu.Unlock()

	return session.Messages, nil
}

// AddMessageToCurrent 添加消息到当前会话
func (m *Manager) AddMessageToCurrent(msg llm.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	m.current.AddMessage(msg)

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// ClearCurrentMessages 清空当前会话的消息
func (m *Manager) ClearCurrentMessages() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	m.current.ClearMessages()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// UpdateCurrentMetadata 更新当前会话的元数据
func (m *Manager) UpdateCurrentMetadata(tokens int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	m.current.UpdateMetadata(tokens)

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// Cleanup 清理过期会话
func (m *Manager) Cleanup() (int, error) {
	if m.config.MaxAge <= 0 {
		return 0, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	fileStore, ok := m.store.(*FileStore)
	if !ok {
		return 0, fmt.Errorf("cleanup only supported for FileStore")
	}

	return fileStore.CleanupOldSessions(m.config.MaxAge)
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存当前会话
	if m.current != nil && m.config.AutoSave {
		if err := m.store.Save(m.current); err != nil {
			return err
		}
	}

	// 如果配置了归档
	if m.config.ArchiveOnClose && m.current != nil {
		m.current.Archive()
		m.store.Save(m.current)
	}

	return m.store.Close()
}

// GetSessionCount 获取会话数量
func (m *Manager) GetSessionCount() (int, error) {
	infos, err := m.store.List()
	if err != nil {
		return 0, err
	}
	return len(infos), nil
}

// Exists 检查会话是否存在
func (m *Manager) Exists(id ID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.sessions[id]; ok {
		return true
	}

	return m.store.Exists(id)
}

// Export 导出会话
func (m *Manager) Export(id ID, exportPath string) error {
	fileStore, ok := m.store.(*FileStore)
	if !ok {
		return fmt.Errorf("export only supported for FileStore")
	}

	return fileStore.Export(id, exportPath)
}

// Import 导入会话
func (m *Manager) Import(importPath string) (*Session, error) {
	fileStore, ok := m.store.(*FileStore)
	if !ok {
		return nil, fmt.Errorf("import only supported for FileStore")
	}

	return fileStore.Import(importPath)
}

// GetCurrentWorkDir 获取当前会话的工作目录
func (m *Manager) GetCurrentWorkDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return ""
	}
	return m.current.WorkDir
}

// SetCurrentWorkDir 设置当前会话的工作目录
func (m *Manager) SetCurrentWorkDir(workDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	m.current.WorkDir = workDir
	m.current.UpdatedAt = time.Now()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// UpdateCurrentRuntime updates runtime snapshot of current session.
func (m *Manager) UpdateCurrentRuntime(runtime RuntimeSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	normalizePermissionSnapshot(&runtime.Permission)
	m.current.Runtime = runtime
	m.current.UpdatedAt = time.Now()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// SetCurrentTracePath updates trace path in current runtime snapshot.
func (m *Manager) SetCurrentTracePath(tracePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	normalizePermissionSnapshot(&m.current.Runtime.Permission)
	m.current.Runtime.TracePath = tracePath
	m.current.UpdatedAt = time.Now()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// UpdateCurrentModel updates model snapshot in current runtime snapshot.
func (m *Manager) UpdateCurrentModel(model ModelSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	normalizePermissionSnapshot(&m.current.Runtime.Permission)
	m.current.Runtime.Model = model
	m.current.UpdatedAt = time.Now()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

// UpdateCurrentPermission updates permission snapshot in current runtime snapshot.
func (m *Manager) UpdateCurrentPermission(snapshot PermissionSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	normalizePermissionSnapshot(&snapshot)
	m.current.Runtime.Permission = snapshot
	m.current.UpdatedAt = time.Now()

	if m.config.AutoSave {
		return m.store.Save(m.current)
	}
	return nil
}

func normalizePermissionSnapshot(snapshot *PermissionSnapshot) {
	if snapshot.ToolPolicies == nil {
		snapshot.ToolPolicies = make(map[string]string)
	}
	if snapshot.CommandPolicies == nil {
		snapshot.CommandPolicies = make(map[string]string)
	}
	if snapshot.PathPolicies == nil {
		snapshot.PathPolicies = make([]PathPolicySnapshot, 0)
	}
}
