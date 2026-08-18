# MAX-004 Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已通过

- `retail-pulse-v1` 采用独立服务，单次读取 comparison+current 范围并复用 `retail-kpi-v1`，没有复制 KPI 公式；
- current/comparison 日期不重叠，7–28 任意整数窗口、六类固定模拟异常、确定性评分、多币种分区和覆盖抑制的主体方向正确；
- Reviewer 定向测试、全量 `go test ./...`、`go vet ./...`、Web `type-check/build` 和 `git diff --check` 全部通过，原 25 个页面仍生成；
- Reviewer 在真实 PostgreSQL 中连续运行两次集成测试，均通过（单次约 3.2 秒，含两法人各 60 店/10,860 facts 的生成）；测试后法人、门店、dataset 残留均为 0；
- 本票没有修改 `web/`、迁移、旧页面或 IFRS 16/Official 业务路径。

## 阻断问题

以下问题均影响 MAX-005 可直接消费的 API 合同、数据解释或任务票验收证据，不属于高级审计或无关重构。

### P1：下钻链接缺少必填查询条件

位置：`core-service/internal/services/retailpulse/retail_pulse.go` 的顶层 URL 模板及 attention drilldown。

当前 KPI/门店下钻 URL 没有 `data_classification`；模拟数据还没有 `simulation_dataset_version`，显式来源请求也没有 `source_system`。MAX-005 跟随模板请求 KPI API 时会直接得到 400，或丢失与 Pulse 相同的数据选择边界。

要求：下钻模板/结构化参数必须带齐 classification、dataset version、source system、store 和两段日期；生成可直接消费的稳定合同，并补 Handler/Service 测试。

### P1：suppression reason 会把质量问题误报为覆盖不足

位置：`retail_pulse.go` 的 `buildAttention`。

只要 `DecisionReady=false`，当前代码除完全缺期间外统一返回 `incomplete_store_day_coverage`。因此 coverage=100% 但 `data_quality_invalid`、mapping 异常、必需 KPI partial/unavailable 时，用户会看到错误原因。

要求：至少区分 `missing_period_facts`、`incomplete_store_day_coverage`、`data_quality_invalid`、mapping 问题和 `partial_or_unavailable_kpi`；若同时存在多项，可增加稳定的 reasons 数组。每类都要有断言。

### P1：空授权范围的响应不是稳定的 Pulse 信封，并伪造 currency

位置：`retail_pulse.go` 的 `buildPartitions` 与 `Build`。

`ExpectedStoreCount=0` 且无事实时，顶层 current/comparison coverage 的 requested dates 为空、daily trend 为空；这违反“每段明确 coverage、current 每个自然日一行”的合同。另一个 fallback 把字符串 `unobserved` 写入 `currency` 字段，它不是货币代码，MAX-005 会误当币种展示。

要求：零门店和有授权门店但零事实两种情况都返回明确日期、expected/observed、decision_ready=false 和 window 天数的 gap trend；currency 应从可信人口/法人默认币种取得，或显式返回 null/unknown status，不得用非货币值冒充 currency。

### P1：Repository/内部故障被错误返回 HTTP 400

位置：`core-service/internal/handlers/retail_pulse.go:60-66`。

除 source conflict 外，Service 的参数错误和 Repository/数据库错误全部映射为 400。数据库不可用或查询失败会被错误标成用户请求问题，前端也无法正确重试/提示。

要求：用可识别的 validation error（或 Handler 完整前置校验）返回 400；`ErrRetailKPISourceConflict` 返回 409；其余内部/Repository 错误返回 500，并补测试。

### P1：Golden 和真实 PostgreSQL 测试不足以证明任务票的验收声明

位置：`retail_pulse_v1_golden.json`、`retail_pulse_test.go`、`retail_pulse_postgres_integration_test.go`、交付报告。

- committed Golden 没有固定/断言 attention rank、current/comparison coverage，severity thresholds 虽写入 JSON 但未实际断言；
- 真实 PostgreSQL 测试只验证目标 signal 存在，没有与 committed Golden 对数 summary/score/rank/coverage；
- 真实 Pulse 路径未覆盖 production、highest version、source conflict/显式 source、store/region/brand scope；
- 没有记录固定数据库查询次数和“60 店默认 Pulse 请求”的实际耗时，报告对这些证据的表述强于测试实现。

要求：补足上述验收，允许复用 MAX-003 已证明的 Repository helper，但至少要从生产 Repository 贯穿 Pulse Service 断言结果；用 tracer/包装器证明固定查询数而非 N+1，并单独记录生成后的一次 60 店请求耗时。真实测试须可连续运行且零残留。

### P2：每店 evidence 扩大了来源范围

位置：`retail_pulse.go` 构造 `Evidence` 的位置。

每个 attention 当前使用整个请求的 `set.SourceSystems` / `set.DatasetVersions`。当不同门店来自不同系统时，会错误声称该门店使用了其他门店的来源。

要求：evidence 从该门店 current+comparison 的事实去重生成 source systems 和 dataset versions；补多门店不同来源的单元测试。

### P2：负贡献评分例外没有完整公开和测试

位置：`contribution_turns_negative` 评分及 MAX-004 报告。

代码对该信号固定加 1 分、threshold=0，但报告只声明所有信号使用 `min(abs(change/threshold),3)`；除零没有定义，Golden 也未覆盖此信号。

要求：明确公开“负贡献翻转固定加 1”的例外（或采用可计算的版本化规则），加入 Golden/单元测试，并实际断言 Golden 中的 severity thresholds。

## 非阻断维护建议

- `QueryFacts` 的连续字符串位置参数、固定 scope/drilldown 的 map DTO，以及多处 KPI code 清单属于可维护性问题；按当前 70/20/10 策略不阻塞 MAX-004，可登记 Hardening Backlog；
- 删除 `buildAttention` 未使用的 `expected` 参数及 `suppressedForMissingPeriods` 未使用的日期参数；
- 任务票、报告和看板状态必须保持一致。

## 最小返工范围

- 只修改 Pulse service/handler/test、必要的 KPI population 查询和 MAX-004 文档；
- 不修改 UI、AppLayout、导航、旧页面/路由、IFRS 16、MAX-001/002 数据契约或高级审计；
- 修复后把任务票、报告、看板统一改为 `IN_REVIEW`，停止等待 Review 2；
- 不进入 MAX-005，不 commit、不 push。
