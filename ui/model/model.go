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
	TaskUpdated    EventType = "TaskUpdated"
	CmdStarted     EventType = "CmdStarted"
	CmdOutput      EventType = "CmdOutput"
	CmdFinished    EventType = "CmdFinished"
	AnalysisReady  EventType = "AnalysisReady"
	AgentReply     EventType = "AgentReply"
	AgentThinking  EventType = "AgentThinking"
	TokenUpdate    EventType = "TokenUpdate"
	ToolRead       EventType = "ToolRead"
	ToolGrep       EventType = "ToolGrep"
	ToolGlob       EventType = "ToolGlob"
	ToolEdit       EventType = "ToolEdit"
	ToolWrite      EventType = "ToolWrite"
	ToolError      EventType = "ToolError"
	ClearScreen    EventType = "ClearScreen"
	ModelUpdate    EventType = "ModelUpdate"
	MouseModeToggle EventType = "MouseModeToggle"
	Done           EventType = "Done"
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
	CtxMax     int
	TokensUsed int
}

// TaskStats tracks execution statistics for the current task.
type TaskStats struct {
	Commands    int // shell commands executed
	FilesRead   int // files read
	FilesEdited int // files edited/written
	Searches    int // grep/glob operations
	Errors      int // errors encountered
}

// State is the central UI state.
type State struct {
	Version          string
	Tasks            []TaskInfo
	ActiveTask       int
	Model            ModelInfo
	Messages         []Message
	ShowTaskSelector bool
	WorkDir          string
	RepoURL          string
	Stats            TaskStats // current task statistics
	IsThinking       bool      // whether AI is currently thinking
	MouseEnabled     bool      // whether mouse mode is enabled (for scrolling)
}

// NewState returns an initial empty state.
func NewState(version, workDir, repoURL, modelName string, ctxMax int) State {
	if modelName == "" {
		modelName = "unknown"
	}
	if ctxMax == 0 {
		ctxMax = 128000 // Default for models like gpt-4o
	}
	return State{
		Version:      version,
		Tasks:        []TaskInfo{},
		Model: ModelInfo{
			Name:   modelName,
			CtxMax: ctxMax,
		},
		WorkDir:      workDir,
		RepoURL:      repoURL,
		Stats:        TaskStats{},
		IsThinking:   false,
		MouseEnabled: true, // default to enabled for scroll wheel
	}
}

// WithTask returns a new State with the given task added.
func (s State) WithTask(t TaskInfo) State {
	return State{
		Version:          s.Version,
		Tasks:            append(append([]TaskInfo{}, s.Tasks...), t),
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            s.Stats,
		IsThinking:       s.IsThinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// WithMessage returns a new State with the given message appended.
func (s State) WithMessage(m Message) State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         append(append([]Message{}, s.Messages...), m),
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            s.Stats,
		IsThinking:       s.IsThinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// WithModel returns a new State with updated model info.
func (s State) WithModel(m ModelInfo) State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            m,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            s.Stats,
		IsThinking:       s.IsThinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// WithStats returns a new State with updated stats.
func (s State) WithStats(stats TaskStats) State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            stats,
		IsThinking:       s.IsThinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// WithThinking returns a new State with updated thinking status.
func (s State) WithThinking(thinking bool) State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            s.Stats,
		IsThinking:       thinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// ResetStats returns a new State with reset stats.
func (s State) ResetStats() State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            TaskStats{},
		IsThinking:       s.IsThinking,
		MouseEnabled:     s.MouseEnabled,
	}
}

// WithMouseEnabled returns a new State with updated mouse mode.
func (s State) WithMouseEnabled(enabled bool) State {
	return State{
		Version:          s.Version,
		Tasks:            s.Tasks,
		ActiveTask:       s.ActiveTask,
		Model:            s.Model,
		Messages:         s.Messages,
		ShowTaskSelector: s.ShowTaskSelector,
		WorkDir:          s.WorkDir,
		RepoURL:          s.RepoURL,
		Stats:            s.Stats,
		IsThinking:       s.IsThinking,
		MouseEnabled:     enabled,
	}
}
