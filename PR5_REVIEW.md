## Detailed Code Review: PR #5 - Tool System Update

### Overall Assessment

This is a **substantial architectural refactoring** (60 files, +6,818/-3,070 lines) that modernizes the tool system from simple stubs into a production-ready, extensible framework. The direction is sound, but there are several issues that should be addressed before merging.

---

### Architecture (Positive)

1. **Interface-based Registry** (`registry.Registry` interface replacing `*tools.Registry` pointer) — Good move. This enables proper mocking in tests and decouples consumers from implementation details.

2. **Structured ToolResult with Parts** — The multi-part result model (`PartTypeText`, `PartTypeJSON`, `PartTypeBinary`, `PartTypeArtifact`) is well-designed and future-proof for multi-modal tool outputs.

3. **Permission Engine with Rule Caching** — Moving from the simple `PermissionService` interface to `permission.Engine` with rule evaluation, wildcard matching, and caching is a major improvement.

4. **Event Bus Pattern** — The `tools/events/bus.go` with both sync (`DefaultEventBus`) and async (`AsyncEventBus`) implementations provides good flexibility.

---

### Issues & Concerns

#### 1. **Breaking Change Magnitude — No Migration Path**
- This PR removes the entire old tool system (`tools/fs/`, `tools/shell/`, `tools/registry.go`, old `permission.PermissionService`) in one shot.
- The `SetExecutorRun()` pattern and `executor` variable in the old `engine.go` are completely eliminated.
- **Suggestion**: Consider if this could be split into 2-3 PRs (interfaces first, then implementation, then removal of old code) to reduce review burden and merge risk.

#### 2. **`generateID()` in `invocation.go` — Weak ID Generation**
```go
func generateID() string {
    return fmt.Sprintf("inv_%d_%d", time.Now().UnixNano(), rand.Intn(10000))
}
```
- `rand.Intn(10000)` with default seed is **not collision-safe** in concurrent scenarios. Two goroutines calling this at the same nanosecond could generate the same ID.
- **Fix**: Use `crypto/rand` or `uuid` package, or at minimum `rand.Int63()` with a wider range.

#### 3. **`ToolContext.AbortSignal` is `context.Context` but Tagged `json:"-"`**
```go
type ToolContext struct {
    AbortSignal context.Context `json:"-"`
    // ...
}
```
- The `ContextOrBackground()` helper is good, but the naming `AbortSignal` is unconventional in Go. Standard Go convention would be to either embed the context or name it `Ctx` / `Context`.
- **Minor**: Consider renaming for Go idiom consistency.

#### 4. **Permission Engine — Potential Deadlock Risk**
The `DefaultEngine.Ask()` method acquires a write lock (`e.mu.Lock()`), stores a pending request, then publishes an event and waits on a channel. If the event handler (which may respond to the permission request) also needs to call methods on the engine that acquire the lock, this could deadlock.
- **Suggestion**: Review the lock scope. Consider using the read lock for the wait phase or switching to a lock-free channel-based design for pending requests.

#### 5. **AsyncEventBus — Silent Event Drop**
```go
// Non-blocking publish that returns "event bus is full" when capacity exceeded
```
- When the async event bus buffer is full, events are silently dropped with just an error return. For permission-related events (`permission:requested`), this could mean a tool execution hangs forever waiting for a response that was never delivered.
- **Fix**: At minimum, log a warning. Better: block with timeout for critical topics or increase buffer size dynamically.

#### 6. **`simpleToolExecutor` in `plan/executor.go` — No-Op Permission**
```go
type simpleToolExecutor struct{}
func (e *simpleToolExecutor) AskPermission(req tools.PermissionRequest) error { return nil }
```
- This bypasses all permission checks during plan execution, which is a security concern. Plans executing destructive tools (bash, write, edit) would never get permission-gated.
- **Suggestion**: Wire the real `permEngine` into plan execution. The `SetPermissionEngine()` method exists but `executeTool()` doesn't use it.

#### 7. **`buildPermissionConfig()` in `bootstrap.go` — Hardcoded Dangerous Tool List**
```go
// Dangerous tool warnings (write, edit, shell, bash)
```
- Hardcoding which tools are "dangerous" is fragile. The `ToolMeta.Cost` field (`CostLevelCritical`) and `Permission.RequireConfirm` already provide this metadata.
- **Suggestion**: Use `ToolMeta.Cost >= CostLevelHigh` or `Permission.RequireConfirm` to determine danger level dynamically.

#### 8. **`extractResultContent()` in `engine.go` — Only Handles Text Parts**
Based on the diff, `extractResultContent()` iterates `result.Parts` but only extracts `PartTypeText` content. JSON parts are silently ignored, meaning tool results with structured data would appear empty in traces/events.
- **Fix**: Handle `PartTypeJSON` by marshaling to string, or at minimum include a `[JSON data]` placeholder.

#### 9. **Missing Tests for New Code**
- 60 files changed but only `engine_context_test.go` and `engine_trace_test.go` are updated (and only import path changes).
- No tests for: `ToolResult`, `ToolContext`, `EventBus`, `DefaultEngine` permission evaluation, `DefaultRegistry`, `DefaultResolver`, or `PlanExecutor.executeTool()`.
- **This is the biggest concern.** The permission system and event bus are critical infrastructure — they must have unit tests before merging.

#### 10. **Chinese Comments Throughout**
- All comments are in Chinese (e.g., `// 工具成本等级`, `// 权限标识`). This is fine for the team, but should be a conscious decision — mixing Chinese comments with English API names/variable names can be inconsistent.
- **Suggestion**: Consider standardizing on one language for code comments, or at minimum ensure exported type/method docs are in English for `godoc` compatibility.

#### 11. **`ReportToMarkdown()` Uses Emoji**
```go
status := "⏳"  // ✅ ❌ ⏭️
```
- Emoji in programmatic output can cause issues with terminals that don't support Unicode or with log parsing tools.
- **Suggestion**: Use text labels (`[PASS]`, `[FAIL]`, `[SKIP]`) as default, emoji as optional.

---

### Summary

| Area | Rating | Notes |
|------|--------|-------|
| Architecture | ✅ Good | Clean interface-based design |
| Code Quality | ⚠️ Needs Work | ID generation, lock safety, silent drops |
| Security | ⚠️ Concern | Plan executor bypasses permissions |
| Test Coverage | ❌ Insufficient | Nearly zero tests for new code |
| Documentation | ⚠️ Mixed | Chinese/English inconsistency |
| PR Size | ⚠️ Too Large | Consider splitting into smaller PRs |

**Recommendation**: Request changes — primarily need tests and the permission bypass fix before merging.
