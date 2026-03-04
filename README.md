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

### Command-Line Options

```bash
# Select URL and model
./ms-cli --url https://api.openai.com/v1 --model gpt-4o

# Use custom config file
./ms-cli --config /path/to/config.yaml

# Set API key directly
./ms-cli --api-key sk-xxx
```

## Commands

In TUI input, use slash commands:

### Project Commands
- `/roadmap status [path]` (default: `roadmap.yaml`)
- `/weekly status [path]` (default: `weekly.md`)

### Model Commands
- `/model` - Show current model configuration
- `/model <model-name>` - Switch to a new model
- `/model <openai:model>` - Backward-compatible provider prefix format (e.g., `/model openai:gpt-4o-mini`)

### Session Commands
- `/compact` - Compact conversation context to save tokens
- `/clear` - Clear chat history
- `/mouse [on|off|toggle|status]` - Control mouse wheel scrolling
- `/exit` - Exit the application
- `/help` - Show available commands

Any non-slash input is treated as a normal task prompt and routed to the engine.

### Slash Command Autocomplete

Type `/` to see available slash commands. Use `↑`/`↓` keys to navigate and `Tab` or `Enter` to select.

## Keybindings

| Key | Action |
|-----|--------|
| `enter` | Send input |
| `mouse wheel` | Scroll chat |
| `pgup` / `pgdn` | Scroll chat |
| `up` / `down` | Scroll chat / Navigate slash suggestions |
| `home` / `end` | Jump to top / bottom |
| `tab` / `enter` | Accept slash suggestion |
| `esc` | Cancel slash suggestions |
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

## Configuration

Configuration can be provided via:

1. **Config file** (`mscli.yaml` or `~/.config/mscli/config.yaml`)
2. **Environment variables**
3. **Command-line flags** (highest priority)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `MSCLI_BASE_URL` | OpenAI-compatible API base URL (higher priority) |
| `MSCLI_MODEL` | Model name |
| `MSCLI_API_KEY` | API key (higher priority) |
| `OPENAI_BASE_URL` | API base URL (fallback) |
| `OPENAI_MODEL` | Model name (fallback) |
| `OPENAI_API_KEY` | API key (fallback) |

### Example Config File

```yaml
model:
  url: https://api.openai.com/v1
  model: gpt-4o-mini
  key: ""
  temperature: 0.7
budget:
  max_tokens: 32768
  max_cost_usd: 10
context:
  max_tokens: 24000
  compaction_threshold: 0.85
```

## Known Limitations

- The real-mode engine flow is still minimal/stub-oriented.
- Running Bubble Tea in non-interactive shells may fail with `/dev/tty` errors.

## Architecture Rule

UI listens to events; agent loop emits events; executor/tools do not depend on UI.
