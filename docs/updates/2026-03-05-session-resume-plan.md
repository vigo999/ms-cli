# Session Resume + Trace 合并更新（2026-03-05）

## 目标
- 支持严格模式恢复：`ms-cli resume <session-id>`
- 支持会话发现：`ms-cli sessions list`
- 将运行期 session 接入主链路，实现消息实时落盘
- 将 trace writer 迁入 `agent/session`，并移除旧 `trace/writer.go`
- 恢复时包含：上下文消息、UI 历史、模型快照、权限快照

## 已落地内容

### 1. 顶层 CLI 子命令
- 新增 `resume` 子命令：
  - `ms-cli resume <session-id>`
  - 缺失 session-id 时返回错误并退出非 0。
- 新增 `sessions list` 子命令：
  - `ms-cli sessions list`
  - 输出字段：`ID UpdatedAt Messages Archived WorkDir`
  - 按 `UpdatedAt desc` 展示（由 session store 排序）。
  - `ID` 格式：`sess_YYMMDD-HHMMSS`，同秒冲突自动追加后缀（如 `-2`）。

### 2. 会话接入主链路
- `Bootstrap` 固定初始化 `<workdir>/.mscli/sessions` 的 `FileStore + Manager`。
- 非 `resume` 启动时自动创建新 session 并设为当前。
- `resume` 启动时加载指定 session 并设为当前。
- `Engine` 增加 `SetMessageSink`，在 user/assistant/tool 消息写入 context 后立即回调 session 持久化（实时落盘）。

### 3. 恢复范围
- `context.Manager` 新增 `ReplaceMessages([]llm.Message)`，恢复时直接灌入历史消息，不走逐条压缩。
- `ui.New(...)` 支持注入初始消息，`resume` 启动会预渲染历史聊天记录。
- `Session` 新增 `RuntimeSnapshot`：
  - `ModelSnapshot`: `URL/Model/Temperature/TimeoutSec/MaxTokens`
  - `PermissionSnapshot`: tool/command/path 三类策略
  - `TracePath`
- `resume` 时恢复模型与权限快照，再应用 CLI flags（flags 最高优先级）。

### 4. Trace 合并
- 新增 `agent/session/event_writer.go`：
  - `NewEventWriter(path)`
  - `NewSessionTraceWriter(storePath, sessionID)`
  - `TracePathForSession(storePath, sessionID)`
- 运行期 trace 固定写入：
  - `<workdir>/.mscli/sessions/<session-id>.trajectory.jsonl`
  - 同一 session 多次 resume 持续 append。
- 旧 `trace/writer.go` 已移除，`agent/loop` 通过最小 `TraceWriter` 接口直接接收会话 writer。

### 5. 命令行为对齐
- `/permission`、`/yolo` 修改策略后同步更新 session runtime snapshot。
- `/model` 走 `SetProvider` 后同步更新 snapshot（含重建 engine 后 hooks 保持）。
- `/clear` 同时清空：
  - UI 聊天显示
  - `context` 消息
  - 当前 session 消息持久化
- 退出时会提示恢复命令：`ms-cli resume <session-id>`（包括 `/exit`）。

## 新增/更新测试
- `app/cli_test.go`
  - 覆盖 `run/resume/sessions list` 解析、`resume` 缺参、未知命令、sessions list 输出排序。
- `app/bootstrap_test.go`
  - 覆盖 `resume` 恢复模型快照、权限快照、context 消息与 UI 初始消息。
  - 覆盖 `/clear` 同时清空 context 与 session。
- `agent/context/manager_test.go`
  - 新增 `ReplaceMessages` 行为测试。
- `agent/loop/engine_message_sink_test.go`
  - 验证 sink 可收到 user/assistant/tool 消息。
- `agent/session/runtime_compat_test.go`
  - 验证旧 session JSON（无 runtime 字段）可加载并升级使用。
  - 验证同 session trace 文件 append 连续写入。
- `agent/session/event_writer_test.go`
  - 覆盖 session trace writer 路径与写盘行为。

## 验证结果
- 已执行：`go test ./...`
- 结果：全部通过。
