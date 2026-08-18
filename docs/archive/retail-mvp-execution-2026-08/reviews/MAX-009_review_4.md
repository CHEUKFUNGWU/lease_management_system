# MAX-009 Review 4

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 总结

Review 3 的运行态与产品缺陷已基本收口。Reviewer 已独立确认：原 PostgreSQL volume healthy；正式 IFRS 16 表的 count/max 未变化；023 已回撤、040 在 existing/fresh-init 两条路径均成立；11 类 MAX-009 fixture residual 为 0；全量 Go/vet、Web 73 tests/type-check/build/diff 与两组真实 PostgreSQL `-count=2` 均通过；旧 routes、AppLayout、既有页面及 IFRS 16/Official 边界完整保留。

本轮不再发现产品实现 P0，也未发现 scope creep。但发布票仍有三项 P1 取证缺口：race 证据不是同一 UI 的真实快速切换；三类 Agent 负向路径只有人工汇总、没有逐 Run 原始 trace；跨法人 action/artifact smoke 没有先证明目标对象在 LE001 确实存在。清单把这三项提前勾为 PASS，故当前仍不能 GO。

本次返工是纯发布取证收口。不得新增功能、改 AppLayout/整体 UI、删除或隐藏旧页面、修改 IFRS 16/Official、扩 schema 或审计范围；不得重跑或重建已正确的产品模块。

## 2. Standards 轴

### P2 — 响应式测试仍未覆盖真实渲染 wiring

`web/app/ai-chat/responsive.test.ts:39` 所称 rendered contract 仍只比较常量和 helper 输出；若 `page.tsx` 不再挂载这些 props、CSS hover/移动可见性回归或回焦调用被删除，测试仍会通过。`responsive.ts:2-14` 因此也形成轻微 Middle Man / Speculative Generality。

该项不阻塞本票，登记为 `HARD-012`。后续应使用现有测试栈覆盖真实 DOM：desktop sidebar vs mobile trigger/Drawer、active/normal More、仅当前行 hover/focus，以及 selection/close 后 focus。当前真实浏览器截图已证明关键操作可达，因此不以此治理增强阻塞发布取证。

Standards 轴其余无 finding。

## 3. Spec 轴

### P1 — race/cancel 证据并非同一 UI 的真实快速切换

`docs/execution/evidence/MAX-009/r3-race.json:3-25` 只把一个直连 `127.0.0.1:18080` 的外部进程终止为 status 143，再单独记录浏览器最终 DOM。它没有证明同一浏览器页面快速切换 scope/filter 时产生重叠请求，也没有证明旧响应被页面取消或由 `requestID` gate 忽略。

修复：在真实 `/operating-pulse` 页面连续快速切换 window 或 store，保留同一次浏览器会话的旧/新请求 URL、开始/结束顺序、status 或 cancellation，以及最终 URL/DOM scope。可使用延迟注入、浏览器路由拦截或测试专用可控响应顺序，但不得改产品合同；证据必须能证明旧请求晚到也不会覆盖新 context。

### P1 — Agent 负向证据是人工摘要，不足以逐例复核

`r3-negative-agent.json:2-60` 只有汇总字段；`confidence=0.40` 只在文件顶层，needs-input、partial、no-facts 三个 run 缺逐事件 trace/status payload/response，invalid-rate 也没有对应 run/trace。对象随后已清理，Reviewer 无法独立复核每例的真实输入、工具顺序、confidence/evidence 与零写入。

修复：用新的唯一 R4 fixture 重跑四例。每例保存脱敏原始 API response、session/run ID、run status、按时间排序的 run events/tool results、该例自己的 confidence/evidence/reason，以及 proposal artifact/business action before/after。invalid-rate 若按产品合同不创建 Agent run，应明确记录实际 endpoint/request/422 response，并说明为何无 run，同时保留零写入查询。完成取证后按精确 ID 清理，并在同一证据文件记录 residual=0。

### P1 — 跨法人 action/artifact 未建立“目标先存在”的正反证据

`r3-cross-scope.json:14-39` 的 action 只查询空队列；artifact 没有目标 artifact ID，只由 run/session 404 间接推断。证据没有证明 smoke 当时 LE001 的目标 action/proposal artifact 确实存在，因此不能排除“全局本来为空”。

修复：使用唯一 R4 对象，在同一取证窗口先以 LE001 读取目标 source、action、run/session/proposal artifact 成功并记录精确 ID；再以 LE002 对相同目标 ID/过滤条件读取，记录 404 或空 envelope。若产品没有独立 artifact GET 路由，使用已有 owner-filtered run/session 详情中可观察的同一 artifact ID，并如实说明访问合同；不得新增路由。取证后精确清理并验证 residual=0。

### P2 — AI proposal 补充截图不是实际 1440×900 位图

`r3-ai-proposal-1440x900.png` 实际为 1429×893。Evidence index 已解释为内容区截图，但文件名和发布票仍要求真实 1440×900。以浏览器 viewport 全页/视口截图重拍一张真实 1440×900，或如实重命名并把它降为非 viewport 证据；不得只靠文件名宣称尺寸。

## 4. 已独立验证

- 原 `lease-postgres` volume healthy，`pg_isready` 成功；
- 正式表 count 为 `29/0/224/10/17/5`，max timestamp 与 R2 before/after 一致；
- 023 未追改；040/current constraint 保留全部旧 artifact type 并加入 `retail_action_proposal`；fresh init 合同一致；
- R3 11 类 exact selector residual=0；历史 R2 dataset/action 也为 0；
- Go/vet、Web 73 tests/type-check/build、`git diff --check` 通过；
- repository 与 agenttools 真实 PostgreSQL `-count=2` 串行通过；
- 基础八张双 viewport 图片尺寸真实；AI Chat 390×844 active/normal More 可见；
- 旧 routes、旧 UI、AppLayout、IFRS 16/Official 构建与数据边界保留。

## 5. 重新提交条件

1. 只补上述三项 P1 原始证据；P2 截图同轮如实收口，不做功能扩展；
2. 每组临时对象先记录 LE001 正向存在，再记录负向/跨法人结果，最后按精确 ID 清理并证明 residual=0；
3. 真实 UI race 必须证明旧请求晚到也不会覆盖新 scope；
4. 更新报告、清单、Evidence index、任务和看板；没有原始证据的清单项不得勾选；
5. 仅重跑与本轮取证有关的定向测试、数据库 residual 和运行态 smoke；无需重复全量实现返工；
6. 完成后回 `IN_REVIEW`，停止等待 Review 5；不创建 MAX-010，不 commit/push。
