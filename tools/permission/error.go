package permission

import (
	"fmt"
)

// PermissionError 权限错误
type PermissionError struct {
	// 错误码
	Code ErrorCode `json:"code"`

	// 错误消息
	Message string `json:"message"`

	// 请求的权限
	Permission string `json:"permission,omitempty"`

	// 请求的模式
	Patterns []string `json:"patterns,omitempty"`

	// 原始错误
	Cause error `json:"-"`
}

// ErrorCode 权限错误码
type ErrorCode string

const (
	// ErrCodePermissionDenied 权限被拒绝
	ErrCodePermissionDenied ErrorCode = "permission_denied"

	// ErrCodePermissionPending 等待权限确认
	ErrCodePermissionPending ErrorCode = "permission_pending"

	// ErrCodePermissionTimeout 权限确认超时
	ErrCodePermissionTimeout ErrorCode = "permission_timeout"

	// ErrCodePermissionCancelled 用户取消
	ErrCodePermissionCancelled ErrorCode = "permission_cancelled"

	// ErrCodeInvalidRequest 无效请求
	ErrCodeInvalidRequest ErrorCode = "invalid_request"

	// ErrCodeEngineNotReady 引擎未就绪
	ErrCodeEngineNotReady ErrorCode = "engine_not_ready"
)

// Error 实现error接口
func (e *PermissionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("permission error [%s]: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("permission error [%s]: %s", e.Code, e.Message)
}

// Unwrap 返回原始错误
func (e *PermissionError) Unwrap() error {
	return e.Cause
}

// IsDenied 是否被拒绝
func (e *PermissionError) IsDenied() bool {
	return e.Code == ErrCodePermissionDenied
}

// IsPending 是否等待中
func (e *PermissionError) IsPending() bool {
	return e.Code == ErrCodePermissionPending
}

// NewDeniedError 创建拒绝错误
func NewDeniedError(permission string, patterns []string) *PermissionError {
	return &PermissionError{
		Code:       ErrCodePermissionDenied,
		Message:    fmt.Sprintf("permission denied: %s on %v", permission, patterns),
		Permission: permission,
		Patterns:   patterns,
	}
}

// NewPendingError 创建等待错误
func NewPendingError(permission string, patterns []string) *PermissionError {
	return &PermissionError{
		Code:       ErrCodePermissionPending,
		Message:    fmt.Sprintf("waiting for permission: %s on %v", permission, patterns),
		Permission: permission,
		Patterns:   patterns,
	}
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(permission string) *PermissionError {
	return &PermissionError{
		Code:       ErrCodePermissionTimeout,
		Message:    fmt.Sprintf("permission request timeout: %s", permission),
		Permission: permission,
	}
}

// NewCancelledError 创建取消错误
func NewCancelledError(permission string) *PermissionError {
	return &PermissionError{
		Code:       ErrCodePermissionCancelled,
		Message:    fmt.Sprintf("permission request cancelled: %s", permission),
		Permission: permission,
	}
}
