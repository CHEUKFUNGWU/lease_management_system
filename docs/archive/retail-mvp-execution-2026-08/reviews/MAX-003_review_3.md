# MAX-003 Review 3

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

Review 2 的 `ExpectedStoreCount` 已修复；真实 PostgreSQL 当前可以读到 10,860 facts 和 60 expected stores。

## 剩余测试夹具错误

真实测试 `-count=2` 均失败：

```text
highest version not selected count=10860 found=false
```

集成测试通过：

```sql
SELECT id FROM stores WHERE legal_entity_id=$1 ORDER BY code LIMIT 1
```

取得该法人种子数据的 production 门店（`DAY-ST-*`），随后却给它插入 `data_classification='simulated'` 的 version 2 和 other_source 事实。KPI Repository 正确要求 fact 与 store 均为 simulated/同 dataset，因此不会选中这些非法测试事实。最高版本与来源冲突两段断言都没有实际覆盖目标路径。

要求：store fixture 必须显式选择 `data_classification='simulated' AND simulation_dataset_version=$2` 的当前数据集门店；保留 version 2 和多来源断言。真实 PostgreSQL `-count=2` 必须全部通过，结束后 `DAY-LE-kpi-*` 残留为 0。无需改生产代码或其他范围。
