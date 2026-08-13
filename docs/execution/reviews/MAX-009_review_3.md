# MAX-009 Review 3

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 总结

Review 2 的主要正向闭环已有可信证据：fixed-seed Golden、Pulse → Store360 → Scenario、Scenario stale、AI 三项 read tool、`retail_action_proposal`、人工确认 Modal、四页基础双 viewport，以及 IFRS 16 正式表 before/after JSON 均有实质改善。既有 AppLayout、旧页面和 IFRS 16/Official 产品语义仍被保留。

但发布验收仍不能通过。当前 `lease-postgres` 已因 Docker 存储空间耗尽退出；报告中的 PostgreSQL healthy 与当前运行态直接矛盾。证据包又把缺失的 needs-input/partial/no-facts、跨法人 source/action/AI、完整 cleanup residual 和 Network race/cancel 提前勾为 PASS。另有 Artifact schema 通过追改既有 023 migration 和手工 ALTER 才在当前库生效，fresh init 的后段约束反而会再次移除 `retail_action_proposal`，不是可安全部署的迁移链。

返工继续遵守原产品保护边界：不删除、隐藏或重命名旧功能/页面，不重排 AppLayout，不扩地产、审计或自动执行能力。仅允许恢复可发布运行态、修复最小 UI 缺陷、补安全增量迁移与缺失证据。

## 2. Standards 轴

### P1 — 当前选中会话的移动 More 图标对比度为零

`web/app/ai-chat/page.tsx:922` 在 active session 时把 More sibling 设为 `var(--fg-inverse)`；但 active 黑底只在左侧选择 button，More 所在 sibling 背景仍是白色。真实 Drawer 截图也显示第一行 More 近乎不可见。

修复：把 active 背景提升到完整 session row，或让 sibling 在白底使用可见前景色。390×844 同时截图 active/non-active 两行 More，computed opacity=1 且视觉可见。

### P1 — 桌面 hover selector 会显示所有 More

`web/app/globals.css:1046-1048` 使用 `*:hover .session-more-btn`。当 `html`/`body` 处于 hover 时，所有 descendant More 都满足，不能实现报告声称的“仅当前行 hover/focus”。

修复：给 session row 明确 scoped class，以 `.row:hover` / `.row:focus-within` 显示该行 More；移动后置 override 保持可见。补运行时或组件测试。

### P1 — 390px 首次 hydration 的服务端/客户端 DOM 不一致

`web/app/ai-chat/page.tsx:1490-1494` 服务端固定初值 1440，客户端首次 render 直接读取 `window.innerWidth`；390px 时服务端产生 desktop sidebar，客户端首次产生 mobile trigger/Drawer，存在 hydration mismatch 风险。当前 Console 证据未覆盖冷加载首次 hydration。

修复：服务端和客户端首次 render 使用同一稳定初值，挂载后再更新 viewport；或使用不会改变 SSR DOM 的 CSS/稳定 media-query 方案。增加冷加载 390px 验证，Console 无 hydration warning。

### P2 — 响应式测试仍主要自证 helper 常量

`web/app/ai-chat/responsive.ts:18,31` 的 `compactHeader` 只添加没有 CSS/行为的 class；`responsive.test.ts:40-51` 比较导出的 class 字符串和 props helper，仍无法在删除实际 SessionSidebar wiring、CSS computed visibility或焦点恢复时失败。

修复：删除未消费的 `compactHeader`，并增加最小 rendered/component contract，至少覆盖 sidebar vs trigger、active/normal More 可见性、选择/close 后焦点回 trigger。不得引入新 UI 框架。

## 3. Spec 轴

### P0 — PostgreSQL 当前宕机，发布状态与报告矛盾

独立检查：

```text
lease-postgres  Exited (1)
PANIC: could not write ... No space left on device
FATAL: could not extend file ... No space left on device
```

这与 `docs/execution/reports/MAX-009.md:18` 的 PostgreSQL healthy 矛盾，也使 Reviewer 无法独立查询 residual。当前不能 GO。

恢复与验收：

1. 先记录 `docker system df`；只允许清理可重建的 Docker build cache/未使用构建缓存，不删 volume、不 `down -v`、不 prune volume；
2. 释放空间后只启动原 PostgreSQL 容器，让 WAL recovery 完成；保留同一 volume；
3. 检查日志无新的 PANIC/FATAL、`pg_isready` 成功、Core API 恢复；对正式表 count/max 与 R2 before/after 快照再次对数；
4. 再跑任务指定真实 PG tests 和 residual 查询。若原 volume 无法无损恢复，立即保持 NO-GO 回报，不得重建空库冒充恢复。

### P1 — 三类负向 Agent 证据缺失

`docs/execution/MAX-009_发布检查清单.md:14` 勾选 needs-input/partial/no-facts/invalid-rate 全部有真实证据，但 `r2-api-smoke.json:46-50` 只有跨法人 404 和 invalid-rate 422；positive AI JSON 只有成功 proposal。没有 needs-input、partial、no-facts 的 run/trace/status/confidence=0.40，也没有各自 artifact/action before/after 为零。

修复：使用唯一 R3 fixture 或可回滚测试场景，分别记录三类输入、HTTP/run status、reason、confidence/evidence、tool trace、proposal/artifact/action 前后计数；结束精确清理。不能只引用单元测试文字。

### P1 — 跨法人隔离 smoke 与 Network race/cancel 仍不完整

`r2-api-smoke.json:34-45` 只覆盖 KPI/Pulse/Scenario；没有 Review 2 要求的 source、action、AI answer/artifact 跨法人不可见。报告也仅声明 stale disabled，未提供快速 scope 切换/取消后旧 response 不覆盖新 context 的 request URL、status、scope 与最终页面证据。

修复：

1. scoped LE002 对 LE001 source/action/AI run/artifact 各做一次不可见 smoke，记录 endpoint/status/response shape；
2. 实际触发一次快速筛选/scope 切换，记录新旧请求标识、status/取消结果和最终 DOM scope；
3. HAR 仍非必需，但关键 URL/status/scope 必须进入脱敏 JSON。

### P1 — cleanup 证据没有覆盖报告声称的全部对象

`r2-formal-after.json:8` 只记录 action/dataset 1→0；没有 facts、stores、batch、fact request、temporary users、refresh/AI sessions/runs/artifacts 的 exact ID 与 residual。报告文字不能替代查询结果，且 PostgreSQL 当前停机使其无法独立复验。

修复：数据库恢复后输出单一脱敏 `r3-residual.json`，逐类包含精确 selector（ID/version/key/username）、expected 0、actual 0 和查询时间；同时确认 MAX-009 历史 dataset/action 也为 0。禁止 wildcard 清理。

### P1 — Artifact schema 修复不可部署

Executor 修改了已存在且可能早已应用的 `db/migrations/023_agent_artifact_protocol.sql:22-36`，并在当前库手工 ALTER。既有实例不会自动重跑 023；同时 `db/init/01_init.sql:1285-1298` 的后段 023 合并块仍会重建一个不含 `retail_action_proposal` 的约束，因此 fresh init 最终也会拒绝 proposal。数据库日志已记录过该 check violation。

Planner 授权一个唯一例外：允许新增最小安全增量 migration（建议下一序号 `040_agent_artifact_retail_proposal.sql`）仅修复此约束。要求：

1. 不追改既有 023 migration；新 migration 幂等 drop/re-add constraint，并保留全部旧 artifact types；
2. `db/init/01_init.sql` 的最终约束与新 migration 完全一致，fresh init 最终允许 proposal；
3. 在恢复的既有 volume 显式应用新 migration，并查询 `pg_get_constraintdef`；
4. 用临时事务插入/回滚一个 `retail_action_proposal`，同时验证旧 artifact type 仍可插入；
5. 不新增表、业务字段、能力或审计范围。

### P2 — 三张截图 viewport 元数据不实

Evidence index 把 `r2-scenario-stale-1440x900.png`、`r2-action-confirm-modal-1440x900.png`、`r2-ai-proposal-1440x900.png` 标为 1440×900；独立 `sips` 检查均为 1280×720。基础八张 viewport 正确，但这三张命名/索引不实。

修复：以真实 1440×900 重拍，或重命名并在索引如实标为 1280×720；不得把文件名当证据。

## 4. 已独立验证

- `web/app/ai-chat/responsive.test.ts`：14 tests PASS；
- `npm run type-check`：PASS；
- `git diff --check`：PASS；
- R2 基础四页 desktop/mobile 截图尺寸正确；
- stale、confirm Modal、positive proposal 内容真实存在，但三张补充图实际为 1280×720；
- positive Artifact JSON 含三项 read tool、simulated provenance、Owner/Due 空、`formal_execution=false`、`business_write=false`；
- 正式表 before/after JSON 内容一致，但需数据库恢复后复核。

因 PostgreSQL 宕机，未将此前 Executor 的全量 PG PASS 当作当前发布证据。

## 5. 重新提交条件

1. 修复以上 P0/P1；P2 可同轮收口；
2. PostgreSQL 原 volume 无损恢复并保持 healthy，正式表/fixture 对数通过；
3. 安全增量 migration 在 existing volume 与 fresh-init 最终 schema 两条路径验证；
4. 补齐负向、跨法人、Network/race 和完整 residual JSON；
5. 重跑全量 Go/vet/Web/type-check/build/diff、任务指定 PG tests；
6. 更新报告、清单、Evidence index、任务与看板，只在证据全部闭环后回 `IN_REVIEW`；停止等待 Review 4，不创建 MAX-010、不 commit/push。
