# MAX-009 Review 5

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 总结

Review 4 的大部分缺口已经关闭：四类负向场景确有真实 session/run/events/tool execution；跨法人 source/action/run/session/proposal 使用同一目标的正反证据成立；R4 fixture 当前数据库残留为 0；截图已如实降为 1280×720；Review 4 后没有产品、schema、AppLayout、旧 route 或 IFRS 16 改动。

但发布仍不能 GO。race 证据记录的是两个浏览器 click Promise，不是实际网络请求；没有制造或证明旧响应晚于新响应，也把手工构造的 Core endpoint 当成浏览器完整 URL。更重要的是，needs-input/no-facts/invalid-rate 三例的 `confidence/reason` 只存在于证据文件顶层人工摘要，不在原始 run/trace/assistant message 中；用户实际看到的回答没有完整交付任务要求的 `reason/evidence/confidence`，invalid-rate 也没有保留 `resulting_rate_out_of_range`。

本轮允许两类最小收口：发布取证，以及为现有低证据 Agent 合同补齐用户可见/可追溯字段。不得新增页面、路由、表、迁移、工具、指标或业务能力；不得改 AppLayout、既有 UI 架构、旧页面和 IFRS 16/Official。

## 2. Standards 轴

### P1 hard — race 文件把规范化 endpoint 冒充实际浏览器 URL

`docs/execution/evidence/MAX-009/r4-race-browser.json:7,13` 写的是 `/api/v1/...`。当前部署 `NEXT_PUBLIC_API_URL=/api`，而 client 再传 `/api/v1/...`，浏览器实际同源 URL 是 `/api/api/v1/...`，再由 Next rewrite 到 Core。`docs/execution/reports/MAX-009.md:101` 却称文件保存了“完整同源 URL”。

修复：不能手工拼接或从源码推断后称为网络证据。保存浏览器/代理实际观察到的 URL、HTTP status 与开始/结束时间；若只保存 Core endpoint，字段名必须明确为 `normalized_core_endpoint`，不得称完整浏览器 URL。

### P2 hard — residual selector 使用占位文字

`r4-residual.json:36-53` 把 users/refresh/sessions/runs selector 写成 `four/five exact IDs`，与报告和索引宣称的 exact selector 不符。Reviewer 当前只读查询确认为 0，但证据包本身不可复演。

修复：写入脱敏但稳定的完整 UUID/username selector，或把该文件如实标为汇总并另附原始 SQL/结果。不得使用 wildcard。

Standards 轴未发现 scope creep，也没有新增 smell finding。

## 3. Spec 轴

### P1 — 没有证明旧响应晚到后被 request gate 丢弃

`r4-race-browser.json:5-26,74-75` 明确承认没有 response-end/cancel/结束顺序；两个 `fulfilled` 是点击动作完成，不是 HTTP 请求完成。最终停在 7 天只证明最后一次点击生效，不能证明 28 天旧响应在 7 天新响应之后到达且未覆盖 DOM。这仍未满足 Review 4 要求的“制造旧请求晚于新请求结束并证明旧结果不提交”。

修复（任选一种，但必须有确定的旧晚于新）：

1. 推荐使用一次性、可删除的本地 delay proxy/测试 Web 实例，让同一浏览器 tab 的 28 天响应固定延迟、7 天正常返回；记录浏览器实际 `/api/api/v1/...`、代理 request/response 开始与结束时间、HTTP status，以及最终 7 天 DOM；完成后删除临时容器/文件，不改正式 compose、产品代码或 volume。
2. 若浏览器控制面确实无法导出网络时序，可用同一 tab 快速切换的真实浏览器证据，加一个使用 deferred promises 的最小确定性 request-gate 测试：明确让 new resolve/commit 后 old 才 resolve，并断言 old 不提交。测试必须调用页面实际使用的 gate 逻辑，不能再次自证常量；仅允许最小提取，不改 UI/API/schema。

### P1 — 三类低证据 Agent 结果没有进入原始产品输出

四个 JSON 的顶层 `reason/confidence/evidence_status` 是人工组合字段。只有 partial 的 assistant 原文包含 `confidence=0.40`；needs-input、no-facts、invalid-rate 的原始 run/trace/assistant message 均没有 `confidence=0.40`。此外：

- no-facts 的实际 diagnostics tool error 是 `scope_denied`，原始 trace 没有 `reason=no_facts`；
- invalid-rate 的实际 tool error 只有 `invalid_arguments`，没有 `resulting_rate_out_of_range`；
- needs-input 原始回答虽要求补上下文，但没有结构化或用户可见的 `reason=missing_context / evidence=insufficient / confidence=0.40`。

这不是单纯证据格式问题，而是现有 Agent 低证据合同没有完整投射到用户可见回答与可追溯 trace。允许做最小正确性修复：在既有 deterministic response/assistant message 或既有 event payload 中保留 `reason`、`evidence_status`、`confidence`，并让 invalid-rate 保留服务端具体业务 reason；不新增 endpoint、schema、artifact type、tool 或 UI。补单元测试后，用新唯一 R5 fixture 重跑三例，证据只能从原始 API/run/trace提取，不得在顶层手工补结论。

Spec 轴确认跨法人主项已闭环；独立 artifact GET 路由不存在不构成缺陷，owner-filtered run/session 404 已足够。截图标注与无 scope creep 通过。

## 4. 独立验证

- 五个基础服务运行，PostgreSQL/MinIO healthy；
- 正式表 count/max 仍为 `29/0/224/10/17/5`，与 R2/R3 快照一致；
- R4 users/actions/sessions/runs/artifact 当前只读查询均为 0；
- `r4-*.json` 均为合法 JSON，未检出密码、token、连接串；
- `r4-ai-proposal-1280x720.png` 实际位图为 1269×714，与索引一致；
- Web 定向 25 tests PASS；`git diff --check` PASS；
- `web/app/operating-pulse/page.tsx:148-162` 的 requestID gate 实现存在，但当前 R4 浏览器证据未实际触发“旧响应晚到”。

## 5. 重新提交条件

1. 关闭两个 P1：确定性旧晚响应 race；三类低证据结果进入原始产品输出与 trace；
2. 修正实际浏览器 URL/status 证据和 residual exact selector；
3. 只使用唯一 R5 fixture，先取证、再按完整精确 ID teardown，并附 residual=0；
4. 只跑相关 Agent/前端 request-gate 定向测试、type-check/build、数据库 residual 和运行态 smoke；无需重复无关全量返工；
5. 更新报告、清单、Evidence index、任务与看板；没有原始证据的项不得勾选；
6. 完成后回 `IN_REVIEW` 并停止等待 Review 6；不进入 MAX-010，不 commit/push。
