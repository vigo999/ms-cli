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
- `/model <openai:model>` - OpenAI protocol model (e.g., `/model openai:gpt-4o-mini`)
- `/model <anthropic:model>` - Anthropic protocol model (e.g., `/model anthropic:claude-3-5-sonnet-latest`)
- `/model key <API_KEY>` - Update API key at runtime

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
│   ├── memory/                 # policy, store, retrieve
│   └── session/                # session persistence + trajectory writer
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
| `MSCLI_PROTOCOL` | Protocol override: `openai` or `anthropic` |
| `MSCLI_BASE_URL` | API base URL (highest priority) |
| `MSCLI_MODEL` | Model name (highest priority) |
| `MSCLI_API_KEY` | API key (highest priority) |
| `OPENAI_BASE_URL` | OpenAI base URL (provider fallback) |
| `OPENAI_MODEL` | OpenAI model (provider fallback) |
| `OPENAI_API_KEY` | OpenAI API key (provider fallback) |
| `ANTHROPIC_BASE_URL` | Anthropic base URL (provider fallback) |
| `ANTHROPIC_MODEL` | Anthropic model (provider fallback) |
| `ANTHROPIC_API_KEY` | Anthropic API key (provider fallback) |

Auto protocol selection when `MSCLI_PROTOCOL` is unset:
1. If `OPENAI_BASE_URL` + `OPENAI_MODEL` + `OPENAI_API_KEY` are all set, use `openai`.
2. Else if `ANTHROPIC_BASE_URL` + `ANTHROPIC_MODEL` + `ANTHROPIC_API_KEY` are all set, use `anthropic`.
3. Else keep config/default protocol (`openai` by default).

### Example Config File

```yaml
model:
  protocol: openai
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
