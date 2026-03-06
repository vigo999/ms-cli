package loop

import (
	"sync"
	"testing"

	"github.com/vigo999/ms-cli/test/mocks"
	"github.com/vigo999/ms-cli/tools/registry"
)

type traceRecord struct {
	eventType string
	payload   any
}

type captureTraceWriter struct {
	mu      sync.Mutex
	records []traceRecord
}

func (w *captureTraceWriter) Write(eventType string, payload any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, traceRecord{
		eventType: eventType,
		payload:   payload,
	})
	return nil
}

func (w *captureTraceWriter) count(eventType string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	total := 0
	for _, rec := range w.records {
		if rec.eventType == eventType {
			total++
		}
	}
	return total
}

func (w *captureTraceWriter) hasEvent(eventType string) bool {
	return w.count(eventType) > 0
}

func (w *captureTraceWriter) hasLoopEvent(loopType string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, rec := range w.records {
		if rec.eventType != "event" {
			continue
		}
		ev, ok := rec.payload.(Event)
		if ok && ev.Type == loopType {
			return true
		}
	}
	return false
}

func TestEngineRunWritesTrajectory(t *testing.T) {
	provider := mocks.NewMockProvider()
	provider.AddResponse("done")

	reg := registry.NewRegistry()
	engine := NewEngine(EngineConfig{
		MaxIterations: 3,
		MaxTokens:     8000,
	}, provider, reg)

	traceWriter := &captureTraceWriter{}
	engine.SetTraceWriter(traceWriter)

	_, err := engine.Run(Task{
		ID:          "task_1",
		Description: "say hello",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !traceWriter.hasEvent("run_started") {
		t.Fatalf("expected run_started trace event")
	}
	if !traceWriter.hasEvent("run_finished") {
		t.Fatalf("expected run_finished trace event")
	}
	if !traceWriter.hasEvent("llm_request") {
		t.Fatalf("expected llm_request trace event")
	}
	if !traceWriter.hasEvent("llm_response") {
		t.Fatalf("expected llm_response trace event")
	}
	if !traceWriter.hasEvent("event") {
		t.Fatalf("expected generic loop event trace")
	}
	if !traceWriter.hasLoopEvent(EventTaskStarted) {
		t.Fatalf("expected loop TaskStarted event in trace")
	}
	if !traceWriter.hasLoopEvent(EventAgentReply) {
		t.Fatalf("expected loop AgentReply event in trace")
	}
	if !traceWriter.hasLoopEvent(EventTaskCompleted) {
		t.Fatalf("expected loop TaskCompleted event in trace")
	}
}
