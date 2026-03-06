package plan

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ctxmanager "github.com/vigo999/ms-cli/agent/context"
	"github.com/vigo999/ms-cli/test/mocks"
	"github.com/vigo999/ms-cli/tools"
	"github.com/vigo999/ms-cli/tools/registry"
)

func TestNewPlan(t *testing.T) {
	plan := NewPlan("Test goal")

	if plan == nil {
		t.Fatal("NewPlan returned nil")
	}

	if plan.Goal != "Test goal" {
		t.Errorf("Expected goal 'Test goal', got '%s'", plan.Goal)
	}

	if plan.ID == "" {
		t.Error("Plan ID should not be empty")
	}

	if plan.Status != PlanStatusDraft {
		t.Errorf("Expected status Draft, got %s", plan.Status)
	}

	if len(plan.Steps) != 0 {
		t.Error("New plan should have no steps")
	}
}

func TestAddStep(t *testing.T) {
	plan := NewPlan("Test")
	step := plan.AddStep("Step 1 description")

	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}

	if step.Description != "Step 1 description" {
		t.Errorf("Expected description 'Step 1 description', got '%s'", step.Description)
	}

	if step.Index != 0 {
		t.Errorf("Expected index 0, got %d", step.Index)
	}

	if step.Status != StepStatusPending {
		t.Errorf("Expected status Pending, got %s", step.Status)
	}
}

func TestAddStepWithTool(t *testing.T) {
	plan := NewPlan("Test")
	params := map[string]any{"path": "/tmp"}
	step := plan.AddStepWithTool("Read file", "read", params)

	if step.Tool != "read" {
		t.Errorf("Expected tool 'read', got '%s'", step.Tool)
	}

	if step.ToolParams["path"] != "/tmp" {
		t.Error("Tool params should be set correctly")
	}
}

func TestPlanStatusTransitions(t *testing.T) {
	plan := NewPlan("Test")

	// Start
	plan.Start()
	if plan.Status != PlanStatusRunning {
		t.Errorf("Expected status Running, got %s", plan.Status)
	}

	if plan.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Pause
	plan.Pause()
	if plan.Status != PlanStatusPaused {
		t.Errorf("Expected status Paused, got %s", plan.Status)
	}

	// Resume
	plan.Resume()
	if plan.Status != PlanStatusRunning {
		t.Errorf("Expected status Running after resume, got %s", plan.Status)
	}

	// Complete
	plan.Complete()
	if plan.Status != PlanStatusCompleted {
		t.Errorf("Expected status Completed, got %s", plan.Status)
	}

	if plan.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestPlanStepStatus(t *testing.T) {
	step := &PlanStep{
		ID:     "step_1",
		Index:  0,
		Status: StepStatusPending,
	}

	// Start
	step.Start()
	if step.Status != StepStatusRunning {
		t.Errorf("Expected status Running, got %s", step.Status)
	}

	// Complete
	step.Complete("Result")
	if step.Status != StepStatusCompleted {
		t.Errorf("Expected status Completed, got %s", step.Status)
	}

	if step.Result != "Result" {
		t.Errorf("Expected result 'Result', got '%s'", step.Result)
	}

	// Test fail
	step2 := &PlanStep{Status: StepStatusRunning}
	step2.Fail("Error message")
	if step2.Status != StepStatusFailed {
		t.Errorf("Expected status Failed, got %s", step2.Status)
	}

	// Test skip
	step3 := &PlanStep{Status: StepStatusPending}
	step3.Skip()
	if step3.Status != StepStatusSkipped {
		t.Errorf("Expected status Skipped, got %s", step3.Status)
	}
}

func TestGetProgress(t *testing.T) {
	plan := NewPlan("Test")
	plan.AddStep("Step 1")
	plan.AddStep("Step 2")
	plan.AddStep("Step 3")

	// No steps completed
	if plan.GetProgress() != 0 {
		t.Errorf("Expected 0%% progress, got %.0f%%", plan.GetProgress())
	}

	// Complete one step
	plan.Steps[0].Complete("Done")
	if plan.GetProgress() != 33.33 {
		if plan.GetProgress() < 30 || plan.GetProgress() > 35 {
			t.Errorf("Expected ~33%% progress, got %.0f%%", plan.GetProgress())
		}
	}

	// Complete all
	plan.Steps[1].Complete("Done")
	plan.Steps[2].Complete("Done")
	if plan.GetProgress() != 100 {
		t.Errorf("Expected 100%% progress, got %.0f%%", plan.GetProgress())
	}
}

func TestPlanValidate(t *testing.T) {
	// Valid plan
	validPlan := NewPlan("Valid")
	validPlan.AddStep("Step 1")
	if err := validPlan.Validate(); err != nil {
		t.Errorf("Valid plan should pass validation: %v", err)
	}

	// No goal
	noGoalPlan := NewPlan("")
	noGoalPlan.AddStep("Step 1")
	if err := noGoalPlan.Validate(); err == nil {
		t.Error("Plan without goal should fail validation")
	}

	// No steps
	noStepsPlan := NewPlan("Test")
	if err := noStepsPlan.Validate(); err == nil {
		t.Error("Plan without steps should fail validation")
	}
}

func TestToMarkdown(t *testing.T) {
	plan := NewPlan("Test Plan")
	plan.AddStep("Step 1")
	plan.AddStep("Step 2")

	md := plan.ToMarkdown()

	if !strings.Contains(md, "Test Plan") {
		t.Error("Markdown should contain plan goal")
	}

	if !strings.Contains(md, "Step 1") {
		t.Error("Markdown should contain step descriptions")
	}
}

func TestParseStepLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1. Step one", "Step one"},
		{"2) Step two", "Step two"},
		{"- Step three", "Step three"},
		{"* Step four", "Step four"},
		{"Step 1: Step five", "Step five"},
		{"Regular text", ""},
	}

	for _, test := range tests {
		result := parseStepLine(test.input)
		if result != test.expected {
			t.Errorf("parseStepLine('%s') = '%s', expected '%s'", test.input, result, test.expected)
		}
	}
}

func TestCompactStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected ctxmanager.CompactStrategy
	}{
		{"simple", ctxmanager.CompactStrategySimple},
		{"summarize", ctxmanager.CompactStrategySummarize},
		{"priority", ctxmanager.CompactStrategyPriority},
		{"hybrid", ctxmanager.CompactStrategyHybrid},
		{"unknown", ctxmanager.CompactStrategySimple},
	}

	for _, test := range tests {
		result := ctxmanager.ParseCompactStrategy(test.input)
		if result != test.expected {
			t.Errorf("ParseCompactStrategy('%s') = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestRunMode(t *testing.T) {
	tests := []struct {
		input    string
		expected RunMode
	}{
		{"standard", ModeStandard},
		{"plan", ModePlan},
		{"review", ModeReview},
		{"unknown", ModeStandard},
	}

	for _, test := range tests {
		result := ParseRunMode(test.input)
		if result != test.expected {
			t.Errorf("ParseRunMode('%s') = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestPlanner(t *testing.T) {
	mockProvider := mocks.NewMockProvider()
	mockProvider.AddResponse(`[{"description": "Step 1", "tool": "read"}]`)

	planner := NewPlanner(mockProvider, DefaultPlannerConfig())

	plan, err := planner.GeneratePlan(context.Background(), "Test goal", []string{"read", "write"})
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	if plan.Goal != "Test goal" {
		t.Error("Plan goal should match")
	}

	if len(plan.Steps) == 0 {
		t.Error("Plan should have steps")
	}
}

func TestPlanValidator(t *testing.T) {
	validator := NewPlanValidator([]string{"read", "write"})

	// Valid plan
	validPlan := NewPlan("Valid")
	validPlan.AddStepWithTool("Step 1", "read", nil)
	errors := validator.Validate(validPlan)
	if len(errors) > 0 {
		t.Errorf("Valid plan should have no errors, got %v", errors)
	}

	// Invalid tool
	invalidToolPlan := NewPlan("Invalid")
	invalidToolPlan.AddStepWithTool("Step 1", "invalid_tool", nil)
	errors = validator.Validate(invalidToolPlan)
	if len(errors) == 0 {
		t.Error("Plan with invalid tool should have errors")
	}
}

func TestComplexity(t *testing.T) {
	mockProvider := mocks.NewMockProvider()
	planner := NewPlanner(mockProvider, DefaultPlannerConfig())

	simple := NewPlan("Simple")
	simple.AddStep("Step 1")
	if planner.EstimatePlanComplexity(simple) != ComplexitySimple {
		t.Error("Single step plan should be simple")
	}

	complex := NewPlan("Complex")
	for i := 0; i < 10; i++ {
		complex.AddStep("Step")
	}
	if planner.EstimatePlanComplexity(complex) != ComplexityComplex {
		t.Error("10 step plan should be complex")
	}
}

func TestExecutionReport(t *testing.T) {
	plan := NewPlan("Test")
	plan.AddStep("Step 1")
	plan.AddStep("Step 2")
	plan.Steps[0].Complete("Done")
	plan.Steps[1].Fail("Error")

	report := &ExecutionReport{
		Plan:         plan,
		TotalSteps:   2,
		SuccessSteps: 1,
		FailedSteps:  1,
	}

	md := report.ToMarkdown()
	if !strings.Contains(md, "Execution Report") {
		t.Error("Report markdown should contain title")
	}

	jsonStr, err := report.ToJSON()
	if err != nil {
		t.Errorf("ToJSON failed: %v", err)
	}
	if jsonStr == "" {
		t.Error("JSON should not be empty")
	}
}

func TestPlanExecutor(t *testing.T) {
	reg := registry.NewRegistry()
	reg.Register(newTestTool("test_tool", func(ctx context.Context, params map[string]any) (string, error) {
		return "test result", nil
	}))

	executor := NewPlanExecutor(reg, &DefaultModeCallback{}, DefaultExecutionConfig())

	plan := NewPlan("Test")
	plan.AddStepWithTool("Step 1", "test_tool", nil)
	plan.Approve()

	err := executor.Execute(context.Background(), plan)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if plan.Status != PlanStatusCompleted {
		t.Errorf("Expected status Completed, got %s", plan.Status)
	}
}

func TestModeConfig(t *testing.T) {
	cfg := DefaultModeConfig()

	if cfg.Mode != ModeStandard {
		t.Errorf("Expected default mode Standard, got %v", cfg.Mode)
	}

	if cfg.PlanConfig.MaxSteps != 10 {
		t.Errorf("Expected MaxSteps 10, got %d", cfg.PlanConfig.MaxSteps)
	}

	if !cfg.PlanConfig.RequireApproval {
		t.Error("RequireApproval should be true by default")
	}
}

// testTool 是用于测试的简单工具实现
type testTool struct {
	name    string
	handler func(ctx context.Context, params map[string]any) (string, error)
}

// newTestTool 创建测试工具
func newTestTool(name string, handler func(ctx context.Context, params map[string]any) (string, error)) tools.ExecutableTool {
	return &testTool{
		name:    name,
		handler: handler,
	}
}

// Info 返回工具定义
func (t *testTool) Info() *tools.ToolDefinition {
	return &tools.ToolDefinition{
		Name:        t.name,
		Description: "Test tool: " + t.name,
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

// Execute 执行工具
func (t *testTool) Execute(ctx *tools.ToolContext, exec tools.ToolExecutor, args json.RawMessage) (*tools.ToolResult, error) {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	result, err := t.handler(ctx.ContextOrBackground(), params)
	if err != nil {
		return tools.NewErrorResult(tools.ErrCodeExecutionFailed, "%s", err.Error()), nil
	}

	return tools.NewTextResult(result), nil
}
