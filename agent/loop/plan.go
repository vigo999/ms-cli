package loop

import (
	"fmt"
	"strings"
	"time"
)

// PlanStatus 计划状态
type PlanStatus string

const (
	// PlanStatusDraft 草稿状态
	PlanStatusDraft PlanStatus = "draft"
	// PlanStatusPendingApproval 等待批准
	PlanStatusPendingApproval PlanStatus = "pending_approval"
	// PlanStatusApproved 已批准
	PlanStatusApproved PlanStatus = "approved"
	// PlanStatusRunning 执行中
	PlanStatusRunning PlanStatus = "running"
	// PlanStatusPaused 暂停
	PlanStatusPaused PlanStatus = "paused"
	// PlanStatusCompleted 已完成
	PlanStatusCompleted PlanStatus = "completed"
	// PlanStatusFailed 失败
	PlanStatusFailed PlanStatus = "failed"
	// PlanStatusCancelled 已取消
	PlanStatusCancelled PlanStatus = "cancelled"
)

// StepStatus 步骤状态
type StepStatus string

const (
	// StepStatusPending 等待执行
	StepStatusPending StepStatus = "pending"
	// StepStatusRunning 执行中
	StepStatusRunning StepStatus = "running"
	// StepStatusCompleted 已完成
	StepStatusCompleted StepStatus = "completed"
	// StepStatusFailed 失败
	StepStatusFailed StepStatus = "failed"
	// StepStatusSkipped 已跳过
	StepStatusSkipped StepStatus = "skipped"
	// StepStatusBlocked 被阻塞
	StepStatusBlocked StepStatus = "blocked"
)

// Plan 执行计划
type Plan struct {
	ID          string
	Goal        string
	Description string
	Steps       []*PlanStep
	Status      PlanStatus
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Metadata    map[string]any
}

// PlanStep 计划步骤
type PlanStep struct {
	ID          string
	Index       int
	Description string
	Tool        string          // 可选：指定工具
	ToolParams  map[string]any  // 工具参数
	DependsOn   []string        // 依赖的其他步骤 ID
	Status      StepStatus
	Result      string
	Error       string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Metadata    map[string]any
}

// NewPlan 创建新计划
func NewPlan(goal string) *Plan {
	return &Plan{
		ID:        generatePlanID(),
		Goal:      goal,
		Steps:     make([]*PlanStep, 0),
		Status:    PlanStatusDraft,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// AddStep 添加步骤
func (p *Plan) AddStep(description string) *PlanStep {
	step := &PlanStep{
		ID:          generateStepID(p.ID, len(p.Steps)),
		Index:       len(p.Steps),
		Description: description,
		Status:      StepStatusPending,
		Metadata:    make(map[string]any),
	}
	p.Steps = append(p.Steps, step)
	return step
}

// AddStepWithTool 添加带工具的步骤
func (p *Plan) AddStepWithTool(description, tool string, params map[string]any) *PlanStep {
	step := p.AddStep(description)
	step.Tool = tool
	step.ToolParams = params
	return step
}

// GetStep 根据 ID 获取步骤
func (p *Plan) GetStep(id string) *PlanStep {
	for _, step := range p.Steps {
		if step.ID == id {
			return step
		}
	}
	return nil
}

// GetStepByIndex 根据索引获取步骤
func (p *Plan) GetStepByIndex(index int) *PlanStep {
	if index < 0 || index >= len(p.Steps) {
		return nil
	}
	return p.Steps[index]
}

// Start 开始执行计划
func (p *Plan) Start() {
	p.Status = PlanStatusRunning
	now := time.Now()
	p.StartedAt = &now
}

// Pause 暂停计划
func (p *Plan) Pause() {
	p.Status = PlanStatusPaused
}

// Resume 恢复计划
func (p *Plan) Resume() {
	if p.Status == PlanStatusPaused {
		p.Status = PlanStatusRunning
	}
}

// Complete 完成计划
func (p *Plan) Complete() {
	p.Status = PlanStatusCompleted
	now := time.Now()
	p.CompletedAt = &now
}

// Fail 标记计划失败
func (p *Plan) Fail() {
	p.Status = PlanStatusFailed
	now := time.Now()
	p.CompletedAt = &now
}

// Cancel 取消计划
func (p *Plan) Cancel() {
	p.Status = PlanStatusCancelled
	now := time.Now()
	p.CompletedAt = &now
}

// Approve 批准计划
func (p *Plan) Approve() {
	p.Status = PlanStatusApproved
}

// IsExecutable 检查计划是否可执行
func (p *Plan) IsExecutable() bool {
	return p.Status == PlanStatusApproved || p.Status == PlanStatusRunning
}

// GetCurrentStep 获取当前步骤
func (p *Plan) GetCurrentStep() *PlanStep {
	for _, step := range p.Steps {
		if step.Status == StepStatusPending || step.Status == StepStatusRunning {
			return step
		}
	}
	return nil
}

// GetCompletedSteps 获取已完成步骤数
func (p *Plan) GetCompletedSteps() int {
	count := 0
	for _, step := range p.Steps {
		if step.Status == StepStatusCompleted {
			count++
		}
	}
	return count
}

// GetProgress 获取进度百分比
func (p *Plan) GetProgress() float64 {
	if len(p.Steps) == 0 {
		return 0
	}
	completed := p.GetCompletedSteps()
	return float64(completed) / float64(len(p.Steps)) * 100
}

// CanExecuteStep 检查步骤是否可以执行
func (p *Plan) CanExecuteStep(step *PlanStep) bool {
	// 检查依赖
	for _, depID := range step.DependsOn {
		dep := p.GetStep(depID)
		if dep == nil {
			return false
		}
		if dep.Status != StepStatusCompleted {
			return false
		}
	}
	return true
}

// Validate 验证计划
func (p *Plan) Validate() error {
	if p.Goal == "" {
		return fmt.Errorf("plan goal is required")
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("plan must have at least one step")
	}

	// 检查步骤依赖是否有效
	for _, step := range p.Steps {
		for _, depID := range step.DependsOn {
			if p.GetStep(depID) == nil {
				return fmt.Errorf("step %s depends on non-existent step %s", step.ID, depID)
			}
		}
	}

	return nil
}

// String 返回计划字符串表示
func (p *Plan) String() string {
	return fmt.Sprintf("Plan[%s]: %s (%d steps, %s, %.0f%%)",
		p.ID, p.Goal, len(p.Steps), p.Status, p.GetProgress())
}

// ToMarkdown 转换为 Markdown 格式
func (p *Plan) ToMarkdown() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Plan: %s\n\n", p.Goal)
	fmt.Fprintf(&sb, "**Status:** %s\n\n", p.Status)
	fmt.Fprintf(&sb, "**Progress:** %.0f%% (%d/%d steps)\n\n",
		p.GetProgress(), p.GetCompletedSteps(), len(p.Steps))

	if p.Description != "" {
		fmt.Fprintf(&sb, "## Description\n\n%s\n\n", p.Description)
	}

	fmt.Fprintf(&sb, "## Steps\n\n")
	for _, step := range p.Steps {
		status := "⏳"
		switch step.Status {
		case StepStatusCompleted:
			status = "✅"
		case StepStatusFailed:
			status = "❌"
		case StepStatusRunning:
			status = "🔄"
		case StepStatusSkipped:
			status = "⏭️"
		case StepStatusBlocked:
			status = "🚫"
		}

		fmt.Fprintf(&sb, "%s **%d.** %s\n", status, step.Index+1, step.Description)
		if step.Tool != "" {
			fmt.Fprintf(&sb, "   - Tool: `%s`\n", step.Tool)
		}
		if step.Result != "" {
			fmt.Fprintf(&sb, "   - Result: %s\n", step.Result)
		}
		fmt.Fprintf(&sb, "\n")
	}

	return sb.String()
}

// PlanStep 方法

// Start 开始执行步骤
func (s *PlanStep) Start() {
	s.Status = StepStatusRunning
	now := time.Now()
	s.StartedAt = &now
}

// Complete 完成步骤
func (s *PlanStep) Complete(result string) {
	s.Status = StepStatusCompleted
	s.Result = result
	now := time.Now()
	s.CompletedAt = &now
}

// Fail 标记步骤失败
func (s *PlanStep) Fail(err string) {
	s.Status = StepStatusFailed
	s.Error = err
	now := time.Now()
	s.CompletedAt = &now
}

// Skip 跳过步骤
func (s *PlanStep) Skip() {
	s.Status = StepStatusSkipped
	now := time.Now()
	s.CompletedAt = &now
}

// IsPending 检查步骤是否等待执行
func (s *PlanStep) IsPending() bool {
	return s.Status == StepStatusPending
}

// IsCompleted 检查步骤是否已完成
func (s *PlanStep) IsCompleted() bool {
	return s.Status == StepStatusCompleted
}

// GeneratePlanPrompt 生成计划提示词
func GeneratePlanPrompt(goal string, tools []string) string {
	toolList := "- read: Read file contents\n"
	toolList += "- write: Write file contents\n"
	toolList += "- edit: Edit file contents\n"
	toolList += "- grep: Search file contents\n"
	toolList += "- glob: Find files matching pattern\n"
	toolList += "- shell: Execute shell commands"

	return fmt.Sprintf(`You are a planning assistant. Create a step-by-step plan for the following task.

Task: %s

Available tools:
%s

Create a plan with the following format:
1. Each step should be a clear, actionable instruction
2. Specify which tool to use for each step (if applicable)
3. Steps should be in logical order
4. Keep the plan concise (3-10 steps)

Format your response as a JSON array of steps:
[
  {"description": "Step 1 description", "tool": "tool_name"},
  {"description": "Step 2 description", "tool": "tool_name"}
]`, goal, toolList)
}

// generatePlanID 生成计划 ID
func generatePlanID() string {
	return fmt.Sprintf("plan_%d", time.Now().UnixNano())
}

// generateStepID 生成步骤 ID
func generateStepID(planID string, index int) string {
	return fmt.Sprintf("%s_step_%d", planID, index)
}
