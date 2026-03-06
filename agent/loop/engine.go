package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ctxmanager "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/agent/plan"
	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/tools/registry"
	"github.com/vigo999/ms-cli/trace"
)

// EngineConfig holds engine configuration.
type EngineConfig struct {
	MaxIterations  int
	MaxTokens      int
	Temperature    float32
	TimeoutPerTurn time.Duration
	SystemPrompt   string

	// Plan Mode 配置
	ModeConfig plan.ModeConfig
}

// Engine drives task execution and emits events.
type Engine struct {
	config     EngineConfig
	provider   llm.Provider
	tools      registry.Registry
	ctxManager *ctxmanager.Manager
	permEngine permission.Engine
	trace      trace.Writer

	// Plan Mode 组件
	planner      *plan.Planner
	planExecutor *plan.PlanExecutor
	modeCallback plan.ModeCallback
}

// NewEngine creates a new engine.
func NewEngine(cfg EngineConfig, provider llm.Provider, tools registry.Registry) *Engine {
	// MaxIterations = 0 means no limit
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = defaultSystemPrompt()
	}
	if cfg.ModeConfig.Mode == 0 && cfg.ModeConfig.PlanConfig.MaxSteps == 0 {
		cfg.ModeConfig = plan.DefaultModeConfig()
	}

	engine := &Engine{
		config:       cfg,
		provider:     provider,
		tools:        tools,
		modeCallback: &plan.DefaultModeCallback{},
	}

	// Initialize context manager if not set
	engine.ctxManager = ctxmanager.NewManager(ctxmanager.ManagerConfig{
		MaxTokens:     cfg.MaxTokens,
		ReserveTokens: 4000,
	})
	engine.ctxManager.SetSystemPrompt(cfg.SystemPrompt)

	// Initialize Plan Mode components
	engine.planner = plan.NewPlanner(provider, plan.DefaultPlannerConfig())
	engine.planExecutor = plan.NewPlanExecutor(tools, engine.modeCallback, plan.DefaultExecutionConfig())

	return engine
}

// SetContextManager sets the context manager.
func (e *Engine) SetContextManager(cm *ctxmanager.Manager) {
	if cm == nil {
		return
	}

	// Preserve system prompt when swapping context manager.
	if cm.GetSystemPrompt() == nil {
		switch {
		case e.ctxManager != nil && e.ctxManager.GetSystemPrompt() != nil:
			cm.SetSystemPrompt(e.ctxManager.GetSystemPrompt().Content)
		case e.config.SystemPrompt != "":
			cm.SetSystemPrompt(e.config.SystemPrompt)
		}
	}
	e.ctxManager = cm
}

// SetPermissionEngine sets the permission engine.
func (e *Engine) SetPermissionEngine(pe permission.Engine) {
	e.permEngine = pe
}

// SetModeCallback sets the mode callback.
func (e *Engine) SetModeCallback(cb plan.ModeCallback) {
	e.modeCallback = cb
	e.planExecutor.SetCallback(cb)
}

// SetRunMode sets the run mode.
func (e *Engine) SetRunMode(mode plan.RunMode) {
	e.config.ModeConfig.Mode = mode
}

// SetTraceWriter sets the trace writer.
func (e *Engine) SetTraceWriter(w trace.Writer) {
	e.trace = w
}

// Run executes a task and returns events.
func (e *Engine) Run(task Task) ([]Event, error) {
	ctx := context.Background()
	return e.RunWithContext(ctx, task)
}

// RunWithContext executes a task with context.
func (e *Engine) RunWithContext(ctx context.Context, task Task) ([]Event, error) {
	startedAt := time.Now()
	e.writeTrace("run_started", map[string]any{
		"task_id":      task.ID,
		"description":  task.Description,
		"mode":         e.config.ModeConfig.Mode.String(),
		"max_tokens":   e.config.MaxTokens,
		"temperature":  e.config.Temperature,
		"started_at":   startedAt,
		"max_iter":     e.config.MaxIterations,
		"timeout_turn": e.config.TimeoutPerTurn.String(),
	})

	var (
		events []Event
		err    error
	)
	switch e.config.ModeConfig.Mode {
	case plan.ModePlan:
		events, err = e.runWithPlanMode(ctx, task)
	case plan.ModeReview:
		events, err = e.runWithReviewMode(ctx, task)
	default:
		events, err = e.runStandard(ctx, task)
	}

	finishedTrace := map[string]any{
		"task_id":     task.ID,
		"description": task.Description,
		"finished_at": time.Now(),
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"event_count": len(events),
	}
	if err != nil {
		finishedTrace["error"] = err.Error()
	}
	e.writeTrace("run_finished", finishedTrace)
	return events, err
}

// runStandard 标准模式执行
func (e *Engine) runStandard(ctx context.Context, task Task) ([]Event, error) {
	exec := &executor{
		engine:    e,
		task:      task,
		events:    make([]Event, 0),
		startTime: time.Now(),
	}
	return exec.run(ctx)
}

// runWithPlanMode Plan Mode 执行
func (e *Engine) runWithPlanMode(ctx context.Context, task Task) ([]Event, error) {
	events := make([]Event, 0)
	appendEvent := func(ev Event) {
		events = append(events, ev)
		e.writeTrace("event", ev)
	}

	appendEvent(NewEvent(EventTaskStarted, fmt.Sprintf("Task (Plan Mode): %s", task.Description)))

	// 1. 生成计划
	appendEvent(NewEvent(EventAgentThinking, "Generating plan..."))
	p, err := e.planner.GeneratePlan(ctx, task.Description, e.getAvailableTools())
	if err != nil {
		appendEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Failed to generate plan: %v", err)))
		return events, fmt.Errorf("generate plan: %w", err)
	}
	e.writeTrace("plan_generated", p)

	appendEvent(NewEvent(EventLLMResponse, fmt.Sprintf("Plan created with %d steps", len(p.Steps))))

	// 2. 通知计划创建
	if err := e.modeCallback.OnPlanCreated(p); err != nil {
		appendEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Plan callback error: %v", err)))
		return events, err
	}

	// 3. 等待批准（如果需要）
	if e.config.ModeConfig.PlanConfig.RequireApproval {
		p.Status = plan.PlanStatusPendingApproval
		// 这里应该有一个阻塞调用等待用户批准
		// 简化实现：直接批准
		p.Approve()
		if err := e.modeCallback.OnPlanApproved(p); err != nil {
			appendEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Plan approval error: %v", err)))
			return events, err
		}
	} else {
		p.Approve()
	}

	// 4. 执行计划
	if err := e.planExecutor.Execute(ctx, p); err != nil {
		appendEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Plan execution failed: %v", err)))
		return events, err
	}

	// 5. 生成结果
	report := e.planExecutor.GenerateReport(p)
	e.writeTrace("plan_report", report)
	appendEvent(NewEvent(EventTaskCompleted, report.ToMarkdown()))

	return events, nil
}

// runWithReviewMode Review Mode 执行
func (e *Engine) runWithReviewMode(ctx context.Context, task Task) ([]Event, error) {
	// Review Mode: 每步执行前确认
	// 简化实现：使用 Plan Mode 但每步确认
	events := make([]Event, 0)
	appendEvent := func(ev Event) {
		events = append(events, ev)
		e.writeTrace("event", ev)
	}

	appendEvent(NewEvent(EventTaskStarted, fmt.Sprintf("Task (Review Mode): %s", task.Description)))

	// 生成单步计划
	p := plan.NewPlan(task.Description)
	p.AddStep(task.Description)
	p.Approve()
	e.writeTrace("review_plan_generated", p)

	// 执行并确认
	for _, step := range p.Steps {
		// 请求确认
		confirmed, err := e.modeCallback.OnStepNeedsConfirmation(step, step.Index)
		if err != nil {
			appendEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Confirmation error: %v", err)))
			return events, err
		}
		if !confirmed {
			step.Skip()
			e.writeTrace("review_step_skipped", step)
			continue
		}

		// 执行步骤（使用标准 ReAct 循环）
		stepEvents, err := e.runStandard(ctx, Task{Description: step.Description})
		if err != nil {
			events = append(events, stepEvents...)
			return events, err
		}
		events = append(events, stepEvents...)
		step.Complete("Executed")
		e.writeTrace("review_step_completed", step)
	}

	p.Complete()
	appendEvent(NewEvent(EventTaskCompleted, "Task completed with review"))

	return events, nil
}

func (e *Engine) writeTrace(eventType string, payload any) {
	if e.trace == nil {
		return
	}
	_ = e.trace.Write(eventType, payload)
}

// getAvailableTools 获取可用工具列表
func (e *Engine) getAvailableTools() []string {
	toolList := e.tools.List()
	names := make([]string, len(toolList))
	for i, t := range toolList {
		names[i] = t.Info().Name
	}
	return names
}

// GeneratePlan 生成计划（公开方法）
func (e *Engine) GeneratePlan(ctx context.Context, goal string) (*plan.Plan, error) {
	return e.planner.GeneratePlan(ctx, goal, e.getAvailableTools())
}

// ExecutePlan 执行计划（公开方法）
func (e *Engine) ExecutePlan(ctx context.Context, p *plan.Plan) error {
	return e.planExecutor.Execute(ctx, p)
}

// executor manages execution of a single task.
type executor struct {
	engine     *Engine
	task       Task
	events     []Event
	iterCount  int
	startTime  time.Time
	totalUsage llm.Usage
}

// run executes the ReAct loop.
func (ex *executor) run(ctx context.Context) ([]Event, error) {
	// Add initial user message
	ex.engine.ctxManager.AddMessage(llm.NewUserMessage(ex.task.Description))

	ex.engine.writeTrace("user_task", map[string]any{
		"task_id":      ex.task.ID,
		"description":  ex.task.Description,
		"received_at":  time.Now(),
		"context_size": len(ex.engine.ctxManager.GetMessages()),
	})

	ex.addEvent(NewEvent(EventTaskStarted, fmt.Sprintf("Task: %s", ex.task.Description)))
	ex.addEvent(NewEvent(EventAgentThinking, ""))

	for ex.engine.config.MaxIterations == 0 || ex.iterCount < ex.engine.config.MaxIterations {
		ex.iterCount++

		// Check context cancellation
		if err := ctx.Err(); err != nil {
			ex.addEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Context cancelled: %v", err)))
			return ex.events, err
		}

		// Get messages for LLM
		messages := ex.engine.ctxManager.GetMessages()
		tools := ex.engine.tools.ToLLMTools()

		// Call LLM with timeout - use a separate context that doesn't affect tool execution
		timeout := ex.engine.config.TimeoutPerTurn
		if timeout == 0 {
			timeout = 180 * time.Second // Default 3 minutes
		}

		llmCtx, cancel := context.WithTimeout(ctx, timeout)
		req := &llm.CompletionRequest{
			Model:       "", // Use provider default
			Messages:    messages,
			Tools:       tools,
			Temperature: ex.engine.config.Temperature,
		}
		ex.engine.writeTrace("llm_request", map[string]any{
			"iteration": ex.iterCount,
			"request":   req,
		})
		resp, err := ex.engine.provider.Complete(llmCtx, req)
		cancel()

		if err != nil {
			// Check if it's a timeout error and provide helpful message
			errMsg := fmt.Sprintf("LLM error: %v", err)
			if ctx.Err() == context.DeadlineExceeded || llmCtx.Err() == context.DeadlineExceeded {
				errMsg = fmt.Sprintf("Request timeout. The conversation may be too long (ctx: %d tokens). Try /compact to reduce context size.",
					ex.engine.ctxManager.TokenUsage().Current)
			}
			ex.addEvent(NewEvent(EventTaskFailed, errMsg))
			ex.engine.writeTrace("llm_error", map[string]any{
				"iteration": ex.iterCount,
				"error":     err.Error(),
			})
			return ex.events, fmt.Errorf("LLM completion: %w", err)
		}
		ex.engine.writeTrace("llm_response", map[string]any{
			"iteration": ex.iterCount,
			"response":  resp,
		})

		// Track usage
		ex.totalUsage.PromptTokens += resp.Usage.PromptTokens
		ex.totalUsage.CompletionTokens += resp.Usage.CompletionTokens
		ex.totalUsage.TotalTokens += resp.Usage.TotalTokens

		// Handle response - use original ctx (not the cancelled LLM ctx) for tool execution
		continueLoop, err := ex.handleResponse(ctx, resp)
		if err != nil {
			ex.addEvent(NewEvent(EventTaskFailed, fmt.Sprintf("Handle response error: %v", err)))
			return ex.events, err
		}

		if !continueLoop {
			break
		}
	}

	// Check if stopped due to max iterations (only if limit is set)
	if ex.engine.config.MaxIterations > 0 && ex.iterCount >= ex.engine.config.MaxIterations {
		ex.addEvent(NewEvent(EventTaskFailed, "Task exceeded maximum iterations. The AI may be stuck in a loop or the task is too complex. Try breaking it into smaller steps or being more specific about what you want."))
	} else {
		ex.addEvent(NewEvent(EventTaskCompleted, "Task completed successfully"))
	}

	return ex.events, nil
}

// handleResponse processes the LLM response.
func (ex *executor) handleResponse(ctx context.Context, resp *llm.CompletionResponse) (bool, error) {
	// Add assistant message to context
	assistantMsg := llm.Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}
	ex.engine.ctxManager.AddMessage(assistantMsg)

	// Handle tool calls
	if len(resp.ToolCalls) > 0 {
		// Execute tools
		for _, tc := range resp.ToolCalls {
			if err := ex.executeToolCall(ctx, tc); err != nil {
				return false, err
			}
		}
		return true, nil // Continue loop
	}

	// Final response
	if resp.Content != "" {
		ex.addEvent(NewEvent(EventAgentReply, resp.Content))
	}

	return false, nil // End loop
}

// executeToolCall executes a single tool call.
func (ex *executor) executeToolCall(ctx context.Context, tc llm.ToolCall) error {
	toolName := tc.Function.Name

	// Find tool
	tool, ok := ex.engine.tools.Get(toolName)
	if !ok {
		errMsg := fmt.Sprintf("Tool not found: %s", toolName)
		ex.addEvent(NewEvent(EventToolError, errMsg))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		return nil
	}
	ex.engine.writeTrace("tool_call", tc)

	// Add event
	ex.addEvent(NewEvent(EventToolStarted, fmt.Sprintf("Using tool: %s", toolName)))

	// Create tool context and executor
	toolCtx := &tools.ToolContext{
		SessionID:   ex.task.ID,
		MessageID:   tc.ID,
		CallID:      tc.ID,
		ToolID:      toolName,
		AbortSignal: ctx,
	}

	toolExec := &agentToolExecutor{engine: ex.engine}

	// Execute tool
	result, err := tool.Execute(toolCtx, toolExec, tc.Function.Arguments)
	if err != nil {
		errMsg := fmt.Sprintf("Tool execution error: %v", err)
		ex.addEvent(NewEvent(EventToolError, errMsg))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		ex.engine.writeTrace("tool_exec_error", map[string]any{
			"tool":    toolName,
			"call_id": tc.ID,
			"error":   err.Error(),
		})
		return nil
	}

	// Handle error result
	if result.Error != nil {
		errMsg := result.Error.Message
		ex.addEvent(NewEvent(EventToolError, fmt.Sprintf("Tool %s failed: %s", toolName, errMsg)))
		ex.engine.ctxManager.AddToolResult(tc.ID, errMsg)
		ex.engine.writeTrace("tool_result_error", map[string]any{
			"tool":    toolName,
			"call_id": tc.ID,
			"error":   errMsg,
		})
		return nil
	}

	// Extract content from result
	content := extractResultContent(result)
	ex.engine.writeTrace("tool_result", map[string]any{
		"tool":    toolName,
		"call_id": tc.ID,
		"content": content,
	})

	// Add tool event based on tool type
	ex.addToolEvent(toolName, result, content)

	// Add tool result to context
	ex.engine.ctxManager.AddToolResult(tc.ID, content)

	return nil
}

// addToolEvent adds an event based on tool type.
func (ex *executor) addToolEvent(toolName string, result *tools.ToolResult, content string) {
	eventType := EventToolStarted
	switch toolName {
	case "read":
		eventType = EventToolRead
	case "grep":
		eventType = EventToolGrep
	case "glob":
		eventType = EventToolGlob
	case "edit":
		eventType = EventToolEdit
	case "write":
		eventType = EventToolWrite
	case "bash":
		eventType = EventCmdStarted
	}

	ev := NewEvent(eventType, content)
	ev.ToolName = toolName
	ev.Summary = truncate(content, 200)
	ex.addEvent(ev)
}

// extractResultContent 从ToolResult提取文本内容
func extractResultContent(result *tools.ToolResult) string {
	var parts []string
	for _, part := range result.Parts {
		if part.Type == tools.PartTypeText {
			parts = append(parts, part.Content)
		}
	}
	return strings.Join(parts, "\n")
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// agentToolExecutor 工具执行器实现
type agentToolExecutor struct {
	engine *Engine
}

func (e *agentToolExecutor) UpdateMetadata(meta tools.MetadataUpdate) error {
	return nil
}

func (e *agentToolExecutor) AskPermission(req tools.PermissionRequest) error {
	if e.engine == nil || e.engine.permEngine == nil {
		return nil // 没有权限服务，默认允许
	}

	// Convert tools.PermissionRequest to permission.PermissionRequest
	permReq := permission.PermissionRequest{
		ID:         req.ID,
		SessionID:  req.SessionID,
		ToolID:     req.ToolID,
		CallID:     req.CallID,
		Permission: req.Permission,
		Patterns:   req.Patterns,
		Metadata:   req.Metadata,
		CheckLevel: permission.CheckLevel(req.CheckLevel),
	}

	// 调用权限引擎检查
	if err := e.engine.permEngine.Ask(permReq); err != nil {
		return fmt.Errorf("permission denied for %s: %w", req.ToolID, err)
	}
	return nil
}

// addEvent adds an event to the list.
func (ex *executor) addEvent(ev Event) {
	// Update token usage
	usage := ex.engine.ctxManager.TokenUsage()
	ev.CtxUsed = usage.Current
	ev.CtxMax = usage.Max
	ev.TokensUsed = ex.totalUsage.TotalTokens

	ex.events = append(ex.events, ev)
	ex.engine.writeTrace("event", ev)
}

// defaultSystemPrompt returns the default system prompt.
func defaultSystemPrompt() string {
	return `You are an AI assistant that helps users with software development tasks.

You have access to the following tools:
- read: Read file contents
- write: Create or overwrite files
- edit: Edit files by replacing text
- grep: Search for patterns in files
- glob: Find files matching patterns
- shell: Execute shell commands

Guidelines:
1. Use tools to gather information before making changes
2. Always read files before editing them
3. Make minimal, focused changes
4. Use grep and glob to explore the codebase
5. Run tests with shell to verify changes

IMPORTANT: When you have gathered enough information to answer the user's question, you MUST provide your final answer directly WITHOUT using any more tools. Do not keep calling tools indefinitely - provide a clear, concise response once you have the information needed.

When making edits, ensure the old_string matches exactly (including whitespace and newlines).`
}

func extractPathArg(raw json.RawMessage) string {
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path"} {
		if v, ok := params[key].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
