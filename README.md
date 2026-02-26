<<<<<<< HEAD
# mscode-cli

mindspore agent CLI (build for AI infra)

## Repository Structure

```text
mscode/
├── go.mod                               # Go module definition
├── README.md                            # Project overview and structure guide
├── mscodecli-demo.mp4                   # Demo recording
│
├── app/                                 # Bootstrap/wiring/lifecycle entry layer
│   ├── main.go                          # Process entrypoint
│   ├── bootstrap.go                     # Build top-level dependencies
│   ├── wire.go                          # Bind interfaces to implementations
│   └── run.go                           # Start and shutdown flow
│
├── configs/                             # Runtime configuration files
│   ├── mscodecli.yaml                   # Model, budget, UI, permission, memory knobs
│   ├── executor.yaml                    # Execution backend and limits
│   └── skills.yaml                      # Skills repository and workflow config
│
├── agent/                               # Brain: agent control and reasoning state
│   ├── loop/                            # Core task loop and event contracts
│   │   ├── engine.go                    # Main loop driver (task -> events)
│   │   ├── types.go                     # Shared task/result/event types
│   │   ├── ports.go                     # Interfaces to executor/tools/integrations
│   │   └── permission.go                # Permission gate contract
│   ├── context/                         # Context assembly and budgeting
│   │   ├── manager.go                   # Context pipeline entry
│   │   ├── budget.go                    # Token/context budget model
│   │   └── compact.go                   # Context compaction behavior
│   └── memory/                          # Memory retrieval and retention policy
│       ├── store.go                     # Memory storage interface
│       ├── retrieve.go                  # Retrieval result definitions
│       └── policy.go                    # Retention/size policies
│
├── executor/                            # Hands: command execution backends
│   ├── local/
│   │   └── runner.go                    # Local command execution adapter
│   ├── docker/
│   │   └── runner.go                    # Docker execution adapter
│   └── common/
│       ├── types.go                     # Shared execution request/result types
│       └── stream.go                    # Stream chunk/output types
│
├── tools/                               # Capabilities exposed to the agent
│   ├── shell/
│   │   └── shell.go                     # Shell tool wrapper
│   └── fs/
│       └── fs.go                        # File read/write/patch tool wrapper
│
├── ui/                                  # Face: terminal UI
│   ├── app.go                           # Main TUI app model
│   ├── state.go                         # Central UI state
│   ├── events.go                        # UI event types
│   ├── panels/
│   │   ├── task.go                      # Task and current-step panel
│   │   ├── exec.go                      # Live execution output panel
│   │   └── analysis.go                  # Analysis and next-action panel
│   └── components/
│       ├── spinner.go                   # Spinner widget
│       └── viewport.go                  # Scrollable output widget
│
├── integrations/                        # External service adapters
│   ├── domain/
│   │   ├── client.go                    # Domain /analyze client contract
│   │   └── schema.go                    # Diagnosis schemas
│   └── skills/
│       ├── repo.go                      # Skills repo sync contract
│       └── invoke.go                    # Skills workflow invoke contract
│
├── trace/
│   └── writer.go                        # Structured runtime trace writer
│
├── report/
│   └── summary.go                       # Markdown report model/generator
│
└── bench/
    └── terminalbench2/                  # Benchmark assets and scripts
        ├── README.md
        ├── cases/
        │   ├── basic.yaml
        │   └── medium.yaml
        ├── runner/
        │   ├── run.sh
        │   └── parse.sh
        └── results/
            └── .gitkeep
```

## Architecture Model

- `agent/`: decides what to do.
- `executor/`: runs commands/jobs.
- `tools/`: capability wrappers for agent calls.
- `ui/`: renders state and events.
- `integrations/`: external domain/skills adapters.
- `app/`: wires everything together.


Rule: UI listens to events; agent loop emits events; executor/tools do not depend on UI.
=======
# mscli
>>>>>>> 1ced41a9136fbbad4458e89e96a865d8328e5539
