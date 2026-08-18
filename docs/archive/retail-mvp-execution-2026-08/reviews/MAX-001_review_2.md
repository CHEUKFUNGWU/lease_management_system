# MAX-001 Review 2

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`ACCEPTED`  
Reviewer：Codex 主任务  
日期：2026-08-12  
验收边界：产品优先后的 MAX-001 精简门槛

## 用户价值验收

- 用户可通过受保护 API 写入和读取门店日经营事实；
- 响应明确区分 `production`、`simulated`、`mixed`，并返回模拟数据集版本和来源信封；
- 同一业务键或同一 `Idempotency-Key` 可安全重试，不产生重复事实；同 key 不同载荷返回冲突；
- 法人 A/B 均有事实时，双方仍只看到本法人数据；跨法人门店写入和跨法人导入批次均被拒绝；
- 新日事实表与旧月事实、IFRS 16 Official/计量/过账路径保持隔离。

## 独立验收证据

| 门槛 | Reviewer 命令或检查 | 结果 |
|---|---|---|
| 增量迁移安全 | 对现有 `lease` 库连续两次执行 `db/migrations/038_retail_store_day_facts.sql` | 通过；仅出现 `already exists, skipping`，无破坏性变化 |
| 空库初始化安全 | 创建临时数据库，完整执行 `db/init/01_init.sql`，检查两张新表后删除临时库 | 通过；`retail_store_day_facts` 与 `retail_store_day_fact_requests` 均存在 |
| 法人隔离与来源标识 | Docker 内运行 `go test ./internal/repository -run TestRetailStoreDayFactsPostgres -count=1 -v` | 5 个顶层测试全部通过，含 A/B 双向隔离、跨法人写入/批次拒绝、production/simulated 约束 |
| 基础幂等 | 同一真实 PostgreSQL 测试命令 | 通过；业务键不重复、请求 key 重放不改写、冲突载荷被拒绝 |
| 代码和回归 | `go test ./...` | 通过；含现有 IFRS 16 与月事实相关包 |
| 静态检查 | `go vet ./...`、`git diff --check` | 通过 |

## 非阻断项

- 事实与审计的强原子事务、门店/区域/品牌审计查询权限和完整回放按新策略进入 Hardening Backlog；本次实现已覆盖其中一部分，但不再继续扩展；
- currency 当前只校验三位大写，ISO 4217 白名单可在接入真实数据源时作为数据质量增强；
- 负毛利/销售冲正尚未建模，当前数据库拒绝负值；真实连接器设计时需定义更正事件或冲正事实，而非直接静默放宽。

## 放行决定

MAX-001 满足精简门槛，状态改为 `ACCEPTED`。立即解除 MAX-002 依赖，进入固定 seed 模拟数据生成器。
