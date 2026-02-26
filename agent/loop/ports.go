package loop

import "context"

// ExecPort executes shell commands.
type ExecPort interface {
	Run(ctx context.Context, cmd string) error
}
