# 工单：C1 批次评审整改（AF1–AF4）

> 编制：2026-08-24 · 状态：`ready-for-agent`
> 来源：对 `bcdb8a4..80222b5`（AR1 / AR5b / AR5c / AR5d / AR2 / AR3）的评审
> 母 spec：[agent-runtime-overhaul-c1.md](agent-runtime-overhaul-c1.md)
> 评审结论：测试全绿属实，九项变异在生产装配下的重跑与 AR5-G1 汇流断言（含反向对照）是这批最有价值的产物。以下四单是绿灯没覆盖到的地方。

**优先级与顺序**：AF1 三条必须一起完成——在它们落地之前 `CONTEXT_ASSEMBLER_ENABLED` 保持关闭，AR3 接线在开 flag 后不可用。AF2/AF3/AF4 可与 AF1 并行。

**贯穿要求**：每一条的自检句照旧——**把修好的逻辑改回错的，新测试必须变红**。用与本工单不同的输入独立复验，不要只让既有测试重新通过。

---

## AF1：AR3 计数与接线的三个缺陷（开 flag 即触发）

三条是同一件事的三个面，合并为一单交付。

### AF1-a：把「每轮 prompt 总量」当成「每条消息 token 数」累加

**现状**。`ai_chat_messages.measured_tokens` 存的是 `llm.UsageMetadata.InputTokens`，即 `prompt_tokens`——**整轮 prompt 的总量**（`internal/llm/usage.go:40`）。而 `contextassembler/impl.go` 的 `countMessages`（:178）与 `splitCount`（:192）把每一行的该值**逐条相加**。

**复现**（评审探针，3 轮对话，各轮实测 prompt 为 1000 / 1100 / 1200）：

```
counted=3303 ; true prompt size of the next round is ~1200
```

误差随轮数二次增长。

**后果**。压缩比应有时机提前很多轮触发、无谓砍历史；且 D-C2 立项要建的「可观测的 token 用量指标」本身是错的——`context_compacted` 事件里的 `tokens` / `estimated_tokens` 同样是虚数。这条直接反噬 spec 的 User Story 6/7。

**要做的**。按 `assembler.go` 包注释里**已经写对**的那个设计实现：**取最近一轮的实测 usage 作为基线，只对基线之后未发送过的尾部消息估算**，而不是 sum。若选择改为存每条消息的增量而非轮总量，则迁移 063 的列语义与注释必须同步订正——两种都可以，但存的语义和读的语义必须是同一个。

**为什么单测没抓到**。`assembler_test.go` 给 `MeasuredTokens` 的值是 15/80/6 这种**逐条**语义，与生产写入的**轮总量**语义不是一回事。整改必须补一条以生产写入语义为输入的测试。

### AF1-b：当前用户消息被发给 LLM 三次

**复现**（评审探针，真 assembler + 真 `buildLLMMessages`）：

```
current user message appears 3 times in the LLM request body
```

**三条来源**：

1. `aichat/runtime.go:223` 在 `execute()` **之前**就把 trigger message 落库，`PgHistorySource.Read` 会读回来；
2. `aiagent/agent.go` 的 `assembleTurnHistory` 又把 `req.Message` 放进 `Turn.Messages`；
3. `buildLLMMessages`（`agent.go:2343`）在 history 之后再 append 一次 `userMessage`。

**为什么既有测试漏掉**。`ar3_wiring_test.go` 的 `stubAssembler` 返回的 canned Prompt **不含**当前消息，而真 `Assemble`（`impl.go:114`）返回 `history + turn.Messages`，一定含。stub 的形状与实现的形状不一致，那条测试证不了它想证的东西。

**要做的**。定清楚「当前消息由谁负责放进 wire」这一条契约并落到一处；补一条**用真 assembler 断言最终 wire 消息序列**的测试（stub 只能证接线，证不了这个）。

### AF1-c：模型名不在硬编码预算表里就全线断聊

**现状**。`cmd/api/main.go` 的 `contextBudgetSpecs()` 只登记 `deepseek-v4-flash` 与 `gpt-4o`。而 `.env.example:41-44` 自己列了 `deepseek-v4-pro` / `deepseek-chat` / `deepseek-reasoner` 都是合法的 `DEEPSEEK_MODEL`。

跑其中任一个 + `CONTEXT_ASSEMBLER_ENABLED=true` → 每一轮 `ErrBudgetUnconfigured` → 用户看到「AI agent context assembly failed」。

`CONTEXT_BUDGET_WINDOW_TOKENS` 救不了：它只改**已存在的两个条目**的 window，不会为实际在跑的模型注册条目。

**要做的**。让「实际在跑的模型」一定有预算几何。ErrBudgetUnconfigured 的响亮失败语义**保留**（窗口大小是配置不是猜测，这条没错），但启动时就该发现，而不是让第一个用户发现——倾向于在进程接线处对 `llm.Config().Model` 做启动期校验，缺配置即拒绝启动（与 `collector.fail()` 的 fail-fast 同款纪律）。

### AF1 验收

- 三条各自的反向变异证红证绿
- 补一条真 assembler 的 wire 序列断言（覆盖 AF1-b）
- 补一条以生产写入语义（轮总量）为输入的计数断言（覆盖 AF1-a）
- 启动期校验的反向测试：配一个未登记的模型名，断言进程拒绝启动并报出模型名（覆盖 AF1-c）
- `make test-integration` 实跑非 SKIP

---

## AF2：`sessionmanager.Close` 数据竞争

**复现**（32 goroutine 交替 `Acquire`/`Close`，`-race -count=20`）：

```
WARNING: DATA RACE
Write at ... manager.go:269   Previous read at ... manager.go:252
Write at ... manager.go:162   Previous read at ... manager.go:252
```

`manager.go:252` 在 `m.mu` 之外读 `entry.closing` 与 `entry.refs`，与 `:269` 的 `entry.closing = true`、`:162` 的 `entry.refs--` 竞争。

**注意**。commit 说的「-race 干净」是真的，但空转——AR2 的测试从没并发调用过两个 `Close`。这个模块存在的全部理由就是并发所有权，测试面上正好缺这一格。整改必须**先补上并发 Close 的测试并看到它红**，再修。

**验收**：`-race -count=200 -cpu=8` 干净；新测试在把锁去掉后必须复现红。

---

## AF3：治理链的两个休眠缺陷

### AF3-a：`CapabilityCheck.DescriptorFor` 是 nil 陷阱

`governance.go` 的 `Assembly`（:479）不设该字段，`Deps` 里也没有它。一旦 `facts.Descriptor.Name == ""` 走到 `:197`，即 nil func 调用 panic。当前接线下 facts 恒带 registry descriptor，够不着——但它是死字段加活雷。要么接上并测，要么删掉那条分支。

### AF3-b：`deriveShortCircuit` 与链上顺序不一致

`chatexec.go:297` 先判 ReviewGate 再判 replay，而链上 `IdempotencyGuard`（priority 50）排在 `ReviewGate`（60）**之前**。两者同时成立时链决定 replay、guard 返回 needs_review。生产 `Replay` 为 nil 所以休眠，接上 replay store 那天就是错的。

要做的：让 `deriveShortCircuit` 的判定顺序与 `Assembly` 的挂载顺序同源，不要各写一份。补一条「replay 命中且该工具同时要求 review」的测试。

---

## AF4：AR3 裁剪判据里工具定义算两遍

`impl.go:131` 的 `floor` 已含 `EstimateToolDefs`，`impl.go:138` 的 `countKept` 又加一次。

**复现**（预算 130、真实占用 130）：`dropped=2, want 1`，多砍一轮。

今天生产没炸只因为 `assembleTurnHistory` 压根不传 `ToolDefs`——**而那本身是第二个洞**：线上那批工具的 schema 完全不进预算。两件一起修：去掉重复计数，并把真实的 `ToolDefs` 接进 `Turn`。

顺带清理（同一单）：

- `budget.go` 文件头仍写着本模块「refuses to count without an exact tokenizer (D-C15); see `ErrTokenizerUnavailable`」——该 error 不存在，该决策已被 2026-08-24 的「学 pi」裁决推翻。注释是错的文档债。
- `chatexec.Executor.now` 字段构造后从未被读。
- `agentkernel/importguard_test.go` 里的 `strings.Repeat("", 0)` 与手写 `fmtInt`（`strconv.Itoa` 即可）。

---

## 不在本工单内，但已登记

- **`sessionmanager` 零调用方**。工单确实写明「接线是后续票」，但这正是母 spec 开篇痛批 `agentcore` 的那个形状。请在 `docs/AI_文档索引与现行决策.md` 给 AR2 接线登记到期日。
- **「内核置换」的说法比事实大**。第一方代码实际引用的 picoclaw 符号只有 hook 那一组（`HookManager` / `ToolInterceptor` / `HookDecision` 等）；vendored 9,444 行里 pipeline、providers、events、bus、routing、session 全部无人调用。同时 `agentcore` 1,370 行仍在，`agentcorehooks.Governance` 仍是所有未汇流平面的 guard——九个控制现在有两套实现。真实发生的是**治理链第一次跑在生产 chat 流量上**（成果是实的），加上工具调用分发换成了 picoclaw 的 HookManager。ADR-0028 与 AGENTS.md 按这个口径订正，别让「置换内核」固化成事实。
- **内核纯度守卫的口径偏离**。母 spec 测试表要求断言不 import `database/sql` / `net/http`；实现只查了第一方业务包（vendored providers 本来就用 net/http）。偏离合理，订正 spec 而不是改实现。
- **一条早于本批次的既有问题**：`agenttools/runtime.go` 把 guard 的任何 `Block` 一律映射为 `ErrorScopeDenied`，而未走 guard 的 `Evaluate` 路径经 `policyErrorCode` 会分出 `capability_denied` / `permission_denied` / `invalid_arguments`。能力拒绝与幂等键缺失因此伪装成跨法人拒绝。**不是本批次引入的**（`bbdec17` 即如此），但治理链现在真的在保护生产流量了，这个混淆的代价比以前高。单独开票，不要塞进 AF1–AF4。
