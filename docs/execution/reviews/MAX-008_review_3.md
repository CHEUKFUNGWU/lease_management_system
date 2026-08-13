# MAX-008 Review 3

结论：`CHANGES_REQUESTED`（单一 PG 断言）  
Reviewer：Codex 主任务  
日期：2026-08-12

Review 2 的实际 generator store code、run-unique cleanup 和 diagnostics 授权同群读取均已修复；定向 Tool/Agent tests PASS，真实 PostgreSQL 已跑到 Golden 业务断言。

当前仅剩 `core-service/internal/agenttools/tools/retail_operations_postgres_integration_test.go:192-194`：occupancy anomaly 门店同时返回 `contribution_turns_negative` 和 `occupancy_cost_rate_spike`，测试错误地断言 `ObservedSignals[0]` 必须是 occupancy。Reviewer 两轮实际结果都为 rank=1、severity=high、score=3.02，且 occupancy signal 存在、observed change=10.08，只是排序中 contribution signal 在前。

修复要求：像既有 Pulse PG Golden helper 一样按 `SignalCode == "occupancy_cost_rate_spike"` 查找信号，再断言 observed change；不得依赖 slice index，不改产品排序或阈值。完成后真实 PG `-count=2` 两轮 PASS、残留 0，更新报告并保持 `IN_REVIEW`；MAX-009 继续 `BLOCKED`。
