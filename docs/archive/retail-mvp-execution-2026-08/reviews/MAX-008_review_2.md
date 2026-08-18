# MAX-008 Review 2

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`（两项收尾）  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已关闭

Review 1 的 production date/timestamptz 参数、cleanup 注册时机、scope provenance 重建、单店 Agent Pulse 目标收窄、diagnostics/scenario 失败态、decision-ready gate、stable reason、confidence 和 proposal 抑制均已见实现与定向测试；Go Retail tests、Web 58 tests、type-check、build 通过。

## 仍需修改

### [P1] Store diagnostics 的授权同群被误删

- 位置：`core-service/internal/agenttools/tools/retail_operations.go:335-342`；
- 新增逻辑把 `requestedStoreIDs` 在三个 Tool 共用 reader 中都作为 Repository predicate。`retail.store_diagnostics.read` 的底层 Store 360 服务需要读取授权范围内的同品牌/区域/币种门店来计算 peer benchmark；现在 Repository 只返回目标店，peer 永远不足；
- 修复：不要把 `requestedStoreIDs` 通用地变成 Repository store filter。Agent 的 store-specific 前置 Pulse 已经通过 `pulseArgs.StoreIDs={target}` 显式收窄，足以修复 Review 1；diagnostics 应读取完整的 authenticated dimension scope，在 reader 过滤后保留所有授权门店/事实，同时只用 `requestedStoreIDs` 校验目标店是否可见。Scenario 可继续由服务内部过滤目标店；
- 测试：构造目标店 + 至少满足 `MinimumPeerCount` 的授权同群，断言 diagnostics tool 的 peer benchmark 与直接 Store 360 service 等价且不是 `insufficient_peers`；另加 region/brand scope 排除的门店不进入 peer。

### [P1] 真实 PG 测试仍硬编码不存在的旧门店代码

- 位置：`core-service/internal/agenttools/tools/retail_operations_postgres_integration_test.go:120-182`；
- Reviewer 在 Max 停止写入后再次独立执行 `-count=2`，两轮都在 line 122 报 `no rows in result set`。当前 fixed generator 的真实门店代码是 `SIM-<dataset digest>-006`，不是 `Store006`；测试查询、Attention 搜索和 diagnostics 断言仍全部硬编码 `Store006`；
- 修复：seed 返回或保存实际 `plan`，使用对应固定异常门店的 `plan.Stores[...] .Code` / anomaly manifest 作为 expected code，并以该 code 查询 store ID、断言 Attention/diagnostics；不要在测试里另造不存在的显示名；
- cleanup 还应使用本次 test run 的唯一 token/pattern，而不是声称全局 `AGENT-LE-%` 是 unique。可在创建 fixture 前单独清一次历史遗留，随后本轮 cleanup 只清本轮两个法人，避免独立测试进程互删；
- 验收：真实 Docker PG `-count=2` 两轮 PASS，结束后本轮 fixture 与历史 `AGENT-LE-*` 残留均为 0。

## Reviewer 实际命令

```text
docker run --rm --network lease_management_system_default ... \
  go test -v ./internal/agenttools/tools \
  -run '^TestRetailOperationsPostgresIsolationNoWrites$' -count=2
```

实际结果：两轮均 `retail_operations_postgres_integration_test.go:122: no rows in result set`；cleanup 已执行，当前只读盘点 `AGENT-LE-*` 法人/门店/facts 均为 0。

只修复以上两项及测试/报告，不改 UI、Tool 合同、schema、旧功能或审计范围。重新提交保持 `IN_REVIEW`，MAX-009 继续 `BLOCKED`。
