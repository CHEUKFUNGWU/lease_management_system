# MAX-001 Review 1

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12  
固定比较点：`66568fbe924c27c692dd0af15180cf369517e172`

> 2026-08-12 策略更新：本评审中的“事实与审计原子事务”“细粒度审计查询权限”和未来完整审计回放要求已转入 Hardening Backlog，不再阻塞 MAX-001。MAX-001 现只按代码可编译、迁移安全、跨法人隔离、模拟/正式标识、来源追溯、重复导入保护、IFRS 16 正式台账隔离和测试通过验收。跨法人批次引用仍属于租户隔离底线；基础请求幂等仍属于重复导入保护底线。

## 已通过的证据

- 038 增量迁移成功应用到现有开发库，第二次执行安全；
- 使用 `golang:1.23-alpine` 临时容器连接 `lease-postgres`，两项真实 PostgreSQL 集成测试通过；
- `go test ./...` 通过；
- `go vet ./...` 通过；
- 旧月度事实表与 IFRS 16 路径未被修改；
- 模拟/生产来源组合、基本金额约束、业务键唯一性和法人事实读取隔离方向正确。

## Standards 轴

### P1：事实与审计不是原子事务

位置：`core-service/internal/handlers/retail_store_day_facts.go:105-114`

事实先提交，之后才写审计；审计失败时接口返回 500，但事实已经存在。更新场景的审计还固定使用 `old=nil`，无法还原变更。

要求：一次事实 upsert 与对应审计必须在同一 PostgreSQL 事务中提交或回滚；upsert 前读取同业务键旧值，新增使用 `old=nil`，更新使用真实旧值。增加“审计失败不落事实”的自动化测试。

### P1：审计列表缺少门店/区域/品牌范围

位置：`core-service/internal/repository/audit.go:125-179`

新表只加入法人归属解析，没有加入 `appendAuditDimensionScope`。受限用户可能读取同一法人的其他门店事实审计。

要求：把 `retail_store_day_facts` 纳入门店维度审计过滤，并增加 StoreIDs、Regions 或 Brands 范围下的正反测试。

### P1：`import_batch_id` 可跨法人引用

位置：`db/migrations/038_retail_store_day_facts.sql:23`、`core-service/internal/repository/retail_store_day_facts.go:106-112`

当前只有全局 UUID 外键，Repository 未验证批次法人。法人 A 的事实可以引用法人 B 的导入批次。

要求：在数据库约束或同一写事务中强制批次法人等于门店法人；增加真实 PostgreSQL 集成测试，证明跨法人批次引用被拒绝。不得仅依赖 Handler 提供正确 ID。

## Spec 轴

### P1：未实现请求级幂等键

规格要求：“对同一幂等键的重复请求不能产生重复事实”。当前 Handler 不读取 `Idempotency-Key`，只依赖事实业务键 upsert；相同请求键携带不同内容仍会再次执行并更新事实、重复写审计。

要求：采用项目现有 `Idempotency-Key` 约定，持久化批次/请求结果；相同法人 + key 重放不得再次修改事实或重复写审计，应返回 `idempotent_replay=true`。相同 key 配不同 payload 的行为必须确定并测试，推荐返回 409。

### P1：50,000 行被静默截断

位置：`core-service/internal/repository/retail_store_day_facts.go:136`、`core-service/internal/handlers/retail_store_day_facts.go:324-330`

Repository 固定 `LIMIT 50000`，信封仍把返回行声明为完整 `coverage/total/data`。

要求：不得静默截断。实现明确分页及总数，或至少以 50,001 探测并返回 `truncated=true`、部分 coverage 和可靠的 returned/available 语义。测试必须覆盖截断边界。

### P2：测试矩阵缺口

- Handler 缺 production-only 信封测试；
- 法人隔离集成测试没有先给 B 写入事实再验证 A/B 各自只看到本法人；
- 缺请求级幂等重放与 payload 冲突测试；
- 缺审计失败回滚、审计维度范围、跨法人 batch、截断/分页测试。

## Reviewer 独立复现

```bash
docker exec -i lease-postgres psql -v ON_ERROR_STOP=1 -U lease -d lease < db/migrations/038_retail_store_day_facts.sql

docker run --rm --network lease_management_system_default \
  -v /Users/cheukfungwu/ifrs16_management_system:/workspace \
  -w /workspace/core-service \
  -e 'TEST_DATABASE_URL=postgres://lease:lease_secret@lease-postgres:5432/lease?sslmode=disable' \
  -e GOCACHE=/workspace/core-service/.gocache \
  -e GOMODCACHE=/workspace/core-service/.gomodcache \
  golang:1.23-alpine \
  go test ./internal/repository -run TestRetailStoreDayFactsPostgres -count=1 -v
```

结果：两项 PostgreSQL 集成测试通过，测试清理后事实、门店和法人临时记录均为 0。

## 重新提交门槛

- 修复上述 5 项行为问题及测试缺口；
- 更新 `docs/execution/reports/MAX-001.md`，追加 Review 1 修复证据，不覆盖原始证据历史；
- 执行看板改回 `IN_REVIEW`；
- `go test ./...`、`go vet ./...`、真实 PostgreSQL 集成测试和 `git diff --check` 全部通过；
- 不扩张到 MAX-002、前端、KPI、Agent 或 IFRS 16。
