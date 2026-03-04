package context

import (
	"fmt"
	"sync"
)

// BudgetAllocation 预算分配配置
type BudgetAllocation struct {
	SystemPercent     int // 系统提示词预算百分比
	HistoryPercent    int // 历史消息预算百分比
	ToolResultPercent int // 工具结果预算百分比
	ReservePercent    int // 预留预算百分比
}

// DefaultBudgetAllocation 返回默认预算分配
// 默认: 系统10% + 历史70% + 工具结果10% + 预留10%
func DefaultBudgetAllocation() BudgetAllocation {
	return BudgetAllocation{
		SystemPercent:     10,
		HistoryPercent:    70,
		ToolResultPercent: 10,
		ReservePercent:    10,
	}
}

// Validate 验证预算分配是否有效
func (ba BudgetAllocation) Validate() error {
	total := ba.SystemPercent + ba.HistoryPercent + ba.ToolResultPercent + ba.ReservePercent
	if total != 100 {
		return fmt.Errorf("budget allocation must sum to 100, got %d", total)
	}
	if ba.SystemPercent < 0 || ba.HistoryPercent < 0 || ba.ToolResultPercent < 0 || ba.ReservePercent < 0 {
		return fmt.Errorf("budget percentages cannot be negative")
	}
	return nil
}

// Budget 管理 Token 预算
type Budget struct {
	mu         sync.RWMutex
	maxTokens  int
	allocation BudgetAllocation

	// 各分类预算限制
	systemLimit     int
	historyLimit    int
	toolResultLimit int
	reserveLimit    int

	// 当前使用情况
	systemUsed     int
	historyUsed    int
	toolResultUsed int
}

// NewBudget 创建新的预算管理器
func NewBudget(maxTokens int, allocation BudgetAllocation) (*Budget, error) {
	if maxTokens <= 0 {
		return nil, fmt.Errorf("max tokens must be positive")
	}
	if err := allocation.Validate(); err != nil {
		return nil, err
	}

	b := &Budget{
		maxTokens:    maxTokens,
		allocation:   allocation,
		systemLimit:  maxTokens * allocation.SystemPercent / 100,
		historyLimit: maxTokens * allocation.HistoryPercent / 100,
		toolResultLimit: maxTokens * allocation.ToolResultPercent / 100,
		reserveLimit: maxTokens * allocation.ReservePercent / 100,
	}
	return b, nil
}

// SetSystemUsage 设置系统提示词使用量
func (b *Budget) SetSystemUsage(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.systemUsed = tokens
}

// SetHistoryUsage 设置历史消息使用量
func (b *Budget) SetHistoryUsage(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.historyUsed = tokens
}

// SetToolResultUsage 设置工具结果使用量
func (b *Budget) SetToolResultUsage(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.toolResultUsed = tokens
}

// GetSystemBudget 获取系统提示词预算
func (b *Budget) GetSystemBudget() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.systemLimit
}

// GetHistoryBudget 获取历史消息预算
func (b *Budget) GetHistoryBudget() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.historyLimit
}

// GetToolResultBudget 获取工具结果预算
func (b *Budget) GetToolResultBudget() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.toolResultLimit
}

// GetReserveBudget 获取预留预算
func (b *Budget) GetReserveBudget() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.reserveLimit
}

// IsSystemOverBudget 检查系统提示词是否超预算
func (b *Budget) IsSystemOverBudget() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.systemUsed > b.systemLimit
}

// IsHistoryOverBudget 检查历史消息是否超预算
func (b *Budget) IsHistoryOverBudget() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.historyUsed > b.historyLimit
}

// IsToolResultOverBudget 检查工具结果是否超预算
func (b *Budget) IsToolResultOverBudget() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.toolResultUsed > b.toolResultLimit
}

// GetTotalUsed 获取总使用量
func (b *Budget) GetTotalUsed() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.systemUsed + b.historyUsed + b.toolResultUsed
}

// GetTotalAvailable 获取总可用量（不包括预留）
func (b *Budget) GetTotalAvailable() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.maxTokens - b.reserveLimit - b.GetTotalUsed()
}

// GetUsagePercent 获取使用百分比
func (b *Budget) GetUsagePercent() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.maxTokens == 0 {
		return 0
	}
	return float64(b.GetTotalUsed()) / float64(b.maxTokens) * 100
}

// ShouldCompact 判断是否需要压缩
// threshold: 触发压缩的阈值百分比 (0-100)
func (b *Budget) ShouldCompact(threshold float64) bool {
	return b.GetUsagePercent() > threshold
}

// GetStats 获取预算统计
func (b *Budget) GetStats() BudgetStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	totalUsed := b.systemUsed + b.historyUsed + b.toolResultUsed
	return BudgetStats{
		MaxTokens:       b.maxTokens,
		TotalUsed:       totalUsed,
		TotalAvailable:  b.maxTokens - totalUsed,
		ReserveLimit:    b.reserveLimit,
		UsagePercent:    float64(totalUsed) / float64(b.maxTokens) * 100,
		SystemUsed:      b.systemUsed,
		SystemLimit:     b.systemLimit,
		HistoryUsed:     b.historyUsed,
		HistoryLimit:    b.historyLimit,
		ToolResultUsed:  b.toolResultUsed,
		ToolResultLimit: b.toolResultLimit,
	}
}

// BudgetStats 预算统计
type BudgetStats struct {
	MaxTokens       int
	TotalUsed       int
	TotalAvailable  int
	ReserveLimit    int
	UsagePercent    float64
	SystemUsed      int
	SystemLimit     int
	HistoryUsed     int
	HistoryLimit    int
	ToolResultUsed  int
	ToolResultLimit int
}

// String 返回预算统计的字符串表示
func (s BudgetStats) String() string {
	return fmt.Sprintf(
		"Budget: %d/%d (%.1f%%), System: %d/%d, History: %d/%d, Tool: %d/%d, Reserve: %d",
		s.TotalUsed, s.MaxTokens, s.UsagePercent,
		s.SystemUsed, s.SystemLimit,
		s.HistoryUsed, s.HistoryLimit,
		s.ToolResultUsed, s.ToolResultLimit,
		s.ReserveLimit,
	)
}
