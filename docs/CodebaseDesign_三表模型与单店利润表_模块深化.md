# Codebase Design：三表财务模型与单店利润表 — 模块深化设计

> 配套文档：《PRD_三表财务模型与单店利润表.md》（要什么、口径是什么、怎么算对）。本文用 codebase-design 词汇（**module / interface / seam / adapter / depth**）描述「模块长什么样、接缝在哪、藏在接口后面的行为、怎么测」。只描述接口与接缝，不写实现细节。两份需同步维护。
> 前序文档：《CodebaseDesign_财务BP与FPA支撑补齐_模块深化.md》（N1–N13，已落地）。本文的 SM1–SM8 与其共存，不重复其接缝；模块编号用 SM（Statement Model）以免与 PRD 的批次号 S1–S5、勾稽号 T1–T16 混淆。
> 所有现状断言均已在 HEAD 上核实到具体文件与行号（§1）。

## 0. 设计原则（判定标准）

沿用前序文档 §0 的五条，不再复述。本轮新增两条：

- **纯函数承载全部会计语义，副作用只剩一步。** 三表引擎的 `Run` 是纯函数（输入全注入、输出全是值），落库是单独的 `Persist`。先例已存在：`fpna.Compose/Commit`（`fpna/composition.go:57,160`）。Golden 对数、AI 无副作用试算、正式重跑复用同一个函数——**这是「相同输入必得相同结果」这条验收的结构性保证**，不靠纪律。
- **能用类型让非法状态不可表达的，就不用运行时校验**（前序文档已立）：缺失值用 `*float64` 的 nil 表达（0 填充这条路径在类型上不存在）；勾稽失败的 run 用类型/状态机挡住发布路径，不靠「记得检查」。

## 1. 现状诊断：这一轮要处理的浅结构

| 位置 | 现状 | 为什么是问题 |
|---|---|---|
| 报表结构 | `fpna_plan_lines` 是扁平固定列（revenue…four_wall_ebitda、capex、net_debt，`01_init.sql:1667`），无科目树、无行类型 | 利润表/BS/CF 的结构无处安放。若在页面里硬编码行结构，「门店利润表」与「三表模型」会长成两套各自维护的科目表 |
| 门店利润表 | 四墙 EBITDA 是 `retailkpi` 的一个指标（`retail_kpi.go:108` Definitions 之一），/store-360 是诊断卡 | 指标 ≠ 报表。没有「从快照派生的确定性报表投影」承载逐行下钻与双口径并列 |
| 三表与勾稽 | 全库无 balance sheet / 三表 / 间接法 CF 实现 | 全新模块，但必须嵌进既有投影与版本治理纪律，不能长成孤岛 |
| 租赁 roll-forward 读取 | `reporting` 已有 `LiabilityRollforward`（`disclosure_projection.go:52`）与 `SnapshotBuilder` / `Project(snapshot, ProjectionRequest)`（`snapshot.go:90`、`projection.go:169`） | 披露投影是年度披露口径。模型要按月 roll-forward——正确做法是沿 Report Projection 接缝新增一个 ProjectionKind，而不是让模型包 import `ifrs16` 自算 |
| 资金计划 | `cashplan.Compose(ctx, Request, Sources)` 已落地（`cashplan.go:153`），带租金不双算抵消与 ConservationBridge | 它是**现金口径**计划；三表 CF 是**权责锚定的间接法**。两者回答不同问题，复用其读取器端口与守恒纪律，不合并模块 |
| 折算 | `currencytranslation.NewBasis → Translate → Total(TranslatedSet)` 已用类型锁死「无汇率版本无跨币种合计」（`translation.go:113,156,202`） | S5 直接消费，不重做 |
| AI 模型域工具 | `agenttools/tools/` 无任何 statement/model 工具 | 新增三个工具的 descriptor 形状已有先例：`NewRetailPaperDefinition(reader)`（`retail_paper.go:35`）、`NewS1GenerateDefinition()`（`s1_generate.go:26`） |
| 报表底稿 | `workingpaper` 协议（`Paper/Cell/Provenance/Basis/CoverPage`，`protocol.go`）+ `Lint(p, audits)`（`lint.go:42`）+ 两个构建器（s1、retail，`Build(in Input) (workingpaper.Paper, error)`） | finmodel 构建器是同形状的新增包，协议零改动 |

## 2. 接缝总览

| 接缝位置 | 模块 | 适配器 | 测试面 |
|---|---|---|---|
| 模板解析与校验（纯） | SM1 Statement Template | TemplateStore：Postgres / 内存（两个） | 单元测试：DSL 拒绝路径 |
| 模型引擎（纯 + 一个写入口） | SM2 Model Engine | 五个读取端口各两个适配器（生产薄绑定 / 内存桩） | Golden 对数 + 勾稽反向测试 |
| 单店利润表投影（纯） | SM3 Store P&L Projection | 读取端口：retailkpi / 计划行 / 计量投影（生产 / 桩） | 单元测试：双口径隔离、降级 |
| 期初三道闸（纯） | SM4 Opening Balance Gate | —（纯函数，两个调用方） | 单元测试：三道闸各自反向用例 |
| AI 假设建议（草稿写入口） | SM5 Assumption Suggestion | SuggestionWriter：Postgres / 内存（两个） | 单元测试 + 工具集成 |
| 底稿构建（纯） | SM6 finmodel WorkingPaper Builder | —（协议复用，无新端口） | 保值断言 + lint 门 |
| Agent 工具 | SM7 Statement Model Tools | —（descriptor 注册，治理链全过） | 工具契约 + 越权语义测试 |
| 集团视图 | SM8 Group View（克制，不建服务包） | 复用 currencytranslation | 复用其类型约束测试 |

---

## 3. SM1 Statement Template — 报表模板（PRD S3 核心，S2 的前置）

**问题**：报表结构（科目树、行类型、公式、格式）若不成为一个值对象，它会同时硬编码在单店利润表页、三表引擎、导出模块三处——三处必然漂移。

**接口**：

```go
// 解析即校验：非法模板根本不存在 Template 值
func Parse(def TemplateDef) (*Template, error)

// 模板是不可变值：行树 + 编译后的公式 AST + 行索引
type Template struct {
    Version  TemplateVersion
    Rows     []Row          // tree flattened with depth; RowKind: input|link|formula|subtotal|check
    formulas map[RowKey]ast // 编译产物，对外不可见
}
```

**藏在接口后面**（这里才是深度）：

- **DSL 解析与白名单文法**：行内引用（`rows.xxx`）、跨期引用（`lag(rows.xxx, n)`）、四则、`sum/avg/min/max`、`if`、除零保护比率。SQL 关键字、任意标识符、跨法人引用在**解析期**拒绝并定位到行——不是运行期报错。
- **lint 规则即 PRD 的公式纪律**：① 0 和 1 以外的数值字面量拒绝（`=收入*1.05` 在类型层面无法通过 Parse）；② 行引用图拓扑排序，循环引用拒绝；③ 同一公式行的逐期一致性（公式编译一次应用于全部预测期，「中途变公式」不存在语法承载）；④ basis 混行检测（subtotal 的子行跨 operating/ifrs16 两口径时拒绝）。
- **版本冻结语义**：Template 一旦从 Store 读出即不可变；「修改」= Parse 一个新版本。
- 行索引与求值顺序推导（拓扑序）——求值在 SM2，但**顺序**由模板给出，两模块共享同一份行图，不存在两套排序逻辑。

**接缝**：`TemplateStore` 端口，两个适配器（Postgres / 内存）——符合两适配器规则。模板 rows 存 JSONB（版本冻结、整体读写），对齐 `fpna_scenario_drafts.assumptions` 的 JSONB 先例；不为行建规范化表——没有按行查询的真实调用方，那是假想接缝。

**删除测试**：删掉它，科目结构散进 SM2/SM3/导出三处；「两口径 EBITDA 并排但不混行」这条纪律退化成每处各写一遍的运行时检查。

**决策留痕 D-S1**：**模板即值**。模板不是可在线编辑的活配置，而是经 Parse 校验后不可变的版本化值。编辑即新版本。这同时回答了 PRD §11 开放问题 1 中「公式 DSL 是否允许跨模板引用」——首版不允许：行引用的闭包就是模板自身，Parse 期即可全量校验，无需跨模板解析协议。

---

## 4. SM2 Model Engine — 三表模型引擎（PRD S2）

**问题**：三表联动 + 双口径 + 勾稽 + 降级 + 复演，是本轮最深的模块。最大的结构风险是把它写成「handler 里的一串 SQL + 计算」，于是确定性、Golden 可测性、AI 试算复用全部丧失。

**采用既有先例**：`fpna.Compose/Commit` 已确立「纯函数 Compose + 唯一副作用入口 Commit」的形状（`fpna/composition.go:57,192`）。SM2 是同形状的放大。

**接口**：

```go
// 纯函数：全部输入注入，输出全是值。AI 试算、Golden 对数、正式重跑共用。
func Run(def ModelDef, in ModelInputs) (*RunResult, error)

// 唯一副作用入口，幂等键在这里
func (w *RunWriter) Persist(ctx context.Context, result *RunResult, key string) (*RunRecord, error)

type ModelInputs struct {
    Actuals   FactReader             // store-day 聚合（生产适配器 = retailkpi 薄绑定）
    Lease     LeaseRollforwardReader // 租赁 roll-forward（生产适配器 = reporting.Project 薄绑定）
    Schedules ScheduleReader         // 合同付款计划
    Assumps   AssumptionReader       // fpna_assumption_versions（仅 approved + 本次试算草稿）
    Opening   OpeningBalanceReader   // 期初 BS（经 SM4 闸）
}

type RunResult struct {
    Lines    []LineValue      // 行 × 期间；Value *float64，nil = 缺失（D-S4）
    TieOuts  []TieOutResult   // T1–T16 全量执行结果
    Gaps     []DataGap        // 覆盖不足 / 缺期初 / 混币 / 模拟标识
    Versions VersionSet       // 数据 / 假设 / 模板 / 模型定义 / 汇率 五条版本线
}
```

**藏在接口后面**：

- 期间展开与 Actual 冻结线（actual cutoff 左侧只读聚合，右侧驱动预测）
- 驱动应用：SSSG / 新店爬坡 / 比率法 / 天数法 / 合同驱动（租金行逐合同读付款计划）
- **计算顺序即附录 B 的联动图**：IS → 附表（营运资本 / PPE / 租赁引用）→ CF（间接法）→ BS（现金由 CF 回填，留存收益滚动）——BS 不存在 plug 科目这条代码路径（D-S5）
- 双口径分支：两套行集独立求值，共用 Actual 输入
- 勾稽 T1–T16 全量执行，结果作为值返回（不是日志）；`TieOutResult{ CheckCode, Period, Expected, Actual, Diff, Status }`
- 降级：缺期初 BS → BS 与间接法 CF 的行集为空 + Gaps 记录原因，IS 不受影响（PRD S2-3）
- 期初余额法计息；平均余额开关若在模型定义里开启，迭代上限与收敛阈值在引擎内闭环，不向调用方暴露迭代细节

**接缝**：五个读取端口，各两个适配器（生产薄绑定 / 内存桩）。**克制声明**：生产适配器就是对既有服务签名的薄绑定——`LeaseRollforwardReader` 的生产实现直接调 `reporting.Project`（含新增的逐月 roll-forward ProjectionKind），**不为它再包一层「租赁服务门面」**；端口存在的唯一理由是让 Golden 与勾稽测试可以在纯内存里跑。

**架构约束（有牙齿的两条）**：

1. **import guard**：`finmodel` 包不得 import `ifrs16`——租赁数字只能经 `LeaseRollforwardReader` 以投影结果的形式进入（先例：`agentcore/importguard_test.go`）。这条测试替代「code review 会拦住有人图省事直接调计量引擎」。
2. **唯一写入口**：除 `RunWriter.Persist` 外，全仓库无第二条写 `fin_model_runs` 的路径（架构测试锁定，先例：N10 的 retailingest 约束测试）。

**删除测试**：删掉它，三表联动逻辑只能住进 handler 或前端；「相同输入必得相同结果」从结构保证退化成口头承诺；AI 试算（SM7）将被迫复制一份计算——两套计算，迟早一套是错的。

**决策留痕**：
- **D-S2 纯函数 + 唯一写入口**（对齐 Compose/Commit）。
- **D-S3 租赁数字只经投影端口进入模型**，import guard 强制。
- **D-S4 缺失用 `*float64` nil 表达**：0 填充路径在类型上不存在；合计行遇 nil 子行降级 partial 并记录 Gap，而非静默按 0。
- **D-S5 现金由 CF 回填，无 plug**；勾稽失败是值（`TieOuts`），发布门禁在 Persist 与报表生成两处各自 fail-closed（一处被绕过还有一处）。

---

## 5. SM3 Store P&L Projection — 单店利润表投影（PRD S1）

**问题**：`/store-pnl` 是一张报表，不是一组 KPI 卡。它要从一份受控快照派生确定性视图——CONTEXT.md 的 **Report Projection** 已经定义了这个深模块形状：「投影政策拥有口径、时间桶、聚合、响应形状；HTTP 只解码请求、取快照、编码结果」。SM3 是该形状在门店利润表的实例。

**接口**：

```go
func Project(tmpl *Template, req PnlRequest, readers PnlReaders) (*StorePnl, error)

type PnlRequest struct {
    Store StoreRef; Period PeriodSpec       // 日历期复用 retailperiod/periodutil
    Versions VersionPair                    // Actual / Budget / Forecast / PriorYear 任选两列
    Basis  BasisMode                        // operating | ifrs16 | side_by_side
    PeerColumn bool                         // 同群中位数/分位列（样本不足显式降级）
}
```

**藏在接口后面**：

- Actual 列：store-day 事实经 `retailkpi` 聚合（**不重算**——SM3 调语义层，不从事实表私算）；预算列：`fpna_plan_lines`（store 粒度，复用 `retailkpi/plan_compare.go` 的 PlanReader 形状）
- IFRS 16 口径行：ROU 折旧 / 租赁利息经同一 `LeaseRollforwardReader` 端口进（与 SM2 共用端口，两个消费方是这条接缝真实性的证明）
- 双口径并列的响应形状：两个口径块各自带 basis 标签，合计路径隔离（T15）——**响应类型上就是两个块**，不是一个块里混着两种行
- 下钻载荷：每行携带构成拆分（如占用成本 → 合同级）与 Source Envelope 引用，使「三击到来源事实」是数据自带的路径而非前端拼凑
- 覆盖度合成与 Decision Ready 降级；缺失行 nil 不填 0（D-S4 同源纪律）
- 同群列：`retailcohort` 消费，降级原因透传

**与 SM2 的关系（决策留痕 D-S6）**：**单店利润表与三表模型共用同一个出厂 Template**（SM1）。SM3 是只读投影（无驱动、无预测引擎），SM2 是可运行模型；两者渲染同一模板结构，所以「门店利润表」在页面、在模型、在导出里长得一样——一处结构定义，三个消费方。SM3 不复用 SM2 的引擎（它不需要驱动与滚动），SM2 不复用 SM3 的投影（它需要按模板求值而非按快照投影）——**共享的是模板，不是计算**。

**删除测试**：删掉它，双口径并列与下钻载荷的组装逻辑落进 `/store-pnl` 页面代码，前端开始重算合计与口径分支——直接违反「前端零计算」纪律。

---

## 6. SM4 Opening Balance Gate — 期初三道闸（PRD S2-3 验收的承载者）

**问题**：三道闸（自平衡 / 归并跨期一致 / 租赁余额对引擎）有两个调用方——导入向导（写入前校验）与模型引擎（运行前校验）。写两遍 = 两套闸，必然一松一紧。

**接口**（纯函数，无端口——它是值校验器，不是读取器）：

```go
func ValidateOpening(bs OpeningBalance, leaseRef []ContractBalance, policy MergePolicy) []GateFailure
// 三道闸全过 → 空切片；任一不过 → 具名失败（gate 编号 + 差异金额 + 定位）
```

**藏在接口后面**：① 逐历史期间自平衡（容差 ±0.01）；② 归并规则的跨期稳定性（同一外部科目跨期映射到不同标准行 = 失败；MergePolicy 随导入模板版本传入，是数据不是适配器——与 N3 的 ComparabilityPolicy 同理）；③ 期初租赁负债/ROU 与计量引擎同期余额的逐合同勾稽。

**为什么是独立模块而不是 SM2 的内部函数**：两个真实调用方（导入路径、运行路径）。删掉它，两道调用各写一份闸，「期初带病」就会从松的那道漏进来。

**决策留痕 D-S7**：闸的判定是**纯函数**，失败是**值**而非 error——调用方决定呈现（向导标红 / 引擎拒绝启动），判定逻辑只有一份。

---

## 7. SM5 Assumption Suggestion — AI 假设建议（PRD S4-2/S4-3）

**问题**：AI 填数的价值在初稿，风险在「一键全收」架空人工确认。约束必须结构化，不能靠提示词。

**设计**：不建预测引擎。建议值的**推导**调用既有只读能力（retailkpi 趋势 / 同群 / 计量投影），建议值的**落地**只有一种形状：

```go
// 工具侧（Draft 级）：产出即 draft，source = ai_suggestion，附依据与置信度
type SuggestionDraft struct {
    AssumptionKey string
    Value         json.RawMessage
    Basis         []EvidenceRef  // 具体工具调用 + 数据范围 + 期间；空 = 该建议不得产出
    Confidence    float64        // 由覆盖率 / 样本量 / 规则强度推导（P3-35 纪律），不允许硬编码
    Sourcetag     string         // 恒定 "ai_suggestion"
}
// 唯一写入口（两个适配器：Postgres / 内存）
func (w *SuggestionWriter) SaveDrafts(ctx context.Context, drafts []SuggestionDraft, key string) error
```

**藏在接口后面**：

- **结构性的「无依据不建议」**：`Basis` 为空切片时 `SaveDrafts` 拒绝——AI 编不出数字不是因为提示词写得好，是因为没有依据的建议在类型校验里过不去
- 批量初稿 = 一组 draft 按区分块（收入/费用/营运资本/CAPEX/税务），确认按块进行；「全部接受」在 UI 是二次确认 + 整块留痕（谁、何时、依据版本），**不是工具的一个参数**
- draft → approved 的迁移只认人工确认接口；工具侧不存在 approved 写路径
- 无法建议项（无历史新店）显式列出并说明原因——写进草稿的「未覆盖清单」，不静默跳过

**删除测试**：删掉它，AI 建议会找别的表落（或被拼进 approved 假设），「AI 不直接生效」从结构退化成约定。

---

## 8. SM6 finmodel WorkingPaper Builder — 报表底稿构建器（PRD S4-5）

**采用既有先例**：`workingpaper/retail` 已确立形状——`Build(in Input) (workingpaper.Paper, error)`，单元格 provenance 三类构造（`systemFact / certified(callID, engine) / human`，`retail/retail.go:119-145`），nil 跳格（`numCell` 返回 `(Cell, bool)`），DataGaps 承载降级，构建产物必过 `Lint`（`TestBuildPaperPassesLint`）。

**接口**：同形状新包 `internal/workingpaper/finmodel`：

```go
func Build(in Input) (workingpaper.Paper, error)
// Input = 一份已 Persist 的 RunResult + 封面元数据（五条版本线 + basis + 生成人）
```

**藏在接口后面**：

- **透传而非重算**：每个数字单元格的值与 provenance 1:1 来自 RunResult 的 LineValue——构建器不持有任何公式。保值断言锁死（先例：`TestBuildPreservesEngineValuesOneToOne`）
- 勾稽行进底稿：T1–T16 的结果作为 check 区块呈现，失败的 run 在封面与区块双重标红——**底稿是勾稽门禁的第二处 fail-closed**（D-S5）
- 模拟标识 / 混币 / 覆盖缺口 → DataGaps 逐条进入（先例：`TestBuildMultiCurrencyAndSimulationGaps`）
- 封面页：模型定义版本、模板版本、数据版本、假设版本、汇率版本、basis、生成时间、生成人

**删除测试**：删掉它，报表生成会退化为「前端把表格导出成 xlsx」——单元格 provenance、lint 门、封面版本线全部消失，底稿与正式报表再也无法互相复演。

---

## 9. SM7 Statement Model Tools — Agent 工具（PRD S4-1/S4-5）

**三个工具，按既有工具分级注册**（descriptor 形状先例：`NewRetailPaperDefinition(reader)` / `NewS1GenerateDefinition()`）：

| 工具 | 级别 | 行为 |
|---|---|---|
| `fpna.statement_model.read` | Read | 读模型定义 / 版本 / run 结果 / 勾稽状态；法人范围与权限过滤复用 scoped reader 先例（`retail_paper.go` 的 scopedRetailReader 模式） |
| `fpna.statement_model.evaluate` | Simulation | **调用 SM2.Run 纯函数**做无副作用试算：输入 = 当前 approved 假设 + 对话中的假设草稿；不落库、不覆盖任何版本（Retail Scenario「只评估不落库」语义） |
| `fpna.working_paper.finmodel.generate` | Draft | 调 SM6 构建器 → artifact；LevelDraft + Review Gate；xlsx/docx 导出复用既有 artifacts export 端点 |

**治理**：三工具全过 `agentcore/hooks` 的固定中间件链（TenantScope → CapabilityCheck → ProtectedMeasure → BudgetGuard → IdempotencyGuard → ReviewGate，`hooks/governance.go`）；`scope_denied` 原文透传不改写。注册进评测：新增 `finmodel_sanctity` category（先例：`retail_paper_sanctity`）。

**关键纪律**：`evaluate` 与正式 run **共用同一个 SM2.Run 函数**——「AI 试算的数」与「正式发布的数」不可能因实现分叉而不同。这是 SM2 纯函数设计的第二次回报。

---

## 10. SM8 Group View — 集团视图（PRD S5，克制项）

**不建独立服务包。** 集团视图 = 授权法人集合上若干 RunResult 的按期间汇总 + `currencytranslation` 的显式折算（`NewBasis/Translate/Total` 已用类型锁死「无版本无合计」）。首版它是一个薄的聚合函数，挂在模型读取 handler 后面。

**决策留痕 D-S8**：一个消费者 = 假想接缝。等出现第二个消费方（如集团级底稿构建器）再提为模块——那时折算与汇总的分工才有真实的两种实现要隔离。

**必须守住的类型纪律**：跨币种合计只接受 `TranslatedSet`（currencytranslation 的既有类型约束）；无权限法人在汇总请求里被 TenantScope 挡在读取层，聚合函数永远见不到无权限数据——「不泄露、不静默省略」由 run 清单的显式标注承载（含每个法人的授权状态），而非聚合时才发现。

---

## 11. 数据模型与双交付纪律

PRD §6.1 的五张表方向不变，补两条设计决定：

1. **模板 rows 存 JSONB**（版本冻结、整体读写、无按行查询方）——对齐 `fpna_scenario_drafts` 先例；**run lines 规范化存表**（行 × 期间 × 值 + provenance JSONB）——有真实的按行按期间查询方（报表渲染、版本对比、底稿构建）。
2. **全部 schema 变更双交付**：增量迁移（`db/migrations/`）+ `db/init/01_init.sql` 空库版本，缺一即环境漂移（既有硬规则，PRD 已重申，此处登记为模块验收的一部分）。

新枚举（行类型、basis、check_code、计息方法、利息列报政策、assumption source 的 `ai_suggestion`）登记进 code-lists 契约测试（CONTRACT-001 形状）。

## 12. 测试策略

沿用「**替换，不叠加**」与「接口即测试面」：

- **Golden 模型对数**（本轮最有价值的一条）：固定 seed 数据集 + 固定合同组合 + 固定假设集 → `SM2.Run` 输出逐项对数（测试跨 `Run` 接口驱动，内存桩注入，无需数据库）。模型定义或求值逻辑的任何变更必须重跑 Golden。
- **勾稽反向测试**：T1–T16 每条至少一个「故意破坏必然红」的用例。勾稽测试恒真是本模块最典型的假绿——每个用例先证「破坏后红」，再证「修复后绿」。
- **DSL 拒绝路径**：数值字面量、循环引用、跨法人引用、SQL 关键字、basis 混行、lag 越过首期间——逐条 Parse 期拒绝并定位。
- **三道闸反向用例**（SM4）：不平 / 归并漂移 / 租赁余额不符，各阻止一次。
- **架构测试三条**：① `finmodel` 不得 import `ifrs16`（D-S3）；② 除 `RunWriter.Persist` 外无第二条 run 写入路径；③ `storepnl` / `finmodel` 响应类型不包含「无 basis 标签的数字」（响应形状即 T15 的承载）。
- **保值与 lint**：SM6 复用 retail 构建器的测试组形状（1:1 保值、nil 跳格、DataGaps、必过 Lint）。
- **AI**：draft 假设不进正式 Run（D-S2 的 AssumptionReader 只放 approved + 显式试算草稿）；无依据建议被拒（SM5 类型校验）；越权 `scope_denied` 原文透传。
- **集成**：真实 PostgreSQL 走 `TEST_DATABASE_URL`；IFRS 16 正式表零写入 e2e（前后计数）；回归 `go test ./... && go vet ./...`、`npm run type-check && npm run build && npm test` 全绿。

## 13. 决策留痕汇总

| # | 决策 | 理由一句话 |
|---|---|---|
| D-S1 | 模板即值：Parse 校验 + 不可变 + 版本化；首版不允许跨模板引用 | Parse 期全量校验成立的前提就是引用闭包在模板内 |
| D-S2 | 引擎纯函数 Run + 唯一写入口 Persist | 确定性、Golden 可测、AI 试算复用，同一函数三处受益 |
| D-S3 | 租赁数字只经投影端口进模型，import guard 强制 | 全系统只有一套租赁计算，不会有两套 |
| D-S4 | 缺失 = `*float64` nil，无 0 填充路径 | 「不用 0 填补缺失」从纪律升级为类型 |
| D-S5 | 现金由 CF 回填、无 plug；勾稽失败是值；发布与底稿两处 fail-closed | 平衡是算出来的，不是填出来的 |
| D-S6 | 单店 P&L 与三表模型共用同一出厂模板 | 一处结构定义，三个消费方 |
| D-S7 | 期初三道闸是纯函数、失败是值 | 两个调用方一份判定 |
| D-S8 | 集团视图不建独立服务包 | 一个消费者 = 假想接缝 |
| D-S9 | DSL 白名单 + Parse 期 lint（字面量 / 循环 / 跨法人 / basis 混行） | 口径纪律由解析器执行，不靠 reviewer 肉眼 |

## 14. 依赖与实施顺序

```text
SM1（模板/DSL）──► SM2（引擎）──► SM3 共用模板（可并行于 SM2 后半段）
                └─► SM4（闸，随 SM2 输入路径落地，导入向导同用）
SM2 ──► SM6（底稿构建器）──► SM7（三工具注册）
SM5（假设建议）可与 SM2 并行起步，但其工具注册排在 SM7 同批
SM8（集团视图）最后，依赖 currencytranslation 联调与 ≥2 法人的真实数据形态
```

**必须守住的顺序**：SM1 早于 SM2——三表结构由模板承载，硬编码先行必返工（PRD §7 已立）。SM6/SM7 依赖一份可 Persist 的真实 RunResult，不要对着假数据建底稿。

## 15. 明确不做 / 克制清单

- 不为五个读取端口各建「服务门面」；生产适配器是既有服务签名的薄绑定（§4 克制声明）
- 模板行不建规范化表（无按行查询方，§3）
- 集团视图不建服务包（D-S8）
- 不做模板在线编辑器的「活配置」形态（D-S1：编辑即新版本）
- 不在 SM3 里实现预测驱动、不在 SM2 里实现快照投影（共享模板，不共享计算，§5）
- 平均余额计息首版不开放；开关、迭代上限、收敛阈值按 PRD S2-8 预留在模型定义里，不在引擎里先写分支
