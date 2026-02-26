package local

import "context"

// Runner executes commands and streams output.
type Runner interface {
	Run(ctx context.Context, cmd string) error
}
