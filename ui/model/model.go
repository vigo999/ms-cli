package model

// TaskInfo represents a task in the task pool.
type TaskInfo struct {
	ID   string
	Name string
}

// ModelInfo holds LLM model metadata for the top bar.
type ModelInfo struct {
	Name       string
	CtxUsed    int
	CtxMax     int
	TokensUsed int
}

// MessageKind distinguishes chat message types.
type MessageKind int

const (
	MsgUser     MessageKind = iota
	MsgAgent
	MsgThinking
	MsgTool
)

// DisplayMode controls how a tool message is rendered.
type DisplayMode int

const (
	DisplayExpanded  DisplayMode = iota // full output shown (Shell user-cmd, Edit, Write)
	DisplayCollapsed                    // 1-line summary (Read, Grep, Glob, agent-internal Shell)
	DisplayError                        // expanded + red highlight
)

// Message is a single entry in the chat stream.
type Message struct {
	Kind     MessageKind
	Content  string
	ToolName string
	Display  DisplayMode
	Summary  string // shown when collapsed, e.g. "5 matches", "23 files"
}

// EventType identifies the kind of UI event.
type EventType string

const (
	TaskUpdated   EventType = "TaskUpdated"
	CmdStarted    EventType = "CmdStarted"
	CmdOutput     EventType = "CmdOutput"
	CmdFinished   EventType = "CmdFinished"
	AnalysisReady EventType = "AnalysisReady"
	AgentReply    EventType = "AgentReply"
	AgentThinking EventType = "AgentThinking"
	TokenUpdate   EventType = "TokenUpdate"
	ToolRead      EventType = "ToolRead"
	ToolGrep      EventType = "ToolGrep"
	ToolGlob      EventType = "ToolGlob"
	ToolEdit      EventType = "ToolEdit"
	ToolWrite     EventType = "ToolWrite"
	ToolError     EventType = "ToolError"
	Done          EventType = "Done"
)

// Event is sent from the agent loop to the TUI.
// Implements tea.Msg so Bubble Tea can route it.
type Event struct {
	Type       EventType
	Task       string
	Message    string
	ToolName   string
	Summary    string
	CtxUsed    int
	TokensUsed int
}

// State is the central UI state.
type State struct {
	Tasks            []TaskInfo
	ActiveTask       int
	Model            ModelInfo
	Messages         []Message
	ShowTaskSelector bool
	WorkDir          string
	RepoURL          string
}

// NewState returns an initial empty state.
func NewState(workDir, repoURL string) State {
	return State{
		Tasks: []TaskInfo{},
		Model: ModelInfo{
			Name:   "deepseek-r1",
			CtxMax: 128000,
		},
		WorkDir: workDir,
		RepoURL: repoURL,
	}
}

// WithTask returns a new State with the given task added.
func (s State) WithTask(t TaskInfo) State {
	return State{
		Tasks:            append(append([]TaskInfo{}, s.Tasks...), t),
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
	}
}

// WithMessage returns a new State with the given message appended.
func (s State) WithMessage(m Message) State {
	return State{
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         append(append([]Message{}, s.Messages...), m),
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
	}
}

// WithModel returns a new State with updated model info.
func (s State) WithModel(m ModelInfo) State {
	return State{
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            m,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
	}
}
