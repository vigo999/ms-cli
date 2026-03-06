package context

import (
	"fmt"
	"sync"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// ManagerConfig holds the manager configuration.
type ManagerConfig struct {
	MaxTokens           int
	ReserveTokens       int
	CompactionThreshold float64
	MaxHistoryRounds    int

	// 新增配置
	EnableSmartCompact bool             // 启用智能压缩
	CompactStrategy    CompactStrategy  // 压缩策略
	Allocation         BudgetAllocation // 预算分配
	EnablePriority     bool             // 启用优先级系统
}

// DefaultManagerConfig 返回默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxTokens:           24000,
		ReserveTokens:       4000,
		CompactionThreshold: 0.85,
		MaxHistoryRounds:    10,
		EnableSmartCompact:  true,
		CompactStrategy:     CompactStrategyHybrid,
		Allocation:          DefaultBudgetAllocation(),
		EnablePriority:      true,
	}
}

// Manager manages conversation context.
type Manager struct {
	config   ManagerConfig
	mu       sync.RWMutex
	messages []llm.Message
	system   *llm.Message
	usage    TokenUsage

	// 增强组件
	budget    *Budget
	tokenizer *Tokenizer
	compactor *Compactor
	scorer    *PriorityScorer

	// 统计
	stats Stats
}

// TokenUsage represents token usage statistics.
type TokenUsage struct {
	Current   int
	Max       int
	Reserved  int
	Available int
}

// Stats 上下文统计
type Stats struct {
	MessageCount    int
	ToolCallCount   int
	CompactCount    int
	LastCompactAt   *time.Time
	TotalTokensUsed int
}

// NewManager creates a new context manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 24000
	}
	if cfg.ReserveTokens == 0 {
		cfg.ReserveTokens = 4000
	}
	if cfg.CompactionThreshold == 0 {
		cfg.CompactionThreshold = 0.85
	}
	if cfg.MaxHistoryRounds == 0 {
		cfg.MaxHistoryRounds = 10
	}
	if cfg.Allocation.SystemPercent == 0 &&
		cfg.Allocation.HistoryPercent == 0 &&
		cfg.Allocation.ToolResultPercent == 0 &&
		cfg.Allocation.ReservePercent == 0 {
		cfg.Allocation = DefaultBudgetAllocation()
	}

	// 创建预算管理器
	budget, _ := NewBudget(cfg.MaxTokens, cfg.Allocation)

	// 创建压缩器
	compactor := NewCompactor(CompactorConfig{
		Strategy:        cfg.CompactStrategy,
		MaxKeepMessages: cfg.MaxHistoryRounds * 2,
	})

	m := &Manager{
		config:    cfg,
		messages:  make([]llm.Message, 0),
		budget:    budget,
		tokenizer: NewTokenizer(),
		compactor: compactor,
		scorer:    NewPriorityScorer(),
		usage: TokenUsage{
			Max:       cfg.MaxTokens,
			Reserved:  cfg.ReserveTokens,
			Available: cfg.MaxTokens - cfg.ReserveTokens,
		},
	}

	return m
}

// SetSystemPrompt sets the system prompt.
func (m *Manager) SetSystemPrompt(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := llm.NewSystemMessage(content)
	m.system = &msg

	// 更新系统预算
	if m.budget != nil {
		systemTokens := m.tokenizer.EstimateMessage(msg)
		m.budget.SetSystemUsage(systemTokens)
	}

	m.recalculateUsage()
}

// GetSystemPrompt returns the system prompt.
func (m *Manager) GetSystemPrompt() *llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.system == nil {
		return nil
	}
	msg := *m.system
	return &msg
}

// AddMessage adds a message to the context.
func (m *Manager) AddMessage(msg llm.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 估算新消息的 Token
	msgTokens := m.tokenizer.EstimateMessage(msg)

	// 检查是否需要压缩
	if m.shouldCompactLocked(msgTokens) {
		if err := m.compactLocked(); err != nil {
			return fmt.Errorf("compact context: %w", err)
		}
	}

	// 再次检查预算（压缩后）
	if m.budget != nil {
		currentHistory := m.tokenizer.EstimateMessages(m.messages)
		if currentHistory+msgTokens > m.budget.GetHistoryBudget() {
			// 仍然超预算，可能需要丢弃一些消息
			if err := m.emergencyCompactLocked(); err != nil {
				return fmt.Errorf("emergency compact: %w", err)
			}
		}
	}

	m.messages = append(m.messages, msg)

	// 更新预算
	if m.budget != nil {
		m.budget.SetHistoryUsage(m.tokenizer.EstimateMessages(m.messages))
	}

	m.recalculateUsage()
	m.stats.MessageCount++
	if msg.Role == "tool" {
		m.stats.ToolCallCount++
	}

	return nil
}

// AddToolResult adds a tool result to the context.
func (m *Manager) AddToolResult(callID, content string) error {
	return m.AddMessage(llm.NewToolMessage(callID, content))
}

// GetMessages returns all messages including system prompt.
func (m *Manager) GetMessages() []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]llm.Message, 0, len(m.messages)+1)
	if m.system != nil {
		result = append(result, *m.system)
	}
	result = append(result, m.messages...)
	return result
}

// GetNonSystemMessages returns only non-system messages.
func (m *Manager) GetNonSystemMessages() []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]llm.Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// Clear clears all messages except system prompt.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]llm.Message, 0)
	if m.budget != nil {
		m.budget.SetHistoryUsage(0)
	}
	m.recalculateUsage()
}

// Compact manually triggers context compaction.
func (m *Manager) Compact() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.compactLocked()
}

// TokenUsage returns current token usage.
func (m *Manager) TokenUsage() TokenUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.usage
}

// EstimateTokens estimates token count for messages.
func (m *Manager) EstimateTokens(msgs []llm.Message) int {
	return m.tokenizer.EstimateMessages(msgs)
}

// IsWithinBudget checks if adding a message would exceed budget.
func (m *Manager) IsWithinBudget(msg llm.Message) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.budget == nil {
		// 回退到简单估算
		estimated := m.tokenizer.EstimateMessages(m.messages) + m.tokenizer.EstimateMessage(msg)
		return estimated <= m.config.MaxTokens-m.config.ReserveTokens
	}

	currentHistory := m.tokenizer.EstimateMessages(m.messages)
	msgTokens := m.tokenizer.EstimateMessage(msg)
	return currentHistory+msgTokens <= m.budget.GetHistoryBudget()
}

// GetBudgetStats returns budget statistics.
func (m *Manager) GetBudgetStats() BudgetStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.budget == nil {
		return BudgetStats{}
	}
	return m.budget.GetStats()
}

// GetStats returns context statistics.
func (m *Manager) GetStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"total_messages":    len(m.messages),
		"has_system_prompt": m.system != nil,
		"token_usage":       m.usage,
		"max_tokens":        m.config.MaxTokens,
		"compact_count":     m.stats.CompactCount,
		"tool_call_count":   m.stats.ToolCallCount,
		"last_compact_at":   m.stats.LastCompactAt,
	}
}

// GetDetailedStats returns detailed statistics.
func (m *Manager) GetDetailedStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgetStats := BudgetStats{}
	if m.budget != nil {
		budgetStats = m.budget.GetStats()
	}

	stats := map[string]any{
		"messages": map[string]any{
			"total":     len(m.messages),
			"user":      m.countByRole("user"),
			"assistant": m.countByRole("assistant"),
			"tool":      m.countByRole("tool"),
		},
		"tokens": map[string]any{
			"current":   m.usage.Current,
			"max":       m.usage.Max,
			"reserved":  m.usage.Reserved,
			"available": m.usage.Available,
		},
		"budget": budgetStats,
		"stats":  m.stats,
	}

	return stats
}

// shouldCompactLocked checks if compaction is needed (must hold lock).
func (m *Manager) shouldCompactLocked(additionalTokens int) bool {
	if m.budget != nil {
		current := m.tokenizer.EstimateMessages(m.messages) + additionalTokens
		return m.budget.ShouldCompact(float64(current) / float64(m.config.MaxTokens) * 100)
	}

	// 回退到简单估算
	estimatedTokens := m.tokenizer.EstimateMessages(m.messages) + additionalTokens
	return float64(estimatedTokens) > float64(m.config.MaxTokens)*m.config.CompactionThreshold
}

// compactLocked compacts the context (must hold lock).
func (m *Manager) compactLocked() error {
	if len(m.messages) <= m.config.MaxHistoryRounds {
		return nil
	}

	// 使用智能压缩
	if m.config.EnableSmartCompact && m.compactor != nil {
		compacted, result := m.compactor.Compact(m.messages, m.system)
		m.messages = compacted
		m.stats.CompactCount++
		now := time.Now()
		m.stats.LastCompactAt = &now
		_ = result // 可以在日志中记录
	} else {
		// 简单压缩
		keepCount := m.config.MaxHistoryRounds * 2
		if keepCount < len(m.messages) {
			removed := len(m.messages) - keepCount
			summary := fmt.Sprintf("[Earlier conversation: %d messages summarized]", removed)
			summaryMsg := llm.NewSystemMessage(summary)
			m.messages = append([]llm.Message{summaryMsg}, m.messages[removed:]...)
			m.stats.CompactCount++
			now := time.Now()
			m.stats.LastCompactAt = &now
		}
	}

	// 更新预算
	if m.budget != nil {
		m.budget.SetHistoryUsage(m.tokenizer.EstimateMessages(m.messages))
	}

	m.recalculateUsage()
	return nil
}

// emergencyCompactLocked performs emergency compaction when budget is exceeded.
func (m *Manager) emergencyCompactLocked() error {
	// 紧急压缩：保留更少消息
	keepCount := m.config.MaxHistoryRounds
	if keepCount < 4 {
		keepCount = 4
	}

	if len(m.messages) > keepCount {
		removed := len(m.messages) - keepCount
		m.messages = m.messages[removed:]
		m.stats.CompactCount++
		now := time.Now()
		m.stats.LastCompactAt = &now
	}

	// 更新预算
	if m.budget != nil {
		m.budget.SetHistoryUsage(m.tokenizer.EstimateMessages(m.messages))
	}

	return nil
}

// recalculateUsage recalculates token usage (must hold lock).
func (m *Manager) recalculateUsage() {
	total := m.tokenizer.EstimateMessages(m.messages)
	if m.system != nil {
		total += m.tokenizer.EstimateMessage(*m.system)
	}

	m.usage = TokenUsage{
		Current:   total,
		Max:       m.config.MaxTokens,
		Reserved:  m.config.ReserveTokens,
		Available: m.config.MaxTokens - total - m.config.ReserveTokens,
	}

	m.stats.TotalTokensUsed = total
}

// countByRole counts messages by role (must hold lock).
func (m *Manager) countByRole(role string) int {
	count := 0
	for _, msg := range m.messages {
		if msg.Role == role {
			count++
		}
	}
	return count
}

// SetCompactStrategy sets the compaction strategy.
func (m *Manager) SetCompactStrategy(s CompactStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.CompactStrategy = s
	if m.compactor != nil {
		m.compactor.SetStrategy(s)
	}
}

// GetMessagePriority returns the priority of a message.
func (m *Manager) GetMessagePriority(index int) Priority {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if index < 0 || index >= len(m.messages) {
		return PriorityLow
	}

	return m.scorer.ScoreMessage(m.messages[index], index, len(m.messages))
}

// TruncateTo truncates messages to the specified count (keeping the most recent).
func (m *Manager) TruncateTo(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if count < 0 {
		count = 0
	}
	if count >= len(m.messages) {
		return
	}

	m.messages = m.messages[len(m.messages)-count:]
	m.recalculateUsage()
}

// ReplaceMessages replaces all non-system messages without running compaction.
func (m *Manager) ReplaceMessages(msgs []llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]llm.Message, len(msgs))
	copy(m.messages, msgs)

	if m.budget != nil {
		m.budget.SetHistoryUsage(m.tokenizer.EstimateMessages(m.messages))
	}

	toolCount := 0
	for _, msg := range m.messages {
		if msg.Role == "tool" {
			toolCount++
		}
	}
	m.stats.MessageCount = len(m.messages)
	m.stats.ToolCallCount = toolCount
	m.recalculateUsage()
}
