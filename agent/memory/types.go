package memory

import (
	"fmt"
	"math/rand"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	// MemoryTypeSession 会话历史
	MemoryTypeSession MemoryType = "session"
	// MemoryTypeFact 事实知识
	MemoryTypeFact MemoryType = "fact"
	// MemoryTypeTask 任务记录
	MemoryTypeTask MemoryType = "task"
	// MemoryTypePreference 用户偏好
	MemoryTypePreference MemoryType = "preference"
	// MemoryTypeCode 代码片段
	MemoryTypeCode MemoryType = "code"
	// MemoryTypeDecision 决策记录
	MemoryTypeDecision MemoryType = "decision"
)

// String returns the string representation.
func (mt MemoryType) String() string {
	return string(mt)
}

// IsValid checks if the memory type is valid.
func (mt MemoryType) IsValid() bool {
	switch mt {
	case MemoryTypeSession, MemoryTypeFact, MemoryTypeTask,
		MemoryTypePreference, MemoryTypeCode, MemoryTypeDecision:
		return true
	}
	return false
}

// MemoryItem 单个记忆项
type MemoryItem struct {
	ID         string
	Type       MemoryType
	Content    string
	Metadata   map[string]any
	Tags       []string
	Embedding  []float32 // 可选：用于语义检索
	Importance int       // 1-10 重要性评分

	// 时间戳
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time // TTL 支持，nil 表示永不过期

	// 统计
	AccessCount int
	LastAccess  *time.Time
}

// NewMemoryItem creates a new memory item.
func NewMemoryItem(memType MemoryType, content string) *MemoryItem {
	now := time.Now()
	return &MemoryItem{
		ID:         generateID(),
		Type:       memType,
		Content:    content,
		Metadata:   make(map[string]any),
		Tags:       make([]string, 0),
		Importance: 5, // 默认中等重要性
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// SetTTL sets the TTL (time-to-live) for the memory item.
func (item *MemoryItem) SetTTL(duration time.Duration) {
	expires := time.Now().Add(duration)
	item.ExpiresAt = &expires
}

// IsExpired checks if the memory item has expired.
func (item *MemoryItem) IsExpired() bool {
	if item.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*item.ExpiresAt)
}

// AddTag adds a tag to the memory item.
func (item *MemoryItem) AddTag(tag string) {
	for _, t := range item.Tags {
		if t == tag {
			return
		}
	}
	item.Tags = append(item.Tags, tag)
}

// RemoveTag removes a tag from the memory item.
func (item *MemoryItem) RemoveTag(tag string) {
	for i, t := range item.Tags {
		if t == tag {
			item.Tags = append(item.Tags[:i], item.Tags[i+1:]...)
			return
		}
	}
}

// SetMetadata sets a metadata value.
func (item *MemoryItem) SetMetadata(key string, value any) {
	if item.Metadata == nil {
		item.Metadata = make(map[string]any)
	}
	item.Metadata[key] = value
}

// GetMetadata gets a metadata value.
func (item *MemoryItem) GetMetadata(key string) (any, bool) {
	if item.Metadata == nil {
		return nil, false
	}
	v, ok := item.Metadata[key]
	return v, ok
}

// RecordAccess records an access to the memory item.
func (item *MemoryItem) RecordAccess() {
	item.AccessCount++
	now := time.Now()
	item.LastAccess = &now
}

// Query 查询参数
type Query struct {
	Types      []MemoryType
	Keywords   []string
	Tags       []string
	Metadata   map[string]any
	TimeRange  *TimeRange
	Limit      int
	Offset     int
	MinImportance int
	OrderBy    OrderBy
	OrderDesc  bool
}

// TimeRange 时间范围
type TimeRange struct {
	Start *time.Time
	End   *time.Time
}

// OrderBy 排序字段
type OrderBy string

const (
	OrderByCreatedAt   OrderBy = "created_at"
	OrderByUpdatedAt   OrderBy = "updated_at"
	OrderByImportance  OrderBy = "importance"
	OrderByAccessCount OrderBy = "access_count"
	OrderByLastAccess  OrderBy = "last_access"
)

// DefaultQuery returns a default query.
func DefaultQuery() Query {
	return Query{
		Limit:         10,
		Offset:        0,
		MinImportance: 0,
		OrderBy:       OrderByCreatedAt,
		OrderDesc:     true,
	}
}

// MemoryStats 记忆统计
type MemoryStats struct {
	TotalCount     int64
	ByType         map[MemoryType]int64
	AvgImportance  float64
	ExpiredCount   int64
	TotalSizeBytes int64
}

// Config 记忆系统配置
type Config struct {
	StorePath        string
	MaxItems         int
	MaxBytes         int64
	DefaultTTL       time.Duration
	EnableEmbedding  bool
	AutoCompact      bool
	CompactThreshold float64
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		MaxItems:         1000,
		MaxBytes:         10 * 1024 * 1024, // 10MB
		EnableEmbedding:  false,
		AutoCompact:      true,
		CompactThreshold: 0.9,
	}
}

// generateID generates a unique ID for memory items.
func generateID() string {
	return fmt.Sprintf("mem_%d_%d", time.Now().UnixNano(), rand.Int63())
}
