# Codebase Design：Agent Runtime 完整升级 — 模块深化设计

> 编制：2026-08-23 · 状态：Current
> 前序：[Spec：Agent Runtime 完整升级（C1 批次）](specs/agent-runtime-overhaul-c1.md)（D-C0~D-C10 在那里，本文不重复，只在需要时引用）
> 同级：[CodebaseDesign_蓝图缺口补齐_模块深化.md](CodebaseDesign_蓝图缺口补齐_模块深化.md)（BG1–BG6）、[CodebaseDesign_经营工作站诚实性与能力补齐_模块深化.md](CodebaseDesign_经营工作站诚实性与能力补齐_模块深化.md)（RH1–RH8）
> 模块编号用 **AR**（Agent Runtime），决策留痕从 **D-C11** 起接续 Spec。

---

## 0. 设计原则

1. **深度看接口的杠杆。** 问「删掉它，复杂度是消失还是散到 N 个调用方去」。
2. **一个适配器 = 假想接缝，两个才是真接缝。**
3. **接口就是测试面。**
4. **把错的东西做成表达不出来。** 本文最重要的一条。上下文污染的每一种成因，都要变成「写不出那行代码」，而不是「评审时会发现」。只有用户一个人类 + 一堆 AI，靠人盯的控制项会失效。
5. **本轮零删除**（既有页面、路由、API、导航一个不动），但 `agentcore` 是例外——它是生产死代码，替换它不影响任何在跑的东西。

---

## 1. 现状诊断：这一轮要处理的浅结构

| 问题 | 形状 |
|---|---|
| 上下文管理不存在 | `State.Messages()` 原样返回全量。没有计数、预算、压缩，连观察手段都没有 |
| 会话有存储无管理者 | 生命周期散在六个文件；「谁拥有会话」没有答案 |
| 隔离键靠记得传 | `legal_entity_id` / `user_id` / `session_id` 都是 `string`，顺序写反也编译通过 |
| 内核未上生产 | `agentcore.New` 非测试代码零调用 |

第三行是本文的重心。三个同类型的字符串参数是错误发生器：`f(userID, legalEntityID)` 与 `f(legalEntityID, userID)` 都能编译，而错了之后**污染不会报错，只会安静地让模型看到别人的上下文**。

---

## 2. 接缝总览

| 模块 | 接口 | 方法数 | 新增? |
|---|---|---|---|
| AR1 ContextKey | 值类型 + 一个构造器 | 1 | **新增** |
| AR2 Session Manager | `Acquire` / `Close` | 2 | **新增**（收拢六处） |
| AR3 Context Assembler | `Assemble` | **1** | **新增** |
| AR4 Subturn Delegator | `Delegate` | 1 | **新增** |
| AR5 Kernel Adapter | `Execute` | 1 | 替换 `agentcore` |
| AR6 Memory | ContextKey 的消费者，无独立接缝 | — | 新增实现，复用 AR1 |

六个模块，接口方法合计 6 个。

---

## 3. AR1 ContextKey — 隔离键（本轮的地基）

### 问题

上下文污染的所有成因最终都归结为一件事：**取上下文时用的键不完整或不正确**。而当前所有相关标识都是裸 `string`，编译器不提供任何保护。

D-C9 要求键是 `(legal_entity_id, user_id)`。但「要求」写在文档里不构成保护——需要让不带 `legal_entity_id` 的键**在类型上无法构造**。

### 接口

```go
// ContextKey 标识「这份上下文属于谁」。
// 字段全部非导出：调用方无法拼装一个残缺的键。
type ContextKey struct {
    legalEntityID string
    userID        string
    sessionID     string
    scopeFinger   string // 见 D-C12
}

// KeyFrom 是唯一的构造器。它要求一个已解析的 Principal——
// 因此拿不到 Principal 就拿不到 Key，也就取不到任何上下文。
func KeyFrom(p agenttools.Principal, sessionID string) (ContextKey, error)

// Cache 返回缓存键。所有字段参与，由 AR1-G1 守卫保证。
func (k ContextKey) Cache() string
```

**一个类型 + 一个构造器 + 一个方法。**

### 深度理由

删掉它，每个缓存、记忆、摘要的调用点都退回成「传三个字符串」。三个同类型字符串意味着：可以漏传、可以传错顺序、可以传空串——三种错误全都编译通过，全都在运行时表现为「模型看到了不该看到的上下文」，且**没有任何一处会报错**。

它把这三类错误一次性变成编译期不可表达：

- **漏传**：字段非导出，`ContextKey{userID: x}` 在包外写不出来
- **传错顺序**：只有一个构造器，参数类型不同（`Principal` 与 `string`），错位即类型错误
- **传空的 `legal_entity_id`**：`KeyFrom` 从 `Principal.Scope` 取值，取不到就返回错误，调用方拿不到键

**为什么构造器要吃 `Principal` 而不是三个 string。** 这是本模块的全部价值所在。`Principal` 只能由权限解析器产出（与 JWT 同一条路径），因此「有 Key」这件事本身就证明了「走过了权限解析」。若允许 `KeyFrom(entityID, userID, sessionID string)`，任何地方都能凭空造一个键，模块就退化成一个三字段结构体，没有任何保护力。

### 决策留痕 D-C11

> **`ContextKey` 不实现 `String()`，只提供 `Cache()`。** 若实现 `String()`，它会被 `%v`、日志、错误信息隐式调用，把 `legal_entity_id` 与 `user_id` 撒进日志。`Cache()` 是显式调用，且返回值只用于缓存键不用于展示。需要日志时用专门的脱敏方法。

### 决策留痕 D-C12：键必须含 scope 指纹

`access.Scope` 不只有 `LegalEntityID`，还有 `Global` / `StoreIDs` / `Regions` / `Brands` / `Plants` / `ProductionLines` / `EquipmentIDs`。

**同一个 `(法人, 用户)` 的 scope 会随时间变化**——门店重新分配、区域调整、权限收回。若缓存的上下文只按 `(legal_entity_id, user_id)` 键，那么在 scope 收窄之后，这个用户自己的缓存里**仍然含有他已经不该看到的门店数据**，并且会继续被送进他的 prompt。

这是一条 Spec 未覆盖的污染向量，且它不触发任何权限检查——取缓存的确实是本人，法人也确实没变。

因此 `ContextKey` 含 `scopeFinger`：`Scope` 全字段的稳定指纹。**scope 一变，键就变，旧上下文自然不再命中**，无需显式失效逻辑。

指纹计算要求：字段顺序固定、切片先排序、空切片与 nil 同形（否则同一 scope 会算出两个指纹，缓存永不命中，退化为无缓存——不产生错误但白费）。

### 守卫 AR1-G1：全字段参与缓存键

一个反射测试遍历 `ContextKey` 的所有字段，断言每个字段的值变化都会改变 `Cache()` 的输出。

**这条守卫是为将来加字段的人写的**：日后若新增一个隔离维度而忘了放进 `Cache()`，测试立刻变红。人不会记得，测试会。

### 消费者

AR2 Session Manager、AR3 Context Assembler、AR6 Memory，以及压缩摘要缓存——**全部只接受 `ContextKey`，不接受任何裸字符串标识**。

### 守卫 AR1-G2：消费方不得旁路

架构测试断言：上述包的导出函数签名中不出现 `legalEntityID string` / `userID string` 这类参数名，取上下文的入口只有 `ContextKey`。

---

## 4. AR2 Session Manager — 会话生命周期

### 问题

七张表俱在，生命周期散在六个文件：`handlers/ai_chat.go`、`handlers/ai_chat_runtime.go`、`handlers/agent_gateway_sessions.go`、`aichat/runtime.go`、`aichat/continuation.go`、`agentrunner/runner.go`。「这个会话现在什么状态、能不能续、该不该淘汰、并发进来两条消息怎么办」没有单一回答处。

### 接口

```go
type Manager interface {
    // Acquire 取得（或创建）key 对应的会话，并持有独占租约。
    // 返回的 release 必须被调用。同一 key 的第二次 Acquire 阻塞至前者释放。
    Acquire(ctx context.Context, key ContextKey) (*Session, release func(), error)

    // Close 结算并释放会话资源。幂等。
    Close(ctx context.Context, key ContextKey) error
}
```

**两个方法。** 淘汰不在接口里——它是内部的后台行为，调用方不需要知道它存在。

### 深度理由：为什么「取会话」与「加锁」是同一个操作

分成 `Get(key)` 与 `Lock(key)` 两个方法，调用方就可以只调前者——于是「同一会话的两条消息不得交叉执行」这条要求退化成一句纪律。合并之后，**拿到会话就必然持有租约**，忘记加锁在结构上不可能。

代价是调用方必须记得 `release`。这个代价用 `defer release()` 消化，且忘记 release 会表现为**后续请求阻塞**——一个响亮、立刻可见的失败，远好过安静的交叉执行。

删掉本模块，六个文件各自实现 get-or-create、并发控制与淘汰，且必然分叉。

### 决策留痕 D-C13

> **淘汰策略不进接口。** 曾考虑暴露 `Evict(policy)` 让调用方配置。否决：淘汰是模块对自己内存的管理，调用方既不知道也不该知道有多少实例在内存里。策略经构造参数注入，不经接口。

### 存储经端口，模块不做 IO

```go
type Store interface {
    Load(ctx context.Context, key ContextKey) (*Session, error)
    Save(ctx context.Context, key ContextKey, s *Session) error
}
```

一个适配器（Postgres）+ 一个 fake（测试）= 两个，是真接缝。全部生命周期分支可以不起库测试。

### 测试

- 单元：并发 `Acquire` 同一 key 断言串行化；不同 key 断言并行；`release` 后可再取
- 单元：淘汰后再 `Acquire` 从 Store 重建，状态一致
- 集成（`make test-integration`）：跨法人——法人 A 的账号取不到法人 B 的会话，拒绝保持 `scope_denied` 不被软化

---

## 5. AR3 Context Assembler — 上下文工程

### 问题

零实现。且这是三个污染通道中最隐蔽的一个：压缩摘要是自然语言，**键错了不会有类型错误，只会安静地变成模型眼里的「事实」**。

### 接口

```go
// Assemble 产出本回合送给模型的消息序列。
// 计数、预算、压缩全在内部——调用方不选择、也无法跳过。
type Assembler interface {
    Assemble(ctx context.Context, key ContextKey, turn Turn) (Prompt, error)
}

type Prompt struct {
    Messages  []Message
    Tokens    int          // 实际计数，非估算
    Budget    int          // 该模型的预算
    Compacted bool
    Preserved []MessageRef // 审计承载内容：从不参与压缩
    Dropped   []MessageRef // 离开 prompt 的内容，仍在 ai_chat_messages
}
```

**一个方法。**

### 深度理由：为什么不暴露 `Count` / `Budget` / `Compact` 三个方法

暴露三个方法，调用方就能：不计数直接压缩（凭感觉裁剪）、用错分词器计数（预算在边界处系统性失效）、跳过压缩直接发（撞上限）。三者的正确顺序是 D-C2 的内容，把它交给调用方等于把规格交给纪律。

一个方法之后，顺序是实现细节，调用方**没有表达错误顺序的语法**。

**接口薄而返回值厚**是这里的关键手法。测试需要观察「压缩发生了没有、丢了什么、留了什么」，这些经 `Prompt` 的字段回答，而不是靠增加接口方法。接口深度与可测性因此不冲突。

### 决策留痕 D-C14：审计承载内容通过「不给它看见」来保护

D-C3 要求压缩永不丢弃工具调用、参数、结果、Artifact 引用、审批动作与权限结论。

**实现方式不是在压缩器里写 `if isAuditBearing { skip }`**——那是一条可以被后来者改错的规则。而是：

```go
// classify 是纯函数，按消息类型切成两段。
func classify(msgs []Message) (preserved, compactable []Message)
```

压缩器的签名**只接受 `compactable`**：

```go
func compact(compactable []Message, budget int) ([]Message, []MessageRef)
```

它在类型上拿不到审计内容，因此**不可能丢弃它**。规则从「压缩器要记得跳过」变成「压缩器看不见」。

守卫 AR3-G1：反向测试——构造一个含工具调用与 Artifact 引用的超预算会话，断言压缩后这些内容一条不少；再把 `classify` 的分类改错，该测试必须变红。

### 决策留痕 D-C15：分词器按模型选择，且缺失时拒绝而非估算

计数必须与实际所用模型的分词器一致。**分词器不可得时返回错误，不退化为字符数估算。**

估算的误差在预算边界处最大，而边界正是唯一重要的地方——估算低了就撞上限（本要解决的问题原样保留），估算高了就过度压缩（无谓丢上下文）。一个会在关键处失效的近似值比没有更糟，因为它看起来在工作。

这与「不用 0 填补缺失」「缺失是 nil 不反推」是同一条纪律。

### 决策留痕 D-C16：压缩是呈现层裁剪，`Dropped` 是它的证据

`Dropped` 里的 `MessageRef` 必须能解析回 `ai_chat_messages` 的实际行。这不只是调试信息——它是「压缩没有删记录」这一断言的可执行形式。

测试：压缩后逐个解析 `Dropped`，断言全部命中原表。

### 内部接缝

`tokenizer`（按模型，两个以上实现 → 真接缝）、`budget`（按模型的配置查询）、`compact`（策略）。三者均为私有，本模块自己的测试直接测——刻度边界与压缩策略的用例太多，经 `Assemble` 测太绕。它们不出现在接口里。

### 与 AR6 Memory 的关系

记忆是 `Assemble` 的一个输入源，不是独立接口。它按同一个 `ContextKey` 取——**因此 D-C12 的 scope 指纹自动覆盖记忆**，无需为记忆单独设计失效逻辑。这是把键做成类型的复利。

---

## 6. AR4 Subturn Delegator — 子任务委派

### 接口

```go
type Delegator interface {
    Delegate(ctx context.Context, parent TurnRef, tasks []Task) ([]Result, error)
}
```

**一个方法，收列表。** 与 BG3 的 `Decide` 同理：收列表让「并发是常态」成为接口事实，避免调用方写 `for` 循环从而把深度限制、并发上限与审计链接散出去。

### 深度理由

深度上限、并发上限、每个子回合独立走治理链、scope 收窄、审计链接到父回合——全在实现里。调用方只提交任务列表。

删掉它，每个想派生子任务的地方都要自己算深度、自己限并发、自己接治理链——而漏接治理链的那一处就是绕过审批的通道。

### 决策留痕 D-C17：子 Principal 只能派生，不能构造

```go
// narrow 返回一个可证明不宽于 parent 的 Principal。
// 非导出：包外无法产生子 Principal。
func narrow(parent agenttools.Principal, req ScopeRequest) (agenttools.Principal, error)
```

实现是**求交**，不是替换：子的每个 scope 维度取父与请求的交集。因此结构上不可能扩权——`narrow` 没有能让结果变宽的代码路径。

守卫 AR4-G1：架构测试断言本包内不出现 `access.Scope{...}` 或 `agenttools.Principal{...}` 字面量构造，子身份只能经 `narrow` 产生。这与 ADR-0027 §5 对渠道层的守卫同构。

### 决策留痕 D-C18：子回合不继承父回合的放行结论

父回合过了 Review Gate，不代表子回合的工具调用免检。每个子回合的每次调用独立走完整治理链。

反向测试：构造一个子回合调用父回合 scope 外的数据，断言被拒且原因是 `scope_denied`（不被软化）；构造超过深度上限的委派，断言被拒。

### 上下文边界

子回合的上下文**不是父回合上下文的副本**，而是父回合显式传入的任务描述加子回合自己的历史。默认不继承父的完整对话——那是同一账号内的跨话题污染，也会让子回合的 token 预算被父回合的历史吃光。

---

## 7. AR5 Kernel Adapter — 内核置换与汇流

### 问题

`agentcore` 是死代码，生产跑的是 `aichat.Runtime[T]`。C1 第一层的难点是汇流，不是置换。

### 接口

```go
// Executor 是生产执行的唯一接缝。
// 迁移期两个实现并存，但同一时刻只有一个被接线。
type Executor interface {
    Execute(ctx context.Context, key ContextKey, prompt Prompt) (Outcome, error)
}
```

**一个方法。** 治理链、工具执行、流式事件、steering 全在实现内。

两个适配器（新内核 / 既有 `aichat.Runtime` 兜底）= 真接缝，且让迁移可以逐步切换与回退（D-C10 分层可回退）。

### 守卫 AR5-G1：汇流必须可被断言

一个生产路径测试断言：chat 入口实际经过新内核实现，而非 `aichat.Runtime[T]`。

**G1 之所以能悬而未决这么久，正是因为从来没有这样一条断言。** 「内核建好了」与「内核在跑」之间的差距，只有机器检查得出来——`agentcore.New` 零调用这件事，靠读代码几个月都没人发现。

### 守卫 AR5-G2：ACORE-2 九项在新挂点上重新成立

九项变异逐项先证红再证绿，一项不少。任一项无法在新挂点上重新成立，迁移中止而不是带着削弱的链继续（D-C6）。

**「测试改成匹配新实现所以绿了」不算通过。** 判据是：删掉对应的治理中间件，这条测试会不会红？

### 决策留痕 D-C19：隔离形状二选一，禁止第三种

按 D-C9，只承认：

- **A. 无状态共享**——实例不持有任何 per-run / per-account 可变状态
- **B. 每账号独立实例**——按 `ContextKey` 的 `(legalEntityID, userID)` 部分键隔离，且带淘汰策略

**倾向 A。** `agentcore.Agent` 现有五个可变字段（`state` / `steering` / `followUp` / `cancel` / `running`），picoclaw 的 `pkg/agent` 有 `instance.go`——适配时要把这些状态推到调用参数里，而不是留在实例上。

守卫 AR5-G3：若选 A，架构测试断言共享实例的结构体无 map / channel / mutex / 可变切片字段。若选 B，并发测试断言两个账号的 history 与 steering 互不可见，且模块设计里要写明 A 为何不够。

---

## 8. 交付顺序与依赖

| 顺序 | 模块 | 阻塞项 |
|---|---|---|
| 1 | **AR1 ContextKey** | 无。**必须最先做**——AR2/AR3/AR4/AR6 全部依赖它 |
| 2 | AR5 内核置换 + 汇流 | ACORE-2 九项重锁；AR5-G1 汇流断言 |
| 3 | AR2 Session Manager · AR3 Context Assembler | 依赖 AR1 与 AR5；两者可并行 |
| 4 | AR4 Subturn · AR6 Memory · MCP · 路由 · 定时 · 健康探针 | 依赖 AR3（都要经 `Assemble`） |

**AR1 先做的理由**：它是唯一一个「做晚了要返工全部调用点」的模块。先把键做成类型，后面每个模块自然带着保护长出来；后做则每个模块都会先长出裸字符串签名，再逐个改回去。

---

## 9. 本文与 Spec 的差异登记

| 项 | Spec | 本文 | 原因 |
|---|---|---|---|
| 隔离键 | `(legal_entity_id, user_id)` | **加 scope 指纹**（D-C12） | `access.Scope` 有七个维度且会随时间变化；scope 收窄后旧缓存仍含已不可见的门店数据，且不触发任何权限检查 |
| 审计内容保护 | 「永不参与压缩」 | 改为**压缩器在类型上看不见**（D-C14） | 规则可以被改错，类型不会 |
| 分词器缺失 | 未规定 | **拒绝，不估算**（D-C15） | 近似值在预算边界处系统性失效，而边界是唯一重要的地方 |

Spec 的 D-C9 应按 D-C12 补上 scope 指纹一条。
