package permission

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

	// 是否静默（不提示用户，用于预检查）
	Silent bool `json:"silent,omitempty"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`
}

// PermissionResponse 权限响应
type PermissionResponse struct {
	// 请求ID
	RequestID string `json:"requestId"`

	// 决策动作
	Action Action `json:"action"`

	// 是否始终允许此模式
	AlwaysAllow bool `json:"alwaysAllow,omitempty"`

	// 响应时间戳
	Timestamp int64 `json:"timestamp"`

	// 响应元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewPermissionRequest 创建新的权限请求
func NewPermissionRequest(sessionID, permission string, patterns []string) *PermissionRequest {
	return &PermissionRequest{
		SessionID:  sessionID,
		Permission: permission,
		Patterns:   patterns,
		CheckLevel: CheckLevelExecution,
	}
}

// WithToolID 设置工具ID
func (r *PermissionRequest) WithToolID(toolID string) *PermissionRequest {
	r.ToolID = toolID
	return r
}

// WithCallID 设置调用ID
func (r *PermissionRequest) WithCallID(callID string) *PermissionRequest {
	r.CallID = callID
	return r
}

// WithCheckLevel 设置检查层级
func (r *PermissionRequest) WithCheckLevel(level CheckLevel) *PermissionRequest {
	r.CheckLevel = level
	return r
}

// WithSilent 设置静默模式
func (r *PermissionRequest) WithSilent(silent bool) *PermissionRequest {
	r.Silent = silent
	return r
}

// WithMetadata 设置元数据
func (r *PermissionRequest) WithMetadata(key string, value interface{}) *PermissionRequest {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}
