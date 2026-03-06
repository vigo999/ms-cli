package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/ms-cli/ui/components"
	"github.com/vigo999/ms-cli/ui/model"
	"github.com/vigo999/ms-cli/ui/panels"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	topBarHeight   = 3 // brand line + info line + divider
	chatLineHeight = 2
	hintBarHeight  = 2
	inputHeight    = 1
	verticalPad    = 2
)

var chatLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))

// App is the TUI root model.
type App struct {
	state         model.State
	viewport      components.Viewport
	input         components.TextInput
	thinking      components.ThinkingSpinner
	width         int
	height        int
	eventCh       <-chan model.Event
	userCh        chan<- string // sends user input to the engine bridge
	lastInterrupt time.Time     // track last ctrl+c for double-press exit
}

// New creates a new App driven by the given event channel.
// userCh may be nil (demo mode) — user input won't be forwarded.
func New(ch <-chan model.Event, userCh chan<- string, version, workDir, repoURL, modelName string, ctxMax int) App {
	return App{
		state:    model.NewState(version, workDir, repoURL, modelName, ctxMax),
		input:    components.NewTextInput(),
		thinking: components.NewThinkingSpinner(),
		eventCh:  ch,
		userCh:   userCh,
	}
}

func (a App) waitForEvent() tea.Msg {
	ev, ok := <-a.eventCh
	if !ok {
		return model.Event{Type: model.Done}
	}
	return ev
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.thinking.Tick(),
		a.waitForEvent,
	)
}

func (a App) chatHeight() int {
	h := a.height - topBarHeight - chatLineHeight - hintBarHeight - inputHeight - verticalPad
	// Adjust for input height (including suggestions)
	inputH := a.input.Height()
	if inputH > 1 {
		h -= (inputH - 1)
	}
	if h < 1 {
		return 1
	}
	return h
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		return a.handleKey(msg)

	case tea.MouseMsg:
		if !a.state.MouseEnabled {
			return a, nil
		}
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
		return a, nil

	case model.Event:
		return a.handleEvent(msg)

	default:
		var cmd tea.Cmd
		a.thinking, cmd = a.thinking.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// 重新渲染 viewport 以显示动画
		a.updateViewport()
	}

	return a, tea.Batch(cmds...)
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if there's a pending permission request
	if a.state.PendingPermission != nil {
		return a.handlePermissionSelection(msg)
	}

	// Check if we're in slash suggestion mode
	if a.input.IsSlashMode() {
		switch msg.String() {
		case "up", "down", "tab", "enter", "esc":
			// Let input handle these for suggestion navigation
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			// Recalculate chat height if suggestions changed
			a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
			return a, cmd
		}
	}

	switch msg.String() {
	case "ctrl+c":
		now := time.Now()
		// If last ctrl+c was within 1 second, quit
		if now.Sub(a.lastInterrupt) < time.Second {
			return a, tea.Quit
		}
		// Otherwise, cancel current input and show hint
		a.lastInterrupt = now
		a.input = a.input.Reset()
		a.state = a.state.WithMessage(model.Message{
			Kind:    model.MsgAgent,
			Content: "⚠️  Interrupted. Press Ctrl+C again within 1 second to exit.",
		})
		a.updateViewport()
		return a, nil

	case "enter":
		// Don't process enter if in slash mode (handled above)
		if a.input.IsSlashMode() {
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
			return a, cmd
		}

		val := a.input.Value()
		if val == "" {
			return a, nil
		}
		// Reset stats for new task
		a.state = a.state.ResetStats()
		a.state = a.state.WithThinking(false)
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgUser, Content: val})
		a.input = a.input.Reset()
		a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
		a.updateViewport()
		if a.userCh != nil {
			select {
			case a.userCh <- val:
			default:
				// drop if buffer full — avoids freezing the UI
			}
		}
		return a, nil

	case "pgup", "pgdown", "home", "end":
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case "up", "down":
		// Only scroll chat if not in input at top/bottom or if shift is held
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	default:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		// Recalculate chat height if suggestions appeared/disappeared
		a.viewport = a.viewport.SetSize(a.width-4, a.chatHeight())
		return a, cmd
	}
}

func (a App) handlePermissionSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pending := a.state.PendingPermission
	if pending == nil {
		return a, nil
	}

	// Get the permission request message
	msgIndex := pending.MessageIndex
	if msgIndex >= len(a.state.Messages) {
		return a, nil
	}

	// Make a copy of messages to modify
	msgs := make([]model.Message, len(a.state.Messages))
	copy(msgs, a.state.Messages)
	permMsg := &msgs[msgIndex]

	switch msg.String() {
	case "up":
		// Move selection up (with wrap)
		if permMsg.SelectedIndex > 0 {
			permMsg.SelectedIndex--
		} else {
			permMsg.SelectedIndex = len(permMsg.PermissionOptions) - 1
		}
		a.state.Messages = msgs
		a.updateViewport()
		return a, nil

	case "down":
		// Move selection down (with wrap)
		if permMsg.SelectedIndex < len(permMsg.PermissionOptions)-1 {
			permMsg.SelectedIndex++
		} else {
			permMsg.SelectedIndex = 0
		}
		a.state.Messages = msgs
		a.updateViewport()
		return a, nil

	case "enter":
		// Confirm selection
		selected := permMsg.PermissionOptions[permMsg.SelectedIndex]

		// Send response
		if pending.RespCh != nil {
			switch selected.Key {
			case "allow":
				pending.RespCh <- model.PermissionResponse{Granted: true, Remember: false}
			case "remember":
				pending.RespCh <- model.PermissionResponse{Granted: true, Remember: true}
			case "deny":
				pending.RespCh <- model.PermissionResponse{Granted: false, Remember: false}
			}
		}

		// Add confirmation message
		a.state = a.state.WithMessage(model.Message{
			Kind:    model.MsgAgent,
			Content: "Permission " + strings.ToLower(selected.Label),
		})

		// Clear pending permission
		a.state.PendingPermission = nil
		a.updateViewport()
		return a, nil

	default:
		// Ignore other keys
		return a, nil
	}
}

func (a App) handleEvent(ev model.Event) (tea.Model, tea.Cmd) {
	var eventCmd tea.Cmd

	switch ev.Type {
	case model.PermissionRequest:
		// Create permission request message
		msg := model.Message{
			Kind:     model.MsgPermissionRequest,
			Content:  ev.PermissionPath,
			ToolName: ev.PermissionTool,
			SelectedIndex: 0,
			PermissionOptions: []model.PermissionOption{
				{Key: "allow", Label: "Allow once"},
				{Key: "remember", Label: "Allow for session"},
				{Key: "deny", Label: "Deny"},
			},
		}
		msgIndex := len(a.state.Messages)
		a.state = a.state.WithMessage(msg)
		a.state.PendingPermission = &model.PendingPermission{
			MessageIndex: msgIndex,
			RespCh:       ev.PermissionRespCh,
		}
		a.updateViewport()
		return a, a.waitForEvent

	case model.AgentThinking:
		// Start thinking - set flag and ensure we have a thinking message
		a.state = a.state.WithThinking(true)
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgThinking})

	case model.AgentReply:
		// Stop thinking and show result
		a.state = a.state.WithThinking(false)
		a.state = a.replaceThinking(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.CmdStarted:
		// Update command count
		stats := a.state.Stats
		stats.Commands++
		a.state = a.state.WithStats(stats)
		// Shell tool message already contains the full output with $ prefix
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Shell",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.CmdOutput:
		a.state = a.appendToLastTool(ev.Message)

	case model.CmdFinished:
		// output already in the tool block

	case model.ToolRead:
		// Update files read count
		stats := a.state.Stats
		stats.FilesRead++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Read",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGrep:
		// Update search count
		stats := a.state.Stats
		stats.Searches++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Grep",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGlob:
		// Update search count
		stats := a.state.Stats
		stats.Searches++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Glob",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolEdit:
		// Update files edited count
		stats := a.state.Stats
		stats.FilesEdited++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Edit",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolWrite:
		// Update files edited count
		stats := a.state.Stats
		stats.FilesEdited++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Write",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolError:
		// Update error count
		stats := a.state.Stats
		stats.Errors++
		a.state = a.state.WithStats(stats)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: ev.ToolName,
			Display:  model.DisplayError,
			Content:  ev.Message,
		})

	case model.AnalysisReady:
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.TokenUpdate:
		mi := a.state.Model
		mi.CtxUsed = ev.CtxUsed
		mi.CtxMax = ev.CtxMax
		mi.TokensUsed = ev.TokensUsed
		a.state = a.state.WithModel(mi)

	case model.TaskUpdated:
		// no-op for now

	case model.ClearScreen:
		// Clear all messages and add the notification
		a.state.Messages = []model.Message{
			{Kind: model.MsgAgent, Content: ev.Message},
		}

	case model.ModelUpdate:
		// Update model name in top bar
		mi := a.state.Model
		mi.Name = ev.Message
		a.state = a.state.WithModel(mi)

	case model.MouseModeToggle:
		enabled := a.state.MouseEnabled
		switch strings.ToLower(strings.TrimSpace(ev.Message)) {
		case "", "toggle":
			enabled = !enabled
		case "on", "enable", "enabled", "true", "1":
			enabled = true
		case "off", "disable", "disabled", "false", "0":
			enabled = false
		}
		a.state = a.state.WithMouseEnabled(enabled)
		if enabled {
			eventCmd = tea.EnableMouseCellMotion
		} else {
			eventCmd = tea.DisableMouse
		}

	case model.Done:
		return a, tea.Quit
	}

	a.updateViewport()
	if eventCmd != nil {
		return a, tea.Batch(eventCmd, a.waitForEvent)
	}
	return a, a.waitForEvent
}

func (a App) replaceThinking(m model.Message) model.State {
	msgs := make([]model.Message, 0, len(a.state.Messages))
	for _, msg := range a.state.Messages {
		if msg.Kind != model.MsgThinking {
			msgs = append(msgs, msg)
		}
	}
	msgs = append(msgs, m)
	next := a.state
	next.Messages = msgs
	return next
}

func (a App) appendToLastTool(line string) model.State {
	msgs := make([]model.Message, len(a.state.Messages))
	copy(msgs, a.state.Messages)

	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind == model.MsgTool {
			msgs[i] = model.Message{
				Kind:     model.MsgTool,
				ToolName: msgs[i].ToolName,
				Display:  msgs[i].Display,
				Content:  msgs[i].Content + "\n" + line,
			}
			break
		}
	}

	next := a.state
	next.Messages = msgs
	return next
}

func (a *App) updateViewport() {
	content := panels.RenderMessages(a.state, a.thinking.View())
	a.viewport = a.viewport.SetContent(content)
}

func (a App) chatLine() string {
	return chatLineStyle.Render(strings.Repeat("─", a.width))
}

func (a App) View() string {
	topBar := panels.RenderTopBar(a.state, a.width)
	line := a.chatLine()
	chat := a.viewport.View()
	input := "  " + a.input.View()

	// Show different hint when there's a pending permission request
	hintBar := panels.RenderHintBar(a.width)
	if a.state.PendingPermission != nil {
		hintBar = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↑/↓ select • Enter confirm")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		line,
		chat,
		line,
		input,
		hintBar,
	)
}
