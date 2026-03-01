package main

import "mscli/ui/model"

// Bootstrap wires top-level dependencies.
func Bootstrap() (*Application, error) {
	ch := make(chan model.Event, 16)
	return &Application{EventCh: ch}, nil
}
