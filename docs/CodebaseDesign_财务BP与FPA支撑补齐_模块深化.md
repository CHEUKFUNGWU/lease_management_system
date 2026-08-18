# Codebase Design：财务 BP 与 FP&A 支撑补齐 — 模块深化设计

> 配套文档：《PRD_财务BP与FPA岗位支撑补齐方案.md》（要什么）。本文用 codebase-design 词汇（**module / interface / seam / adapter / depth**）描述「模块长什么样、接缝在哪、藏在接口后面的行为、怎么测」。只描述接口与接缝，不写实现细节。
> 前序文档：《CodebaseDesign_零售经营分析工作站_模块深化.md》（M1–M8，已落地）。本文的 N1–N13 与其共存，不重复其接缝。
> 所有现状断言均已在 HEAD 上核实到具体文件与行号。

## 0. 设计原则（判定标准）

- **深度 = 接口的杠杆**：小接口 + 大量藏在里面的行为。**删除测试**：删掉这个模块，复杂度是消失了，还是重新扩散到 N 个调用方？后者说明它在挣自己的饭钱。
- **接缝要真实**：一个适配器 = 假想接缝；两个适配器 = 真实接缝。不引入没有东西跨过它的接缝。
- **接口即测试面**：调用方与测试跨同一接缝。想测穿接口内部，多半是模块形状不对。
- **接受依赖，不创建依赖；返回结果，不产生副作用。**
- **本轮新增一条**：**能用类型让非法状态不可表达的，就不要用运行时校验。** F3 的折算与 F9 的竞品域都靠这一条守住不变量——校验会被绕过，类型不会（见 §5、§13）。

## 1. 现状诊断：这一轮要处理的浅结构

| 位置 | 现状 | 为什么是问题 |
|---|---|---|
| FP&A 能力暴露 | `performanceApi` 封装了 11 个 FP&A 端点（`web/app/lib/api.ts:1372-1460`），除 `managementBrief` 外**全部零调用** | 能力不是缺失，是没有接缝把它们组织成一个可用整体。若直接在页面里逐个调用，版本生命周期规则会散进 5 个 Tab |
| 滚动预测 | `fpna.HybridForecast`（`fpna/decision.go:109`）是纯函数，算完不落库 | 要闭环就会诱使 handler 写「算完顺手插一条」，预测语义（截止期、血缘、单草稿约束）从此散在 handler 里 |
| 缺失字段可用性判定 | `availableFor` 用 `if code == "sales_per_sqm" \|\| code == "average_daily_area_sqm"` 硬编码特判存量语义（`retail_kpi.go:496`） | 已经是一处浅特判。F7 引入库存（存量）后会变成三处、五处，最终一定有人把存量跨日求和 |
| 主数据解析 | `StoreResolution` 是门店专用形状（`retailingest.go:157-163`） | F8 需要解析 SKU 与品类。照着再写两份 = 三套模糊匹配、三套未匹配语义 |
| 事实写入口 | 目前两个写入方（JSON API、模拟器）+ 导入器 | F6 要加四种传输。若各自直写事实表，Fact Version / Source Envelope / 覆盖度会出现第二套实现 |
| 币种 | 混币种直接 422 拒绝（`fpna_governance.go:234`），`exchange_rate_version` 只是一个字符串标签 | 拒绝是当前唯一的保护。一旦 F3-c 开放折算，只剩「记得传版本」这条纪律，必然退化成隐式合并 |

## 2. 接缝总览

| 接缝位置 | 模块 | 适配器 | 测试面 |
|---|---|---|---|
| FP&A 工作台（客户端） | N1 FPnA Workbench Runtime | — | 组件测试跨 runtime 接口 |
| 预测编制（纯 + 端口） | N2 Forecast Composition | PlanVersionWriter：Postgres / 内存（两个） | 单元测试（纯部分）+ 集成（写入） |
| 可比店口径（纯函数） | N3 Comparable Cohort | — | 单元测试：进出清单 |
| 度量种类（语义层） | N4 Measure Kind（retailkpi 深化） | — | 单元测试：存量拒绝求和 |
| 资金计划合成 | N5 Cash Plan Composition | 三个来源读取器（经营 / 租赁 / CAPEX） | 单元测试：守恒 |
| 折算（类型即约束） | N6 Currency Translation | 汇率版本读取器：Postgres / 内存（两个） | 单元测试：无版本无跨币种数字 |
| 明细↔汇总对账 | N7 Detail Reconciliation | — | 单元测试：tie / mismatch |
| 毛利结构分解 | N8 Margin Decomposition（并入既有 Bridge） | — | 单元测试：守恒 + 残差 |
| 活动归因 | N9 Promotion Attribution | RunRateReader：复用 retailscenario（一个生产 + 一个测试桩） | 单元测试：重叠期降级 |
| **数据接入传输** | N10 Source Feed（retailingest 深化） | 上传 / 推送 API / 文件落地 / 自助数据源（**四个**） | 四适配器落库一致性对比测试 |
| 机器凭据 | N11 Machine Credential | —（无端口，见 §11） | 单元测试 + handler |
| 主数据解析 | N12 Master Data Resolution（ResolveStores 泛化） | 建议来源：规则匹配 / AI（两个） | 单元测试 + 集成（映射固化） |
| 竞品参考域 | N13 Competitor Reference | — | 架构测试：不可与事实联表 |

---

## 3. N1 FP&A 工作台运行时 — 客户端（F0）

**问题**：F0 是「接线」批次，最大的风险恰恰是接出一个**浅**结构——一个页面直接调 11 个端点，版本状态机、血缘装配、对比请求组装、混币种降级各写一遍散在 Tab 里。

**采用既有先例**：`CONTEXT.md` 已有 **Contract Workspace** 这个深模块——「视图观察一份快照并发送用户意图；HTTP 调用与表单到载荷的映射留在工作区接缝之后」。N1 照抄这个形状，不发明新模式。

**接口**：

```ts
useFPnAWorkbench(scope: { period: string; legalEntityId?: string })
  → { snapshot: WorkbenchSnapshot; commands: WorkbenchCommands }
```

`WorkbenchSnapshot` 是一份值：版本清单、血缘树、当前对比结果、数据质量队列、治理登记簿、每块各自的加载与降级状态。
`WorkbenchCommands` 是意图：`createVersion / freeze / promoteToOfficial / compare / setDataQualityStatus`。

**藏在接口后面**：
- 版本状态机的合法迁移（draft → review → approved → official → retired），非法迁移在命令层被拒绝，而不是等后端报错
- 由 `prior_version_id` 装配血缘树（后端只返回平表）
- 对比请求的组装与**混币种降级**：把后端的 `mixed currencies require exchange_rate_version` 翻译成可操作引导，而不是弹一个错误框
- 变更后的定向刷新策略（冻结只刷版本块，不整页重取）
- basis（Working / Official / Scenario）与覆盖度始终随数字一起下发，视图无法拿到「没有 basis 的数字」

**删除测试**：删掉它，状态机与血缘装配扩散到 5 个 Tab；「冻结后该刷新什么」会在每个 Tab 各判断一次，且必然不一致。

**接缝**：客户端模块接缝，无适配器。取数仍走既有 `useRetailQuery`，呈现走 `classifyDataState` / StateBlock——N1 组织它们，不替换它们。

**明确不做的事**：不为 N1 引入「后端 BFF 聚合端点」。目前只有一个消费者，那是假想接缝。

---

## 4. N2 预测编制 — Forecast Composition（F1）

**问题**：`HybridForecast` 是纯函数且应当保持纯。闭环需要落库，诱惑是在 handler 里「算完顺手插」。

**设计遵循既有纪律**：`CONTEXT.md` 的 **Scenario Evaluation** 与 **Action Proposal** 已经确立「评估 ≠ 持久化，持久化是单独的显式动作」。N2 沿用同一形状。

**接口**：

```go
func Compose(baseline, actual PlanSet, req ComposeRequest) (*ProposedForecast, error)
// ProposedForecast 是一个值：混合后的计划行 + 逐期替换标记 + 血缘意图 + 覆盖度
func (w *Writer) Commit(ctx, proposed *ProposedForecast, key string) (*PlanVersion, error)
```

**藏在接口后面**：混合规则（实际替换已过期期间，其余保留预测）、`actual_cutoff_period` 语义、血缘意图（指向被替换的上一版预测）、**同一 period 单一 draft forecast 的不变量**、覆盖度与降级。

**为什么拆成 Compose / Commit 两步**：`Compose` 是纯函数——返回结果不产生副作用，测试无需数据库。`Commit` 是唯一有副作用的入口，幂等键在这里。UI 的「预览差异 → 确认保存」两步流程直接对应这两个函数，不需要额外的中间状态。

**接缝**：`PlanVersionWriter` 端口，两个适配器（Postgres / 内存）——**符合两适配器规则**。

**删除测试**：删掉它，混合规则与单草稿不变量落到 handler；第二个调用方（例如 AI agent 生成预测草稿）出现时会复制一份。

---

## 5. N3 可比店口径 — Comparable Cohort（F2）

**问题**：SSSG 最容易被质疑的就是口径。口径若是 SQL 里的一个 `WHERE`，这个指标就是废的。

**采用既有先例**：`Peer Cohort` 与 `Suppressed Attention` 已经确立「排除必须具名、不得静默」。N3 是同一模式的第二次应用。

**接口**：

```go
func Comparable(stores []StoreLifecycle, window PeriodPair, policy ComparabilityPolicy) CohortResult
// CohortResult{ Included []StoreID; Excluded []Exclusion{StoreID, Reason}; Undecidable []Exclusion }
```

**藏在接口后面**：可比窗口月数、翻新/闭店期间处理、业态变更是否打断可比性、`opening_date` 缺失时判为 **Undecidable 而非 Excluded**（这两者语义不同：一个是「不可比」，一个是「不知道可不可比」，混为一谈会系统性高估 SSSG）。

**policy 是数据不是适配器**：三条规则各公司做法不同，做成 `ComparabilityPolicy` 值传入。不为它建端口——没有第二个适配器。

**生命周期状态是纯函数，不落库**：`LifecycleStatus(opening, closing, asOf, rampMonths) → 未开业|爬坡期|成熟期|已关闭`。落库会立刻产生「状态该由谁刷新」的问题。

**SSSG 本身不是新模块**：它是 `retailkpi` 定义表里追加的一条定义，消费 N3 的输出。**能用一条定义解决的，不建模块。**

---

## 6. N4 度量种类 — retailkpi 深化（F7 的前置）

**这是本轮唯一一处「深化既有浅结构」，而非新增。**

**现状**：`availableFor`（`retail_kpi.go:496`）用 `if code == "sales_per_sqm" || code == "average_daily_area_sqm"` 硬编码特判——因为面积是**存量**（按不同营业日取平均，从不跨日求和），而其余指标是**流量**（求和）。这个区分今天以字符串比较的形式存在于一个私有函数里。

**为什么必须先做**：F7 的库存金额、库存数量、库龄分桶全是存量。若沿用特判，这个 `if` 会长到五六个 code，而且**任何新增存量指标的人都必须记得去改它**——这正是「复杂度扩散到 N 个调用方」的定义。

**接口变更**（在既有定义结构上加一个字段，不新增模块）：

```go
type Definition struct {
    Code string
    MeasureKind MeasureKind  // flow | stock   ← 新增
    ...
}
```

**藏在接口后面**：聚合函数依据 `MeasureKind` 选择求和或按快照点平均；**存量度量的跨期求和路径直接不存在**，而不是靠调用方自觉。面积的特判随之删除——它变成 `MeasureKind: stock` 的一个普通实例。

**删除测试**：删掉这个字段，存量/流量的区分回到字符串特判，每加一个存量指标就要改一处私有函数，且漏改不会报错、只会算错。

**GUARD-001 约束**：这是替换类改动——验收必须证明面积指标走的是新的 stock 路径（运行时对照或规则体断言），只证旧特判消失不算。

---

## 7. N5 资金计划合成 — Cash Plan Composition（F3-a/b）

**接口**：

```go
func Compose(req CashPlanRequest, sources CashSources) (*CashPlan, error)
// CashSources{ Operating OperatingReader; Lease LeaseReader; Capex CapexReader }
```

**藏在接口后面**（这里才是真正的深度）：
- **租金不双算**：经营现金流里已扣的租金现金支出，与租赁侧的付款计划是同一笔钱。抵消规则是这个模块存在的首要理由
- 现金口径 ≠ 权责口径的换算
- 按月、按法人、按币种分区
- **守恒检查**：各分项之和与合计的残差显式保留，不平就报告为不平（沿用 Contribution Bridge 的既有纪律，不新造）
- 覆盖度合成：三个来源覆盖度不一致时，合计的覆盖度取最弱者

**接缝**：三个读取器端口。**这里要克制**——`LeaseReader` 的生产适配器就是既有的租赁付款计划服务，不要为它新建一层包装；端口存在的理由是测试能注入桩，让守恒与抵消规则可以在纯内存里测。

**删除测试**：删掉它，「租金抵消」这条规则会出现在 UI 的合计逻辑里——那意味着它会被算错，而且没人测得到。

---

## 8. N6 折算 — Currency Translation（F3-c）

**这是本轮风险最高的一个设计。** 折算一旦开放，`Currency Partition` 这条硬约束就出现了第一个例外。靠「记得传汇率版本」这种纪律守不住。

**设计思路：让非法状态不可表达。**

```go
// 唯一的构造入口：没有汇率版本就拿不到 Basis
func NewBasis(ctx, versionRef string, reader RateVersionReader) (*TranslationBasis, error)

// 跨币种合计只接受已折算集合，而已折算集合只能由 Basis 产出
func (b *TranslationBasis) Translate(set MultiCurrencySet) (TranslatedSet, error)
func Total(set TranslatedSet) Amount
```

**关键点**：`Total` 的入参类型是 `TranslatedSet`，而 `TranslatedSet` **只能**从 `TranslationBasis.Translate` 得到。因此「没有汇率版本却算出了跨币种合计」这个状态在类型上不存在——不需要运行时检查，也无法被绕过。`TranslatedSet` 同时携带汇率版本引用，UI 拿到它就必然能显示版本标识。

**藏在接口后面**：汇率类型选择（closing 用于时点余额、average 用于期间流量——既有 `exchange_rates` 表已有这个区分）、缺率处理、原币种明细的保留、版本生效期间匹配。

**接缝**：`RateVersionReader`，两个适配器（Postgres / 内存）。

**对既有行为的影响**：`ComparePlanVersions` 当前的 422（`fpna_governance.go:234`）从「唯一保护」降级为「无可用汇率版本时的正常降级路径」。保护职责转移到类型系统。

**必须同步更新 CONTEXT.md**：在 `Currency Partition` 词条下补一句——折算是显式的第二视图，由 `TranslationBasis` 承载，原币种分区仍是默认。不写进去，半年后没人记得这是个刻意的例外。

---

## 9. N7 明细↔汇总对账 + N8 毛利结构分解（F4）

### N7 Detail Reconciliation

品类明细表是**平行表**（PRD §3.F4 已定案）。平行表的可信度完全取决于一件事：明细汇总起来对不对得上门店日总数。

**接口**：

```go
func Reconcile(detail []CategoryFact, summary []DailyFact, tolerance Tolerance) ReconciliationResult
// → tie | within_tolerance | mismatch | no_detail（逐店逐日）
```

**藏在接口后面**：容差规则、四种状态的判定、**不一致时不自动调平**（这是整个设计的关键约束：调平会让一个坏数据看起来是好数据）。mismatch 结果流入既有的 `fpna_data_quality_items` 队列。

**接缝纪律（最重要的一条）**：品类事实**不得**通过读门店日事实的同一个读取器返回。两个独立读取器、两个独立类型。理由：任何能同时拿到两者的调用方，早晚会把它们联表求和，然后双算。**接缝在这里的作用是让错误的用法写不出来。**

### N8 Margin Decomposition

「毛利下滑是因为卖的东西变了（结构效应），还是同样的东西卖便宜了（率效应）」——这是 BP 最想要的那一刀。

**不新建桥模块**。既有 Contribution Bridge 已经拥有「分解必须守恒、残差显式保留、不平报告为不平」的全部纪律。N8 是在它内部增加一种分解，共用同一套守恒检查。**两个桥实现 = 两套守恒规则 = 迟早有一套是错的。**

---

## 10. N9 活动归因 — Promotion Attribution（F5）

**接口**：

```go
func Attribute(promo Promotion, actual []DailyFact, baseline RunRate, overlaps []Promotion) AttributionResult
```

**藏在接口后面**：增量销售/增量毛利的推导、ROI 计算、**重叠活动时的归因不可分离降级**（活动期内若有其他活动或门店异动，结果标为不可分离，而不是给一个看起来精确的数字）。

**接缝**：`RunRateReader` 端口，生产适配器**就是既有的 `retailscenario` 的 run-rate 推导**。

**这一条是纯粹的 locality 论证**：如果 N9 自己推一套基线，产品里就有两个「基线」，而它们必然会在某个门店某个期间给出不同的数——那时 BP 会同时看到两个基线并失去对系统的信任。复用既有推导不是为了省代码，是为了让「基线」这个词在产品里只有一个含义。

---

## 11. N10 数据接入 — Source Feed（F6）+ N11 机器凭据

### N10 Source Feed

**这是本轮最重要的接缝，也是唯一一处四适配器的真实接缝。**

`retailingest` 已经是一个深模块：`ParseTemplate` → `SuggestMapping(Assisted)` → `ResolveStores` → `Validate` → `EstimateOverlap` → `Commit`（分块 + 幂等 + 版本化更正）。F6 不改它的语义，只在它前面开一个传输接缝。

**接口**（新增，极小）：

```go
type SourceFeed interface {
    Fetch(ctx context.Context, cursor Cursor) (Batch, error)
}
// Batch{ Headers []string; Rows [][]string; Envelope Envelope; NextCursor Cursor }
```

**四个适配器**：人工上传 / 推送 API / 文件落地（SFTP、对象存储）/ 自助数据源配置。四个都只做一件事：产出 `headers + rows + Envelope`。

**藏在 retailingest 接口后面的东西一个都不变**：Fact Version、Source Envelope、覆盖度、幂等、数据质量留痕。

**用架构测试把原则变成约束**：全仓库除 `retailingest.Commit` 外，不得有第二条写入 `retail_store_day_facts` 的路径。「传输是插件，语义只有一处」若只是文档里的一句话，第三个传输方式落地时一定会被绕过；写成测试它才有牙齿。

**验收的核心**：四个适配器喂同一份数据，落库结果在 Source Envelope、Fact Version、覆盖度上完全一致。这条测试同时就是「传输确实只是插件」的证明。

### N11 Machine Credential

推送 API 需要机器账号——**当前完全不存在**（只有面向人的 JWT + `auth_refresh_sessions`）。

**接口**：`Issue / Verify / Revoke`，三个方法。
**藏在后面**：scope 求值、法人绑定、有效期、吊销即时生效、调用审计。

**明确不为它建端口**：只有一个实现，没有第二个适配器。**一个适配器 = 假想接缝。** 它是一个深模块，不是一条接缝。

---

## 12. N12 主数据解析 — Master Data Resolution（F8）

**现状**：`StoreResolution`（`retailingest.go:157-163`）是门店专用形状。F8 要解析 SKU 与品类。照着再写两份 = 三套模糊匹配 + 三套未匹配语义。

**泛化既有模块**：

```go
func Resolve(ctx, kind EntityKind, raws []string, source SuggestionSource) (Resolution, error)
// EntityKind: store | sku | category
// Resolution{ Resolved map[string]ID; Unknown []string; Ambiguous []Candidate }
```

**藏在接口后面**：先查已确认的持久化映射（扩展 `fpna_master_data_mappings` 的 `mapping_type` 增加 `sku` / `category`——该表已有 external_system / external_id / 生效期间 / 审批状态，正好合用）；**只对剩余的未知项调用建议来源**；置信度低于阈值不预选；确认结果写回映射表。

**接缝**：`SuggestionSource`，两个适配器——规则匹配（确定性）与 AI（`MappingSuggester` 已经是这个形状，见 `retailingest.go:244-271`，且已带 `"ai" | "rule"` 来源标识）。**符合两适配器规则。**

**这个设计最关键的一点**：「AI 成本收敛」不是一条要求人记住的纪律，而是模块的**结构性属性**——已确认的映射先命中，AI 只看未知增量。第二次导入同一数据源时 AI 调用量自然趋近于零，不依赖任何人的自觉。

**边界**：AI 只推断**映射关系**，绝不推断**数值**。确认门沿用既有 AI Intake 的强制人工复核语义，不新造。

---

## 13. N13 竞品参考域 — Competitor Reference（F9）

模块很小，但它承载一条很硬的不变量：**竞品观测永不进入 KPI 语义层**。

**接口**：独立的读写接口，`CompetitorObservation` 类型**不与任何事实类型共享字段结构**。

**用结构而非纪律来保证**：
1. 独立模块、独立读取器，没有任何函数能同时返回观测与 Store-Day Fact
2. 架构测试断言竞品表不出现在任何 KPI 查询路径中
3. `Data Classification` 保持 production/simulated 二元不变——竞品的溯源由自己的字段承载（观测人、观测时间、观测方式、证据）

**为什么不加第三个 classification 值**：那会让既有的 DB CHECK 约束、`CONTEXT.md` 词条、以及全部读路径的二元假设同时松动，换来的只是省一张表。**代价与收益完全不成比例。**

---

## 14. 测试策略

沿用「**替换，不叠加**」：

- 新测试写在深化后模块的接口上；被它取代的浅层测试直接删除，不保留
- 断言可观察产出，不断言内部状态；实现重构时测试不应需要改动
- **N4 是替换类改动**（GUARD-001）：必须证明面积指标走的是新的 `MeasureKind: stock` 路径，只证旧特判消失不算
- **N10 的一致性测试是本轮最有价值的一条**：四个传输适配器喂同一份数据，落库结果必须逐字段一致
- **两条架构测试**（它们是把设计原则变成约束的手段，不是可选项）：
  1. 除 `retailingest.Commit` 外无第二条零售事实写入路径（N10）
  2. 竞品表不出现在任何 KPI 查询路径（N13）

## 15. 依赖与实施顺序

```
N1 ────────────────────────────► F0
N2 ──(依赖 N1 的工作台容器)────► F1
N3 ────────────────────────────► F2
N4 ──(F7 的前置，但可随 F2 提前做)
N10 + N11 ─────────────────────► F6-a/b
N5 ──► N6 ─────────────────────► F3
N7 + N8 ───────────────────────► F4
   └─► N4 已就绪后 ──► F7
   └─► N12 ──────────────────► F8
N9 ────────────────────────────► F5
N13 ───────────────────────────► F9
```

**两条必须守住的顺序**：

1. **N4 必须早于 F7。** 库存是存量度量，若在 `MeasureKind` 就位前落地，`availableFor` 的字符串特判会再长三条，之后再拆的成本高得多。N4 本身很小，**建议随 F2 一起做掉**，不要等到 F7。
2. **N7 的品类树必须早于 F7 与 F8。** 库存按品类归集、SKU 挂品类树，两者共用同一棵树。并行开工会得到三处各自定义的品类树。

**一处克制**：N1 不做后端 BFF 聚合层，N11 不做端口，`LeaseReader` 不为既有服务加包装。三处都只有一个消费者或一个实现——**都是假想接缝**。等第二个出现时再开，那时它才是真的。
