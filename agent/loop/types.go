package loop

import (
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Task represents a user task.
type Task struct {
	ID          string
	Description string
	Context     map[string]string
}

// Event represents an engine event.
type Event struct {
	Type       string
	Task       string
	Message    string
	ToolName   string
	Summary    string
	CtxUsed    int
	CtxMax     int
	TokensUsed int
	Usage      llm.Usage
	Timestamp  time.Time
}

// NewEvent creates a new event.
func NewEvent(eventType, message string) Event {
	return Event{
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// Event types.
const (
	// Task lifecycle
	EventTaskStarted   = "TaskStarted"
	EventTaskCompleted = "TaskCompleted"
	EventTaskFailed    = "TaskFailed"

	// LLM events
	EventLLMThinking   = "LLMThinking"
	EventLLMResponse   = "LLMResponse"
	EventToolCallStart = "ToolCallStart"

	// Tool events
	EventToolStarted   = "ToolStarted"
	EventToolCompleted = "ToolCompleted"
	EventToolError     = "ToolError"

	// UI compatible events
	EventCmdStarted    = "CmdStarted"
	EventCmdOutput     = "CmdOutput"
	EventCmdFinished   = "CmdFinished"
	EventAgentReply    = "AgentReply"
	EventAgentThinking = "AgentThinking"
	EventTokenUpdate   = "TokenUpdate"
	EventToolRead      = "ToolRead"
	EventToolGrep      = "ToolGrep"
	EventToolGlob      = "ToolGlob"
	EventToolEdit      = "ToolEdit"
	EventToolWrite     = "ToolWrite"
	EventAnalysisReady = "AnalysisReady"
	EventDone          = "Done"
)
