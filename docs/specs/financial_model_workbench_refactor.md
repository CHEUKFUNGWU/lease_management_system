# Spec: 三表财务模型工作台重构（/financial-model）

> 状态：ready-for-agent（无 issue tracker，按仓库惯例落 docs/specs/）
> 来源：2026-08-22 UIUX 审查后续；用户判定现页面「差到爆炸」，要求先理解后端再重构
> 配套设计：见文末「模块与接缝」（codebase-design 词汇）

## Problem Statement

用户打开 /financial-model 看到的是引擎测试控制台，不是工作台页：手填定义 ID、裸 JSON textarea 当唯一输入、期初闸要求用户照 Go struct 猜 JSON 形状、「运行模型」禁用无解释、右半屏全空白、run 的发布/导出/取消/历史全部不可达。而后端实际有 ~15 个端点（模板治理、异步 run、发布门、导出、分组视图、保存视图），前端只用了 2 个。

## Solution

按 DESIGN.md §8.3 报表型页面骨架重写为两栏工作台：左栏「定义与假设」输入区，右栏 run 结果区。输入结构化（期初三道闸从 textarea 变表单），运行走异步轮询（不再冻结），勾稽/Gap/导出成为结果主角，空态诚实（定义列表后端是桩就明说）。

## User Stories

1. 作为财务编辑，我想从定义列表选择模型定义而不是手填 ID，以免拼错 UUID。
2. 作为财务编辑，当定义列表为空时，我想看到诚实的空态和创建指引，而不是以为系统坏了。
3. 作为财务编辑，我想用键值对编辑假设并在输错时立即看到解析错误，而不是提交后才失败。
4. 作为高级用户，我仍想直接粘贴 JSON 编辑假设，以便批量迁移既有配置。
5. 作为财务编辑，我想点「运行模型」后立即看到 queued/running 状态并能取消，而不是界面冻结等待同步返回。
6. 作为财务复核人，我想一眼看到勾稽总状态（passed/failed/pending）与 T1–T16 明细表，以判断能否发布。
7. 作为财务审批人，我想在 run 页面直接发布（走 POST /runs/:id/publish），而不是找 API 工具。
8. 作为财务编辑，我想下载 run 的导出文件（支持 fold=quarter），而不需要构造 curl。
9. 作为财务编辑，我想用结构化表单填写期初 BS 三道闸（主体/币种/期间行 + 两列合约余额子表），而不是猜 JSON 结构。
10. 作为高级用户，期初闸保留 JSON 模式折叠项，以便复用既有脚本产物。
11. 作为财务编辑，我想在 Gap 存在时看到逐条具名 Gap 卡片（kind/period/detail），而不是一句拼接的 warning。
12. 作为任何角色，首次进入页面时我想看到「这是什么、第一步做什么」的引导空态，而不是三个空白卡片。
13. 作为权限受限用户，scope_denied 时我要看到真实的权限拒绝原因（不被软化为无数据）。

## Implementation Decisions

- 页面重写为 `app/financial-model/page.tsx`（壳 ≤200 行）+ 深模块 `workbench.ts`（状态机，纯函数可测）。
- 运行流程默认 async:true：POST definitions/:id/runs → 202 {run_id,queued} → 每 2s GET runs/:id 轮询至 completed/failed/cancelled → 支持 cancel。同步路径仅作为 async 失败的回退。
- 期初闸表单按后端结构组装：balance{legal_entity_id,currency,periods[]} + lease_ref[]/engine[]（ContractBalance 仅 contract_id/lease_liability/rou_asset 三字段）+ policy{version}；JSON 模式为折叠高级项。
- 定义选择：调用 GET /definitions；当前后端是空桩 → 渲染诚实空态 + ID 手填降级入口（不造假数据）。
- 发布按钮仅在 tie_out_status=passed 且未发布时可用；权限不足时按钮隐藏（后端 permission fin_models:approve）。
- i18n 全量三语（zh-CN/zh-HK/en），新枚举登记进 code-lists-contract.test.ts（CONTRACT-001）。
- 遵守 DESIGN.md：金额 fmtNum、缺失显示 —、§8.3 五约束（前端不算任何模型行）。

## Testing Decisions

- 主测接缝只有一个：`workbench.ts` 状态机（输入：事件；输出：UI 状态）。覆盖 idle→running→polling→completed/failed/cancelled、JSON 解析失败、scope_denied 透传、opening 表单→payload 组装。
- 组件渲染断言仿 ai-chat/styles.test.ts 先例（SSR renderToStaticMarkup）；i18n 键完整性仿 i18n-keys.test.ts。
- 不 mock 后端形状之外的任何东西；payload 组装测试锁住与 handler struct 的契约。

## Out of Scope

- 后端 List/Create Definition 实装（B 阶段，Go 侧另票）；run 历史列表端点；模板管理 UI；GroupRuns 视图 UI。

## Further Notes

- B 阶段两个 Go 端点落地后，「定义下拉」替换「ID 手填+诚实空态」即闭环。
- 幂等键语义保持现状（会话内同输入同 key）。

---

## 模块与接缝（codebase-design）

**深模块**：`app/financial-model/workbench.ts`
**接口（小面）**：
```ts
type WbEvent =
  | { t: "select_definition"; id: string }
  | { t: "edit_assumptions"; text: string }
  | { t: "run_requested" }            // 校验通过 → dispatching
  | { t: "run_dispatched"; runId: string }
  | { t: "run_polled"; status: "queued"|"running"|"completed"|"failed"|"cancelled"; run?: FinModelRun }
  | { t: "cancel_done" }
  | { t: "error"; kind: "parse"|"scope_denied"|"failed"; message?: string }
  | { t: "reset" };
type WbState =
  | { phase: "idle"; definitionId: string; assumptionsError?: string }
  | { phase: "dispatching" } | { phase: "polling"; runId: string; status: "queued"|"running" }
  | { phase: "completed"; run: FinModelRun }
  | { phase: "failed"; message?: string }
  | { phase: "scope_denied"; reason?: string };
reduce(state, event): WbState   // 纯函数
```
**为什么深**：异步生命周期/错误分诊/scope_denied 保持原因等全部行为藏在 reduce 里，页面壳只做「事件→reduce→渲染」。删除该模块则这些逻辑散回页面。
**适配器**：轮询定时器与 fetch 是壳侧副作用（useEffect），不进纯函数——测试不需要假计时器。
