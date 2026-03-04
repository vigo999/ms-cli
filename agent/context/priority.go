package context

import (
	"strings"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Priority 消息优先级
type Priority int

const (
	// PriorityCritical 关键消息（系统提示词、重要工具结果）
	PriorityCritical Priority = 100
	// PriorityHigh 高优先级（用户当前任务、最近的对话）
	PriorityHigh Priority = 75
	// PriorityMedium 中优先级（一般对话历史）
	PriorityMedium Priority = 50
	// PriorityLow 低优先级（旧对话、摘要信息）
	PriorityLow Priority = 25
	// PriorityDiscardable 可丢弃（已摘要的旧消息）
	PriorityDiscardable Priority = 10
)

// MessageMetadata 消息元数据
type MessageMetadata struct {
	Priority   Priority
	Topic      string
	Timestamp  time.Time
	Importance int // 1-10 自定义重要性
	Tags       []string
}

// PriorityScorer 优先级评分器
type PriorityScorer struct {
	// 配置
	recentWindow     time.Duration // 最近消息窗口
	keywords         map[string]Priority
	topicBoost       map[string]Priority
}

// NewPriorityScorer 创建新的优先级评分器
func NewPriorityScorer() *PriorityScorer {
	return &PriorityScorer{
		recentWindow: 5 * time.Minute,
		keywords: map[string]Priority{
			"error":   PriorityHigh,
			"failed":  PriorityHigh,
			"success": PriorityMedium,
			"result":  PriorityMedium,
			"summary": PriorityLow,
		},
		topicBoost: make(map[string]Priority),
	}
}

// SetRecentWindow 设置最近消息窗口
func (ps *PriorityScorer) SetRecentWindow(d time.Duration) {
	ps.recentWindow = d
}

// AddKeywordBoost 添加关键词优先级提升
func (ps *PriorityScorer) AddKeywordBoost(keyword string, boost Priority) {
	ps.keywords[strings.ToLower(keyword)] = boost
}

// AddTopicBoost 添加主题优先级提升
func (ps *PriorityScorer) AddTopicBoost(topic string, boost Priority) {
	ps.topicBoost[topic] = boost
}

// ScoreMessage 为消息评分
func (ps *PriorityScorer) ScoreMessage(msg llm.Message, index int, total int) Priority {
	baseScore := ps.getBasePriority(msg)
	
	// 基于位置的调整（越新的消息优先级越高）
	recencyBoost := ps.calculateRecencyBoost(index, total)
	
	// 基于内容的调整
	contentBoost := ps.calculateContentBoost(msg.Content)
	
	// 基于时间的调整
	timeBoost := ps.calculateTimeBoost(msg)
	
	finalScore := baseScore + recencyBoost + contentBoost + timeBoost
	
	// 确保在有效范围内
	if finalScore > PriorityCritical {
		finalScore = PriorityCritical
	}
	if finalScore < PriorityDiscardable {
		finalScore = PriorityDiscardable
	}
	
	return finalScore
}

// getBasePriority 获取消息的基础优先级
func (ps *PriorityScorer) getBasePriority(msg llm.Message) Priority {
	switch msg.Role {
	case "system":
		return PriorityCritical
	case "user":
		return PriorityHigh
	case "assistant":
		return PriorityMedium
	case "tool":
		// 工具结果根据内容判断
		if len(msg.Content) > 1000 {
			return PriorityMedium // 长结果可能包含重要信息
		}
		return PriorityLow
	default:
		return PriorityMedium
	}
}

// calculateRecencyBoost 计算基于位置的优先级提升
func (ps *PriorityScorer) calculateRecencyBoost(index, total int) Priority {
	if total == 0 {
		return 0
	}
	
	// 最新的消息获得更高优先级
	position := float64(index) / float64(total)
	if position > 0.8 { // 最近 20% 的消息
		return 15
	} else if position > 0.5 { // 中间 30%
		return 5
	}
	return 0
}

// calculateContentBoost 计算基于内容的优先级提升
func (ps *PriorityScorer) calculateContentBoost(content string) Priority {
	contentLower := strings.ToLower(content)
	boost := Priority(0)
	
	for keyword, priority := range ps.keywords {
		if strings.Contains(contentLower, keyword) {
			boost += priority / 5 // 关键词提升为其优先级的 20%
		}
	}
	
	return boost
}

// calculateTimeBoost 计算基于时间的优先级提升
func (ps *PriorityScorer) calculateTimeBoost(msg llm.Message) Priority {
	// 这里可以基于消息的时间戳计算
	// 由于 llm.Message 没有 Timestamp 字段，暂时返回 0
	return 0
}

// PrioritizedMessage 带优先级的消息
type PrioritizedMessage struct {
	Message  llm.Message
	Priority Priority
	Index    int // 原始索引
}

// ByPriority 按优先级排序（高优先级在前）
type ByPriority []PrioritizedMessage

func (a ByPriority) Len() int           { return len(a) }
func (a ByPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPriority) Less(i, j int) bool { return a[i].Priority > a[j].Priority }

// PriorityQueue 优先级队列
type PriorityQueue struct {
	items []PrioritizedMessage
}

// NewPriorityQueue 创建新的优先级队列
func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		items: make([]PrioritizedMessage, 0),
	}
}

// Push 添加消息到队列
func (pq *PriorityQueue) Push(pm PrioritizedMessage) {
	pq.items = append(pq.items, pm)
}

// Pop 弹出最高优先级的消息
func (pq *PriorityQueue) Pop() (PrioritizedMessage, bool) {
	if len(pq.items) == 0 {
		return PrioritizedMessage{}, false
	}
	
	// 找到最高优先级的消息
	maxIdx := 0
	for i := 1; i < len(pq.items); i++ {
		if pq.items[i].Priority > pq.items[maxIdx].Priority {
			maxIdx = i
		}
	}
	
	item := pq.items[maxIdx]
	pq.items = append(pq.items[:maxIdx], pq.items[maxIdx+1:]...)
	return item, true
}

// Peek 查看最高优先级的消息（不移除）
func (pq *PriorityQueue) Peek() (PrioritizedMessage, bool) {
	if len(pq.items) == 0 {
		return PrioritizedMessage{}, false
	}
	
	maxIdx := 0
	for i := 1; i < len(pq.items); i++ {
		if pq.items[i].Priority > pq.items[maxIdx].Priority {
			maxIdx = i
		}
	}
	
	return pq.items[maxIdx], true
}

// Len 返回队列长度
func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

// GetByPriorityRange 获取指定优先级范围内的消息
func (pq *PriorityQueue) GetByPriorityRange(min, max Priority) []PrioritizedMessage {
	result := make([]PrioritizedMessage, 0)
	for _, item := range pq.items {
		if item.Priority >= min && item.Priority <= max {
			result = append(result, item)
		}
	}
	return result
}

// GlobalPriorityScorer 全局优先级评分器
var GlobalPriorityScorer = NewPriorityScorer()

// ScoreMessage 使用全局评分器为消息评分
func ScoreMessage(msg llm.Message, index, total int) Priority {
	return GlobalPriorityScorer.ScoreMessage(msg, index, total)
}
