# 2026-03-05 Anthropic Protocol + No-Key Startup Implementation

## Scope
Implemented the integration and runtime changes to support both OpenAI and Anthropic protocols, with lazy key validation so the app can boot without API keys.

## Implemented Changes

### 1) Config model protocol
- Added `model.protocol` to config schema.
- Default protocol is `openai`.
- Protocol validation accepts only `openai` and `anthropic`.

Files:
- `configs/types.go`
- `configs/mscli.yaml`

### 2) Env override precedence and auto protocol selection
Implemented protocol-aware env loading with the following behavior:
1. `MSCLI_PROTOCOL` (if valid) wins.
2. Otherwise auto-detect protocol:
   - choose `openai` when `OPENAI_BASE_URL + OPENAI_MODEL + OPENAI_API_KEY` are all present
   - else choose `anthropic` when `ANTHROPIC_BASE_URL + ANTHROPIC_MODEL + ANTHROPIC_API_KEY` are all present
   - else keep config/default protocol
3. Apply provider-specific env values by selected protocol.
4. Apply `MSCLI_BASE_URL/MSCLI_MODEL/MSCLI_API_KEY` last as highest priority.

Files:
- `configs/loader.go`
- `configs/loader_test.go`

### 3) Anthropic provider (non-streaming)
Added a new provider implementation using Anthropic Messages API:
- `Name() == "anthropic"`
- `Complete()` implemented (`POST /messages`)
- `CompleteStream()` returns explicit not-implemented error
- Tool schemas and tool calls mapped to Anthropic format
- Response `text` + `tool_use` mapped back to unified `llm.CompletionResponse`

Files:
- `integrations/llm/anthropic/client.go`
- `integrations/llm/anthropic/client_test.go`

### 4) No-key startup (lazy failure)
Boot no longer fails when API key is missing.
- Added unconfigured provider placeholder.
- On first model request, user sees clear guidance to set key via `/model key <KEY>`.

Files:
- `app/bootstrap.go`
- `app/provider_unconfigured.go`
- `app/bootstrap_test.go`

### 5) `/model` command enhancements
Extended model command behavior:
- `/model <model>`
- `/model openai:<model>`
- `/model anthropic:<model>`
- `/model key <KEY>`

Also:
- provider switching now supports `openai|anthropic`
- when switching provider and URL is empty/default, URL auto-switches to provider default endpoint
- `/model` display now includes `Protocol`
- `/test` shows explicit key-not-configured message

Files:
- `app/commands.go`
- `app/wire.go`
- `ui/slash/commands.go`
- `app/commands_model_test.go`

### 6) Key masking in UI
Input masking was added for user echo in chat history:
- `/model key ...` is displayed as `/model key ****`
- raw command still flows to command handler

Files:
- `ui/app.go`
- `ui/app_mask_test.go`

### 7) Persist protocol into state/session runtime snapshots
- State persistence now includes protocol.
- Session runtime model snapshot includes protocol and is restored on resume.

Files:
- `configs/state.go`
- `agent/session/types.go`
- `app/bootstrap.go`
- `app/wire.go`

### 8) Documentation updates
Updated user docs with protocol and env variables:
- added `MSCLI_PROTOCOL`
- added `ANTHROPIC_*` vars
- added `/model anthropic:<model>` and `/model key <KEY>` usage

Files:
- `README.md`

## Notes
- Anthropic streaming remains intentionally unimplemented in this iteration.
- API key storage behavior is unchanged (state persistence still stores key as before).
