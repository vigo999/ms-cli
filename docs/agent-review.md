# agent/ 代码系统评审

> 评审范围：`agent/context`、`agent/loop`、`agent/plan`、`agent/memory`、`agent/session` 全部 `.go` 文件（含测试）。
> 评审目标：逐文件职责说明、内部组件关系、外部模块如何接入 agent。

## 1. 总览

`agent/` 当前包含 5 个子系统：

1. `loop`：运行时主循环（ReAct）、工具调用、事件流、Plan/Review 模式入口。
2. `context`：会话上下文管理（消息堆栈、token 估算、预算、压缩、优先级）。
3. `plan`：计划数据模型、计划生成器、计划执行器、模式回调。
4. `memory`：长期记忆抽象（策略、检索、SQLite 存储、导入导出）。
5. `session`：会话生命周期与文件持久化。

当前真实运行路径核心是：`app -> agent/session + agent/loop (+ agent/context)`。其中 `session` 已接入主执行链路并支持 `ms-cli resume <id>` 恢复，`memory` 仍处于未接入状态。

## 2. 逐文件评审

## 2.1 `agent/context`

| 文件 | 主要职责 | 关键内容 | 与其他组件关系 |
| --- | --- | --- | --- |
| `agent/context/budget.go` | token 预算管理 | `BudgetAllocation`、`Budget`、`BudgetStats`，按 system/history/tool/reserve 分桶统计与阈值判断 | 被 `context.Manager` 持有并更新；不直接对外暴露到 app |
| `agent/context/tokenizer.go` | token 启发式估算 | `Tokenizer`（消息、文本、代码、tool call估算），`SimpleTokenizer`，全局估算函数 | 被 `Manager` 与 `Compactor` 使用 |
| `agent/context/priority.go` | 消息优先级评分 | `PriorityScorer`、关键词/位置加权、`PriorityQueue` | 被 `Compactor`（优先级压缩）与 `Manager.GetMessagePriority` 使用 |
| `agent/context/compact.go` | 上下文压缩策略 | `Compactor` + 4策略（simple/summarize/priority/hybrid），`CompactResult` | 被 `Manager.compactLocked` 调用 |
| `agent/context/manager.go` | 上下文主控制器 | `ManagerConfig`、`Manager`、`AddMessage`、`Compact`、`GetStats`、`SetCompactStrategy` | 被 `loop.Engine` 持有并作为每轮 LLM 请求上下文来源 |
| `agent/context/manager_test.go` | 测试覆盖 | 覆盖初始化、消息增删、预算、压缩、策略切换、优先级、统计 | 验证 `Manager` 对外行为契约 |

补充说明：

1. `Manager` 是本包真正入口，封装了 tokenizer/budget/compactor/scorer 四组件。
2. 压缩有自动触发（`AddMessage`）和手动触发（`Compact`）两条路径。
3. `GetMessages` 保证系统提示词在消息列表首位。

## 2.2 `agent/loop`

| 文件 | 主要职责 | 关键内容 | 与其他组件关系 |
| --- | --- | --- | --- |
| `agent/loop/types.go` | 任务/事件模型 | `Task`、`Event`、事件常量（任务、工具、UI兼容） | `app/run.go` 直接消费事件类型并映射到 UI 事件 |
| `agent/loop/engine.go` | Agent 运行引擎 | `Engine`、`RunWithContext`、标准模式 ReAct 循环、Plan 模式、Review 模式、tool 执行、权限校验、trace 写入 | 核心依赖 `context.Manager`、`plan.Planner`、`plan.PlanExecutor`、`tools.Registry`、`permission.PermissionService` |
| `agent/loop/engine_context_test.go` | 上下文替换行为测试 | 验证 `SetContextManager` 保留系统提示词，且 Run 时消息顺序正确 | 保证 context manager 热替换安全 |
| `agent/loop/engine_trace_test.go` | 轨迹追踪测试 | 验证 run/llm/event trace 关键事件均写出 | 保障 trace 可观测性 |

补充说明：

1. `executor.run` 是标准 ReAct：`用户任务 -> LLM -> (可选工具调用) -> 工具结果回灌上下文 -> 下一轮`。
2. `runWithPlanMode` 使用 `Planner` 先生成计划，再交由 `PlanExecutor` 执行。
3. `runWithReviewMode` 按步骤回调确认（`ModeCallback.OnStepNeedsConfirmation`）。
4. `toolRegistryAdapter/toolAdapter` 把 `tools.Registry` 适配为 `plan` 子系统的 `ToolRegistry/Tool`。

## 2.3 `agent/plan`

| 文件 | 主要职责 | 关键内容 | 与其他组件关系 |
| --- | --- | --- | --- |
| `agent/plan/plan.go` | 计划领域模型 | `Plan`、`PlanStep`、状态机（draft/approved/running/...）、Markdown 输出、计划提示词 | 被 `Planner` 生成、被 `PlanExecutor` 执行、被 `loop.Engine` 驱动 |
| `agent/plan/planner.go` | 计划生成器 | `Planner.GeneratePlan` 调 LLM 产出 JSON/文本步骤并解析，`PlanValidator` 校验 | 被 `loop.Engine` 在 Plan 模式调用 |
| `agent/plan/executor.go` | 计划执行器 | `PlanExecutor.Execute`、步骤依赖检查、工具执行、权限检查、执行报告 | 被 `loop.Engine` 持有并执行计划 |
| `agent/plan/mode.go` | 模式与回调协议 | `RunMode`、`ModeConfig`、`ModeCallback` 默认实现 | 被 `loop.Engine` 配置与回调注入使用 |
| `agent/plan/planner_test.go` | 测试覆盖 | 覆盖 plan 构建、状态流转、解析、验证、复杂度估算、执行报告、执行器 | 验证 plan 子系统基础行为 |

补充说明：

1. `Planner` 负责“生成计划”，`PlanExecutor` 负责“执行计划”，模型与执行解耦。
2. `ModeCallback` 是关键扩展点，允许 UI/交互层接入审批、逐步确认。

## 2.4 `agent/memory`

| 文件 | 主要职责 | 关键内容 | 与其他组件关系 |
| --- | --- | --- | --- |
| `agent/memory/types.go` | 记忆数据模型 | `MemoryItem`、`MemoryType`、`Query`、`Config`、`MemoryStats` | 作为 store/retriever/manager 的共享模型 |
| `agent/memory/store.go` | 存储接口抽象 | `Store` / `ExtendedStore` | `Manager` 依赖接口而非具体存储 |
| `agent/memory/sqlite_store.go` | SQLite 存储实现 | schema 初始化、CRUD、Query 构建、统计、embedding 序列化 | `Manager` 通过 `Store` / `ExtendedStore` 使用 |
| `agent/memory/policy.go` | 保留与压缩策略 | `Policy`、`RetentionRule`、`PolicyEvaluator`、`CompactionPolicy` | `Manager.Save/Compact` 调用策略判断 |
| `agent/memory/retrieve.go` | 检索层 | `Retriever`（关键词/标签/时间/重要性/上下文相关性）、`SemanticRetriever`、`SimpleRetriever` | `Manager.Query` 与多类 retrieve 方法调用 |
| `agent/memory/embedding.go` | 向量化支持 | `Embedder` 接口、`MockEmbedder`、向量相似度、缓存 | 被 `SemanticRetriever` 或外部 embedding 流程使用 |
| `agent/memory/memory.go` | 记忆管理器主入口 | `Manager`：保存/查询/删除/压缩/备份/恢复/导入导出/会话转记忆 | 对内编排 store + retriever + policy |
| `agent/memory/memory_test.go` | 测试覆盖 | 覆盖 item 行为、policy、manager 基础读写、SQLite 联通、过期删除 | 当前主要保障基础功能可用 |

补充说明：

1. `memory.Manager` 提供高层 API（`SaveFact`/`SavePreference` 等），隐藏底层存储细节。
2. 支持结构化查询 + 启发式相关性检索 + 语义检索预留。
3. 当前仓库中该子系统尚未接入 `app/loop` 主流程（仍仅在测试内使用）。

## 2.5 `agent/session`

| 文件 | 主要职责 | 关键内容 | 与其他组件关系 |
| --- | --- | --- | --- |
| `agent/session/types.go` | 会话领域模型 | `Session`、`Metadata`、`RuntimeSnapshot`（model/permission/trace）、`Info`、`Config`、`Filter` | 作为 manager/store 与 app 恢复逻辑共享模型 |
| `agent/session/store.go` | 存储接口抽象 | `Store`（save/load/list/filter/delete/exists） | `Manager` 依赖该接口 |
| `agent/session/file_store.go` | 文件存储实现 | 基于 JSON 文件存取、过滤、排序、导入导出、清理过期会话 | 默认实现，可被 manager 强转用于 import/export/cleanup |
| `agent/session/manager.go` | 会话管理器 | 创建/加载/当前会话切换/归档/标签/消息管理/清理/导入导出、runtime 快照更新 | 对内编排 store + 缓存 + autosave |
| `agent/session/event_writer.go` | 会话轨迹写入 | `EventWriter`、`NewSessionTraceWriter`、`TracePathForSession` | 作为 trace 真实落盘实现，供 loop 通过最小 `TraceWriter` 接口注入 |
| `agent/session/manager_test.go` | 测试覆盖 | 覆盖 manager 生命周期、标签、归档、导入导出、cleanup、ID 格式与冲突后缀 | 验证 session 层行为一致性 |
| `agent/session/runtime_compat_test.go` | 兼容与恢复测试 | 旧 session JSON 兼容加载、同会话 trace append | 验证恢复与兼容行为 |
| `agent/session/event_writer_test.go` | 轨迹写入测试 | session trace 路径与写盘行为 | 验证轨迹写入契约 |

补充说明：

1. `session.Manager` 提供当前会话（`current`）语义，便于上层直接 append 消息。
2. `FileStore` 把每个 session 存为单独 JSON 文件，便于导出迁移。
3. 该子系统已通过 `app` 层接入主流程：启动时创建/恢复 session，运行时通过 `Engine.SetMessageSink` 实时持久化消息，并在退出时可通过 session id 恢复。

## 3. agent 内部组件关系

## 3.1 包级依赖关系

```text
loop  ---> context
  |
  +----> plan

plan  ---> (通过接口依赖) tools/permission
context ---> llm.Message
memory  ---> (独立子系统，当前未被 loop/context/plan 引用)
session ---> (通过 app 层接入运行时；包级仍与 loop/context/plan 解耦)
```

## 3.2 主运行链路（标准模式）

1. `loop.Engine.Run` 接收 `Task`。
2. `executor.run` 把用户任务写入 `context.Manager`。
3. `context.Manager.GetMessages()` 作为 LLM 请求上下文。
4. LLM 返回：
   - 有 `ToolCalls`：走 `executeToolCall`，经 `permission.Request` 授权后执行 `tools.Registry` 工具，再把结果 `AddToolResult` 回灌上下文。
   - 无 `ToolCalls`：直接产生 `EventAgentReply` 并结束循环。
5. 全流程事件通过 `Event` 列表输出，并可同步写入 `TraceWriter`（默认由 `session.EventWriter` 实现）。

## 3.3 Plan/Review 模式链路

1. `loop.Engine` 根据 `ModeConfig.Mode` 选择 `runWithPlanMode` 或 `runWithReviewMode`。
2. Plan 模式：`Planner.GeneratePlan` -> `ModeCallback.OnPlanCreated` -> `PlanExecutor.Execute`。
3. Review 模式：按步骤触发 `OnStepNeedsConfirmation`，逐步执行。
4. `PlanExecutor` 在步骤有工具时通过 `ToolRegistry` 执行，并可用 `PermissionService` 做授权。

## 3.4 context 内部关系

1. `Manager` 持有 `Tokenizer`、`Budget`、`Compactor`、`PriorityScorer`。
2. `AddMessage` 先预估 token，再按预算/阈值决定是否压缩。
3. `Compactor` 可按 simple/summarize/priority/hybrid 策略保留关键信息。
4. `TokenUsage` 与 `BudgetStats` 分别提供全局和分桶视角。

## 3.5 memory/session 与主链路关系

1. `memory` 与 `session` 设计为可独立接入的中长期状态层。
2. 当前主运行链路已调用 `session`（会话创建/恢复、消息实时落盘、权限与模型快照恢复、按会话轨迹落盘）。
3. `memory` 仍未接入主运行链路；当前“长期状态”主要由 `session` 提供，会话内短期推理由 `context.Manager` 承担。

## 4. 外部模块如何使用 agent

## 4.1 已接入的外部调用点

| 外部文件 | 调用方式 | 说明 |
| --- | --- | --- |
| `app/main.go` + `app/cli.go` | 解析 `run/resume/sessions list` | 提供会话恢复与可发现性入口 |
| `app/bootstrap.go` | 创建 `session.Manager`、`context.Manager` 与 `loop.Engine`，并在 `resume` 时恢复消息/模型/权限快照 | 生产模式主装配入口 |
| `app/wire.go` | `SetProvider` 时重建 `loop.Engine`，复用原 `ctxManager`/`permService`/`traceWriter`，并同步 session runtime 快照 | 模型切换后保持上下文与会话状态一致 |
| `app/run.go` | `Engine.Run(task)` 执行任务，`loop.Event` -> `ui/model.Event` 映射 | UI 与 agent 的桥接层 |
| `executor/runner.go` | 使用 `loop.Task` 类型（兼容接口） | 遗留兼容函数，真实执行由 `Engine.Run` 完成 |

## 4.2 尚未接入的 agent 子系统

| 子系统 | 现状 |
| --- | --- |
| `agent/memory` | 全仓库未发现业务代码 import，仅在 `agent/memory/*_test.go` 使用 |
| `agent/session` | 已在 `app/bootstrap.go`、`app/wire.go`、`app/run.go`、`app/commands.go`、`app/cli.go` 中接入 |

## 5. 评审观察与风险点

以下是从实现细节观察到的可改进点：

1. `context.Manager.shouldCompactLocked` 在启用 `Budget` 分支时阈值比较逻辑异常：把“当前预计使用率”当作 `Budget.ShouldCompact` 的 threshold 传入，导致判断几乎总为 false，自动压缩触发不稳定。
2. `plan.GeneratePlanPrompt(goal, tools)` 入参 `tools` 未被使用，工具列表被硬编码，可能让计划生成与真实可用工具不一致。
3. `plan.ExecutionConfig` 的 `MaxRetries`、`TimeoutPerStep` 目前未在执行流程生效，配置项与行为不一致。
4. `memory.Query.Metadata` 字段在 `SQLiteStore.Query` 中未实现过滤，接口能力与实现存在缺口。
5. `memory.Retriever` 保存了 `policy` 字段，但当前检索流程几乎未用该策略参与排序/过滤（除过期判断外），策略落地较弱。
6. `/compact` 命令（`app/commands.go`）当前只输出提示文字，未实际调用 `context.Manager.Compact`，用户预期与行为有偏差。

## 6. 结论

1. `agent/loop + agent/context + agent/plan` 已形成可运行主干，支持标准 ReAct 和计划化执行。
2. `agent/session` 已接入主流程并提供可恢复会话能力（`resume` + 实时持久化 + runtime 快照 + 会话轨迹）。
3. `agent/memory` 仍处于“预备态”；若下一步继续提升跨会话智能，建议在已接入 `session` 的基础上再接入 `memory`。
