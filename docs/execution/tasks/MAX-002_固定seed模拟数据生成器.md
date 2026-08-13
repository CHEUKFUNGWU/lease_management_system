# MAX-002：固定 seed 的模拟数据生成器

状态：`ACCEPTED`  
优先级：P0  
资源类别：数据与测试（20% 配额）  
依赖：MAX-001 `ACCEPTED`  
Executor：Max  
Reviewer：Codex 主任务

## 1. 用户价值

### 用户能看到什么

- 一个受保护的“一键生成模拟经营数据”API；
- 生成结果中的 seed、数据集版本、日期范围、门店数、事实行数、异常清单、来源和 `idempotent_replay`；
- 后续通过 MAX-001 GET API 查询时，所有生成事实均明确显示为 `simulated`，并带相同 `simulation_dataset_version`。

### 用户能完成什么经营任务

- 在没有设计伙伴、深度访谈或真实客户数据的情况下，生成稳定、可重复、可解释的多店零售经营样本；
- 用同一 seed 反复复演经营晨检、门店异常下钻、情景分析和 Agent 问答；
- 安全重试生成请求，不产生重复门店或重复事实，也不污染 IFRS 16 正式台账。

## 2. 目标

实现确定性的模拟数据生成器：相同法人、规范化参数和生成器版本必须产生相同门店、日事实、异常清单和数据集版本。默认生成 60 店、固定日期范围和多类可解释异常，为 MAX-003 Golden KPI 及后续 UI 提供唯一基准数据集。

不得使用 LLM 生成数字；全部数值由版本化、可测试的确定性算法生成。

## 3. 必须实现

### 3.1 模拟数据集登记与门店标识

新增迁移 `039_retail_simulation_datasets.sql` 并同步 `db/init/01_init.sql`：

1. 新增 `retail_simulation_datasets`，至少包含：
   - `id`、`legal_entity_id`、`dataset_version`、`generator_version`、`seed`；
   - `date_from`、`date_to`、`store_count`、`fact_count`；
   - `parameters` JSONB、`anomaly_manifest` JSONB、`payload_sha256`；
   - `status`（`generating/completed/failed`）、`created_by`、`created_at`、`completed_at`；
   - 唯一键至少覆盖 `legal_entity_id + dataset_version`，并约束日期、数量、哈希和状态。
2. 为 `stores` 增加一等来源标识：
   - `data_classification`，只允许 `production` / `simulated`，既有行安全回填并默认为 `production`；
   - `simulation_dataset_version`：simulated 必须非空，production 必须为空；
   - 必须保证空库初始化和已有库迁移均安全。
3. 模拟门店编码固定为 `SIM-<dataset短码>-001...060` 或等价的确定性格式；不得覆盖同法人任何 production 门店。
4. 如果修改现有门店 Repository/API 返回模型，必须保持旧调用兼容，同时让新零售路径能读到门店分类和数据集版本。

### 3.2 确定性生成器

在 Go Core Service 内建立独立、可测试的 generator/service 模块，至少支持输入：

```json
{
  "seed": 20260812,
  "date_from": "2026-01-01",
  "date_to": "2026-06-30",
  "store_count": 60
}
```

规则：

- 省略字段时使用以上固定默认值；不得使用“今天”作为隐式日期；
- `store_count` MVP 允许 10–100，日期范围允许 28–366 天；
- `generator_version` 固定为代码常量（例如 `retail-sim-v1`）；
- `dataset_version` 必须由 generator version + 规范化参数稳定推导，不能由当前时间或随机 UUID 决定；
- 相同参数生成的业务数据逐字段一致；时间戳、数据库 UUID 可不同，但不得进入 Golden 业务哈希；
- 所有事实写入 MAX-001 的 `retail_store_day_facts`，固定：`data_classification=simulated`、正确 dataset version、`source_system=retail_simulator`、确定性 `source_record_id`、`version=1`；
- 使用批量事务或等价安全策略，失败不得留下“completed 但事实不完整”的数据集；
- 同参数重放不得增加 store/fact 行；如提供 `Idempotency-Key`，同 key 不同 payload 返回 409；
- 不删除或改写任何 production 数据。

### 3.3 经营数据与异常场景

每店每天至少生成：

- `revenue`、`gross_profit`、`transactions`、`footfall`、`area_sqm`；
- `labor_cost`、`fixed_rent`、`variable_rent`、`non_lease_cost`、`other_controllable_cost`；
- currency、as_of、来源记录和质量状态。

数据必须具有可用于产品演示的经营结构：

- 门店基础差异：品牌、区域、面积、成熟度/规模代理；
- 周内规律：工作日/周末差异；
- 月度/季节趋势：不可只有白噪声；
- 指标恒等关系在容差内成立：`transactions <= footfall`，收入可由交易数与客单价解释，毛利不高于收入；
- 成本率落在合理、明确记录的区间。

固定异常清单至少包含并返回受影响门店、日期、类型和预期方向：

1. 客流连续下降；
2. 转化率骤降；
3. 客单价下滑；
4. 毛利率压缩；
5. 人工成本异常上升；
6. 占用成本负担偏高。

异常必须由算法显式注入并写入 `anomaly_manifest`，不能仅依赖随机波动“可能出现”。至少保留一组无注入异常的对照门店。

### 3.4 API

新增：

- `POST /api/v1/retail/simulations/store-days/generate`

要求：

- 复用 JWT、TenantMiddleware；暂用 `master_data:manage`；
- 法人由认证上下文决定，不允许请求体指定或覆盖；全局 admin 没有明确法人上下文时返回 400，不得跨法人生成；
- 支持可选 `Idempotency-Key`，遵守 MAX-001 的重放/冲突语义；
- 响应至少包含：`dataset_version`、`generator_version`、`seed`、`date_from/to`、`store_count`、`fact_count`、`data_classification=simulated`、`source_system`、`anomaly_manifest`、`idempotent_replay`、`basis=Working`；
- 响应不得使用 `Official`，不得触发任何会计、月结、分录或过账服务；
- 不要求本票增加前端页面。

## 4. 测试与验收标准

Reviewer 只在以下全部通过时接受：

1. **确定性**：同 seed/参数运行两次，规范化业务哈希、dataset version、门店编码、事实值和异常清单完全相同；
2. **差异性**：改变 seed 后，dataset version 与业务哈希均改变；
3. **规模**：默认生成 60 店 × 181 天 = 10,860 条日事实；响应和数据库计数一致；
4. **模拟标识**：生成的 stores 和 facts 全部为 simulated，dataset version 一致；production 数据零改动；
5. **来源追溯**：任取事实可回到 source system、source record、dataset registry、参数和异常清单；
6. **幂等**：同参数/同 key 重放不增加行；同 key 不同 payload 为 409；
7. **法人隔离**：A 生成后 B 不可读取、重放、覆盖或引用 A 的数据集；A/B 各自生成相同参数时得到法人隔离的数据；
8. **经营关系**：自动测试覆盖 `transactions <= footfall`、`gross_profit <= revenue`、非负约束和六类异常的预期方向；
9. **迁移**：039 在已有库重复执行安全；完整 `db/init/01_init.sql` 可在空库执行；
10. **IFRS 16 隔离**：测试前后 `lease_contracts`、计量结果、journal entries/Official 相关表计数不因生成器改变；
11. **回归**：`gofmt`、`go test ./...`、`go vet ./...`、`git diff --check` 通过；
12. **交付报告**：`docs/execution/reports/MAX-002.md` 列出算法版本、公式/区间、异常注入规则、全部变更文件、命令原文、结果和风险。

至少提供：

- 纯 generator 单元测试（固定 Golden hash 或稳定抽样值）；
- Handler 输入/权限/信封/幂等冲突测试；
- 真实 PostgreSQL 集成测试（60 店/10,860 facts、A/B 隔离、重放不增行、production 和 IFRS 16 表不变）；
- 迁移与空库初始化复现命令。

## 5. 明确不做

- 不实现 KPI 聚合、同比/环比、异常评分或排行榜；
- 不实现经营脉搏首页、门店 360、情景 UI 或 Agent；
- 不接入真实 POS/ERP/WFM/客流系统；
- 不生成或修改租赁合同、付款计划、计量、月结、分录、Official 报表；
- 不实现财务审计级回放、强审计原子性或审计权限重构；
- 不创建 commit、不 push，不修改用户已有未跟踪文档。

## 6. 执行顺序

1. 先读 `AGENTS.md`、MAX-001 Review 2 和本任务全文；
2. 先提交算法/数据契约与 Golden 测试，再实现数据库和 API；
3. 小步运行相关测试，最后跑全量测试与真实 PostgreSQL 集成；
4. 更新交付报告和看板为 `IN_REVIEW`；
5. 停止，不得自行进入 MAX-003，等待 Reviewer 放行。
