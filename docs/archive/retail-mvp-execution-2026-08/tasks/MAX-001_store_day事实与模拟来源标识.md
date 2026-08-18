# MAX-001：Store-day 日粒度事实与模拟来源标识

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

状态：`IN_REVIEW`  
优先级：P0  
对应总体任务：B-05、D-02、E-01、E-07  
Executor：Max  
Reviewer：Codex 主任务

## 1. 目标

在不破坏现有月度 `store_operating_facts` 的前提下，建立独立的门店日粒度经营事实写入/读取切片，并把 `SIMULATED` 来源作为一等字段贯穿数据库、Repository 和 API。

本任务是后续模拟数据生成器、Golden KPI、经营脉搏和 Retail Agent 的共同前置条件。

### 用户能看到什么

- API 响应明确标识 `production`、`simulated` 或 `mixed`，并显示模拟数据集版本和来源字段；
- 重复导入会返回稳定结果，不会悄悄生成重复事实；
- 只会看到当前法人权限范围内的门店日经营事实。

### 用户能完成什么经营任务

- 安全写入和读取门店日销售、毛利、客流、人工和占用成本等经营事实；
- 在没有真实客户数据时使用模拟事实开展后续经营分析，同时不污染正式数据；
- 对同一导入请求安全重试，并能追溯事实来自哪个系统、记录和导入批次。

## 2. 必须实现

### 2.1 数据库

新增迁移 `038_retail_store_day_facts.sql`，并同步合入 `db/init/01_init.sql`。

新增表建议命名为 `retail_store_day_facts`，至少包含：

- `id`、`store_id`、`business_date`、`currency`；
- `revenue`、`gross_profit`、`transactions`、`footfall`、`area_sqm`、`labor_cost`；
- `fixed_rent`、`variable_rent`、`non_lease_cost`、`other_controllable_cost`；
- `source_system`、`source_record_id`、`import_batch_id`、`as_of_at`、`version`；
- `reconciliation_status`、`mapping_status`、`data_quality_status`；
- `data_classification`，只允许 `production` 或 `simulated`；
- `simulation_dataset_version`：`simulated` 时必须非空，`production` 时必须为空；
- `created_by`、`created_at`、`updated_at`。

数据库约束：

- 唯一性至少覆盖 `store_id + business_date + version + source_system`；
- 金额/数量字段不得为负；如果现有业务允许负销售更正，不要擅自放宽，请在交付报告中登记为后续设计问题；
- 状态字段沿用现有经营事实的允许值；
- 建立按 `store_id + business_date + version desc` 查询的索引；
- 不删除、不改名、不回填现有月表；迁移必须可在已有数据库和空库上安全执行。

### 2.2 Go Repository

在现有 operating facts repository 邻近位置新增明确的 `RetailStoreDayFact` 类型和方法：

- 单条幂等写入或 upsert；
- 按 `legal_entity_id`、日期范围、可选 `store_id` 列表读取；
- 读取必须通过 `stores` 关联实施法人范围过滤；
- 返回门店 code/name/brand/region 及来源、版本、质量、模拟标识；
- SQL 错误必须包装上下文，禁止吞错。

不得让 Agent 或 Handler 直接访问数据库。

### 2.3 HTTP API

新增兼容新产品语义的路由：

- `POST /api/v1/retail/operating-facts/store-days`
- `GET /api/v1/retail/operating-facts/store-days`

要求：

- 复用现有 JWT、TenantMiddleware、权限和最小操作留痕；本票可暂时复用 `master_data:manage` 写权限和 `reports:read` 读权限，权限资源重构另票处理；
- POST 支持批量但必须设置合理上限；超过上限返回明确的 4xx；
- 校验 ISO 日期、日期范围、currency、数据分类和模拟 dataset version；
- 对同一幂等键的重复请求不能产生重复事实；
- GET 要求 `date_from` 和 `date_to`，限制最大范围，支持可选 `store_id`；
- 响应顶层至少包含 `basis=Working`、`as_of`、`data_classification` 或混合状态、`simulation_dataset_versions`、`coverage`、`total` 和 `data`；
- 模拟与生产数据混合时必须显式返回 `mixed`，不能默认为 production；
- 每次写入保留基础操作留痕；事实与审计原子事务、细粒度审计查询权限及完整审计回放不属于本票阻断项；
- 不增加前端页面，不修改现有月度 API 行为。

### 2.4 测试

至少覆盖：

1. `simulated` 缺少 dataset version 被拒绝；
2. `production` 携带 dataset version 被拒绝；
3. 非法日期、反向日期范围、超长日期范围被拒绝；
4. 批次超过上限被拒绝；
5. 重复幂等请求不会重复写入；
6. 法人 A 无法读取法人 B 门店事实；
7. 返回信封正确区分 `production`、`simulated`、`mixed`；
8. 现有月度 operating facts 测试保持通过；
9. 迁移结构/约束至少有 Repository 集成测试或等价数据库验证。

优先复用现有测试工具。若某项因测试基础设施客观缺失无法自动化，必须在交付报告中给出复现命令、证据和补齐任务，不得直接声明通过。

## 3. 明确不做

- 不实现 60 店模拟数据生成器；
- 不实现零售日历、同店 cohort、KPI 聚合或经营脉搏；
- 不修改前端；
- 不重命名旧表、旧 API、旧 `lease.*` Agent 工具或容器；
- 不实现真实 POS/ERP 连接器；
- 不创建 Official/过账路径；
- 不修改 IFRS 16 计量逻辑；
- 不实现事实与审计的强原子事务、细粒度审计查询权限或完整审计回放；
- 不做制造模块改造。

## 4. 验收标准

Reviewer 只有在以下全部满足时才接受：

- 增量迁移与空库初始化 SQL 一致；
- 新表约束能阻止不合法的模拟来源组合和重复事实；
- Repository 所有读取都受法人范围约束；
- API 具有日期、批次、权限、基础幂等、最小操作留痕和明确来源信封；
- 模拟数据在任何响应中不会被误表示为 production/Official；
- 现有月表、API 和 IFRS 16 行为无回归；
- `gofmt`、`go test ./...`、`go vet ./...`、`git diff --check` 通过；
- Max 的交付报告列出全部变更文件、命令原文、结果和遗留风险。

MAX-001 的收尾门槛只包括：代码可编译、迁移安全、法人隔离、模拟数据标识、基础幂等和测试通过。高级审计能力统一登记到 Hardening Backlog，不得继续阻塞本票。

## 5. 交付报告模板

创建 `docs/execution/reports/MAX-001.md`：

```markdown
# MAX-001 交付报告

## 结论
完成 / 部分完成 / 阻塞

## 变更文件
- 文件：用途

## 设计说明
- 数据版本与幂等
- 多租户过滤
- 模拟来源传播
- 向后兼容

## 验收证据
| 验收项 | 命令/测试 | 结果 | 证据摘要 |
|---|---|---|---|

## 未完成与风险
- 不得省略；没有则写“无已知未完成项”

## Reviewer 复现步骤
1. ...
```

## 6. 执行约束

- 先阅读仓库 `AGENTS.md` 和本任务全文；
- 使用 `apply_patch` 编辑；
- 保留用户和 Planner 的未跟踪文件，不修改本任务之外的战略报告；
- 不创建 commit、不 push；
- 如发现任务要求会破坏现有设计，停止相关变更，在报告中提交最小证据与建议，不自行改变产品边界。
