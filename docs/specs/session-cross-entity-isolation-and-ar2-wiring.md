# 工单：跨法人会话续接的隔离缺口 + AR2 接线（SI1）

> 编制：2026-08-24 · 状态：`ready-for-agent`
> 来源：为 G9（sessionmanager 接线，到期日 2026-09-30）做准备时的实测发现
> 相关：[母 spec D-C4/D-C9](agent-runtime-overhaul-c1.md) · [AI 文档索引 G9](../AI_文档索引与现行决策.md)

**两部分，A 先行且可独立交付、独立回退。** A 是隔离缺口，B 是接线重构。**不要合并成一个提交**——A 要是被 B 的问题连累回滚，那就是拿底线换整洁。

---

## Part A：会话续接不校验法人（先做）

### 实测事实

`repository/ai_chat_runtime.go:240` 的 `GetSessionByID(ctx, sessionID, userID)`：

```sql
WHERE id = $1 AND user_id = $2
```

**只按 `user_id` 过滤，不带 `legal_entity_id`。**

而 `legal_entity_id` 在 `aichat/runtime.go` 只在**创建**时写入（第 78-79 行 `OpenSession`、第 168-169 行 `prepare`），**加载既有会话后全仓没有任何一处把它与请求上下文的法人做比较**（我搜过 `aichat/runtime.go` 与 `continuation.go` 的全部 `LegalEntityID` 命中，只有这两处写入）。

### 后果

**同一个自然人可以横跨多个法人**（集团 FP&A、大区 BP 都是常态，D-C9 的原话）。这样的用户在法人 B 的上下文里带上一个属于法人 A 的 session id，会话照常续接，法人 A 的对话历史继续参与本次回合。

工具调用取数**仍然是安全的**——scope 来自 JWT 而不是会话行。但这正是 D-C9 拆开的两条轴：

| 轴 | 问题 | 现状 |
|---|---|---|
| 数据隔离 | 工具调用能取到谁的数据 | 已由 scope 接线处理 |
| **上下文污染** | **谁的消息进了 prompt** | **本单要修的就是这条** |

值得注意的是：**两个新模块都做对了**——`sessionmanager.PostgresStore.Load` 与 `contextassembler.PgHistorySource.Read` 都比对法人与用户、都以 `scope_denied` 拒绝且不软化。做错的是旧路径。

### 要做的

1. **先证红。** 写出「属于 (用户 U, 法人 A) 的会话，在法人 B 的上下文里被加载时必须拒绝、且拒绝原因是 `scope_denied`」的测试。**这条测试今天写出来就该是红的**（因为今天不拒绝），修完变绿——这就是「先证红」。

   **不要为了让它今天变绿而去断言当前的泄漏行为**：那种测试修完还得再改一次，而「改测试让它绿」是本仓最忌讳的形状。

   **测试不要写在 repository 那一层。** `GetSessionByID` 不收 entity，那一层根本表达不了「调用方的法人」，会逼着你先改签名才能写测试——顺序就反了。写在法人现成可得的那一层（handler / 会话续接路径，`middleware.GetTenantID`）：种一个 (U, 法人 A) 的会话 → 用 U 的身份但上下文法人为 B 走续接 → 断言拒绝。这样**不动任何签名就能编译，且今天就是红的**。签名怎么改由测试倒逼出来。

   跨法人证据按既有惯例进集成测试（`make test-integration`，skip 不算证据）。
2. **加载路径比对法人**，语义照 `sessionmanager.PostgresStore.Load` 那份（它已经是正确实现，别另发明一套）：归属不符以 `scope_denied` 拒绝，**不得软化成 not_found 或空结果**。
3. **NULL 法人的存量行与全局管理员**这两种情况要一并裁决。注意 `governance.identityComplete` 有一条相关的既有结论：生产 chat 的全局管理员携带空租户 id，旧链放行，AR5d 时特意保留了。**别把管理员锁在门外**，也别为了放行管理员把校验做成形同虚设——**这两条怎么兼顾，动手前把方案抛回来**。

### Part A 验收

- 缺口测试：修前绿（缺口在）、修后红→绿；把法人比对删掉必须再次变红
- 拒绝保持 `scope_denied`，不被软化
- 全局管理员与 NULL 法人存量行的行为有专门测试钉住
- `make test-integration` 实跑非 SKIP

---

## Part B：AR2 接线（G9，到期日 2026-09-30）

Part A 通过后再开工。**本单只接 chat 平面**；gateway 与 runner 平面继续挂在 G9 下，不在本单范围。

### 为什么这件事拖不得

`sessionmanager` 至今零生产调用方。母 spec 开篇痛批 `agentcore` 的正是这个形状——建成、有测试、没人调用。区别只在于当时没人给它设到期日。

### 动手前必须先回答的三个问题

1. **两条创建路径要不要收敛。** `aichat/runtime.go` 有两处 `CreateSession`：`OpenSession`（第 70 行，标题来自 command）与 `prepare`（第 164 行，标题由首条消息摘要）。两者写入的字段不同。**不收敛就谈不上「单一所有者」**，收敛则要确认标题语义谁说了算。
2. **`sessionmanager.Session` 的字段面不够。** 它只镜像了 entity/user/session/classification/title/status/时间戳，而 `repository.AIChatSession` 还有 `BoundContractID` / `ContextSnapshot` / `Initiator` / `LastMessageAt` / `ArchivedAt`。哪些搬到模块后面、哪些留在既有 repository，要有明确的划线理由，不要「先搬一半看看」。
3. **并发语义是一次真实的行为变更。** `Acquire` 持独占租约，意味着**同一会话的两条消息将串行而不再交叉**。这是 D-C4 与 Story 13 的既定意图，是对的——但它改变的是生产行为，不是内部重构。要有测试证明串行确实发生，并确认前端在等待期的表现可接受（超时、并发提交两条消息）。

### Part B 验收

- 生产 chat 的会话创建与加载确实经过 `sessionmanager`（照 AR5-G1 的做法加机器断言，**并配反向对照**——把接线退回去必须变红，否则断言恒真）
- 同一会话并发两条消息，断言不交叉执行
- 既有 chat 行为无回归；`OpenSession` 与 `prepare` 两条路径的收敛结果有测试
- G9 在 `docs/AI_文档索引与现行决策.md` 中的状态随之更新

---

## 顺带清理（Part A 或 B 任一单捎带即可）

`sessionmanager/store_postgres.go` 第 56-63 行附近，`Save` 的文档注释块**重复出现了两次**（连续两段一模一样的 "Save inserts or updates the module-owned columns..."）。
