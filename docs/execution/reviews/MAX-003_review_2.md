# MAX-003 Review 2

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## Review 1 修复已确认

- ranked CTE 的 brand/region alias 已修复；真实 PostgreSQL 不再报 SQLSTATE 42703；
- 50% 覆盖率会降级 `decision_ready=false` 并返回 `incomplete_store_day_coverage`；
- 全部指定比率/效率的零分母 Golden 已补齐；
- PostgreSQL cleanup 顺序已修复。本轮失败后查询 `DAY-LE-kpi-*` 残留数为 0。

## 唯一剩余阻断项

真实 PostgreSQL 连续复现当前失败：

```text
A facts=10860 expected stores=0
```

位置：`core-service/internal/repository/retail_kpi.go`。

`QueryFacts` 正确执行 `SELECT COUNT(*)` 写入局部变量 `expected`，但构造 `RetailKPIFactSet` 时没有赋值 `ExpectedStoreCount: expected`。因此 API 顶层 coverage 的 `expected_store_days` 会成为 0/空，完整 60 店数据无法证明 100% 覆盖；缺失数据也可能失去覆盖率闸门。

要求：把已查询的 expected population 写入返回对象，保留当前集成断言；运行真实 PostgreSQL 测试 `-count=2` 并确认两次均通过、结束后 `DAY-LE-kpi-*` 残留为 0。无需修改其他范围。
