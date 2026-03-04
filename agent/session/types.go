package session

import (
	"fmt"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// ID 会话唯一标识符
type ID string

// Session 会话实体
type Session struct {
	ID        ID
	Name      string
	WorkDir   string
	Messages  []llm.Message
	Metadata  Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	Archived  bool
}

// Metadata 会话元数据
type Metadata struct {
	TotalTokens   int
	MessageCount  int
	ToolCallCount int
	TaskCount     int
	Tags          []string
	Description   string
}

// Info 会话简要信息（用于列表显示）
type Info struct {
	ID          ID
	Name        string
	WorkDir     string
	MessageCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Archived    bool
}

// New 创建新会话
func New(name, workDir string) *Session {
	now := time.Now()
	return &Session{
		ID:        generateID(),
		Name:      name,
		WorkDir:   workDir,
		Messages:  make([]llm.Message, 0),
		Metadata:  Metadata{},
		CreatedAt: now,
		UpdatedAt: now,
		Archived:  false,
	}
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(msg llm.Message) {
	s.Messages = append(s.Messages, msg)
	s.Metadata.MessageCount++
	if msg.Role == "tool" {
		s.Metadata.ToolCallCount++
	}
	s.UpdatedAt = time.Now()
}

// ClearMessages 清空消息（保留元数据）
func (s *Session) ClearMessages() {
	s.Messages = make([]llm.Message, 0)
	s.Metadata.MessageCount = 0
	s.Metadata.ToolCallCount = 0
	s.Metadata.TotalTokens = 0
	s.UpdatedAt = time.Now()
}

// UpdateMetadata 更新元数据
func (s *Session) UpdateMetadata(tokens int) {
	s.Metadata.TotalTokens = tokens
	s.UpdatedAt = time.Now()
}

// Archive 归档会话
func (s *Session) Archive() {
	s.Archived = true
	s.UpdatedAt = time.Now()
}

// Unarchive 取消归档
func (s *Session) Unarchive() {
	s.Archived = false
	s.UpdatedAt = time.Now()
}

// GetDuration 获取会话持续时间
func (s *Session) GetDuration() time.Duration {
	return time.Since(s.CreatedAt)
}

// ToInfo 转换为简要信息
func (s *Session) ToInfo() Info {
	return Info{
		ID:           s.ID,
		Name:         s.Name,
		WorkDir:      s.WorkDir,
		MessageCount: s.Metadata.MessageCount,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		Archived:     s.Archived,
	}
}

// Config 会话管理配置
type Config struct {
	StorePath      string
	AutoSave       bool
	MaxSessions    int
	MaxAge         time.Duration
	ArchiveOnClose bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		StorePath:      ".ms-cli/sessions",
		AutoSave:       true,
		MaxSessions:    50,
		MaxAge:         30 * 24 * time.Hour,
		ArchiveOnClose: false,
	}
}

// Filter 会话过滤器
type Filter struct {
	Archived   *bool
	WorkDir    string
	Tags       []string
	NamePrefix string
}

// generateID 生成唯一 ID
func generateID() ID {
	return ID(fmt.Sprintf("sess_%d", time.Now().UnixNano()))
}
