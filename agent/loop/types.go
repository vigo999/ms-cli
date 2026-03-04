package loop

import "time"

type ModelSpec struct {
	Provider string
	Name     string
	Endpoint string
}

type Task struct {
	ID               string
	SessionID        string
	Mode             string
	Description      string
	MaxSteps         int
	ContextMaxTokens int
	Model            ModelSpec
}

type EventType string

const (
	EventThinking         EventType = "thinking"
	EventReply            EventType = "reply"
	EventApprovalRequired EventType = "approval_required"
	EventApprovalResolved EventType = "approval_resolved"
	EventToolGlob         EventType = "tool_glob"
	EventToolRead         EventType = "tool_read"
	EventToolGrep         EventType = "tool_grep"
	EventToolEdit         EventType = "tool_edit"
	EventToolWrite        EventType = "tool_write"
	EventCmdStarted       EventType = "cmd_started"
	EventCmdOutput        EventType = "cmd_output"
	EventCmdFinish        EventType = "cmd_finished"
	EventToolError        EventType = "tool_error"
	EventTokenUsage       EventType = "token_usage"
)

type Event struct {
	Type           EventType
	Message        string
	ToolName       string
	Summary        string
	ApprovalID     int64
	ApprovalTool   string
	ApprovalAction string
	CtxUsed        int
	TokensUsed     int
	Time           time.Time
}
