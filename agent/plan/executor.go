package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/tools/registry"
)

// PlanExecutor 计划执行器
type PlanExecutor struct {
	toolRegistry registry.Registry
	callback     ModeCallback
	permEngine   permission.Engine
	config       ExecutionConfig
}

// ExecutionConfig 执行配置
type ExecutionConfig struct {
	ContinueOnError bool
	MaxRetries      int
	TimeoutPerStep  int // seconds
}

// DefaultExecutionConfig 返回默认执行配置
func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		ContinueOnError: false,
		MaxRetries:      1,
		TimeoutPerStep:  60,
	}
}

// NewPlanExecutor 创建新执行器
func NewPlanExecutor(reg registry.Registry, callback ModeCallback, cfg ExecutionConfig) *PlanExecutor {
	if callback == nil {
		callback = &DefaultModeCallback{}
	}
	return &PlanExecutor{
		toolRegistry: reg,
		callback:     callback,
		config:       cfg,
	}
}

// SetCallback updates the callback used by the executor.
func (e *PlanExecutor) SetCallback(cb ModeCallback) {
	if cb == nil {
		e.callback = &DefaultModeCallback{}
		return
	}
	e.callback = cb
}

// SetPermissionEngine configures permission checks for tool-executing steps.
func (e *PlanExecutor) SetPermissionEngine(pe permission.Engine) {
	e.permEngine = pe
}

// Execute 执行计划
func (e *PlanExecutor) Execute(ctx context.Context, plan *Plan) error {
	if !plan.IsExecutable() {
		return fmt.Errorf("plan is not executable: %s", plan.Status)
	}

	if plan.Status == PlanStatusApproved {
		plan.Start()
	}

	// 通知开始
	if err := e.callback.OnPlanApproved(plan); err != nil {
		return fmt.Errorf("plan approval callback: %w", err)
	}

	// 执行每个步骤
	for _, step := range plan.Steps {
		// 检查上下文取消
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("execution cancelled: %w", err)
		}

		// 跳过已完成的步骤
		if step.IsCompleted() {
			continue
		}

		// 跳过已跳过的步骤
		if step.Status == StepStatusSkipped {
			continue
		}

		// 检查依赖
		if !plan.CanExecuteStep(step) {
			step.Status = StepStatusBlocked
			continue
		}

		// 执行步骤
		if err := e.executeStep(ctx, plan, step); err != nil {
			step.Fail(err.Error())
			if !e.config.ContinueOnError {
				plan.Fail()
				return fmt.Errorf("step %d failed: %w", step.Index, err)
			}
		}
	}

	// 检查是否全部完成
	allCompleted := true
	for _, step := range plan.Steps {
		if !step.IsCompleted() && step.Status != StepStatusSkipped {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		plan.Complete()
	}

	return nil
}

// executeStep 执行单个步骤
func (e *PlanExecutor) executeStep(ctx context.Context, plan *Plan, step *PlanStep) error {
	// 通知步骤开始
	if err := e.callback.OnStepStarted(step, step.Index); err != nil {
		return err
	}

	step.Start()

	var result string
	var err error

	// 如果有指定工具，使用工具执行
	if step.Tool != "" {
		result, err = e.executeTool(ctx, step.Tool, step.ToolParams)
	} else {
		// 没有指定工具，返回描述作为结果
		result = step.Description
	}

	if err != nil {
		step.Fail(err.Error())
		return err
	}

	step.Complete(result)

	// 通知步骤完成
	if cbErr := e.callback.OnStepCompleted(step, step.Index, result); cbErr != nil {
		return cbErr
	}

	return nil
}

// executeTool 执行工具
func (e *PlanExecutor) executeTool(ctx context.Context, toolName string, params map[string]any) (string, error) {
	tool, ok := e.toolRegistry.Get(toolName)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// 转换参数为 JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}

	// 创建工具上下文
	toolCtx := &tools.ToolContext{
		SessionID:   "",
		MessageID:   "",
		CallID:      "",
		ToolID:      toolName,
		AbortSignal: ctx,
	}

	// 创建简单的执行器
	toolExec := &simpleToolExecutor{}

	// 执行工具
	result, err := tool.Execute(toolCtx, toolExec, paramsJSON)
	if err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
	}

	// 提取文本结果
	var resultParts []string
	for _, part := range result.Parts {
		if part.Type == tools.PartTypeText {
			resultParts = append(resultParts, part.Content)
		}
	}

	return strings.Join(resultParts, "\n"), nil
}

// simpleToolExecutor 简单的工具执行器实现
type simpleToolExecutor struct{}

func (e *simpleToolExecutor) UpdateMetadata(meta tools.MetadataUpdate) error { return nil }
func (e *simpleToolExecutor) AskPermission(req tools.PermissionRequest) error { return nil }

// ExecuteStep 执行指定步骤
func (e *PlanExecutor) ExecuteStep(ctx context.Context, plan *Plan, stepIndex int) error {
	if stepIndex < 0 || stepIndex >= len(plan.Steps) {
		return fmt.Errorf("step index out of range: %d", stepIndex)
	}

	step := plan.Steps[stepIndex]
	return e.executeStep(ctx, plan, step)
}

// SkipStep 跳过指定步骤
func (e *PlanExecutor) SkipStep(plan *Plan, stepIndex int) error {
	if stepIndex < 0 || stepIndex >= len(plan.Steps) {
		return fmt.Errorf("step index out of range: %d", stepIndex)
	}

	step := plan.Steps[stepIndex]
	step.Skip()
	return nil
}

// Resume 恢复执行暂停的计划
func (e *PlanExecutor) Resume(ctx context.Context, plan *Plan) error {
	if plan.Status != PlanStatusPaused {
		return fmt.Errorf("plan is not paused: %s", plan.Status)
	}

	plan.Resume()
	return e.Execute(ctx, plan)
}

// StepExecutionResult 步骤执行结果
type StepExecutionResult struct {
	Step     *PlanStep
	Success  bool
	Result   string
	Error    error
	Duration int64 // milliseconds
}

// ExecutionReport 执行报告
type ExecutionReport struct {
	Plan         *Plan
	Results      []StepExecutionResult
	StartTime    int64
	EndTime      int64
	TotalSteps   int
	SuccessSteps int
	FailedSteps  int
	SkippedSteps int
}

// GenerateReport 生成执行报告
func (e *PlanExecutor) GenerateReport(plan *Plan) *ExecutionReport {
	report := &ExecutionReport{
		Plan:       plan,
		Results:    make([]StepExecutionResult, 0, len(plan.Steps)),
		TotalSteps: len(plan.Steps),
	}

	for _, step := range plan.Steps {
		result := StepExecutionResult{
			Step:    step,
			Success: step.IsCompleted(),
			Result:  step.Result,
		}

		if step.Error != "" {
			result.Error = fmt.Errorf("%s", step.Error)
		}

		report.Results = append(report.Results, result)

		switch step.Status {
		case StepStatusCompleted:
			report.SuccessSteps++
		case StepStatusFailed:
			report.FailedSteps++
		case StepStatusSkipped:
			report.SkippedSteps++
		}
	}

	return report
}

// ReportToJSON 将报告转换为 JSON
func (r *ExecutionReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReportToMarkdown 将报告转换为 Markdown
func (r *ExecutionReport) ToMarkdown() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Execution Report\n\n")
	fmt.Fprintf(&sb, "**Plan:** %s\n\n", r.Plan.Goal)
	fmt.Fprintf(&sb, "**Status:** %s\n\n", r.Plan.Status)

	fmt.Fprintf(&sb, "## Summary\n\n")
	fmt.Fprintf(&sb, "- Total Steps: %d\n", r.TotalSteps)
	fmt.Fprintf(&sb, "- Successful: %d\n", r.SuccessSteps)
	fmt.Fprintf(&sb, "- Failed: %d\n", r.FailedSteps)
	fmt.Fprintf(&sb, "- Skipped: %d\n\n", r.SkippedSteps)

	fmt.Fprintf(&sb, "## Steps\n\n")
	for _, result := range r.Results {
		status := "⏳"
		if result.Success {
			status = "✅"
		} else if result.Step.Status == StepStatusFailed {
			status = "❌"
		} else if result.Step.Status == StepStatusSkipped {
			status = "⏭️"
		}

		fmt.Fprintf(&sb, "%s **Step %d:** %s\n", status, result.Step.Index+1, result.Step.Description)

		if result.Result != "" {
			fmt.Fprintf(&sb, "   - Result: %s\n", result.Result)
		}

		if result.Error != nil {
			fmt.Fprintf(&sb, "   - Error: %s\n", result.Error.Error())
		}

		fmt.Fprintf(&sb, "\n")
	}

	return sb.String()
}
