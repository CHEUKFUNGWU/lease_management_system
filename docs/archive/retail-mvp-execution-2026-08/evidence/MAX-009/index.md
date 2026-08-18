# MAX-009 Evidence Index

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

状态：`ACCEPTED / GO`（Review 8 无 P0/P1）。所有截图与 JSON 来自真实 `http://localhost:3000` / Core；无密码、token、连接串。固定上下文：seed `20260812`、dataset `retail-sim-v1-2853d9537f358baf`、as-of `2026-06-05`、source `retail_simulator`、7 天窗口。

## R2 有效证据

| 文件 | viewport / 类型 | 关键证据 |
|---|---|---|
| `r2-pulse-1440x900.png` / `r2-pulse-390x844.png` | 1440×900 / 390×844 | 420/420、rank1 `SIM-2853D953-007`、high/3.02、occupancy 10.08pp、来源可信条 |
| `r2-store360-1440x900.png` / `r2-store360-390x844.png` | 1440×900 / 390×844 | 目标身份、同群、trend、bridge/evidence；移动无溢出 |
| `r2-scenario-1440x900.png` / `r2-scenario-390x844.png` | 1440×900 / 390×844 | 12 月 labor -10%、Baseline/Plan、守恒 bridge、Working/IFRS16 隔离 |
| `r2-scenario-stale-1440x900.png` | 1440×900 | 假设变化后结果 stale、保存禁用；重新计算后恢复 |
| `r2-action-confirm-modal-1440x900.png` | 1440×900 | 保存前真实二次确认 Modal |
| `r2-aichat-1440x900.png` / `r2-aichat-390x844.png` | 1440×900 / 390×844 | AI context、三 read tools、deterministic fallback、来源 |
| `r2-ai-proposal-1440x900.png` / `r2-ai-proposal-390x844.png` | 1440×900 / 390×844 | proposal card、simulated/dataset、Owner/Due 空；展开结构化 JSON 含 formal_execution/business_write false |
| `r2-ai-session-drawer-390x844.png` | 390×844 | Drawer、native session buttons、More/delete 可见 |
| `r2-ai-trace-artifact-sanitized.json` | 脱敏 JSON | run/artifact ID、三 read tool 顺序、proposal provenance、无 secrets |
| `r2-api-smoke.json` | 脱敏 JSON | fixed seed/fact/action replay、409、A/B、production、negative statuses |
| `r2-formal-before.json` / `r2-formal-after.json` | DB 快照 JSON | 正式表 count/max 不变；ticket fixture 1→0 |

## R3 收口证据

| 文件 | viewport / 类型 | 关键证据 |
|---|---|---|
| `r3-action-confirm-modal-1440x900.png` | 真实 1440×900 | 保存前确认 Modal，Owner/Due 可空，未跳过二次确认 |
| `r3-scenario-1440x900.png` / `r3-ai-proposal-1440x900.png` | 浏览器 inner viewport 1440×900；截图内容区 1429×893（垂直滚动条占用） | 真实 Scenario bridge 与 AI proposal/provenance；不把内容区像素误报为 viewport |
| `r3-ai-chat-390x844.png` | 390×844 | mobile Drawer、session selector、More/delete 可见；scrollWidth=clientWidth=390 |
| `r3-postgres-recovery.json` | recovery JSON | 原 volume、仅 build-cache 回收、WAL recovery、pg_isready、正式表对数 |
| `r3-migration.json` | migration/init JSON | 023 回撤、040 existing constraint/rollback、fresh init 临时资源精确清理 |
| `r3-negative-agent.json` | 脱敏 JSON | needs-input/partial/no-facts/invalid 的 status、reason、confidence=.40、无 proposal/action |
| `r3-cross-scope.json` | 脱敏 API JSON | LE002 对 LE001 source/action/AI run/session 的实际 status/response shape |
| `r3-race.json` | 脱敏 API + DOM JSON | old/new request key、URL、cancel status、最终新 scope DOM |
| `r3-residual.json` | 精确 SQL JSON | dataset/facts/stores/batch/fact request/actions/users/sessions/runs/artifacts residual=0，含 query timing |

R3 额外说明：`r2-scenario-stale-1440x900.png`、`r2-action-confirm-modal-1440x900.png`、`r2-ai-proposal-1440x900.png` 的实际位图为 1280×720，故不再作为 R3 的 viewport 证据；R3 文件明确记录浏览器 inner viewport 与内容区像素差异。

浏览器 DOM 证据：R3 AI Chat 390px 实测 `scrollWidth=390/clientWidth=390`，mobile `.session-more-btn` active/normal 均 opacity=`1`；trigger、Drawer 和原生 session button 可访问。旧 R2 的 Enter/Escape 回焦点记录保留为历史证据，R3 不把旧截图当作新 viewport 证据。

## 历史无效截图（保留但不引用）

`01-*`、`02-*`、`blocked-*`、`valid-*` 属于 Review 1/旧运行态或未含 R2 teardown 的历史文件，均不作为本轮最终证据；R2 证据以本页 `r2-*` 和 JSON 为准。

## R4 纯发布取证

| 文件 | 类型 / viewport | 关键证据 |
|---|---|---|
| `r4-race-browser.json` | 历史无效网络记录；同一真实浏览器 tab 的 DOM 观察 | 28 天→7 天 Promise 状态与最终 7 天 420/420 DOM；其中 `/api/v1/...` 仅为 `normalized_core_endpoint`/历史记录，不是完整浏览器同源 request URL |
| `r4-negative-needs_input.json` | 脱敏原始 API/trace | session/run、5 个按序 events、missing_context、confidence 0.40、insufficient、action 0→0 |
| `r4-negative-partial.json` | 脱敏原始 API/trace | pulse→diagnostics、partial_coverage、confidence 0.40、insufficient、action 0→0 |
| `r4-negative-no_facts.json` | 脱敏原始 API/trace | no_facts、diagnostics rejected、confidence 0.40、insufficient、action 0→0 |
| `r4-negative-invalid_rate.json` | 脱敏原始 API/trace | scenario rejected invalid_arguments、confidence 0.40、insufficient、无 proposal/action |
| `r4-cross-scope.json` | LE001/LE002 同目标原始 API envelope | LE001 source/action/run/session/artifact 同 ID 存在；LE002 source 空 envelope、action total=0、run/session 404 |
| `r4-residual.json` | exact SQL residual | dataset/facts/stores/batch/request/action/users/refresh/session/run/artifact selector 均 `actual=0`，含 query timing |
| `r4-ai-proposal-1280x720.png` | 真实浏览器 viewport 1280×720；位图 1269×714 | proposal provenance、simulated/dataset/source、Owner/Due 空、仅建议不写业务；不冒充 1440×900 |

R4 浏览器控制面不提供 HAR/Resource Timing/route interception；`r4-race-browser.json` 只保留历史同页 click Promise、normalized Core endpoint、request key 和最终 DOM，已降级为无效网络证据，不与当前有效 race 证据并列。R5 的 `r5-race-gate.json` 与实际页面 deferred gate 测试才是当前有效 race 证据。rendered-DOM 自动化缺口登记 `HARD-012`。

## R5 收口证据

| 文件 | 类型 | 关键证据 |
|---|---|---|
| `r5-race-gate.json` | 同页浏览器观察 + 实际页面 gate 测试 | 真实 route pattern `/api/api/v1/...` 与 Core endpoint 分开标注；新 7 天先 commit、旧 28 天 deferred commit 被拒，最终 DOM 7 天 420/420；Browser timing export limitation 如实记录 |
| `r5-negative-agent.json` | 脱敏 raw trace/event 选段 | needs-input=`missing_context`、no-facts=`no_facts`（底层 `scope_denied`）、invalid-rate=`resulting_rate_out_of_range`（底层 `invalid_arguments`）；assistant message 直接含 `evidence=insufficient`/`confidence=0.40`，proposal/action 均 0 |
| `r5-residual.json` | 精确 teardown SQL 结果 | 唯一 R5 user/dataset/batch/facts/stores/AI sessions/runs/artifacts/action selectors、查询耗时与 residual 全部 0；无 wildcard |

R5 代码只做 request gate 最小提取与低证据 deterministic assistant/tool reason 投射；未新增 route/schema/tool/能力。R5 fixture 通过现有 Admin API 创建、logout 后按精确 FK 顺序清理，产品 Admin API 无 delete 用户能力这一事实已在报告披露。

## R6 收口证据

- `retail_operations.go` 已删除 dataset 名称到 `no_facts` 的权限语义分支；定向测试证明普通法人 scope 的结构化空 population 为 `no_facts`，而 `scope_denied + no-fact 名称` 仍为 `scope_denied`。新增单测证据为 deterministic Response/ProjectResult、`SideEffects=false`、无 proposal/action/artifact；不声称其生成 raw message_end/action DB，raw message_end 仅引用 R5。
- R4 race 文件中的 `/api/v1/...` 已明确降级为历史无效网络记录/`normalized_core_endpoint`，不得作为完整浏览器同源 request URL；当前有效 race 证据是 `r5-race-gate.json`。
- `r4-residual.json` 新增 `ai_chat_sessions.id='66fbe25c-e12e-46b1-b7d0-a02c6c74199e'` 与 `ai_chat_runs.id='19dee11e-2090-4987-9416-c555b6255ea2'` exact selectors，均 actual=0。

## R7 收口证据

- `retail_operations_test.go` 的 no-facts fixture 使用非 Global `access.Scope{LegalEntityID:"entity-a"}` 且无门店/区域/品牌过滤；另一条越权门店测试保留 `scope_denied`。两例均显式断言 `SideEffects=false`、proposal 为 nil、ProjectResult artifacts 为 0。
- 本轮只修测试语义与文档证据边界，无新增 fixture；R4 四个 username 的 users/refresh 精确集合 selector 与可恢复 B_LE001 UUID selector 均 actual=0。当前状态 `IN_REVIEW`，等待 Review 8。
