# ms-cli 模块完善计划 V2

## 项目现状分析

### 当前架构状态

| 模块 | 状态 | 说明 |
|------|------|------|
| LLM Provider | ✅ 完整 | OpenAI/OpenRouter 客户端实现完成 |
| Tool System | ✅ 完整 | FS 和 Shell 工具已实现 |
| Agent Loop (Engine) | ⚠️ 基础 | ReAct 循环基础实现，需增加 Plan Mode |
| Context Manager | ⚠️ 基础 | 消息管理、Token估算已实现，需完善预算和压缩 |
| Permission System | ⚠️ 基础 | 权限级别和策略已实现，需完善 UI 集成和持久化 |
| Memory System | ❌ 缺失 | 仅接口定义，需完整实现 |
| Session Manager | ❌ 缺失 | 完全缺失，需新建 |
| Test Coverage | ❌ 缺失 | 完全没有测试文件 |

---

## Phase 1: 构建 Memory System (记忆系统)

### 目标
实现跨会话的长期记忆存储与检索系统，支持重要信息持久化和语义检索。

### 设计架构

```
agent/memory/
├── types.go         # 核心类型定义 (MemoryItem, Query, etc.)
├── store.go         # 存储接口扩展
├── sqlite_store.go  # SQLite 存储实现
├── memory.go        # 记忆管理器 (核心业务逻辑)
├── retrieve.go      # 检索逻辑实现
├── policy.go        # 保留策略实现
└── embedding.go     # 向量化接口 (预留扩展)
```

### 核心接口设计

```go
// MemoryItem 单个记忆项
type MemoryItem struct {
    ID        string
    Type      MemoryType      // session, fact, task, preference
    Content   string
    Metadata  map[string]any
    Embedding []float32       // 可选：用于语义检索
    CreatedAt time.Time
    UpdatedAt time.Time
    ExpiresAt *time.Time      // TTL 支持
    Importance int            // 1-10 重要性评分
}

// MemoryType 记忆类型
type MemoryType string
const (
    MemoryTypeSession    MemoryType = "session"    // 会话历史
    MemoryTypeFact       MemoryType = "fact"       // 事实知识
    MemoryTypeTask       MemoryType = "task"       // 任务记录
    MemoryTypePreference MemoryType = "preference" // 用户偏好
    MemoryTypeCode       MemoryType = "code"       // 代码片段
)

// Manager 记忆管理器
type Manager struct {
    store    Store
    policy   *Policy
    config   Config
}

// Store 存储接口
type Store interface {
    Save(item *MemoryItem) error
    Get(id string) (*MemoryItem, error)
    Query(query Query) ([]*MemoryItem, error)
    Delete(id string) error
    DeleteBefore(t time.Time) error
    Close() error
}

// Query 查询参数
type Query struct {
    Types      []MemoryType
    Keywords   []string
    Metadata   map[string]any
    Limit      int
    Offset     int
    MinImportance int
}
```

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `memory/types.go` | 定义 MemoryItem, MemoryType, Query 等类型 |
| 2 | P0 | `memory/store.go` | 扩展 Store 接口，定义存储契约 |
| 3 | P0 | `memory/sqlite_store.go` | 实现基于 SQLite 的存储 |
| 4 | P0 | `memory/memory.go` | 实现记忆管理器，CRUD 操作 |
| 5 | P1 | `memory/retrieve.go` | 实现检索逻辑（关键词、类型、时间过滤） |
| 6 | P1 | `memory/policy.go` | 实现保留策略（TTL、容量限制） |
| 7 | P2 | `memory/embedding.go` | 预留向量化接口（可选） |
| 8 | P1 | `memory/store_test.go` | 存储层单元测试 |
| 9 | P1 | `memory/memory_test.go` | 记忆管理器单元测试 |

### 关键特性

1. **持久化存储**: 使用 SQLite 存储记忆项
2. **TTL 支持**: 自动清理过期记忆
3. **重要性评分**: 根据内容自动或手动设置重要性
4. **多类型支持**: session, fact, task, preference, code
5. **检索能力**: 关键词搜索、类型过滤、时间范围、重要性筛选
6. **与 Context Manager 集成**: 自动保存关键对话到记忆

---

## Phase 2: 完善 Context Manager (上下文管理)

### 目标
增强上下文管理能力，实现更智能的 Token 预算控制和压缩策略。

### 现有实现分析
- ✅ 基础消息管理
- ✅ 简单 Token 估算（字符/4）
- ✅ 基础压缩策略（保留最近 N 轮）
- ⚠️ Budget 结构未使用
- ⚠️ Compact 只有 stub 实现

### 增强计划

```go
// 增强后的 Manager 配置
type ManagerConfig struct {
    MaxTokens           int
    ReserveTokens       int
    CompactionThreshold float64
    MaxHistoryRounds    int
    
    // 新增配置
    EnableSmartCompact  bool           // 启用智能压缩
    PriorityTopics      []string       // 高优先级主题
    BudgetConfig        BudgetConfig   // 预算分配
}

// Budget 增强
type Budget struct {
    MaxTokens        int
    ReserveTokens    int
    SystemBudget     int    // 系统提示词预算
    ToolResultBudget int    // 工具结果预算
    HistoryBudget    int    // 历史消息预算
    CurrentUsage     int
}

// 压缩策略增强
type CompactStrategy int
const (
    CompactStrategySimple CompactStrategy = iota   // 简单丢弃旧消息
    CompactStrategySummarize                       // 摘要旧消息
    CompactStrategyPriority                        // 基于优先级保留
)
```

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `context/budget.go` | 完善 Budget 结构，实现预算分配 |
| 2 | P0 | `context/manager.go` | 集成 Budget，完善预算检查 |
| 3 | P0 | `context/compact.go` | 实现多种压缩策略 |
| 4 | P1 | `context/tokenizer.go` | 更准确的 Token 估算（tiktoken 风格） |
| 5 | P1 | `context/priority.go` | 实现消息优先级系统 |
| 6 | P1 | `context/manager_test.go` | Context Manager 单元测试 |

### 压缩策略实现

```go
// 智能压缩：基于优先级和时间的复合策略
func (m *Manager) smartCompact() error {
    // 1. 标记消息优先级
    // 2. 生成低优先级消息的摘要
    // 3. 保留高优先级和最近的消息
    // 4. 将摘要插入到上下文中
}
```

---

## Phase 3: 增加 Session Manager (会话管理)

### 目标
实现会话生命周期管理，支持会话创建、恢复、保存和列表查看。

### 设计架构

```
agent/session/
├── types.go         # Session 类型定义
├── manager.go       # 会话管理器
├── store.go         # 会话存储接口
├── file_store.go    # 文件存储实现
└── session.go       # Session 实体操作
```

### 核心接口设计

```go
// Session 会话实体
type Session struct {
    ID        string
    Name      string
    WorkDir   string
    Messages  []llm.Message    // 当前会话消息
    Metadata  SessionMetadata
    CreatedAt time.Time
    UpdatedAt time.Time
}

// SessionMetadata 会话元数据
type SessionMetadata struct {
    TotalTokens   int
    MessageCount  int
    ToolCallCount int
    TaskCount     int
    Tags          []string
}

// Manager 会话管理器
type Manager struct {
    store      Store
    current    *Session
    config     Config
}

// Store 会话存储接口
type Store interface {
    Save(session *Session) error
    Load(id string) (*Session, error)
    List() ([]SessionInfo, error)
    Delete(id string) error
}

// Manager 方法
func (m *Manager) Create(name, workDir string) (*Session, error)
func (m *Manager) Load(id string) (*Session, error)
func (m *Manager) Save() error
func (m *Manager) List() ([]SessionInfo, error)
func (m *Manager) Current() *Session
func (m *Manager) Archive(id string) error
```

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `session/types.go` | 定义 Session, SessionMetadata 等类型 |
| 2 | P0 | `session/store.go` | 定义存储接口 |
| 3 | P0 | `session/file_store.go` | 实现文件存储（JSON/YAML） |
| 4 | P0 | `session/manager.go` | 实现会话管理器 |
| 5 | P1 | `session/manager_test.go` | 单元测试 |

### 与 Context Manager 的集成

```go
// 在 Engine 初始化时
func (e *Engine) initSession() {
    // 恢复或创建会话
    session := e.sessionManager.Current()
    if session == nil {
        session, _ = e.sessionManager.Create("default", e.workDir)
    }
    
    // 将会话消息加载到 Context Manager
    for _, msg := range session.Messages {
        e.ctxManager.AddMessage(msg)
    }
}
```

---

## Phase 4: 完善 Agent Loop - 增加 Plan Mode

### 目标
实现 Plan Mode（规划模式），让 Agent 在执行前先制定计划，提高复杂任务的处理能力。

### 当前实现分析
- ✅ 基础 ReAct 循环
- ✅ 工具调用执行
- ❌ 无计划制定阶段
- ❌ 无计划跟踪
- ❌ 无计划调整

### 设计架构

```
agent/loop/
├── engine.go        # 现有：扩展支持 Plan Mode
├── types.go         # 现有：扩展 Plan 相关类型
├── executor.go      # 新建：执行器，处理单步执行
├── planner.go       # 新建：规划器，制定和跟踪计划
├── plan.go          # 新建：计划实体定义
└── mode.go          # 新建：运行模式定义
```

### 核心概念

```go
// RunMode 运行模式
type RunMode int
const (
    ModeStandard RunMode = iota  // 标准模式：直接执行
    ModePlan                     // 计划模式：先制定计划
    ModeReview                   // 审核模式：每步需要确认
)

// Plan 执行计划
type Plan struct {
    ID          string
    Goal        string
    Steps       []PlanStep
    Status      PlanStatus
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
}

// PlanStep 计划步骤
type PlanStep struct {
    ID          string
    Description string
    Tool        string          // 可选：指定工具
    DependsOn   []string        // 依赖的其他步骤
    Status      StepStatus
    Result      string
}

// PlanStatus 计划状态
type PlanStatus string
const (
    PlanStatusDraft     PlanStatus = "draft"
    PlanStatusApproved  PlanStatus = "approved"
    PlanStatusRunning   PlanStatus = "running"
    PlanStatusPaused    PlanStatus = "paused"
    PlanStatusCompleted PlanStatus = "completed"
    PlanStatusFailed    PlanStatus = "failed"
)
```

### Plan Mode 流程

```
1. 用户输入任务
2. [PLAN MODE] Agent 分析任务并生成计划
   - 将计划展示给用户
   - 用户可以选择：
     a) 批准计划 → 进入执行阶段
     b) 修改计划 → 重新生成
     c) 取消 → 退出
3. [EXECUTE] 按计划步骤执行
   - 每个步骤执行前展示
   - 执行后更新状态
   - 支持暂停/继续
4. 完成所有步骤或遇到错误
5. 生成最终总结
```

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `loop/mode.go` | 定义 RunMode, PlanModeConfig |
| 2 | P0 | `loop/plan.go` | 定义 Plan, PlanStep, PlanStatus 等 |
| 3 | P0 | `loop/planner.go` | 实现规划器：生成、解析、跟踪计划 |
| 4 | P0 | `loop/executor.go` | 实现执行器：执行单步、更新状态 |
| 5 | P0 | `loop/engine.go` | 扩展 Engine 支持 Plan Mode |
| 6 | P1 | `loop/planner_test.go` | 规划器单元测试 |
| 7 | P1 | `loop/engine_test.go` | Engine 单元测试 |

### Planner 实现

```go
type Planner struct {
    provider llm.Provider
    config   PlannerConfig
}

// GeneratePlan 根据任务生成计划
func (p *Planner) GeneratePlan(ctx context.Context, task string, tools []Tool) (*Plan, error) {
    prompt := buildPlanPrompt(task, tools)
    resp, err := p.provider.Complete(ctx, &llm.CompletionRequest{
        Messages: []llm.Message{llm.NewUserMessage(prompt)},
    })
    if err != nil {
        return nil, err
    }
    
    // 解析 LLM 响应，提取计划步骤
    return parsePlan(resp.Content)
}

// buildPlanPrompt 构建计划生成提示词
func buildPlanPrompt(task string, tools []Tool) string {
    return fmt.Sprintf(`You are a planning assistant. Create a step-by-step plan for the following task.

Task: %s

Available tools: %v

Create a plan in JSON format:
{
  "steps": [
    {"description": "step description", "tool": "optional_tool_name"}
  ]
}`, task, tools)
}
```

---

## Phase 5: 完善 Permission 管理系统

### 目标
完善权限管理系统，实现与 UI 的集成、配置持久化和更细粒度的控制。

### 当前实现分析
- ✅ 权限级别定义 (Deny, Ask, AllowOnce, AllowSession, AllowAlways)
- ✅ DefaultPermissionService 基础实现
- ✅ 基于工具类型的默认策略
- ⚠️ 缺少 UI 交互实现
- ⚠️ 缺少配置持久化
- ⚠️ 缺少更细粒度的控制（如命令级别）

### 增强设计

```go
// PermissionConfig 权限配置
type PermissionConfig struct {
    DefaultLevel   PermissionLevel
    ToolPolicies   map[string]PermissionLevel
    CommandPolicies map[string]PermissionLevel  // 新增：命令级别策略
    PathPatterns   []PathPermission            // 新增：路径级别策略
    PersistDecisions bool                      // 是否持久化决策
}

// PathPermission 路径权限
type PathPermission struct {
    Pattern string          // glob 模式
    Level   PermissionLevel
}

// PermissionDecision 权限决策记录（用于持久化）
type PermissionDecision struct {
    Tool      string
    Action    string
    Path      string
    Level     PermissionLevel
    Timestamp time.Time
}
```

### 增强 PermissionService

```go
type DefaultPermissionService struct {
    // ... 现有字段 ...
    
    // 新增字段
    commandPolicies map[string]PermissionLevel
    pathPatterns    []PathPermission
    decisions       []PermissionDecision
    store           PermissionStore  // 持久化存储
}

// CheckCommand 检查命令权限
func (s *DefaultPermissionService) CheckCommand(cmd string) PermissionLevel {
    // 1. 检查命令级别策略
    // 2. 检查黑名单/白名单
    // 3. 返回默认级别
}

// CheckPath 检查路径权限
func (s *DefaultPermissionService) CheckPath(path string) PermissionLevel {
    // 检查路径模式匹配
}
```

### 危险命令检测

```go
// DangerousCommand 危险命令定义
type DangerousCommand struct {
    Pattern     string   // 正则或关键字
    Level       PermissionLevel
    Description string
}

var defaultDangerousCommands = []DangerousCommand{
    {Pattern: `rm\s+-rf`, Level: PermissionAsk, Description: "Recursive force delete"},
    {Pattern: `>\s*/dev/`, Level: PermissionDeny, Description: "Direct device write"},
    {Pattern: `dd\s+if=`, Level: PermissionAsk, Description: "Disk operations"},
    {Pattern: `sudo`, Level: PermissionAsk, Description: "Elevated privileges"},
    {Pattern: `chmod\s+777`, Level: PermissionAsk, Description: "Wide permissions"},
}
```

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `loop/permission.go` | 扩展 PermissionService 接口 |
| 2 | P0 | `loop/permission.go` | 实现命令级别权限检查 |
| 3 | P0 | `loop/permission.go` | 实现路径级别权限检查 |
| 4 | P1 | `loop/permission_store.go` | 实现权限决策持久化 |
| 5 | P1 | `loop/dangerous.go` | 危险命令检测 |
| 6 | P1 | `loop/permission_test.go` | 单元测试 |
| 7 | P2 | `ui/permission_dialog.go` | UI 权限确认对话框 |

---

## Phase 6: 补充 Test Cases (测试覆盖)

### 目标
建立完整的测试体系，确保代码质量和稳定性。

### 测试策略

```
测试目录结构:
├── agent/
│   ├── loop/*_test.go         # Agent Loop 测试
│   ├── context/*_test.go      # Context Manager 测试
│   ├── memory/*_test.go       # Memory System 测试
│   └── session/*_test.go      # Session Manager 测试
├── tools/
│   ├── fs/*_test.go           # 文件工具测试
│   └── shell/*_test.go        # Shell 工具测试
├── integrations/
│   └── llm/*_test.go          # LLM Provider 测试
├── configs/*_test.go          # 配置系统测试
└── testdata/                  # 测试数据
```

### 测试类型

1. **单元测试**: 测试单个函数/方法
2. **集成测试**: 测试组件间交互
3. **Mock 测试**: 模拟 LLM 响应和外部依赖

### 测试覆盖率目标

| 模块 | 目标覆盖率 |
|------|-----------|
| agent/loop | > 70% |
| agent/context | > 70% |
| agent/memory | > 70% |
| agent/session | > 70% |
| tools/fs | > 60% |
| tools/shell | > 60% |
| integrations/llm | > 50% |
| configs | > 70% |

### 实现任务清单

| 任务 | 优先级 | 文件 | 说明 |
|------|--------|------|------|
| 1 | P0 | `agent/loop/engine_test.go` | Engine 单元测试 |
| 2 | P0 | `agent/loop/planner_test.go` | Planner 单元测试 |
| 3 | P0 | `agent/context/manager_test.go` | Context Manager 测试 |
| 4 | P0 | `agent/memory/store_test.go` | Memory Store 测试 |
| 5 | P0 | `agent/memory/memory_test.go` | Memory Manager 测试 |
| 6 | P1 | `agent/session/manager_test.go` | Session Manager 测试 |
| 7 | P1 | `tools/fs/*_test.go` | 文件工具测试 |
| 8 | P1 | `tools/shell/*_test.go` | Shell 工具测试 |
| 9 | P1 | `configs/*_test.go` | 配置系统测试 |
| 10 | P1 | `test/mocks/llm.go` | LLM Provider Mock |
| 11 | P2 | `test/integration/*_test.go` | 集成测试 |

### Mock 实现示例

```go
// test/mocks/llm.go
package mocks

type MockProvider struct {
    Responses []llm.CompletionResponse
    CallCount int
}

func (m *MockProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
    if m.CallCount < len(m.Responses) {
        resp := m.Responses[m.CallCount]
        m.CallCount++
        return &resp, nil
    }
    return &llm.CompletionResponse{Content: "mock response"}, nil
}
// ... 实现其他接口方法
```

---

## 开发顺序建议

### 推荐开发顺序（考虑依赖关系）

```
Week 1: 基础设施
├── Day 1-2: Context Manager 完善
│   └── budget.go, compact.go 增强
├── Day 3-4: Memory System 基础
│   └── types.go, store.go, sqlite_store.go
└── Day 5: Memory System 业务
    └── memory.go, retrieve.go

Week 2: 核心业务
├── Day 1-2: Session Manager
│   └── 完整实现
├── Day 3-4: Agent Loop - Plan Mode
│   └── mode.go, plan.go, planner.go
└── Day 5: Agent Loop - 集成
    └── executor.go, engine.go 扩展

Week 3: 权限与测试
├── Day 1-2: Permission System 完善
│   └── 命令级别、路径级别检查
├── Day 3-5: 测试覆盖
│   └── 各模块单元测试

Week 4: 集成与优化
├── Day 1-2: 系统集成
│   └── bootstrap.go 更新，组件初始化
├── Day 3: 集成测试
└── Day 4-5: 优化与文档
```

---

## 文件变更汇总

### 新增文件 (18个)

```
agent/
├── memory/
│   ├── types.go
│   ├── sqlite_store.go
│   ├── memory.go
│   └── memory_test.go
├── session/
│   ├── types.go
│   ├── store.go
│   ├── file_store.go
│   ├── manager.go
│   └── manager_test.go
└── loop/
    ├── mode.go
    ├── plan.go
    ├── planner.go
    ├── executor.go
    ├── planner_test.go
    └── engine_test.go

agent/context/
├── tokenizer.go
├── priority.go
└── manager_test.go

agent/loop/
├── permission_store.go
├── dangerous.go
└── permission_test.go

tools/fs/*_test.go
tools/shell/*_test.go
configs/*_test.go
test/mocks/llm.go
test/integration/*_test.go
```

### 修改文件 (8个)

```
agent/
├── memory/
│   ├── store.go (扩展接口)
│   ├── retrieve.go (实现检索)
│   └── policy.go (实现策略)
├── context/
│   ├── budget.go (完善实现)
│   ├── compact.go (完善实现)
│   └── manager.go (集成增强)
└── loop/
    ├── engine.go (支持 Plan Mode)
    ├── types.go (扩展类型)
    └── permission.go (完善权限)
```

---

## 验收标准

### Phase 1: Memory System
- [ ] SQLite 存储实现完成
- [ ] 记忆 CRUD 操作正常
- [ ] 检索功能工作正常
- [ ] TTL 自动清理生效
- [ ] 单元测试覆盖率 > 70%

### Phase 2: Context Manager
- [ ] Budget 预算分配实现
- [ ] 智能压缩策略实现
- [ ] Token 估算更准确
- [ ] 单元测试覆盖率 > 70%

### Phase 3: Session Manager
- [ ] 会话创建、保存、加载正常
- [ ] 会话列表查看功能
- [ ] 会话与 Context Manager 集成
- [ ] 单元测试覆盖率 > 70%

### Phase 4: Plan Mode
- [ ] Plan Mode 流程完整
- [ ] 计划生成、解析正常
- [ ] 计划执行跟踪正常
- [ ] 用户确认流程正常
- [ ] 单元测试覆盖率 > 70%

### Phase 5: Permission System
- [ ] 命令级别权限检查
- [ ] 路径级别权限检查
- [ ] 危险命令检测
- [ ] 权限决策持久化
- [ ] 单元测试覆盖率 > 70%

### Phase 6: Test Coverage
- [ ] 核心模块测试覆盖率 > 70%
- [ ] Mock 框架可用
- [ ] 集成测试通过
- [ ] CI 测试流水线配置

---

## 附录: 技术决策记录

### ADR 1: Memory 存储选择 SQLite
- **决策**: 使用 SQLite 作为默认存储
- **理由**: 
  - 零配置，单文件存储
  - 支持 SQL 查询，便于检索
  - Go 原生支持 (sqlite3 驱动)
  - 无需额外服务
- **替代方案**: 
  - 纯文件存储 - 检索不便
  - 嵌入式 KV (BoltDB) - 不支持复杂查询
  - 外部数据库 - 增加部署复杂度

### ADR 2: Session 存储选择 JSON 文件
- **决策**: 使用 JSON 文件存储会话
- **理由**:
  - 人类可读，便于调试
  - 与现有项目风格一致
  - 会话数据量不大
- **替代方案**:
  - SQLite - 会话需要频繁读写，但结构简单

### ADR 3: Plan Mode 实现方式
- **决策**: 独立 Planner 组件 + 扩展 Engine
- **理由**:
  - 规划逻辑与执行逻辑分离
  - 便于独立测试
  - 支持多种规划策略
- **替代方案**:
  - 直接集成到 Engine - 耦合度高

### ADR 4: 测试框架选择
- **决策**: 使用标准库 testing + testify
- **理由**:
  - testify 提供方便的断言和 Mock
  - 标准库兼容性最好
- **依赖**: `github.com/stretchr/testify`
