# FIX-002 + FIX-003 + FIX-004 / Review 1：`ACCEPTED`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-15
被评审对象：`feat/fix-002-004-empty-states` @ `9c40cd7`
基线：`main @ db81611`

**结论：`ACCEPTED`。无 P0/P1/P2，可合并。**

---

## 1. FIX-002：根因比票面描述的更深，且诊断正确

任务票只说「500 应该是 422」。实施者查出的实际链条是：

- `SnapshotBuilder` **首错即返**，只带出一个裸 UUID；
- `buildSnapshot` 把所有构建错误一律 500；
- 于是 `writeProjectionError` 里那个 **422 分支永远轮不到**。

修法是让快照构建**收集全部缺率合同**，经新增的 `reporting.DiscountRateMissingError` 映射为 422。该类型 `Unwrap` 回 `contractsvc.ErrDiscountRateRequired`，**既有 `errors.Is` 判定不受影响**——这是对的做法，没有为了新错误类型去动既有判定。

**红线守住了**：`git diff -- db/` 为 **0 行**，未给任何合同填折现率，未改 `ErrDiscountRateRequired` 的触发条件。引擎仍然拒绝猜测（AGENTS.md §Discount Rate 人机协同）。

**code 选择的理由成立**：选 `data_unavailable` 而非 `business_failure`，依据是仓库先例（`retail_scenarios` 的证据不足降级已用该 code 表达「数据条件不满足」）。**未新增 code**，符合票面约束。

6 条调用路径逐条核查（3 条本就 422、2 条批处理软跳过属设计行为、1 条即本次修复点）——比票面要求的 5 条还多查了一条。

## 2. FIX-003：等高用的是可验证的契约，不是眼睛

```css
.pulse-kpi-card { height: 156px; overflow: hidden; }
```

`min-height` → `height` + `overflow: hidden`，数值行统一到共享类 `.pulse-kpi-value`（常量 margin、`tabular-nums`），易换行的内层统一 `nowrap` + `ellipsis`。

`kpi-card-height.test.ts` 是**解析 CSS 与页面源码的契约测试**，断言固定高度、无 `min-height` 残留、移动端断点同样固定、`tabular-nums`、截断规则。

这是本票最值得肯定的地方：**实施者看不见界面，就用机械可验的方式把「等高」这件事钉死**，而不是声称自己看过。报告里也如实写明「好看与否」需用户实地确认。

## 3. FIX-004：范围精确

`tableScrollX(rowCount, width)` —— 有行才产出 `{x}`，空态返回 `undefined`，AntD 不渲染滚动容器。

**范围核实**：`scroll={{ x` 全仓 main **22** 处 → 分支 **20** 处，精确减少 `/portfolio` 的 2 处，其余 20 处未动并已列清单。

## 4. 复核结果

| # | 检查 | 结果 |
|---|---|---|
| — | Go 全量 + vet（真实 PostgreSQL） | ✅ FAIL 0 |
| — | Web | ✅ **31 套件 / 197 用例**，lint / `enforce-design` 无新增违规 |
| A2 | `details` 带合同编号 | ✅ `CT-LE001` 这类，非裸 UUID |
| B1 | 卡片等高 | ✅ CSS 契约测试 |
| C4 | 未越界 | ✅ 22 → 20 |
| — | 守卫语义 | ✅ **连续第六批零改动** |

实施者报告期间被守卫拦了两次，都是测试文件里的断言字面量被误判，**改写断言绕开、未动守卫一行**——处理正确。

## 5. 评审人补记：守卫对多行注释有个已知误判形状

评审人在修 PR #19 时也撞上同一个形状：CSS 多行块注释的**续行若不以 `*` 开头**，会被 CJK 检查判为硬编码中文（豁免规则是 `/^(\/\/|\/\*|\*)/`）。

改成惯例的 ` * ` 续行格式即可绕开，**不需要动守卫**。但这是第三次有人撞上它（实施者两次 + 评审人一次），建议后续在守卫自己的 commit 里把续行形状一并纳入豁免。记入后续，本批不动。

## 6. 关于共享工作树的分支竞争

实施者报告：执行期间另一会话在同一工作树并行做品牌 Logo，其 FIX-002 commit 一度落到对方分支上，已用 `git commit-tree` 重放回本分支，双方历史经 `git diff` 核对干净。

**评审人独立确认**：本批改动 17 个文件（报告称 15，差额为交付报告与新增测试文件，非产品代码），与 PR #19 的 4 个文件（`AppLayout` / `BrandIcon` / `globals.css` / `login/page.tsx`）**无重叠**。

评审人自己在本会话中也两次踩到同一类坑（一次提交落到同事分支、一次 worktree 基线用了落后的本地 `main` 导致提交丢失并重写）。**结论一致：并行任务必须用独立 worktree，且一律从 `origin/main` 建，不用本地分支名。**

## 7. 合并意见

同意合并，保留 merge commit。

## 8. 后续待办

| 项 | 归属 |
|---|---|
| 守卫豁免纳入多行块注释续行形状 | 守卫自己的 commit |
| 其余 20 处 `scroll={{ x` 空态门控 | 后续批次 |
| `/reports` 其余错误站点转错误契约 | 后续批次 |
| `next/font/google` 构建期依赖 Google（CI 随机挂） | 待开票 |
