package resolver

import (
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/permission"
)

// ProviderType 协议类型
type ProviderType string

const (
	// ProviderTypeMCP MCP协议
	ProviderTypeMCP ProviderType = "mcp"

	// ProviderTypeOpenAI OpenAI协议
	ProviderTypeOpenAI ProviderType = "openai"

	// ProviderTypeA2A A2A协议
	ProviderTypeA2A ProviderType = "a2a"

	// ProviderTypeInternal 内部协议
	ProviderTypeInternal ProviderType = "internal"
)

// ResolveInput 解析输入
type ResolveInput struct {
	// 内置工具定义
	Definitions []tools.ToolDefinition

	// Agent信息
	Agent tools.AgentInfo

	// 会话信息
	Session tools.SessionInfo

	// 目标协议类型（影响Schema转换）
	Provider ProviderType

	// 当前会话历史
	Messages []tools.Message

	// 权限引擎
	PermissionEngine permission.Engine
}

// ResolveOutput 解析输出
type ResolveOutput struct {
	// 可执行工具映射（key为工具名）
	Tools map[string]tools.ExecutableTool

	// 转换后的协议Schema
	ProviderSchemas []ProviderToolSchema
}

// ProviderToolSchema 协议工具Schema
type ProviderToolSchema struct {
	// 工具名称
	Name string `json:"name"`

	// 工具描述
	Description string `json:"description"`

	// 参数Schema（协议特定格式）
	Parameters interface{} `json:"parameters"`

	// 原始工具定义
	Definition *tools.ToolDefinition `json:"-"`

	// 协议类型
	Provider ProviderType `json:"-"`
}

// ResolutionError 解析错误
type ResolutionError struct {
	// 错误类型
	Type ErrorType

	// 错误消息
	Message string

	// 相关工具
	ToolName string
}

// ErrorType 错误类型
type ErrorType string

const (
	// ErrorTypeNotFound 工具未找到
	ErrorTypeNotFound ErrorType = "not_found"

	// ErrorTypePermissionDenied 权限不足
	ErrorTypePermissionDenied ErrorType = "permission_denied"

	// ErrorTypeInvalidSchema Schema无效
	ErrorTypeInvalidSchema ErrorType = "invalid_schema"

	// ErrorTypeConversionFailed 转换失败
	ErrorTypeConversionFailed ErrorType = "conversion_failed"
)

// Error 实现error接口
func (e *ResolutionError) Error() string {
	if e.ToolName != "" {
		return string(e.Type) + ": " + e.Message + " (tool: " + e.ToolName + ")"
	}
	return string(e.Type) + ": " + e.Message
}

// IsNotFound 是否未找到
func (e *ResolutionError) IsNotFound() bool {
	return e.Type == ErrorTypeNotFound
}

// IsPermissionDenied 是否权限不足
func (e *ResolutionError) IsPermissionDenied() bool {
	return e.Type == ErrorTypePermissionDenied
}
