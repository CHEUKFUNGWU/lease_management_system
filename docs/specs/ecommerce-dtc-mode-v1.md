# Spec：电商独立站经营分析模式（P0 批次）

> 编制：2026-08-26 · 状态：`draft-for-product-review`（随源 PRD 评审同步转正为 `ready-for-agent`）
> 来源：[PRD_电商独立站经营分析模式.md](../PRD_电商独立站经营分析模式.md)（E1–E9 分批；本 spec 只覆盖 **P0**：E1–E4 全部 + E5 的保本/情景部分。P1/P2 需求在本 spec 只登记不展开）
> 配套：[CodebaseDesign_电商独立站模式_模块深化.md](../CodebaseDesign_电商独立站模式_模块深化.md)（EM1–EM11 接缝设计）
> 发布位置说明：本仓无 issue tracker，按既有惯例落 `docs/specs/`

---

## 0. 本 spec 在定案什么

PRD 是「要什么、口径是什么、怎么算对、在哪测」；本 spec 把它翻译成**可验收的需求项**并**关掉全部四个待决问题**（PRD §8.3）。实施 kickoff 不再需要任何额外决策输入。

## 1. 待决问题定案

| # | 待决问题（PRD §8.3） | 定案 | 一句话理由 |
|---|---|---|---|
| ① | Storefront 与 Store 词汇冲突 | **采用 Storefront**；本 spec 附带 CONTEXT.md 词条修订（§8），实施第一批次一并提交 | Store 是物理门店根对象，电商站点需要独立的法人归属与币种语义，共用会污染行级过滤 |
| ② | 电商语义层独立还是并入 retailkpi | **独立模块 `ecomkpi`** | 事实族（站点日 ≠ store-day）与指标族（MER/CM/ROAS ≠ 坪效/人效）都不同；共享的只有「严格 null / 零分母 / Surface 启动校验」这套**形状**。并入会让一个包拥有两套 Definitions map，中文名单一真相源失效 |
| ③ | Agent 工具命名清单 | 见 §6，一次定全共 7 个 | 按 Agent_Tool_包装规范「看数据属于哪个域」逐个过判定法 |
| ④ | PayPal 滚动准备金是否进 P0 | **进 P0** | 现金影响大、实现薄（一张台账 + 占用/释放状态机）；砍掉它验收场景 2 不成立 |

## 2. 术语（规范级；词条全文见 §8）

**Storefront（站点）**、Storefront-Day Fact、Campaign-Day Fact（双口径）、Net Revenue（经营口径）、Landed Cost、CM1 / CM2、Break-even MER / ROAS、Settlement Reconciliation、Rolling Reserve、Media Rebate —— 定义以 PRD §4.1 为准，本 spec 不复制全文，只钉三条硬规则：

- **R-T1** 「站点」一词在代码、API、UI 中一律对应 Storefront；CONTEXT.md Store 词条的 avoid 注记按 §8 修订后，「Site」仍是禁用词（指物理门店时），不得借机复活。
- **R-T2** Settlement Reconciliation 与 Tie-Out 是两个词：前者是 GL 对账域的三方证据匹配，后者是模型勾稽。任何代码标识符、UI 文案、文档不得混用。
- **R-T3** 账面广告费与实付广告费是两个事实（Campaign-Day Fact 两行并存），不是同一事实的两个视图；任何聚合若需合并口径必须在 Metric Definition Version 里显式声明并出新的版本号。

## 3. 需求项（P0）

> 编号 R-E{n}-{seq}。每条含验收。「MUST」级不达成则该批次不算完成。

### E1 数据接入

- **R-E1-1** 五类来源（Shopify 订单/退款/payout、Meta/Google/TikTok campaign 日耗、代理发票、PayPal/Stripe 结算文件、3PL 账单、采购/头程发票）经受控 CSV 批次导入，每批携带完整 Source Envelope（source_system / import_batch_id / fact_version / as_of_at / data_classification）。
  *验收*：任一来源导入后，库内每行五字段非空；缺任一字段的批次整体拒绝（fail-closed）。
- **R-E1-2** 广告日耗入**账面口径**、代理发票入**实付口径**，两者在 Campaign-Day Fact 中并存为两行。
  *验收*：同一 campaign-day 可查出两行，basis 字段分别为 `booked` / `paid`；不存在第三种取值。
- **R-E1-3** 业务键幂等（平台订单号 / payout ID / 发票号）+ 请求级幂等；重放不产生第二条记录。
  *验收*：带库集成测试真 RUN——同一批次重放两次，行数不变（skip 不构成证据）。
- **R-E1-4** 无 API 来源有标准化 CSV 模板，信封字段一个不少；模板版本随导入留痕。
  *验收*：每个来源一份模板下载端点；用旧模板版本导入时被拒并提示当前版本。

### E2 经营事实层

- **R-E2-1** 分析原子粒度 = 站点 × 日 × 渠道 × SKU 聚合事实；订单行明细仅作下钻与对账证据，**不进分析读路径**。
  *验收*：所有分析端点的 SQL 只触聚合表；订单行表仅出现在下钻与对账端点（代码评审项 + 端点清单核对）。
- **R-E2-2** 退款/拒付/账单调整到达时以**新 Fact Version** 重述原期间，读取走 Highest Fact Version，被重述期间带修订标记。
  *验收*：带库集成测试——写入 v1 后写入 v2，读路径返回 v2 且 v1 仍在库；原期间标记 `restated=true`。
- **R-E2-3** 缺失显示「—」加原因；覆盖不足降级 Decision Ready = false；禁止补 0、禁止反推。
- **R-E2-4** 平台报数与内部口径冲突时并列降级展示（Source Conflict），不改名贴标签。
- **R-E2-5** 多币种默认 Currency Partition 分区展示；折算是显式第二视图且带汇率版本；无汇率整体降级。
- **R-E2-6** 销售税/VAT 不进收入行，独立负债行参与对账。

### E3 单站利润表与单位经济

- **R-E3-1** 行结构固定：GMV → 净收入 → 落地成本 → 履约 → 支付通道费 → CM1 → 广告（实付）→ CM2 → 分摊固定费 → 经营利润；支持月度与周度。
  *验收*：handler golden 锁响应形状；子合计恒由子行推导（构造性质，见 PRD §6 红线 12 类比——测中间值序列，不测恒等式本身）。
- **R-E3-2** 下钻到订单行/账单行/发票，每行携带 Source Envelope 引用（三击到来源）。
- **R-E3-3** 盈亏平衡 MER = (固定费用 + 目标利润) ÷ CM1 率；盈亏平衡 ROAS = 1 ÷ CM1 率；**CM1 率 ≤ 0 时 MUST 返回具名 `unachievable`**，不得返回数值。
  *验收*：纯函数 golden——正常值、零值、负值三组用例；负值组断言 unachievable 而非任何数字。
- **R-E3-4** 付费新客 CAC 与混合 CAC 并列展示，分子分母在响应中标明。
- **R-E3-5** 经营口径净收入与会计口径收入并列、永不互换；会计口径只读来自 GL 导入，本模式**永不自算法定收入**。
  *验收*：全仓 grep——ecom 包不得出现 revenue recognition 计算逻辑；会计收入行的唯一来源是 GL 事实读取端口。

### E4 收款对账

- **R-E4-1** payout 明细 × 订单应收 × 银行到账三方匹配，差异逐笔归入具名类别：手续费 / 汇兑 / 拒付 / 在途 / 调整 / 准备金（六类封闭枚举）。
  *验收*：匹配器纯函数 golden——六类差异各至少一条先证红再证绿的用例。
- **R-E4-2** PayPal 滚动准备金占用与释放跟踪，作为现金流预测输入。
- **R-E4-3** **口径门禁**：未对平期间的收入/现金数字不得进入 Approved 口径报表；差异自动进 Data Quality Queue。
  *验收*：集成测试——构造未对平期间，Approved 口径查询不含该期间数字，Data Quality Queue 出现对应条目。
- **R-E4-4** 对账结果 Draft → Prepare → Pending → Approved 签认，职责分离留痕（复用既有审批链）。
- **R-E4-5** Auditor 只读导出对账证据链与口径版本。

### E5 保本与情景模拟（P0 部分）

- **R-E5-1** 输入广告预算 + CPM/CPC/CVR/AOV，输出 GMV / CM / 保本 MER 情景对比。
- **R-E5-2** 改价 / 运费 / 折扣敏感度模拟。
- **R-E5-3** 模拟输出全程带 Data Classification 模拟标识，永不混入正式口径（底线 2）。
  *验收*：响应体顶层 classification 字段断言 + 前端契约测试登记该枚举。

## 4. 端到端验收场景（产品级，PRD §2.3 场景 1–4 原样适用）

P0 完成定义为：场景 1（周一脉搏 ≤10 分钟）、场景 2（月结对账两工作日，含准备金与返点差异表）、场景 3（大促保本 ≤10 分钟）全部达成且上述 R 项全绿。场景 4（Agent 归因问答）属 P1/E8，不在本 spec。

## 5. Agent 工具命名（待决问题③定案）

| 工具名 | 读/写 | 判定依据 |
|---|---|---|
| `retail.site_pulse.read` | 读 | 经营脉搏数据，零售经营域 |
| `retail.site_diagnostics.read` | 读 | 站点诊断，同上 |
| `retail.site.scenario.evaluate` | 读（评估不落库） | 情景评估，经营域 |
| `fpna.site_pnl.read` | 读 | 利润表是财务报表面 |
| `fpna.settlement.read` | 读 | 对账属 GL 对账域（财务） |
| `fpna.settlement_recon_draft.create` | 写→只写草稿层 | 对账签认建议落草稿 |
| `fpna.ecom_assumption.suggest` | 写→只写草稿层 | 预算假设建议，`source=ai_suggestion` |

全部声明 Permissions；approved-only 读取路径永不回采草稿；工具数运行时枚举（`Runtime.Describe` golden 清单扩一行），注册失败 fail-fast。**不开第四个根命名空间。**

## 6. 测试缝（沿用 PRD §6，落到守卫）

1. Handler JSON golden（最高缝）：四个新页面的 API 各一组；
2. 纯函数 golden：sitepnl 投影、settlement 匹配器（六类差异 × 先红后绿）、保本评估器（三组）、情景评估器；
3. 启动校验：`ecomkpi.MetricSurface` 引用的 code 必须存在于 Definitions，否则启动失败；前端新枚举登记契约测试（CONTRACT-001 同款）；
4. 带库集成：幂等（R-E1-3）、重述（R-E2-2）、跨法人隔离（storefront 属 legal_entity，行级过滤）、口径门禁（R-E4-3）——**RUN 过才算数**；
5. Agent 注册守卫：工具清单 golden 扩容 + 尝试数 == 成功数。

## 7. 明确不做（本 spec 边界）

PRD §7 全部继承。追加两条工程边界：① 订单行明细不建分析索引（它是证据不是查询对象）；② 不为电商指标建第二张中文名映射（唯一真相源在 `ecomkpi`）。

## 8. 随附：CONTEXT.md 词条修订草案（实施第一批次一并提交）

- **新增 Storefront 词条**：定义见 PRD §4.1；Related: Legal Entity（归属）、Currency（收单币种）；Avoid: Shop / Site / 门店。
- **修订 Store 词条 avoid 注记**：「Site 禁用于物理门店；电商域站点使用 Storefront（见该词条），不要把电商站点挂到 Store 上」。

---

*历史注记：P1/P2（E6–E9：Contribution Bridge、预算滚动、Agent 归因问答、库存×现金流、多主体往来、LTV cohort）在本 spec 登记不展开；启动时按同格式出 v2 增补。*
