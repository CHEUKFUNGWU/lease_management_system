# MAX-003：核心经营 KPI 语义层与 Golden 对数

状态：`ACCEPTED`  
优先级：P0  
资源类别：数据与测试（20% 配额，同时交付用户可见 API）  
依赖：MAX-002 `ACCEPTED`  
Executor：Max  
Reviewer：Codex 主任务

## 1. 用户价值

### 用户能看到什么

- 一组有中文名称、公式、单位、必需字段、空值规则和版本号的核心经营 KPI 定义；
- 按集团汇总、区域、品牌或门店查看所选日期范围的 KPI；
- 每个结果明确显示数据分类、模拟数据集版本、币种、覆盖率、来源、数据截止时间和是否可用于决策。

### 用户能完成什么经营任务

- 判断销售、毛利、客流、转化、客单、坪效、人工、租金/占用成本和单店贡献表现；
- 在集团—区域—品牌—门店之间使用同一公式比较，而不会把日面积重复相加、跨币种求和或把缺失值当成零；
- 用固定模拟数据集复核每个数字，并为经营脉搏和门店 360 提供唯一可信计算层。

## 2. 产品边界

本票只实现确定性 KPI 语义层、聚合查询 API 和 Golden 对数，不实现异常评分、排行榜、趋势卡片、前端首页、门店 360 或 Agent。

不得直接复用现有月度 `CalculateFourWall` 作为日粒度 KPI 结果；可复用通用小函数，但日事实的面积、版本、来源覆盖和空值语义必须独立定义。

本票必须是纯增量实现：不得删除、重命名、隐藏或改变任何既有页面、导航、API、月度经营分析、合同、IFRS 16、月结、报表或 Agent 行为；不得修改现有 UI。

## 3. KPI v1 定义

公式版本常量：`retail-kpi-v1`。所有百分比 API 值使用 0–100 表示并标记 `unit=percent`；金额保留 2 位，数量/面积最多 2 位。

### 3.1 加总指标

| code | 中文名 | 公式 |
|---|---|---|
| `revenue` | 销售额 | `Σ revenue` |
| `gross_profit` | 毛利额 | `Σ gross_profit` |
| `footfall` | 客流 | `Σ footfall` |
| `transactions` | 交易数 | `Σ transactions` |
| `labor_cost` | 人工成本 | `Σ labor_cost` |
| `fixed_rent` | 固定现金租金 | `Σ fixed_rent` |
| `variable_rent` | 变动租金 | `Σ variable_rent` |
| `non_lease_cost` | 非租赁占用成本 | `Σ non_lease_cost` |
| `other_controllable_cost` | 其他可控成本 | `Σ other_controllable_cost` |
| `occupancy_cash_cost` | 经营占用现金成本 | `fixed_rent + variable_rent + non_lease_cost` |
| `store_contribution` | 门店贡献额 | `gross_profit - labor_cost - occupancy_cash_cost - other_controllable_cost` |

`occupancy_cash_cost` 是经营现金口径，不是 IFRS 16 的折旧、利息、ROU 或租赁负债计量；API 定义中必须明确警示二者不可混用。

### 3.2 比率与效率指标

| code | 中文名 | 公式 |
|---|---|---|
| `gross_margin_rate` | 毛利率 | `gross_profit / revenue × 100` |
| `conversion_rate` | 转化率 | `transactions / footfall × 100` |
| `average_transaction_value` | 客单价 | `revenue / transactions` |
| `labor_cost_rate` | 人工成本率 | `labor_cost / revenue × 100` |
| `rent_to_sales_rate` | 租金销售比 | `(fixed_rent + variable_rent) / revenue × 100` |
| `occupancy_cash_cost_rate` | 经营占用成本率 | `occupancy_cash_cost / revenue × 100` |
| `store_contribution_margin` | 门店贡献率 | `store_contribution / revenue × 100` |
| `sales_per_sqm` | 期间坪效 | `revenue / average_daily_area_sqm` |
| `revenue_per_store_day` | 单店日均销售 | `revenue / observed_store_days` |

面积规则：`average_daily_area_sqm = Σ store_day.area_sqm / distinct_business_days`。这代表所选范围内每日有效经营面积的平均值；禁止直接用 `Σ area_sqm` 做分母，也禁止只取任意一行面积。结果必须同时返回 `average_daily_area_sqm`、`observed_store_days` 和 `distinct_business_days` 作为解释字段。

## 4. 数据选择和正确性规则

1. 只读 `retail_store_day_facts`，通过 `stores` 应用 JWT 法人和现有 store/region/brand data scope；
2. 对同 `store_id + business_date + source_system` 只取最高 `version`；
3. 请求必须显式提供 `data_classification=production|simulated`，禁止隐式混合；
4. `simulated` 必须提供 `simulation_dataset_version`；`production` 携带模拟版本应返回 400；
5. 支持可选 `source_system`。同一 store-day 若存在多个未筛选来源，返回明确的数据质量冲突，不得重复加总；
6. 不得跨币种求和。结果按 dimension + currency 分行；总览存在多币种时返回多行并标明 `multi_currency=true`；
7. 日期范围必填、最长 366 天；支持可选多个 `store_id`；
8. `group_by` 只允许 `total|region|brand|store`；store 行返回 store id/code/name、brand、region；
9. 来源信封至少返回 source systems、dataset versions、最高 as_of、事实版本范围、事实行数；
10. 空结果返回稳定空信封，不伪造 0 KPI。

## 5. 缺失、零分母和覆盖率

- revenue 在事实表为必填；其他输入仍可能为 null；
- 每个 KPI 返回结构化对象，至少包含：`value`、`unit`、`status=complete|partial|unavailable`、`formula_version`、`required_fields`、`available_fact_count`、`fact_count`，不可只返回裸数字；
- 任一必需字段覆盖不足时，派生 KPI `value=null`，状态为 `partial` 或 `unavailable`，不得把 null 当 0；
- 分母为 0 时 `value=null`，并返回 `reason=zero_denominator`；
- 加总指标若字段部分缺失，也不得用剩余行之和冒充完整总额；可返回诊断 subtotal，但正式 `value` 必须为 null；
- 行级返回 overall `decision_ready`，只有本票核心 KPI 的必需覆盖完整、无多来源冲突且来源映射/质量不为 invalid 时才为 true；
- 返回 `coverage`：requested dates、observed dates、observed store-days、expected store-days、coverage rate、缺失字段列表。expected store-days 的计算口径必须写入定义并测试；无法可靠确定时不得编造 100%。

## 6. API

新增：

- `GET /api/v1/retail/kpis/definitions`
- `GET /api/v1/retail/kpis/store-days`

权限暂用 `reports:read`，复用 JWT/TenantMiddleware。

查询示例：

```text
/api/v1/retail/kpis/store-days
  ?date_from=2026-01-01
  &date_to=2026-06-30
  &data_classification=simulated
  &simulation_dataset_version=retail-sim-v1-...
  &group_by=store
```

响应顶层至少包含：

- `basis=Working`、`formula_version=retail-kpi-v1`；
- `data_classification`、`simulation_dataset_versions`；
- 请求范围、group_by、as_of、source、coverage；
- `multi_currency`、`total_rows`、`data`；
- 定义 endpoint 的可定位链接或 definition codes。

不得返回 `Official`，不得调用或读取 IFRS 16 measurement、ROU、interest、journal 或 closing 表。

## 7. Golden 与测试

### 7.1 固定 Golden

新增受版本控制的 Golden 文件，例如：

`core-service/internal/services/retailkpi/testdata/retail_kpi_v1_golden.json`

Golden 必须保存明确常数，不能在断言时用被测函数即时生成“expected”。至少包含：

- MAX-002 默认 60 店/181 天组合总体的全部 KPI；
- 对照门店 001 的全期 KPI；
- 六类异常门店各自在 anomaly manifest 窗口内的核心 KPI；
- 一组 0 分母输入和一组缺失字段输入的预期状态。

Golden 报告应能一键输出 expected/actual/delta/pass，并在任一超容差时失败。金额容差不超过 0.01，百分比容差不超过 0.01 个百分点。

### 7.2 必测项

1. 每项公式、舍入和百分比单位；
2. 聚合比率使用“总分子/总分母”，禁止平均门店百分比；
3. 面积不重复累计，验证 `average_daily_area_sqm` 和 `sales_per_sqm`；
4. null、部分覆盖和零分母不变成健康 0；
5. 最高事实版本选择；
6. 多来源冲突被拒绝，显式 source filter 后可计算；
7. 多币种不求和；
8. total/region/brand/store 四种 grouping；
9. production/simulated 参数组合及来源信封；
10. A 法人无法查询 B 的 KPI，门店/区域/品牌 scope 正反测试；
11. Golden 总体、对照店和六个异常窗口全部通过；
12. KPI 查询前后 IFRS 16/Official 表零写入；
13. `go test ./...`、`go vet ./...`、`git diff --check` 通过。

至少提供纯 KPI 单元测试、Handler 测试和真实 PostgreSQL 集成测试。真实库测试应先调用 MAX-002 生成默认数据，再通过 Repository/API 所用查询计算并与 Golden 对数，不能只对内存 Plan 测试。

## 8. 交付报告

创建 `docs/execution/reports/MAX-003.md`，至少记录：

- 用户可见内容和可完成任务；
- KPI 数据字典和公式版本；
- 日面积、覆盖率、空值、零分母、版本、来源和币种决策；
- Golden 文件位置、完整命令、expected/actual/delta；
- 全部变更文件、测试结果、性能数据和已知限制；
- 明确说明 occupancy cash cost 与 IFRS 16 会计计量隔离。
- 增加“原产品功能保留与回归证据”，列明未删除/改名的既有路由与功能，并记录全量回归结果。

## 9. 明确不做

- 不实现趋势比较、异常优先级、排行榜或经营脉搏组合接口；
- 不实现 UI、图表、门店 360、情景分析或 Agent；
- 不接真实 POS/ERP/WFM；
- 不新增持久化 KPI 表，除非有经报告证明的必要性；优先查询时确定性计算；
- 不扩展高级审计，不修改 MAX-001/002 已接受的数据契约；
- 不删除、重命名、隐藏或替换原有功能、页面、导航或 API，不修改既有 UI；
- 不改 IFRS 16、月结、分录、Official 报表或过账路径；
- 不 commit、不 push，不自行进入 MAX-004。

## 10. 执行顺序

1. 阅读本票、MAX-002 Review 2 和现有 `operating/fourwall.go`，明确哪些月粒度逻辑不能照搬；
2. 先实现 KPI definitions、纯函数和 Golden，再实现 Repository 与 API；
3. 先小范围测试，再运行真实 PostgreSQL Golden 对数和全量回归；
4. 更新交付报告和看板为 `IN_REVIEW` 后停止，等待 Reviewer。
