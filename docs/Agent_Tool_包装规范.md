# Agent Tool 包装规范

> 编制：2026-08-23 · 状态：Current
> 适用：给 Agent 新增一个 Tool 时照此写。Batch B 之后还有十几张单要加工具，这份文档就是为了不必在每张单里重复这些约束。
> 上位约束：[AGENTS.md](../AGENTS.md) 的「AI Agent 约束」与「五条不可突破的底线」；[ADR-0019](adr/0019-agent-tool-runtime-policy-and-threat-model.md) 的 Tool Runtime 政策与威胁模型。

---

## 0. 为什么是包装 Tool，而不是让模型写 SQL

三个后果，按严重性排：

**脏写破坏正式台账。** 模型可能 UPDATE 一张已关账的表。Tool 层能做到写口在接口上不存在，模型即使想写也没有可调的方法。

**越权。** 裸 SQL 无法保证 `WHERE legal_entity_id = ?` 一定被带上。Tool 层从 `Principal` 取 scope，取不到就拒绝，这是底线 1 的落点。

**算错数。** IFRS 16 折现、连环替代归因、T1–T16 勾稽，模型算不准。这些留在 Go 里逐格锁定，模型只负责识别意图和组织叙述。

模型的职责因此收窄成：**读懂用户想干什么，选对工具，填对参数**。取数、计算、拦截 100% 由 Go 执行。

## 1. Tool 与 HTTP 是并列入口，不是「把 API 包装一层」

```
LLM ──► Runtime.Execute ──► 治理链 ──► Tool Handler ─┐
                                                      ├──► 同一个窄接口 ──► handler / repository / service
浏览器 ──► gin 路由 ──► permission 中间件 ──────────┘
```

**Tool 不发 HTTP 请求。** 全仓 `internal/agenttools/tools/` 里 `net/http` 零命中。工具依赖一组窄读接口，由 HTTP 那侧同样在用的 handler 或 repository 满足。`fpna.store_pnl.read` 用的是 `NewStorePnlAgentReader(storePnlHandler)`，**同一个 handler 对象**，不是同一个 URL。

三个后果：

- 没有序列化、没有二次鉴权、没有网络失败模式
- **接缝比 API 面窄，这是安全属性不是巧合**（见 §4）
- **新增 API 不会自动产生 Tool**。`/reports` 曾有 25 条路由对 1 个工具，`storepnl` 引擎完整但零工具。两个入口各自建，所以各自漏

## 2. 一个 Tool 的实际形状

不是「四方法接口」。是一个结构体：

```go
// internal/agenttools/registry.go
type ToolDefinition struct {
    Descriptor ToolDescriptor
    SkillIDs   []string
    Handler    ToolHandler
}
```

`Descriptor` 承载治理链的全部输入。治理**不在 Handler 里写**，是 Runtime 读 Descriptor 后统一施加的：

```go
type ToolDescriptor struct {
    Name                string          // 见 §3 命名
    Version             string          // "v1"
    DisplayName         string          // 人看的名字
    Description         string          // 模型靠它做意图路由，见 §6
    Level               ToolLevel       // LevelRead / LevelDraft / LevelCommand
    ReadOnly            bool
    Permissions         []Permission    // 必填，见 §5
    InputSchema         json.RawMessage // 必须是合法 JSON 对象，见 §8
    OutputSchema        json.RawMessage
    Review              ReviewPolicy    // LevelDraft 必须要求人工确认
    Retry               RetryPolicy
    SupportsDryRun      bool
    SupportsIdempotency bool
    MaxRows             int
    TimeoutSeconds      int
}
```

把这些放进 Descriptor 而不是 Handler，是因为它们要在**调用发生之前**被读取：权限不足的工具连列都不该列给模型（`Runtime.Describe` 会按 `Principal` 过滤）。

## 3. 命名

只有三个顶层命名空间：`lease.*` / `fpna.*` / `retail.*`。**不得开第四个根**，新领域用二级段表达（`fpna.store_pnl.read`，不是 `store_pnl.read`；`retail.charts.waterfall.render`，不是 `render_waterfall_svg`）。

**归哪个命名空间，看工具读写的数据属于哪个域，不看哪个页面调它。** 2026-08-23 盘点出 15 个错位工具，共同成因就是按调用方归类：`lease.store.performance` 读的是 store-day 经营事实，`lease.fpna.action.draft.create` 写的是 FP&A 行动表。

动词收在末段并保持一致：`.read` / `.preview` / `.evaluate` / `.simulate` / `.generate` / `.draft.create`。

## 4. 接缝必须比 HTTP 面窄

这是「绝对的数据安全」的实现方式，不是自动继承来的。

月结有 18 条 HTTP 路由，含 `generate` / `approve` / `post` / `lock` / `unlock`。Tool 侧的接口是：

```go
type MonthlyClosingReader interface {
    GetBatches(...)
    ListJournalEntries(...)
    ListEntryPeriods(...)
    IsPeriodLocked(...)
}
```

四个读方法。写口**不在接口里**，所以「Agent 不能过账」不是「没有调用写方法」，是**没有写方法可调**。如果按「一套 API 两处挂」做，写口会连带进来，然后只能靠纪律不去调。

**写这一步时问自己**：这个接口里有没有一个方法，是我不希望模型在任何情况下调用的？有就删掉，别留着靠 Descriptor 拦。

## 5. 权限与租户

每个 Descriptor **必须**声明 `Permissions`，缺失会被 `Descriptor.Validate` 以 `at least one permission is required` 拒绝。而注册失败一度是静默的——`lease.file.triage` 因此从未进过 registry。

取值按**工具自身的读写性质**，不按下游对象：`lease.file.triage` 只读零写入用 `ai_chat:use`，不用 `lease.file.parse_*` 的 `contracts:create`，否则「还不知道是不是合同」的分诊会被挡在合同创建权之后。

租户上下文的**唯一正确写法**：

```go
execution, err := agenttools.RequireExecutionContext(ctx)
if err != nil {
    return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
}
legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
if legalEntityID == "" {
    return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
}
```

**禁止 `ctx.Value("legal_entity_id").(string)`。** 全仓零命中，别开这个头。三个问题：字符串键跨包冲突；`.(string)` 没有 comma-ok，值不存在时直接 panic；最要紧的是它绕过 `Principal`，而 `Principal` 是唯一由权限解析器产出的东西——绕过它就是第二条权限判定路径。

`Scope` 不只有 `LegalEntityID`，还有 `Global` / `StoreIDs` / `Regions` / `Brands` / `Plants` / `ProductionLines` / `EquipmentIDs`。只取法人会漏掉门店级授权。

### 5.1 别把「取数隔离」和「上下文隔离」混成一件事

两把钥匙，回答两个不同的问题，不能互相替代：

| | 问题 | 键 | 现状 |
|---|---|---|---|
| **工具取数** | 这次调用能读到哪些行 | `Principal.Scope`（法人 + 门店 + 区域 + 品牌…） | 已实现，底线 1 |
| **上下文装配** | 谁的消息进 prompt | `ContextKey`（法人 + 用户 + 会话 + scope 指纹 + 数据分类） | C1 批次的 AR1，尚未实现 |

**写 Tool 时只用第一把。** 把取数也改成按账户键会破坏底线 1：同一法人下两个用户的门店授权不同，按账户拿不到这个区分，按 scope 才能。账户身份不替代授权范围。

第二把钥匙作用在**结果回到模型之前**，不在取数环节，因此不出现在 Tool Handler 里。它防的是另一类失败：访问控制管工具调用能读什么，管不着模型**已经看见了谁的消息**。设计见 [Spec C1](specs/agent-runtime-overhaul-c1.md) D-C9 与[模块深化](CodebaseDesign_AgentRuntime升级_模块深化.md) AR1。

## 6. Description 是给模型读的，不是给人读的

模型靠它做意图路由，所以它要回答「什么时候该用我」和「我的口径是什么」，而不是复述函数名。

写进去的应包括：适用场景、口径声明、降级语义。例：

> 按期间预览租赁会计分录及审批/过账状态。Draft 口径（report_basis=draft）：最低包含到 Draft，即 Draft + Pending + Approved 全含，响应声明 is_official_version=false。Approved 口径仅含已批准分录。只读：不改变任何分录的审批或过账状态。

> 按 retail-kpi-v1 从 store-day 事实聚合 KPI。严格 null 语义：缺失 KPI 的 value 是 nil 不是 0；覆盖不足时 decision_ready=false 并给原因；来源冲突返回 source_conflict。

口径写进 Description，模型才不会把「不可用」讲成「是 0」。

## 7. 返回：诚实拒绝，不产出数字

端口未接线时返回 unavailable，**不产出数字**：

```go
if reader == nil {
    return rejected(call.CallID, agenttools.ErrorDataUnavailable, "store pnl reader unavailable"), nil
}
```

注意返回的是 `(ToolResult, nil)` 而不是 `(_, err)`——业务性拒绝是一个**结果**，不是 Go 错误。

可用错误码：`ErrorInvalidArguments` / `ErrorUnauthenticated` / `ErrorPermissionDenied` / `ErrorScopeDenied` / `ErrorReviewRequired` / `ErrorCapabilityDenied` / `ErrorBusinessFailure` / `ErrorSystemFailure` / `ErrorTimeout` / `ErrorCancelled` / `ErrorNotFound`。

**`scope_denied` 不得被软化**成「无数据」之类。软化会掩盖权限问题，触及底线 1。

**跨租户不得泄漏存在性**：异法人的对象与根本不存在的对象，对外应完全同形（同一错误码、同一文案）。否则模型可以用「拒绝的措辞不同」枚举出别的法人有哪些门店。

## 8. 注册

在 `internal/aiagent/agent.go` 的相应分支加一行：

```go
collector.add(agenttooldefs.NewXxxDefinition(reader))
```

**端口为 nil 时不要无条件注册。** 生产分支注册真实端口，无仓库时才注册 nil 版（工具诚实拒绝）。反过来会让 nil 注册把真实端口挡在外面（P0-8 教训，`agent.go` 现按 `finModelRepo == nil` 二选一分支）。

注册失败现在会 **fail-fast**（`registerCollector`，panic 带工具名与原始错误）。历史上有两个工具因静默丢错而从未进过 registry：`fpna.assumptions.suggest`（InputSchema 多一个右花括号）与 `lease.file.triage`（未声明 Permissions），两者各自带着绿测试——**单元测试直接调 Handler 不经注册，证明不了工具可达**。

`InputSchema` / `OutputSchema` 必须是合法 JSON 对象，`TestSchemaLiteralsAreValidJSON`（TOOL-001）在源码层校验本包所有字面量。

## 9. 验收清单

- [ ] `Permissions` 已声明，取值按工具自身读写性质
- [ ] 租户从 `Principal.Scope` 取，缺失即 `scope_denied`；无 `ctx.Value` 字符串键
- [ ] 接缝里没有任何我不希望模型调用的方法
- [ ] 端口不可得返回 unavailable，有测试
- [ ] 跨法人隔离测试，**用 `make test-integration` 实跑**（skip 掉的集成测试不构成证据）
- [ ] 异法人与不存在对外同形，无存在性泄漏
- [ ] Description 写了适用场景与口径，不是复述函数名
- [ ] 运行时枚举确认工具真的在 registry 里（不是「代码写了」）
- [ ] `cd core-service && GOCACHE=$(pwd)/.gocache go test ./... && go vet ./...`

## 10. 可以照抄的模板

```go
type XxxReader interface {
    // 只放读方法。写方法一旦出现在这里，模型就能调到它。
    ListXxx(ctx context.Context, entity access.EntityFilter, period string) ([]*repository.Xxx, error)
}

func NewXxxReadDefinition(reader XxxReader) agenttools.ToolDefinition {
    return agenttools.ToolDefinition{
        Descriptor: agenttools.ToolDescriptor{
            Name: "retail.xxx.read", Version: "v1",
            DisplayName: "读取 Xxx",
            Description: "适用场景……。口径：……。缺失语义：缺 X 时返回 nil 不是 0。只读：不改变任何状态。",
            Level:       agenttools.LevelRead,
            ReadOnly:    true,
            Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
            InputSchema: json.RawMessage(`{
                "type":"object","additionalProperties":false,
                "required":["period"],
                "properties":{"period":{"type":"string","description":"YYYY-MM"}}
            }`),
            SupportsDryRun: true,
            MaxRows:        2000,
            TimeoutSeconds: 20,
        },
        SkillIDs: []string{"retail_operations"},
        Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
            if reader == nil {
                return rejected(call.CallID, agenttools.ErrorDataUnavailable, "xxx reader unavailable"), nil
            }
            execution, err := agenttools.RequireExecutionContext(ctx)
            if err != nil {
                return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
            }
            legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
            if legalEntityID == "" {
                return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
            }
            args, err := decodeXxxArgs(call.Arguments)
            if err != nil {
                return rejected(call.CallID, agenttools.ErrorInvalidArguments, err.Error()), nil
            }
            rows, err := reader.ListXxx(ctx, execution.Principal.Scope.EntityFilter(), args.Period)
            if err != nil {
                return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load xxx"), nil
            }
            return agenttools.ToolResult{
                CallID: call.CallID,
                Status: agenttools.StatusCompleted,
                Data:   shapeXxx(rows), // 带上来源信封：data_classification / source_system / as_of_at / version
            }, nil
        },
    }
}
```

## 附：写类工具的额外约束

- **一律只写 draft 层**，绝不碰正式台账与 IFRS 16 正式表（底线 5）
- `Level: LevelDraft` + `Review: ReviewPolicy{Required: true, ...}`，人工确认后才流转
- 底稿类（`*.working_paper.*.generate`）产出 Artifact，不落业务表
- approved-only 的读取路径**永不回采 draft**，这条有测试锁定
- 零售侧只给只读 Tool
