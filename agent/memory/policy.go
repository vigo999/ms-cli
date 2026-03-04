package memory

import (
	"time"
)

// Policy controls memory retention and limits.
type Policy struct {
	MaxItems        int
	MaxBytes        int64
	DefaultTTL      time.Duration
	MinImportance   int
	RetentionRules  []RetentionRule
}

// RetentionRule 保留规则
type RetentionRule struct {
	Type           MemoryType
	MaxAge         time.Duration
	MinImportance  int
	MaxCount       int
}

// DefaultPolicy returns a default policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxItems:      1000,
		MaxBytes:      10 * 1024 * 1024, // 10MB
		DefaultTTL:    7 * 24 * time.Hour, // 7 days
		MinImportance: 1,
		RetentionRules: []RetentionRule{
			{Type: MemoryTypeSession, MaxAge: 24 * time.Hour, MinImportance: 3},
			{Type: MemoryTypeFact, MaxAge: 30 * 24 * time.Hour, MinImportance: 5},
			{Type: MemoryTypeTask, MaxAge: 7 * 24 * time.Hour, MinImportance: 3},
			{Type: MemoryTypePreference, MaxAge: 0, MinImportance: 1}, // never expire
			{Type: MemoryTypeCode, MaxAge: 14 * 24 * time.Hour, MinImportance: 4},
			{Type: MemoryTypeDecision, MaxAge: 7 * 24 * time.Hour, MinImportance: 4},
		},
	}
}

// ShouldKeep 判断是否应该保留该记忆项
func (p *Policy) ShouldKeep(item *MemoryItem) bool {
	// 检查重要性
	if item.Importance < p.MinImportance {
		return false
	}

	// 检查类型特定规则
	for _, rule := range p.RetentionRules {
		if rule.Type == item.Type {
			// 检查最大年龄
			if rule.MaxAge > 0 {
				age := time.Since(item.CreatedAt)
				if age > rule.MaxAge && item.Importance < rule.MinImportance {
					return false
				}
			}
			break
		}
	}

	return true
}

// GetTTLForType 获取指定类型的默认 TTL
func (p *Policy) GetTTLForType(memType MemoryType) time.Duration {
	for _, rule := range p.RetentionRules {
		if rule.Type == memType {
			if rule.MaxAge > 0 {
				return rule.MaxAge
			}
			return 0 // never expire
		}
	}
	return p.DefaultTTL
}

// PolicyEvaluator 策略评估器
type PolicyEvaluator struct {
	policy Policy
}

// NewPolicyEvaluator 创建新的策略评估器
func NewPolicyEvaluator(policy Policy) *PolicyEvaluator {
	return &PolicyEvaluator{policy: policy}
}

// Evaluate 评估记忆项
func (pe *PolicyEvaluator) Evaluate(item *MemoryItem) EvaluationResult {
	result := EvaluationResult{
		Item:       item,
		ShouldKeep: true,
		Reasons:    make([]string, 0),
	}

	// 检查重要性
	if item.Importance < pe.policy.MinImportance {
		result.ShouldKeep = false
		result.Reasons = append(result.Reasons, "importance too low")
		return result
	}

	// 检查过期
	if item.IsExpired() {
		result.ShouldKeep = false
		result.Reasons = append(result.Reasons, "expired")
		return result
	}

	// 检查类型规则
	for _, rule := range pe.policy.RetentionRules {
		if rule.Type == item.Type {
			if rule.MaxAge > 0 {
				age := time.Since(item.CreatedAt)
				if age > rule.MaxAge && item.Importance < rule.MinImportance {
					result.ShouldKeep = false
					result.Reasons = append(result.Reasons, "exceeds max age")
				}
			}
			break
		}
	}

	return result
}

// EvaluationResult 评估结果
type EvaluationResult struct {
	Item       *MemoryItem
	ShouldKeep bool
	Reasons    []string
}

// CompactionPolicy 压缩策略
type CompactionPolicy struct {
	TriggerThreshold float64 // 触发压缩的阈值 (0-1)
	TargetRatio      float64 // 压缩后的目标比例 (0-1)
	PriorityFactors  []PriorityFactor
}

// PriorityFactor 优先级因子
type PriorityFactor struct {
	Name   string
	Weight float64
	Score  func(*MemoryItem) float64
}

// DefaultCompactionPolicy returns default compaction policy.
func DefaultCompactionPolicy() CompactionPolicy {
	return CompactionPolicy{
		TriggerThreshold: 0.9,
		TargetRatio:      0.7,
		PriorityFactors: []PriorityFactor{
			{
				Name:   "importance",
				Weight: 0.4,
				Score: func(item *MemoryItem) float64 {
					return float64(item.Importance) / 10.0
				},
			},
			{
				Name:   "recency",
				Weight: 0.3,
				Score: func(item *MemoryItem) float64 {
					age := time.Since(item.CreatedAt)
					// 越新分数越高，7天前为0分
					maxAge := 7 * 24 * time.Hour
					if age > maxAge {
						return 0
					}
					return 1.0 - float64(age)/float64(maxAge)
				},
			},
			{
				Name:   "access",
				Weight: 0.2,
				Score: func(item *MemoryItem) float64 {
					// 访问次数越多分数越高
					if item.AccessCount > 10 {
						return 1.0
					}
					return float64(item.AccessCount) / 10.0
				},
			},
			{
				Name:   "type",
				Weight: 0.1,
				Score: func(item *MemoryItem) float64 {
					// 不同类型不同权重
					switch item.Type {
					case MemoryTypePreference:
						return 1.0
					case MemoryTypeFact:
						return 0.8
					case MemoryTypeCode:
						return 0.7
					case MemoryTypeTask:
						return 0.6
					case MemoryTypeDecision:
						return 0.5
					case MemoryTypeSession:
						return 0.3
					default:
						return 0.5
					}
				},
			},
		},
	}
}

// CalculatePriority 计算记忆项的优先级分数
func (cp *CompactionPolicy) CalculatePriority(item *MemoryItem) float64 {
	var totalScore, totalWeight float64

	for _, factor := range cp.PriorityFactors {
		score := factor.Score(item)
		totalScore += score * factor.Weight
		totalWeight += factor.Weight
	}

	if totalWeight == 0 {
		return 0
	}
	return totalScore / totalWeight
}
