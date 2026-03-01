package main

import "mscli/ui/model"

// Application is the top-level composition container.
type Application struct {
	EventCh chan model.Event
}
