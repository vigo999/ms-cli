package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/internal/project"
	"github.com/vigo999/ms-cli/ui/model"
)

// handleCommand dispatches slash commands.
func (a *Application) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/roadmap":
		a.cmdRoadmap(parts[1:])
	case "/weekly":
		a.cmdWeekly(parts[1:])
	case "/model":
		a.cmdModel(parts[1:])
	case "/perm":
		a.cmdPerm(parts[1:])
	case "/approve":
		a.cmdApprove(parts[1:])
	case "/reject":
		a.cmdReject()
	case "/clear":
		a.cmdClear()
	case "/compact":
		a.cmdCompact(parts[1:])
	case "/exit":
		a.cmdExit()
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Unknown command: %s", parts[0]),
		}
	}
}

// cmdRoadmap handles "/roadmap status [path]".
func (a *Application) cmdRoadmap(args []string) {
	if len(args) == 0 || args[0] != "status" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /roadmap status [path] (default: roadmap.yaml)",
		}
		return
	}

	path := "roadmap.yaml"
	if len(args) > 1 {
		path = args[1]
	}

	a.EventCh <- model.Event{Type: model.AgentThinking}

	rm, err := project.LoadRoadmapFromFile(path)
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "roadmap",
			Message:  err.Error(),
		}
		return
	}

	status, err := project.ComputeRoadmapStatus(rm, time.Now())
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "roadmap",
			Message:  err.Error(),
		}
		return
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: string(data),
	}
}

// cmdWeekly handles "/weekly status [path]".
func (a *Application) cmdWeekly(args []string) {
	if len(args) == 0 || args[0] != "status" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /weekly status [path] (default: weekly.md, fallback: docs/updates/WEEKLY_TEMPLATE.md)",
		}
		return
	}

	path := resolveWeeklyPath()
	if len(args) > 1 {
		path = args[1]
	}

	a.EventCh <- model.Event{Type: model.AgentThinking}

	wu, err := project.LoadWeeklyUpdateFromFile(path)
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "weekly",
			Message:  err.Error(),
		}
		return
	}

	data, _ := json.MarshalIndent(wu, "", "  ")
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: string(data),
	}
}

func (a *Application) cmdModel(args []string) {
	if len(args) == 0 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /model list | /model show | /model use <provider>/<model>",
		}
		return
	}

	switch args[0] {
	case "list":
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: a.listModelProviders(),
		}
	case "show":
		ctxWindow := a.Config.ResolveContextWindow(a.SessionModel.Provider, a.SessionModel.Name)
		ctxBudget := a.Config.ResolveContextBudget(a.SessionModel.Provider, a.SessionModel.Name)
		a.EventCh <- model.Event{
			Type: model.AgentReply,
			Message: fmt.Sprintf(
				"Current model: %s/%s (%s)\nctx_window: %d\nctx_budget: %d",
				a.SessionModel.Provider,
				a.SessionModel.Name,
				a.SessionModel.Endpoint,
				ctxWindow,
				ctxBudget,
			),
		}
	case "use":
		if len(args) < 2 {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Usage: /model use <provider>/<model>",
			}
			return
		}
		provider, name, ok := parseModelRef(args[1])
		if !ok {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Invalid model format. Expect: <provider>/<model>",
			}
			return
		}
		switch provider {
		case "openai", "openrouter":
		default:
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: "Unsupported provider. Use openai or openrouter.",
			}
			return
		}

		a.SessionModel = a.Config.ResolveModel(provider, name)
		ctxMax := a.Config.ResolveContextWindow(a.SessionModel.Provider, a.SessionModel.Name)
		if err := a.persistSessionState(); err != nil {
			a.EventCh <- model.Event{
				Type:     model.ToolError,
				ToolName: "model",
				Message:  "persist session state failed: " + err.Error(),
			}
		}
		a.EventCh <- model.Event{
			Type:          model.ModelUpdate,
			ModelProvider: a.SessionModel.Provider,
			ModelName:     a.SessionModel.Name,
			ModelCtxMax:   ctxMax,
		}
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Switched model to %s/%s", a.SessionModel.Provider, a.SessionModel.Name),
		}
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /model list | /model show | /model use <provider>/<model>",
		}
	}
}

func (a *Application) cmdClear() {
	a.EventCh <- model.Event{Type: model.ClearChat}
}

func (a *Application) cmdCompact(args []string) {
	keep := 12
	if len(args) > 0 {
		// optional: "/compact 20"
		if n, err := parsePositiveInt(args[0]); err == nil && n > 0 {
			keep = n
		}
	}
	a.EventCh <- model.Event{
		Type:         model.CompactChat,
		KeepMessages: keep,
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Compacted chat history to last %d messages.", keep),
	}
}

func (a *Application) cmdExit() {
	a.cancelActiveTask()
	a.EventCh <- model.Event{Type: model.Done}
}

func (a *Application) cmdApprove(args []string) {
	if a.Permission == nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission manager is not initialized.",
		}
		return
	}
	if len(args) == 0 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /approve once | /approve session",
		}
		return
	}

	switch args[0] {
	case "once":
		req, err := a.Permission.ApproveOncePending()
		if err != nil {
			a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
			return
		}
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Approved once: %s %s", req.Tool, req.Action),
		}
	case "session":
		req, err := a.Permission.ApproveSessionPending()
		if err != nil {
			a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
			return
		}
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Approved this session: %s %s", req.Tool, req.Action),
		}
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /approve once | /approve session",
		}
	}
}

func (a *Application) cmdReject() {
	if a.Permission == nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission manager is not initialized.",
		}
		return
	}
	req, err := a.Permission.RejectPending()
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: err.Error()}
		return
	}
	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Rejected: %s %s", req.Tool, req.Action),
	}
}

func (a *Application) cmdPerm(args []string) {
	if a.Permission == nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission manager is not initialized.",
		}
		return
	}

	if len(args) == 0 || args[0] == "status" {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: a.renderPermStatus()}
		return
	}

	switch args[0] {
	case "yolo":
		if len(args) < 2 {
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm yolo on|off"}
			return
		}
		switch strings.ToLower(args[1]) {
		case "on":
			a.Permission.SetYolo(true)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "YOLO mode enabled."}
		case "off":
			a.Permission.SetYolo(false)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "YOLO mode disabled."}
		default:
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm yolo on|off"}
		}

	case "whitelist":
		a.cmdPermListSet("whitelist", args[1:])

	case "blacklist":
		a.cmdPermListSet("blacklist", args[1:])

	default:
		a.EventCh <- model.Event{
			Type: model.AgentReply,
			Message: "Usage: /perm status | /perm yolo on|off | " +
				"/perm whitelist list|add|remove <tool> | " +
				"/perm blacklist list|add|remove <tool>",
		}
	}
}

func (a *Application) cmdPermListSet(listName string, args []string) {
	if len(args) == 0 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Usage: /perm %s list|add|remove <tool>", listName),
		}
		return
	}
	action := strings.ToLower(args[0])
	tool := ""
	if len(args) > 1 {
		tool = strings.ToLower(strings.TrimSpace(args[1]))
	}

	switch listName {
	case "whitelist":
		switch action {
		case "list":
			st := a.Permission.Status()
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "whitelist: " + strings.Join(st.Whitelist, ", ")}
		case "add":
			if tool == "" {
				a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm whitelist add <tool>"}
				return
			}
			a.Permission.AddWhitelist(tool)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "whitelist updated."}
		case "remove":
			if tool == "" {
				a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm whitelist remove <tool>"}
				return
			}
			a.Permission.RemoveWhitelist(tool)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "whitelist updated."}
		default:
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm whitelist list|add|remove <tool>"}
		}

	case "blacklist":
		switch action {
		case "list":
			st := a.Permission.Status()
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "blacklist: " + strings.Join(st.Blacklist, ", ")}
		case "add":
			if tool == "" {
				a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm blacklist add <tool>"}
				return
			}
			a.Permission.AddBlacklist(tool)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "blacklist updated."}
		case "remove":
			if tool == "" {
				a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm blacklist remove <tool>"}
				return
			}
			a.Permission.RemoveBlacklist(tool)
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "blacklist updated."}
		default:
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /perm blacklist list|add|remove <tool>"}
		}
	}
}

func (a *Application) renderPermStatus() string {
	st := a.Permission.Status()
	lines := []string{
		fmt.Sprintf("yolo_mode: %t", st.Yolo),
		fmt.Sprintf("session_approved: %d", st.SessionApproved),
		fmt.Sprintf("whitelist: %s", joinOrNone(st.Whitelist)),
		fmt.Sprintf("blacklist: %s", joinOrNone(st.Blacklist)),
		fmt.Sprintf("pending_count: %d", st.PendingCount),
	}
	if st.Pending != nil {
		lines = append(lines, fmt.Sprintf("pending: [%d] %s %s", st.Pending.ID, st.Pending.Tool, st.Pending.Action))
	} else {
		lines = append(lines, "pending: none")
	}
	return strings.Join(lines, "\n")
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func (a *Application) listModelProviders() string {
	lines := []string{
		"Available providers:",
		fmt.Sprintf("- openai  (%s)", a.Config.Providers.OpenAI.Endpoint),
		fmt.Sprintf("- openrouter  (%s)", a.Config.Providers.OpenRouter.Endpoint),
		"",
		"Current:",
		fmt.Sprintf("- %s/%s", a.SessionModel.Provider, a.SessionModel.Name),
	}
	return strings.Join(lines, "\n")
}

func parseModelRef(ref string) (provider string, modelName string, ok bool) {
	ref = strings.TrimSpace(ref)
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	p := strings.ToLower(strings.TrimSpace(parts[0]))
	m := strings.TrimSpace(parts[1])
	if p == "" || m == "" {
		return "", "", false
	}
	return p, m, true
}

func parsePositiveInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func resolveWeeklyPath() string {
	candidates := []string{
		"weekly.md",
		"docs/updates/WEEKLY_TEMPLATE.md",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "weekly.md"
}
