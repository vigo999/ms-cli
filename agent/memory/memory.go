package memory

import (
	"fmt"
	"sync"
	"time"
)

// Manager 记忆管理器
type Manager struct {
	mu        sync.RWMutex
	store     Store
	policy    Policy
	config    Config
	retriever *Retriever

	// 缓存
	cache     map[string]*MemoryItem
	cacheSize int

	// 统计
	stats Stats
}

// Stats 统计信息
type Stats struct {
	TotalSaved     int64
	TotalRetrieved int64
	TotalDeleted   int64
	CompactCount   int64
}

// NewManager 创建新的记忆管理器
func NewManager(store Store, cfg Config) *Manager {
	if store == nil {
		panic("store cannot be nil")
	}

	return &Manager{
		store:     store,
		policy:    DefaultPolicy(),
		config:    cfg,
		retriever: NewRetriever(store, DefaultPolicy()),
		cache:     make(map[string]*MemoryItem),
		cacheSize: 100,
	}
}

// SetPolicy 设置保留策略
func (m *Manager) SetPolicy(policy Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.retriever = NewRetriever(m.store, policy)
}

// Save 保存记忆项
func (m *Manager) Save(item *MemoryItem) error {
	if item == nil {
		return fmt.Errorf("item cannot be nil")
	}

	// 检查是否应该保留
	if !m.policy.ShouldKeep(item) {
		return fmt.Errorf("item does not meet retention policy")
	}

	// 设置 TTL（如果没有设置）
	typeTTL := m.policy.GetTTLForType(item.Type)
	if item.ExpiresAt == nil && typeTTL > 0 {
		item.SetTTL(typeTTL)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.Save(item); err != nil {
		return fmt.Errorf("save to store: %w", err)
	}

	// 更新缓存
	m.updateCache(item)

	m.stats.TotalSaved++

	// 检查是否需要压缩
	if m.config.AutoCompact {
		m.checkAndCompact()
	}

	return nil
}

// SaveSessionMemory 保存会话记忆
func (m *Manager) SaveSessionMemory(sessionID string, content string, importance int) error {
	item := NewMemoryItem(MemoryTypeSession, content)
	item.Importance = importance
	item.SetMetadata("session_id", sessionID)
	return m.Save(item)
}

// SaveFact 保存事实知识
func (m *Manager) SaveFact(fact string, tags []string) error {
	item := NewMemoryItem(MemoryTypeFact, fact)
	item.Importance = 7
	item.Tags = tags
	return m.Save(item)
}

// SaveTask 保存任务记录
func (m *Manager) SaveTask(description string, status string) error {
	item := NewMemoryItem(MemoryTypeTask, description)
	item.Importance = 6
	item.SetMetadata("status", status)
	return m.Save(item)
}

// SavePreference 保存用户偏好
func (m *Manager) SavePreference(key string, value string) error {
	content := fmt.Sprintf("%s: %s", key, value)
	item := NewMemoryItem(MemoryTypePreference, content)
	item.Importance = 8 // 偏好通常很重要
	item.SetMetadata("key", key)
	item.SetMetadata("value", value)
	// 偏好不过期
	item.ExpiresAt = nil
	return m.Save(item)
}

// SaveCodeSnippet 保存代码片段
func (m *Manager) SaveCodeSnippet(code string, language string, description string) error {
	item := NewMemoryItem(MemoryTypeCode, code)
	item.Importance = 6
	item.Tags = []string{language, "code"}
	item.SetMetadata("language", language)
	item.SetMetadata("description", description)
	return m.Save(item)
}

// Get 获取记忆项
func (m *Manager) Get(id string) (*MemoryItem, error) {
	// 先查缓存
	m.mu.RLock()
	if item, ok := m.cache[id]; ok {
		m.mu.RUnlock()
		item.RecordAccess()
		return item, nil
	}
	m.mu.RUnlock()

	// 查存储
	item, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	// 更新缓存
	m.mu.Lock()
	m.updateCache(item)
	m.stats.TotalRetrieved++
	m.mu.Unlock()

	item.RecordAccess()
	return item, nil
}

// Query 查询记忆
func (m *Manager) Query(q Query) ([]*MemoryItem, error) {
	result, err := m.retriever.Retrieve(q)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.stats.TotalRetrieved += int64(len(result.Items))
	m.mu.Unlock()

	return result.Items, nil
}

// RetrieveRecent 获取最近记忆
func (m *Manager) RetrieveRecent(duration time.Duration, limit int) ([]*MemoryItem, error) {
	return m.retriever.RetrieveRecent(duration, limit)
}

// RetrieveImportant 获取重要记忆
func (m *Manager) RetrieveImportant(limit int) ([]*MemoryItem, error) {
	return m.retriever.RetrieveImportant(7, limit)
}

// RetrieveForContext 为上下文获取相关记忆
func (m *Manager) RetrieveForContext(context string, limit int) ([]*MemoryItem, error) {
	return m.retriever.RetrieveForContext(context, limit)
}

// RetrieveByType 按类型获取记忆
func (m *Manager) RetrieveByType(memType MemoryType, limit int) ([]*MemoryItem, error) {
	return m.retriever.RetrieveByType(memType, limit)
}

// Search 搜索记忆
func (m *Manager) Search(keyword string, limit int) ([]*MemoryItem, error) {
	return m.retriever.RetrieveByKeyword(keyword, limit)
}

// Delete 删除记忆
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 删除缓存
	delete(m.cache, id)

	if err := m.store.Delete(id); err != nil {
		return err
	}

	m.stats.TotalDeleted++
	return nil
}

// DeleteExpired 删除过期记忆
func (m *Manager) DeleteExpired() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清理缓存中的过期项
	for id, item := range m.cache {
		if item.IsExpired() {
			delete(m.cache, id)
		}
	}

	// 清理存储中的过期项
	extendedStore, ok := m.store.(ExtendedStore)
	if ok {
		return extendedStore.DeleteExpired()
	}

	return nil
}

// Compact 手动压缩
func (m *Manager) Compact() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.compact()
}

// compact 执行压缩（必须持有锁）
func (m *Manager) compact() error {
	extendedStore, ok := m.store.(ExtendedStore)
	if !ok {
		return fmt.Errorf("store does not support compaction")
	}

	// 获取所有记忆项
	items, err := extendedStore.List(10000, 0)
	if err != nil {
		return fmt.Errorf("list items: %w", err)
	}

	// 评估每个记忆项
	evaluator := NewPolicyEvaluator(m.policy)
	compactionPolicy := DefaultCompactionPolicy()

	var toDelete []string
	scores := make(map[string]float64)

	for _, item := range items {
		// 检查是否应该保留
		result := evaluator.Evaluate(item)
		if !result.ShouldKeep {
			toDelete = append(toDelete, item.ID)
			continue
		}

		// 计算优先级分数
		scores[item.ID] = compactionPolicy.CalculatePriority(item)
	}

	// 如果数量超过限制，删除低优先级的
	if len(items)-len(toDelete) > m.policy.MaxItems {
		// 按分数排序
		type scored struct {
			id    string
			score float64
		}
		var scoredItems []scored
		for id, score := range scores {
			scoredItems = append(scoredItems, scored{id: id, score: score})
		}

		// 按分数降序排序
		for i := 0; i < len(scoredItems); i++ {
			for j := i + 1; j < len(scoredItems); j++ {
				if scoredItems[j].score > scoredItems[i].score {
					scoredItems[i], scoredItems[j] = scoredItems[j], scoredItems[i]
				}
			}
		}

		// 删除低优先级的
		toKeep := m.policy.MaxItems
		if len(scoredItems) > toKeep {
			for i := toKeep; i < len(scoredItems); i++ {
				toDelete = append(toDelete, scoredItems[i].id)
			}
		}
	}

	// 执行删除
	for _, id := range toDelete {
		delete(m.cache, id)
		m.store.Delete(id)
	}

	m.stats.CompactCount++
	return nil
}

// checkAndCompact 检查并执行压缩（必须持有锁）
func (m *Manager) checkAndCompact() {
	extendedStore, ok := m.store.(ExtendedStore)
	if !ok {
		return
	}

	count, err := extendedStore.Count()
	if err != nil {
		return
	}

	// 检查是否需要压缩
	if float64(count)/float64(m.policy.MaxItems) > m.config.CompactThreshold {
		m.compact()
	}
}

// updateCache 更新缓存（必须持有锁）
func (m *Manager) updateCache(item *MemoryItem) {
	// 如果缓存已满，删除一个旧项
	if len(m.cache) >= m.cacheSize {
		for id := range m.cache {
			delete(m.cache, id)
			break
		}
	}
	m.cache[item.ID] = item
}

// GetStats 获取统计信息
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetStoreStats 获取存储统计
func (m *Manager) GetStoreStats() (MemoryStats, error) {
	extendedStore, ok := m.store.(ExtendedStore)
	if !ok {
		return MemoryStats{}, fmt.Errorf("store does not support stats")
	}
	return extendedStore.GetStats()
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.Close()
}

// CreateMemoryFromSession 从会话创建记忆
func (m *Manager) CreateMemoryFromSession(sessionID string, messages []string, summary string) error {
	// 保存摘要
	if summary != "" {
		item := NewMemoryItem(MemoryTypeSession, summary)
		item.Importance = 6
		item.SetMetadata("session_id", sessionID)
		item.SetMetadata("message_count", len(messages))
		if err := m.Save(item); err != nil {
			return err
		}
	}

	// 保存重要消息
	for i, msg := range messages {
		// 只保存重要的消息（简单启发式：长度较长的）
		if len(msg) > 200 {
			item := NewMemoryItem(MemoryTypeSession, msg)
			item.Importance = 4
			item.SetMetadata("session_id", sessionID)
			item.SetMetadata("message_index", i)
			m.Save(item) // 忽略错误，继续保存其他
		}
	}

	return nil
}

// FindRelatedMemories 查找相关记忆
func (m *Manager) FindRelatedMemories(item *MemoryItem, limit int) ([]*MemoryItem, error) {
	// 基于标签和类型查找相关记忆
	q := DefaultQuery()
	q.Types = []MemoryType{item.Type}
	q.Tags = item.Tags
	q.Limit = limit
	q.OrderBy = OrderByImportance
	q.OrderDesc = true

	return m.store.Query(q)
}

// ExportMemories 导出记忆
func (m *Manager) ExportMemories(memType MemoryType) ([]*MemoryItem, error) {
	q := DefaultQuery()
	if memType != "" {
		q.Types = []MemoryType{memType}
	}
	q.Limit = 10000
	return m.store.Query(q)
}

// ImportMemories 导入记忆
func (m *Manager) ImportMemories(items []*MemoryItem) error {
	for _, item := range items {
		// 重新生成 ID 避免冲突
		item.ID = generateID()
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
		if err := m.Save(item); err != nil {
			return err
		}
	}
	return nil
}

// Backup 备份记忆到文件
func (m *Manager) Backup(filepath string) error {
	items, err := m.ExportMemories("")
	if err != nil {
		return err
	}

	// 序列化为 JSON
	// TODO: 实现备份逻辑
	_ = items
	_ = filepath
	return nil
}

// Restore 从文件恢复记忆
func (m *Manager) Restore(filepath string) error {
	// TODO: 实现恢复逻辑
	_ = filepath
	return nil
}
