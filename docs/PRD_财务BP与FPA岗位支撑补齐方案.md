# PRD：补齐财务 BP 与 FP&A 两岗位的日常工作支撑

> 来源：2026-08-17 以财务BP / FP&A / 运营分析师 / 经营分析师四视角对产品的 review（依据《零售业财务与分析岗位调研.pdf》的 JD 拆解、职能矩阵与核心指标表）。本次方案**只覆盖财务 BP 与 FP&A 两个岗位**，运营分析师与经营分析师所需的库存 / 品类 / 会员 / 履约数据不在本方案范围内（见 §6 明确不做的事）。
>
> 本文所有「现状」断言均已在 HEAD 上核实到具体文件、表或路由，见 §1.2 复核表。
> **配套文档**：《CodebaseDesign_财务BP与FPA支撑补齐_模块深化.md》（模块长什么样、接缝在哪、怎么测，N1–N13）。本文写「要什么」，那份写「代码长什么样」。两份需同步维护。
> 本文与《PRD_零售经营分析工作站_BP日常支撑完善.md》(P0–P5) 是接续关系：那份的 P0–P5 已基本落地（导出、跨区域视图、日历期、store-day 导入器、续租决策卡均已存在），本文处理它之后暴露出来的下一层缺口。

---

## 1. Problem Statement

### 1.1 最关键的发现：FP&A 后端已建成约 80%，前端接线为 0

这是本次 review 最反直觉、也是杠杆最高的一条：

`fpna_plan_versions` / `fpna_plan_lines` 这套数据模型的完备度远超我的预期——版本类型（actual / prior_year / budget / forecast / scenario）、情景类型（baseline / upside / downside）、状态机（draft → review → approved → official → retired）、冻结与 official 标记、prior_version_id 版本谱系、assumption_version / exchange_rate_version / metric_definition_version 三条口径版本线、grain 从 group 到 store 的多粒度、actual_flag / forecast_flag——**这是一套认真设计过的 FP&A 版本治理模型**（`db/init/01_init.sql:1637-1644`）。

对应的服务与路由也都在：

| 能力 | 后端 | 前端 |
|---|---|---|
| 计划版本列表 / 创建 | `GET,POST /performance/plan-versions` | ❌ 无 |
| 版本冻结 / 转 official | `POST /performance/plan-versions/:id/freeze` | ❌ 无 |
| 版本对比（预算 v1 vs v2 vs 预测） | `GET /performance/plan-versions/compare` | ❌ 无 |
| 滚动预测（实际替换已过期期间） | `POST /performance/forecast/hybrid`（`fpna/decision.go:109`） | ❌ 无 |
| 预测准确度与偏差(bias) | `GET /performance/forecast-accuracy`（`fpna/decision.go:169`） | ❌ 无 |
| 假设版本登记 | `GET,POST /performance/assumptions` | ❌ 无 |
| 指标定义治理 | `GET /performance/metrics` | ❌ 无 |
| 主数据映射 | `GET /performance/mappings` | ❌ 无 |
| 数据质量队列 | `GET /performance/data-quality` + `:id/status` | ❌ 无 |
| 决策备忘录 | `GET,POST /performance/decision-memos` + `:id/status` | ❌ 无 |
| WBR/MBR/QBR 报告包 | `GET,POST /performance/report-packs` + `download` | ❌ 无 |

核实方式：`web/app/lib/api.ts` 的 `performanceApi` 把上述函数**全部封装好了**（`api.ts:1372-1460`），但在 `web/app/**/*.tsx` 里检索这些函数的调用点，除 `managementBrief` 外**全部为零调用**。`web/app/performance/page.tsx` 只用了 `overview / actions / storePerformance / equipmentPerformance / storeScenario / bulkUpdateActions / exportActions` 七个。

**结论：FP&A 这个岗位今天用不了产品，主要不是因为能力没建，而是因为没有入口。** 这决定了本方案第一批次的性质是「接线」而非「造新」。

### 1.2 现状复核表（BP + FP&A 关心的能力）

| 能力 | 状态 | 证据 |
|---|---|---|
| 单店经济模型 / 四面墙 EBITDA | ✅ 可用 | `four_wall_ebitda`、`break_even_sales`（performance 页） |
| 租销比 / 占用成本（经营口径 ≠ IFRS16 口径） | ✅ 可用，且是差异化 | `retail_kpi.go:82,88,89` |
| 门店诊断 / 守恒桥 / 同群分位 | ✅ 可用 | `/retail/stores/:id/diagnostics`、`store-performance/benchmarks\|cohorts` |
| 零售侧 Actual vs Plan（门店粒度） | ✅ 已接前端 | `retailkpi/plan_compare.go` → 脉搏页 `page.tsx:667`、门店360 `page.tsx:516` |
| 数据入口（store-day / 计划 / TB） | ✅ 已接前端 | `retail-data-import/page.tsx` |
| 导出（CSV / XLSX / PPTX） | ✅ 可用 | `retailexport`、`RetailExportMenu.tsx` |
| 可信度治理（Fact Coverage / Decision Ready） | ✅ 强项 | `CONTEXT.md` + `retail_kpi.go` |
| **FP&A 版本治理全套** | 🔴 后端有，前端无 | 见 §1.1 |
| **SSSG（同店销售增长）** | 🔴 完全没有 | 全库检索无实现；`stores` 表无 `opening_date`（`01_init.sql:51-69`） |
| **人效（按工时）** | 🔴 完全没有 | 只有 `labor_cost` 金额，无 `labor_hours` / `headcount` |
| **经营口径现金流** | 🔴 只有租赁口径 | `cashflow/scenario.go:22` 输入类型是 `Lease`，产出是租金流出 |
| **多币种折算 / 合并视图** | 🔴 只有声明，无实现 | `exchange_rate_version` 只是版本标签；混币种直接 422 拒绝（`fpna_governance.go:234`） |
| **毛利归因到品类** | 🔴 数据模型不支持 | `retail_store_day_facts` 主键是 store×date×currency，无品类维度 |
| **促销活动 ROI 闭环** | 🟡 有测算无主数据 | 有 `store-promotion-roi` 端点，但无「活动」实体，无法按活动归集与复盘 |
| 预算口径分裂 | ⚠️ 设计风险 | `budget_versions/budget_lines` 是**租赁口径**（contract_id / interest_expense / depreciation），`fpna_plan_versions/plan_lines` 是**经营口径**（revenue / gross_profit / four_wall_ebitda）。两套并存且都叫「预算」，UI 上不区分会直接导致 BP 与 FP&A 读错数 |

### 1.3 两个岗位各自还差什么

**财务 BP**（当前支撑度约 60%）——功能骨架对齐，缺的是「钱以外的业务钩子」：
- 毛利率异常只能归因到门店，归因不到品类/商品，BP 拿着结论进不了业务例会
- 活动 ROI 有算法无主数据，做不了「这次促销花了多少、拉了多少增量、值不值」的闭环复盘
- 人效只有成本没有工时，门店产能与排班话题接不上
- 缺 SSSG，无法区分「增长来自新开店」还是「来自内生」

**FP&A**（当前支撑度约 45%）——治理模型优秀，缺的是入口、闭环与集团视角：
- 全套版本治理没有 UI（§1.1）
- 滚动预测是一个**函数**不是一个**流程**：`HybridForecast` 算完的结果没有落库成新的预测版本，无法编制→评审→冻结→复盘
- 现金流只有租赁口径，年度资金计划做不出来
- 无折算/合并，向区域总部报数要回 Excel
- 缺 CAPEX 计划维度（`fpna_plan_lines` 无 capex 字段），新开店投资并不进集团总预算

---

## 2. 方案总览

六个批次，每批独立可交付、独立验收。**编号即建议排期顺序**，理由是杠杆比（价值 ÷ 工程量）递减。

| 批次 | 主题 | 主要受益岗位 | 后端工作量 | 前端工作量 |
|---|---|---|---|---|
| **F0** | FP&A 工作台接线（零新模型） | FP&A | 极小 | 大 |
| **F1** | 滚动预测闭环 + 预测准确度复盘 | FP&A | 中 | 中 |
| **F2** | 门店生命周期、SSSG 与人时 | BP + FP&A | 中 | 小 |
| **F3** | 经营口径现金流 + CAPEX + 合并折算 | FP&A | 大 | 中 |
| **F4** | 品类维度与毛利归因 | BP | 大 | 中 |
| **F5** | 促销活动主数据与 ROI 闭环 | BP | 中 | 中 |
| **F6** | 数据接入分层（推送 API / 文件落地 / 自助数据源） | 全部 | 中 | 中 |
| **F7** | 库存事实与周转、Sales-Inventory-Margin 联动 | BP + 运营 | 中 | 中 |
| **F8** | SKU 级汇总事实（Excel + AI 辅助入库） | BP + 经分 | 中 | 中 |
| **F9** | 竞品价格参考域（客户录入，不自建采集） | BP + 经分 | 小 | 小 |

> F6–F9 系 2026-08-17 与产品负责人讨论后补入。F7/F8/F9 原被划为「运营分析师 / 经营分析师范围、暂不做」，经确认这三项对财务 BP 的毛利归因与库存贬值风险判断同样必要，故纳入本方案；F9 的实现方式由「自建竞品采集」改为「客户录入的外部参考域」，理由见 §3.F9。

**贯穿全部批次的约束**（沿用既有底线，不重复论证）：
1. 跨法人隔离、模拟/正式数据分区、来源追溯、重复导入保护、IFRS 16 正式台账隔离
2. 经营占用口径 ≠ IFRS 16 口径，两条基准并存，任何新指标必须声明自己属于哪一条
3. 不用 0 填补缺失值；覆盖不足显式降级并给出原因（`Decision Ready` 语义）
4. 币种不隐式合并（F3 引入的折算是**显式、带版本、可追溯**的第二视图，不替换 Currency Partition）
5. 前端一律走既有接缝：`useRetailQuery` 取数、`classifyDataState`/`StateBlock` 呈现状态、`tableScrollX`/容器原语（DESIGN.md §8.1）、新枚举登记进 code-lists 契约测试
6. 所有经营结论继续保持 `unvalidated` 口径

---

## 3. 批次详细设计

### F0｜FP&A 工作台接线

**目标**：把已经存在的 FP&A 后端能力全部暴露成一个 FP&A 岗位每天会打开的工作台。**本批次不新增任何数据表，后端改动仅限于补齐分页/筛选参数。**

**新增路由**：`/fpna-workbench`（导航置于「经营脉搏」与「报表中心」之间）

**页面结构**（四个 Tab，用 `EnterpriseTable` + `BentoGrid` 承载）：

1. **版本管理**
   - 版本列表：名称、类型、情景、期间范围、状态、is_official、创建人、冻结时间
   - 版本谱系树：按 `prior_version_id` 渲染，让人看清 Budget v1 → v2 → Forecast Q3 的血缘
   - 动作：创建版本、冻结、转 official（转 official 需二次确认，且展示「此操作不可逆，将成为对外口径」）
   - 三条口径版本线（assumption / exchange_rate / metric_definition）在版本详情里明示
   - 接线：`performanceApi.planVersions / createPlanVersion / freezePlanVersion`

2. **版本对比**
   - 左右版本选择器 + 期间 + 粒度（group/segment/brand/region/store）+ 币种
   - 差异表：科目 × 期间，展示 left / right / 变动额 / 变动率，超出阈值高亮
   - 混币种时**不静默失败**：把后端返回的 `mixed currencies require exchange_rate_version` 渲染成可操作的引导（提示去选一个汇率版本，F3 之前该选项为空则说明尚不支持）
   - 覆盖度（`result.Coverage`）必须与数字同屏展示
   - 接线：`performanceApi.comparePlanVersions`

3. **数据质量队列**
   - `fpna_data_quality_items` 的工作队列：按 severity / status / period 筛选，支持置为 acknowledged / resolved / accepted
   - 每条带 evidence（source_table + source_record_id）可跳转
   - 接线：`performanceApi.dataQuality` + `:id/status`

4. **治理登记簿**（只读为主）
   - 假设版本、指标定义、主数据映射三张表并列
   - 指标定义要展示 formula / grain / currency_policy / fiscal_period_rule / owner —— 这是 FP&A 对「口径由谁定」的核心诉求
   - 接线：`performanceApi.assumptions / metrics / mappings`

**同时处理预算口径分裂**（§1.2 最后一行）：
- `reports` 页现有的 BudgetVariancePanel 标题改为明确的「租赁预算差异（IFRS 16 口径）」
- FP&A 工作台内一律标注「经营口径」
- 两处互相加一句交叉说明，点击可跳转对方

**后端改动**（小）：
- `GET /performance/plan-versions` 补 `status`、`as_of_period`、分页参数
- `GET /performance/data-quality` 补分页

**验收标准**：
- [ ] FP&A 可以在不碰 API 的前提下完成：创建预算版本 → 查看谱系 → 与上一版对比 → 冻结 → 转 official
- [ ] 版本对比在混币种时给出可操作引导而非报错弹窗
- [ ] 所有数字旁必须有覆盖度与 basis（Working/Official/Scenario）标识
- [ ] `performanceApi` 中除 equipment 相关外不再有零调用函数（用一个检索脚本作为回归测试）
- [ ] 新增枚举（version_type / scenario_type / status / grain）进 code-lists 契约测试

---

### F1｜滚动预测闭环与预测准确度复盘

**目标**：把 `HybridForecast` 从一个函数变成一条 FP&A 每月都会走的流程。PDF 里 FP&A 的月度节律是「月初关账 → 撰写损益变动解读 → 主持经营分析会 → 预测窗口期更新预测」，本批次覆盖后两段。

**后端**：
1. `POST /performance/forecast/hybrid` 增加 `persist` 模式：把混合结果**落库为一个新的 forecast 类型 plan version**，`prior_version_id` 指向被替换的上一版预测，`actual_cutoff_period` 记录实际截止期。当前实现只返回计算结果（`fpna/decision.go:109`），不落库，所以无法形成版本谱系。
2. 新增 `GET /performance/forecast-accuracy/trend`：多期间的准确度与 bias 序列（现有端点只算单次比较），用于识别系统性高估/低估。
3. 预测编制的锁：一个 period 只允许一个 draft forecast 版本，避免多人并行编制打架。

**前端**（在 F0 的工作台内新增第五个 Tab「滚动预测」）：
- 编制向导三步：选基线预测版本 → 选实际版本与截止期 → 预览混合结果差异 → 保存为新版本
- 预览必须展示「哪些期间被实际替换了、哪些保留预测」，逐期标注
- 准确度看板：按 driver / dimension 分组的 forecast vs actual vs variance vs bias；bias 连续同向三期以上要显式标记为「系统性偏差」
- 复盘表可直接导出（复用现有 XLSX/PPTX 导出通道），这是月度经营分析会的材料

**验收标准**：
- [ ] 一次滚动预测编制全程在 UI 内完成，产出一个可冻结的 forecast 版本
- [ ] 版本谱系树上能看到「Budget → FC1 → FC2 → FC3」的链路
- [ ] 同一 period 尝试创建第二个 draft forecast 时被拒绝并说明原因
- [ ] 准确度看板能标出系统性偏差，且能导出

---

### F2｜门店生命周期、SSSG 与人时

**目标**：补两个成本极低但直接影响岗位可用性的数据缺口。

**数据模型改动**：

1. `stores` 表新增：
   - `opening_date DATE`
   - `closing_date DATE`（可空）
   - `store_format VARCHAR(50)`（业态：旗舰店/标准店/快闪/前置仓…，PDF 里经营分析师的 Store Formats 概念，BP 侧也需要用来做同业态对比）
   - `lifecycle_status` 派生（未开业/爬坡期/成熟期/已关闭），由 opening_date 与配置的爬坡月数推导，**不落库**
2. `retail_store_day_facts` 新增可空字段：
   - `labor_hours DECIMAL(18,2)`
   - `headcount DECIMAL(18,2)`

**新指标**（进 `retailkpi` 定义表，`retail_kpi.go:73` 起）：
| Code | 公式 | 说明 |
|---|---|---|
| `sssg` | (可比店本期销售 − 可比店上年同期销售) / 可比店上年同期销售 × 100 | 可比店定义：`opening_date` 早于对比期起点满 12 个月，且期间内无 `closing_date`；不满足的店进入显式排除清单 |
| `sales_per_labor_hour` | revenue / labor_hours | 人效 |
| `labor_hours_per_transaction` | labor_hours / transactions | 门店产能 |

**可比店规则必须是显式对象，不能是 SQL 里的一个 where**：仿照现有 `Peer Cohort` / `Suppressed Attention` 的做法，返回「进入可比口径的门店数 / 被排除的门店数 / 每家被排除的原因」。SSSG 最容易被质疑的就是口径，口径不透明这个指标就是废的。

**Formula version 处理**：新增指标不改变既有指标语义，`retail-kpi-v1` 保持不变，新指标以追加方式登记（与既有 additive-only 约束一致）。

**导入通道**：store-day 受控模板增加 `labor_hours` / `headcount` 两列（可空，向后兼容）；门店主数据需要一个 `opening_date` / `store_format` 的维护入口（`/settings` 或 master-data 页加一个门店编辑抽屉即可，不需要新页面）。

**前端**：
- 经营脉搏、门店 360 增加 SSSG 卡片，旁边固定展示可比店口径（「12 家可比 / 3 家排除 →」可展开看原因）
- 人效指标进现有 KPI 网格
- 门店 360 顶部展示 lifecycle_status 与开业日期（爬坡期门店的低毛利不应被误读为经营问题——这是 BP 每天都会遇到的误判）
- 筛选器增加 `store_format`

**验收标准**：
- [ ] SSSG 在缺少 opening_date 的门店上不静默排除，而是报告为「口径不可判定」
- [ ] 可比店进出清单可导出
- [ ] labor_hours 缺失时人效指标返回 partial + `missing_required_field`，不返回 0
- [ ] 爬坡期门店在关注榜上带生命周期标记

---

### F3｜经营口径现金流、CAPEX 与合并折算

**目标**：让 FP&A 能在系统内做年度资金计划与集团合并汇报，这是当前必须回 Excel 的两件事。

**三个子项，可拆成三张票：**

**F3-a 经营口径现金流**
- 现有 `cashflow/scenario.go` 只吃 `Lease`，产出租金流出。新增一层 `OperatingCashflow`：门店经营现金流（来自 store-day facts 的 revenue − 各项现金成本）+ 租赁流出（复用现有）+ CAPEX（F3-b）+ 手工调整项
- 输出按月、按法人、按币种分区，带覆盖度
- **必须明确这是现金口径不是权责口径**，且与 IFRS 16 的付款计划不重复计算（租金在经营侧已扣的部分要显式抵消，桥要守恒）

**F3-b CAPEX 计划维度**
- `fpna_plan_lines` 新增 `capex DECIMAL(18,2)` 与 `capex_category VARCHAR(50)`（新开店/翻新/设备/IT）
- 新开店投资从 `/pre-deal` 的评估结果可一键生成一条 CAPEX 计划行草稿（草稿仍需人确认后才进版本，遵循 Action Proposal 语义）

**F3-c 合并折算**
- 新增 `exchange_rate_versions` 表（version 名、类型 closing/average/budget rate、生效期间、来源、审批状态），现有 `exchange_rates` 表挂到版本下
- 折算服务：给定一组多币种 plan lines + 一个汇率版本，产出报告币种视图，**同时保留原币种明细**
- `ComparePlanVersions` 当前的 422（`fpna_governance.go:234`）改为：有可用汇率版本时给出折算视图，无版本时保持拒绝
- **硬约束**：折算视图必须始终标注汇率版本与类型；Currency Partition 仍是默认视图，折算是显式切换出来的第二视图，不能成为默认

**前端**：
- `/cashflow-forecast` 页从「租赁现金流」升级为「资金计划」，用 Tab 分「经营 / 租赁 / CAPEX / 合计」，合计页展示守恒桥
- 工作台的版本对比与报告包增加「报告币种」切换器，切换时顶部常驻汇率版本条

**验收标准**：
- [ ] 经营+租赁+CAPEX 的合计现金流与三个分项守恒，残差显式保留
- [ ] 租金在经营侧与租赁侧不双算，有测试覆盖
- [ ] 折算视图任何位置都能看到汇率版本；未选版本时不出现任何跨币种合计数字
- [ ] 原币种视图仍是默认

---

### F4｜品类维度与毛利归因

**目标**：让 BP 的毛利结论能落到业务可执行的颗粒度。这是本方案里最大的一批，也是唯一触及核心事实表的一批。

**数据模型**（关键设计决策，需在动工前定稿）：

推荐**不改** `retail_store_day_facts`，而是新增一张平行事实表：

```
retail_store_day_category_facts
  store_id × business_date × currency × category_code   (唯一键)
  revenue, gross_profit, transactions, units            (可空)
  + 完整的 Source Envelope（source_system / import_batch_id / as_of_at / version
    / data_classification / simulation_dataset_version / data_quality_status）
```

加一张 `retail_categories`（category_code、name、parent_code 支持两级、legal_entity_id、生效期间）。

**理由**：
- 品类明细天然稀疏（不是每家店每天都有全品类数据），塞进主事实表会让「门店日事实缺失」与「品类数据缺失」两种语义混在一起，直接污染现有的 Fact Coverage 判定
- 主事实表的唯一键一旦扩维，所有既有查询、Highest Fact Version 逻辑、Source Conflict 检测都要改，风险面过大
- 平行表可以独立演进覆盖度，且「品类明细不全但门店总数可信」是完全合理的中间状态

**一致性规则（必须实现，否则这张表不可信）**：品类明细的 revenue/gross_profit 汇总与主事实表的门店日数值做对账，产出 `category_reconciliation_status`（tie / within_tolerance / mismatch / no_detail）。**不一致时不自动调平**，报告为 mismatch 并进入数据质量队列。

**新增能力**：
- 毛利守恒桥增加「品类结构效应 vs 品类内毛利率效应」的二级拆解（这是 BP 最想要的那一刀：毛利下滑是因为卖的东西变了，还是因为同样的东西卖便宜了）
- 品类维度进 KPI 查询的 `group_by`
- 门店 360 增加品类明细面板

**前端**：
- 门店 360 新增「品类构成」区块：品类销售占比、毛利贡献、同比变动，带 mismatch 状态提示
- 守恒桥支持下钻到品类
- 导入页增加品类明细模板

**验收标准**：
- [ ] 品类明细缺失时，门店级结论完全不受影响（现有页面无回归）
- [ ] 结构效应 + 毛利率效应 + 残差 = 毛利总变动，守恒有测试
- [ ] mismatch 的品类数据在 UI 上不能被当作可信数据展示
- [ ] 品类主数据支持两级，且支持生效期间（品类重组是零售常态）

---

### F5｜促销活动主数据与 ROI 闭环

**目标**：把 PDF 里 BP 的「促销补贴审查」与「活动 ROI 复盘」两个交付物做成闭环。

**数据模型**：
- `promotions`：活动编码、名称、类型（折扣/满减/买赠/会员日）、起止日期、适用门店范围（全部/区域/品牌/指定清单）、预算金额、负责人、审批状态
- `promotion_costs`：活动 × 期间 × 成本类型（补贴/物料/人力/推广）的实际发生额
- 与 `fpna_action_items` 打通：活动复盘结论可直接生成行动项

**能力**：
- 活动期 vs 基线期的增量测算：基线取活动前 N 周同星期的运行率（复用 `retailscenario` 已有的透明 run-rate 推导逻辑，不新造方法）
- ROI = 增量毛利 / 活动总成本；同时给出增量销售、增量毛利、成本明细
- **必须标注这是关联性不是因果性**：同期若有其他活动重叠、或门店有其他异动，要显式提示（这一条决定了这个功能是「诚实的工具」还是「拍脑袋的背书」）

**前端**：
- 新增 `/promotions` 页：活动列表 + 活动详情（含事前预算审查视图与事后 ROI 复盘视图）
- 门店 360 与经营脉搏的时间轴上叠加活动标记，让异动与活动可视对齐

**验收标准**：
- [ ] 活动期重叠时明确提示归因不可分离
- [ ] 基线口径与门店诊断使用同一套 run-rate 推导（不出现两个「基线」）
- [ ] 复盘结论可一键生成行动项并进入现有闭环

---

### F6｜数据接入分层

**目标**：让真实客户的数据能进来。当前唯一的非模拟入口是人工上传受控模板——这是试点阶段的最大阻塞，但解法不是「为每个客户写连接器」。

**前提认知：最难的一层已经建好了。** `core-service/internal/services/retailingest/retailingest.go` 已经是一条完整的接入接缝：`ParseTemplate` → `SuggestMapping` / `SuggestMappingAssisted`（AI 辅助列映射）→ `ResolveStores`（门店编码与别名解析）→ `Validate` → `EstimateOverlap`（重复导入预估）→ `Commit`（分块 + 幂等键 + 版本化更正）。

**因此本批次的架构原则是一句话：传输是插件，语义只有一处。** 任何新传输方式只负责产出 `headers + rows` 并交给这条接缝，绝不允许绕过它直写事实表——否则 Fact Version、Source Envelope、覆盖度、数据质量留痕会立刻出现第二套实现。

**四种传输，按建议顺序：**

**F6-a 客户推送 API（先做）**
- 新增机器账号与 scoped token。**当前系统完全没有这个机制**（只有面向人的 JWT + `auth_refresh_sessions`），需要新建：token 绑定 legal_entity + 数据域 scope + 有效期 + 可吊销 + 调用审计
- 端点接收受控模板对应的 JSON/NDJSON，带 `Idempotency-Key`，内部直接调用 `retailingest.Commit`
- 配套一份对外的《接入契约文档》：字段定义、营业日切点约定、重述与更正语义、幂等规则、错误码
- **理由**：客户 IT 只需出站调用，不必为我们开入站端口、不必交出 POS 凭据；这类定时任务是他们本来就在做的事。这也是销售周期里最有说服力的材料——客户 IT 能直接评估「接你们要做什么」

**F6-b SFTP / 对象存储落地（再做）**
- 客户按约定路径与命名投放 CSV/XLSX，我们按计划轮询取回，走同一条 ingest 接缝
- **理由**：这是零售企业最通用的集成模式，任何老旧 POS 都具备定时导文件的能力，且同样是纯出站

**F6-c 自助数据源配置器（然后做）**
- 设置区新增「数据源」页。**它不是一个填 URL 的表单**，而是四段配置：
  1. 连接：URL、认证方式（API Key / Bearer / Basic / HMAC 签名）、超时与重试
  2. 抽取：分页方式、增量水位字段、回补窗口
  3. 映射：把响应字段映射到受控模板列 —— **直接复用 `SuggestMapping` 那套映射层**
  4. 试跑：拉一页真实数据，走 preview 展示解析结果与覆盖度预估，人确认后才启用
- 配置保存为带版本的数据源定义，每次同步产出一个 `operating_fact_batches` 批次
- **理由**：只做第 1 段（填 URL）大约只能覆盖两成场景，客户点进去发现填不了，反而是负面体验；四段齐了才是可用产品

**F6-d 本地采集代理（有设计伙伴要求时再做）**
- 客户内网运行的小程序，读本地数据库或文件共享，出站推送至 F6-a 的端点
- **理由**：完全封闭的本地 POS 只有这条路。工程量大，在拿到明确需求前不投入

**明确不做**：为单个客户编写定制连接器；自建爬虫式采集；代客户保管 POS 生产库凭据。

**验收标准**：
- [ ] 四种传输方式的落库结果在 Source Envelope、Fact Version、覆盖度上完全一致，有对比测试
- [ ] 任何传输路径都不能绕过 `retailingest.Commit`（用架构测试约束）
- [ ] 机器 token 可吊销，吊销后调用立即失败，且全部调用进审计日志
- [ ] 同一批数据重复推送不产生重复事实（幂等），重述推送产生新 Fact Version 而非覆盖
- [ ] 《接入契约文档》可独立交付给客户 IT 阅读

---

### F7｜库存事实与 Sales-Inventory-Margin 联动

**目标**：补上 BP 的「库存贬值风险」判断依据，以及零售分析里最标志性的销存毛联动。

**关键技术要点：库存是时点存量，我们现有全部事实都是期间流量。** 这两类的汇总规则完全不同——存量绝不可跨日相加。好消息是这个先例已经存在：`area_sqm` 就是「按不同营业日取平均，从不跨日求和」（`retail_kpi.go:91`）。同一套纪律直接迁移，不要新造机制。

**数据模型**：新增 `retail_store_inventory_facts`
```
store_id × business_date × currency × category_code   (唯一键；category_code 可为汇总层)
  inventory_cost_amount      期末库存成本金额
  inventory_retail_amount    期末库存零售价金额（可空）
  inventory_units            期末库存数量（可空）
  aging_bucket_amounts       库龄分桶金额 JSONB（0-30/31-60/61-90/90+ 天）
  + 完整 Source Envelope（与 store-day fact 同构）
```
频率上不要求每日，周末或月末快照都可接受——覆盖度按「快照日」而非「营业日」计算，这一点必须在 Fact Coverage 里单独建模，不能沿用日事实的覆盖率公式。

**新指标**（进 `retailkpi` 定义表，追加式，不改 `retail-kpi-v1` 既有语义）：
| Code | 公式 | 备注 |
|---|---|---|
| `cogs` | revenue − gross_profit | 从既有字段推导，无需新采集 |
| `average_inventory` | 期初与期末库存的平均（按快照点，非跨日求和） | 存量语义 |
| `inventory_turnover_days` | average_inventory / cogs × 期间天数 | JD 明确指标 |
| `inventory_to_sales_ratio` | average_inventory / revenue | 存销比 |
| `aged_inventory_rate` | 90+ 天库龄金额 / 总库存金额 × 100 | 贬值风险预警，BP 直接用 |

**Sales-Inventory-Margin 联动视图**：在门店 360 与品类面板中，把销售、库存、毛利率三条线放在同一时间轴上，并标出典型组合信号——高库存+低动销+毛利率下行 = 减值风险；低库存+高动销 = 缺货损失。**只呈现信号，不自动下结论**，与现有 Attention Signal 的纪律一致（给出观测值、阈值、方向，不替人判断）。

**验收标准**：
- [ ] 库存金额在任何视图下都不出现跨日累加
- [ ] 快照频率不足时显式降级，不用插值补齐
- [ ] `cogs` 推导在 gross_profit 缺失时返回 partial，不返回 revenue
- [ ] 库龄分桶缺失时，周转天数仍可用，但贬值预警显式标为不可判定

---

### F8｜SKU 级汇总事实（Excel + AI 辅助入库）

**目标**：支撑「哪些是滞销品该退货、哪些是高潜品该补」这类决策。

**先划一条能守住的线：日粒度只到品类，SKU 层只做「门店 × 周 × SKU」的汇总事实。**
- 量级理由：100 店 × 1 万 SKU × 365 天 ≈ 3.6 亿行/年，塞进当前 Postgres 事实表会拖垮查询层；按周汇总砍掉约 30 倍
- 业务理由：SKU 层要回答的是周/月级的退补货决策，日级 SKU 只有补货系统需要，那不是本产品的定位
- 这条线是**产品边界**不是**技术妥协**，应写进文档并对外一致表述

**数据模型**：
- `retail_skus`：sku_code、name、category_code、brand、legal_entity_id、生效期间
- `retail_store_week_sku_facts`：store_id × week_start_date × currency × sku_id（唯一键），含 sales_amount / sales_units / gross_profit / ending_inventory_units / ending_inventory_amount + 完整 Source Envelope

**新指标**：售罄率（sell-through）、动销率、滞销天数、SKU 毛利贡献排名。

**入库方式：Excel/CSV 上传 + AI 辅助，复用既有接缝。** `SuggestMappingAssisted` 已经存在，本批次是把它扩展到 SKU 模板，而非新建一套。AI 在这里承担三件事：

1. **列映射建议**（已有能力，直接复用）
2. **SKU 主数据对齐**——这才是 SKU 导入真正的难点，不是列映射。客户 Excel 里的 SKU 编码往往与我们已有主数据不一致（POS 编码 vs ERP 编码 vs 条码），需要模糊匹配 + 人工确认
3. **品类归属推断**——客户表里常只有 SKU 没有品类，或用的是他们自己的品类名，需要映射到我们的两级品类树

**三条必须遵守的纪律**：
- **AI 只产出「建议 + 置信度 + 证据」，一律经人工确认后才落库**，走既有的 preview → confirm → commit 门（与 `CONTEXT.md` 的 AI Intake「强制人工复核门」语义一致，AI 输出永远不是正式数据）
- **确认后的映射固化成可复用的映射记录，不是每次导入现推。** 扩展 `fpna_master_data_mappings` 的 `mapping_type` 增加 `sku` 与 `category`（该表已支持 external_system / external_id / 生效期间 / 审批状态，正好适用）。这样第二次导入只需处理增量未知项——**AI 成本收敛，结果稳定可复现**，这是本设计最关键的一点
- **AI 绝不推断数值**，只推断映射关系。缺失值保持缺失

**Excel 的现实上限必须提前说清楚**：Excel 单表约 105 万行，一个百店万 SKU 连锁的**一周**全量明细就可能触顶。因此：
- Excel 适用于月度 SKU 汇总、单店或单品类切片、试点期数据
- 超出规模引导至 CSV（无行数限制）或 F6-a 的推送通道
- UI 上要在上传前按文件大小与行数预估给出明确提示，而不是让用户传完再失败

**验收标准**：
- [ ] SKU 映射确认一次后持久化，第二次导入同一数据源不再重复询问
- [ ] AI 置信度低于阈值的映射默认不预选，强制人工选择
- [ ] 品类树变更时历史映射按生效期间保留，不追溯改写
- [ ] 超出 Excel 行数上限时在上传前给出提示与替代路径
- [ ] SKU 事实缺失完全不影响品类级与门店级结论（无回归）

---

### F9｜竞品价格参考域（客户录入）

**目标**：满足竞品价格与促销监控的需求，但**不自建采集**。

**实现方式确定为：客户录入 / 导入的外部参考数据。**

不做爬虫式采集的理由（这是一个刻意的产品立场，需要对外一致表述）：
1. **合规风险**：ToS 违反、反爬对抗、数据合规审查
2. **工程极度脆弱**：目标页面一改即断，维护成本持续且不可预测
3. **最根本的一条：它违背产品的可信度立场。** 全产品其余部分都是「客户自有数据 + 全链路溯源 + 覆盖度显式降级」，一份来源不可靠、随时可能失效的采集数据放进同一个结论链，会污染整套 Fact Coverage / Decision Ready 治理

**数据模型**：`competitor_price_observations` —— **独立域，永不并入 Store-Day Fact，不进 KPI 语义层**
```
competitor_name, our_sku_id (可空), competitor_product_name,
observed_price, currency, observed_at, store_or_channel,
observation_method (门店走访 / 官网 / 小程序 / 第三方采购数据),
observed_by, evidence (照片/截图/文件引用), note
```

**Data Classification 保持 production/simulated 二元不变**，不新增第三个值——竞品数据通过「独立域 + 自有溯源字段」承载，既有不变量与 DB CHECK 约束不受影响。

**能力**：
- 录入入口（单条表单 + 批量 Excel 导入，走同一 ingest 接缝）
- 价格带对比视图：我方售价 vs 竞品观测价，按品类/SKU 并列展示，标注观测时间与方法
- 价格观测的时效衰减提示：超过配置天数的观测标为「陈旧」，不参与当期对比

**能力边界（必须在 UI 上明示）**：竞品观测是**抽样参考**，不是市场全量；不得用于计算市场份额；不进入任何自动化结论或 Attention Signal。

**给大陆客户的补充路径**：若客户需要全网价格覆盖，正确解法是采购第三方数据（星图 / 魔镜 / Nielsen 一类），以「第三方采购数据」作为一种 `observation_method` 导入。这是 BD 决策，不是工程决策。

**验收标准**：
- [ ] 竞品数据在任何查询路径下都无法与 Store-Day Fact 联表进入 KPI 计算（用架构测试约束）
- [ ] 每条观测必须有方法与观测人，缺失则拒绝录入
- [ ] 陈旧观测在对比视图中显式标记且默认排除
- [ ] 视图上固定展示「抽样参考，非市场全量」的口径说明

---

## 4. 建议排期

| 阶段 | 批次 | 说明 |
|---|---|---|
| 第一波 | **F0** + **F2** | 两批互不依赖，可并行。F0 是纯接线，F2 是小字段+新指标，都能在短周期内让两个岗位的可用度明显跳一档 |
| 第二波 | **F1** + **F6-a/b** | F1 依赖 F0 的工作台容器；F6-a/b 与之无依赖可并行，且推送契约文档本身是销售阶段可用的 GTM 资产，越早越好 |
| 第三波 | **F3** | 三个子项可再拆并行，F3-b 依赖 F3-a 的现金流框架 |
| 第四波 | **F4** → **F7** → **F8** | 严格串行：F7 的库存按品类归集依赖 F4 的品类维度；F8 的 SKU 需要挂在 F4 的品类树下。这三批共同构成「毛利与库存归因」主线，是工程量最大的一段 |
| 第五波 | **F5** + **F9** + **F6-c** | 三者互不依赖。F5 在 F4 之后价值更高（能看到活动拉动了哪些品类）；F9 工程量小，可择机插入任意波次 |
| 待触发 | **F6-d** | 有设计伙伴明确提出内网封闭 POS 需求时才启动 |

**依赖关系上必须守住的一条**：F4（品类）是 F7（库存按品类）与 F8（SKU 挂品类树）的共同前置。三者若并行开工，品类树会被三处分别定义，最终一定要返工。宁可串行。

阶段性预期：完成 F0–F3 后，FP&A 支撑度从 45% 到约 80%；再完成 F2 + F4 + F5 + F7 + F8，财务 BP 从 60% 到约 90%——其中 F4 + F7 + F8 这条主线是 BP 从「只能定位到门店」走到「能定位到品类与单品」的关键，也是与通用 BI 拉开差距的地方。F6 不直接提升任一岗位的功能覆盖度，但它决定前面所有能力能否作用在真实数据上，属于前置基础设施。

---

## 5. 风险与需要提前定的事

1. **F4 的事实表设计是不可逆决策**，动工前需要确认「平行表」方案。如果选择扩主表唯一键，工程量和回归风险至少翻倍。
2. **预算口径分裂**（租赁 `budget_versions` vs 经营 `fpna_plan_versions`）在 F0 里只做 UI 澄清。中期需要决定是否合并成一套，或者永久保持两套并明确各自领域。建议先不合并——它们服务不同岗位、不同基准，强行统一会污染两边。
3. **F3-c 的折算一旦开放，Currency Partition 这条硬约束就出现了第一个例外。** 必须把「折算是显式的第二视图」写进 CONTEXT.md，否则会逐步退化成隐式合并。
4. **F2 的 SSSG 口径需要业务确认**：可比店的窗口是 12 个月还是 13 个月、翻新闭店期间如何处理、业态变更是否打断可比性——这三条不同公司做法不同，应做成可配置而非硬编码。
5. **全部结论仍为 `unvalidated`**：无真实 POS/ERP 联调、无客户验证，本方案不改变这一定性。
6. **F8 的 SKU 粒度边界（周级，非日级）是产品边界，不是技术妥协。** 一旦对外表述成「暂时只能做到周」，客户会持续要求日级，最终把我们推向补货系统的战场。统一表述为：SKU 层服务周/月级的退补货与品类结构决策，日级 SKU 属于补货系统职责。
7. **F6 在没有设计伙伴前，只能验证契约与映射层，验证不了与真实系统的对接。** 因此 F6-a/b 的验收以「契约自洽 + 四种传输落库一致」为准，不承诺任何具体 POS 品牌的开箱兼容。
8. **F8 的 AI 辅助存在成本失控风险**，缓解手段是映射固化（确认后复用，不每次现推）。若某客户的映射持续无法收敛，说明其主数据本身混乱，应转为数据治理议题，而不是继续加大 AI 投入。

---

## 6. 明确不做的事

以下能力仍在本方案范围外：

- 会员/客户实体、LTV、CAC、留存
- 履约 SLA、配送延迟、拣货流程、排班优化
- 小时级 / 实时数据粒度（最细为日；SKU 层最细为周）
- 日级 SKU 明细（理由见 §5.6）
- 自建竞品数据采集（F9 已改为客户录入，理由见 §3.F9）
- 为单个客户定制数据连接器（理由见 §3.F6）
- 自助 SQL / 自定义指标 / 自定义看板

其中「自助分析层」是运营分析师岗位的准入门槛，也是四岗位视角下最大的单项缺口，但它是平台级能力，应当独立立项，不应挂在本方案下。

---

## 7. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-08-17 | 初版：F0–F5，覆盖财务 BP 与 FP&A |
| 2026-08-17 | 补入 F6（数据接入分层）、F7（库存）、F8（SKU）、F9（竞品参考域）。F7/F8/F9 原列为「不做」，经确认对 BP 的毛利归因与库存贬值风险判断同样必要，纳入范围；F9 的实现方式由自建采集改为客户录入；排期改为五波，并明确 F4 → F7 → F8 的串行约束 |
