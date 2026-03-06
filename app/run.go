package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/tools/events"
	permEngine "github.com/vigo999/ms-cli/tools/permission"
	"github.com/vigo999/ms-cli/ui"
	"github.com/vigo999/ms-cli/ui/model"
)

// Run starts the TUI. In demo mode it feeds fake events; in real mode it
// bridges user input to the engine.
func (a *Application) Run() error {
	if closer, ok := a.traceWriter.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	if a.Demo {
		return a.runDemo()
	}
	return a.runReal()
}

// runReal starts the TUI and a goroutine that reads user input from the
// channel, dispatches to the engine, and sends resulting events back.
func (a *Application) runReal() error {
	userCh := make(chan string, 8)
	tui := ui.New(a.EventCh, userCh, Version, a.WorkDir, a.RepoURL, a.Config.Model.Model, a.Config.Context.MaxTokens)
	// Mouse wheel scrolling is enabled by default.
	// Use /mouse off to disable if needed.
	p := tea.NewProgram(tui, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Set up permission event bus for TUI
	eventBus := events.NewEventBus()
	if engine, ok := a.permEngine.(*permEngine.DefaultEngine); ok {
		engine.SetEventBus(eventBus)
	}

	// Subscribe to permission request events
	eventBus.Subscribe(events.PermissionRequestedTopic, func(evt events.Event) {
		permReq, ok := evt.Data().(*permEngine.PermissionRequestedEvent)
		if !ok {
			return
		}

		// Send permission request to TUI
		respCh := make(chan model.PermissionResponse, 1)
		a.EventCh <- model.Event{
			Type:             model.PermissionRequest,
			PermissionTool:   permReq.ToolID,
			PermissionAction: permReq.Permission,
			PermissionPath:   strings.Join(permReq.Patterns, ", "),
			PermissionRespCh: respCh,
		}

		// Wait for response
		go func() {
			resp := <-respCh
			action := permEngine.ActionDeny
			if resp.Granted {
				action = permEngine.ActionAllow
			}
			if engine, ok := a.permEngine.(*permEngine.DefaultEngine); ok {
				engine.HandlePermissionResponse(permReq.RequestID, permEngine.PermissionResponse{
					RequestID:   permReq.RequestID,
					Action:      action,
					AlwaysAllow: resp.Remember,
					Timestamp:   time.Now().Unix(),
				})
			}
		}()
	})

	go a.inputLoop(userCh)

	_, err := p.Run()
	close(userCh)
	return err
}

// inputLoop reads user input submitted via the TUI and routes it to the
// engine or slash-command handler.
func (a *Application) inputLoop(userCh <-chan string) {
	for input := range userCh {
		a.processInput(input)
	}
}

// processInput handles a single user input string: either a slash command
// or a free-form task sent to the engine.
func (a *Application) processInput(input string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return
	}

	// Slash commands
	if strings.HasPrefix(trimmed, "/") {
		a.handleCommand(trimmed)
		return
	}

	// Free-form: send to engine in a goroutine
	go a.runTask(trimmed)
}

// runTask runs a task through the engine and sends events to UI.
func (a *Application) runTask(description string) {
	// Send thinking event
	a.EventCh <- model.Event{Type: model.AgentThinking}

	task := loop.Task{
		ID:          generateTaskID(),
		Description: description,
	}

	events, err := a.Engine.Run(task)
	if err != nil {
		// Provide user-friendly error message
		errMsg := err.Error()
		if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline") {
			errMsg = fmt.Sprintf("%s\n\nTip: The request timed out. This can happen with long conversations. Try:\n  1. Run /compact to reduce context size\n  2. Start a new conversation with /clear\n  3. Increase timeout in config (model.timeout_sec)", errMsg)
		}
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "Engine",
			Message:  errMsg,
		}
		return
	}

	// Convert loop events to UI events
	for _, ev := range events {
		uiEvent := a.convertEvent(ev)
		if uiEvent != nil {
			a.EventCh <- *uiEvent
		}
	}
}

// convertEvent converts a loop event to a UI event.
func (a *Application) convertEvent(ev loop.Event) *model.Event {
	switch ev.Type {
	case loop.EventAgentReply:
		return &model.Event{
			Type:       model.AgentReply,
			Message:    ev.Message,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventAgentThinking:
		return &model.Event{
			Type:       model.AgentThinking,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolRead:
		return &model.Event{
			Type:       model.ToolRead,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			Summary:    ev.Summary,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolGrep:
		return &model.Event{
			Type:       model.ToolGrep,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			Summary:    ev.Summary,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolGlob:
		return &model.Event{
			Type:       model.ToolGlob,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			Summary:    ev.Summary,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolEdit:
		return &model.Event{
			Type:       model.ToolEdit,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolWrite:
		return &model.Event{
			Type:       model.ToolWrite,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventToolError:
		return &model.Event{
			Type:       model.ToolError,
			Message:    ev.Message,
			ToolName:   ev.ToolName,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventCmdStarted:
		return &model.Event{
			Type:       model.CmdStarted,
			Message:    ev.Message,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventAnalysisReady:
		return &model.Event{
			Type:       model.AnalysisReady,
			Message:    ev.Message,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventTokenUpdate:
		return &model.Event{
			Type:       model.TokenUpdate,
			CtxUsed:    ev.CtxUsed,
			CtxMax:     ev.CtxMax,
			TokensUsed: ev.TokensUsed,
		}

	case loop.EventTaskCompleted:
		// Task completed, no specific UI event needed
		return nil

	case loop.EventTaskFailed:
		return &model.Event{
			Type:     model.ToolError,
			ToolName: "Task",
			Message:  ev.Message,
		}

	default:
		// Map other events to AgentReply
		if ev.Message != "" {
			return &model.Event{
				Type:    model.AgentReply,
				Message: ev.Message,
			}
		}
		return nil
	}
}

// generateTaskID generates a unique task ID.
func generateTaskID() string {
	return time.Now().Format("20060102-150405-000")
}

// runDemo starts the TUI with fake events for preview/testing.
func (a *Application) runDemo() error {
	go a.fakeAgentLoop()

	tui := ui.New(a.EventCh, nil, Version, a.WorkDir, a.RepoURL, "demo-model", a.Config.Context.MaxTokens)
	// Mouse support disabled to allow free text selection.
	p := tea.NewProgram(tui, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// fakeAgentLoop simulates agent events for preview.
func (a *Application) fakeAgentLoop() {
	send := func(e model.Event) {
		a.EventCh <- e
	}
	sleep := func(ms int) {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	sleep(500)

	// ==========================================
	// Task 1: check accuracy on qwen
	// ==========================================
	send(model.Event{Type: model.TokenUpdate, CtxUsed: 2400, TokensUsed: 1200})
	sleep(300)

	send(model.Event{Type: model.AgentThinking})
	sleep(1500)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "I'll check the accuracy on the fine-tuned Qwen 7B model. Let me find the eval config first.",
	})
	sleep(600)

	// COLLAPSED: Glob — agent searching for files
	send(model.Event{Type: model.ToolGlob, Message: "configs/**/*.yaml", Summary: "3 files"})
	sleep(400)

	// COLLAPSED: Read — agent reading config
	send(model.Event{Type: model.ToolRead, Message: "configs/eval.yaml", Summary: "28 lines"})
	sleep(400)

	// COLLAPSED: Grep — agent searching for patterns
	send(model.Event{Type: model.ToolGrep, Message: "\"qwen\" configs/", Summary: "5 matches"})
	sleep(400)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 8600, TokensUsed: 3400})
	sleep(300)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Found the eval config. Running the benchmark now.",
	})
	sleep(600)

	// EXPANDED: Shell — user-facing command with full output
	send(model.Event{
		Type:    model.CmdStarted,
		Message: "python eval.py --model qwen-7b-ft --dataset mmlu",
	})
	sleep(1500)
	send(model.Event{Type: model.CmdOutput, Message: "Loading model qwen-7b-ft..."})
	sleep(800)
	send(model.Event{Type: model.CmdOutput, Message: "Running evaluation on MMLU (14042 samples)..."})
	sleep(1200)
	send(model.Event{Type: model.CmdOutput, Message: "accuracy: 0.847"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "f1_score: 0.839"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "exit status 0"})
	sleep(300)
	send(model.Event{Type: model.CmdFinished})
	sleep(500)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 24000, TokensUsed: 12400})
	sleep(300)

	send(model.Event{Type: model.AgentThinking})
	sleep(1500)

	send(model.Event{
		Type:    model.AnalysisReady,
		Message: "Accuracy is 84.7% on MMLU, 2.3% above baseline. F1 is 83.9%. The fine-tuned model looks good.",
	})
	sleep(1000)

	// ==========================================
	// Task 2: fix OOM in training loop
	// ==========================================
	send(model.Event{Type: model.AgentThinking})
	sleep(1800)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Now I'll investigate the OOM issue in the training loop.",
	})
	sleep(600)

	// COLLAPSED: Grep — searching for allocation patterns
	send(model.Event{Type: model.ToolGrep, Message: "\"make\\(\\[\\]float\" model/", Summary: "12 matches"})
	sleep(400)

	// COLLAPSED: Read — reading source file
	send(model.Event{Type: model.ToolRead, Message: "model/layer3.go", Summary: "42 lines"})
	sleep(400)

	// COLLAPSED: Read — reading another file
	send(model.Event{Type: model.ToolRead, Message: "model/layer1.go", Summary: "38 lines"})
	sleep(300)

	// COLLAPSED: Glob — finding test files
	send(model.Event{Type: model.ToolGlob, Message: "model/**/*_test.go", Summary: "7 files"})
	sleep(300)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 38000, TokensUsed: 22000})
	sleep(300)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Found the leak in model/layer3.go line 4. Every forward pass allocates a new []float32 buffer that never gets reused. I'll profile to confirm.",
	})
	sleep(800)

	// EXPANDED: Shell — profiler command
	send(model.Event{
		Type:    model.CmdStarted,
		Message: "go tool pprof -alloc_space mem.prof",
	})
	sleep(1000)
	send(model.Event{Type: model.CmdOutput, Message: "Showing top 10 nodes"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "flat   flat%   cum    cum%"})
	sleep(300)
	send(model.Event{Type: model.CmdOutput, Message: "512MB  40.0%   512MB  40.0%  allocTensor"})
	sleep(300)
	send(model.Event{Type: model.CmdOutput, Message: "256MB  20.0%   768MB  60.0%  forwardPass"})
	sleep(300)
	send(model.Event{Type: model.CmdOutput, Message: "128MB  10.0%   896MB  70.0%  backprop"})
	sleep(300)
	send(model.Event{Type: model.CmdFinished})
	sleep(500)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 48000, TokensUsed: 28600})

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Confirmed: allocTensor leaks 512MB per step. Applying sync.Pool fix.",
	})
	sleep(800)

	// EXPANDED: Edit — diff with +/- coloring
	send(model.Event{
		Type:    model.ToolEdit,
		Message: "model/layer3.go\n\n-    buf := make([]float32, l.size)\n+    buf := l.pool.Get(l.size)\n+    defer l.pool.Put(buf)",
	})
	sleep(800)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Patched. Running memory test to verify.",
	})
	sleep(600)

	// EXPANDED: Shell — test verification
	send(model.Event{
		Type:    model.CmdStarted,
		Message: "go test -run TestLayer3Memory -memprofile mem2.prof ./model/",
	})
	sleep(1000)
	send(model.Event{Type: model.CmdOutput, Message: "=== RUN   TestLayer3Memory"})
	sleep(600)
	send(model.Event{Type: model.CmdOutput, Message: "    layer3_test.go:42: alloc before: 512MB"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "    layer3_test.go:43: alloc after:  12MB"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "--- PASS: TestLayer3Memory (2.34s)"})
	sleep(300)
	send(model.Event{Type: model.CmdOutput, Message: "PASS"})
	sleep(200)
	send(model.Event{Type: model.CmdOutput, Message: "ok  \tmscli/model\t2.345s"})
	sleep(300)
	send(model.Event{Type: model.CmdFinished})
	sleep(500)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 64000, TokensUsed: 40000})
	sleep(300)

	send(model.Event{
		Type:    model.AnalysisReady,
		Message: "Memory dropped from 512MB to 12MB per step (97.6% reduction). Fix verified.",
	})
	sleep(1000)

	// ==========================================
	// Task 3: benchmark inference — with an error
	// ==========================================
	send(model.Event{Type: model.AgentThinking})
	sleep(1800)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Starting inference benchmark across batch sizes.",
	})
	sleep(600)

	// COLLAPSED: Read — reading bench config
	send(model.Event{Type: model.ToolRead, Message: "bench/config.yaml", Summary: "15 lines"})
	sleep(300)

	// ERROR: first attempt fails
	send(model.Event{
		Type:     model.ToolError,
		ToolName: "Shell",
		Message:  "$ python bench/inference_bench.py --model qwen-7b-ft\n\nTraceback (most recent call last):\n  File \"bench/inference_bench.py\", line 23, in <module>\n    model = load_model(args.model)\n  File \"bench/loader.py\", line 45, in load_model\n    raise RuntimeError(\"CUDA out of memory\")\nRuntimeError: CUDA out of memory. Tried to allocate 2.00 GiB",
	})
	sleep(1000)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "CUDA OOM on full load. I'll reduce batch size and use --half precision to fit in memory.",
	})
	sleep(800)

	// EXPANDED: Shell — retry with fix, succeeds
	send(model.Event{
		Type:    model.CmdStarted,
		Message: "python bench/inference_bench.py --model qwen-7b-ft --half --batch 1,8,32,128",
	})
	sleep(800)
	send(model.Event{Type: model.CmdOutput, Message: "Loading model in fp16..."})
	sleep(600)
	send(model.Event{Type: model.CmdOutput, Message: "Running 100 iterations per batch size..."})
	sleep(800)
	send(model.Event{Type: model.CmdOutput, Message: ""})
	send(model.Event{Type: model.CmdOutput, Message: "Batch  Latency(ms)  Throughput(tok/s)  GPU Mem(MB)"})
	send(model.Event{Type: model.CmdOutput, Message: "─────  ───────────  ─────────────────  ──────────"})
	sleep(500)
	send(model.Event{Type: model.CmdOutput, Message: "    1       12.4              80.6         2,048"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "    8       18.7             427.8         3,584"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "   32       42.1           1,520.4         8,192"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: "  128      156.3           1,637.2        16,384"})
	sleep(400)
	send(model.Event{Type: model.CmdOutput, Message: ""})
	send(model.Event{Type: model.CmdOutput, Message: "Peak throughput at batch=128: 1637.2 tok/s"})
	sleep(300)
	send(model.Event{Type: model.CmdFinished})
	sleep(500)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 88000, TokensUsed: 56000})
	sleep(300)

	// EXPANDED: Write — creating a new config file
	send(model.Event{
		Type:    model.ToolWrite,
		Message: "bench/production.yaml\n\n+ model: qwen-7b-ft\n+ precision: fp16\n+ batch_size: 32\n+ max_vram_gb: 8\n+ throughput_target: 1500",
	})
	sleep(800)

	send(model.Event{Type: model.AgentThinking})
	sleep(1500)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Throughput plateaus at batch=128 (1637 tok/s) due to memory bandwidth. Batch=32 gives the best tradeoff: 1520 tok/s at 8GB VRAM.",
	})
	sleep(800)

	send(model.Event{
		Type:    model.AgentReply,
		Message: "Created bench/production.yaml with recommended settings. Use batch_size=32 with fp16 for production.",
	})
	sleep(600)

	send(model.Event{Type: model.TokenUpdate, CtxUsed: 104000, TokensUsed: 71200})
}
