package ui

// EventType is the UI event type.
type EventType string

const (
	TaskUpdated   EventType = "TaskUpdated"
	CmdStarted    EventType = "CmdStarted"
	CmdOutput     EventType = "CmdOutput"
	CmdFinished   EventType = "CmdFinished"
	AnalysisReady EventType = "AnalysisReady"
	Done          EventType = "Done"
)
