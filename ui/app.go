package ui

import (
	"github.com/charmbracelet/lipgloss"
	"mscli/ui/components"
	"mscli/ui/model"
	"mscli/ui/panels"

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
	state    model.State
	viewport components.Viewport
	input    components.TextInput
	spinner  components.Spinner
	width    int
	height   int
	eventCh  <-chan model.Event
}

// New creates a new App driven by the given event channel.
func New(ch <-chan model.Event, workDir, repoURL string) App {
	return App{
		state:   model.NewState(workDir, repoURL),
		input:   components.NewTextInput(),
		spinner: components.NewSpinner(),
		eventCh: ch,
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
		a.spinner.Model.Tick,
		a.waitForEvent,
	)
}

func (a App) chatHeight() int {
	h := a.height - topBarHeight - chatLineHeight - hintBarHeight - inputHeight - verticalPad
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
		a.spinner, cmd = a.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return a, tea.Quit

	case "enter":
		val := a.input.Value()
		if val == "" {
			return a, nil
		}
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgUser, Content: val})
		a.input = a.input.Reset()
		a.updateViewport()
		return a, nil

	default:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		return a, cmd
	}
}

func (a App) handleEvent(ev model.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case model.AgentThinking:
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgThinking})

	case model.AgentReply:
		a.state = a.replaceThinking(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.CmdStarted:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Shell",
			Display:  model.DisplayExpanded,
			Content:  "$ " + ev.Message,
		})

	case model.CmdOutput:
		a.state = a.appendToLastTool(ev.Message)

	case model.CmdFinished:
		// output already in the tool block

	case model.ToolRead:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Read",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGrep:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Grep",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolGlob:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Glob",
			Display:  model.DisplayCollapsed,
			Content:  ev.Message,
			Summary:  ev.Summary,
		})

	case model.ToolEdit:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Edit",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolWrite:
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Write",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolError:
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
		mi.TokensUsed = ev.TokensUsed
		a.state = a.state.WithModel(mi)

	case model.TaskUpdated:
		// no-op for now

	case model.Done:
		return a, tea.Quit
	}

	a.updateViewport()
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
	return model.State{
		Model:    a.state.Model,
		Messages: msgs,
		WorkDir:  a.state.WorkDir,
		RepoURL:  a.state.RepoURL,
	}
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

	return model.State{
		Model:    a.state.Model,
		Messages: msgs,
		WorkDir:  a.state.WorkDir,
		RepoURL:  a.state.RepoURL,
	}
}

func (a *App) updateViewport() {
	content := panels.RenderMessages(a.state.Messages, a.spinner.View())
	a.viewport = a.viewport.SetContent(content)
}

func (a App) chatLine() string {
	s := ""
	for i := 0; i < a.width; i++ {
		s += "─"
	}
	return chatLineStyle.Render(s)
}

func (a App) View() string {
	topBar := panels.RenderTopBar(a.state, a.width)
	line := a.chatLine()
	chat := a.viewport.View()
	input := "  " + a.input.View()
	hintBar := panels.RenderHintBar(a.width)

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		line,
		chat,
		line,
		input,
		hintBar,
	)
}
