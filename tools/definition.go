package tools

import (
	"encoding/json"
)

// CostLevel 工具成本等级
type CostLevel int

const (
	CostLevelFree     CostLevel = iota // 免费操作（如读取）
	CostLevelLow                       // 低成本（如简单计算）
	CostLevelMedium                    // 中等成本（如网络请求）
	CostLevelHigh                      // 高成本（如长时间运行）
	CostLevelCritical                  // 关键操作（如删除数据）
)

func (c CostLevel) String() string {
	switch c {
	case CostLevelFree:
		return "free"
	case CostLevelLow:
		return "low"
	case CostLevelMedium:
		return "medium"
	case CostLevelHigh:
		return "high"
	case CostLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Permission 工具权限定义
type Permission struct {
	// 权限标识，如 "file:write", "bash:execute"
	Name string `json:"name"`

	// 权限描述
	Description string `json:"description"`

	// 是否需要动态确认（即使规则允许也需要确认）
	RequireConfirm bool `json:"requireConfirm,omitempty"`
}

// ToolMeta 工具元数据
type ToolMeta struct {
	// 工具分类，如 "filesystem", "shell", "network"
	Category string `json:"category"`

	// 成本等级
	Cost CostLevel `json:"cost"`

	// 所需权限列表
	Permissions []Permission `json:"permissions,omitempty"`

	// 是否只读（不修改系统状态）
	ReadOnly bool `json:"readOnly,omitempty"`

	// 是否幂等（多次执行结果相同）
	Idempotent bool `json:"idempotent,omitempty"`

	// 超时时间（秒），0表示使用默认值
	Timeout int `json:"timeout,omitempty"`
}

// ToolDefinition 工具定义
// 这是工具的静态描述，不包含执行逻辑
type ToolDefinition struct {
	// 工具唯一标识
	Name string `json:"name"`

	// 工具显示名称
	DisplayName string `json:"displayName,omitempty"`

	// 工具描述（用于LLM理解）
	Description string `json:"description"`

	// 参数JSON Schema
	Parameters map[string]interface{} `json:"parameters"`

	// 工具元数据
	Meta ToolMeta `json:"meta"`

	// 版本号
	Version string `json:"version,omitempty"`

	// 示例调用
	Examples []ToolExample `json:"examples,omitempty"`
}

// ToolExample 工具使用示例
type ToolExample struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output,omitempty"`
}

// AgentInfo Agent元数据
type AgentInfo struct {
	// Agent类型/角色
	Type string `json:"type"`

	// Agent标识
	ID string `json:"id"`

	// Agent版本
	Version string `json:"version,omitempty"`

	// 额外信息
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	// 会话ID
	ID string `json:"id"`

	// 会话创建时间
	CreatedAt int64 `json:"createdAt,omitempty"`

	// 会话元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Message 消息结构（用于MessageStore）
type Message struct {
	// 消息ID
	ID string `json:"id"`

	// 角色：user, assistant, system, tool
	Role string `json:"role"`

	// 消息内容
	Content string `json:"content"`

	// 消息元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 时间戳
	Timestamp int64 `json:"timestamp,omitempty"`

	// 如果是工具消息，关联的工具调用ID
	ToolCallID string `json:"toolCallId,omitempty"`
}
