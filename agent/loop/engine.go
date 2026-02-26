package loop

import "context"

// Engine drives task execution and emits events.
type Engine struct{}

func (e *Engine) Run(ctx context.Context, task string) error {
	_ = ctx
	_ = task
	return nil
}
