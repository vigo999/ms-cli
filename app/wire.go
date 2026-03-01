package main

import (
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/ui/model"
)

const Version = "ms-cli v0.1.0"

// Application is the top-level composition container.
type Application struct {
	Engine  *loop.Engine
	EventCh chan model.Event
	Demo    bool
	WorkDir string
	RepoURL string
}
