# MONEY-004 + FIX-001 + I18N-002 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-15
被评审对象：`feat/money-004-fix-001-i18n-002` @ `ac3ee21`
基线：`main @ 4133a31`

**结论：`ACCEPTED`。无 P0/P1/P2，可合并。**

---

## 1. 保护网抓到了一个正在发货的用户可见缺陷

I18N-002 要求「先让未知 key 响亮失败，再删任何东西」。这条顺序**当场就有了回报**。

评审人独立确认：

```
web/app/scenario-workbench/logic.ts:11
  labor_cost: "retail.kpi.labor_cost"
web/app/lib/i18n.ts (main)
  "retail.kpi.labor_cost"  →  不存在
```

同组的 `retail.kpi.revenue`、`gross_profit`、`fixed_rent` 都在，**唯独 `labor_cost` 缺失**。而 `t()` 对未知 key 返回空字符串、不报错。

**结果：`/scenario-workbench` 上有一行 KPI 的标签是空白的——在 `main` 上，现在就是。** 这个缺陷早于本批存在，没有任何测试或错误提示会暴露它，直到有人建了这张网。

实施者按任务边界**没有自行补文案**（票面明确要求"既有缺陷不要顺手补"），而是在测试里显式豁免并记录为行为票。处理正确。

**这条独立证明了指令里那个顺序要求的价值**——若先删后建网，171 个删除里任何一个删错，都会变成同样静默的空白，且无人知晓。

## 2. 死 key 重算：171 而不是 334

朴素 grep 说 334，实际 **171**。差出来的 163 个是通过常量表、模板前缀、三元表达式等**字面量看不见的路径**被引用的——正是指令 §3.1 警告的那类。

若按 334 删，会误删 163 个在用 key，产出 163 处静默空白文案。

删除是**纯删除**：`0 新增 / 855 删除`（评审人 `--numstat` 核实），三语一并移除，删后重算死 key = 0。

**K1 评审人自行验证**：把 `store-360/logic.ts` 的动态 key 换成 `"no.such.key.review"` → 扫描测试**失败并点名该 key**；还原后 6 项全过。网是真的。

## 3. MONEY-004：披露报表包完成迁移

上一批遗留的审计缺口（精确性止于 `ifrs16`/`monthend`，披露仍是 float64 派生）已补上。

| 检查 | 结果 |
|---|---|
| D1 三文件 float64 | 117 → **19**，残留全为非金额（比值、期数） |
| D2 `.Float64()` | 范围内仅剩 **1 处**（比值计算），已说明 |
| D4 148 条断言，容差 0 | ✅ 全过 |
| D5 其余包计数 | ✅ 逐包比对 `main`：fpna 31、retailkpi 26、cashflow 18、dealcompare 84、predeal 40、renewaldecision 33、operating 107，**全部一致** |
| D6 全量 `-count=2` + vet + 真实 PG | ✅ FAIL 0 |

**D3 按更正后的标准执行**：6 个披露端点量化到分后零差异，其中 3 个逐字节一致，另 3 个仅 47 对 float64 噪音（`483295.49909999996 → 483295.4991`）。这正是上一批 Review §6 确立的口径——语义等价而非字节等价——且已固化为常驻测试。

保留两处既有线上行为（workpaper 利率显示 2 位小数、`unit_price` 继续用 `roundProjection`）并如实说明，未擅自改变对外输出。

## 4. FIX-001

| 检查 | 结果 |
|---|---|
| `duration_ms` | 在唯一执行漏斗计时；**未执行的调用保持缺失，不是 0**（F2） |
| confidence 持久化 | 迁移 `041` **与 `db/init/01_init.sql` 同时提供**（AGENTS.md 硬要求），旧消息 NULL 列正常加载 |
| Web | ✅ 21 套件 / 132 用例，type-check 通过 |

## 5. 流程

守卫（`enforce-design` / `enforce-money`）语义**连续第五批零改动**。

一处小记：I18N-002 的保护网与删除在**同一个 commit** 内分两步。票面要求的是「报告里体现顺序」而非拆分 commit，故满足；但拆成两个 commit 会让"网先于删"这件事在 `git log` 上不言自明。**非缺陷，供下次参考。**

---

## 6. 需要用户看的

| 应当看到 | 位置 |
|---|---|
| **一行 KPI 标签是空白的**（既有缺陷，本批未修，已记行为票） | `/scenario-workbench` 的人工成本行 |
| 重载 AI 会话后，置信度徽标**连同降级原因**仍在（此前刷新即丢） | `/ai-chat` |
| AI 工具 chip 上出现**耗时**（此前无此数据） | `/ai-chat` |
| 披露报表数字**不应有任何变化** | `/reports` 披露报表包 |

最后一行是重点：MONEY-004 是纯表示法迁移，**披露数字看到任何变化都算回归**。

## 7. 合并意见

同意合并，保留 merge commit。

## 8. 后续待办

| 项 | 归属 |
|---|---|
| **补 `retail.kpi.labor_cost` 文案**（三语） | 行为票，建议尽快——是用户可见的空白 |
| 其余包金额字段（fpna / retailkpi / cashflow / dealcompare / predeal / renewaldecision / operating） | 后续分批 |
| `principal_repayment` 正确计算 | 行为票 |
| 汇率仍为 float64，与金额相乘处的精度边界 | 评估后决定 |
