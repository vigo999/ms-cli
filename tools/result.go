package tools

import (
	"encoding/json"
	"fmt"
)

// ToolErrorCode 工具错误码
type ToolErrorCode string

const (
	// ErrCodeUnknown 未知错误
	ErrCodeUnknown ToolErrorCode = "unknown"

	// ErrCodeInvalidInput 输入参数无效
	ErrCodeInvalidInput ToolErrorCode = "invalid_input"

	// ErrCodePermissionDenied 权限被拒绝
	ErrCodePermissionDenied ToolErrorCode = "permission_denied"

	// ErrCodeTimeout 执行超时
	ErrCodeTimeout ToolErrorCode = "timeout"

	// ErrCodeCancelled 用户取消
	ErrCodeCancelled ToolErrorCode = "cancelled"

	// ErrCodeExecutionFailed 执行失败
	ErrCodeExecutionFailed ToolErrorCode = "execution_failed"

	// ErrCodeNotFound 资源未找到
	ErrCodeNotFound ToolErrorCode = "not_found"

	// ErrCodeAlreadyExists 资源已存在
	ErrCodeAlreadyExists ToolErrorCode = "already_exists"

	// ErrCodeNetworkError 网络错误
	ErrCodeNetworkError ToolErrorCode = "network_error"

	// ErrCodeRateLimited 被限流
	ErrCodeRateLimited ToolErrorCode = "rate_limited"
)

// ToolError 工具错误
type ToolError struct {
	// 错误码
	Code ToolErrorCode `json:"code"`

	// 错误消息
	Message string `json:"message"`

	// 详细错误信息
	Details string `json:"details,omitempty"`

	// 是否可以重试
	Retryable bool `json:"retryable,omitempty"`

	// 重试延迟（毫秒）
	RetryAfter int `json:"retryAfter,omitempty"`

	// 额外数据
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// Error 实现error接口
func (e *ToolError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Is 实现错误比较
func (e *ToolError) Is(target error) bool {
	if te, ok := target.(*ToolError); ok {
		return e.Code == te.Code
	}
	return false
}

// PartType 结果部分类型
type PartType string

const (
	// PartTypeText 文本内容
	PartTypeText PartType = "text"

	// PartTypeJSON JSON数据
	PartTypeJSON PartType = "json"

	// PartTypeBinary 二进制数据
	PartTypeBinary PartType = "binary"

	// PartTypeError 错误信息
	PartTypeError PartType = "error"

	// PartTypeArtifact 产物（如生成的文件）
	PartTypeArtifact PartType = "artifact"
)

// Part 结果部分
type Part struct {
	// 类型
	Type PartType `json:"type"`

	// 内容（文本或base64编码的二进制）
	Content string `json:"content,omitempty"`

	// JSON数据（当Type为json时）
	Data interface{} `json:"data,omitempty"`

	// MIME类型
	MimeType string `json:"mimeType,omitempty"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	// 是否成功
	Success bool `json:"success"`

	// 结果部分（可包含多个部分）
	Parts []Part `json:"parts,omitempty"`

	// 错误信息（当Success为false时）
	Error *ToolError `json:"error,omitempty"`

	// 元数据
	Metadata ResultMetadata `json:"metadata,omitempty"`
}

// ResultMetadata 结果元数据
type ResultMetadata struct {
	// 执行耗时（毫秒）
	Duration int64 `json:"duration,omitempty"`

	// Token消耗
	TokensUsed int `json:"tokensUsed,omitempty"`

	// 额外元数据
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// NewTextResult 创建文本结果
func NewTextResult(text string) *ToolResult {
	return &ToolResult{
		Success: true,
		Parts: []Part{
			{
				Type:    PartTypeText,
				Content: text,
			},
		},
	}
}

// NewJSONResult 创建JSON结果
func NewJSONResult(data interface{}) *ToolResult {
	return &ToolResult{
		Success: true,
		Parts: []Part{
			{
				Type: PartTypeJSON,
				Data: data,
			},
		},
	}
}

// NewErrorResult 创建错误结果
func NewErrorResult(code ToolErrorCode, message string, args ...interface{}) *ToolResult {
	return &ToolResult{
		Success: false,
		Error: &ToolError{
			Code:    code,
			Message: fmt.Sprintf(message, args...),
		},
	}
}

// NewErrorResultWithDetails 创建带详细信息的错误结果
func NewErrorResultWithDetails(code ToolErrorCode, message, details string) *ToolResult {
	return &ToolResult{
		Success: false,
		Error: &ToolError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// AddPart 添加结果部分
func (r *ToolResult) AddPart(part Part) *ToolResult {
	r.Parts = append(r.Parts, part)
	return r
}

// Text 获取第一个文本部分的内容
func (r *ToolResult) Text() string {
	for _, p := range r.Parts {
		if p.Type == PartTypeText {
			return p.Content
		}
	}
	return ""
}

// JSON 获取第一个JSON部分的数据
func (r *ToolResult) JSON() interface{} {
	for _, p := range r.Parts {
		if p.Type == PartTypeJSON {
			return p.Data
		}
	}
	return nil
}

// MarshalJSON 自定义JSON序列化
func (r *ToolResult) MarshalJSON() ([]byte, error) {
	type Alias ToolResult
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}
