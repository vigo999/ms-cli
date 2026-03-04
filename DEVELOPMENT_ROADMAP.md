# ms-cli 后续开发计划

基于代码检查结果（总体完成度约75%），制定以下开发计划。

---

## 📌 总体目标

**目标**: 完成剩余25%的核心功能，达到生产可用状态

**时间规划**: 4-6 周

**优先级原则**: 
1. 先补测试（保证质量）
2. 再补核心功能（记忆、追踪）
3. 最后优化体验

---

## Week 1: 单元测试覆盖

### 目标
建立测试体系，为核心模块添加单元测试，覆盖率目标 > 60%

### 任务清单

#### Day 1-2: configs 模块测试
```
configs/
├── loader_test.go      # 配置加载测试
├── types_test.go       # 配置类型测试
└── state_test.go       # 状态管理测试
```

**测试要点**:
- [ ] 配置文件解析（YAML格式正确性）
- [ ] 环境变量覆盖优先级
- [ ] 配置验证逻辑（必填项、范围检查）
- [ ] 状态保存/加载
- [ ] 错误处理（文件不存在、格式错误）

#### Day 3-4: integrations/llm 模块测试
```
integrations/llm/
├── provider_test.go
├── registry_test.go
├── openai/client_test.go
└── openrouter/client_test.go
```

**测试要点**:
- [ ] Provider 注册/获取
- [ ] 请求构建（消息格式、工具格式）
- [ ] 响应解析（正常响应、错误响应）
- [ ] Mock 测试（模拟 API 响应）
- [ ] 流式响应处理

#### Day 5: tools 模块测试
```
tools/
├── fs/
│   ├── read_test.go
│   ├── write_test.go
│   ├── edit_test.go
│   ├── grep_test.go
│   └── glob_test.go
└── shell/
    └── runner_test.go
```

**测试要点**:
- [ ] 工具 Schema 生成
- [ ] 参数解析
- [ ] 执行结果格式化
- [ ] 错误处理

---

## Week 2: Agent Loop 完善 + 上下文压缩

### 目标
完善 ReAct 循环，实现真正的上下文压缩功能

### 任务清单

#### Day 1-2: 上下文压缩实现
```
agent/context/
├── compact.go          # 重写：实现智能压缩
└── compact_test.go
```

**功能要求**:
- [ ] 实现摘要生成（使用 LLM 对历史消息做摘要）
- [ ] 消息优先级排序（System > Recent > ToolResults > Thinking）
- [ ] 自动丢弃策略（当接近 Token 上限时）
- [ ] 保留重要消息（错误、用户明确标记的）

**算法草案**:
```go
func (m *Manager) CompactIfNeeded() {
    if m.tokenUsage < threshold {
        return
    }
    
    // 1. 对旧消息生成摘要
    summary := m.summarizeOldMessages()
    
    // 2. 保留最近 N 轮完整对话
    // 3. 丢弃中间轮次的详细思考过程
    // 4. 保留所有工具执行结果
}
```

#### Day 3: Tokenizer 实现
```
agent/context/tokenizer.go
```

**方案选择**:
- 方案A: 使用 tiktoken-go (OpenAI 官方)
- 方案B: 简单估算 (1 token ≈ 4 chars)
- **推荐**: 方案B（简单够用，避免依赖）

#### Day 4-5: Agent Loop 优化
```
agent/loop/engine.go
```

**优化点**:
- [ ] 添加执行超时控制（防止单步执行过久）
- [ ] 改进错误恢复（API 失败重试）
- [ ] 添加执行日志（详细记录每轮决策）

---

## Week 3: 记忆系统 (Phase 3.2)

### 目标
实现跨会话记忆功能

### 任务清单

#### Day 1: 存储层设计
```
agent/memory/
├── store.go            # 重写：实现文件存储
├── sqlite.go           # 新建：SQLite 实现（可选）
└── store_test.go
```

**存储内容**:
```go
type Memory struct {
    ID          string
    SessionID   string
    Type        string      // "fact", "code", "error", "decision"
    Content     string
    Tags        []string
    Importance  int         // 1-5
    CreatedAt   time.Time
    AccessCount int
}
```

#### Day 2-3: 记忆提取与检索
```
agent/memory/
├── retrieve.go         # 重写：实现检索
├── retrieve_test.go
└── embedding.go        # 新建：向量化接口（可选）
```

**检索策略**:
- 方案A: 关键词匹配（简单）
- 方案B: 向量相似度（需要 Embedding API）
- **推荐**: 方案A + 标签过滤

**实现要点**:
- [ ] 按类型过滤（用户查询"之前的错误"→只搜 error 类型）
- [ ] 按重要性排序
- [ ] 按时间衰减（太久远的降低权重）

#### Day 4: 记忆生成策略
```
agent/memory/
├── policy.go           # 重写：记忆保留策略
└── policy_test.go
```

**策略规则**:
- [ ] 重要决策自动记忆
- [ ] 错误教训自动记忆
- [ ] 用户明确标记的记忆
- [ ] 定期清理低重要性记忆

#### Day 5: 集成到 Agent Loop
```
agent/loop/engine.go
```

**集成点**:
- [ ] 启动时加载相关记忆
- [ ] 结束时保存重要信息
- [ ] 每轮决策前检索相关记忆

---

## Week 4: 执行追踪与报告 (Phase 3.3)

### 目标
实现完整的执行追踪和报告生成

### 任务清单

#### Day 1-2: 执行追踪系统
```
trace/
├── writer.go           # 重写：结构化追踪
├── file.go             # 新建：文件写入
├── format.go           # 新建：格式化
└── writer_test.go
```

**追踪内容**:
```go
type Trace struct {
    TaskID      string
    StartedAt   time.Time
    CompletedAt time.Time
    Steps       []Step
    TokenUsage  TokenUsage
    Status      string  // "success", "failed", "cancelled"
}

type Step struct {
    Type        string  // "llm_call", "tool_call", "thinking"
    Timestamp   time.Time
    Input       string
    Output      string
    Duration    time.Duration
    Error       error
}
```

#### Day 3: 报告生成系统
```
report/
├── summary.go          # 重写：报告生成
├── template.go         # 新建：模板系统
├── markdown.go         # 新建：Markdown 格式
└── summary_test.go
```

**报告类型**:
- [ ] 执行摘要（一句话总结）
- [ ] 详细报告（每步执行详情）
- [ ] Token 使用报告
- [ ] 时间分析报告

**输出格式**:
- Markdown（人类可读）
- JSON（机器解析）

#### Day 4-5: 集成到 UI
```
ui/
├── app.go              # 添加报告导出命令
└── panels/report.go    # 新建：报告预览面板
```

**功能**:
- [ ] `/export` 命令导出报告
- [ ] 实时显示执行统计
- [ ] 错误时显示详细追踪

---

## Week 5: 权限交互 + 体验优化

### 目标
完善权限确认交互，优化用户体验

### 任务清单

#### Day 1-2: 权限确认 UI
```
ui/components/
├── confirm.go          # 新建：确认对话框组件
└── confirm_test.go
```

**交互流程**:
```
AI 请求执行: shell rm -rf /
↓
显示确认对话框:
┌─────────────────────────────────────┐
│ ⚠️  需要确认                          │
│                                      │
│ 工具: shell                           │
│ 命令: rm -rf /                        │
│                                      │
│ [Once] [Session] [Always] [Deny]     │
└─────────────────────────────────────┘
```

#### Day 3: TUI 体验优化
```
ui/
├── app.go
├── panels/
│   ├── chat.go
│   └── topbar.go
```

**优化点**:
- [ ] 添加加载状态指示
- [ ] 优化消息渲染性能（大数据量时卡顿）
- [ ] 添加消息折叠/展开功能

#### Day 4-5: 性能优化
```
├── integrations/llm/
│   └── client_pool.go  # 新建：HTTP 连接池
├── agent/loop/
│   └── engine.go       # 优化：并发工具执行
```

**优化项**:
- [ ] HTTP Client 连接池
- [ ] 并发执行独立工具（无依赖的工具并行执行）
- [ ] 大文件读取优化（分页加载）

---

## Week 6: 集成测试 + 文档

### 目标
完整测试，文档完善，准备发布

### 任务清单

#### Day 1-2: 集成测试
```
tests/
├── integration/
│   ├── agent_test.go       # Agent 端到端测试
│   ├── provider_test.go    # Provider 集成测试
│   └── tools_test.go       # 工具集成测试
└── e2e/
    └── workflow_test.go    # E2E 场景测试
```

**测试场景**:
- [ ] "读取文件并分析"
- [ ] "搜索代码并修改"
- [ ] "执行命令并处理错误"

#### Day 3-4: 文档完善
```
docs/
├── README.md               # 重写：用户文档
├── CONFIG.md               # 新建：配置说明
├── COMMANDS.md             # 新建：命令参考
└── TROUBLESHOOTING.md      # 新建：故障排除
```

#### Day 5: 发布准备
- [ ] 版本号更新
- [ ] CHANGELOG 编写
- [ ] 二进制构建测试
- [ ] Git Tag 打标签

---

## 📊 里程碑检查点

| 周次 | 检查点 | 验收标准 |
|------|--------|----------|
| Week 1 | 单元测试 | 测试覆盖率 > 60%，CI 通过 |
| Week 2 | 上下文压缩 | `/compact` 真正压缩上下文 |
| Week 3 | 记忆系统 | 跨会话能记住重要信息 |
| Week 4 | 追踪报告 | 能导出执行报告 |
| Week 5 | 权限交互 | 危险操作有确认对话框 |
| Week 6 | 发布就绪 | 文档完整，测试通过 |

---

## 🎯 关键决策

### 技术选型决策

#### 1. 记忆存储：JSON 文件 vs SQLite
- **建议**: JSON 文件（简单够用）
- **理由**: 
  - 无需外部依赖
  - 用户可手动编辑
  - 数据量不大时性能足够

#### 2. Token 计算：精确 vs 估算
- **建议**: 简单估算（1 token ≈ 4 chars）
- **理由**:
  - 无需引入 tiktoken 依赖
  - 估算足够用于上下文管理
  - API 调用时实际 Token 从响应获取

#### 3. Embedding：实现 vs 跳过
- **建议**: 第一阶段跳过
- **理由**:
  - 需要额外 API 调用（成本）
  - 关键词+标签搜索足够
  - 后续可以扩展

---

## 📋 日常开发检查清单

每天开始开发前检查：
- [ ] 昨天代码能编译通过
- [ ] 测试能跑通
- [ ] 清楚今天要完成的具体任务

每完成一个功能：
- [ ] 代码能编译
- [ ] 添加了测试
- [ ] 测试通过
- [ ] 更新了文档（如需要）

每周结束：
- [ ] 运行全部测试
- [ ] 更新本计划进度
- [ ] 调整下周计划（如需要）

---

## 🚨 风险预警

| 风险 | 可能性 | 缓解措施 |
|------|--------|----------|
| Week 1 测试工作量大 | 高 | 优先测试核心模块，边缘模块可延后 |
| Week 3 记忆系统设计复杂 | 中 | 简化第一版，关键词搜索即可 |
| 第6周集成问题多 | 中 | 每完成模块就集成测试，不堆到最后 |

---

## 💡 开发建议

1. **保持可运行**: 每天结束时代码要能编译运行
2. **小步快跑**: 每个功能拆小，快速迭代
3. **测试先行**: 先写测试，再写实现
4. **及时提交**: 功能完成就 commit，不要攒大提交
5. **文档同步**: 代码改动同步更新文档

---

*计划制定时间: 2026-03-03*
*下次评审: Week 1 结束*
