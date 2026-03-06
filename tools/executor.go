package tools

import (
	"encoding/json"
)

// ToolExecutor 工具执行接口 - agent需实现该接口
type ToolExecutor interface {
	// UpdateMetadata 更新调用元数据（如token消耗、执行耗时）
	UpdateMetadata(meta MetadataUpdate) error

	// AskPermission 执行层权限检查入口
	// 返回nil表示允许，返回error表示拒绝或需用户确认
	AskPermission(req PermissionRequest) error
}

// ExecutableTool 可执行工具接口 - 所有工具必须实现
type ExecutableTool interface {
	// Info 返回工具定义信息
	Info() *ToolDefinition

	// Execute 执行工具
	// ctx: 工具上下文
	// exec: 执行器接口（用于权限检查和元数据更新）
	// args: JSON格式的参数
	Execute(ctx *ToolContext, exec ToolExecutor, args json.RawMessage) (*ToolResult, error)
}

// MetadataUpdate 元数据更新
type MetadataUpdate struct {
	// Token消耗
	TokensUsed int `json:"tokensUsed,omitempty"`

	// 执行耗时（毫秒）
	Duration int64 `json:"duration,omitempty"`

	// 额外元数据
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// PermissionRequest 权限请求
type PermissionRequest struct {
	// 请求ID
	ID string `json:"id,omitempty"`

	// 会话ID
	SessionID string `json:"sessionId"`

	// 工具ID
	ToolID string `json:"toolId,omitempty"`

	// 调用ID
	CallID string `json:"callId,omitempty"`

	// 权限标识（如file:write）
	Permission string `json:"permission"`

	// 检查目标（如文件路径）
	Patterns []string `json:"patterns"`

	// 用户选择"始终允许"的模式
	Always []string `json:"always,omitempty"`

	// 额外元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 检查层级标记
	CheckLevel CheckLevel `json:"checkLevel"`
}

// CheckLevel 检查层级 - 用于避免重复检查
type CheckLevel int

const (
	// CheckLevelResolver Resolver层静态检查
	CheckLevelResolver CheckLevel = iota

	// CheckLevelExecution Execution层动态检查
	CheckLevelExecution
)

func (c CheckLevel) String() string {
	switch c {
	case CheckLevelResolver:
		return "resolver"
	case CheckLevelExecution:
		return "execution"
	default:
		return "unknown"
	}
}
