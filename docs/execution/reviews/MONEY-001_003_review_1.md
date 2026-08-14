# MONEY-001 + MONEY-002 + MONEY-003 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-15
被评审对象：`feat/money-representation` @ `f81fab5`
基线：`main @ 41d8f32`
依据：[ADR-0020](../../adr/0020-money-representation.md)

**结论：`ACCEPTED`。无 P0/P1/P2，可合并。** 一条验收标准需由评审人更正（是我写得不准，不是实现有问题）。

---

## 1. 最强的一条证据：148 个报告数值零变化

评审人自行 diff 了迁移前后的回归报告全文：

```
@@ -4 +4 @@
-- 生成时间：2026-08-10T15:55:40+10:00
+- 生成时间：2026-08-15T02:33:28+10:00
@@ -6 +6 @@
-- 容忍差异：1.00
+- 容忍差异：0.00
```

**整份报告只差两行：生成时间戳，和我们主动改掉的容差值。** 148 条断言的期望、实际、差异、结果**逐字节相同**。

把金额从 `float64` 换成 decimal，报告里 148 个数值**一个都没变**。这正是 C3 想证明的：**只改了表示法，没改业务结果**。

## 2. 安全网确实会兜住（评审人自行验证）

T4 不采信报告输出。评审人把 `TC-01` 的 `initial_liability` 加了 `0.01`：

```
--- FAIL: TestRegressionFixtures/TC-01
    initial_liability: expected 116895.45, actual 116895.44, delta 0.01, tolerance 0.00
```

还原后通过。**容差 0 的网是真的。** MONEY-001 先行的意义在此——迁移期间任何偏差会立刻暴露，而不是藏进 ±1 元。

## 3. 范围控制精确

| 检查 | 结果 |
|---|---|
| C1 范围内 float64 金额字段 | ✅ 0。残留的 `float64` 全是**非金额**（折现率、系数、比例、容差、汇率、配置读取） |
| C2 范围内 `round2` | ✅ `ifrs16` 43 → 0；其余 9 个包**逐包计数完全不变** |
| C8 其余包 float64 | ✅ 评审人逐包比对 `main` 与分支：predeal 40=40、reporting 117=117、fpna 31=31、retailkpi 26=26、cashflow 18=18、dealcompare 84=84、renewaldecision 33=33 |
| 前端 | ✅ 0 个文件 |

`predeal` 与 `reporting` 有改动，评审人核查后确认是**类型变更强制的边界适配**（`calculation.InitialLiability` 变成 `money.Amount`，调用方加 `.Float64()` 保持自身表示法不变），不是范围蔓延——C8 的逐包计数不变正好佐证。

## 4. `money` 包设计正确

| 要求（ADR） | 实现 |
|---|---|
| 零值安全 | `Amount{value, set bool}` + `IsSet()`，零值可判别 |
| 除零 | `Div` 返回 `error`，不是 panic 也不是 Inf |
| 币种小数位 | `ScaleFor` / `Round(currency)` / `ValidatePrecision(currency)` |
| 分摊守恒 | `Allocate(currency, weights)` 最大余数法 |
| DB 互操作 | `Value()` / `Scan()`，并补了 pgx `pgtype.Numeric` 路径 |
| JSON | `MarshalJSON` 输出不带引号的 number |

## 5. 复跑结果

| # | 检查 | 结果 |
|---|---|---|
| C7 | 全量 `go test ./... -count=2`（真实 PostgreSQL）+ vet | ✅ **FAIL 数 0**，vet 干净 |
| C3 | 148 条断言，容差 0 | ✅ `TestRegressionFixtures` PASS |
| T4 | 扰动 Golden 值 | ✅ 见 §2 |
| M6 | 守卫：注入 `TotalAmount float64` | ✅ **拦截**并报出 `budget.go:398`；还原后 41 个变更文件无违规 |
| T5 | `review_status` | ✅ 仍是 `pending_third_party_review` |

---

## 6. C6 的验收标准是我写错了，不是实现有问题

我在票面写的是「JSON 响应与迁移前**逐字节一致**」。实测：5 个载荷中有 **35 对**字节不同，形如

```
40.200000000000024  →  40.2
```

**逐字节一致这个标准本身就不可能达成，而且不该达成。** `40.200000000000024` 正是 float64 聚合噪音——是这批要消除的东西。要求字节不变，等于要求把噪音保留下来。

正确的标准应当是「量化到币种精度后零差异」，实施者做的正是这件事并如实报告了 35 对的分值全等。

**前端影响：无。** 两个值格式化到两位小数后都是 `40.20`。

这条记在这里是为了**下次我写这类验收标准时不要再犯**：涉及浮点噪音消除的迁移，验收要锚定**语义等价**，不是字节等价。

---

## 7. 迁移过程中暴露的三个既有问题（处理均正确）

| 问题 | 性质 | 处理 |
|---|---|---|
| `aggregateMonthly` 按 map 迭代序返回 | **既有缺陷**：线上 JSON 每次调用字段序都可能不同，旧代码自己都无法复现自己的输出 | 改为按年月排序。这是任何字节级比对成立的前提，也让 API 响应确定化 |
| `monthend/fx.go` 初稿把 `math.Abs` 写成 `Neg` | 迁移中的自身失误 | 未提交即修正，并新增 `money.Abs`。**自查发现并主动披露** |
| 休眠列 `principal_repayment` 旧代码静默写 0 | 既有行为 | 迁移后显式保留 0，正确计算留给行为票。**未擅自改变持久化语义** |

第一条值得单独说：**旧代码的线上输出是非确定性的**——同一请求两次调用可能得到不同的字段顺序。这是迁移过程顺带发现的既有缺陷，不是本批引入的。

---

## 8. 一个必须写清楚的边界

精确性**止于 `ifrs16` / `monthend` 边界**。`reporting`、`predeal` 等消费方通过 `.Float64()` 转回浮点，因此**披露报表包的数字仍是 float64 派生的**。

这符合 ADR §Decision 8 的分批设计，不是缺陷。但要如实理解：**「审计链已迁移」指的是计量与分录，不是审计师看到的全部内容。** 披露报表在后续批次。

---

## 9. 合并意见

同意合并，保留 merge commit。

## 10. 后续待办

| 项 | 归属 |
|---|---|
| `reporting` / `disclosure_projection` 迁移（披露报表包仍 float64 派生） | 下一批金额票 |
| 其余包金额字段（fpna / retailkpi / cashflow / dealcompare / predeal 等） | 后续分批 |
| `principal_repayment` 的正确计算 | 行为票 |
| 汇率仍为 float64，与金额相乘处的精度边界 | 评估后决定是否迁 |
| 后端 `AgentToolCall` 补耗时字段、ai-chat confidence 未持久化、278 个死 i18n key | 上批遗留小票 |
