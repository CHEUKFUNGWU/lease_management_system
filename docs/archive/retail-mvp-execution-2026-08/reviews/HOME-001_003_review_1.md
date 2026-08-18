# HOME-001 + HOME-002 + HOME-003 / Review 1：`ACCEPTED`

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

评审人：Codex 主任务（Planner / Reviewer）
评审时间：2026-08-15
被评审对象：`feat/home-001-003-agent-home` @ `9230250`
基线：`main @ af34138`

**结论：`ACCEPTED`。无 P0/P1/P2，可合并。** 视觉观感待用户确认（见 §5），符合任务票 §9 约定。

---

## 1. 三条设计约束全部守住

任务票 §1 列了三条「照字面做会出错」的约束。逐条核实：

### 1.1 中间栏不是空输入框 ✅

`BriefColumn.tsx` 在挂载时经 `useEffect` 调用 `runBrief()` **自动跑晨检**，追问 composer 在简报**下方**，不是首屏主体。

这是本批最容易做错的地方——「参照 Codex」的字面理解就是空 prompt。实现按 §1.1 的结论走了简报优先。

### 1.2 角色感知 ✅

```ts
export function canViewHomeBrief(user) {
  return hasRole(user, "admin") || hasRole(user, "readonly") || hasRole(user, "auditor");
}
```

与 `AppLayout.tsx:78` 的 `canViewAnalysis` **逐字一致**，不是另写一套近似规则。

B3 六角色矩阵测试完整：`admin/readonly/auditor → true`，`editor/reviewer/approver → false`，后三者渲染 `WorkQueueFocus` 而非空简报、也不是权限错误。

**这条是我在写指令时才发现的约束**——若漏掉，每天做合同和月结的三个财务角色会得到一个自己点不进去的首页。

### 1.3 增量叠加 ✅

报告给出 H1 逐项对照表，六类内容全部有落点与测试证据：

| 原内容 | 迁移后 |
|---|---|
| 待复核合同 / 待过账分录 | 右栏 `WorkQueueSummaryCard` |
| 临近关键日期 | 右栏 `UpcomingDatesCard` |
| 月结就绪度 | 右栏就绪度卡（含「存在阻塞项」标签与计数） |
| 总租赁负债 / 本月租赁费用 | 右栏 `MoneyKPICard` ×2 |

**H2 复用而非重写**：`git diff -- web/app/components/dashboard/` **为空**，`RightColumn.tsx` 只 import 不自实现。`DashboardHeader` 仍在 `page.tsx:214` 使用。

---

## 2. 复核结果

| # | 检查 | 结果 |
|---|---|---|
| H1 | 六类内容 | ✅ 逐项对照 + 测试断言 |
| H2 | 组件复用 | ✅ dashboard 目录 diff 为空 |
| H3 | 断点 | ✅ `HOME_RIGHT_DRAWER_BREAKPOINT = 768`，与 DESIGN.md §5.2 一致；`< 768` 单栏 + Drawer |
| H4 | `enforce-design` | ✅ 22 个变更文件，无新增违规（含内联样式） |
| B1 | 首屏自动产出 | ✅ 挂载即跑，无需输入 |
| B2 | 复用组件 | ✅ DataTrustBar / ToolChip / ConfidenceBadge / SourceCitation / ThinkingTrace / SeverityDot 全部复用，无第二套实现 |
| B3 | 六角色 | ✅ 见 §1.2 |
| B4 | 三态 | ✅ `no_data` / 非 decision-ready / `scope_denied` 各有分支与测试；`scope_denied` 走 `t("api.scope_denied")`，测试断言**不含「暂无数据」类文案** |
| B5 | 不重复请求 | ✅ `briefGate.ts` + 测试 |
| P2 | 确认前零写入 | ✅ `proposals.test.ts` 三处 `saveMock).not.toHaveBeenCalled()` |
| P3 | 采纳走既有 API | ✅ `scenario-action-drafts`，同幂等键契约，无新端点 |
| — | Web 全量 | ✅ **28 套件 / 183 用例**，type-check、build 通过 |

`vitest.config.ts` 新增 `esbuild: { jsx: "automatic" }` —— 与 Next 的 automatic JSX runtime 对齐，否则共享组件在测试里会 `React is not defined`。是**修正测试环境失配**，不是放宽测试。

---

## 3. 报告格式：结构清单取代外观清单，有效

上一轮把「外观变化清单」改成「结构变更清单」（因为实施者看不到界面）。本批是第一次应用，报告给出了组件落位、新增 CSS 类、断点契约、角色分支、新增文案 key —— **全是可从 diff 核实的事实**，评审人据此推导视觉影响（见 §5），无一处需要实施者猜测观感。

这个分工继续沿用。

---

## 4. 未完成项（如实记录，符合 §9）

本环境无法访问本地端口，**三栏在 1440×900 / 390×844 / 1280×720 下的实际观感未取得浏览器证据**。任务票 §9 已约定此项由用户确认。

**这不是缺陷，是环境限制被如实披露。** 评审人同样无法登录（需输入密码），故由用户完成。

---

## 5. 给用户的视觉检查表

评审人从结构清单与 diff 推导，标明**预期变化**与**回归信号**：

### 应当看到的变化（预期）

| 位置 | 变化 |
|---|---|
| `/` 首屏 | 从单栏仪表盘变为**三栏**：左导航 / 中间今日简报 / 右侧行动与待办 |
| 中间栏 | **打开即自动出简报**，不需要输入任何东西；下方有追问输入框 |
| 中间栏 | 简报里出现工具 chip、置信度徽标、引用角标、可信度条 |
| 右栏 | 原首页六类内容全部搬到这里 |
| 右栏 | 顶部新增「待确认建议」区，AI 提议以卡片呈现（采纳 / 修改 / 拒绝） |
| `< 768` 窄屏 | 右栏收进 **Drawer**，主区只剩简报 |

### 应当**不**变（看到即回归）

| 项 | 说明 |
|---|---|
| 六类既有内容的**数值与条数** | 只换了位置，数字不该变 |
| 用 editor / reviewer / approver 登录 | 中间栏应是**工作队列**，不是空简报，更不是权限报错 |
| `/scenario-workbench` 人工成本标签 | 上一批已补，应显示「人工成本」 |

### 特别留意

- **1280×720 中等宽度**：三栏在这个宽度下右栏是否被挤到不可用？这是任务票 §9 列的停下来问的情形之一，实施者未能验证。
- **无数据法人**：简报会不会产出一个误导性的空简报？

---

## 6. 两处工作区事项

1. **任务文件不在同事的旧分支上** —— 他基于最新 `main` 重建了分支，处理正确；
2. **工作区有一处未提交改动**：`ai_chat_runtime_postgres_integration_test.go` 两行注释的引号被替换成弯引号。**该改动自本会话开始时就存在**，不属于任何批次，实施者未纳入 commit 是对的。建议用户确认后丢弃。

---

## 7. 合并意见

同意合并，保留 merge commit。视觉确认由用户在合并后进行；若 §5 的「不应变」项出现问题，单独开修复票。

## 8. 后续待办

| 项 | 归属 |
|---|---|
| 三栏在 1280×720 的观感（若被挤压需调断点） | 用户确认后定 |
| 其余包金额字段迁移 | 后续分批 |
| `principal_repayment` 正确计算 | 行为票 |
| 暗色模式（token 已就绪，`ThemeProvider` 仍只是 ConfigProvider） | UIUX 阶段四 |
| 候选 05 fetch 模块（175 个传输方法、29 页手搓 loading） | 架构方案 |
