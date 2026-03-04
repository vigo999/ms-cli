package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// CompactStrategy 压缩策略类型
type CompactStrategy int

const (
	// CompactStrategySimple 简单策略：直接丢弃旧消息
	CompactStrategySimple CompactStrategy = iota
	// CompactStrategySummarize 摘要策略：将旧消息摘要为一句话
	CompactStrategySummarize
	// CompactStrategyPriority 优先级策略：基于优先级保留消息
	CompactStrategyPriority
	// CompactStrategyHybrid 混合策略：结合多种策略
	CompactStrategyHybrid
)

// String 返回策略名称
func (s CompactStrategy) String() string {
	switch s {
	case CompactStrategySimple:
		return "simple"
	case CompactStrategySummarize:
		return "summarize"
	case CompactStrategyPriority:
		return "priority"
	case CompactStrategyHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// ParseCompactStrategy 解析策略字符串
func ParseCompactStrategy(s string) CompactStrategy {
	switch strings.ToLower(s) {
	case "simple":
		return CompactStrategySimple
	case "summarize":
		return CompactStrategySummarize
	case "priority":
		return CompactStrategyPriority
	case "hybrid":
		return CompactStrategyHybrid
	default:
		return CompactStrategySimple
	}
}

// Compactor 上下文压缩器
type Compactor struct {
	strategy        CompactStrategy
	scorer          *PriorityScorer
	tokenizer       *Tokenizer
	maxKeepMessages int // 最大保留消息数
}

// CompactorConfig 压缩器配置
type CompactorConfig struct {
	Strategy        CompactStrategy
	MaxKeepMessages int
}

// NewCompactor 创建新的压缩器
func NewCompactor(cfg CompactorConfig) *Compactor {
	if cfg.MaxKeepMessages <= 0 {
		cfg.MaxKeepMessages = 20
	}
	return &Compactor{
		strategy:        cfg.Strategy,
		scorer:          NewPriorityScorer(),
		tokenizer:       NewTokenizer(),
		maxKeepMessages: cfg.MaxKeepMessages,
	}
}

// SetStrategy 设置压缩策略
func (c *Compactor) SetStrategy(s CompactStrategy) {
	c.strategy = s
}

// Compact 执行压缩
func (c *Compactor) Compact(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	if len(messages) <= c.maxKeepMessages {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}

	switch c.strategy {
	case CompactStrategySimple:
		return c.compactSimple(messages, systemMsg)
	case CompactStrategySummarize:
		return c.compactSummarize(messages, systemMsg)
	case CompactStrategyPriority:
		return c.compactPriority(messages, systemMsg)
	case CompactStrategyHybrid:
		return c.compactHybrid(messages, systemMsg)
	default:
		return c.compactSimple(messages, systemMsg)
	}
}

// compactSimple 简单压缩策略
func (c *Compactor) compactSimple(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 保留最近的消息
	keepCount := c.maxKeepMessages
	if systemMsg != nil {
		keepCount-- // 为系统消息留一个位置
	}
	
	if keepCount >= len(messages) {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}
	
	removed := len(messages) - keepCount
	result := messages[len(messages)-keepCount:]
	
	return result, CompactResult{
		Kept:      len(result),
		Removed:   removed,
		Strategy:  CompactStrategySimple,
		Summary:   fmt.Sprintf("Removed %d old messages", removed),
	}
}

// compactSummarize 摘要压缩策略
func (c *Compactor) compactSummarize(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 保留最近的消息
	keepCount := c.maxKeepMessages - 2 // 留出位置给摘要和系统消息
	if keepCount < 4 {
		keepCount = 4
	}
	
	if keepCount >= len(messages) {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}
	
	// 需要摘要的消息
	toSummarize := messages[:len(messages)-keepCount]
	
	// 生成摘要
	summary := c.generateSummary(toSummarize)
	summaryMsg := llm.NewSystemMessage(summary)
	
	// 保留的消息
	result := append([]llm.Message{summaryMsg}, messages[len(messages)-keepCount:]...)
	
	return result, CompactResult{
		Kept:      len(result),
		Removed:   len(toSummarize),
		Strategy:  CompactStrategySummarize,
		Summary:   summary,
	}
}

// compactPriority 优先级压缩策略
func (c *Compactor) compactPriority(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 为所有消息评分
	prioritized := make([]PrioritizedMessage, len(messages))
	for i, msg := range messages {
		prioritized[i] = PrioritizedMessage{
			Message:  msg,
			Priority: c.scorer.ScoreMessage(msg, i, len(messages)),
			Index:    i,
		}
	}
	
	// 按优先级排序
	sort.Sort(ByPriority(prioritized))
	
	// 保留优先级最高的消息
	keepCount := c.maxKeepMessages
	if keepCount > len(prioritized) {
		keepCount = len(prioritized)
	}
	
	// 获取要保留的消息
	kept := prioritized[:keepCount]
	removed := prioritized[keepCount:]
	
	// 按原始索引排序，保持顺序
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].Index < kept[j].Index
	})
	
	// 提取消息
	result := make([]llm.Message, len(kept))
	for i, pm := range kept {
		result[i] = pm.Message
	}
	
	return result, CompactResult{
		Kept:      len(result),
		Removed:   len(removed),
		Strategy:  CompactStrategyPriority,
		Summary:   fmt.Sprintf("Kept %d high-priority messages, removed %d", len(result), len(removed)),
	}
}

// compactHybrid 混合压缩策略
func (c *Compactor) compactHybrid(messages []llm.Message, systemMsg *llm.Message) ([]llm.Message, CompactResult) {
	// 策略：
	// 1. 保留最近的几条消息（高优先级）
	// 2. 基于优先级选择保留的较旧消息
	// 3. 将其他旧消息摘要
	
	recentCount := c.maxKeepMessages / 2 // 保留一半给最新消息
	if recentCount < 3 {
		recentCount = 3
	}
	
	if len(messages) <= c.maxKeepMessages {
		return messages, CompactResult{Kept: len(messages), Removed: 0}
	}
	
	// 保留最近的消息
	recentMessages := messages[len(messages)-recentCount:]
	
	// 处理旧消息
	oldMessages := messages[:len(messages)-recentCount]
	
	// 对旧消息进行优先级评分
	prioritized := make([]PrioritizedMessage, len(oldMessages))
	for i, msg := range oldMessages {
		prioritized[i] = PrioritizedMessage{
			Message:  msg,
			Priority: c.scorer.ScoreMessage(msg, i, len(oldMessages)),
			Index:    i,
		}
	}
	
	// 按优先级排序
	sort.Sort(ByPriority(prioritized))
	
	// 保留高优先级的旧消息
	oldKeepCount := c.maxKeepMessages - recentCount - 1 // 留出位置给摘要
	if oldKeepCount < 0 {
		oldKeepCount = 0
	}
	
	var result []llm.Message
	
	// 如果有需要摘要的旧消息，添加摘要
	if len(prioritized) > oldKeepCount {
		toSummarize := make([]llm.Message, len(prioritized)-oldKeepCount)
		for i := oldKeepCount; i < len(prioritized); i++ {
			toSummarize[i-oldKeepCount] = prioritized[i].Message
		}
		summary := c.generateSummary(toSummarize)
		result = append(result, llm.NewSystemMessage(summary))
	}
	
	// 添加保留的高优先级旧消息
	if oldKeepCount > 0 {
		highPriorityOld := prioritized[:oldKeepCount]
		// 按原始索引排序
		sort.Slice(highPriorityOld, func(i, j int) bool {
			return highPriorityOld[i].Index < highPriorityOld[j].Index
		})
		for _, pm := range highPriorityOld {
			result = append(result, pm.Message)
		}
	}
	
	// 添加最近的消息
	result = append(result, recentMessages...)
	
	removed := len(messages) - len(result) + 1 // +1 for summary
	
	return result, CompactResult{
		Kept:      len(result),
		Removed:   removed,
		Strategy:  CompactStrategyHybrid,
		Summary:   fmt.Sprintf("Hybrid compact: kept %d messages including %d recent", len(result), recentCount),
	}
}

// generateSummary 生成消息摘要
func (c *Compactor) generateSummary(messages []llm.Message) string {
	userCount := 0
	assistantCount := 0
	toolCount := 0
	
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "tool":
			toolCount++
		}
	}
	
	parts := []string{"[Context Summary]"}
	parts = append(parts, fmt.Sprintf("Earlier conversation: %d messages", len(messages)))
	
	if userCount > 0 {
		parts = append(parts, fmt.Sprintf("%d user messages", userCount))
	}
	if assistantCount > 0 {
		parts = append(parts, fmt.Sprintf("%d assistant responses", assistantCount))
	}
	if toolCount > 0 {
		parts = append(parts, fmt.Sprintf("%d tool calls", toolCount))
	}
	
	return strings.Join(parts, ", ")
}

// CompactResult 压缩结果
type CompactResult struct {
	Kept     int
	Removed  int
	Strategy CompactStrategy
	Summary  string
}

// String 返回压缩结果的字符串表示
func (r CompactResult) String() string {
	return fmt.Sprintf("Compact [%s]: kept %d, removed %d - %s",
		r.Strategy, r.Kept, r.Removed, r.Summary)
}

// SimpleCompact 简单压缩函数（保持向后兼容）
func SimpleCompact(messages []llm.Message, maxKeep int) []llm.Message {
	if len(messages) <= maxKeep {
		return messages
	}
	return messages[len(messages)-maxKeep:]
}
