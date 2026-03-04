package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentcontext "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/integrations/domain"
)

const plannerSystemPrompt = `You are ms-cli planner.
Return ONLY JSON, no markdown.
Schema:
{
  "action":"glob|read|grep|edit|write|shell|final",
  "path":"optional",
  "pattern":"optional regex for grep",
  "old_text":"optional for edit",
  "new_text":"optional for edit",
  "content":"optional for write",
  "command":"optional for shell",
  "final":"required when action=final"
}
Rules:
- Prefer small, safe steps.
- Use relative paths.
- Use shell only when needed.
- Do not repeat the same shell command unless there is a clear reason.
- Do not repeat the same glob/read/grep action with identical inputs more than once unless new evidence appears.
- Prefer read/grep for code structure analysis; avoid repeated "ls -la".
- If enough evidence exists, return action=final with concise conclusion.`

type Config struct {
	FS                    FSTool
	Shell                 ShellTool
	ModelFactory          domain.Factory
	Permission            PermissionService
	Trace                 TraceWriter
	DefaultMaxStep        int
	MaxOutputLines        int
	ContextMaxTokens      int
	ContextCompactionRate float64
	ContextMaxEntries     int
	MaxRepeatedShell      int
	MaxWallTimeSec        int
	MaxTotalTokens        int
	MaxTotalCostUSD       float64
	RequireApprovalBlock  bool
}

// Engine drives task execution and emits events.
type Engine struct {
	fs                    FSTool
	shell                 ShellTool
	modelFactory          domain.Factory
	permission            PermissionService
	trace                 TraceWriter
	defaultMaxStep        int
	maxOutputLines        int
	contextMaxTokens      int
	contextCompactionRate float64
	contextMaxEntries     int
	maxRepeatedShell      int
	maxWallTime           time.Duration
	maxTotalTokens        int
	maxTotalCostUSD       float64
	requireApprovalBlock  bool
}

const (
	maxRepeatedScanAction = 3
)

const assistantSystemPrompt = `You are ms-cli assistant in a terminal UI.
Be concise and practical.`

type plannerAction struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
	Content string `json:"content"`
	Command string `json:"command"`
	Final   string `json:"final"`
}

func NewEngine(cfg Config) *Engine {
	maxSteps := cfg.DefaultMaxStep
	if maxSteps < 0 {
		maxSteps = 0
	}
	maxOutputLines := cfg.MaxOutputLines
	if maxOutputLines <= 0 {
		maxOutputLines = 200
	}
	contextTokens := cfg.ContextMaxTokens
	if contextTokens <= 0 {
		contextTokens = 12000
	}
	contextRatio := cfg.ContextCompactionRate
	if contextRatio <= 0 || contextRatio >= 1 {
		contextRatio = 0.8
	}
	contextEntries := cfg.ContextMaxEntries
	if contextEntries <= 0 {
		contextEntries = 80
	}
	maxRepeatedShell := cfg.MaxRepeatedShell
	if maxRepeatedShell <= 0 {
		maxRepeatedShell = 2
	}
	maxWallTime := time.Duration(cfg.MaxWallTimeSec) * time.Second
	maxTotalTokens := cfg.MaxTotalTokens
	if maxTotalTokens < 0 {
		maxTotalTokens = 0
	}
	maxTotalCostUSD := cfg.MaxTotalCostUSD
	if maxTotalCostUSD < 0 {
		maxTotalCostUSD = 0
	}

	return &Engine{
		fs:                    cfg.FS,
		shell:                 cfg.Shell,
		modelFactory:          cfg.ModelFactory,
		permission:            cfg.Permission,
		trace:                 cfg.Trace,
		defaultMaxStep:        maxSteps,
		maxOutputLines:        maxOutputLines,
		contextMaxTokens:      contextTokens,
		contextCompactionRate: contextRatio,
		contextMaxEntries:     contextEntries,
		maxRepeatedShell:      maxRepeatedShell,
		maxWallTime:           maxWallTime,
		maxTotalTokens:        maxTotalTokens,
		maxTotalCostUSD:       maxTotalCostUSD,
		requireApprovalBlock:  cfg.RequireApprovalBlock,
	}
}

func (e *Engine) Run(task Task) ([]Event, error) {
	return e.runWithContext(context.Background(), task, nil)
}

func (e *Engine) RunWithContext(ctx context.Context, task Task) ([]Event, error) {
	return e.runWithContext(ctx, task, nil)
}

func (e *Engine) RunWithContextStream(ctx context.Context, task Task, emit func(Event)) error {
	_, err := e.runWithContext(ctx, task, emit)
	return err
}

func (e *Engine) runWithContext(ctx context.Context, task Task, emit func(Event)) ([]Event, error) {
	events := make([]Event, 0, 32)
	push := func(ev Event) {
		events = append(events, ev)
		if emit != nil {
			emit(ev)
		}
	}

	if strings.TrimSpace(task.Description) == "" {
		return nil, fmt.Errorf("task description is required")
	}
	if e.modelFactory == nil {
		return nil, fmt.Errorf("model factory is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		push(e.newEvent(EventReply, "任务已暂停。", "", ""))
		return events, err
	}

	client, err := e.modelFactory.ClientFor(domain.ModelSpec{
		Provider: task.Model.Provider,
		Model:    task.Model.Name,
		Endpoint: task.Model.Endpoint,
	})
	if err != nil {
		ev := e.newEvent(EventToolError, err.Error(), "Model", "")
		push(ev)
		return events, err
	}

	if shouldDirectReply(task.Description) {
		directEvents, directErr := e.runDirectReply(ctx, client, task)
		if errors.Is(directErr, context.Canceled) {
			push(e.newEvent(EventReply, "任务已暂停。", "", ""))
			return events, directErr
		}
		for _, ev := range directEvents {
			push(ev)
		}
		return events, directErr
	}

	maxSteps := task.MaxSteps
	if maxSteps < 0 {
		maxSteps = 0
	}
	if maxSteps == 0 {
		maxSteps = e.defaultMaxStep
	}
	if maxSteps < 0 {
		maxSteps = 0
	}

	contextBudget := e.contextMaxTokens
	if task.ContextMaxTokens > 0 {
		contextBudget = task.ContextMaxTokens
	}

	ctxManager := agentcontext.NewManager(
		contextBudget,
		e.contextCompactionRate,
		e.contextMaxEntries,
	)
	ctxManager.Add("task", task.Description)
	shellGuard := newShellLoopGuard(e.maxRepeatedShell, 6)
	scanGuard := newShellLoopGuard(maxRepeatedScanAction, 8)

	totalTokens := 0
	totalCostUSD := 0.0
	startedAt := time.Now()

	for step := 1; ; step++ {
		if maxSteps > 0 && step > maxSteps {
			break
		}
		if e.maxWallTime > 0 && time.Since(startedAt) > e.maxWallTime {
			msg := fmt.Sprintf("Stopped: wall-time budget exceeded (%s).", e.maxWallTime.Round(time.Second))
			push(e.newEvent(EventReply, msg, "", ""))
			_ = e.writeTrace("budget_exceeded", map[string]any{"type": "wall_time", "limit_sec": int(e.maxWallTime.Seconds())})
			return events, nil
		}
		if err := ctx.Err(); err != nil {
			push(e.newEvent(EventReply, "任务已暂停。", "", ""))
			return events, err
		}
		thinking := fmt.Sprintf("Planning step %d...", step)
		if maxSteps > 0 {
			thinking = fmt.Sprintf("Planning step %d/%d...", step, maxSteps)
		}
		push(e.newEvent(EventThinking, thinking, "", ""))

		renderedContext := ctxManager.Render()
		ctxUsedNow := agentcontext.ApproxTokens(renderedContext)

		action, usage, raw, planErr := e.planNext(ctx, client, task, renderedContext, step, maxSteps)
		if usage != nil {
			totalTokens += usage.TotalTokens
			totalCostUSD += estimateCostUSD(task.Model.Provider, *usage)
			push(Event{
				Type:       EventTokenUsage,
				CtxUsed:    ctxUsedNow,
				TokensUsed: totalTokens,
				Time:       time.Now().UTC(),
			})
			if e.maxTotalTokens > 0 && totalTokens >= e.maxTotalTokens {
				msg := fmt.Sprintf("Stopped: token budget exceeded (%d/%d).", totalTokens, e.maxTotalTokens)
				push(e.newEvent(EventReply, msg, "", ""))
				_ = e.writeTrace("budget_exceeded", map[string]any{"type": "tokens", "used": totalTokens, "limit": e.maxTotalTokens})
				return events, nil
			}
			if e.maxTotalCostUSD > 0 && totalCostUSD >= e.maxTotalCostUSD {
				msg := fmt.Sprintf("Stopped: estimated cost budget exceeded (%.4f/%.4f USD).", totalCostUSD, e.maxTotalCostUSD)
				push(e.newEvent(EventReply, msg, "", ""))
				_ = e.writeTrace("budget_exceeded", map[string]any{"type": "cost_usd", "used": totalCostUSD, "limit": e.maxTotalCostUSD})
				return events, nil
			}
		}
		if planErr != nil {
			if errors.Is(planErr, context.Canceled) {
				push(e.newEvent(EventReply, "任务已暂停。", "", ""))
				return events, planErr
			}
			ev := e.newEvent(EventToolError, planErr.Error(), "Planner", "")
			push(ev)
			_ = e.writeTrace("planner_error", map[string]any{"error": planErr.Error()})
			return events, planErr
		}

		if strings.TrimSpace(action.Action) == "" {
			action.Action = "final"
			action.Final = strings.TrimSpace(raw)
		}

		switch strings.ToLower(action.Action) {
		case "final":
			finalMsg := strings.TrimSpace(action.Final)
			if finalMsg == "" {
				finalMsg = strings.TrimSpace(raw)
			}
			if finalMsg == "" {
				finalMsg = "Done."
			}
			push(e.newEvent(EventReply, finalMsg, "", ""))
			_ = e.writeTrace("task_complete", map[string]any{"task": task.Description, "steps": step})
			return events, nil

		case "glob":
			pattern := strings.TrimSpace(action.Pattern)
			if pattern == "" {
				pattern = strings.TrimSpace(action.Path)
			}
			if pattern == "" {
				pattern = "**/*"
			}
			target := "."
			if repeats := scanGuard.Observe("glob|" + normalizeCommand(target+" "+pattern)); repeats > maxRepeatedScanAction {
				warn := fmt.Sprintf("detected repeated glob action %q in %s (%d times). choose different action or return final.", pattern, target, repeats)
				push(e.newEvent(EventToolError, warn, "Planner", ""))
				ctxManager.Add("guard", warn)
				if repeats > maxRepeatedScanAction+1 {
					push(e.newEvent(EventReply, "检测到重复 Glob 循环，已自动停止。请细化任务范围或指定明确路径。", "", ""))
					return events, nil
				}
				continue
			}
			allowed, permErr := e.checkPermission(ctx, "glob", pattern, target, push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("glob", "denied by permissions: "+pattern)
				continue
			}
			matches, globErr := e.fs.Glob(target, pattern, 100)
			if globErr != nil {
				push(e.newEvent(EventToolError, globErr.Error(), "Glob", ""))
				ctxManager.Add("glob", "failed: "+globErr.Error())
				continue
			}
			msg := fmt.Sprintf("%q in %s", pattern, target)
			push(e.newEvent(EventToolGlob, msg, "Glob", fmt.Sprintf("%d files", len(matches))))
			ctxManager.Add("glob", fmt.Sprintf("%s\n%s", msg, truncate(strings.Join(matches, "\n"), 2000)))
			_ = e.writeTrace("tool_glob", map[string]any{"path": target, "pattern": pattern, "matches": len(matches)})

		case "read":
			path := strings.TrimSpace(action.Path)
			if repeats := scanGuard.Observe("read|" + normalizeCommand(path)); repeats > maxRepeatedScanAction {
				warn := fmt.Sprintf("detected repeated read action %q (%d times). choose different action or return final.", path, repeats)
				push(e.newEvent(EventToolError, warn, "Planner", ""))
				ctxManager.Add("guard", warn)
				if repeats > maxRepeatedScanAction+1 {
					push(e.newEvent(EventReply, "检测到重复读取循环，已自动停止。请细化任务范围。", "", ""))
					return events, nil
				}
				continue
			}
			allowed, permErr := e.checkPermission(ctx, "read", action.Path, action.Path, push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("read", "denied by permissions: "+strings.TrimSpace(action.Path))
				continue
			}
			content, readErr := e.fs.Read(action.Path)
			if readErr != nil {
				// Model may occasionally choose reading "." or a directory.
				// Keep this internal and let planner continue instead of surfacing noisy tool errors.
				if isDirectoryErr(readErr) {
					ctxManager.Add("read", fmt.Sprintf("%s failed: target is a directory", action.Path))
					continue
				}
				push(e.newEvent(EventToolError, readErr.Error(), "Read", ""))
				ctxManager.Add("read", "failed: "+readErr.Error())
				continue
			}
			summary := fmt.Sprintf("%d lines", countLines(content))
			push(e.newEvent(EventToolRead, action.Path, "Read", summary))
			ctxManager.Add("read", fmt.Sprintf("%s (%s)\n%s", action.Path, summary, truncate(content, 2000)))
			_ = e.writeTrace("tool_read", map[string]any{"path": action.Path})

		case "grep":
			target := strings.TrimSpace(action.Path)
			if target == "" {
				target = "."
			}
			if repeats := scanGuard.Observe("grep|" + normalizeCommand(target+" "+action.Pattern)); repeats > maxRepeatedScanAction {
				warn := fmt.Sprintf("detected repeated grep action %q in %s (%d times). choose different action or return final.", action.Pattern, target, repeats)
				push(e.newEvent(EventToolError, warn, "Planner", ""))
				ctxManager.Add("guard", warn)
				if repeats > maxRepeatedScanAction+1 {
					push(e.newEvent(EventReply, "检测到重复检索循环，已自动停止。请细化检索模式。", "", ""))
					return events, nil
				}
				continue
			}
			allowed, permErr := e.checkPermission(ctx, "grep", action.Pattern, target, push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("grep", fmt.Sprintf("denied by permissions: %q in %s", action.Pattern, target))
				continue
			}
			matches, grepErr := e.fs.Grep(target, action.Pattern, 50)
			if grepErr != nil {
				push(e.newEvent(EventToolError, grepErr.Error(), "Grep", ""))
				ctxManager.Add("grep", "failed: "+grepErr.Error())
				continue
			}
			msg := fmt.Sprintf("%q %s", action.Pattern, target)
			push(e.newEvent(EventToolGrep, msg, "Grep", fmt.Sprintf("%d matches", len(matches))))
			ctxManager.Add("grep", fmt.Sprintf("%s\n%s", msg, truncate(strings.Join(matches, "\n"), 2000)))
			_ = e.writeTrace("tool_grep", map[string]any{"path": target, "pattern": action.Pattern, "matches": len(matches)})

		case "edit":
			allowed, permErr := e.checkPermission(ctx, "edit", action.Path, action.Path, push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("edit", "denied by permissions: "+strings.TrimSpace(action.Path))
				continue
			}
			diff, editErr := e.fs.Edit(action.Path, action.OldText, action.NewText)
			if editErr != nil {
				push(e.newEvent(EventToolError, editErr.Error(), "Edit", ""))
				ctxManager.Add("edit", "failed: "+editErr.Error())
				continue
			}
			push(e.newEvent(EventToolEdit, action.Path+"\n\n"+diff, "Edit", ""))
			ctxManager.Add("edit", fmt.Sprintf("%s updated\n%s", action.Path, truncate(diff, 1200)))
			_ = e.writeTrace("tool_edit", map[string]any{"path": action.Path})

		case "write":
			allowed, permErr := e.checkPermission(ctx, "write", action.Path, action.Path, push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("write", "denied by permissions: "+strings.TrimSpace(action.Path))
				continue
			}
			written, writeErr := e.fs.Write(action.Path, action.Content)
			if writeErr != nil {
				push(e.newEvent(EventToolError, writeErr.Error(), "Write", ""))
				ctxManager.Add("write", "failed: "+writeErr.Error())
				continue
			}
			msg := fmt.Sprintf("%s\n\n+ wrote %d bytes", action.Path, written)
			push(e.newEvent(EventToolWrite, msg, "Write", ""))
			ctxManager.Add("write", fmt.Sprintf("%s wrote %d bytes", action.Path, written))
			_ = e.writeTrace("tool_write", map[string]any{"path": action.Path, "bytes": written})

		case "shell":
			allowed, permErr := e.checkPermission(ctx, "shell", action.Command, "", push)
			if permErr != nil {
				return events, permErr
			}
			if !allowed {
				ctxManager.Add("shell", "denied by permissions: "+strings.TrimSpace(action.Command))
				continue
			}
			if repeats := shellGuard.Observe(action.Command); repeats > e.maxRepeatedShell {
				warn := fmt.Sprintf("detected repeated shell command %q (%d times). choose different action or return final.", action.Command, repeats)
				push(e.newEvent(EventToolError, warn, "Planner", ""))
				ctxManager.Add("guard", warn)
				if repeats > e.maxRepeatedShell+1 {
					push(e.newEvent(EventReply, "检测到重复命令循环，已自动停止。请细化任务或指定分析路径。", "", ""))
					return events, nil
				}
				continue
			}

			push(e.newEvent(EventCmdStarted, action.Command, "Shell", ""))

			output, exitCode, runErr := e.shell.Run(ctx, action.Command)
			for _, line := range splitLines(output, e.maxOutputLines) {
				push(e.newEvent(EventCmdOutput, line, "Shell", ""))
			}
			if runErr != nil {
				if errors.Is(runErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					push(e.newEvent(EventReply, "任务已暂停。", "", ""))
					push(e.newEvent(EventCmdFinish, "", "Shell", ""))
					return events, context.Canceled
				}
				errMsg := fmt.Sprintf("command failed (exit=%d): %v", exitCode, runErr)
				push(e.newEvent(EventToolError, errMsg, "Shell", ""))
				ctxManager.Add("shell", fmt.Sprintf("cmd: %s\nresult: %s\noutput:\n%s", action.Command, errMsg, truncate(output, 1200)))
			} else {
				push(e.newEvent(EventCmdOutput, fmt.Sprintf("exit status %d", exitCode), "Shell", ""))
				ctxManager.Add("shell", fmt.Sprintf("cmd: %s\nresult: exit status %d\noutput:\n%s", action.Command, exitCode, truncate(output, 1200)))
			}
			push(e.newEvent(EventCmdFinish, "", "Shell", ""))
			_ = e.writeTrace("tool_shell", map[string]any{"command": action.Command, "exit_code": exitCode})

		default:
			finalMsg := strings.TrimSpace(raw)
			if finalMsg == "" {
				finalMsg = "Planner returned unsupported action."
			}
			push(e.newEvent(EventReply, finalMsg, "", ""))
			return events, nil
		}
	}

	msg := "Stopped after max steps without a final answer."
	push(e.newEvent(EventReply, msg, "", ""))
	return events, nil
}

func (e *Engine) planNext(
	ctx context.Context,
	client domain.ModelClient,
	task Task,
	contextText string,
	step int,
	maxSteps int,
) (plannerAction, *domain.Usage, string, error) {
	stepInfo := fmt.Sprintf("%d", step)
	if maxSteps > 0 {
		stepInfo = fmt.Sprintf("%d of %d", step, maxSteps)
	}

	userPrompt := fmt.Sprintf(
		"Task:\n%s\n\nModel:\nprovider=%s\nname=%s\n\nStep:\n%s\n\nContext:\n%s",
		task.Description,
		task.Model.Provider,
		task.Model.Name,
		stepInfo,
		contextText,
	)

	resp, err := client.Generate(ctx, domain.GenerateRequest{
		Model:        task.Model.Name,
		SystemPrompt: plannerSystemPrompt,
		Input:        userPrompt,
		Temperature:  0.1,
		MaxTokens:    800,
	})
	if err != nil {
		return plannerAction{}, nil, "", err
	}

	raw := strings.TrimSpace(resp.Text)
	action, parseErr := parsePlannerAction(raw)
	if parseErr != nil {
		return plannerAction{
			Action: "final",
			Final:  raw,
		}, &resp.Usage, raw, nil
	}
	return action, &resp.Usage, raw, nil
}

func (e *Engine) checkPermission(ctx context.Context, tool, action, path string, push func(Event)) (bool, error) {
	if e.permission == nil {
		return true, nil
	}
	var pending *ApprovalRequest

	for {
		decision, req, err := e.permission.Request(tool, action, path)
		if err != nil {
			push(e.newEvent(EventToolError, err.Error(), title(tool), ""))
			return false, err
		}

		switch decision {
		case DecisionAllow:
			if pending != nil {
				push(Event{
					Type:           EventApprovalResolved,
					Message:        fmt.Sprintf("approval resolved: #%d approved", pending.ID),
					ApprovalID:     pending.ID,
					ApprovalTool:   pending.Tool,
					ApprovalAction: pending.Action,
					Time:           time.Now().UTC(),
				})
				_ = e.writeTrace("approval_resolved", map[string]any{
					"id":     pending.ID,
					"tool":   pending.Tool,
					"action": pending.Action,
					"result": "approved",
				})
			}
			return true, nil

		case DecisionDeny:
			msg := fmt.Sprintf("permission denied: %s %s", tool, strings.TrimSpace(action))
			push(e.newEvent(EventToolError, msg, title(tool), ""))
			if req != nil {
				push(Event{
					Type:           EventApprovalResolved,
					Message:        fmt.Sprintf("approval resolved: #%d rejected", req.ID),
					ApprovalID:     req.ID,
					ApprovalTool:   req.Tool,
					ApprovalAction: req.Action,
					Time:           time.Now().UTC(),
				})
				_ = e.writeTrace("approval_resolved", map[string]any{
					"id":     req.ID,
					"tool":   req.Tool,
					"action": req.Action,
					"result": "rejected",
				})
			}
			return false, nil

		case DecisionPending:
			if req == nil {
				return false, fmt.Errorf("permission pending without request details")
			}
			if pending == nil {
				pending = req
				msg := (&ApprovalRequiredError{Request: *req}).Error()
				push(e.newEvent(EventToolError, msg, title(tool), ""))
				push(Event{
					Type:           EventApprovalRequired,
					Message:        msg,
					ApprovalID:     req.ID,
					ApprovalTool:   req.Tool,
					ApprovalAction: req.Action,
					Time:           time.Now().UTC(),
				})
				_ = e.writeTrace("approval_required", map[string]any{
					"id":     req.ID,
					"tool":   req.Tool,
					"action": req.Action,
					"path":   req.Path,
				})
			}
			if !e.requireApprovalBlock {
				return false, nil
			}
			select {
			case <-ctx.Done():
				push(e.newEvent(EventReply, "任务已暂停。", "", ""))
				return false, ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}

		default:
			return false, fmt.Errorf("unsupported permission decision: %q", decision)
		}
	}
}

func (e *Engine) newEvent(t EventType, msg, toolName, summary string) Event {
	return Event{
		Type:     t,
		Message:  msg,
		ToolName: toolName,
		Summary:  summary,
		Time:     time.Now().UTC(),
	}
}

func parsePlannerAction(raw string) (plannerAction, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return plannerAction{}, fmt.Errorf("empty planner response")
	}

	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}

	var action plannerAction
	if err := json.Unmarshal([]byte(trimmed), &action); err != nil {
		return plannerAction{}, err
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	return action, nil
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

func splitLines(s string, max int) []string {
	if max <= 0 {
		max = 200
	}
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, min(len(raw), max))
	for _, line := range raw {
		if len(out) >= max {
			break
		}
		out = append(out, line)
	}
	if len(raw) > max {
		out = append(out, "... output truncated ...")
	}
	return out
}

func estimateCostUSD(provider string, usage domain.Usage) float64 {
	// Conservative fallback estimate when per-model pricing is not configured.
	ratePer1K := 0.01
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		ratePer1K = 0.01
	case "openai":
		ratePer1K = 0.01
	}
	return (float64(usage.TotalTokens) / 1000.0) * ratePer1K
}

type shellLoopGuard struct {
	window int
	recent []string
}

func newShellLoopGuard(limit, window int) *shellLoopGuard {
	if limit <= 0 {
		limit = 2
	}
	if window <= 0 {
		window = 6
	}
	return &shellLoopGuard{
		window: window,
		recent: make([]string, 0, window),
	}
}

func (g *shellLoopGuard) Observe(command string) int {
	cmd := normalizeCommand(command)
	if cmd == "" {
		return 0
	}

	g.recent = append(g.recent, cmd)
	if len(g.recent) > g.window {
		g.recent = append([]string{}, g.recent[len(g.recent)-g.window:]...)
	}

	count := 0
	for _, c := range g.recent {
		if c == cmd {
			count++
		}
	}
	return count
}

func normalizeCommand(command string) string {
	s := strings.TrimSpace(strings.ToLower(command))
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (e *Engine) writeTrace(eventType string, payload any) error {
	if e.trace == nil {
		return nil
	}
	return e.trace.Write(eventType, payload)
}

func (e *Engine) runDirectReply(ctx context.Context, client domain.ModelClient, task Task) ([]Event, error) {
	resp, err := client.Generate(ctx, domain.GenerateRequest{
		Model:        task.Model.Name,
		SystemPrompt: assistantSystemPrompt,
		Input:        task.Description,
		Temperature:  0.2,
		MaxTokens:    200,
	})
	if err != nil {
		ev := e.newEvent(EventToolError, err.Error(), "Model", "")
		return []Event{ev}, err
	}

	events := []Event{
		{
			Type:       EventTokenUsage,
			CtxUsed:    resp.Usage.PromptTokens,
			TokensUsed: resp.Usage.TotalTokens,
			Time:       time.Now().UTC(),
		},
		e.newEvent(EventReply, strings.TrimSpace(resp.Text), "", ""),
	}
	_ = e.writeTrace("direct_reply", map[string]any{
		"input": task.Description,
	})
	return events, nil
}

func shouldDirectReply(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.Trim(s, "!?.,;:()[]{}\"'")
	if s == "" {
		return false
	}
	greetings := map[string]struct{}{
		"hi": {}, "hello": {}, "hey": {}, "yo": {}, "hola": {},
		"你好": {}, "您好": {}, "嗨": {},
	}
	_, ok := greetings[s]
	return ok
}

func isDirectoryErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "is a directory")
}
