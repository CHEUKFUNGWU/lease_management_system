# MAX-007 Review 3

结论：`CHANGES_REQUESTED`（单一测试断言）  
Reviewer：Codex 主任务  
日期：2026-08-12

Review 2 的四项修改均已落地。Reviewer 使用真实 Docker PostgreSQL 连续运行两次；两次都在同一断言失败：

```text
retail_scenario_postgres_integration_test.go:241:
new key action rows=3, want 2
```

原因：新增的 `scenario-pg-concurrent-first` 首次并发测试已经为同一 entity/store/source/category 创建 1 行；随后 `scenario-pg-key` 和 `scenario-pg-key-v2` 又创建 2 行，因此该 scope 总计应为 3。产品行为正确，旧行没有被覆盖；测试仍沿用新增并发 fixture 前的总数 2。

要求：让断言明确区分 concurrent fixture 与普通 new-key 场景，优先按 idempotency key 精确断言 `scenario-pg-key` 与 `scenario-pg-key-v2` 各一行、两 ID 不同且字段正确；不要只把模糊总数机械改成 3。随后将任务/报告/看板回 `IN_REVIEW`，停止等待。

不得改 product code、repository、schema、旧 UI/API、IFRS 16/Official；不 commit/push，不进入 MAX-008。Reviewer 将再次真实 PG `-count=2`。
