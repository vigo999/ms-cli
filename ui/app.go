package ui

import (
	"fmt"
	"os"
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
	baseHintHeight = 2 // divider + hint header
	inputHeight    = 1
	verticalPad    = 2
	ctrlCExitAfter = 2 * time.Second
	altScrollSeq   = "\x1b[?1007h"
)

const InterruptSignal = "__mscli_interrupt__"

type slashCommandSpec struct {
	command     string
	usage       string
	subcommands []string
}

type slashCandidate struct {
	command string
	display string
	insert  string
}

type stepProgress struct {
	commands      int
	viewedFiles   map[string]struct{}
	modifiedFiles map[string]struct{}
}

var (
	chatLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	slashCatalog  = []slashCommandSpec{
		{
			command:     "/roadmap",
			usage:       "status [path]",
			subcommands: []string{"status"},
		},
		{
			command:     "/weekly",
			usage:       "status [path]",
			subcommands: []string{"status"},
		},
		{
			command:     "/model",
			usage:       "use <provider>/<model> | list | show",
			subcommands: []string{"use", "list", "show"},
		},
		{
			command:     "/perm",
			usage:       "status | yolo on|off | whitelist ... | blacklist ...",
			subcommands: []string{"status", "yolo", "whitelist", "blacklist"},
		},
		{
			command:     "/approve",
			usage:       "once | session",
			subcommands: []string{"once", "session"},
		},
		{
			command: "/reject",
		},
		{
			command: "/compact",
			usage:   "[keep]",
		},
		{
			command: "/clear",
		},
		{
			command: "/exit",
		},
	}
)

// App is the TUI root model.
type App struct {
	state           model.State
	viewport        components.Viewport
	input           components.TextInput
	spinner         components.Spinner
	step            stepProgress
	width           int
	height          int
	eventCh         <-chan model.Event
	userCh          chan<- string // sends user input to the engine bridge
	slashCandidates []slashCandidate
	slashSelected   int
	ctrlCArmed      bool
	lastCtrlCAt     time.Time
}

// New creates a new App driven by the given event channel.
// userCh may be nil (demo mode) — user input won't be forwarded.
func New(ch <-chan model.Event, userCh chan<- string, version, workDir, repoURL, modelProvider, modelName string, ctxMax int) App {
	return App{
		state:         model.NewState(version, workDir, repoURL, modelProvider, modelName, ctxMax),
		input:         components.NewTextInput(),
		spinner:       components.NewSpinner(),
		step:          newStepProgress(),
		eventCh:       ch,
		userCh:        userCh,
		slashSelected: 0,
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
		enableAltScrollCmd(),
		a.spinner.Model.Tick,
		a.waitForEvent,
	)
}

func enableAltScrollCmd() tea.Cmd {
	return func() tea.Msg {
		_, _ = os.Stdout.WriteString(altScrollSeq)
		return nil
	}
}

func (a App) chatHeight() int {
	h := a.height - topBarHeight - chatLineHeight - a.hintHeight() - inputHeight - verticalPad
	if h < 1 {
		return 1
	}
	return h
}

func (a App) hintHeight() int {
	if len(a.slashCandidates) == 0 {
		return baseHintHeight
	}
	return baseHintHeight + len(a.slashCandidates)
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
		a.resizeViewport()
		return a, enableAltScrollCmd()

	case model.Event:
		return a.handleEvent(msg)

	default:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if a.hasThinkingMessage() {
			a.updateViewport()
		}
	}

	return a, tea.Batch(cmds...)
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "ctrl+c" {
		a.ctrlCArmed = false
	}

	switch msg.String() {
	case "ctrl+c":
		if a.ctrlCArmed && time.Since(a.lastCtrlCAt) <= ctrlCExitAfter {
			return a, tea.Quit
		}

		a.ctrlCArmed = true
		a.lastCtrlCAt = time.Now()
		a.input = a.input.Reset()
		a.slashCandidates = nil
		a.slashSelected = 0
		a.resizeViewport()
		a.state = a.state.WithMessage(model.Message{
			Kind:    model.MsgAgent,
			Content: "已取消当前输入，并发送暂停任务请求。再次按 Ctrl+C 退出。",
		})
		a.updateViewport()
		if a.userCh != nil {
			select {
			case a.userCh <- InterruptSignal:
			default:
			}
		}
		return a, nil

	case "tab":
		if a.hasSlashSuggestions() {
			a.applySelectedSlash()
			a.refreshSlashSuggestions()
			return a, nil
		}
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		a.refreshSlashSuggestions()
		return a, cmd

	case "up":
		if a.hasSlashSuggestions() {
			a.cycleSlash(-1)
			return a, nil
		}
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case "down":
		if a.hasSlashSuggestions() {
			a.cycleSlash(1)
			return a, nil
		}
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	case "enter":
		val := strings.TrimSpace(a.input.Value())
		if val == "" {
			return a, nil
		}
		if val == "/" && a.hasSlashSuggestions() {
			val = a.slashCandidates[a.slashSelected].command
		}

		a.state = a.state.WithMessage(model.Message{Kind: model.MsgUser, Content: val})
		a.input = a.input.Reset()
		a.slashCandidates = nil
		a.slashSelected = 0
		a.resizeViewport()
		a.updateViewport()
		if a.userCh != nil {
			select {
			case a.userCh <- val:
			default:
				a.state = a.state.WithMessage(model.Message{
					Kind:    model.MsgAgent,
					Content: "Input queue is busy; the message was not sent. Please retry.",
				})
				a.updateViewport()
			}
		}
		return a, nil

	case "pgup", "pgdown", "home", "end":
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		return a, cmd

	default:
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		a.refreshSlashSuggestions()
		return a, cmd
	}
}

func (a App) handleEvent(ev model.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case model.AgentThinking:
		a.completeThinkingStep()
		a.resetStepProgress()
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgThinking})

	case model.AgentReply:
		a.completeThinkingStep()
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.ApprovalRequired:
		msg := fmt.Sprintf("Approval required (#%d): %s %s", ev.ApprovalID, ev.ApprovalTool, strings.TrimSpace(ev.ApprovalAction))
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: msg})

	case model.ApprovalResolved:
		msg := fmt.Sprintf("Approval resolved (#%d): %s", ev.ApprovalID, strings.TrimSpace(ev.Message))
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: msg})

	case model.CmdStarted:
		a.step.commands++
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
		a.trackViewedFile(ev.Message)
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
		a.trackModifiedFile(ev.Message)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Edit",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolWrite:
		a.trackModifiedFile(ev.Message)
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: "Write",
			Display:  model.DisplayExpanded,
			Content:  ev.Message,
		})

	case model.ToolError:
		a.completeThinkingStep()
		a.state = a.state.WithMessage(model.Message{
			Kind:     model.MsgTool,
			ToolName: ev.ToolName,
			Display:  model.DisplayError,
			Content:  ev.Message,
		})

	case model.AnalysisReady:
		a.completeThinkingStep()
		a.state = a.state.WithMessage(model.Message{Kind: model.MsgAgent, Content: ev.Message})

	case model.TokenUpdate:
		mi := a.state.Model
		mi.CtxUsed = ev.CtxUsed
		mi.TokensUsed = ev.TokensUsed
		a.state = a.state.WithModel(mi)

	case model.ModelUpdate:
		mi := a.state.Model
		if ev.ModelProvider != "" {
			mi.Provider = ev.ModelProvider
		}
		if ev.ModelName != "" {
			mi.Name = ev.ModelName
		}
		if ev.ModelCtxMax > 0 {
			mi.CtxMax = ev.ModelCtxMax
		}
		a.state = a.state.WithModel(mi)

	case model.ClearChat:
		a.resetStepProgress()
		a.state = a.withMessages(nil)
		a.viewport = a.viewport.Clear()

	case model.CompactChat:
		keep := ev.KeepMessages
		if keep <= 0 {
			keep = 12
		}
		msgs := a.state.Messages
		if len(msgs) > keep {
			msgs = append([]model.Message{}, msgs[len(msgs)-keep:]...)
		}
		a.state = a.withMessages(msgs)

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
	return a.withMessages(msgs)
}

func newStepProgress() stepProgress {
	return stepProgress{
		viewedFiles:   make(map[string]struct{}),
		modifiedFiles: make(map[string]struct{}),
	}
}

func (a *App) resetStepProgress() {
	a.step = newStepProgress()
}

func (a *App) completeThinkingStep() {
	if !a.hasThinkingMessage() {
		return
	}
	summary, ok := a.stepDoneSummary()
	if ok {
		a.state = a.replaceThinking(model.Message{
			Kind:    model.MsgAgent,
			Content: summary,
		})
	} else {
		a.state = a.removeThinking()
	}
	a.resetStepProgress()
}

func (a App) stepDoneSummary() (string, bool) {
	parts := make([]string, 0, 3)
	if a.step.commands > 0 {
		parts = append(parts, fmt.Sprintf("commands: %d", a.step.commands))
	}
	if viewed := len(a.step.viewedFiles); viewed > 0 {
		parts = append(parts, fmt.Sprintf("files viewed: %d", viewed))
	}
	if modified := len(a.step.modifiedFiles); modified > 0 {
		parts = append(parts, fmt.Sprintf("files modified: %d", modified))
	}
	if len(parts) == 0 {
		return "", false
	}
	return "Done  -- " + strings.Join(parts, ", "), true
}

func (a *App) trackViewedFile(raw string) {
	path := firstNonEmptyLine(raw)
	if path == "" {
		return
	}
	if a.step.viewedFiles == nil {
		a.step.viewedFiles = make(map[string]struct{})
	}
	a.step.viewedFiles[path] = struct{}{}
}

func (a *App) trackModifiedFile(raw string) {
	path := firstNonEmptyLine(raw)
	if path == "" {
		return
	}
	if a.step.modifiedFiles == nil {
		a.step.modifiedFiles = make(map[string]struct{})
	}
	a.step.modifiedFiles[path] = struct{}{}
}

func firstNonEmptyLine(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (a App) removeThinking() model.State {
	msgs := make([]model.Message, 0, len(a.state.Messages))
	for _, msg := range a.state.Messages {
		if msg.Kind != model.MsgThinking {
			msgs = append(msgs, msg)
		}
	}
	return a.withMessages(msgs)
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
	return a.withMessages(msgs)
}

func (a App) withMessages(msgs []model.Message) model.State {
	return model.State{
		Version:          a.state.Version,
		Tasks:            a.state.Tasks,
		ActiveTask:       a.state.ActiveTask,
		Model:            a.state.Model,
		Messages:         msgs,
		ShowTaskSelector: a.state.ShowTaskSelector,
		WorkDir:          a.state.WorkDir,
		RepoURL:          a.state.RepoURL,
	}
}

func (a *App) updateViewport() {
	wrapWidth := a.width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	content := panels.RenderMessages(a.state.Messages, a.spinner.View(), wrapWidth)
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
	hintBar := panels.RenderHintBar(a.width, a.slashCandidateDisplays(), a.slashSelected)

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		line,
		chat,
		line,
		input,
		hintBar,
	)
}

func (a *App) refreshSlashSuggestions() {
	val := strings.TrimSpace(a.input.Value())
	if !strings.HasPrefix(val, "/") {
		a.slashCandidates = nil
		a.slashSelected = 0
		a.resizeViewport()
		return
	}

	candidates := make([]slashCandidate, 0, len(slashCatalog))
	for _, spec := range slashCatalog {
		if slashMatches(spec, val) {
			candidates = append(candidates, makeSlashCandidate(spec, val))
		}
	}
	if len(candidates) == 0 {
		a.slashCandidates = nil
		a.slashSelected = 0
		a.resizeViewport()
		return
	}
	a.slashCandidates = candidates
	if a.slashSelected >= len(candidates) {
		a.slashSelected = 0
	}
	a.resizeViewport()
}

func (a App) hasSlashSuggestions() bool {
	return len(a.slashCandidates) > 0
}

func (a App) hasThinkingMessage() bool {
	for _, msg := range a.state.Messages {
		if msg.Kind == model.MsgThinking {
			return true
		}
	}
	return false
}

func (a *App) cycleSlash(delta int) {
	if len(a.slashCandidates) == 0 {
		return
	}
	n := len(a.slashCandidates)
	a.slashSelected = (a.slashSelected + delta + n) % n
}

func (a *App) applySelectedSlash() {
	if len(a.slashCandidates) == 0 {
		return
	}
	a.input = a.input.SetValue(a.slashCandidates[a.slashSelected].insert)
}

func (a App) slashCandidateDisplays() []string {
	out := make([]string, 0, len(a.slashCandidates))
	for _, c := range a.slashCandidates {
		out = append(out, c.display)
	}
	return out
}

func (a *App) resizeViewport() {
	if a.width <= 0 {
		return
	}
	w := a.width - 4
	if w < 1 {
		w = 1
	}
	a.viewport = a.viewport.SetSize(w, a.chatHeight())
}

func slashMatches(spec slashCommandSpec, input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "/" {
		return true
	}

	cmdPart, subPart, hasSub := splitSlashInput(trimmed)
	if cmdPart == "" {
		return true
	}

	target := strings.TrimPrefix(spec.command, "/")
	if !hasSub {
		return strings.HasPrefix(strings.ToLower(target), strings.ToLower(cmdPart))
	}

	if !strings.EqualFold(target, cmdPart) {
		return strings.HasPrefix(strings.ToLower(target), strings.ToLower(cmdPart))
	}

	if subPart == "" {
		return true
	}

	for _, sub := range spec.subcommands {
		if strings.HasPrefix(strings.ToLower(sub), strings.ToLower(subPart)) {
			return true
		}
	}
	return false
}

func makeSlashCandidate(spec slashCommandSpec, input string) slashCandidate {
	display := spec.command
	if spec.usage != "" {
		display += "  " + spec.usage
	}

	insert := spec.command
	if spec.usage != "" {
		insert = spec.command + " "
	}

	cmdPart, subPart, hasSub := splitSlashInput(strings.TrimSpace(input))
	target := strings.TrimPrefix(spec.command, "/")
	if !hasSub {
		return slashCandidate{
			command: spec.command,
			display: display,
			insert:  insert,
		}
	}

	if strings.EqualFold(target, cmdPart) && subPart != "" {
		if sub, ok := resolveSubcommand(spec.subcommands, subPart); ok {
			insert = spec.command + " " + sub
			if subcommandNeedsArg(spec.command, sub) {
				insert += " "
			}
		}
	}

	return slashCandidate{
		command: spec.command,
		display: display,
		insert:  insert,
	}
}

func splitSlashInput(input string) (cmdPart string, subPart string, hasSub bool) {
	s := strings.TrimSpace(input)
	if !strings.HasPrefix(s, "/") {
		return "", "", false
	}
	payload := strings.TrimPrefix(s, "/")
	if payload == "" {
		return "", "", false
	}

	parts := strings.Fields(payload)
	if len(parts) == 0 {
		return "", "", false
	}
	cmdPart = parts[0]

	switch {
	case len(parts) >= 2:
		return cmdPart, parts[1], true
	case strings.HasSuffix(s, " "):
		return cmdPart, "", true
	default:
		return cmdPart, "", false
	}
}

func resolveSubcommand(subcommands []string, partial string) (string, bool) {
	match := ""
	needle := strings.ToLower(strings.TrimSpace(partial))
	for _, sub := range subcommands {
		if strings.HasPrefix(strings.ToLower(sub), needle) {
			if match != "" {
				return "", false
			}
			match = sub
		}
	}
	if match == "" {
		return "", false
	}
	return match, true
}

func subcommandNeedsArg(command, sub string) bool {
	switch command {
	case "/model":
		return sub == "use"
	case "/roadmap", "/weekly":
		return sub == "status"
	case "/perm":
		return sub == "yolo" || sub == "whitelist" || sub == "blacklist"
	default:
		return false
	}
}
