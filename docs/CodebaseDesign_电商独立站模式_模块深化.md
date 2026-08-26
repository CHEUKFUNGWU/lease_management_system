# CodebaseDesign：电商独立站模式 模块深化

> 编制：2026-08-26 · 状态：`draft-for-product-review`（随 PRD / spec 同步转正）
> 规格：[specs/ecommerce-dtc-mode-v1.md](specs/ecommerce-dtc-mode-v1.md)（需求项 R-E*-{seq} 在那里，本文只讲接缝）
> 词汇：Module / Interface / Seam / Adapter / Depth / Leverage / Locality，按 codebase-design 深模块词汇表使用，不换词。
> 范围：P0 模块（EM1–EM7）给全接口；P1/P2（EM8–EM9 等）只立边界防误入。

---

## 0. 设计原则（判定标准）

1. **模式复用，不复制实现。** 零售侧已经付过学费的形状——受控导入的 IngestBatch 接缝、KPI 语义层的 Surface 启动校验、利润表的纯函数投影、差异进队列才有人处理——电商侧**同形不同体**：新表族、新指标族、新包，但接口形状照抄已验证的那个。判断标准：抄的是接口纪律，不是代码。
2. **每个新包必须通过删除测试。** 删掉它，复杂度若只是搬进 handler 或前端，它就不该存在。
3. **两个 adapter 才是一条真接缝。** EM2 的 FactReader 有三个真实消费方（sitepnl、settlement、ecomkpi），这条缝是真的；凡是只有一个调用方的「接口」，一律降级为包函数。
4. **纯函数优先，端口未接线退化为具名 Gap**——沿用三表模型的既有纪律，不 panic、不填 0。

## 1. 现状诊断：直接复用 vs 新建

| 既有资产 | 复用方式 |
|---|---|
| `controlledintake` + `sourceenvelope` | 原样复用（C3 刚收拢成 IngestBatch 接缝；sourceenvelope 是读写共享深模块，**不吞并**） |
| `retailkpi` 的语义层形状 | EM3 平行复刻其纪律：Definitions map、Surface 启动校验、中文名单一真相源、严格 null |
| `storepnl` 投影形状 | EM4 同形：纯函数 + Readers 注入 + 子合计由子行推导 |
| `varianceattribution` 连环替代 | EM8（P1）模式复用，顺序回显纪律原样 |
| Data Quality Queue / draftapp 审批链 / workingpaper Review Gate | EM5 差异队列与对账签认直接挂现有对象 |
| Agent 治理链 / 工具注册 fail-fast | EM10 原样；工具清单 golden 扩容 |

**新建**只有五件事：站点日事实族（EM2）、电商语义层（EM3）、单站利润表投影（EM4）、对账引擎（EM5）、单位经济+情景（EM6/EM7）。

## 2. 接缝总览

```
CSV 五来源 ──► EM1 ecomintake ──► repository(事实表族) ──► EM2 ecomfact.FactReader
                                                            │
                    ┌───────────────┬───────────────────────┼─────────────┐
                    ▼               ▼                       ▼             ▼
              EM4 sitepnl      EM5 settlement          EM3 ecomkpi    EM10 tools
                    │               │                  (Surface 校验)     │
                    ▼               ▼                                     ▼
              EM6 unitecon ◄── EM7 ecomsim                      /site-pulse 等页面
           （保本/单位经济）   （情景模拟）
```

一条主缝贯穿：**EM2 FactReader 是唯一事实读口**，三个消费方（EM4/EM5/EM3）都从它取数，谁也不私连事实表。

## 3. EM1 `services/ecomintake` — 受控导入编排

**问题**：五类来源各自一套列结构，但没有各自一套导入纪律。幂等、信封、模板版本必须只有一份实现——刚在 C3 收拢的 IngestBatch 形状就是为这一刻准备的。

**接口**：

```go
func ImportBatch(ctx context.Context, spec SourceSpec) (*ImportResult, error)

type SourceSpec struct {
    Source    string            // shopify | ads_booked | ad_invoice | settlement | 3pl | procurement
    BatchID   string            // 请求级幂等键
    Rows      []RowEnvelope     // 业务键 + 载荷 + 信封字段
    TemplateVersion string
}
```

**藏在接口后面**：业务级幂等（平台订单号/payout ID/发票号）、信封完整性校验（缺字段整批拒绝）、模板版本比对（旧版本拒绝并报当前版）、经 `controlledintake` 的重放短路、分块落库。

**为什么独立于 retailingest**：事实族不同（store-day vs storefront-day/campaign-day），业务键不同；共享的是 controlledintake 与 sourceenvelope 这两层，不是编排本身。合并会让一个 Service 拥有两套业务键规则。

**删除测试**：删掉它，五套来源各自的导入分支落回 handler，幂等键定义开始漂移——底线 4 从第一天就破。

## 4. EM2 `services/ecomfact` — 事实读口（唯一事实缝）

**问题**：退款 120 天后到达要重述期间，「读最新版本」这个决定如果由各消费方自己做，必然有人读了 v1。

**接口**：

```go
type FactReader interface {
    StorefrontDays(ctx, f EntityFilter, w Window) ([]StorefrontDayFact, error) // 已解析 Highest Fact Version
    CampaignDays(ctx, f EntityFilter, w Window, basis AdBasis) ([]CampaignDayFact, error) // booked | paid
    OrderLines(ctx, evidenceRef EvidenceRef) ([]OrderLine, error) // 仅下钻与对账路径
}

func Highest(facts []Versioned) Versioned        // 版本选择只有这一份
```

**藏在接口后面**：Highest Fact Version 解析、修订标记（restated=true 谱系）、币种分区过滤、跨法人行级过滤（storefront → legal_entity，底线 1 从第一查询就生效）、订单行证据裁剪列。

**两条铁律**：
- 分析端点 SQL 只触聚合事实表；触订单行表的代码路径必须经过 `OrderLines` 且携带证据引用（R-E2-1 的可核对形态）。
- 本包**禁 import `ifrs16`**（P2 的 finmodel Port 反向只读接入，方向是 finmodel 读电商事实，不是相反）。

## 5. EM3 `services/ecomkpi` — 电商指标语义层

**问题**：「MER 是什么口径」「CM 分母含不含退款」这类问题如果没有单一真相源，growth 团队和财务的争论会逐字重演零售侧踩过的坑。

**接口**：

```go
var Definitions = map[string]Definition{...}   // 每个 Definition 挂 Metric Definition Version
var Surface = []SurfaceEntry{...}              // 哪些 code 露出到哪个页面
func Evaluate(code string, facts ..., opts) (Value, error)
```

**藏在接口后面**：严格 null / 零分母语义、覆盖门槛 → Decision Ready 降级、Source Conflict 拒收、Currency Partition、中文名唯一真相源（消费包不得建第二张 labels map）。启动时校验 Surface 引用的 code 必须存在，否则启动失败——与 retailkpi 同款守卫，测试同构。

**为什么独立而不是并入 retailkpi**（决策留痕 D-E3）：见 spec 待决问题②定案。一句话：共享纪律，不共享 Definitions map。

## 6. EM4 `services/sitepnl` — 单站利润表投影

**问题**：单站利润表是一张确定性报表，不是一组 KPI 卡。子合计必须由子行推导，口径被挑战时要能给出证据链。

**接口**（纯函数，一切输入注入）：

```go
func Project(req SitePnlRequest, readers SitePnlReaders) (*SitePnlStatement, error)

type SitePnlRequest struct {
    Storefront StorefrontRef; Period PeriodSpec   // 月度 | 周度
    Breakdown  Breakdown                          // channel | campaign | sku（下钻维）
}
type SitePnlReaders struct {
    Facts   ecomfact.FactReader   // 经营口径全部行
    GL      GLRevenueReader       // 会计收入行，只读来自总账
    Fixed   FixedCostReader       // 分摊固定费
}
```

**藏在接口后面**：固定行序 GMV→净收入→落地→履约→支付费→CM1→广告(实付)→CM2→固定费→经营利润；每行携带构成拆分与 Source Envelope 引用（三击到来源）；经营口径与会计口径是响应类型上的**两个块**，basis 标签上在块级；缺失行 nil 不填 0。

**红线**：会计收入行的唯一来源是 GL reader——本包没有任何收入确认计算，这是 R-E3-5 的工程落点。

## 7. EM5 `services/settlement` — 收款对账引擎与口径门禁

**问题**：三方匹配（payout × 订单应收 × 银行到账）是整个模式的信任锚点。「未对平不得进 Approved」如果只是一条提示，它会被无视；必须是门。

**接口**：

```go
func Match(payouts, receivables, bankLines []Line, policy MatchPolicy) []MatchResult
// MatchResult: Matched | Diff{Category, Amount, EvidenceRef}
// Category 六值封闭枚举: fee | fx | chargeback | in_transit | adjustment | reserve

func ApprovalGate(period Period, results []MatchResult) GateVerdict
// 全对平 → allow；否则 → deny + 差异清单（自动进 Data Quality Queue）
```

**藏在接口后面**：六类具名差异的判定规则（每类至少一条先红后绿 golden）；准备金占用/释放台账的状态机（reserve 类别的载体）；对账单签认流挂既有 Draft→Prepare→Pending→Approved 链与职责分离中间件。

**为什么 ApprovalGate 是独立函数而不是 sitepnl 内部检查**：两个真实调用方（报表口径服务、月度回顾包），且它是底线级门禁——单独成函数才能被集成测试独立钉住（R-E4-3）。

## 8. EM6 `services/unitecon` — 保本与单位经济

**接口**：

```go
func BreakEven(cm1Rate decimal.Decimal, fixedCost, targetProfit Money) (BreakEvenResult, error)
// cm1Rate <= 0 → Unachievable（具名值，非 error、非巨大数字）

func CACView(paid, blended CACInput) CACReport   // 付费新客与混合并列，分子分母显式
```

纯函数无端口。**为什么不并入 sitepnl**：三个消费方（sitepnl 页面、ecomsim、agent tools）都要保本语义，而它们不需要整张利润表——独立小包让 ecomsim 不必拖起投影。

## 9. EM7 `services/ecomsim` — 大促与定价情景

**接口**：

```go
func EvaluateBFCM(input ScenarioInput, readers CMReader) (*ScenarioOutput, error)
func PriceSensitivity(delta PriceDelta, base CMBase) (*ScenarioOutput, error)
```

输出顶层强制 `data_classification=simulated`（响应类型字段，不是约定）；永不写正式表；现金缺口提示（备货+广告预充+回款滞后）P0 只做静态公式版。**不做**：预测模型、补货优化（PRD §7）。

## 10. EM8–EM9（P1，只立边界）

- EM8 Contribution Bridge：复用 `varianceattribution` 形状——量/价/混合/汇率/广告效率五因子，顺序固定并回显；守恒等式是构造性质，测中间值序列。Attention Signal 不足证据显式 Suppressed。
- EM9 预算滚动：**不建第二套版本对象**，直接消费 `fpna_plan_versions`；Growth 角色建议只走 `fpna.ecom_assumption.suggest` 草稿。

## 11. EM10 Agent Tools（命名定案见 spec §5）

七个工具全部按包装规范声明 Permissions；写类只落草稿层（`source=ai_suggestion`）；approved-only 读不回采草稿；`scope_denied` 原样透传。注册进既有 bundle 结构（lease/fpna/retail 三 bundle 中归属对应域），工具清单 golden 追加七行，运行时枚举核对。

## 12. EM11 Pages（四个路由）

`/site-pulse`、`/site-360`、`/site-pnl`、`/settlement-workbench`。骨架照 §8.2 页面范式（PageHeader→筛选条→DataTrustBar→主体→脚注）；取数走既有 useRetailQuery 缝、状态走 classifyDataState/StateBlock、新枚举登记 CONTRACT-001 契约测试；样式约束 DESIGN.md §13 强制。**前端零计算**：所有行、合计、评分来自后端。

## 13. 数据模型与双交付纪律

新表族（迁移同时交付增量文件 + `01_init.sql` 空库版）：`storefront`（法人 FK、币种、市场）、`storefront_day_facts`、`campaign_day_facts`（booked/paid 双行）、`order_line_evidence`（裁剪列）、`payout_lines` / `bank_lines` / `ad_invoices` / `media_rebates` / `rolling_reserve_events`、`settlement_runs`（签认状态机）。事实族全部携带五信封字段；分析表建索引，证据表不建分析索引。

## 14. 测试策略

三条主缝两守卫，spec §6 已列；守卫落点：`ecomkpi` surface 启动校验测试、前端契约测试扩容、`tool_inventory_golden_test.go` 追加七工具、带库集成四件套（幂等/重述/隔离/口径门禁）。**skip 不构成证据。**

## 15. 决策留痕汇总

| # | 决策 | 一句话理由 |
|---|---|---|
| D-E1 | ecomintake 独立于 retailingest，共享 controlledintake/sourceenvelope | 事实族与业务键不同；共享层已是深模块 |
| D-E2 | EM2 是唯一事实读口，三消费方共用 | 一条真接缝（≥2 adapter）；Highest Version 只实现一次 |
| D-E3 | ecomkpi 独立于 retailkpi | 共享纪律不共享 Definitions map；中文名真相源不可二主 |
| D-E4 | 会计收入只读 GL 端口，sitepnl 无收入确认逻辑 | 两口径永不互换的工程落点 |
| D-E5 | ApprovalGate 独立函数 | 底线门禁需要被独立钉住 |
| D-E6 | unitecon 独立小包 | 三个消费方只要保本语义 |
| D-E7 | 保本 unachievable 是具名返回值非 error | 「不可达成」是有效结论不是失败 |
| D-E8 | ecomsim 分类标识在响应类型上 | 底线 2 靠类型不靠自觉 |

## 16. 依赖与实施顺序

```
EM1 ──► EM2 ──┬──► EM3 ──► EM11(页面)
              ├──► EM4 ──► EM6 ──► EM7
              └──► EM5（可与 EM4 并行，互不依赖）
EM10 依赖 EM2–EM6 的读口稳定后收尾；EM8/EM9 P1 再启。
```

关键路径：EM1→EM2→EM4。EM5 与利润表线并行推进（对账不依赖投影）。

## 17. 明确不做 / 克制清单

- 不做 API 连接器（P0 只有受控 CSV）；不做实时；不动 marketplace；
- 不给订单行证据表建分析索引、不做它的通用查询面；
- 不建第二套预算/版本对象、第二张中文名映射、第二套租赁或收入计算；
- 不吞并 sourceenvelope / controlledintake 进任何电商包（它们是跨域共享层）；
- P1/P2 内容（Bridge、预算、Agent 问答、库存×现金流、多主体、LTV cohort）只留边界不留桩。
