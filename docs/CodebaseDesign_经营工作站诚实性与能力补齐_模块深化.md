# Codebase Design：经营工作站诚实性与能力补齐 — 模块深化设计

> 编制：2026-08-22 · 状态：Current
> 前序：[Spec：经营工作站的诚实性止血与能力补齐（R1 批次）](specs/retail-workstation-honesty-and-capability-r1.md)（D-R1~D-R9 在那里，本文不重复，只在需要时引用）
> 同级：[CodebaseDesign_三表模型与单店利润表_模块深化.md](CodebaseDesign_三表模型与单店利润表_模块深化.md)（SM1–SM8）、[CodebaseDesign_零售经营分析工作站_模块深化.md](CodebaseDesign_零售经营分析工作站_模块深化.md)
> 模块编号用 **RH**（Retail Honesty & capability），以免与 SM / N / MAX 撞号；本文新增决策留痕从 **D-R10** 起接续 Spec。

---

## 0. 设计原则（判定标准）

1. **深度看接口的杠杆，不看实现行数。** 一个模块值不值得存在，问「删掉它，复杂度是消失还是散到 N 个调用方去」。
2. **一个适配器 = 假想接缝，两个才是真接缝。** 本轮四个新模块里只有两个建端口，理由逐个写在下面。
3. **接口就是测试面。** 想绕过接口测内部，说明模块形状不对。
4. **缺失是值，不是异常，更不是 0。** 沿用 `finmodel` 的 `*float64` nil 纪律。
5. **本轮零删除。** 表、列、路由、API、导航项一个不动（`AGENTS.md`「转型是增量叠加」）。

---

## 1. 现状诊断：这一轮要处理的浅结构

| 位置 | 现状 | 为什么是问题 |
|---|---|---|
| `/performance` 同群对标表 | `web/app/performance/page.tsx:164-169`。列标题「同群平均坪效达成率」（硬编码中文，未过 i18n）渲染 `row.fact.oee_pct \|\| row.fact.utilization_pct`；门店副标题 `row.fact.plant_code \|\| "核心商圈"`、`row.fact.production_line_code \|\| "标杆同群"` | 这不是命名问题，是**渲染层伪造语义**。设备综合效率被贴上坪效标签，工厂代码空值被替换成商圈名。页面越专业，误导性越强。这类逻辑住在 JSX 的列定义里，没有任何测试能拦住它——因为没有接缝 |
| 指标 → 页面的选取 | `retailstore360/store360.go:203,205` 的 `benchmarkCodes` / `summaryCodes` 是两个包内 `var` 字符串切片；`retailkpi` 已定义 `sales_per_labor_hour`、`labor_hours_per_transaction` 但两张表都没列 | 指标语义层做对了（零分母 → nil、必需字段声明、覆盖率门槛），但「哪些指标露出到哪个页面」是硬编码列表。加一个指标要改 Go 常量 + 中文名映射 + 前端消费，三处，且没有守卫防止漏其中一处 |
| 中文名的两处定义 | `retailkpi/retail_kpi.go:136-137` 与 `retailstore360/store360.go:747-752` 各有一张 `map[string]string` 标签表，内容重叠 | 两个真相源。加指标时漏改一处，前端拿到的是指标码而不是中文名——这正是 F0 批次在 `/financial-model` 修过的同一类缺陷 |
| 促销 | `promotionattribution.Attribute` 完整（基线 run-rate、增量营收/毛利、ROI、`baseline_unavailable`），但只有投后 | 投前测算不存在。若在前端算，投前投后会用两套基线口径，两个数对不上 |
| 新店测算 | `services/predeal` 的 `Draft`/`YearlyImpact`/`EBITDABridge`/`ExitPoint`/`BalanceSheetImpact`/`Briefing` 全是租约商务与会计影响 | 无业务侧输入（客流/进店率/转化率/客单价），无 CAPEX、爬坡、回本期、IRR。这是真缺口 |
| 差异归因 | `margindecomposition` 有品类层的量/结构/价三效应 | 缺门店利润层的因子分解。若新写一套品类拆解会与它分叉 |
| 模板 DSL | `finmodel/template/formula.go` 完整（词法/语法/`lag(x,n)`/比较/条件），`template.go:356` 有 `validateNoCycles`，`POST /api/v1/financial-model/templates` 已开放 | 引擎齐了，**没有只校验不保存的入口**。前端要么自己复刻一套解析器（必然分叉），要么让用户提交后才知道写错了 |
| 滚动预测 | `finmodel` 引擎已按 `def.ActualCutoffPeriod` 区分冻结线左右；`fpna_plan_versions` 有 prior 谱系与 `scenario_type` | 机制齐了，`/fpna-workbench`（149 行）没把它暴露成可操作动作 |

**共同形状**：本轮八处里有五处是「后端已具备能力，接缝缺一段」，只有两处（新店测算、投前保本）是真的要写新实现。这决定了本设计的重心是**接缝放置**，不是新建服务。

---

## 2. 接缝总览

| 接缝位置 | 模块 | 适配器 | 测试面 | 新建? |
|---|---|---|---|---|
| 指标露出选择（纯，Go） | RH1 Metric Surface | —（一份清单，三个消费方） | 单元：清单 ↔ `retailkpi` 定义的一致性 | 新 |
| 展示口径判定（纯，TS） | RH2 Display Basis Guard | —（一个判定函数，多处调用） | 源码级守卫 + 渲染断言 | 新 |
| 促销保本测算（纯，Go） | RH3 Promotion Breakeven | —（与投后同包同基线） | 单元：四条边界 | 新 |
| 新店可行性（纯 + Ports） | RH4 New Store Feasibility | 租赁投影端口：生产薄绑定 / 内存桩（两个） | Golden 逐格 + Gap 路径 + import guard | 新 |
| 利润差异归因（纯，Go） | RH5 Variance Attribution | —（一个调用方，暂不建端口） | 守恒反向测试 + 顺序敏感性 | 新 |
| 模板校验（复用现有编译路径） | RH6 Template Dry-Run | —（同一 `Compile`，只是不落库） | 三类错误的契约测试 | 半新 |
| 滚动滚入（编排，非计算） | RH7 Forecast Roll-Forward | —（复用 `finmodel/persist` 唯一写入口） | 冻结线左右行为 + publish 门 | 新 |
| 用户文案 | RH8 Copy Discipline | —（i18n + 三条 CI 守卫） | 源码级断言 + 类型约束 | 半新 |

---

## 3. RH1 Metric Surface — 指标露出清单

### 问题

`summaryCodes` / `benchmarkCodes` 两个字符串切片 + 两张中文名 map，散在两个包里。加一个指标要改四处，漏一处的表现是前端显示指标码。

### 接口

一个模块，替代四处散落定义：

```go
// package retailkpi

// Surface 声明一个页面区块要露出哪些指标。
// 校验在包初始化时完成：清单里的每个 code 必须在 Definitions 里存在。
type Surface struct {
    Codes []string
}

// Label 返回指标的中文名。唯一的标签真相源。
// 未定义的 code 返回 (code, false) —— 调用方据此渲染「未识别指标」，不静默显示码值。
func Label(code string) (string, bool)

// ValidateSurface 在启动时校验清单闭包。任一 code 不在 Definitions 里即返回错误。
func ValidateSurface(s Surface) error
```

`retailstore360` 改为持有两个 `retailkpi.Surface` 值，删除包内的 `labels` map（该 map 的内容合并进 `retailkpi` 的唯一标签表）。

### 深度理由

删除测试：删掉 RH1，中文名映射要在 `retailstore360`、`retailpulse`、未来每个消费页各维护一份，且「清单里的 code 必须真的有定义」这条不变量无处安放。复杂度会散到 N 处 —— 它在挣它的位置。

接口只有三个符号，背后是「指标码 → 定义 → 中文名 → 零分母语义」的整条链路。

### 本轮的具体改动

`summaryCodes` 追加 `sales_per_labor_hour`、`labor_hours_per_transaction`、`headcount`；`benchmarkCodes` 追加 `sales_per_labor_hour`。中文名沿用 `retailkpi` 已有的「销售人效」「单均工时」，不另起译名。

**不做的事**：不新增任何指标计算。`sales_per_labor_hour = revenue / labor_hours` 已在 `retail_kpi.go:400`，零分母返回 nil 已在。**严禁**从 `labor_cost` 除以一个假定时薪反推 `labor_hours` —— 那会造出一个类型上是事实、语义上是估计的值，绕过整套覆盖率门槛。

### 决策留痕 D-R10

> **指标露出做成受校验的清单，不是自由字符串切片。** 理由：本仓已经因为「两处标签表」在 `/financial-model` 吃过一次亏（F0 批次）。清单闭包校验是 CI 可强制的，"记得四处都改"不是（单人团队，`docs/AI_文档索引与现行决策.md` §6）。

---

## 4. RH2 Display Basis Guard — 展示口径判定

### 问题

`/performance` 的列定义直接把 `oee_pct` 渲染成坪效。这类错误的根因是**没有接缝**：口径判定长在 JSX 的 `render` 闭包里，测试要断言它必须先渲染整页。

### 接口

一个纯判定函数 + 一个渲染契约：

```typescript
// web/app/lib/displayBasis.ts

export type Basis = "retail_store" | "equipment" | "unknown";

/** 一个数值在某个展示语境下是否可用。不可用时给出面向用户的原因。 */
export function resolveBasis(
  metricCode: string,
  sourceBasis: Basis,
  displayContext: Basis,
): { usable: true } | { usable: false; reasonKey: string };
```

规则很短：`sourceBasis !== displayContext` 即不可用，`reasonKey` 指向一条 i18n 文案（「本期无门店口径数据」）。调用方拿到 `usable: false` 一律渲染 `—` + tooltip，**没有第二条分支**。

### 深度理由

接口是一个函数、三个参数。背后要挡住的是「任何一个未来的开发者，在任何一张表里，把任何一个非零售口径的字段贴上零售标题」。删除测试：删掉它，这条规则只能靠 code review —— 而这个仓库没有第二个人类。

### 本轮的具体改动

1. `/performance` 的 `peerColumns` 全部经 `resolveBasis` 判定。当前数据源（`performanceApi.equipmentPerformance`）的 `sourceBasis` 是 `"equipment"`，页面展示语境是 `"retail_store"`，因此整块降级为具名空态：「本期无门店同群对标数据 —— 该数据源当前只包含设备口径事实」。
2. 删除 `plant_code || "核心商圈"`、`production_line_code || "标杆同群"` 两处兜底。空值就是 `—`。
3. 硬编码中文列标题「同群平均坪效达成率」下线，不换标题，是整列不再存在于零售语境。

**后端零改动。** `performanceApi.equipmentPerformance` 契约不变、表不变、权限过滤不变（Spec D-R1）。

### 配套守卫

```
web/app/performance/basis-guard.test.ts
```

- 给一个只含设备事实的 fixture，断言渲染结果**不出现**「坪效」「商圈」「同群」字样；
- 断言出现具名空态文案；
- **自检句**：把 `resolveBasis` 的返回改成恒 `usable: true`，这条测试必须红。

### 决策留痕 D-R11

> **口径冲突的默认动作是降级，不是转换。** 不提供「设备口径 → 零售口径」的换算，因为不存在这样的换算。可用则显示、不可用则显示 `—` 加原因，只有两条路。给第三条路（估算/近似）就会有人走。

---

## 5. RH3 Promotion Breakeven — 促销投前保本测算

### 接口

进 `services/promotionattribution` 同包，与投后 `Attribute` 并列：

```go
type BreakevenRequest struct {
    Currency            string
    BaselineRevenue     float64  // 活动期若无活动的预计销售额
    BaselineMarginRate  float64
    PromoMarginRate     float64  // 折后毛利 / 折后销售额
    FixedMarketingCost  float64
}

type BreakevenResult struct {
    Currency               string
    RequiredIncrementalRevenue *float64 `json:"required_incremental_revenue,omitempty"`
    RequiredUpliftRate         *float64 `json:"required_uplift_rate,omitempty"` // 相对基线的增幅
    MarginSacrifice            float64  `json:"margin_sacrifice"`              // 原有销量的让利损失
    Status                     string   `json:"status"` // achievable | unachievable | invalid_input
    UnachievableReason         string   `json:"unachievable_reason,omitempty"`
}
```

公式在 Spec D-R3，此处不重复。

### 关键约束

- `PromoMarginRate <= 0` → `Status = "unachievable"`，两个指针字段为 nil。让利后边际毛利非正意味着卖得越多亏得越多，**不存在保本点**。返回一个巨大的数字假装有解是本模块最容易犯的错。
- `BaselineRevenue < 0` 或 `PromoMarginRate > 1` → `invalid_input`。
- 基线口径必须与 `Attribute` 的 run-rate 同源。做法：两个函数共用同一个 `RunRate` 类型（已存在，`attribution.go:52`），不新定义一个「投前基线」类型。

### 深度理由（为什么同包而不是新包）

它和 `Attribute` 共享基线定义、货币处理、四舍五入策略。拆包会立刻产生两套基线口径 —— 而 User Story 19 要的正是两者对得上。**同包不是偷懒，是把"必须一致"这件事变成物理上无法违反。**

### 测试

四条边界，先红后绿：正常、`PromoMarginRate = 0`、`PromoMarginRate < 0`、`BaselineRevenue = 0`。前两者断言 `Status == "unachievable"` 且 `RequiredIncrementalRevenue == nil`。

### 决策留痕 D-R12

> **投前与投后同包共用 `RunRate`。** 一致性靠类型共享，不靠文档约定。

---

## 6. RH4 New Store Feasibility — 新店可行性测算

### 接口

新包 `services/newstorefeasibility`，形状照 `finmodel`：纯函数引擎 + Ports。

```go
// 唯一入口。纯函数，不做 IO。
func Evaluate(in Input, ports Ports) Result

type Input struct {
    Currency string
    Business BusinessDrivers  // 客流/进店率/转化率/客单价/营业天数/综合毛利率
    Invest   InvestmentPlan   // 装修设备一次性投入/首批铺货/爬坡月数与逐月系数
    Lease    LeaseTerms       // 合同引用 + 免租期 + 租期（金额不在这里）
    Horizon  int              // 评估月数
    DiscountRate *float64     // nil = 未确定
}

type Ports struct {
    LeaseProjection LeaseProjectionReader // 唯一外部依赖
}

type Result struct {
    MonthlyCashFlows []MonthlyCashFlow
    StaticPayback    *float64  // 月
    DynamicPayback   *float64  // 月
    IRR              *float64
    NPV              *float64
    BreakEvenSales   *float64  // 月度
    Gaps             []Gap     // 具名缺口，不是 0
}
```

### 三条硬约束

**1. 禁止 import `ifrs16`。** 租金与使用权资产/负债影响一律经 `LeaseProjectionReader` 从 `measurement_results` 只读投影取。守卫照 `finmodel/importguard_test.go` 写，**遍历全部子包，不只根包** —— 那份测试当初就是因为只查根包差点漏掉。

理由与 D-S3 同：全系统只能有一套租赁计算。新店测算自己算一个 ROU，两个数字必然分叉且没人说得清该信哪个（`AGENTS.md` 风险红线 14）。

**2. 折现率缺失 fail-closed。** `DiscountRate == nil` 时 `IRR` / `NPV` / `DynamicPayback` 三项返回 nil，`Gaps` 追加 `{Kind: "discount_rate_missing"}`。**不使用任何默认值。** 这是 `AGENTS.md`「AI 不得猜测折现率」在测算侧的对应物。`StaticPayback` 与 `BreakEvenSales` 不依赖折现率，仍然返回 —— 部分降级，不是整体拒绝。

**3. 端口未接线时是具名 Gap，不 panic、不填 0。** `ports.LeaseProjection == nil` → 租赁相关行全部 nil + `{Kind: "lease_projection_unwired"}`。这是 `finmodel` 引擎已确立的行为（D-S4）。

### 深度理由

接口是一个函数。背后是逐月爬坡、免租期、抽成租金分段、现金流折现、IRR 求根、盈亏平衡反解。调用方（HTTP handler、未来的 Agent 工具、底稿构建器）各自只需要知道 `Evaluate(in, ports)`。

**为什么这里建端口而 RH5 不建**：租赁投影有两个真实适配器 —— 生产薄绑定与内存桩，后者是 Golden 测试的唯一可行路径（不能为了测 IRR 去起一个 PostgreSQL）。这符合「两个适配器才是真接缝」。

### 测试

- **Golden 逐格**：一组固定输入 → 逐月现金流与五项指标全部锁定。改任一公式即红。
- **Gap 路径**：折现率缺失、端口未接线各一条，断言具体 Gap kind 而不只是"有 gap"。
- **import guard**：遍历 `newstorefeasibility/...` 全部子包。
- **不测**：IRR 求根算法的迭代次数、内部辅助函数名。

### 与 `/pre-deal` 的关系

`predeal` 包完全不动。`/pre-deal` 页面新增一个「业务可行性」区块消费 RH4，与既有的租约商务测算并列 —— **并列，不是替换**（`AGENTS.md` 增量叠加）。建议书提出的「把 `/deal-compare` 降级为抽屉」不做：那是隐藏既有导航入口。

### 决策留痕 D-R13

> **新店测算与 `predeal` 分包。** 两者回答不同问题（业务经济性 vs 租约商务条件与会计影响），输入集合几乎不相交，且 RH4 需要 import guard 而 `predeal` 不需要。合包会让 guard 的作用域被迫扩大到不相关的代码。

### 决策留痕 D-R14

> **折现率缺失是部分降级而非整体拒绝。** 静态回本期与盈亏平衡销售额不依赖折现率，把它们一起挡掉是过度保守 —— BP 在等折现率批复的那几天里，这两个数是有用的。

---

## 7. RH5 Variance Attribution — 利润差异归因

### 接口

新包 `services/varianceattribution`，纯函数，暂不建端口：

```go
func Attribute(base, current StorePeriodFacts, order []Factor) (Result, error)

type Result struct {
    TotalVariance     float64
    Factors           []FactorContribution // 按 order 排列
    Residual          float64
    ResidualMaterial  bool
    DecompositionOrder []Factor            // 回显，前端必须展示
    Status            string               // complete | unavailable
    MissingFacts      []string
}
```

分解链与固定替代顺序在 Spec D-R5。

### 三条硬约束

**1. 顺序回显。** 连环替代法的结果依赖替代顺序 —— 换个顺序数字就变。不回显顺序的归因数字无法被任何人复核。`DecompositionOrder` 是响应的必填字段，前端必须渲染（守卫测试断言页面上出现顺序说明）。

**2. 残差不摊。** `Residual = TotalVariance − Σ Factors`，显式返回。**不得**把它摊进最后一个因子（那是最常见的做法，也是最坏的：它让最后一个因子承担全部方法误差，而读者不知道）。`|Residual| / |TotalVariance|` 超阈值时 `ResidualMaterial = true`。

**3. 缺一即整体不可用。** 任一必需事实在基期或当期缺失 → `Status = "unavailable"` + `MissingFacts` 列出缺哪些。不做部分归因：七个因子里少两个，剩下五个的贡献值是错的，不是"不完整"。

### 守恒断言的恒真陷阱（重要）

`Σ Factors + Residual == TotalVariance` 这条断言**按上面的定义是恒真的** —— 因为 `Residual` 就是这么算出来的。按 `AGENTS.md` 风险红线 12，这属于「拿定义式倒减自己」，是假装在检查。

处理方式二选一，必须选一个并写进测试注释：

- **（推荐）改写为构造断言**：显式声明这条按构造成立，测试改为断言**两条独立路径**的对照 —— 路径 A 是逐因子替代累加，路径 B 是直接用期末减期初算总差异。两者的差就是残差；断言的是「路径 A 的各步中间值符合手算的连环替代序列」，用固定 fixture 逐格锁定。
- 或者：为残差找一个独立定义（如高阶交叉项的解析式），断言两种算法一致。

**不接受**留一条恒真断言假装在验守恒。

### 深度理由（为什么不建端口）

只有一个调用方（HTTP handler），事实读取在 handler 里完成后以值传入。**一个调用方 = 假想接缝**（D-S8 的同一判断）。出现第二个消费方（如归因底稿构建器）时再提端口 —— 那时才存在两种实现要隔离。

### 与 `margindecomposition` 的关系

品类层下钻**复用** `margindecomposition` 的量/结构/价三效应，不重写。RH5 负责门店利润层的七因子，点开「毛利率效应」时下钻到 `margindecomposition` 的品类拆解。两层各自守恒，不互相校验（口径不同，强行对齐会产生一条恒真勾稽）。

### 决策留痕 D-R15

> **界面文案叫「利润差异归因」，不叫「杜邦分析」。** 杜邦是净资产收益率的三因子分解（利润率 × 周转率 × 权益乘数），本模块做的是连环替代法的差异分析。用户里有真懂杜邦的人，叫错会失去信任 —— 而信任是这个产品唯一的卖点。

---

## 8. RH6 Template Dry-Run — 模板校验

### 问题

`finmodel/template` 的编译路径完整，但只有「保存」入口。前端要么复刻解析器（两套必然分叉），要么让用户提交后才知道写错了。

### 接口

一个端点，复用同一条编译路径：

```
POST /api/v1/financial-model/templates/validate
```

请求体与 `POST .../templates` 完全相同（同一个 `TemplateDef`）。响应：

```go
type ValidationResult struct {
    Valid       bool
    Errors      []ValidationError
}

type ValidationError struct {
    RowKey    string
    Kind      string   // syntax | unknown_reference | circular_reference | invalid_lag
    Message   string   // 面向用户的中文
    Position  *int     // 公式串内的字符偏移，nil = 非位置性错误
    CycleePath []string // 仅 circular_reference：完整引用链路
}
```

**实现是薄的，接缝是关键。** handler 调 `template.Compile` 后不落库，把已有的 error 转成结构化结果。`validateNoCycles` 当前返回 `fmt.Errorf("template: circular reference: a -> b -> a")` —— 要改成携带 `[]string` 链路的错误类型，让链路可结构化返回而不是让前端去 split 字符串。

### 深度理由

这是本设计里唯一一个**故意浅**的模块。它的价值不在实现量，在于**保证只有一套解析器**。删除测试：删掉它，前端会长出第二套解析器，届时"界面说没问题、保存时报错"比现在难查得多。

### 决策留痕 D-R16

> **前端不复刻 DSL 解析器，一次校验也不做本地的。** 包括括号配对这种"显然安全"的检查也走后端 —— 一旦开了本地校验的口子，它会长大，然后与后端分叉。代价是每次输入停顿后一个网络往返，可接受。

---

## 9. RH7 Forecast Roll-Forward — 滚动预测滚入

### 接口

这是**编排**，不是计算。`finmodel` 引擎已经按 `def.ActualCutoffPeriod` 区分冻结线左右，RH7 只是把「推进截止期」做成一个动作：

```
POST /api/v1/financial-model/definitions/:id/roll-forward
  body: { new_actual_cutoff_period: "2026-09", inherit_assumptions: true }
  → { new_definition_id, new_run_id }
```

行为：以现定义为 prior 派生新定义 → 更新 `ActualCutoffPeriod` → 承继 approved 假设版本 → 经 **`finmodel/persist` 唯一写入口** 落 run。

### 三条约束

1. **不新开写入口。** `fin_model_runs` 只有一条写入口，架构测试是 `finmodel/writeguard_test.go`。RH7 走 `persist`，不为异步流程再开一条（D-S2 的既有教训）。
2. **`simulated` / `mixed` 不得 publish。** 底线 2，已有约束，滚动路径不得绕过。
3. **四线对比不新建表。** 实际 / 最新预测 / 原始预算 / 去年同期全部走 `fpna_plan_versions` 已有的 prior 谱系与 `scenario_type`。

### 深度理由

删除测试：删掉 RH7，每个 BP 每月手工建定义、改截止期、拷假设、跑 run —— 复杂度不会消失，会变成每月一次的手工流程和随之而来的操作错误。这是**流程复杂度的封装**，和封装算法一样算深度。

### 决策留痕 D-R17

> **滚动预测不是新机制，是既有 `ActualCutoffPeriod` 的可操作化。** 抵制把它做成独立模型的诱惑：那会产生第二套"什么算 Actual"的判定，而 P0-3 的教训（用假设推导 Actual 期毛利让 T13 在真实数据上必然失败）说明这条判定只能有一个来源。

---

## 10. RH8 Copy Discipline — 文案纪律

三条 CI 可强制的守卫。它们替代的是「有人 review 时会注意到」—— 而这个仓库没有第二个人类。

| 守卫 | 断言什么 | 先例 |
|---|---|---|
| 枚举不裸奔 | 源码级：`{x.status}` / `{x.kind}` / `{x.*_status}` 形态不得出现在 JSX。映射表用 `Record<联合类型, i18n键>`，漏一个后端值 TypeScript 编不过 | `web/app/financial-model/page-enums.test.ts` |
| pp 与 % 不混用 | 凡 `ChangeType === "percentage_point"` 的值必走 pp 格式化分支 | `web/app/lib/` 既有格式化测试 |
| 内部词汇不外泄 | 对 i18n **值**做黑名单断言（批次号形态、阶段号、模块代号） | F0-4 的既有做法 |

**推广范围**：`/performance`、`/promotions`、`/store-360` 三页补第一条守卫（`/financial-model` 已有）。

### 用户可见文案的写法（unslop 口径）

界面上的每一句话按这四条写。它们不是风格偏好 —— 违反其中任一条，使用者就得猜。

1. **说这个数是什么，不说它有多重要。** ✅「销售人效：每工时产生的销售额」 ❌「核心人效指标，助力精细化运营」
2. **不可用时说清为什么与怎么办。** ✅「—　工时数据缺失，去「经营事实导入」补 `labor_hours` 列」 ❌「暂无数据」
3. **不用机器词，也不用把机器词直译的怪词。** ✅「排队中」 ❌「queued」 ❌「已入队列」
4. **数字的口径写在数字旁边，不藏在帮助文档里。** ✅「经营占用成本率 12.4%（基本租金 + 服务费 + 当期变量租金 ÷ 销售额）」

**中文破折号保持中文用法**，em dash 的清除只对英文文案生效（沿用 `60b9957` 的既有口径）。

### 决策留痕 D-R18

> **文案守卫做成源码级断言而不是渲染快照。** 快照测试会被"更新快照"一键绕过；源码级断言配合 `Record<联合类型,…>` 的类型约束，漏一个后端枚举值会在 `npm run type-check` 阶段就红。

---

## 11. 与既有架构约束的对照

逐条确认本设计不降低任何既有底线。

| 约束 | 本设计如何满足 |
|---|---|
| 底线 1 跨法人隔离 | 三个新端点全部走既有 scoped reader 模式，带库集成测试逐个验 |
| 底线 2 模拟/正式区分 | RH7 的 publish 门沿用既有 `data_classification` 判定，不绕过 |
| 底线 3 来源追溯 | RH1 露出的指标全部带 `retail-kpi-v1` 既有的来源信封 |
| 底线 4 重复导入保护 | 本轮不动导入路径 |
| 底线 5 IFRS 16 台账隔离 | RH4 的 import guard；所有新模块零写入 IFRS 16 正式表 |
| 增量叠加 | 零删除。表、列、路由、API、导航项全部保留 |
| 不用 0 填补缺失 | RH1 沿用 nil 语义；RH2 降级为 `—`；RH4/RH5 具名 Gap |
| 勾稽不得恒真 | RH5 §7 明确处理了守恒断言的恒真陷阱 |
| 前端不重算 | RH6 前端零解析；RH1 前端只按 code 取值 |
| store-day 粒度 | 人效走既有 store-day 事实；时段排班明确出范围 |
| DESIGN.md §13 止血 | 本轮不新增内联样式；新面板走既有容器原语（§8.1 PRIM-001） |

---

## 12. 实施顺序与依赖

```
RH2 展示口径守卫  ─┐
RH1 指标露出清单  ─┼─► 可立即开工，互不依赖，全部不改后端契约
RH8 文案纪律      ─┘

RH3 促销保本  ─┐
RH4 新店可行性 ─┼─► 各自独立，任意顺序
RH5 差异归因  ─┘

RH6 模板校验端点 ──► 公式编辑器 UI（依赖 RH6）

RH7 滚动滚入 ──► 独立
```

**不给日期。** 团队是一位人类 + AI 协作者，没有可并行的第二条人力线；建议书按 2026-Q3 起算「一周」的排期在 2026-08-22 已不成立。

---

## 13. 决策留痕汇总

Spec 侧 D-R1~D-R9 见 [Spec](specs/retail-workstation-honesty-and-capability-r1.md)，本文接续：

| # | 决策 | 理由一句话 |
|---|---|---|
| D-R10 | 指标露出做成受校验清单，标签表收敛为一处 | 四处散落定义已经吃过一次亏，闭包校验能进 CI |
| D-R11 | 口径冲突只降级不换算 | 给第三条路就会有人走 |
| D-R12 | 投前与投后同包共用 `RunRate` | 一致性靠类型共享，不靠文档约定 |
| D-R13 | 新店测算与 `predeal` 分包 | 合包会让 import guard 的作用域被迫扩大 |
| D-R14 | 折现率缺失是部分降级 | 静态回本期不依赖折现率，一起挡掉是过度保守 |
| D-R15 | 叫「利润差异归因」不叫「杜邦」 | 用户里有真懂杜邦的人 |
| D-R16 | 前端零本地 DSL 校验 | 开了口子就会长大，然后分叉 |
| D-R17 | 滚动预测是既有字段的可操作化，不是新模型 | 「什么算 Actual」只能有一个判定来源 |
| D-R18 | 文案守卫用源码级断言 + 类型约束 | 快照测试会被一键更新绕过 |

---

## 14. 明确的非目标

不做，且各自的重新进入条件写在 [Spec §Out of Scope](specs/retail-workstation-honesty-and-capability-r1.md)。此处只列对**代码结构**的影响：

- **不合并 `/performance` 与 `/operating-pulse`**（2026-08-22 决定）。RH2 只让 `/performance` 停止说假话，不改它的后端调用。
- **不删除设备维度**。RH2 是展示层判定，`equipment_*` 表、`access_scope_sql.go` 的三层过滤、`fpna_plan_lines` 的三列一律不动。
- **不新增 hour-grain 事实层**。RH1 只露出 store-day 能支撑的人效指标。
- **不做集团合并抵销**。SM8 的薄聚合保持原样（D-S8：一个消费者 = 假想接缝仍然成立）。
- **不做密度切换**。`globals.css` 的 `!important` 级联未治理完（`docs/UIUX改善方案.md` 阶段一），在不可预测的级联上加密度变量会放大问题。
