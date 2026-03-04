# ms-cli

MindSpore CLI — an AI infrastructure agent with a terminal UI.

## Prerequisites

- Go 1.24.2+ (see `go.mod`)

## Quick Start

Build:

```bash
go build -o ms-cli ./app
```

Run demo mode:

```bash
go run ./app --demo
# or
./ms-cli --demo
```

Run real mode:

```bash
go run ./app
# or
./ms-cli
```

## Commands

In TUI input, use slash commands:

- `/roadmap status [path]` (default: `roadmap.yaml`)
- `/weekly status [path]` (default: `weekly.md`)
- `/model list`
- `/model show`
- `/model use <provider>/<model>` (provider: `openai` or `openrouter`)
- `/perm status`
- `/perm yolo on|off`
- `/perm whitelist list|add|remove <tool>`
- `/perm blacklist list|add|remove <tool>`
- `/approve once|session`
- `/reject`
- `/compact [keep]` (compact chat history, default keep=12)
- `/clear` (clear chat panel)
- `/exit` (exit TUI)

Any non-slash input is treated as a normal task prompt and routed to the engine.

When input starts with `/`, the hint bar shows slash-command candidates. Use `up/down` to cycle and `tab` to apply the selected command.

## Model Provider Setup

`ms-cli` supports OpenAI and OpenRouter.

- Set API key env vars as needed:
  - `OPENAI_API_KEY`
  - `OPENROUTER_API_KEY`
- Optional OpenAI base URL override:
  - `OPENAI_BASE_URL`
- Optional runtime overrides:
  - `MSCLI_MODEL_PROVIDER`
  - `MSCLI_MODEL_NAME`
  - `MSCLI_MODEL_ENDPOINT`
- Base config lives in `configs/mscli.yaml` (supports `providers.openai.base_url`).
- Session state is persisted in `.mscli/session.yaml` (model selection and provider API keys).

## Keybindings

| Key | Action |
|-----|--------|
| `enter` | Send input |
| `pgup` / `pgdn` | Scroll chat |
| `up` / `down` | Scroll chat |
| `home` / `end` | Jump to top / bottom |
| `/` | Start a slash command |
| `ctrl+c` | Quit |

## Project Status Data

Roadmap status engine:

- `internal/project/roadmap.go`
- Parses roadmap YAML, validates schema, and computes phase + overall progress.

Weekly update parser (Markdown + YAML front matter):

- `internal/project/weekly.go`
- Template: `docs/updates/WEEKLY_TEMPLATE.md`

Public roadmap page:

- `docs/roadmap/ROADMAP.md`

Project reports:

- `docs/updates/` (see latest `*-report.md`)

## Repository Structure

```text
ms-cli/
├── app/                        # entry point + wiring
│   ├── main.go
│   ├── bootstrap.go
│   ├── wire.go
│   ├── run.go
│   └── commands.go
├── agent/
│   ├── loop/                   # engine, task/event types, permissions
│   ├── context/                # budget, compaction, context manager
│   └── memory/                 # policy, store, retrieve
├── executor/
│   └── runner.go               # pluggable task executor
├── integrations/
│   ├── domain/                 # external domain client + schema
│   └── skills/                 # skill invocation + repo
├── internal/
│   └── project/
│       ├── roadmap.go
│       └── weekly.go
├── tools/
│   ├── fs/                     # filesystem operations
│   └── shell/                  # shell command runner
├── trace/
│   └── writer.go               # execution trace logging
├── report/
│   └── summary.go              # report generation
├── ui/
│   ├── app.go                  # root Bubble Tea model
│   ├── model/model.go          # shared state types
│   ├── components/             # spinner, textinput, viewport
│   └── panels/                 # topbar, chat, hintbar
├── docs/
│   ├── roadmap/ROADMAP.md
│   └── updates/
├── go.mod
└── README.md
```

## Known Limitations

- Planner currently depends on model-generated JSON; malformed model outputs can degrade task quality.
- No provider-level automatic failover (OpenAI/OpenRouter switch is manual via `/model use`).
- Running Bubble Tea in non-interactive shells may fail with `/dev/tty` errors.

## Architecture Rule

UI listens to events; agent loop emits events; executor/tools do not depend on UI.
