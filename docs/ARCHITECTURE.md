# ms-cli Architecture

## 1. Goals and Scope

This document describes only the real architecture implementation and runtime flow in the current `ms-cli` codebase. It does not contain version planning or roadmap goals.

Scope constraints:

1. Only covers behavior in the current mainline code (`app/`, `agent/`, `tools/`, etc.).
2. Does not define future-version commitments; evolution plans are maintained in roadmap documents.
3. All architecture conclusions are code-first. This document helps implementers quickly locate module relationships and extension points.

## 2. Layering and Module Responsibilities

`ms-cli` can currently be understood with the following layers:

1. `app`: CLI/TUI entry and runtime assembly. Parses commands (`run/resume/sessions list`), initializes dependencies, and bridges UI and Agent.
2. `agent`: Core intelligent execution layer.
3. `tools`: Executable tool layer (read/write files, search, shell), called by Agent through a unified schema.
4. `integrations`: External model integration layer (OpenAI/Anthropic protocols).
5. `permission`: Tool-call authorization layer (tool-level, command-level, path-level policies).
6. `ui`: Bubble Tea terminal UI layer, consumes UI events mapped from `loop.Event`.
7. `configs`: Configuration loading and precedence merge layer (config file, environment variables, CLI args).

Additional notes:

1. The `executor` directory is kept only for compatibility; real execution is handled by `agent/loop.Engine`.
2. `session` is integrated into the runtime path; `memory` is still not integrated into the main execution path.
3. TUI adds a manual subagent command `/subagent`, triggered in `app/commands.go` and calling `loop.Engine.RunSubagent(...)`.

## 3. Runtime Bootstrap

Entry files:

1. `app/main.go`
2. `app/cli.go`
3. `app/bootstrap.go`
4. `app/wire.go`

### Demo Path (`--demo`)

1. Initialize config and `session.Manager` in `app/bootstrap.go`.
2. Create `loop.Engine` (stub provider).
3. Inject into `Application`, then `runDemo()` in `app/run.go` drives a simulated event stream.

### Real Path (default)

`app/bootstrap.go` handles key dependency injection:

1. Provider: `initProvider` creates OpenAI/Anthropic clients by protocol.
2. Tool registry: `initTools` registers `read/write/edit/grep/glob/shell`.
3. Context manager: creates `context.Manager` for short-term context.
4. Session manager: creates `session.Manager` for persistent sessions.
5. Permission service: `permission.NewDefaultPermissionService`.
6. Trajectory writer: `session.NewSessionTraceWriter`, writes JSONL to a fixed per-session path.
7. Engine: `loop.NewEngine` creates the core executor.

Then `attachEngineHooks` in `app/wire.go` injects:

1. `Engine.SetContextManager(...)`
2. `Engine.SetPermissionService(...)`
3. `Engine.SetTraceWriter(...)`
4. `Engine.SetMessageSink(...)` (persist session messages to `session.Manager` in real time)

## 4. Core Execution Sequence (Run)

Core step flow (Real mode):

1. User enters a task in TUI (`app/run.go`).
2. `Application.runTask` calls `Engine.Run(task)`.
3. `loop.Engine` writes user messages into `context.Manager` (short-term context).
4. `context.Manager.GetMessages()` provides the current message window to provider.
5. Provider generates a response.
6. If response includes tool calls, `loop.Engine` first requests authorization via `permission.PermissionService`.
7. After approval, the corresponding tool is executed in `tools.Registry`, and results are fed back into `context.Manager`.
8. Key events per round are written to session trajectory (`run_started`, `llm_request`, `tool_result`, `run_finished`, etc.).
9. At the same time, `MessageSink` persists user/assistant/tool messages to the current session.
10. `loop.Event` is returned to `app/run.go`, mapped to UI events, and rendered.

Simplified flow:

`user input -> loop.Engine -> context -> provider -> tools -> permission -> session (trajectory) -> UI`

### 4.1 Manual Subagent Delegation Sequence (`/subagent`)

Command entry: `/subagent [--allow-write] [--allow-shell] <task>`

Execution flow:

1. `app/commands.go` parses args and applies a single-flight limit (only one subagent at a time).
2. Calls `loop.Engine.RunSubagent(req)` to create an isolated child executor (independent context, does not reuse the main session message window).
3. The child executor reuses the current provider and permission service.
4. Child executor filters tools by allowlist.
5. Default tools: `read/grep/glob`.
6. `--allow-write` additionally enables `write/edit`.
7. `--allow-shell` additionally enables `shell`.
8. `subagent` is always excluded (prevents recursion).
9. Child run traces are written to current session trajectory via `subagent_*` events (including `run_id`).
10. When execution finishes, the main session writes summary message `[subagent:<run_id>] <summary>` into both `context.Manager` and `session.Manager`.

## 5. Session Resume Sequence (Resume)

Command entry: `ms-cli resume <session-id>`

Resume flow:

1. `app/cli.go` parses `resume` and writes `BootstrapConfig.ResumeSessionID`.
2. `app/bootstrap.go` loads session JSON via `sessionManager.Load(id)`.
3. Apply runtime snapshot.
4. `applyModelSnapshot(...)` restores protocol, model, URL, and related model config.
5. `applyPermissionSnapshot(...)` restores tool/command/path permission policies.
6. `ctxManager.ReplaceMessages(currentSession.Messages)` restores context messages.
7. `sessionMessagesToUI(...)` converts history messages into initial UI messages.
8. `session.NewSessionTraceWriter(sessionStorePath, currentSession.ID)` reuses the same session trajectory file.
9. `sessionManager.SetCurrentTracePath(...)` and `syncSessionRuntime()` write back current runtime state.
10. After entering `runReal()`, subsequent messages continue through the same context/session persistence path (including trajectory).

## 6. Agent Subsystem Responsibilities

`agent` currently has five subsystems: `loop/context/plan/session/memory`.

1. `agent/loop`: Main execution engine. Handles ReAct loop, tool calls, plan mode, manual subagent execution, event emission, and trajectory writes.
2. `agent/context`: Short-term context. Handles message storage, token estimation, budget, and compaction policies.
3. `agent/plan`: Plan generation and execution, including `Planner` (generation) and `PlanExecutor` (execution).
4. `agent/session`: Session persistence and resume. Handles session JSON, runtime snapshots, and session trajectory writes.
5. `agent/memory`: Long-term memory module (policy, retrieval, SQLite storage). Not integrated into the main runtime path yet.

Key relationships:

1. `loop -> context`: every round depends on context.
2. `loop -> plan`: schedules plan subsystem in Plan/Review mode.
3. `app -> session`: integrates session layer through message sink and resume mechanism.
4. `memory` is currently decoupled from runtime path and used only in tests.

## 7. State and Persistence

Current state layers:

1. Short-term state (dialog window): `context.Manager` maintains messages, budget, and compaction stats in memory.
2. Long-term session state: `session.Manager` persists full messages and runtime snapshots.
3. Session trajectory state: `session.EventWriter` outputs per-session JSONL traces of runtime events.

On-disk format:

1. Session JSON: `.mscli/sessions/<session-id>.json`
2. Trace JSONL: `.mscli/sessions/<session-id>.trajectory.jsonl`

Boundary principles:

1. `context` primarily serves current reasoning window and token budget control.
2. `session` provides cross-process and cross-restart recovery.
3. Trajectory is for observability and is not a direct reasoning-context source.
4. Detailed `/subagent` process is kept in trajectory (`subagent_run_started/subagent_event/subagent_run_finished`), while main session keeps only summary messages.

## 8. Extension Points and Evolution Interfaces

Primary entry points for new/extended capabilities:

1. Add a provider.
2. Implement `llm.Provider` under `integrations/llm/<provider>/`.
3. Add protocol branch and config mapping in `app/bootstrap.go:initProvider`.
4. Add a tool.
5. Implement `tools.Tool` interface (`tools/types.go`).
6. Register it in `app/bootstrap.go:initTools` into `tools.Registry`.
7. Add a run mode.
8. Extend `RunMode` in `agent/plan/mode.go`, and add branch logic in `agent/loop/engine.go`.
9. Add a permission policy.
10. Extend policy checks and snapshot sync logic in `permission.DefaultPermissionService`.
11. Integrate memory (future).
12. `memory.Manager` can be integrated in `app/wire.go` or `loop.Engine` message lifecycle (save/retrieval policies must be explicitly defined).
13. Extend subagent.
14. Add concurrency scheduling, independent model config, or finer-grained tool policy on `agent/loop.Engine.RunSubagent`.

## 9. Current Limitations and Known Risks

The following risks are summarized from current code and `docs/agent-review.md`:

1. The budget-threshold branch comparison in `context.Manager.shouldCompactLocked` has logic issues and may cause unstable auto-compaction triggers.
2. In `plan.GeneratePlanPrompt(goal, tools)`, the `tools` argument is not actually used for prompt construction; tool list remains hardcoded.
3. `MaxRetries` and `TimeoutPerStep` in `plan.ExecutionConfig` are not fully enforced in the execution path.
4. `memory.Query.Metadata` filtering is not implemented in `SQLiteStore.Query`.
5. In `memory.Retriever`, policy fields are weakly applied; retrieval strategy still mainly relies on heuristic scoring.
6. The `/compact` command in `app/commands.go` is still a placeholder and does not directly call `context.Manager.Compact()`.

## 10. Related Document Index

1. Project overview: [`README.md`](../README.md)
2. Agent code review: [`docs/agent-review.md`](./agent-review.md)
3. Roadmap docs: [`docs/roadmap/ROADMAP.md`](./roadmap/ROADMAP.md)

Note: evolution plans and phased goals are maintained in roadmap documents. This file only maintains architecture facts of the current implementation.
