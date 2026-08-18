# MAX-009 Review 6

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 总结

Review 5 的两个核心收口已大体实现：经营脉搏页面确实使用被测的 `createLatestRequestGate`，deferred 测试让新响应先提交、旧响应后到并被拒；needs-input/no-facts/invalid-rate 的原始 `message_end` 已直接包含 reason/evidence/confidence，invalid-rate 保留 `resulting_rate_out_of_range`；R5 fixture 当前 residual 为 0；正式 IFRS 16 表 count/max 不变。

但本轮引入一项新的 P1 正确性问题：生产 Agent 仅依据调用方可控的 `dataset_version` 字符串是否包含 `no-fact`，把真实 `scope_denied` 改写为 `no_facts`。这会掩盖权限边界，直接触及不可降低的权限/法人隔离底线，因此发布仍为 NO-GO。正确的结构化判断已在同文件后续基于 Pulse 事实状态实现，本轮只需删除魔法命名分支并补权限回归，不需要扩产品能力。

不得新增页面、route、schema、migration、tool、指标或 UI；不得改 AppLayout、旧页面和 IFRS 16/Official。只修该 P1，并收正文档/证据 P2。

## 2. Standards 轴

### P1 hard correctness — 验收 fixture 命名泄漏进生产权限逻辑

`core-service/internal/aiagent/retail_operations.go:342-347` 根据 caller-controlled `dataset_version` 是否包含 `no-fact`，把真实 `scope_denied` 改写为 `no_facts`。任意越权请求都可通过版本命名掩盖权限拒绝，违反 AGENTS.md 的 AI 权限范围与引用正确红线。

修复：删除该分支。Pulse tool 已拒绝且没有可信结构化事实状态时必须保留 `scope_denied`；只有获得可信 Pulse 响应并由结构化 coverage/fact 状态判断为 no facts 时，才允许映射为 `no_facts`。补回归：dataset 名即使包含 `no-fact`，真实 scope denial 仍输出 `scope_denied`；合法授权门店的结构化空事实仍输出 `no_facts`。

该实现同时构成 judgement-call 的 Mysterious Name / Primitive Obsession，但阻断原因是上面的 hard correctness，不是气味本身。

### P2 hard evidence — R4 网络证据的旧错误描述仍未失效

`docs/execution/reports/MAX-009.md:101` 与 `docs/execution/evidence/MAX-009/index.md:47,56` 仍把 R4 `/api/v1/...` 称为完整同源/request URL，与 Review 5 和 R5 的正确说明自相矛盾。

修复：把 R4 网络 URL/status 明确标为历史无效网络证据或 `normalized_core_endpoint`；不得继续与有效证据并列。R5 gate 测试及 route pattern 才是当前有效 race 证据。

### P3 judgement call — 未使用的公开方法

`web/app/operating-pulse/requestGate.ts:3` 暴露了生产与测试都未调用的 `isCurrent`。可收为闭包内部实现，但不阻塞本票，也不要求为此扩测试。

Standards 轴其余通过；未发现页面、route、schema、tool、AppLayout、UI 或 IFRS 16 scope creep。

## 3. Spec 轴

### P1 — no-facts 的实现依赖调用方魔法命名

Review 5 只授权把低证据 reason 投射到既有原始输出，不授权让测试数据集命名决定业务/权限语义。`retail_operations.go:342-347` 的实现会把真实权限拒绝错误解释成数据缺失。

R5 实际链路不需要该分支：Pulse 已成功返回，随后 diagnostics 的原始 `scope_denied` 可由 `retail_operations.go:405-410` 基于可信 Pulse 的结构化 no-facts 状态正确映射。删除前一分支不会破坏已验收方案。

验收：

1. scope-denied + 任意 dataset 名（包括包含 `no-fact`）仍为 `scope_denied`；
2. 已授权 scope + 结构化空事实才为 `no_facts`；
3. 两者 assistant message 均保留实际 reason/evidence/confidence，且无 proposal/action。

### P2 — R4 positive session/run 未纳入 residual 文件

`r4-cross-scope.json:668-670` 的 positive session `66fbe25c-e12e-46b1-b7d0-a02c6c74199e` / run `19dee11e-2090-4987-9416-c555b6255ea2` 未列入 `r4-residual.json:46-53`，当前文件只列四个负向 session/run。Reviewer 独立查询确认两者当前均为 0，但报告“完整 exact selector”仍不准确。

修复：把这两个精确 ID 加入 R4 selector 并保留 actual=0，或如实拆分为 negative/positive selectors；不得使用 wildcard。

Spec 轴其余通过：同一 production gate 的 deferred 测试、三类原始 Agent 输出、R5 residual、正式台账隔离均成立；未发现 scope creep。

## 4. 独立验证

- 页面 `page.tsx:135,151-164` import 并实际使用 `createLatestRequestGate`；
- deferred 测试明确断言 new commit=true、late old commit=false、最终 state=`new-7`；
- R5 raw `message_end` 直接含三类 reason/evidence/confidence；
- R5 users/dataset/facts/stores/sessions/runs/artifacts 当前查询均为 0；
- 正式表 count/max 仍为 `29/0/224/10/17/5`，与此前快照一致；
- 五服务运行，PostgreSQL/MinIO healthy，`pg_isready` PASS；
- Web 定向 17 tests、type-check、production build PASS；
- Go aiagent/agenttools 定向 tests PASS；`git diff --check` PASS。

## 5. 重新提交条件

1. 删除 caller-controlled dataset 名称到 `no_facts` 的分支；
2. 补 scope_denied 不被魔法名称改写、结构化空事实正确映射的定向测试；
3. 仅在必要时用唯一 R6 fixture 复演两例；若创建 fixture，按精确 ID cleanup/residual=0；
4. 修正 R4 网络证据历史标记，并把 positive session/run 加入 exact residual；
5. 跑 Go aiagent/agenttools 定向测试与 vet、Web request-gate 测试/type-check/build、数据库 residual、`git diff --check`；
6. 更新报告、清单、Evidence index、任务和看板后回 `IN_REVIEW`，停止等待 Review 7；不进入 MAX-010，不 commit/push。
