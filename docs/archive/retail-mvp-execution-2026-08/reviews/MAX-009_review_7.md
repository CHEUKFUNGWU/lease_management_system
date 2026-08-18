# MAX-009 Review 7

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
发布结论：`NO-GO`  
任务状态：退回 `IN_PROGRESS`  
日期：2026-08-13  
Reviewer：Codex 主任务

## 1. 总结

Review 6 的生产正确性阻断已经关闭：caller-controlled `dataset_version` 不再把真实 `scope_denied` 改写为 `no_facts`；越权回归、R4 网络证据降级、positive session/run exact residual、正式 IFRS 16 台账和关键自动化均通过。未发现新增页面、route、schema、migration、tool、指标、UI、AppLayout 或 IFRS 16 scope creep。

本轮不再要求产品功能改动，只剩测试语义与发布证据一致性收口。不得新增 fixture、页面、route、schema 或能力；不得改旧 UI/IFRS 16。完成后回 `IN_REVIEW` 等待 Review 8。

## 2. Standards 轴

### P1 hard — 空事实回归使用 Global scope，未覆盖普通法人范围

`core-service/internal/aiagent/retail_operations_test.go:308-316` 清空门店 population 后使用 `Global:true`；`scopedRetailReader` 对 Global scope 直接返回原 set。虽然这不构成生产越权，且魔法名称分支已正确删除，但该测试没有覆盖 Review 6 Standards 要求的“普通已授权法人范围 + 结构化空事实”。

修复：改为非 Global 的 `access.Scope{LegalEntityID: "entity-a"}`，不配置窄化 StoreIDs；让 scoped reader 在法人范围内得到结构化空 population，再由 Pulse 的结构化状态映射 `no_facts`。保留另一条“不在授权 StoreIDs 内仍为 `scope_denied`”的回归。

### P2 hard — 发布清单与演示脚本状态自相矛盾

`docs/execution/MAX-009_发布检查清单.md:14,31` 仍保留 Review 6 前的两个未勾选待修项，但同文件 46-50 已声明完成；`docs/execution/MAX-009_演示脚本.md:3` 仍写等待 Review 6。必须统一为 Review 7 收口后的真实状态，不得同时声称待修与完成。

### P2 hard — R4 四个临时用户的 residual 声明超过证据

`docs/execution/evidence/MAX-009/r4-residual.json:35-43` 只列出一个 user/refresh selector，但 `docs/execution/reports/MAX-009.md:123` 声称四个 Admin fixture users 与 refresh sessions 均有精确 residual。补齐四个用户及 refresh session 的可复现 exact UUID/username selectors 和 actual=0；若历史 UUID 无法恢复，必须下调报告声明并提供不依赖 wildcard 的真实可复核证据，不能保留“四个均已精确证明”的表述。

Standards 轴无额外 baseline smell；定向 test/vet/diff check 通过。

## 3. Spec 轴

### P2 — 测试没有生成 trace，且 no-action 声明超过断言

`core-service/internal/aiagent/retail_operations_test.go:314-316,337-340` 直接以 `emit=nil` 调用 `executeRetailOperations`，只检查 Response answer、proposal nil 与 ProjectResult artifact=0，没有生成 run `message_end` trace，也没有断言 `RetailOperations.SideEffects=false` 或 action DB。

因此 `docs/execution/reports/MAX-009.md:236-237` 与 Evidence index 的“两例 assistant/trace、无 action”表述超过现有证据。最小收口：

1. 两例测试均显式断言 `SideEffects=false`、proposal nil、ProjectResult artifacts=0；
2. 文档将本轮证据准确表述为 deterministic Response/ProjectResult；不得把 `emit=nil` 单测称为 raw trace 或 action DB 证据；
3. raw `message_end` trace 只能引用既有真实 R5 evidence；若要声称新增 scope-denied trace，则必须用真实 runtime 生成并可复核，不能人工拼接。无需为本票新增数据库 fixture。

### P2 — 发布清单仍把已完成事项标为待修

`docs/execution/MAX-009_发布检查清单.md:14,31` 与同文件的 `IN_REVIEW`、Review 6 完成区块冲突，发布状态不真实。修正后全文件不得再有已关闭事项的未勾选条目。

Spec 轴其余通过：危险分支已删除；越权名称不再改变 reason；Global 在产品定义中属于授权的无限维度 scope，但本轮仍按 Standards 要求改成普通法人范围以增强验收；R4 historical 标注与 positive session/run residual 正确；无 scope creep。

## 4. Reviewer 独立验证

- Go aiagent/agenttools `-count=1` tests 与 vet PASS；
- Web request-gate test、type-check、production build PASS；
- 五服务运行，PostgreSQL/MinIO healthy，`pg_isready` PASS；
- 正式表 count 仍为 `29/0/224/10/17/5`，max timestamps 未变化；
- R4 positive session/run 当前数据库均为 0；
- R4 `/api/v1/...` 已正确标为 historical invalid/`normalized_core_endpoint`；
- `git diff --check` 与 R4 JSON parse PASS。

## 5. 重新提交条件

1. 将 no-facts 回归改为非 Global、法人已授权的结构化空 population；
2. 两条语义回归显式断言 `SideEffects=false`、无 proposal/artifact，并准确区分 Response 证据与 raw trace/action DB 证据；
3. 清除发布清单的两个过期待修项，演示脚本更新为等待 Review 8；
4. 补齐或如实降级 R4 四用户/refresh exact residual 声明；
5. 跑 Go 定向 tests/vet、Web gate test/type-check/build、JSON parse、正式表与 residual、`git diff --check`；
6. 更新任务、报告、清单、演示脚本、Evidence index、看板为 `IN_REVIEW` 后停止等待 Review 8；不进入 MAX-010，不 commit/push。
