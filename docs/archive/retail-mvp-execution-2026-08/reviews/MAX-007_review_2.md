# MAX-007 Review 2

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`（仅验收收尾）  
Reviewer：Codex 主任务  
日期：2026-08-12

## Review 1 主项核对

以下实现已确认关闭：

- scenario action 已改为独立 `CreateScenarioAction` 单语句 `ON CONFLICT DO NOTHING`，fingerprint/idempotency-derived rule code 不再覆盖旧 scope 行，返回数据库真实持久行；旧 `CreateAction` 与旧 action API 未改；
- 新 key + 修改字段可生成第二行；同 key 重放、payload 冲突、跨法人复用 key 均已有真实数据库路径；
- current period 已复用 `retail-kpi-v1` DecisionReady；no facts/partial/missing/invalid/mapping/zero denominator 和 resulting rate 均走 422 reason + evidence；
- 页面已加入 evaluation snapshot/freshness gate、request sequence、options error、证据与公式 Collapse、真实 action 结果及 `/performance` 链接；旧 UI、route、IFRS 16 与 Official 边界保持；
- Reviewer 真实 PostgreSQL 已跑到测试末尾，action/production/simulated/zero-touch 主体断言通过。

## 剩余最小修复

### P1：真实 PostgreSQL `-count=2` 仍失败于零残留检查 SQL

位置：`core-service/internal/repository/retail_scenario_postgres_integration_test.go:43-55`。

Reviewer 通过 Docker 网络执行 MAX-007 用例，实际结果：

```text
scenario simulated+production SQL queries=14 query_rows=0
scenario residual check: ERROR: operator does not exist:
character varying = uuid (SQLSTATE 42883)
```

同一聚合 SQL 的其他子查询把 `$1/$2` 推断成 UUID，但 `retail_store_day_fact_requests.scope_key` 是 varchar，导致 `scope_key IN ($1,$2)` 类型冲突。清理操作本身已执行，Reviewer 独立查询相关法人/action/request 残留均为 0。

要求：仅修正残留查询的显式 text cast/参数类型，连续真实 PG `-count=2` 必须 PASS，结束后零残留。

### P1：并发与 scope 的测试名称强于实际覆盖

位置：`core-service/internal/repository/retail_scenario_postgres_integration_test.go:162-196,242-255`。

- 当前“Concurrent double-clicks”发生在 `scenario-pg-key` 已经存在之后，只测试两个并发 GET/replay，没有覆盖两个首次请求同时经过“查无 → INSERT”的竞态；
- region/brand 只验证匹配 scope 可运行，没有验证不匹配 scope 被拒绝；
- 已验证 A 不能 evaluate B，但没有验证 B 法人不能 save A 门店。

要求：

1. 使用一个从未创建的新 idempotency key，让两个 goroutine 同时首次保存；断言两个 200、同一真实 ID、数据库仅一行，至少一个 replay；
2. region/brand 各补一个不匹配 scope，稳定 `ErrStoreNotFound`；
3. B 路由尝试保存 A store/dataset，返回 404，B 法人不得产生该 action；
4. 不改 schema、不扩展审计。

### P2：过期结果卡把旧 horizon 标成当前 horizon

位置：`web/app/scenario-workbench/page.tsx:188`。

freshness gate 已禁止保存，但结果卡的 horizon 标签使用当前 state `horizon`，用户从 12 改到 3 后，旧的 12 个月累计结果会被标成“3个月”，同时显示“结果已过期”。

要求：结果卡展示 `response.horizon_months`，确保旧结果即使保留作对照也不被错误标注；补一个最小 formatter/view-model 测试或等价断言。

## 重新提交标准

- 只改上述 MAX-007 PG test、场景页/纯逻辑测试和报告/状态；若实现已支持真实首次并发，不要重写 repository；
- 定向 Go、Web、type-check/build、diff check 通过；Reviewer 将再次通过 Docker 网络运行真实 PG `-count=2`；
- 旧页面/AppLayout/route/API/IFRS 16/Official 不变，不新增 migration，不做高级审计；
- 状态回 `IN_REVIEW` 后停止等待，不 commit/push，不进入 MAX-008。
