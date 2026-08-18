# ENV-001 + KPI-001 + ENV-002 / Review 1：`ACCEPTED`

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-14
被评审对象：`feat/env-001-kpi-001-env-002` @ `2f1a60e`
基线：`main @ 04865ee`

**结论：`ACCEPTED`。无 P0/P1/P2，可合并。** 这是本项目至今**修掉真实数据正确性缺陷最多的一批**。

评审方式：独立 worktree + 一次性 `postgres:15-alpine`（端口 55436，按 CI 方式建库），跑完销毁。

---

## 1. 三处捏造与误抹，全部确认清零

| 缺陷 | 修复前 | 修复后 | 评审人核实 |
|---|---|---|---|
| **捏造分母** `max(f.Revenue, 1)` | 零营收门店贡献率算出 **−3,400,000%** | `null` | ✅ 全仓代码命中 **0**；剩余 2 处命中是**注释**（说明已移除），非代码 |
| **零填充缺失信号** `CoverageRate = ptr(0)` | 覆盖率缺失被写成 0 | `null` | ✅ 全仓 **0** 处 |
| **覆盖谓词 `!=`** `store360.go:242/252` | 超额覆盖整片抹掉 Peer Benchmark | 统一判定 | ✅ store360 的 `!=` **0** 处；`CoverageIncomplete` 单一判定，7 处引用 |

`−3,400,000%` 这个数字很说明问题：旧引擎不是"略有偏差"，是**在零营收门店上输出了一个荒谬但看起来像数字的值**。DESIGN.md §10 与 AGENTS.md「不用 0 填补缺失」终于在代码层面成立。

---

## 2. K5：`/performance` 的差异逐条可归因

这是本批最重要的证据。实施者报告的五项差异，评审人逐条核对归因是否成立：

| 差异 | 归因 | 成立？ |
|---|---|---|
| 租售比 8 → 10 | 旧引擎 `固定 / 营收`，把变量租金丢了；新引擎 `(固定 + 变量) / 营收` | ✅ 与架构评审记录的缺陷一致 |
| 占用成本率 10 → 11 | 非租赁成分此前未计入 | ✅ 与 CONTEXT.md `Occupancy Cost` 定义一致（基本租金 + 服务费 + 当期变量租金） |
| EBITDA −1000 → `null` | 缺必填字段时旧引擎照算，新引擎返回 null | ✅ 正是「不用 0 / 不捏造」规则 |
| 基准 peers 7 → 4 | 排除自身 + 剔除跨币种 | ✅ 与 CONTEXT.md `Peer Cohort` 词条一致 |
| USD 不再污染 CNY 群 | 同上 | ✅ |

**每一条差异都是旧引擎的缺陷被修正，没有一条是新引擎引入的回归。** 报告中无「无法归因」项——这正是任务票 §8 要求停下来问的情形，未触发。

### 2.1 同群规则确实统一了（评审人追加核实）

任务票要求 `/performance` 与零售线用同一套规则。评审人追踪了 `MinimumPeerCount` 的全部消费点：

```
retailkpi.MinimumPeerCount = 3                     ← 唯一定义
retailstore360.MinimumPeerCount = retailkpi.…      ← 别名，非副本
operating/store_analysis.go:65 → retailkpi.…       ← /performance 路径
```

且 `handlers/operating_facts.go:381` 对外发布的 `peer_definition` 为
「same brand + region + currency, decision-ready, excluding target, minimum 3 peers」
—— **与 CONTEXT.md 的 Peer Cohort 词条逐项一致**。不是两套规则碰巧取了同一个数，是同一个常量。

### 2.2 `/performance` 未被删改

`fourwall.go` 保留，改为转调 `retailkpi.EvaluateStorePeriod`；路由、页面、响应形状均未动。符合增量叠加底线——**换实现不是删功能**。

---

## 3. 独立复跑

| # | 检查 | 结果 |
|---|---|---|
| — | 全量 `go test ./... -count=2`（真实 PostgreSQL） | ✅ **FAIL 数 0** |
| — | `go vet ./...` | ✅ 干净 |
| K2/K3 | 关键用例 | ✅ `TestFourWallZeroRevenueYieldsNullRatios`、`TestCoverageIncompleteSingleVerdict`、`TestMixedCurrencyPeerIsExcludedAndInsufficientPeersAreExplicit`、`TestBuildCoverageRateIsNullWhenExpectedUnknown` 等全 PASS |
| P3 | 前端版本字面量 | ✅ 0 |
| D1 | DataTrustBar 四页复用 | ✅ 四页均引用；旧 `pulse-trust-strip` **0** 处（1300 字符 Alert 与死 CSS 已删） |
| D2 | `decision_ready=false` 降级 | ✅ 组件测试断言 `is-degraded` + 原因 + KPI 角标 |
| D3 | 覆盖缺失渲染 | ✅ `if (!coverage) return "—"`，测试钉住，从不零填充 |
| — | Web 全量 | ✅ 17 套件 / 108 用例，type-check 通过，`enforce-design` 无新增违规 |

---

## 4. 常设规则连续两批遵守

三个功能 commit **均未触碰** `enforce-design.mjs`。守卫语义变更必须独立成 commit 这条规则，连续两批被遵守，流程问题已闭环，本轮不再重复。

关于 gofmt 导致的 commit 重整（amend 回 ENV-001、重提 KPI-001）：分支推送前完成，无历史重写问题，**且结果是三个功能 commit 边界干净**——处理正确。

---

## 5. 需要用户视觉确认

| 应当**看到**变化 | 位置 |
|---|---|
| 顶部可信条从多行文本块变为**一行摘要 + 可展开** | `/operating-pulse`、`/store-360`、`/scenario-workbench`、`/ai-chat` |
| 数据未就绪时**整条变黄 + 显示原因**，且每张 KPI 卡出现「仅供查看」角标 | 同上 |
| 模拟 / 正式从彩色 Tag 变**状态点** | 同上 |
| `/performance` 的**数字会变**（租售比、占用成本率、部分 EBITDA 变「—」、基准同群变少） | `/performance` |

**`/performance` 那一栏要特别留意**：数字变化是**预期的修正**，不是回归——旧值是错的。若看到某个数字变化在上表五条归因之外，请告诉我。

---

## 6. 合并意见

同意合并，保留 merge commit。

## 7. 后续待办

| 项 | 归属 |
|---|---|
| 信封 Coverage 未含 `missing_fields`（实施者附带发现） | 后续小票 |
| `aiagent` 两处 rate 式判定未强制收敛（语义一致，非缺陷） | 后续小票 |
| 场景页无响应时由查询参数构造占位可信条 | 后续小票 |
| scoped 资源 404 携带 `details.resource` | 上批遗留 |
| 其余 735 处错误站点、内联样式 876 处、`!important` 139 处 | 后续分批 |
