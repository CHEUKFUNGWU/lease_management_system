# Codebase Design：零售经营分析工作站 — 模块深化设计

> 配套文档：《PRD_零售经营分析工作站_BP日常支撑完善.md》（要什么）。
> 本文用 codebase-design 词汇（**module / interface / seam / adapter / depth**）描述「模块长什么样、接缝在哪、藏在接口后面的行为、怎么测」。只描述接口与接缝，不写文件路径与实现细节；实施批次按接口契约落地。
> 所有现状描述均已在仓库中验证（2026-08-15 两份 Review——BP/AI 双视角与数据链路「文件 → AI 解析 → 入库 → BI 展示」——实锤 + 代码复核；数据链路 9 项断言全部复核通过）。

## 0. 设计原则（判定标准）

- **深度 = 接口的杠杆**：小接口 + 大量藏在里面的行为。删除测试：删掉这个模块，复杂度是消失了，还是重新扩散到 N 个调用方？后者说明它在挣自己的饭钱。
- **接缝要真实**：一个适配器 = 假想接缝；两个适配器 = 真实接缝。不引入没有东西跨过它的接缝。
- **接口即测试面**：调用方与测试跨同一接缝。想测穿接口内部，多半是模块形状不对。
- **接受依赖，不创建依赖；返回结果，不产生副作用；接口面越小越好。**

## 1. 现状诊断：浅的地方与缺口

| 缺口 | 现状 | 为什么是问题 |
|---|---|---|
| 环比公式 | 两处实现：pulse 用 `(c/p−1)×100`，store360 桥用 `(c−p)/|p|×100` | 同一语义两套行为，负基数时方向相反（−100→−50：一处 −50%、一处 +50%）。修复要改两处、测试写两遍 |
| 期间语义 | `window_days ∈ {7,14,28}` 在三处服务各自校验，默认值 7/14/14 不一致；页面硬编码重置为 7 天；as_of 语义三页不一 | 口径混乱是既成事实，无单点可修 |
| 零售导出 | 三页零导出；先例存在：FP&A 行动 CSV 导出、前端工作簿库 | 无口径头、无分类标识规范，导出各自为政会再次制造口径漂移 |
| 预算对比 | fpna-version-lifecycle spec 已写未实现；零售日粒度无 plan 比较 | FP&A 每日工作量的大头，语义层没有对应能力 |
| 跨店视图 | 语义层聚合已支持 `group_by=total|region|brand|store`，KPI 端点已暴露；pulse 服务与页面未接 | 能力存在但被浅层接线挡住 |
| AI 路由 | 关键词匹配 skill registry | 匹配策略太浅：「A 门店毛利下滑」这种自然问法漏匹配 |
| AI 引用 | 模型没写引用时回退挂全部已知来源 | 接口语义含糊——「没有引用」与「引用全部」不分，保真度缺陷 |
| AI 护栏 | 全库无 RateLimit；context 无上限（全量 dump + 未校验 History） | 模块不存在 |
| 数据入口 | store-day 事实无文件入口（只有 JSON API ≤500/批与模拟器两个写入方）；月度导入器后端通、前端断（web 客户端函数零调用）；预算仅 JSON；Trial Balance 无路由 | 财务无法把 POS 导出的 Excel 弄进 BI；唯一非模拟入口是手写 JSON——开发者行为，不是用户行为 |

## 2. 接缝总览

| 接缝位置 | 模块 | 适配器 | 测试面 |
|---|---|---|---|
| 语义层（纯函数） | retailkpi 差异计算（M1）、预算对比（M4） | — | 单元测试（testdata 先例） |
| 期间解析（纯函数） | retailperiod（M2） | — | 单元测试：边界 |
| 导出（纯函数 + handler 接线） | retailexport（M3） | CSV writer（Go）/ XLSX（前端既有库） | 单元测试 + handler 测试 |
| 服务构建 | retailpulse 扩展（M5） | — | 单元测试 + handler 测试 |
| plan 数据源 | M4 的 PlanReader | fpna_plan_lines / 模拟 plan（两个适配器） | 集成测试 + 单测 |
| agent 护栏 | agentguard（M6） | 内存计数 / DB 计数（两个适配器） | 单元测试 |
| skill 匹配 | agentskill 深化（M6） | — | 单元测试：路由用例 |
| 决策卡 | renewal（M7） | 与计量引擎共用 snapshot/projection 接缝 | 单元测试 + e2e 零写入证据 |
| 文件导入 | retailingest（M8） | 受控模板解析（Go 确定性）/ AI 列映射建议（Assist Mode） | 单元测试 + handler + 集成（幂等） |

## 3. M1 差异/环比语义单源 — retailkpi

**目标**：全产品只有一个「变化率/差异」语义，符号方向、零分母、空值行为只定义一次。

**接口**（收敛到既有函数，不新增）：

```go
func ChangeRateType(code string) string                                   // "pct" | "pp" | "delta" ...
func ChangeRate(current, comparison *float64, changeType string) (*float64, string)
```

**藏在接口后面**：公式选择（环比百分比 / 百分点 / 绝对差异）；负基数方向约定；零分母与空值；变化方向在基准为负时的稳定性。

**关键行为变更**：基准约定统一为 `(c−p)/|p|`（分母取绝对值）作为唯一契约，pulse 的 `(c/p−1)` 弃用。从 −100 改善到 −50 必须显示为「改善」，两个页面一致。这是行为变更，需回归既有展示与测试。

**删除测试**：删掉它，两套公式的符号分歧重新出现，修复扩散到 KPI 卡、桥、关注榜等 N 个展示点。

**接缝**：语义层纯函数，无适配器。修 P0-3，并成为 M4 预算差异的方向基准。

## 4. M2 期间规范 — retailperiod（新模块）

**目标**：滚动窗口（7/14/28 天）与日历期（月/季）只有一个解析、一组默认值、一套重置语义。修 Review 实锤的「窗口口径混乱」，并支撑日历月/季查询（P1/P2）。

**接口**：

```go
type Period struct {
    Kind   PeriodKind   // rolling | calendar
    Days   int          // rolling: 7|14|28
    Year   int          // calendar
    Month  int          // calendar: 1..12（0 表示不限定月）
    Quarter int         // calendar: 1..4（0 表示不限定季）
}

func Parse(spec string, asOf time.Time) (from, to, compareFrom, compareTo time.Time, err error)
func Default(kind PeriodKind) Period          // 全产品统一默认（滚动 14）
func Label(p Period) string                   // "近 14 天" / "2026-07" / "2026-Q3"
func Normalize(p Period) (Period, error)      // 枚举合法性、日历边界
```

**藏在接口后面**：日历月/季的起止边界与上一等长比较期的解析；`as_of` 与期间类型的交互（as_of 是日历日锚点，滚动窗口从它往回推，日历期以它为「落在哪个月/季」）；「上月」「本季度」的自然语言 → 期间映射（P2 前先支持显式 `YYYY-MM` / `YYYY-Qn`）；与既有 7/14/28 校验的兼容收口。

**为什么深**：三处服务校验 + 三页默认值 + 导出口径 + 预算对比的期间配对，全部收敛到一个解析入口。删除测试：删除后，默认值不一致、硬编码重置、as_of 漂移的复杂度重新扩散到 6+ 处。

**边界**：不做 fiscal 日历（`period_basis` 里的 fiscal_month 留待真实客户需求）；不改动既有 URL 参数形态（`window_days` 继续可用，新增 `period` 参数与其互斥）。

## 5. M3 零售导出 — retailexport（新模块）

**目标**：零售三页的导出行为与口径头只实现一次。导出与展示共享同一受控响应，不重读仓库（对齐既有 projection 纪律：先有受控响应，再投影导出）。

**接口**：

```go
type ExportKind string   // "operating_pulse" | "store_diagnostics" | "scenario"
type Format    string    // "csv" | "xlsx"

func Export(kind ExportKind, format Format, response any, envelope sourceenvelope.Envelope) (filename string, content []byte, err error)
```

**藏在接口后面**：口径头生成（data_classification、dataset_version、as_of、formula_version、period/窗口、group_by、法人）；Working/模拟标识（文件名 `operating-pulse-2026-08-15-working.csv` + 表头标记）；CSV 转义与 BOM；XLSX 工作簿组装（多 sheet：KPI 卡/表/桥/明细）；列名与排序的单一来源。

**接缝与适配器**：handler 层同一 GET 端点带 `format=csv|xlsx` 参数，响应即数据源。适配器决策：服务端只出 CSV（Go 标准库，口径权威、审计可复演）；XLSX 由前端用既有工作簿库从同源响应生成。两个适配器（服务端 CSV / 前端 XLSX）⇒ 真实接缝。CSV 先行，XLSX 跟进。**后续扩展**：PPTX 为 P1 后续项（PRD P1-19），生成位置（服务端或前端库）与格式枚举扩展待真实汇报需求确认后决定；口径头与 Working/模拟标识必须与 CSV/XLSX 一致。

**测试面**：行结构、口径头、转义、文件名、分类标识、非法 kind/format 的错误路径。

## 6. M4 预算对比 — retailkpi.ComparePlan + PlanReader 适配器

**目标**：落地 fpna-version-lifecycle spec 的语义（budget/forecast/scenario 版本、Actual 只读、比较下钻、ties_out），并在零售侧把「本月实际 vs 本月预算」做成与 KPI 同纪律的语义层能力。

**接口**（语义层，纯函数）：

```go
type PlanFact struct {
    StoreID string; Period string        // "2026-07"
    Currency string
    Revenue, GrossProfit, LaborCost, OccupancyCost, OtherCost *float64
    VersionType string                   // budget | forecast | scenario
    Source string
}

type PlanVariance struct {
    KPI                string
    Actual, Plan, Variance, VariancePct float64
    MaterialityExceeded bool
    DecisionReady      bool
    DowngradeReason    string
}

func ComparePlan(actual []DailyFact, plan []PlanFact, req Request) ([]PlanVariance, error)
```

**藏在接口后面**：期间配对（用 M2 的日历期把 store-day 聚合到月/季）；覆盖率门槛复用（同群纪律：样本不足/覆盖不足降级 `DecisionReady=false` 并说明原因）；材料性阈值（从既有系统设置读取，不硬编码）；币种混杂显式降级；null 语义（缺 plan 行 ≠ 0）；不编造数据点。

**接缝与适配器**：`PlanReader` 接口（`QueryPlan(ctx, legalEntityID, period, storeIDs) ([]PlanFact, error)`）。适配器一：既有 plan 行表（store 粒度，月线）；适配器二：模拟 plan（固定 seed，与模拟事实同标识纪律）。两个适配器 ⇒ 真实接缝。

**依赖**：M2（期间配对）。lease 侧的版本比较（budget vs forecast 下钻合同、ties_out）按既有 spec 独立落地，与 M4 共享「Actual 只读」语义但接缝不同。

## 7. M5 跨店/区域视图 — retailpulse 扩展

**目标**：经营脉搏支持 `group_by=total|region|brand|store`，复用既有语义层聚合与信号规则，不新增端点族。

**接口**：扩展既有 Query 与 Build（服务接口形状不变）：

```go
type Query struct {
    // 既有字段不变
    GroupBy string   // "" | "total" | "region" | "brand" | "store"；空 = 保持现状（per-currency 分区）
}
```

**藏在接口后面**：`GroupBy=""` 时行为与现在完全一致（零回归）；有 GroupBy 时走语义层聚合，并把信号规则应用到分组级（区域级关注榜）；分组内多币种处理（分区或显式降级）；分组样本门槛沿用 MinimumPeerCount 语义；分组的 drilldown URL 模板。前端加分组切换，复用既有响应契约。

**为什么深**：「分组聚合 + 信号 + 门槛 + 降级」整段行为藏在一个参数后面；调用方（页面、Agent Tool、导出）接口不膨胀。

## 8. M6 AI 意图路由、引用保真与护栏

### 8.1 路由加深 — agentskill

**接口不变**：`Select(intent Intent) (Definition, bool)`。**实现加深**：高频经营词并入 retail skill 匹配（毛利、下滑、为什么、同群、贡献、闭店、异常…）+ 中文表达变体（“为咩”“为何”）+ 数值特征（门店名 + 指标词）+ 兜底顺序与优先级。深度判定：匹配策略的复杂度从调用方移入 registry，调用方接口不膨胀。**明确不引入** LLM function calling 主链路（保持确定性，与治理模型一致）。

### 8.2 引用保真 — 提取器

**行为契约**：模型未写引用 → 返回空列表，回答标注「无引用」；**删除全量回退**。正则解析与 token 匹配保留；UI 对空引用显示「未附来源」。语义从「引用可以挂一切」变为「引用 = 模型明确声明的来源 ∩ 已知来源」。

### 8.3 护栏 — agentguard（新模块）

**接口**：

```go
type BudgetStore interface {
    Allow(ctx, userID string, kind string) (bool, error)      // 速率 + 成本预算检查
    Record(ctx, userID string, kind string, tokens int, cost float64) error
}

type Guard struct { store BudgetStore; cfg Config }           // Config: 每分钟消息数、每日成本上限、context 预算

func New(store BudgetStore) *Guard
func (g *Guard) Check(ctx, userID string, kind string) error  // 拒绝返回 429 + 原因（不软化）
```

**藏在接口后面**：滑动窗口速率（每分钟消息数）；每日成本上限（按模型单价估算）；context 组装预算（字符上限 + 截断策略：有界摘要优先于全量 dump；History 截断到近 N 条）；拒绝语义（429 + 原因，不软化，对齐「权限拒绝必须保持原因」纪律）。

**适配器**：内存计数（单实例、测试用）与 DB 计数（跨实例）两个适配器 ⇒ 真实接缝。**接入点**：Web chat 与 agent-runner 共用；agent-runner 既有预算（max-tool-calls / max-model-tokens）语义对齐。

### 8.4 AI 工程收尾（P3 补丁级改动，不改模块形状）

Review 在 AI 工程评估中点名的次级项，均为既有模块内改动，不构成新接缝：

- **会话持久化**：零售页内嵌 AI 抽屉的对话上下文跨页面往返保留，或明确定义一次性会话语义并在 UI 提示（PRD P3-32 二选一）。测试：页面往返后上下文保留的组件测试。
- **死代码清理**：既有深链构造被真实接线调用，或确认无调用方后删除（PRD P3-33 二选一）。测试：移除后全仓无引用。
- **访问审计收口**：合同读取从绕过 Tool runtime 的直连改为经 Tool runtime（带审计与维度收窄），与「权限拒绝必须保持原因」纪律一致。测试：审计日志可断言的集成测试。
- **置信度来源化**：零售 Agent 置信度从硬编码改为由覆盖率、样本量、规则命中强度推导，输出向后兼容。测试：样本不足时置信度下降的单元用例。
- **流式响应**：Web chat 长回答从同步阻塞/DB 轮询改为增量输出，或明确超时与进度语义；与 agent-runner 预算语义对齐。测试：分片输出/超时语义。

## 9. M7 续租/终止决策卡 — renewal（新模块）

**目标**：按《renewal-termination-decision-card》spec 落地「续租 / 重谈 / 终止」决策卡，兑现工作台 E（占用成本与合同情景）这一核心差异化。

**接口**：

```go
type Scenario struct {
    Decision       string      // renew | renegotiate | terminate
    TermMonths     int
    MonthlyRent    float64
    FreeRentMonths int
    EscalationPct  float64
    OtherMonthlyCost float64
    DiscountRateSource string   // 空 → 拒绝 PV/ROU/负债输出
}

type Card struct {
    ContractSnapshot  any        // 不可变快照
    Scenarios         []ScenarioResult  // 逐年现金/费用/EBITDA/ROU/负债/退出成本
    OperatingMetrics  any        // 最新 store-day 经营分母 + 数据版本标注
    Sources           []SourceTag // formal_data | scenario_assumption
}

func Build(input CardInput) (Card, error)
func Save(card Card, owner, opinion, decisionDate, evidence string) (snapshot, error)
```

**藏在接口后面**：情景模型（renew / renegotiate / terminate 的会计与经济展开）；折现率来源解析（缺失 → 拒绝输出，不猜测——既有 HITL 纪律）；退出成本（剩余承诺、终止罚金、ROU 冲销、负债解除、P&L 影响）；五年现金/费用/EBITDA/ROU/负债曲线；`formal_data` vs `scenario_assumption` 来源标记；经营分母取最新 store-day 事实并标注数据期间/来源/版本；保存为不可变快照（不随合同后续变化漂移）。

**接缝**：与计量引擎共用 snapshot/projection 接缝（先加载受控快照，再投影决策卡），不绕行仓库直读；复用既有折现率解析与付款计划。**边界**：不自动批准事件、不写正式台账、不替代法务对终止条款的解释。

**测试面**：折现率缺失拒绝；快照不可变；保存/加载后正式合同/付款计划/计量结果零写入（e2e 前后计数，先例：MAX-009）；五年曲线总额可复核。

## 10. M8 零售事实导入 — retailingest（新模块）

**目标**：把「POS/财务导出文件 → 受控的 production store-day 事实」整段行为藏在一个导入模块后面，复制合同侧已验证的流水线模式（受控模板 + 批次 + 数据质量留痕），解锁真实数据试点（PRD P5，Review 二判定的最高杠杆项）。

**接口**（导入页的四个阶段对应四个函数，调用方只学四个签名）：

```go
type RawRow   []string
type Mapping  map[string]string        // 文件列名 → 标准字段
type Envelope struct { SourceSystem, ImportBatchID string; AsOfAt time.Time }

func ParseTemplate(file []byte, format Format) (headers []string, rows [][]string, err error)
func SuggestMapping(headers []string) (Mapping, error)          // AI 建议，Assist Mode，人工确认
func Validate(mapping Mapping, rows [][]string) (ValidationReport, error)
func Commit(ctx, rows [][]string, mapping Mapping, envelope Envelope, idempotencyKey string) (*ImportReport, error)
```

**藏在接口后面**：受控模板解析（表头规范化、列映射、位置解析）；行级错误收集与部分成功；预计覆盖率（对既有 store-day 数据的时间/门店重叠估计）；数值解析（Go 确定性——LLM 只建议映射，不读数字）；envelope 强制（source_system / import_batch_id / as_of_at 缺一拒绝）；500 行/批分块与既有原子 upsert 复用；幂等（Idempotency-Key + payload SHA，重放不产生第二条）；只产 production 事实；批次与数据质量留痕（与既有批次模式同构）。

**接缝与适配器**：新 handler 端点（沿用既有 store-day 写入权限）+ 新「经营数据导入」页。适配器：受控模板解析（Go 确定性）与 AI 列映射建议（LLM adapter，Assist Mode）两个适配器 ⇒ 真实接缝；AI 输出只到映射建议为止，人工确认后才进入确定性路径。

**删除测试**：删除后，「文件 → 事实」的表头校验、行级错误、信封、幂等、分批逻辑重新扩散到页面与 handler，且大概率各自为政再次漂移。

**测试面**：幂等重放（同文件 + 同 Key 只落一批）、行级错误与部分成功、覆盖率报告、envelope 缺字段拒绝、分批边界；AI 映射建议不含数值；集成断言只写 store-day、零写 IFRS 16 正式表。

## 11. 快速修复清单（不改模块形状）

既有模块内的小修，不构成新接缝；第 1–10 项随 P0 交付，P1/P5 小项随各自批次交付：

1. store-360 深链丢失 store_id：发现数据集的 effect 补写 store_id（与相邻分支一致）。
2. scenario-workbench 未评估时伪造「决策就绪」：无结果时渲染「未评估」态，不渲染可信条。
3. scenario 页 selectedKey 允许切回 baseline（「什么都不做」对照可见）。
4. ai-chat 读取 `?message=` 预填问题（与既有 `page/title/tags` 键处理对齐）。
5. 情景假设 URL 回写：setQuery 写入七个杠杆参数，与 parseURL 对称（分享链接可复现）。
6. 采纳 AI 提案时验证窗口与评估窗口一致（不塌缩）。
7. 混币门店币种归属：不做 last-write-wins；显式降级或按币种分区（retailpulse 内部修正）。
8. 覆盖超额检测：重复 (store_id, business_date) 行报警（覆盖语义补全，与覆盖不足对称）。
9. ai_chat 集成测试注释两处弯引号还原（与所引 SQL 逐字一致）。
10. pulse 输入即查询：本地 state + Apply（store-360 已有先例，收敛做法）。

### P1 小项（同样不改模块形状）

11. 原始事实列表 API 增加 `data_classification` 过滤参数：收紧列表读取面（PRD P1-18；聚合路径保持现状安全）。测试：handler 参数回显与错误路径。
12. 导出格式枚举扩展 PPTX（PRD P1-19，后续项）：无真实汇报需求可延后；口径头与 Working/模拟标识与 CSV/XLSX 一致。

### P5 小项（同样不改模块形状）

13. 月度导入器僵尸收口：/performance 补导入 UI（客户端 API 已存在，纯接线）或明确 deprecated 标注（PRD P5-48）。测试：上传路径可用性或页面无死链。
14. 解析格式补齐：ai-service 解析适配器引入 AnyDoc（Rust/MIT/纯本地）接管 office 家族（docx/doc、ppt、xls/xlsb、odt、rtf、epub → GFM Markdown），CSV 用标准库确定性解析；PDF/图片路径不动（PaddleOCR 保持）；docx 无坐标体系，证据降级为 quote 锚点（PRD P5-51）。测试：docx/ppt/xls 中文用例、加密/超限错误路径、quote 锚点证据。

## 12. 测试策略

- **接口即测试面**：每个模块的测试从它的接口驱动；不测穿接口内部。
- M1：负基数方向（−100→−50 显示改善）、零分母、空值、既有输出回归（golden 对数）。
- M2：月末/季末边界、as_of 交互、默认值全产品一致、非法输入错误路径。
- M3：口径头、转义、文件名、分类标识；handler 集成（`format` 参数）。
- M4：覆盖率不足、混币、材料性阈值、null（缺 plan 行 ≠ 0）；PlanReader 双适配器。
- M5：`GroupBy=""` 零回归；分组信号、门槛、多币种处理。
- M6：路由命中（自然问法）、引用空回退、护栏拒绝语义 + 预算记录；agenttools 只读属性回归。
- M7：折现率缺失拒绝、快照不可变、正式台账零写入（前后计数）。
- M8：幂等重放、行级错误与部分成功、覆盖率报告、envelope 强制、分批边界；AI 映射建议不含数值；集成断言只写 store-day、零写 IFRS 16 正式表。
- 集成测试沿用 `TEST_DATABASE_URL` 模式（未设置时 skip）；每批跑通 AGENTS.md 验证命令。

## 13. 依赖与实施顺序

```
P0:  M1 + 快速修复清单                        （无新依赖，直接修可信度）
P1:  M2（期间）+ M3（导出）+ M5（区域视图）+ 事实列表过滤与 PPT 后续小项
P2:  M4（预算对比，依赖 M2）+ fpna-version-lifecycle 落地
P3:  M6（路由 + 引用 + 护栏 + 工程收尾）
P4:  M7（决策卡，依赖既有 snapshot/projection 与折现率解析）
P5:  M8（导入器 + 导入页）+ 月度僵尸收口 + FP&A 导入（与 P2 联动）+ 解析格式补齐
```

每批独立可交付、独立验收；顺序即优先级：可信度 → 日常价值 → 差异化。**P5-1（store-day 导入器）按 Review 二为最高杠杆项，建议排期与 P0/P1 并行，批次编号不代表先后。** 所有批次遵守五条底线与「模拟/正式」标识纪律，导出与决策卡输出必须带 Working/模拟标注。
