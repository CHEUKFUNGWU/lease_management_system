# MAX-003 Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已通过

- `retail-kpi-v1` 为独立增量模块，两个新 GET 路由受现有 `reports:read` 保护；
- 固定 Golden 覆盖默认总体、对照门店和六类异常窗口；主要公式、面积、币种分行、null 与来源冲突设计方向正确；
- 全量 `go test ./...`、`go vet ./...`、`git diff --check` 在 Reviewer 环境通过；
- `web/` 无变更，原页面、导航、API、IFRS 16、月结、报表和 Agent 均保留；
- production/simulated 参数隔离、最高事实版本和 store/region/brand scope 已有测试。

## 阻断问题

以下均属于 KPI 数据正确性或测试可靠性，不是高级治理。

### P1：真实 PostgreSQL KPI 查询无法执行

位置：`core-service/internal/repository/retail_kpi.go` 的 `ranked` CTE。

Reviewer 运行任务票规定的真实 PostgreSQL 测试：

```text
TestRetailKPIPostgresGoldenVersionSourceCurrencyAndTenantBoundaries
query KPI facts: ERROR: column "brand" does not exist (SQLSTATE 42703)
```

CTE 中 `COALESCE(s.brand,'')` 和 `COALESCE(s.region,'')` 没有显式 alias，外层却读取 `brand`、`region`。当前 API 在真实数据库上会 500，不能进入经营脉搏。

要求：给 CTE 派生列稳定 alias，并用真实 PostgreSQL 复跑完整测试。

### P1：50% 覆盖率仍可能标记 `decision_ready=true`

位置：`core-service/internal/services/retailkpi/retail_kpi.go` 的 `aggregateGroup` / `allCoreComplete`。

Reviewer 以 1 个门店、请求 2 天、只有 1 天完整正数事实复现：

```text
coverage=50.00 decision_ready=true
```

当前 `DecisionReady` 只看“已经出现的事实是否字段完整”，没有看整个请求范围是否缺 store-day。这违反任务票“核心 KPI 必需覆盖完整才 decision ready”，会把缺半段数据的门店显示成可决策。

要求：覆盖不足时不得有 `decision_ready=true`；至少采用保守的全局降级，并返回 `incomplete_store_day_coverage`。如实现 group 级 expected coverage，应对 total/region/brand/store 都有明确测试。

### P1：门店贡献率零分母返回错误状态

位置：同一文件对 zero denominator 的 code 判断。

Reviewer 用完整字段但 revenue=0 复现：

```text
store_contribution_margin={value:null status:complete reason:""}
```

任务票要求所有零分母为 `value=null,status=unavailable,reason=zero_denominator`。当前判断遗漏了 `store_contribution_margin`。

要求：用基于定义/分母的统一规则或完整枚举修复；Golden 零分母应覆盖 `gross_margin_rate`、`conversion_rate`、`average_transaction_value`、`labor_cost_rate`、`rent_to_sales_rate`、`occupancy_cash_cost_rate`、`store_contribution_margin` 和 `sales_per_sqm`，避免再次漏项。

### P2：PostgreSQL 测试清理顺序泄漏数据

位置：`core-service/internal/repository/retail_kpi_postgres_integration_test.go` 的 `t.Cleanup`。

测试没有先删除 `retail_simulation_datasets`，随后删除 `operating_fact_batches` 和 `legal_entities` 会被 FK 阻止，但错误被忽略。Reviewer 本次失败后确认遗留 2 个 KPI 测试法人、2 个 dataset 和 2 个 batch，已按精确 UUID 清理。

要求：按 facts → datasets → batches → stores → legal entities 的安全顺序清理，并对关键 cleanup error 进行检查或至少保证不会静默泄漏。连续运行真实测试两次后，数据库不得残留 `DAY-LE-kpi-*`。

## 最小返工范围

- 只修改 KPI Repository、语义层测试、PostgreSQL 测试清理和交付报告；
- 不改 UI、导航、既有路由、IFRS 16、MAX-001/002 数据契约或高级审计；
- 返工后运行 KPI 定向测试、全量 Go 测试、vet、diff check，并由 Reviewer 再跑真实 PostgreSQL；
- 不进入 MAX-004。
