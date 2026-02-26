package common

import "context"

// ExecRequest describes a command execution request.
type ExecRequest struct {
	Command string
	WorkDir string
	Env     map[string]string
}

// ExecResult describes execution outcome.
type ExecResult struct {
	ExitCode int
	Error    string
}

// Runner executes commands.
type Runner interface {
	Run(ctx context.Context, req ExecRequest) (ExecResult, error)
}
