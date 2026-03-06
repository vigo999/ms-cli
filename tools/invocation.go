package tools

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// InvocationStatus 调用状态
type InvocationStatus string

const (
	// InvocationStatusPending 等待中
	InvocationStatusPending InvocationStatus = "pending"

	// InvocationStatusRunning 执行中
	InvocationStatusRunning InvocationStatus = "running"

	// InvocationStatusSuccess 成功
	InvocationStatusSuccess InvocationStatus = "success"

	// InvocationStatusFailed 失败
	InvocationStatusFailed InvocationStatus = "failed"

	// InvocationStatusCancelled 已取消
	InvocationStatusCancelled InvocationStatus = "cancelled"

	// InvocationStatusWaitingForPermission 等待权限确认
	InvocationStatusWaitingForPermission InvocationStatus = "waiting_for_permission"
)

// ToolInvocation 工具调用记录
type ToolInvocation struct {
	// 调用ID（唯一）
	ID string `json:"id"`

	// 会话ID
	SessionID string `json:"sessionId"`

	// 消息ID
	MessageID string `json:"messageId"`

	// 工具ID
	ToolID string `json:"toolId"`

	// 工具名称
	ToolName string `json:"toolName"`

	// 调用参数
	Arguments json.RawMessage `json:"arguments,omitempty"`

	// 调用状态
	Status InvocationStatus `json:"status"`

	// 调用结果
	Result *ToolResult `json:"result,omitempty"`

	// 开始时间
	StartedAt time.Time `json:"startedAt,omitempty"`

	// 结束时间
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// 更新时间
	UpdatedAt time.Time `json:"updatedAt"`

	// 元数据
	Metadata InvocationMetadata `json:"metadata,omitempty"`
}

// InvocationMetadata 调用元数据
type InvocationMetadata struct {
	// Agent信息
	Agent AgentInfo `json:"agent,omitempty"`

	// 执行耗时（毫秒）
	Duration int64 `json:"duration,omitempty"`

	// Token消耗
	TokensUsed int `json:"tokensUsed,omitempty"`

	// 重试次数
	RetryCount int `json:"retryCount,omitempty"`

	// 权限ID（如果需要权限确认）
	PermissionID string `json:"permissionId,omitempty"`

	// 额外元数据
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// InvocationContext 调用上下文（用于执行过程中）
type InvocationContext struct {
	// 调用记录
	Invocation *ToolInvocation

	// 工具上下文
	ToolContext *ToolContext

	// 执行器
	Executor ToolExecutor

	// 工具定义
	Definition *ToolDefinition

	// 开始时间
	startTime time.Time
}

// NewInvocation 创建新的调用记录
func NewInvocation(sessionID, messageID, toolID, toolName string, args json.RawMessage) *ToolInvocation {
	now := time.Now()
	return &ToolInvocation{
		ID:        generateID(),
		SessionID: sessionID,
		MessageID: messageID,
		ToolID:    toolID,
		ToolName:  toolName,
		Arguments: args,
		Status:    InvocationStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Start 标记开始执行
func (i *ToolInvocation) Start() {
	i.Status = InvocationStatusRunning
	i.StartedAt = time.Now()
	i.UpdatedAt = i.StartedAt
}

// Complete 标记完成
func (i *ToolInvocation) Complete(result *ToolResult) {
	i.Result = result
	if result.Success {
		i.Status = InvocationStatusSuccess
	} else {
		i.Status = InvocationStatusFailed
	}
	now := time.Now()
	i.FinishedAt = &now
	i.UpdatedAt = now
	if !i.StartedAt.IsZero() {
		i.Metadata.Duration = now.Sub(i.StartedAt).Milliseconds()
	}
}

// Cancel 标记取消
func (i *ToolInvocation) Cancel() {
	i.Status = InvocationStatusCancelled
	now := time.Now()
	i.FinishedAt = &now
	i.UpdatedAt = now
	if !i.StartedAt.IsZero() {
		i.Metadata.Duration = now.Sub(i.StartedAt).Milliseconds()
	}
}

// WaitForPermission 标记等待权限
func (i *ToolInvocation) WaitForPermission(permissionID string) {
	i.Status = InvocationStatusWaitingForPermission
	i.Metadata.PermissionID = permissionID
	i.UpdatedAt = time.Now()
}

// IsFinished 是否已完成
func (i *ToolInvocation) IsFinished() bool {
	return i.Status == InvocationStatusSuccess ||
		i.Status == InvocationStatusFailed ||
		i.Status == InvocationStatusCancelled
}

// IsRunning 是否正在执行
func (i *ToolInvocation) IsRunning() bool {
	return i.Status == InvocationStatusRunning
}

// generateID 生成唯一ID（简单实现）
func generateID() string {
	// 使用当前时间纳秒和随机数生成ID
	return fmt.Sprintf("inv_%d_%d", time.Now().UnixNano(), rand.Int63())
}
