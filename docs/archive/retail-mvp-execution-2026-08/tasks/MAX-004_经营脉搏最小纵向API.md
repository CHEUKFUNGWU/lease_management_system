# MAX-004：经营脉搏最小纵向 API

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

状态：`ACCEPTED`
优先级：P0  
资源类别：用户可见功能（70% 配额）  
依赖：MAX-003 `ACCEPTED`  
Executor：Max  
Reviewer：Codex 主任务

## 1. 用户价值

### 用户能看到什么

- 一个“截至某日，最近 7 天经营怎样”的统一响应：核心指标、上一等长周期对比、每日趋势和优先关注门店；
- 每个关注项明确显示哪个指标偏离、当前值、对比值、变化幅度、阈值、证据日期和数据来源；
- 明确的 production/simulated、数据集版本、覆盖率、公式版本和是否可用于决策。

### 用户能完成什么经营任务

- 在一次经营晨检中回答“整体表现如何、变化来自哪个指标、先看哪几家店”；
- 从集团摘要进入具体门店和证据范围，为 MAX-005 首页与 MAX-006 门店 360 提供同一后端合同；
- 在没有真实客户数据时，使用固定模拟异常稳定复演客流、转化、客单、毛利、人工和占用成本六类经营问题。

## 2. 产品边界

本票交付一条可供首页直接消费的纵向 API 与确定性关注规则，不实现前端、不做根因推断、不生成行动、不调用 LLM。

必须复用 MAX-003 `retail-kpi-v1` 语义层计算数字；不得复制另一套 KPI 公式。新模块只读日事实，不创建 KPI/告警持久表。

本票必须纯增量：不得删除、重命名、隐藏或改变任何既有页面、导航、API、合同、IFRS 16、月结、报表或 Agent 行为，不修改 `web/`。

## 3. API 合同

新增：

```text
GET /api/v1/retail/operating-pulse
```

权限：沿用 `reports:read`、JWT、TenantMiddleware 和 store/region/brand scope。

查询参数：

- `as_of=YYYY-MM-DD`：必填，不使用服务器“今天”作为隐式值；
- `window_days`：默认 7，允许 7–28；
- `data_classification=production|simulated`：必填；
- simulated 必须提供 `simulation_dataset_version`，production 禁止携带；
- 可选 `source_system`、多个 `store_id`；
- `attention_limit` 默认 10，允许 1–50。

期间定义：

- current：`as_of-window_days+1 ... as_of`；
- comparison：current 之前紧邻的等长日期范围；
- 不允许 current/comparison 重叠；响应必须返回两段明确日期。

## 4. 响应结构

顶层至少包含：

- `basis=Working`；
- `pulse_version=retail-pulse-v1`、`formula_version=retail-kpi-v1`；
- 法人权限范围、data classification、dataset version、source systems、最高 as_of；
- current/comparison 日期、各自 coverage、overall `decision_ready`；
- `summary`、`daily_trend`、`attention`、`attention_count`；
- `generated_at` 只作为响应时间，不参与业务 hash；
- definitions/KPI/drilldown 的可定位链接模板。

### 4.1 Summary

至少包含：

- revenue、gross_profit、gross_margin_rate；
- footfall、transactions、conversion_rate、average_transaction_value；
- labor_cost_rate、occupancy_cash_cost_rate；
- store_contribution、store_contribution_margin、sales_per_sqm。

每项返回：current KPI 对象、comparison KPI 对象、`change_value`、`change_type=percent|percentage_point|absolute`、status。

变化规则：

- 金额/数量/客单/坪效：`(current/comparison - 1) × 100`，comparison=0 时 null + `zero_comparison`；
- 比率类：`current - comparison`，单位 percentage point；
- contribution 可同时返回 amount percent change 与 margin pp，但不得混淆单位。

聚合比率必须继续采用总分子/总分母，禁止平均门店百分比。

### 4.2 Daily trend

- current window 每个自然日一行，按日期升序；
- 至少返回 revenue、gross_margin_rate、footfall、conversion_rate、average_transaction_value、labor_cost_rate、occupancy_cash_cost_rate、store_contribution；
- 每日使用 `retail-kpi-v1` 计算；缺日期必须显式 gap，不得用 0 补齐。

### 4.3 Attention list

这是“观察到的信号”，不是根因或因果结论。每项至少包含：

- rank、store id/code/name、brand、region、currency；
- deterministic score、severity、`observed_signals`；
- current/comparison KPI 摘要；
- evidence：两段日期、fact count、source systems、dataset version、formula/pulse version；
- `drilldown` 查询参数/链接模板。

固定规则版本 `retail-pulse-v1`：

| signal_code | current 对 comparison 的触发条件 |
|---|---|
| `revenue_decline` | revenue ≤ -15% |
| `footfall_decline` | footfall ≤ -15% |
| `conversion_drop` | conversion_rate ≤ -3.0 pp |
| `average_ticket_drop` | average_transaction_value ≤ -15% |
| `gross_margin_compression` | gross_margin_rate ≤ -5.0 pp |
| `labor_cost_rate_spike` | labor_cost_rate ≥ +5.0 pp |
| `occupancy_cost_rate_spike` | occupancy_cash_cost_rate ≥ +5.0 pp |
| `contribution_turns_negative` | comparison contribution ≥ 0 且 current contribution < 0 |

每个 signal 返回 observed change、threshold、direction、current、comparison、unit，不允许只返回文案。

评分必须是版本化、确定性纯函数。建议每个信号贡献 `min(abs(change/threshold), 3)`，权重先全部为 1；总分降序，分数相同按 store code 升序。若采用其他简单公式，必须在报告/定义/Golden 中完全公开。

severity 必须由 score/最强信号的固定门槛推导，不能由 LLM 或随机数生成。

## 5. 数据质量闸门

- current 和 comparison 都必须分别计算 expected/observed store-days；
- overall 只有两段 coverage 均完整、来源无冲突、KPI decision ready 才可 `decision_ready=true`；
- 任一门店任一期间覆盖不完整、关键 KPI partial/unavailable 或 data quality invalid 时，不得进入正常 attention 排名；应进入 `suppressed_attention` 或返回明确 suppression reason；
- comparison=0、缺字段、缺日期不得制造无穷大或假告警；
- 多币种不得汇总到同一 summary。MVP 若 scope 含多币种，按币种返回多个 pulse partition；不能任选一个币种；
- 未指定 source 且同一 store-day 多来源时继续返回 409，而非静默选取。

## 6. 实现约束

- 新建独立 `retailpulse` service，接收 MAX-003 KPI 聚合结果或日事实并调用其公开计算入口；
- Repository 查询 current+comparison 可一次完成，但版本、来源、法人和 scope 规则必须复用/委托 MAX-003，不复制易漂移 SQL；
- 一次默认请求不应对 60 店发起 N+1 数据库查询；目标为一次事实查询 + 内存确定性聚合；
- Handler 不自行计算 KPI/score；只校验、编排、构造信封；
- 无写入、无审计级增强、无 migration。

## 7. Golden 与验收

新增版本化 Golden，例如：

`core-service/internal/services/retailpulse/testdata/retail_pulse_v1_golden.json`

至少包含 MAX-002 默认数据集的六个 as_of 场景：

| as_of | 必须被识别的门店/信号 |
|---|---|
| `2026-01-31` | 模拟门店 002 / `footfall_decline` |
| `2026-02-25` | 模拟门店 003 / `conversion_drop` |
| `2026-03-22` | 模拟门店 004 / `average_ticket_drop` |
| `2026-04-16` | 模拟门店 005 / `gross_margin_compression` |
| `2026-05-11` | 模拟门店 006 / `labor_cost_rate_spike` |
| `2026-06-05` | 模拟门店 007 / `occupancy_cost_rate_spike` |

默认 `window_days=7`，comparison 为前 7 天。Golden 至少固定 summary 关键值、attention 排名/score、signal change/threshold 和 coverage；对照门店 001 不得因普通确定性噪声触发上述主要异常。

必测项：

1. current/comparison 日期边界和 7/14/28 天；
2. amount percent change、rate pp、zero comparison；
3. 六类 fixed-seed 异常及控制店；
4. score、severity、排序稳定性；
5. current/comparison/daily trend 全部复用 KPI v1 口径并与 MAX-003 对数；
6. incomplete/partial/invalid 数据被抑制，不产生假 attention；
7. 多币种分 partition；
8. highest version、source conflict、production/simulated、A/B 法人、store/region/brand scope；
9. 默认 60 店请求无 N+1；报告实际查询次数和耗时；
10. 查询前后 IFRS 16/Official 相关表零写入；
11. 原 API/UI 全部保留，`go test ./...`、`go vet ./...`、`npm run type-check`、`npm run build`、`git diff --check` 通过。

至少提供：纯 pulse 单元测试、Handler 测试、真实 PostgreSQL 端到端 Golden 测试。真实测试应先用 MAX-002 生成默认数据，再通过生产 Repository 与 pulse service，不得只测内存 fixture。

## 8. 交付报告

创建 `docs/execution/reports/MAX-004.md`，记录：

- 用户能看到什么、能完成什么任务；
- pulse 版本、规则/评分/severity 公式；
- 六个 Golden 场景 expected/actual/delta；
- coverage suppression、多币种、zero comparison 与来源规则；
- 查询次数、60 店默认请求耗时；
- 法人/模拟/IFRS 16 隔离证据；
- 原产品功能、页面、路由和 UI 架构保留证据；
- 全部测试命令、结果、风险和未完成项。

## 9. 明确不做

- 不实现前端首页、卡片、图表或导航重排（MAX-005）；
- 不实现门店 360（MAX-006）、情景分析、行动草稿或 Agent；
- 不把 signal 描述为根因或因果；
- 不新增告警持久化、通知、自动任务或外部写回；
- 不修改现有 UI、页面、导航、旧 API 或 IFRS 16 路径；
- 不 commit、不 push，不自行进入 MAX-005。

## 10. 执行顺序

1. 先冻结 pulse contract、规则和 Golden；
2. 实现纯 `retailpulse` 与单元测试；
3. 用 MAX-003 查询层做一次 current+comparison 读取与编排；
4. 补 Handler、真实 PostgreSQL、性能与兼容回归；
5. 报告和看板改 `IN_REVIEW` 后停止，等待 Reviewer。
