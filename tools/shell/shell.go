package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/tools"
)

// ShellTool wraps shell execution as a Tool.
type ShellTool struct {
	runner *Runner
	schema tools.Schema
}

// NewShellTool creates a new shell tool.
func NewShellTool(runner *Runner) *ShellTool {
	return &ShellTool{
		runner: runner,
		schema: tools.NewSchema().
			String("command", "The shell command to execute (e.g., 'go test ./...', 'git status')").
			Int("timeout", "Timeout in seconds (default: 60, max: 1800)").
			Required("command").
			Build(),
	}
}

// Name returns the tool name.
func (t *ShellTool) Name() string {
	return "shell"
}

// Description returns the tool description.
func (t *ShellTool) Description() string {
	return "Execute a shell command. Use this for running tests, building, git operations, etc. Commands have a timeout and destructive operations may require confirmation."
}

// Schema returns the tool parameter schema.
func (t *ShellTool) Schema() llm.ToolSchema {
	return tools.ToLLMToolSchema(t.schema)
}

// Validate validates the parameters against the schema.
func (t *ShellTool) Validate(params json.RawMessage) []tools.ValidationError {
	return tools.ValidateAgainstSchema(params, t.schema)
}

type shellParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// Execute executes the shell tool.
func (t *ShellTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	// Validate parameters first
	if errs := t.Validate(params); len(errs) > 0 {
		return tools.ErrorResultf("validation failed: %v", errs), nil
	}

	var p shellParams
	if err := tools.ParseParams(params, &p); err != nil {
		return tools.ErrorResult(err), nil
	}

	command := strings.TrimSpace(p.Command)
	if command == "" {
		return tools.ErrorResultf("command is required"), nil
	}

	// Apply custom timeout if specified
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeoutFromInt(p.Timeout))
		defer cancel()
	}

	// Run command
	result, err := t.runner.Run(ctx, command)
	if err != nil {
		return tools.ErrorResultf("execute command: %w", err), nil
	}

	// Build output with command for UI display
	var parts []string
	// Include command at the beginning for clarity
	parts = append(parts, fmt.Sprintf("$ %s", command))

	if result.Stdout != "" {
		parts = append(parts, result.Stdout)
	}

	if result.Stderr != "" {
		parts = append(parts, fmt.Sprintf("[stderr]\n%s", result.Stderr))
	}

	output := strings.Join(parts, "\n")

	// Summary
	summary := fmt.Sprintf("exit %d", result.ExitCode)
	if result.Error != nil {
		summary = fmt.Sprintf("error: %s", result.Error.Error())
	}

	return tools.StringResultWithSummary(output, summary), nil
}

func timeoutFromInt(seconds int) time.Duration {
	if seconds < 1 {
		return 60 * time.Second
	}
	if seconds > 1800 {
		return 1800 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
