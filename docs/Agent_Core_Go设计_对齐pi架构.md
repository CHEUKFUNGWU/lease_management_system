# Agent Core（Go）设计 —— 对齐 pi 架构

> 文档状态：Draft for Review
> 编制日期：2026-08-18
> 关联文档：
> - `docs/AI_底稿与Paperwork_Agent设计方案.md`（能力层：底稿产出、Tier A/B、不变量）
> - `docs/AI_Chat_升级方案_pi_agent_参考.md`（早期 pi 借鉴，运行时层）
> - `docs/AI_Agent_与_CLI_架构演进_PRD.md`（Tool Runtime 与 Gateway，已交付）
> - ~~`docs/AI_Agent_填表升级_tau_anydoc_实施计划.md`~~（**tau 部分作废**，见 §9；anydoc 部分保留）

---

## 1. 目标与非目标

### 1.1 目标

用 Go 重写 Agent 内核，**架构对齐 pi-agent-core**，并让自研 Python 归零。

三件事同时完成：

1. **换内核**：把当前 Runner-centric 的循环，重构为 pi 式的 **Agent-centric 有状态对象 + 纯循环函数**。
2. **收拢治理**：把散落在 descriptor、runtime、handler 里的权限/范围/Review Gate/幂等/审计/配额，统一挂到 `beforeToolCall` / `afterToolCall` 中间件链上。
3. **退役 ai-service**：LLM provider、planner、文档解析全部迁 Go（解析栈见 §8）。

### 1.2 非目标

- **不引入 tau**（Python 3.12 容器）——与"自研 Python 归零"直接冲突。
- **不引入 pi 本体**（TypeScript）——只对齐设计，不引入 Node 后端。
- **不做开放扩展生态**。pi 的 Extensions/Packages 市场不适合财务合规系统，工具注册必须受控。
- **不改 Tool Runtime 的对外契约**。`agenttools` 的 descriptor、schema 校验、Gateway API 保持不变。

---

## 2. 为什么对齐 pi：三件真正值钱的事

pi 的价值不在"像个聊天框"，而在三个结构选择。逐条说明它们对本项目意味着什么。

### 2.1 循环是纯的，I/O 全靠注入

`agent-loop.ts` 的 `agentLoop()` / `agentLoopContinue()` 是无 I/O 依赖的函数；`Agent` 类只是有状态的包装。所有外部能力都是构造时注入的函数：`streamFn`（必需）、`getApiKey`、`convertToLlm`、`transformContext`。

**对本项目的意义**：现在 `aichat.Runtime` 把 Postgres 持久化直接编织进循环（`prepare` / `complete` / `fail` / `persistAssistantMessage` / `appendEvent` 都在同一个对象里）。这让循环无法脱离数据库测试，也让"换一个 planner"要动持久化代码。纯循环把这两件事解耦。

### 2.2 `beforeToolCall` / `afterToolCall` 是唯一的策略挂载点

pi 只有两个闸点，且语义明确：`beforeToolCall` 在参数校验之后运行，可以 `block: true` 阻断；`afterToolCall` 在最终事件之前后处理结果。

**对本项目的意义——这是本次重构最大的收益。** 你们的治理逻辑现在分布在至少四处：

| 治理项 | 当前位置 |
|---|---|
| 权限 / 资源动作 | `ToolDescriptor.Permissions` |
| Review Gate | `ToolDescriptor.Review` + handler 内部 |
| 幂等 | `SupportsIdempotency` + handler 内部 |
| 租户范围 | `agenttools/scope.go` + 各 handler 自行取 `execution.Principal.Scope` |
| 审计 | `agenttools/audit.go` + runtime |
| 配额 | runner `Limits` |
| Skill allowlist | `agentskill/registry.go` + runner 校验 |

分散的后果是**没人能一眼看出"一次工具调用到底过了哪些闸"**，新增工具时容易漏挂某一项。收拢成一条有序中间件链后，这个问题变成结构性不可能。

### 2.3 持久化不在 core 里，而是订阅者

pi 明确不做本地持久化，`sessionId` 只透传给 provider 做缓存亲和；SQLite 后端是独立包。更关键的是它的 **run 结算语义**：`agent_end` 是最后一个事件，但**直到所有 listener 的 promise resolve，run 才算结束**。

**对本项目的意义**：审计、事件落库、SSE 推送、checkpoint 全部变成订阅者，而 run 结算语义保证了**审计不会因为循环先结束而丢事件**。这比现在"在循环里同步写库"更可靠，也更容易加新的旁路消费者（比如 §7 的 provenance 收集）。

---

## 3. 现状映射

| pi 概念 | 现有 Go 资产 | 差距 |
|---|---|---|
| `Agent` 有状态对象 | `agentrunner.Runner`（worker）+ `aichat.Runtime[T]`（会话）两套 | 无统一可嵌入的 Agent 对象 |
| `agentLoop()` 纯函数 | `Runner.Run` 内聚，含 Gateway/Capability I/O | 缺低阶纯入口 |
| `AgentState` | 状态散在 runbook、Request、DB 行 | state 不是一等公民 |
| `AgentTool.execute(id, params, signal, onUpdate)` | `agenttools.ToolHandler(ctx, ToolCall) (ToolResult, error)` | **缺 `onUpdate` 流式进度** |
| `executionMode: sequential\|parallel` | 无 | 缺 |
| `beforeToolCall` / `afterToolCall` | 等价能力存在但分散（见 §2.2） | **缺统一闸点** |
| 10 种细粒度事件 | 事件流偏 run 级 | 缺 `message_update` / `tool_execution_update` |
| steering / followUp 双队列 + mode | 有 steer / follow-up API，无队列语义与 mode | 部分 |
| `waitForIdle()` 结算语义 | 无 | 缺（审计可能丢事件）|
| `compact()` 上下文压缩 | 无 | 缺 |
| session backend 可插拔 | Postgres 绑死在 `aichat.Runtime` | 差距 |
| `abort()` | 有 cancel API | 有 |
| checkpoint / 断点续跑 | `agentrunner/checkpoint.go` | **pi 没有，是我们的加分项，须保留** |

结论：**骨架要换，资产基本都能复用。** `agenttools`（注册表、descriptor、schema 校验）、`agentcapability`、`agentskill`、`agentartifact` 全部保留不动。

---

## 4. 目标包结构

```
internal/agentcore/          # 纯循环 + 状态 + 事件。禁止 import database/sql、net/http、repository
├─ state.go                  # State
├─ loop.go                   # Loop / LoopContinue（纯函数）
├─ agent.go                  # Agent（有状态包装）
├─ event.go                  # Event 联合类型
├─ queue.go                  # steering / followUp 队列
└─ hooks.go                  # BeforeToolCall / AfterToolCall 类型 + Chain

internal/agentcore/hooks/    # 治理中间件（本次收拢的重点）
├─ tenant.go   capability.go   router.go
├─ idempotency.go   budget.go   review.go
└─ audit.go    artifact.go

internal/agentsession/       # 订阅者：持久化 / SSE / checkpoint / 指标
internal/llm/                # StreamFunc 的 provider 实现（Go 版 pi-ai）
internal/docparse/           # anydoc + PaddleOCR（见 §8）
internal/agenttools/         # 保留不动：注册表 / descriptor / schema 校验
```

**依赖方向单一**：`agentcore` 不认识任何上层包；`hooks` 只依赖 `agentcore` + `agenttools` + `agentcapability`；`agentsession` 只依赖 `agentcore` + `repository`。

---

## 5. 核心类型

### 5.1 State

```go
type State struct {
    mu sync.RWMutex

    SystemPrompt  string
    Model         ModelRef
    ThinkingLevel ThinkingLevel

    tools    []Tool
    messages []Message

    // 运行态（只读对外）
    streaming        bool
    streamingMessage *Message
    pendingToolCalls map[string]struct{}
    lastError        error
}

func (s *State) Tools() []Tool          // 返回副本
func (s *State) SetTools(t []Tool)      // 复制入参
func (s *State) Messages() []Message    // 返回副本
func (s *State) SetMessages(m []Message)
func (s *State) IsStreaming() bool
func (s *State) PendingToolCalls() []string
```

对齐 pi 的 `AgentState`：赋值时复制顶层切片，避免外部持有内部数组。Go 没有 getter/setter 语法，用方法对显式表达同一语义。

### 5.2 Tool

复用现有 `agenttools.ToolDefinition`，只补 pi 有而我们缺的两项：

```go
type ExecutionMode int
const (
    Sequential ExecutionMode = iota   // 默认。所有写入类工具必须是 Sequential
    Parallel
)

type UpdateFunc func(partial any)

type Tool interface {
    Descriptor() agenttools.ToolDescriptor
    Execute(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) (agenttools.ToolResult, error)
    ExecutionMode() ExecutionMode
}

// 把既有 ToolDefinition 适配成 Tool，默认 Sequential、onUpdate 忽略。
// 存量工具零改动即可接入。
func FromDefinition(d agenttools.ToolDefinition, mode ExecutionMode) Tool
```

**硬规则**：`Descriptor().Level != LevelRead` 的工具，`ExecutionMode` 必须是 `Sequential`。在注册期校验，不是运行期。理由：并行执行会打乱幂等与审计顺序。

pi 的 `terminate: true` 早停能力映射为 `ToolResult` 上的 `Terminate bool`。

### 5.3 事件

```go
type Event interface{ isAgentEvent() }

type AgentStart struct{}
type AgentEnd   struct{ Messages []Message }
type TurnStart  struct{}
type TurnEnd    struct{ Message Message; ToolResults []ToolResultMessage }

type MessageStart  struct{ Message Message }
type MessageUpdate struct{ Message Message; Delta AssistantDelta }
type MessageEnd    struct{ Message Message }

type ToolExecutionStart  struct{ CallID, ToolName string; Args json.RawMessage }
type ToolExecutionUpdate struct{ CallID, ToolName string; Args json.RawMessage; Partial any }
type ToolExecutionEnd    struct{ CallID, ToolName string; Result any; IsError bool }
```

与 pi 的事件联合逐项对齐。前端现有的 tool_start / tool_end 渲染是这套的子集，可平滑迁移。

### 5.4 闸点

```go
type BeforeContext struct {
    Call       agenttools.ToolCall
    Descriptor agenttools.ToolDescriptor
    State      *State
    Principal  agenttools.Principal
}

type BeforeResult struct {
    Block  bool                     // 阻断
    Reason string                   // 阻断原因，进审计与用户可见信息
    Args   json.RawMessage          // 非空则改写参数
    Short  *agenttools.ToolResult   // 非空则短路返回（Review Gate 用这个）
}

type BeforeToolCall func(context.Context, BeforeContext) (BeforeResult, error)
type AfterToolCall  func(context.Context, AfterContext)  (AfterResult, error)

func ChainBefore(hooks ...BeforeToolCall) BeforeToolCall  // 顺序执行，首个 Block 即止
func ChainAfter(hooks ...AfterToolCall) AfterToolCall     // 全部执行，错误聚合
```

`ChainBefore` 短路、`ChainAfter` 全执行——**这个不对称是刻意的**：阻断应该尽早，而审计类后置钩子必须全部跑到，不能因为前一个失败就跳过。

### 5.5 循环

```go
type Deps struct {
    Stream      StreamFunc                       // 必需，唯一的 LLM I/O 出口
    Before      BeforeToolCall
    After       AfterToolCall
    ShouldStop  func(context.Context, *State) bool
    PrepareNext func(context.Context, *State) error
    Emit        func(Event)
}

func Loop(ctx context.Context, s *State, d Deps) error
func LoopContinue(ctx context.Context, s *State, d Deps) error
```

`Loop` / `LoopContinue` **不 import 数据库、不 import net/http**。这条由 §10 的 ACORE-1 用测试固化。

### 5.6 Agent

```go
type Agent struct{ /* State + 两条队列 + 订阅者 + abort */ }

func New(opts Options) *Agent

func (a *Agent) Prompt(ctx context.Context, msg ...Message) error
func (a *Agent) Continue(ctx context.Context) error

func (a *Agent) Steer(msg Message)      // 助手回合结束后注入
func (a *Agent) FollowUp(msg Message)   // 仅当 Agent 将要停止时注入
func (a *Agent) ClearQueues()
func (a *Agent) HasQueued() bool

func (a *Agent) Subscribe(fn func(context.Context, Event) error) (unsubscribe func())
func (a *Agent) Abort()
func (a *Agent) WaitForIdle(ctx context.Context) error
func (a *Agent) Reset()
func (a *Agent) State() StateView       // 只读视图
```

队列 mode 对齐 pi 的 `QueueMode`：`All` | `OneAtATime`。

---

## 6. 治理中间件链（核心章节）

这是本次重构真正的产出：**一次工具调用要过哪些闸，一眼可见，且顺序固定。**

```go
before := agentcore.ChainBefore(
    hooks.TenantScope(...),        // 1  法人/门店范围交集，越权即 Block
    hooks.CapabilityCheck(...),    // 2  能力令牌有效性 + Skill allowlist
    hooks.ProtectedMeasure(...),   // 3  受保护度量判定（Tier A/B 路由）
    hooks.BudgetGuard(...),        // 4  调用数 / token / 时长配额
    hooks.IdempotencyGuard(...),   // 5  幂等键命中则短路返回既有结果
    hooks.ReviewGate(...),         // 6  Draft 级工具短路为 needs_review
)

after := agentcore.ChainAfter(
    hooks.AuditRecorder(...),      // 审计，不可绕过
    hooks.ArtifactCollector(...),  // provenance 收集（喂 WorkingPaper）
    hooks.MetricsRecorder(...),
)
```

**顺序的理由**（每一条都不是随意排的）：

| # | 中间件 | 为什么在这个位置 |
|---|---|---|
| 1 | TenantScope | 最便宜、最致命。越权请求不应消耗后续任何资源 |
| 2 | CapabilityCheck | 身份先于策略。令牌无效时后面的判定都无意义 |
| 3 | ProtectedMeasure | 必须在配额之前：被红线拒绝的请求不该计入用户配额 |
| 4 | BudgetGuard | 在真正产生成本之前 |
| 5 | IdempotencyGuard | 必须在 ReviewGate 之前——重放已确认的调用不应再次要求复核 |
| 6 | ReviewGate | 最后一道，短路返回 `needs_review`，把控制权交回人 |

**`ProtectedMeasure` 就是底稿方案 §4.2 的请求期路由**：命中 IFRS 16 受保护度量且无 Certified 工具可满足 → `Block: true` + 有帮助的拒绝理由。两份设计在这里合流。

**`ArtifactCollector` 就是底稿方案 §7.1 的 provenance 来源**：每次成功的 Certified 调用在此登记 `tool_call_id` / `engine_version` / `input_hash`，WorkingPaper 组装时直接取用，不需要事后回查。这解决了不变量 I2 的实现问题。

---

## 7. 订阅者与结算语义

```go
agent.Subscribe(agentsession.PostgresPersister(repo))   // 事件落库
agent.Subscribe(agentsession.SSEBroadcaster(hub))       // 推前端
agent.Subscribe(agentsession.CheckpointWriter(store))   // 断点续跑
agent.Subscribe(agentsession.UsageRecorder(store))      // 用量与成本
```

**结算语义（对齐 pi，且对我们更重要）**：`AgentEnd` 事件发出后，run **不算结束**，直到所有订阅者返回。`WaitForIdle` 等的就是这个。

这条保证了：**审计写入慢或阻塞时，run 会等它，而不是先结束再丢事件。** 现在 `aichat.Runtime` 在循环里同步写库，看似更安全，实际上把持久化故障变成了业务故障；订阅者模型让两者解耦，同时通过结算语义保住不丢。

订阅者失败策略：审计类订阅者失败 → 整个 run 标记失败（审计不可选）；推送类订阅者失败 → 记录告警，不影响 run。这个区分要在注册时显式声明，不能靠约定。

---

## 8. ai-service 退役映射

| ai-service 组件 | 行数 | Go 归属 |
|---|---|---|
| `services/llm.py` | 144 | `internal/llm/`（provider 抽象，借鉴 pi-ai 思路，不引依赖） |
| `routers/agent_plan.py` | 170 | 并入 `agentcore.Loop` 的 `Stream` 实现，**删掉一次 HTTP 往返和一个 token** |
| `routers/chat.py` | 117 | 同上 |
| `routers/files.py` + `services/storage.py` | 176 | `minio-go`；core-service 本就有凭据 |
| `services/paddleocr.py` | 446 | `internal/docparse/paddleocr.go`，纯 HTTP 提交+轮询+JSON 遍历 |
| `intake/` schema + prompt 编排 | ~400 | `internal/aiintake/`（Go 已有同名包，扩展） |
| 文档提取 — Excel | — | `excelize`（读+写，**顺带补上 xlsx 底稿产出**） |
| 文档提取 — office 家族 / 纯文本 PDF | — | **anydoc**（Rust CLI，MIT，Go 子进程调用） |
| 文档提取 — 扫描件 / 需坐标 PDF | — | **PaddleOCR API**（已提供块级 `{page, coordinates, quote}`） |
| **PyMuPDF** | — | **删除**（AGPL 风险一并消失） |

### 8.1 惰性证据（lazy evidence）

删掉 PyMuPDF 后，纯文本 PDF 失去了免费的坐标来源。不要对每份文件都跑 OCR：

```
1. 上传 → anydoc 出文本 → 抽取字段 → 生成草稿   （快、免费、本地）
2. 用户点某字段的「查看证据」→ 才对该文件跑一次 PaddleOCR
   → 缓存 locators，后续复用
```

把 OCR 成本从"每份文件"降到"用户真正要看证据的文件"，首次响应也快得多。

### 8.2 必须同步修正的失败语义

底稿方案 §8 里"OCR 不可用 → 降级到 PyMuPDF 文本层"这一行**已失效**。新规则：

> **OCR 不可用 → 降级 anydoc，产出文本，但证据状态标记为 `unavailable`，且不得声称任何坐标。**

必须写死。否则会出现"有文本就假装有证据"的静默降级，正好踩中 R3 红线。

---

## 9. 不照搬 pi 的部分

| pi 的做法 | 我们的选择 | 理由 |
|---|---|---|
| 无权限系统 | **保留 capability token + allowlist + 租户范围** | pi 是单机 coding agent，我们是多租户财务系统。这是护城河不是负担 |
| 无持久化，`sessionId` 仅透传 | **保留 Postgres 会话 + checkpoint + 断点续跑** | 我们的 run 可能跑几分钟并被 worker claim，崩溃必须能恢复 |
| 开放 Extensions / Packages 生态 | **工具注册受控** | 合规系统不能允许运行时注入任意工具 |
| 会话树 / 分支做得很重 | **先只做 checkpoint + 线性分支** | 业务优先级是可审计、可恢复、可中断，不是树形 UI |
| 无 Review Gate | **保留，且提升为一等中间件** | Assist Mode 是产品的基本承诺 |
| TypeBox schema | **沿用现有 `json.RawMessage` + 严格校验** | `agenttools/protocol.go` 已实现，且 Gateway 契约不变 |

一句话：**借鉴 pi 的运行时抽象，不借鉴它的信任模型。**

---

## 10. 迁移路径

每一波都可独立上线、可回退，且必须过平价门。

| 波次 | 内容 | 平价门 |
|---|---|---|
| **W1** | 抽出 `agentcore`（State / Loop / Event / Agent），用现有 planner 行为跑通，**不接前端** | `agent-evaluation.v1` 全绿 + skill contract replay 通过 |
| **W2** | 把散落的治理搬进 `hooks` 链，行为等价 | 同上 + §11 ACORE-2 变异测试通过 |
| **W3** | `aichat.Runtime` 的持久化改造为订阅者 | 事件序列与改造前逐条一致 |
| **W4** | `internal/llm` 落地，`chat.py` / `agent_plan.py` 退役 | 同输入下工具调用序列一致 |
| **W5** | `internal/docparse`（anydoc + PaddleOCR + excelize），**ai-service 完全退役、PyMuPDF 删除** | 抽取准确率不低于现基线（CORR-2） |
| **W6** | `agentrunner` 收敛为 `agentcore` 的一个 driver，删除重复循环 | worker 断点续跑用例通过 |

**W1–W3 不改变任何对外行为**，纯结构重构；W4 起才有可观测变化。

---

## 11. 验收标准

| ID | 验收项 | 验证方法 | 通过判据 |
|---|---|---|---|
| **ACORE-1** | 循环纯度 | 静态检查 `internal/agentcore` 的 import 图 | **不出现** `database/sql`、`net/http`、`internal/repository`、`github.com/minio` |
| **ACORE-2** | **治理不可绕过（变异测试）** | 逐个从 `ChainBefore` 中移除一个中间件，跑全量 SEC/PROV 用例 | **移除任意一个，都必须有测试变红**。全绿说明该中间件没有被任何用例覆盖 |
| **ACORE-3** | 审计不丢事件 | 注入一个故意慢 2s / 故意 panic 的审计订阅者 | 慢：run 等待其完成才结束；panic：run 标记失败，不静默吞掉 |
| **ACORE-4** | 行为等价 | 固定 seed 的 20 个会话，新旧循环对比 | 工具调用序列、参数、顺序**逐条一致** |
| **ACORE-5** | 并发工具安全 | 注册期校验 + 运行期断言 | 任何 `Level != Read` 的工具声明 `Parallel` → **注册失败**（编译期之后、启动期之前） |
| **ACORE-6** | 中断语义 | 运行中调用 `Abort()` | 无新工具调用发起；已开始的收到 context 取消；`WaitForIdle` 正常返回 |
| **ACORE-7** | 幂等在复核之前 | 重放一个已确认的 Draft 级调用 | 返回既有结果，**不再次要求复核** |
| **ACORE-8** | 队列语义 | steer / followUp 各测 `All` 与 `OneAtATime` | 与 pi 语义一致：steering 先于 follow-up 排空；follow-up 仅在将停止时注入 |
| **ACORE-9** | ai-service 归零 | W5 后检查 | 仓库内无自研 Python 服务；`pymupdf` 从依赖中消失 |

**ACORE-2 是这套验收里最重要的一条。** 它检验的不是"治理有没有实现"，而是"治理有没有被测试真正锁住"。一个删掉也不会让测试变红的中间件，等于没有。

---

## 12. 待决策

1. **`controlledxlsx` 的去留**。引入 excelize 后，自研的零依赖 XLSX 读取器是保留还是合并。倾向保留——受控模板路径的零依赖是优点，不该为统一而统一。
2. **上下文压缩（`compact()`）是否进 W1–W6**。pi 有，我们没有。建议先不做：底稿场景的会话长度可控，等出现真实的上下文溢出再补。
3. **`agentrunner` 是否彻底删除**，还是保留为 `agentcore` 的 worker driver（W6 两种收敛方式）。建议后者，断点续跑逻辑值得独立。
4. **anydoc 的接入形态**：Go 子进程调 CLI，还是编 Rust 静态库经 cgo。倾向子进程——进程边界更干净，且不引 cgo 编译复杂度。

---

## 13. 一句话总结

> 照抄 pi 的**运行时抽象**（纯循环 + 注入依赖 + 两个闸点 + 订阅者结算），不照抄它的**信任模型**（无权限、无持久化、开放生态）。收益不是"更像 pi"，而是**把六项散落的治理收拢成一条顺序固定、可被变异测试锁死的中间件链**，顺带让自研 Python 归零。
