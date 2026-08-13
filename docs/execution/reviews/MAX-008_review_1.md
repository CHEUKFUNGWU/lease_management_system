# MAX-008 Review 1

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 总体判断

增量产品方向正确：既有 `/ai-chat`、会话/上传/Artifact 框架、AppLayout、旧 Skill/Tool、合同与 IFRS 16 页面均保留；三个零售 read tool 复用已验收的 Pulse、Store 360、Scenario 服务，行动输出仍是 proposal，没有业务写入入口。定向 Go 与 Web 测试通过。

当前不能接受，原因是一个真实 PostgreSQL 验收阻断、两个作用域/证据错误和一组 Agent 失败态错误。修复范围只限经营正确性、来源追溯和验收清理，不扩展 UI、schema、审计或自主执行能力。

## 必须修改

### [P1] 真实 PostgreSQL 测试必败并留下大量 fixture

- 位置：`core-service/internal/agenttools/tools/retail_operations_postgres_integration_test.go:62-65`；
- Reviewer 在现有 Docker PostgreSQL 网络以 `-count=2` 执行，两个 iteration 均在 production fact 插入时报 `SQLSTATE 42P08: inconsistent types deduced for parameter $2`；同一个 `$2` 同时用于 `business_date` 和 `as_of_at`，分别需要 date / timestamptz；
- `t.Cleanup` 在 production 插入之后才注册，失败路径没有清理。Reviewer 只读盘点确认当前已留下 `4` 个 `AGENT-LE-*` 法人、`242` 家门店和 `43,440` 条 retail facts；
- 修复：为日期/时间使用独立参数或显式无歧义 cast；在创建第一个 fixture 后立即注册可靠 cleanup；清理本测试遗留的、可精确识别的 `AGENT-LE-*` fixture；补 cleanup residual 断言；真实 PG `-count=2` 必须 PASS，结束后上述 fixture 残留为 0。

### [P1] scope 过滤后仍携带过滤前 provenance

- 位置：`core-service/internal/agenttools/tools/retail_operations.go:343-398`；
- `filtered := *set` 会复制原始 `SourceSystems` / `DatasetVersions`，随后又把允许门店的值 append 进去。结果会重复来源；更严重的是 region/brand/store scope 过滤为空时，响应仍可保留未授权门店的 source/dataset 元数据；
- 修复：过滤前把 `filtered.SourceSystems`、`filtered.DatasetVersions` 清空，再只从允许事实重建、去重、排序；
- 测试：至少构造两家不同 scope、不同 source/dataset 的门店，断言允许 scope 只返回允许 provenance；零授权事实返回空 provenance，不泄漏也不重复。

### [P1] 单店诊断/情景被全法人 Pulse 错误阻断

- 位置：`core-service/internal/aiagent/retail_operations.go:333-360`；
- 页面通常只传 `store_id`，但前置 Pulse 只使用 `filters.StoreIDs`。因此单店 scenario/action 会先查询全部授权门店；任何无关门店缺数都可能令 `retailPulseInsufficient` 为 true，错误阻断一个证据完整的目标门店；
- 修复：`store_diagnostics`、`scenario_evaluate`、`action_draft` 的前置 Pulse 必须收窄到目标 `store_id`，不得用全法人总体 coverage 作为单店 evidence gate；Pulse summary 意图仍保留显式 repeated `store_ids`；
- 测试：目标门店 current/comparison 完整、另一授权门店缺数时，目标情景仍按 `pulse(target) -> diagnostics(target) -> scenario(target)` 完成；每个来源保留同一目标 store scope。

### [P1] Agent 将失败的诊断/情景标为 completed，且可能保留 0.90 confidence

- 位置：`core-service/internal/aiagent/retail_operations.go:379-425,454-469`；
- diagnostics intent 在 tool 失败时仍把三个 plan step 标为 `completed`；scenario tool 失败后仍沿用 diagnostics 的 complete evidence，可能返回 `confidence=0.90`，并把失败步骤标为 completed；`retailToolReason` 也没有保留 `data_unavailable`；
- 对 scenario/action，还必须显式检查 `Diagnostics.DecisionReady`，为 false 时不得调用 Scenario；
- 修复：失败 tool 的 trace 保持 `failed`；未执行步骤为 `needs_review`/等价未完成态；invalid context/越界假设设置 `needs_input=true`，data unavailable 使用稳定 reason 和 `evidence_status=insufficient`，confidence 为 0.40；不得生成 proposal；
- 测试：diagnostics data unavailable、diagnostics `decision_ready=false`、Scenario resulting-rate 越界和 Scenario data unavailable；逐项断言 tool 次数、reason、needs_input、confidence、plan/trace 状态和零 proposal。

## Reviewer 已通过的证据

- `go test ./internal/agenttools/tools -run 'TestRetail' -count=1`：PASS；
- `go test ./internal/aiagent -run 'TestRetailAgent' -count=1`：PASS；
- Web：10 files / 58 tests PASS；`npm run type-check` PASS；
- `git diff --check`：PASS；
- 认证 scope 经 AI Chat Runtime 的 `context.WithoutCancel` 保留，未发现异步 Run 丢失 region/brand/store scope；
- 三个工具仍为 Read，Skill allowlist 仅三项，行动仍为 `retail_action_proposal`，没有新增业务写工具。

## 重新提交验收

1. 完成上述四组定向修复，不新增 migration、不重做 UI、不修改旧租赁/IFRS 16 功能；
2. 更新 `docs/execution/reports/MAX-008.md`，把真实 PG 结果和 Reviewer 发现如实替换，不能继续写 Docker socket blocker；
3. 运行定向 Go、Web、type-check、build、vet、diff check；
4. 在现有 Docker 网络执行本票 PG test `-count=2`，记录两轮 PASS 与 residual=0；
5. 任务票、报告和看板保持 `IN_REVIEW`，等待下一轮 Reviewer；MAX-009 继续 `BLOCKED`。
