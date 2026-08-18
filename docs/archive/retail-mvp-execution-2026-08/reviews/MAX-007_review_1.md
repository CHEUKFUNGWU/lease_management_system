# MAX-007 Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已确认通过

- 采用增量方式新增 `/scenario-workbench`、Evaluate API 和 Action Draft API；旧页面、旧 route、旧情景合同、AppLayout、IFRS 16 与 Official 主链路未删除或改名；
- 导航位置符合冻结票：经营脉搏 → 门店 360 → 情景工作台 → 原经营工作台；Store 360 原“刷新”“返回经营脉搏”保留，另加独立“情景分析”；
- 计算服务以一次 `QueryFacts` 读取同一门店事实，在内存生成 30-day run-rate；七类 delta、Baseline/Plan、Golden 数值、贡献桥、金额/百分比 rounding 和 revenue'=0 的 null margin 已有纯服务测试；
- production/simulated 查询合同、TenantMiddleware 权限路由、source conflict、只读 Evaluate 方向正确；
- Executor 如实记录真实 PostgreSQL 为 SKIP 和浏览器未验收，没有把跳过伪装成通过；Reviewer 已通过 Docker 网络实际连接数据库复现测试。

## Spec 轴：阻断问题

### P1：第二份行动草稿会命中旧 scope 去重并返回不存在的“幽灵 ID”

位置：

- `core-service/internal/handlers/retail_scenarios.go:120-148`
- `core-service/internal/repository/operating_facts.go:620-646`
- `db/migrations/034_operating_decision_platform.sql:129-174`

`SaveAction` 固定使用 `rule_code=retail_store_scenario_v1`、`source_table=retail_store_day_facts`、`source_record_id=store_id`、`period=verification_period`。现有表另有 `(legal_entity_id, rule_code, source_table, source_record_id, period)` 唯一键，而通用 `CreateAction` 在该键冲突时执行 UPDATE，只返回时间戳，不返回数据库中真实 ID。

因此同门店/同核验期间修改 Plan 或行动字段并生成新 idempotency key 时，不会新增新草稿；旧行会被部分覆盖，API 却返回一个数据库中不存在的新 UUID。并发双击还存在“先查后写”竞态，同 key 不同 payload 可能被数据库唯一键转成 500。该问题直接破坏用户的行动清单和“修改字段生成新 key”合同。

要求：不新增 migration、不改变旧 action API 语义；为本场景生成能区分 request fingerprint 的稳定 scope dedupe 值（source table/record 仍必须准确指向目标门店），并确保 Create 后返回数据库真实持久行。处理并发同 key：同 payload 只得到同一真实 action；不同 payload 稳定 409。至少新增真实 PG 断言：

1. 同 key/同 payload 串行与并发重放只有一行、同一真实 ID；
2. 同 key/不同 payload 为 409；
3. 修改字段 + 新 key 生成第二行，两条 ID 均可查、字段/evidence 各自正确；
4. 跨法人可复用同 key 且互不冲突；
5. 不得为修复该问题改 schema 或删除旧唯一键。

### P1：页面可把“旧计算”保存到新门店、新窗口或新假设

位置：`web/app/scenario-workbench/page.tsx:89-148`。

`response` 在修改 store/classification/dataset/as-of/window/source、七项 assumptions 或 horizon 后仍保留；`saveAction` 却使用当前 query/horizon 与旧 response 中的 assumptions。用户可能看到 A 店/12 个月结果，却把同一组旧 Plan 作为 B 店/3 个月行动保存。Evaluate 也没有 request sequence/AbortController，较早请求可覆盖较新筛选；store options 请求同样没有 active guard。

要求：为成功计算保存完整 evaluation snapshot key（事实 scope + horizon + assumptions），任一字段变化立即使结果过期并禁止保存；旧请求结果不得覆盖新请求。重试同一 action payload 复用 key，修改任一 scenario/action 字段得到新 key。把 freshness、请求竞态、模式切换清 store、retry key、confirm/save 状态做成可测纯逻辑并补 Vitest；不要重排旧页面或重做 UI。

### P1：服务没有真正执行冻结票的 `current decision_ready=true` 门槛

位置：

- `core-service/internal/services/retailscenario/scenario.go:188-231,303-366`
- `core-service/internal/handlers/retail_scenarios_test.go:27-39`

`buildBase` 只检查本场景七个金额字段、coverage、invalid/mapping 和 revenue，不复用 `retailkpi.AggregateFacts` 的正式 `DecisionReady`。现有 Handler fixture 故意没有 transactions、footfall、area，按 `retail-kpi-v1` 应为非 decision-ready，但 Evaluate 仍返回 200。这与冻结规则“目标门店 current period decision_ready=true”不符，也会让 Store 360 判为不可决策的数据在情景页重新变成可保存结果。

要求：在仍保持一次 Repository `QueryFacts` 的前提下，复用/等价执行 `retail-kpi-v1` current-period decision-ready 和 required KPI complete 语义；非 decision-ready 返回 422，不补 0。补完整、partial、missing auxiliary/core field、invalid/mapping、zero denominator、currency conflict 和 no facts 运行时测试。

### P1：422 合同和失败 evidence 不符合冻结规格

位置：

- `core-service/internal/services/retailscenario/scenario.go:294-300`
- `core-service/internal/handlers/retail_scenarios.go:188-201`

计算后 gross margin/variable rent rate 超出 `[0,100]` 当前返回 `ErrInvalidRequest`，Handler 映射为 400；冻结票明确要求 422。所有 data unavailable 422 只返回 `error/reason`，没有 evidence，用户无法知道门店、窗口、observed/expected、classification/dataset/source 与 drilldown。

要求：参数结构/范围错误保持 400；计算后 resulting rate 越界返回 422 + 稳定 reason；partial/no facts/zero denominator/currency conflict/decision-not-ready 的 422 均返回最小可追溯 evidence（门店、current window、observed/expected、classification/dataset/source、fact version/最高 as-of 能取得则返回、KPI drilldown URL）。补 Handler JSON 断言，不只断 status code。

### P1：真实 PostgreSQL 验收脚本当前确定失败，且关键隔离/幂等场景未覆盖

位置：`core-service/internal/repository/retail_scenario_postgres_integration_test.go:43-147`。

Reviewer 在真实 Docker PostgreSQL 执行：

```text
go test -v ./internal/repository \
  -run '^TestRetailScenarioPostgresGoldenIsolationAndZeroTouch$' \
  -count=1
```

实际失败：

```text
scenario changed IFRS/production boundary
before={LeaseContracts:29 Measurements:10 Journals:17 ProductionFacts:0}
after={LeaseContracts:29 Measurements:10 Journals:17 ProductionFacts:28}
```

28 条 production facts 是测试在 `:137` 自己插入的 fixture，不是 Evaluate/Save 的副作用；快照位置错误导致必败。现有测试只覆盖 store scope，没有 region/brand scope；只验证同 key replay/冲突，没有“新 key + 修改字段新建第二行”、跨法人同 key、并发双击、action evidence/amount/Owner/Due，也没有 B 法人 save 被拒绝。

要求：把 fixture 建设与产品 zero-touch 快照分阶段；financial boundary 至少覆盖 lease contracts/events、measurements、journals、monthly closing/Official 相关计数，production facts 在 fixture 完成后再拍 action/evaluate 前后快照。补上述 scope/idempotency/action 字段断言。真实 Docker PG 连续 `-count=2` 必须通过并在测试后确认 `SCENARIO-*` 法人、门店、facts、datasets、actions、batches 残留均为 0。

### P1：用户可见的行动闭环与证据区尚未完成

位置：`web/app/scenario-workbench/page.tsx:151-160`。

冻结 IA 要求“证据与公式（折叠）”，当前页面只有可信条，没有展示 response evidence、覆盖、版本、required fields、公式或 KPI drilldown；保存成功只显示 status/id，没有 Owner、Due Date、replay 状态和“前往经营工作台查看”链接。store options 加载失败被静默转为空数组，用户无法区分无授权门店与请求失败。

要求：沿用 Store 360 的 Collapse/Alert/Button 组件，在新页面内补最小证据与公式折叠区；成功卡展示真实 action ID/status、Owner/Due、是否 replay，并提供到既有 `/performance` 的链接；options error 展示可重试错误，合法空 options 仍是独立空状态。不得新增新设计系统、执行按钮或改旧 `/performance`。

## Standards 轴：工程收口

### P2：场景页存在两套 `canSave`，票中纯逻辑没有被页面使用

`web/app/scenario-workbench/logic.ts:58-64` 已定义 `canSaveScenario/selectedScenario`，但页面又在 `page.tsx:148` 内重新实现 `canSave`。这是小型重复逻辑，也使现有 4 个前端测试无法验证真实页面保存门槛。

要求：页面消费抽出的纯逻辑；把 evaluation freshness、action view-model、error/empty state 映射一起收口到最小纯函数，不做大重构。

### P2：报告对真实测试覆盖的描述需与最终证据一致

当前报告称 PG 测试已覆盖 zero-touch、scope 和 action replay，但实际 Reviewer 运行会失败，且 region/brand、跨法人 key、并发、新 key 新行均未覆盖。修复后更新报告为最终实际命令/结果/耗时/Query count/残留；在 Reviewer 复跑前仍不得把 Executor 的本地 SKIP 写成真实通过。

## 重新提交标准

- 资源继续按 70% 用户可见功能、20% 数据与测试、10% 最小控制；本轮只修上述经营正确性与验收问题，不做高级审计、审计回放或新 schema；
- 不删除、隐藏、改名或重排旧页面、旧导航、旧 route/API；不改 `/`、`/performance`、旧 store-decision API、IFRS 16、Official、合同/事件/Agent 主链路；
- 全量 `go test ./...`、`go vet ./...`、Web Vitest/type-check/build、`git diff --check` 通过；
- 真实 Docker PostgreSQL MAX-003/004/006/007 相关回归 `-count=2` 通过并零残留；
- 更新 MAX-007 报告 expected/actual，任务票/报告/看板回到 `IN_REVIEW` 后停止等待 Reviewer；不 commit、不 push、不进入 MAX-008。
