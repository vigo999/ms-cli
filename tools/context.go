package tools

import (
	"context"
)

// ToolContext 纯数据上下文 - agent构造后传入
// 禁止包含任何方法或回调，确保可序列化和可测试
type ToolContext struct {
	// 会话唯一ID
	SessionID string `json:"sessionId"`

	// 当前消息ID
	MessageID string `json:"messageId"`

	// LLM生成的调用ID
	CallID string `json:"callId"`

	// 工具标识
	ToolID string `json:"toolId"`

	// 取消信号（不序列化）
	AbortSignal context.Context `json:"-"`

	// Agent元数据
	Agent AgentInfo `json:"agent"`

	// 历史消息获取接口（不序列化）
	MessageStore MessageStore `json:"-"`

	// 扩展字段
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// MessageStore 消息存储接口 - agent的context manager实现
type MessageStore interface {
	// GetMessages 获取最近N条消息
	GetMessages(limit int) []Message

	// GetMessageCount 获取消息总数
	GetMessageCount() int
}

// Done 检查是否已取消
func (tc *ToolContext) Done() <-chan struct{} {
	if tc.AbortSignal != nil {
		return tc.AbortSignal.Done()
	}
	return nil
}

// Err 返回取消错误
func (tc *ToolContext) Err() error {
	if tc.AbortSignal != nil {
		return tc.AbortSignal.Err()
	}
	return nil
}

// ContextOrBackground 返回有效的context
func (tc *ToolContext) ContextOrBackground() context.Context {
	if tc.AbortSignal != nil {
		return tc.AbortSignal
	}
	return context.Background()
}
