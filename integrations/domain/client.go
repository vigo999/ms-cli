package domain

import "context"

// Client calls external /analyze service.
type Client interface {
	Analyze(ctx context.Context, input string) (*Diagnosis, error)
}
