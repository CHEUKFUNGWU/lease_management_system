# AR5a 分析报告 — picoclaw pkg/agent 依赖闭包 · 挂点对齐 · 形状 A · 汇流

日期：2026-08-24 · 性质：只读分析，未动任何生产代码
上游实测基线：sipeed/picoclaw @ `bbf6893ca7afad27f1d00a0f5a45982a549c6ed6`
本单未 go get、未建包、未改文件。

---

## 1. 最小闭包清单

### 1.1 全包事实

`pkg/agent` 非测试代码 **56 文件 / 19,225 行**，import 22 个 picoclaw 包
（providers×53、config×42、bus×39、logger×33、events×30、tools×23、
session×15、routing×13……）。整包搬 = 两万行 + 把 22 个包的依赖面拖进来，
直接违反 ADR-0028 §1。以下按能力裁剪。

### 1.2 目标能力的核心闭包（纯循环 + 治理挂点 + 流式 + steering/abort）

| 文件 | 行数 | 为什么在闭包里 |
|---|---|---|
| hooks.go | 930 | **挂点本体**：HookManager、LLMInterceptor(Before/AfterLLM)、ToolInterceptor(Before/AfterTool)、ToolApprover、HookDecision(continue/modify/respond/deny_tool/abort_turn/hard_abort) |
| turn_coord.go | 734 | runTurn：单回合的阶段编排（LLM↔工具迭代、上限控制）——这就是"纯循环" |
| pipeline_llm.go | 861 | CallLLM 循环体 + BeforeLLM/AfterLLM 分发点 + streaming delta |
| pipeline_execute.go | 864 | ExecuteTools：BeforeTool/ApproveTool/执行/AfterTool 分发点 |
| pipeline_streaming.go | 496 | 流式输出管线（流式事件的消费侧） |
| pipeline.go/setup/finalize | 310 | Pipeline 结构与首尾阶段 |
| turn_state.go | 916 | per-turn 可变状态（**需形状 A 改造**，见 §3） |
| steering.go | 583 | steering 队列与注入 |
| agent_stop.go + agent_steering.go | 234 | abort / steering 入口 |
| agent_utils.go | 725 | 上述文件的共享工具函数（被引用，非独立能力） |
| event_payloads.go + events_runtime.go + events.go | 345 | 流式事件载荷与别名 |
| definition.go + prompt*.go + thinking.go + llm_media.go 等 prompt 面 | ~1,100 | runTurn 依赖的 prompt 组装 |

**直搬小计 ≈ 7,300 行。**

明确**不在闭包**：instance.go(800)、agent_init.go(408)、registry.go、discovery.go、
agent_mcp.go、evolution_bridge.go(451)、context_seahorse.go、agent_command.go、
agent_media.go、agent_transcribe.go、manager*/dynamic_mux（channels 侧）、
subturn.go(708，属 AR4)、context_budget/legacy（属 AR3 自建）。合计排除 ~12,000 行。

### 1.3 依赖面的处置（照 Ch3b first-party 先例）

| 依赖 | 引用数 | 处置 |
|---|---|---|
| providers | 53 | **最大替代面**。Message/ToolDefinition/LLMResponse/LLMProvider 四个类型渗透全部核心文件。本仓 `internal/llm` 留任（ADR §7），做法二选一：(a) first-party types 包（照 Ch3b bus 模式，把四个类型抄成本仓形状，provider 侧由 llm 适配器实现）；(b) 每处引用做类型转换 wrapper。(a) 成本低得多 |
| config | 42 | first-party 替代（EffectiveTurnProfile 等少数子结构），同 Ch3b |
| logger | 33 | slog 替代，八函数签名兼容，同 Ch3b |
| events | 30 | runtimeevents.Bus 在 pkg/events（1,801 行非测试）。流式事件需要它：搬 bus/channel 两文件或缩为接口。建议搬后裁剪 |
| tools | 23 | picoclaw 工具注册表 → **用本仓 agenttools.ToolRuntime 替代**（工具集已存在且带权限过滤） |
| session / routing | 28 | **不搬清单内**。引用散布在 turn_state/pipeline_setup/agent_message/dispatch_request/turn_context 等 8 个文件里，需要剥离改写——见 §1.4 风险 |
| isolation/memory/seahorse/mcp/evolution/skills/media/state/constants/commands/fileutil | <20 | 整文件排除（对应上游文件已在 §1.2 清单外） |

### 1.4 替代之后的真实闭包

- 直搬上游：~7,300 行 − session/routing 剥离涉及的段落
- first-party 替代包（types/config/logger/events 接口）：~800–1,200 行
- **胶水重写**（形状 A 改造 + session/routing 剥离 + 本仓 ToolRuntime 桥接）：
  ~600–1,000 行，**这不是上游代码，是要写的**

总量级：**~8,500–9,500 行进仓，其中约 1/7 是要自己写的胶水**。比 Ch3b 大 40%，
且胶水比例反转（Ch3b 胶水≈0）。

## 2. 挂点对齐表（ADR-0028 §2）

| 本仓既有（agentcore/hooks） | picoclaw 对应 | 语义等价？ | 差异 |
|---|---|---|---|
| `BeforeToolCall` 链，`BeforeResult{Block, Short}`，error 即拒 | `ToolInterceptor.BeforeTool` → `(request', HookDecision, error)`，Decision ∈ continue/modify/**respond**/deny_tool/abort_turn/hard_abort | **部分** | 见下方逐项 |
| `ChainBefore(...)` 组合 | `HookManager` 按 Priority 快照遍历 | 部分 | picoclaw 无 hook 时**默认放行**（nil manager → continue；approver 缺席 → Approved:true）＝fail-open 形状；本仓靠装配保证链存在，迁移后必须以装配测试钉住，不能依赖默认值 |
| 六个 before 中间件 | BeforeTool 单挂点多实现 | ✅ 结构上可映射 | 每个 hook 一个 ToolInterceptor 实现，Priority 定序 |
| `AfterToolCall` 链，**error 向上传播** | `ToolInterceptor.AfterTool` → `(result', Decision, error)` | **不等价（关键）** | picoclaw 的 `callAfterTool` 对 hook 返回的 error 只 `continue`（跳过该 hook），**错误不会到达执行器**。AuditRecorder 变异要求的「审计失败必须冒泡」在原生 AfterTool 上不成立 |
| `ReviewGate` 短路到 `StatusNeedsReview` | HookActionRespond（返回结果跳过执行） | 部分 | hooks.go:30 上游自注："SECURITY: This bypasses ApproveTool checks"。Respond 会绕过 ApproveTool——若迁移后 ApproveTool 承载任何审批语义，Respond 的旁路就是缺口 |
| `ApproveTool`（本仓无独立挂点，审批在 ReviewGate 内） | `ToolApprover.ApproveTool` | 新增面 | 缺席即放行的默认值与本仓哲学相反 |

### ACORE-2 九项在新挂点上能否重新成立（D-C6 中止条件）

先数清楚：mutation_test.go 有 **8 个 Test 函数**，覆盖 **9 个变异项**——
before 6 项（TenantScope / CapabilityCheck / ProtectedMeasure / BudgetGuard /
IdempotencyGuard / ReviewGate）＋ after 3 项（AuditRecorder / ArtifactCollector /
MetricsRecorder，由 TestMutationAfterHooks 一函数三段覆盖）；
第 8 个函数 TestGovernanceAssemblyOrder 锁链序，不计变异项。**spec 的「九项」成立。**

| # | 变异项 | 新挂点上能否重新成立 | 条件/缺口 |
|---|---|---|---|
| 1 | TenantScope | ⚠️ 需适配 | HookRequest/TurnContext/HookMeta **均无 Principal 或租户字段**。适配层须在构造 request 时把 Principal 放入 wrapper 持有的旁路结构（HookMeta 加字段=就地改上游，违反规矩）。可行，但「Principal 从哪来」必须是 wrapper 的显式设计，不是顺手的 |
| 2 | CapabilityCheck | ⚠️ 需适配 | 请求只有 tool 名与 arguments，**无 Descriptor(Level/ReadOnly)**。需要 name→descriptor 查表（本仓 agenttools registry 有），经 wrapper 注入 |
| 3 | ProtectedMeasure | ✅ | MeasureResolver 是 wrapper 注入的本仓领域端口，判定逻辑全在本侧 |
| 4 | BudgetGuard | ✅ | per-turn 计数可挂 wrapper 闭包或 turnState |
| 5 | IdempotencyGuard（含 replay 短路） | ✅ | Replay store 端口注入；replay 命中映射 HookActionRespond。**注意**：Respond 绕过 ApproveTool——回放结果绕过审批是否可接受需要明确裁决（回放的是历史已批结果，直觉可接受，但要有书面结论） |
| 6 | ReviewGate | ⚠️ 需裁决 | needs_review 短路映射 Respond 可表达；但 Respond 的 bypass-ApproveTool 特性意味着「needs_review 结果不经审批挂点」。本仓 ReviewGate 本身就是审批闸，语义上自洽；裁决点是未来是否有人往 ApproveTool 里加逻辑 |
| 7 | **AuditRecorder** | ❌ **原生不成立** | AfterTool 吞掉 hook error（`!ok → continue`）。审计失败冒泡这条变异无法在原生挂点重现。唯一出路：wrapper 的 audit hook 失败时返回 `Decision{Action: AbortTurn}`——但这会把「该次调用失败」升级成「整个回合中止」，语义强于本仓现状，属于**行为改变**而非等价移植。这是 §5 中止条款的第一个真实触发候选：要么接受更强的语义并让评审签字，要么在上游 wrapper 层补错误传播通道（= 对上游行为的包装改写，ADR 允许，但要登记 divergence） |
| 8 | ArtifactCollector | ✅ | after 结果可达 wrapper sink；失败传播同 #7 问题（collector 不抛错时无影响） |
| 9 | MetricsRecorder | ✅ | 观察性质，无传播要求 |

**点名结论（§5 中止条件）**：#7 是唯一的硬伤。AR5 实施的第一件事应该是
先写「审计失败必须可见」的反向测试，然后三选一并留痕：
(a) wrapper 将 audit error 升格为 AbortTurn（语义加强，需评审签字）；
(b) wrapper 维护一个 audit-failure 标记并在回合末尾使整个 run 失败（延迟失败，
   接近本仓语义，实现复杂度中）；
(c) 上游 wrapper 补一个「after error 传播模式」开关（divergence 登记后随上游 diff 维护）。
**在没有做出选择并验证之前，不得开始搬 hooks.go 之后的东西。**

## 3. 形状 A 可行性（D-C19 / AR5-G3）

**现状判断：AgentLoop 是形状 B 的教科书实现。**

- `AgentLoop` 结构体 30+ 字段全部长生命周期持有：bus、registry、`*state.Manager`、
  fallback chain、channelManager、mediaStore、cmdRegistry、mcp、evolutionBridge、
  `steering *steeringQueue`、`workerSem`、`activeTurnStates sync.Map`、
  `pendingSkills/pendingStops sync.Map`、`mu sync.RWMutex`……
- `Run()` 不是"一次执行的循环"，是**守护进程主循环**：100ms idleTicker +
  `al.bus.InboundChan()` 消费 + per-session LoadOrStore 占位 + worker goroutine
  池（workerSem）+ activeReqCond 等待组。
- 好消息：per-turn 参数已经高度集中在 `processOptions`（20 字段的参数包，
  含 SystemPromptOverride/NoHistory/SkipInitialSteeringPoll 等本仓也需要的开关）；
  `Pipeline` 是每回合新建的（NewPipeline(al)）——但它持 `al *AgentLoop` 反向引用，
  阶段方法直接摸 al 的可变字段，状态并没有真正离开实例。
- `turnState`（916 行）挂在 `al.activeTurnStates` 上，是第三坨 per-run 状态。

**可行性结论：能做成形状 A，但不是"配置"，是"抽取"。** 两条路线：

| 路线 | 做法 | 代价 |
|---|---|---|
| A-1：实例化但不复用 Run() | 每请求新建 AgentLoop？不可行——NewAgentInstance/NewAgentLoop 的初始化绑死 config.Config 全家桶 + provider factory + evolution bridge，构造即拖入不搬的包 | ❌ 排除 |
| **A-2：只取 per-turn 核心（推荐）** | 搬 runTurn + Pipeline 四阶段方法，剥掉 `al *AgentLoop` 反向引用改为显式依赖结构体；turnState 变本次调用的局部值；bus 消费/session 序列化/并发闸留给本仓 runner（HTTP 生产路径本来就是一请求一回合，不需要 idleTicker 主循环） | 胶水重写 600–1,000 行；上游 drift 对这部分不再是免费的（ADR Consequences 已预告 divergence budget） |

A-2 下五个可变字段的去向：state→ContextKey 取持久层（AR2）；steering→调用参数
（processOptions.InitialSteeringMessages 已存在）；followUp→返回值；
cancel→ctx；running→不存在（单次执行）。AR5-G3（执行器结构无 map/channel/
mutex/可变切片）在 A-2 下自然成立，仍需测试锁定。

## 4. 汇流点

生产接线确实只有一行：`internal/handlers/ai_chat.go:169`

```go
handler.agentRuntime = aichat.NewRuntime(runtimeRepo, agent, agent, aiagent.ProjectResult, aichat.Options{...})
```

`aichat.Runtime[T]` 四个注入参数在新内核上的对应：

| 参数 | 现状 | 切换后 |
|---|---|---|
| persistence (`*repository.AIChatRuntimeRepository`) | 七张 ai_chat_* 表的持久化订阅者 | **不动**（写入时机模型保留） |
| planner (`Planner`: Plan(Input, run)) | `aiagent.Agent` 实现 | **保持本仓**（领域接线，picoclaw 无对应物）；它产出的 Plan 是工具集/技能装配决策，新内核同样需要一个"本轮允许哪些工具"的决策点，正好对接 Runtime.Describe 按权限过滤 |
| executor (`Executor[T]`: Execute(ctx, Execution) (T, error)) | `aiagent.Agent`（T 承载内部执行产物） | **换**：写成新内核的适配器——把 Execution.Input 的 Principal/ContextKey/messages 翻译成新内核的单回合调用，挂上治理链 deps 与流式回调。这是汇流的主要工作量 |
| project (`Projector[T]`) | `aiagent.ProjectResult` | T 变了它跟着重写；投影目标 Result 不变 |
| options.ReviewCommit | 提交审查事务 | 不动，继续由 ReviewGate 短路后的提交路径调用 |

**汇流的真实工作量不在这一行，在 Executor 适配器**：它是本仓 18,200 行领域侧
与新内核之间的翻译层（工具集桥接 agenttools↔picoclaw tool schema、流式事件桥
aichat run_events、审计桥既有 recorder）。估算 400–700 行 + 测试。
AR5-G1 汇流断言（机器检查新内核在生产路径上有流量）落在这张票。

## 5. 建议：拆单与预算风险

| 票 | 内容 | 边界 | 预算风险 |
|---|---|---|---|
| ~~AR5a~~ | 本分析 | — | — |
| **AR5b** | vendor 内核切片：搬 §1.2 清单 + first-party 替代包（providers types/config/logger/events 接口）+ session/routing 剥离 + 编译绿；守卫：third_party import 方向 + AR5-G3 | **不做挂点移植、不做汇流**。验收 = third_party 包编译通过 + 守卫绿 + 「未 import 业务包」扫描 | 🔴 **最高**。pipeline_llm/execute/turn_state 合计 2,600 行需要精读剥离 session/routing；上下文大概率在这张爆。对策：允许拆成 b1(types+hooks+pipeline 骨架) 与 b2(turn_state+steering+streaming) |
| **AR5c** | 治理链移植：六个 before + 三个 after 写成 ToolInterceptor/LLMInterceptor 实现；ACORE-2 九项逐项先红后绿；**#7 审计传播三选一裁决先行** | 验收 = 九项变异在新挂点上红→绿，且「测试改匹配实现」不算数 | 🟡 中止条件所在票；裁决没做之前不开工 |
| **AR5d** | 汇流：aichat.Executor 适配器 + ai_chat.go:169 切换 + AR5-G1 汇流断言（生产路径流量计数）+ 形状 A 守卫（AR5-G3 正式版） | 验收 = 断言证明生产请求经过新内核；ACORE-2 九项在**上了路径的**内核上复跑 | 🟢 低（行数少），但它是「第一次真正保护线上流量」的时刻，回归面大 |
| AR5e | steering/abort + 流式事件接 Web 层（SSE/run_events 桥） | 独立增量，可后置 | 🟢 |

**顺序硬约束**：AR5c 的 #7 裁决必须发生在 AR5b 开搬 hooks.go 之前或同时——
如果裁决结果是 (c) 改上游 wrapper，搬运时的文件头 adaptations 清单就要带上它，
事后补等于二次返工。

**一句话总结**：能做，形状 A 要求把上游的实例式架构拆成 per-call 参数包，
这是重写胶水而非搬运；治理链九项里八项可以等价移植，一项（审计失败传播）
在原生挂点上不成立，触发 ADR-0028 §5 的中止条款，必须先裁决再动手。
