# MAX-009 Review 2

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 结论摘要

本轮确认 fixed-seed Pulse、Store360、Scenario 的主要 Golden 数字、四页基础桌面/移动截图以及全量 Go/Web 回归已有实质进展；既有 AppLayout、旧页面、IFRS 16 和 Official 语义未被删除或替换。Reviewer 真实 PostgreSQL 复跑也确认 MAX-002/003/004/006/007/008 的核心集成测试可通过，MAX-007/008 连续两轮通过且专用测试残留为 0。

但 MAX-009 是发布验收票，不能仅以自动化和基础页面截图替代任务票要求的完整真实浏览器闭环。当前仍有 5 项 P1：移动端关键操作不可达、E2E fixture 未清理、五条底线 smoke 不完整、AI/action/负向主链证据不足、旧路由与 Network Gate 被提前勾选。因此当前报告中的 `GO` 不成立。

本轮没有发现删除旧功能、重排 AppLayout、扩展地产能力或触达 IFRS 16 正式台账的 scope creep。返工只允许修复以下发布缺口，不新增产品能力、schema、高级审计或 UI 重构。

## 2. 必须修复（P1）

### P1-1：移动端 More / 删除按钮仍会被 CSS 层叠隐藏

证据：

- `web/app/globals.css:939-941` 在 mobile media query 中设置 `.session-more-btn { opacity: 1 !important; }`；
- 但文件后部 `web/app/globals.css:1045-1048` 以相同 specificity 再设置 `opacity: 0 !important`，后声明规则覆盖移动端规则；
- 触屏没有可靠 hover，现有有效移动截图又没有打开 Drawer，无法证明删除操作可达。

整改与验收：

1. 只调整 AI Chat scoped CSS 层叠顺序或 selector，使 390/767 下 Drawer 中 More 按钮始终可见，768/1440 保持原桌面 hover/focus 行为；
2. 在 390×844 真实打开 Drawer，保存包含 More/删除按钮的截图；用键盘验证 More、取消和焦点返回；
3. 增加能对实际 class/渲染合同产生约束的测试，不能只测试数值 breakpoint helper。

### P1-2：E2E dataset/action 与任务要求的精确清理冲突

任务票第 2、3、4.4、7、9 节要求所有本票 fixture 使用唯一标识并在验收结束精确清理，残留为 0。报告 `docs/execution/reports/MAX-009.md:107,121` 和清单 `docs/execution/MAX-009_发布检查清单.md:30` 却明确保留固定 dataset 与主链 action。

Reviewer 只读核对时仍存在：

- dataset version `retail-sim-v1-2853d9537f358baf`，60 stores / 10,860 facts；
- action `3f98f288-dfe7-4fe2-851e-fafc328d8221`，`status=open`。

整改与验收：

1. 先完成缺失的浏览器与 API 证据，再按精确 dataset ID/version、batch ID、action ID 和临时 username 在一个可回滚事务中清理；不得 wildcard、不得重置 volume；
2. 清理顺序先经只读 FK 检查确认；只删除本票 facts/stores/dataset/batch/action、临时用户/session 及其本票依赖；
3. 报告记录 before/affected/after，最终逐类 `residual=0`；截图、脱敏 trace/artifact JSON 可作为持久证据，不能用生产数据库残留充当证据。

### P1-3：五条底线没有同时满足“自动化 + 本票浏览器/API smoke”

证据：

- 报告 `:84` 的法人隔离仅复用前票 PG；本票 prompt injection 使用跨租户 `admin_user`，不能证明 LE001 身份看不到 LE002；
- 报告 `:87` 只复用 action replay，未在本票复演 fixed-seed replay、事实导入 replay 和异 payload conflict；
- 报告 `:88` 没有本次 E2E 前后正式表计数/关键更新时间快照。

整改与验收：

1. 使用本票唯一 LE001、LE002 scoped 临时用户（或等价现有 scoped 账号）完成至少一个 API/browser A/B smoke：LE001 可见 A，LE002 请求 A 的 store/fact/source/action/AI 上下文统一不可见；完成后精确清理临时身份；
2. 本票实际执行 fixed-seed 同 payload replay、事实导入或等价已暴露幂等入口 replay、异 payload 冲突、action replay，并记录请求语义、status、行数/ID 前后不变；
3. 在最终 E2E 前记录 `lease_contracts`、`lease_events`、付款计划、measurement、journal、monthly closing/Official 的 count 与 max updated timestamp；E2E 后用同一查询逐项对数为不变；
4. production/simulated 与来源 round-trip 继续保留真实 UI/API 证据。

### P1-4：AI proposal、人工 action 和负向主链证据不足

证据：

- `valid-ai-chat-*` 只显示 tool trace/待确认提示，没有完整 proposal 卡、Owner/Due、`formal_execution=false`、`business_write=false` 和可点击来源 round-trip；
- `valid-action-confirm-*` 是保存后的 replay 状态，不是二次确认 Modal；
- 临时用户清理后，当前数据库在本轮时间窗内没有可核验的 `retail_action_proposal` Artifact；
- 没有真实复演修改假设后的 stale/禁止保存，也没有依次覆盖 needs-input、partial/no facts、invalid resulting rate。

整改与验收：

1. 在真实 `/ai-chat` 完成 pulse → diagnostics → scenario 三个 read tool，显示并截图 proposal 卡、模拟数据/来源、Owner/Due 空、两个 false 边界；点击至少一个 source 并记录 round-trip URL/页面；
2. 在清理临时用户前导出脱敏 run trace 与 proposal Artifact JSON 到 `docs/execution/evidence/MAX-009/`，索引写明 run/artifact ID、classification/dataset/as-of；不得包含 token、密码或提示词敏感内容；
3. Scenario Evaluate 后修改任一假设，真实验证旧结果标 stale 且保存禁用；重新 Evaluate 后再继续；
4. 保存前截图真实二次确认 Modal，确认后只生成一条 open action；相同 key replay ID 不变，之后按 P1-2 删除；
5. 用真实 UI/API 依次记录缺 dataset/store/七项假设、partial/no facts、invalid resulting rate 的 needs-input/0.40/failed trace，确认不产生 scenario/proposal/action。

### P1-5：旧 route 与 Network Gate 被提前标为 PASS

证据：

- 任务票要求 `/`、`/performance`、`/contracts`、`/ai-chat`、`/upload`、`/monthly-closing`、`/reports`、`/audit-logs`、`/settings` 及其他现有 route；报告 `:74` 只列出其中 5 个；
- 报告 `:76,112` 明确没有 Resource Timing/HAR 或逐请求证据，却在发布清单 `:24,28` 全部勾选；
- 没有记录 stale/cancel request 不覆盖新结果的浏览器/API 证据。

整改与验收：

1. 至少逐项打开上述 9 个旧 route，记录最终 URL、主内容存在、无 404/白屏/无限 loading；不得删除、隐藏、重命名、重排旧入口；
2. 通过当前浏览器可用的 network/resource 记录、同源 API smoke 或服务日志，逐步记录主链关键 request URL、HTTP status 和 dataset/store scope；不能只写“页面可见无错误”；
3. 快速切换筛选或触发已有 cancel/stale gate，记录旧 response 没有覆盖新上下文；
4. 若工具确实无法导出 HAR，可如实声明没有 HAR，但仍须提供上述可复核请求/status 证据；完整 HAR 不是要求，关键请求正确性是要求。

## 3. 可同轮收口的代码质量项

以下不单独扩展范围，但应随 P1-1 一并整理：

1. `web/app/ai-chat/page.tsx:826-832` 的 selectable session 使用 `role=button` 却配 `aria-current=page`。改为原生 `button` + `aria-pressed`，或使用一致的 listbox/option + `aria-selected`；More 按钮保持 sibling，避免嵌套 button；
2. `web/app/ai-chat/responsive.ts:12-13,27-28,32,45` 的 `compactHeader`、`drawerDefaultOpen`、`escape` 若没有真实消费路径则删除；不要保留测试可通过、运行时不使用的状态合同；
3. `docker-compose.yml` 中关于浏览器 host loopback 的注释若已改为同源相对 URL，应同步事实，避免运维文档误导。

## 4. 已独立确认可保留的结果

- fixed-seed Golden 主指标：420/420、rank 1、severity high、score 3.02、occupancy cash cost rate change 10.08pp；
- Scenario labor -10% 的核心确定性结果和 bridge 守恒；
- 四个经营页面基础 desktop/mobile 无明显页面级横向 overflow；
- 既有 AppLayout、合同、上传、月结、报表、审计、设置、IFRS 16/Official 未被删除或替换；
- 全量 Go test/vet、Web tests/type-check/build 和真实 PG 主回归此前已通过；最终提交仍需重跑并记录。

## 5. 重新提交条件

只修复以上缺口。完成后：

1. 更新报告、evidence index、演示脚本与发布清单，所有勾选项必须有对应真实证据；
2. 重跑全量 Go/vet/Web/type-check/build/diff check 及任务要求的 PG 测试；
3. 清理本票 dataset/action/temp identity 等全部 fixture，逐类 residual=0；
4. 将任务票、报告和看板改回 `IN_REVIEW`，结论可以由 Executor 建议 `GO`，但不得自行标 `ACCEPTED`；
5. 停止等待 Reviewer，不创建 MAX-010、不 commit、不 push。

