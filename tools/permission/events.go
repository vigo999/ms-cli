package permission

import (
	"time"

	"github.com/vigo999/ms-cli/tools/events"
)

// PermissionRequestedEvent 权限请求事件
type PermissionRequestedEvent struct {
	RequestID  string                 `json:"requestId"`
	SessionID  string                 `json:"sessionId"`
	ToolID     string                 `json:"toolId"`
	CallID     string                 `json:"callId"`
	Permission string                 `json:"permission"`
	Patterns   []string               `json:"patterns"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	ResponseCh chan PermissionResponse `json:"-"` // 内部使用，不序列化
}

// Topic 返回事件主题
func (e *PermissionRequestedEvent) Topic() string {
	return events.PermissionRequestedTopic
}

// Data 返回事件数据
func (e *PermissionRequestedEvent) Data() interface{} {
	return e
}

// PermissionRespondedEvent 权限响应事件
type PermissionRespondedEvent struct {
	RequestID   string                 `json:"requestId"`
	Action      Action                 `json:"action"`
	AlwaysAllow bool                   `json:"alwaysAllow,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Topic 返回事件主题
func (e *PermissionRespondedEvent) Topic() string {
	return events.PermissionRespondedTopic
}

// Data 返回事件数据
func (e *PermissionRespondedEvent) Data() interface{} {
	return e
}

// ToEvent 转换为通用事件
func (e *PermissionRequestedEvent) ToEvent() events.Event {
	return events.NewEvent(e.Topic(), e)
}

// ToEvent 转换为通用事件
func (e *PermissionRespondedEvent) ToEvent() events.Event {
	return events.NewEvent(e.Topic(), e)
}
