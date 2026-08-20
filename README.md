# 线下零售经营分析工作站 / Retail Performance Workstation

> 中文：面向线下连锁零售承租方的经营分析工作站。把门店销售、毛利、客流、人工、占用成本与承租合同连接起来，完成「发现问题 — 解释原因 — 模拟方案 — 形成行动」的闭环。IFRS 16 租赁会计从产品全部下沉为一个高价值合规模块。
>
> English: An operations-analytics workstation for offline retail chains that lease their stores. It connects store sales, gross profit, footfall, labour and occupancy cost with the underlying lease contracts, closing the loop from *detect → explain → simulate → act*. IFRS 16 lease accounting is no longer the whole product — it is now one high-value compliance module inside it.

**一句话定位 / Positioning:** 从门店经营信号到可验证行动 · From store signals to verifiable actions.

---

## 目录 / Table of Contents

- [当前状态 / Current Status](#当前状态--current-status)
- [产品功能 / Product Capabilities](#产品功能--product-capabilities)
- [五条不可突破的底线 / Five Non-Negotiable Boundaries](#五条不可突破的底线--five-non-negotiable-boundaries)
- [技术栈 / Tech Stack](#技术栈--tech-stack)
- [项目结构 / Repository Layout](#项目结构--repository-layout)
- [快速开始 / Getting Started](#快速开始--getting-started)
- [关键页面 / Key Screens](#关键页面--key-screens)
- [验证命令 / Verification](#验证命令--verification)
- [开发约束 / Development Constraints](#开发约束--development-constraints)
- [关键文档 / Key Documents](#关键文档--key-documents)

---

## 当前状态 / Current Status

转型采用**增量叠加**方式：新增零售经营分析能力，既有租赁台账、IFRS 16、月结、报表和 Agent 能力全部保留且继续可用。

The transformation is **additive**. Retail operations analytics was layered on top; every pre-existing lease-administration, IFRS 16, month-end close, reporting and Agent capability remains in place and reachable.

| 事项 / Item | 状态 / Status |
|---|---|
| 零售经营分析 MVP（MAX-001 → MAX-009）<br>Retail analytics MVP (MAX-001 → MAX-009) | ✅ 全部验收通过（2026-08-13）<br>All nine tickets accepted (2026-08-13) |
| 端到端演示链路<br>End-to-end demo chain | ✅ 固定 seed 数据 → 晨检 → 门店下钻 → 情景 → 行动草稿，真实浏览器复演通过<br>Fixed-seed data → morning check → store drill-down → scenario → action draft, replayed in a real browser |
| 既有租赁 / IFRS 16 能力<br>Existing lease & IFRS 16 capability | ✅ 零删除、零重命名、零隐藏；旧路由 smoke 通过<br>Nothing deleted, renamed or hidden; legacy route smoke passed |
| 客户与商业验证<br>Customer & commercial validation | ⚠️ **未验证**。当前只完成固定 seed 的内部模拟验证，不能表述为客户采用、真实 ROI 或 PMF<br>**Unvalidated.** Only internal fixed-seed simulation has been done. This must not be presented as customer adoption, realised ROI or product-market fit |
| 真实 POS / ERP 数据接入<br>Live POS / ERP integration | ⚠️ 未完成，是试点第一阻塞项<br>Not done; the number-one blocker for a pilot |
| IFRS 16 计量回归<br>IFRS 16 measurement regression | ⚠️ 22 用例 / 148 断言通过，但标准答案仍为 `pending_third_party_review`，正式审计背书需第三方会计师复核<br>22 cases / 148 assertions pass, but the golden answers remain `pending_third_party_review`; formal audit endorsement requires a third-party accountant |
| 三表财务模型与单店利润表<br>Three-statement model & store P&L | ✅ S1–S5 全部编号功能项落地并有测试锁定（2026-08-20）；19 项评审修复已合并<br>Every numbered S1–S5 requirement is implemented and test-locked (2026-08-20); the 19 review fixes are merged |
| 三表模型的真实 GL / 试算平衡表联调<br>Live GL / trial-balance integration | ⚠️ 未完成。引擎与四个生产适配器已接线，但只跑过构造数据；端口缺数据时诚实降级为缺口，不产出数字<br>Not done. The engine and its four production adapters are wired, but only exercised against constructed data; a port with no data degrades to an explicit gap rather than a number |
| Python `ai-service` 退役（ADR-0023/0024）<br>Retiring the Python AI service | ✅ **已退役并删除（W6，2026-08-20）**。LLM/解析/上传/planner 全部迁入 Go（`internal/llm`、`internal/docparse`、`internal/aiintake` 生产侧、`miniostore`、`agentrunner.PlannerLLM`）；AGPL 版 PDF 依赖已随 ai-service 一起删除<br>**Retired and removed.** All LLM / parsing / upload / planner paths now run in Go; the `pymupdf` AGPL dependency is gone with the ai-service directory |

**关于命名 / On naming:** 仓库、容器、数据库和 JWT 仍使用 `lease_*` 命名。产品定位已经调整，但底层大规模物理重命名要等内部技术门槛通过后再决定 —— 2026-05 已经改过一次名，不再频繁变更。
The repository, containers, database and JWT still use the `lease_*` namespace. Positioning has moved; a second large physical rename is deliberately deferred until the internal validation gate is passed (the project was already renamed once in 2026-05).

---

## 产品功能 / Product Capabilities

### 一、零售经营分析 / Retail Operations Analytics

转型新增的核心能力。数据粒度为 **store-day（门店 × 日）**，指标语义版本为 `retail-kpi-v1`。
The capabilities added by the transformation. Grain is **store-day**; the metric semantics are versioned as `retail-kpi-v1`.

| 功能 / Feature | 说明 / Description |
|---|---|
| **经营脉搏**<br>Operating Pulse<br>`/operating-pulse` | 两分钟晨检：六张核心 KPI 卡（销售、毛利、交易数、客流、转化、客单价）、每日趋势、辅助指标、按影响排序的优先关注门店榜，以及数据可信条（覆盖率、来源系统、公式版本、事实版本、decision-ready 判定）。<br>A two-minute morning check: six core KPI cards (revenue, gross profit, transactions, footfall, conversion, average transaction value), a daily trend, auxiliary metrics, an impact-ranked attention list, and a data-trust strip (coverage, source systems, formula version, fact version, decision-ready verdict). |
| **门店 360 与异常下钻**<br>Store 360 & drill-down<br>`/store-360` | 单店趋势、同群对比（同 brand + region + currency 分位）、销售 / 毛利 / 门店贡献三组确定性变化桥，以及每个结论背后的证据链。同群样本不足或币种混杂时明确降级，不编造对比。<br>Single-store trends, peer-cohort benchmarking (same brand + region + currency percentiles), three deterministic change bridges (sales, gross profit, store contribution), and the evidence chain behind each conclusion. Degrades explicitly when the peer sample is too small or currencies are mixed — it never fabricates a comparison. |
| **情景工作台**<br>Scenario Workbench<br>`/scenario-workbench` | 基于同一批事实生成透明的 30 日 run-rate，对销售、毛利率、人工、固定租金、变量租金率、非租赁成本和其他成本七个杠杆做 What-if；输出守恒的贡献变化桥，并可生成待确认的 open action。结果过期或数据不足时会阻断保存。<br>Builds a transparent 30-day run-rate from the same facts and runs what-if on seven levers (sales, gross-margin rate, labour, fixed rent, variable-rent rate, non-lease cost, other cost). Produces a conservation-checked contribution bridge and can raise an open action for confirmation. Saving is blocked when the result is stale or the data is insufficient. |
| **AI 经营分析 Agent**<br>AI operations analyst<br>`/ai-chat` | `retail_operations@v1` Skill，只允许调用经营脉搏、门店诊断和确定性情景三个**只读** Tool。回答带引用来源、口径说明、置信度和建议下一步；行动建议以 Artifact 形式产出，**不直接写入任何业务表**。权限拒绝保持 `scope_denied` 原因，不被改写成「无数据」。<br>The `retail_operations@v1` skill may call exactly three **read-only** tools: operating pulse, store diagnostics and deterministic scenario. Answers carry citations, metric definitions, a confidence value and a suggested next step. Action suggestions are emitted as artifacts and **never written straight into business tables**. A permission denial keeps its `scope_denied` reason rather than being masked as "no data". |
| **零售 KPI 语义层**<br>Retail KPI semantic layer<br>`retail-kpi-v1` | 统一的日粒度指标定义与中文数据字典，配 Golden 对数。严格的 null / 零分母语义、覆盖率门槛、最高事实版本选择、来源冲突保护（409）和多币种分区。指标由后端计算，前端不重算评分。<br>One day-grain metric definition set with a data dictionary and golden datasets. Strict null / zero-denominator semantics, coverage thresholds, highest-fact-version selection, source-conflict protection (HTTP 409) and multi-currency partitioning. Metrics are computed server-side; the frontend never re-scores. |
| **固定 seed 模拟数据生成器**<br>Fixed-seed simulation generator | 一键生成可重复复演的 60 店 / 181 天数据集，内含六类固定经营异常。模拟数据在数据库、API、导出和 UI 中全程带标识，**永不进入 Official 过账链路**。<br>Generates a reproducible 60-store / 181-day dataset containing six fixed anomaly types. Simulated data is labelled end-to-end in the database, API, exports and UI, and **can never enter the Official posting chain**. |
| **Store-day 事实底座**<br>Store-day fact foundation | production / simulated / mixed 来源信封、批次、版本、as-of 与请求幂等，支持安全导入、重复提交保护和按法人读取。<br>A production / simulated / mixed source envelope with batches, versions, as-of stamps and request idempotency — enabling safe import, duplicate-submission protection and per-legal-entity reads. |

### 二、租赁台账管理 / Lease Administration

| 功能 / Feature | 说明 / Description |
|---|---|
| 集中合同库<br>Central contract ledger | 合同基础信息、门店 / 资产、出租方、承租方、标签、状态、余额与当前生效租金。<br>Contract master data, store/asset, lessor, lessee, tags, status, balances and the rent actually in force. |
| 附件文档<br>Document management | 主合同、补充协议、side letter 等文档元数据。<br>Metadata for master agreements, amendments, side letters and similar documents. |
| 关键日期提醒<br>Critical-date alerts | 续租截止、break notice、租金 review、到期日、保险续保。<br>Renewal deadlines, break notices, rent reviews, expiry and insurance renewal. |
| 条款 / 义务管理<br>Clause & obligation tracking | 维修、CAM、保险、指数调整、恢复义务、押金、通知义务。<br>Repair, CAM, insurance, index adjustment, make-good, deposits and notification obligations. |
| 组合分析<br>Portfolio analysis | 按资产类型、区域 / 品牌、租赁范围查看合同组合、承诺租金与到期分布。<br>Contract mix, committed rent and expiry distribution by asset type, region/brand and lease scope. |

### 三、IFRS 16 租赁会计 / IFRS 16 Lease Accounting

| 功能 / Feature | 说明 / Description |
|---|---|
| 初始计量<br>Initial measurement | 租赁负债现值、使用权资产、初始直接成本、激励、恢复成本。<br>Lease-liability present value, right-of-use asset, initial direct costs, incentives and restoration cost. |
| 后续计量<br>Subsequent measurement | 利息摊销、折旧、付款冲减、负债滚动。<br>Interest accretion, depreciation, payment offset and liability roll-forward. |
| 范围闸门<br>Scope gate | `in_scope` 资本化；`short_term_exempt` / `low_value_exempt` 直线法费用化；`not_a_lease` 跳过资本化。<br>`in_scope` capitalises; `short_term_exempt` / `low_value_exempt` expense straight-line; `not_a_lease` skips capitalisation. |
| 会计区分<br>Accounting distinctions | 先付 / 后付租金、变量租金费用化、非租赁成分费用化。<br>Rent in advance vs in arrears, variable rent expensed, non-lease components expensed. |
| 事件驱动重算<br>Event-driven remeasurement | modification、reassessment、impairment 等事件批准后触发重算。<br>Modification, reassessment and impairment events trigger remeasurement once approved. |
| 月结闭环<br>Month-end close | 生成计量结果与分录，复核、审批、过账、锁账 / 解锁。<br>Generate measurements and journal entries, then review, approve, post, lock and unlock. |
| 双口径并列<br>Dual basis | 财务报告口径（ROU / 负债 / 利息 / 折旧）与经营占用口径（基本租金 + 服务费 + 当期变量租金）分开维护，互不替代。<br>The financial-reporting basis (ROU, liability, interest, depreciation) and the operating-occupancy basis (base rent + service charge + in-period variable rent) are maintained side by side and never substituted for each other. |

### 四、经营决策与 FP&A / Decision Support & FP&A

| 功能 / Feature | 说明 / Description |
|---|---|
| 经营驾驶舱 `/performance`<br>Performance cockpit | 门店四墙利润、租售比、效率指标与 cohort 视角。<br>Four-wall profit, rent-to-sales ratio, efficiency metrics and cohort views. |
| 预算 / 实际差异 `/reports`<br>Budget vs actual variance | 版本化计划、冻结、比较、准确率，及六类差异归因。<br>Versioned plans, freezing, comparison, accuracy tracking and six-category variance attribution. |
| 开店前测算 `/pre-deal`<br>Pre-deal appraisal | 新店经济性测算与门槛判定。<br>Store-level economics and hurdle assessment for a prospective site. |
| 方案比价 `/deal-compare`<br>Deal comparison | 多份报价 / 条款方案的有效租金与经营影响比较。<br>Effective-rent and operating-impact comparison across competing offers or term sheets. |
| 现金流预测 `/cashflow-forecast`<br>Cash-flow forecast | 租赁现金流出预测与情景。<br>Lease cash-outflow forecasting and scenarios. |
| 敏感性分析 `/sensitivity`<br>Sensitivity analysis | 折现率冲击对负债和 ROU 的影响。<br>Discount-rate shocks against liability and ROU. |
| 多准则对比 `/standards`<br>Multi-standard comparison | IFRS 16 / ASC 842 / 本地准则的管理视角差异。<br>Management-view differences across IFRS 16, ASC 842 and local GAAP. |
| ROI 测算 `/roi`<br>ROI calculator | 传统 Excel 工时与系统处理工时的差异测算。<br>Estimated effort difference between spreadsheet workflows and the system. |

### 五、三表财务模型与单店利润表 / Three-Statement Model & Store P&L

财务经理视角的建模能力：单店层看四墙损益，法人层看 IS / BS / CF 联动。规格见 [PRD](docs/PRD_三表财务模型与单店利润表.md)。
The finance-manager layer: four-wall economics per store, and a linked income statement / balance sheet / cash-flow model per legal entity.

| 功能 / Feature | 说明 / Description |
|---|---|
| **单店利润表**<br>Store P&L<br>`/store-pnl` | 日 / 周 / 月 / 季 / 年期间视图；Actual 与第二列（预算 / 预测 / 去年同期）并排；经营口径与 IFRS 16 口径 side-by-side 且块级带 basis 标签，禁止混算；三级下钻（金额 → 构成 → 合同级占用成本拆分 → 来源事实信封）；同群对比列；多店按区域 / 品牌 / 法人汇总，混币种分区呈现且**不做任何跨币种合计**；CSV 与带活公式的 XLSX 导出。<br>Day/week/month/quarter/year views; Actual beside budget, forecast or prior year; operating and IFRS 16 bases shown side by side with block-level basis labels and no mixing; three-level drill-down down to per-contract occupancy split and the source envelope behind each fact; a peer-cohort column; multi-store roll-up by region/brand/entity with currency partitioning and never a cross-currency total; CSV and live-formula XLSX export. |
| **三表模型工作台**<br>Statement model workbench<br>`/financial-model` | 法人级 IS/BS/CF 联动模型。冻结线左侧读事实、右侧跑驱动公式（SSSG × 新店爬坡 × 门店增减）；输入带五条版本线与溯源；期初三道闸（自平衡、归并口径跨期一致、租赁余额对计量引擎勾稽）不过则拒绝运行；**T1–T16 勾稽全绿才允许发布**；同步与异步（进度、取消、幂等重放）两种运行方式；月 / 季 / 年折叠的活公式导出。<br>A linked IS/BS/CF model per legal entity. Facts left of the freeze line, driver formulas right of it. Inputs carry five version lines and provenance. Three opening-balance gates must pass or the run is refused. Publication requires all sixteen tie-outs green. Runs synchronously or asynchronously with progress, cancellation and idempotent replay. Exports fold to month, quarter or year as a live-formula workbook. |
| **受治理的模板与视图**<br>Governed templates & saved views | 白名单公式 DSL（字面量、循环引用、跨法人、basis 混行一律在 Parse 期拒绝）；模板版本冻结、复核批准、复制谱系、共享 / 个人可见性；保存视图带 fail-closed 配置 lint；**未登记的自定义公式行在 JSON、XLSX 与页面三处都带「未经指标治理」标识**。<br>A whitelist formula DSL that rejects literals, cycles, cross-entity references and mixed-basis rows at parse time; versioned, frozen, reviewed and approved templates with copy lineage and shared/personal visibility; saved views behind a fail-closed config lint; and ungoverned custom formula rows marked as such in JSON, XLSX and the interface alike. |
| **AI 填数与差异备忘录**<br>AI assumptions & variance memos | `fpna.*` 六个 Tool：读模型、跑试算、生成底稿、建议假设（单条与批量）、起草四层差异备忘录。**写类工具一律只写 draft 层**，`source=ai_suggestion`，approved-only 的读取路径永不回采 draft；无法建议的项目如实留空而不是编造。<br>Six `fpna.*` tools: read a model, evaluate it, generate a working paper, suggest assumptions (single and batch), and draft a four-layer variance memo. Every writing tool writes only to the draft layer, tagged `source=ai_suggestion`; approved-only reads never pick drafts back up, and an item the model cannot justify is left empty rather than invented. |
| **集团合并视图**<br>Group view | 按授权集合汇总，未授权成员显式呈现而非静默省略；原币分区为默认视图，折算视图必须指定汇率版本与类型（缺汇率整体降级）；固定声明「未做内部交易抵销」。<br>Aggregates the authorised set and shows unauthorised members explicitly instead of silently dropping them; native-currency partitions are the default view and a translated view demands an explicit rate version and type, degrading wholesale when a rate is missing; carries a standing note that intercompany eliminations are not performed. |

### 六、AI 录入与 Agent 运行时 / AI Intake & Agent Runtime

| 功能 / Feature | 说明 / Description |
|---|---|
| AI 录入<br>AI intake | `/ai-chat` 上传合同或台账文件，LLM 抽取字段并生成结构化草稿卡片。解析统一走 Go 侧 `internal/docparse`（CSV / anydoc / PaddleOCR，按是否需要证据坐标分流，惰性 OCR）+ `internal/aiintake` 生产侧抽取归一化；`ai-service` 已退役（W6）。<br>Upload contracts or ledgers at `/ai-chat` for LLM field extraction into structured draft cards. Parsing runs in Go through `internal/docparse` (CSV / anydoc / PaddleOCR, routed by evidence-coordinate need, lazy OCR) and the `internal/aiintake` producer; the Python `ai-service` is retired (W6). |
| 文件分诊<br>File triage | `lease.file.triage` 做确定性分诊；域外文件（发票、劳动合同、宣传册）**显式拒绝并回问用户**，没有「猜成合同」的兜底。<br>`lease.file.triage` classifies deterministically. Out-of-domain files (invoices, employment contracts, brochures) are refused explicitly and referred back to the user; there is no silent "assume it's a contract" fallback. |
| 底稿产出<br>Working papers | `workingpaper` 按单元格级 provenance 渲染 xlsx / docx，经 fail-closed lint 才能导出：Certified 单元格必须挂已完成审计的 Tool 调用，勾稽未全绿的 run 不产出底稿，缺失值跳格而不填 0。<br>`workingpaper` renders xlsx/docx with cell-level provenance behind a fail-closed lint: a certified cell must reference a completed, audited tool call, a run with unpassed tie-outs yields no paper at all, and a missing value leaves the cell empty rather than writing zero. |
| Human-in-the-loop | AI 草稿必须人工确认才能创建合同草稿，正式入库仍走审批。AI 不得猜测折现率，缺失时标记 `discount_rate_missing`。<br>An AI draft only becomes a contract draft after human confirmation, and formal entry still goes through approval. The AI may not guess a discount rate; a missing one is flagged `discount_rate_missing`. |
| Agent Tool Runtime | Web、CLI 与 Pi-like Runner 共用 Tool Registry、Scope Guard、Review Gate、幂等与审计接缝。<br>Web, CLI and the Pi-like runner share one tool registry, scope guard, review gate, idempotency layer and audit seam. |
| Skill Registry | 合同台账、合同复核、租金表、审计包与 `retail_operations@v1` 均以版本化描述注册，并按角色过滤。<br>Contract ledger, contract review, rent schedule, audit pack and `retail_operations@v1` are all registered as versioned descriptors and filtered by role. |
| `lease-agent` CLI | Skill / Tool discovery、合同搜索与只读查询、Draft 命令、Capability 签发与撤销、Run Trace、worker lease 和机器可读退出码。<br>Skill/tool discovery, contract search and read-only queries, draft commands, capability issue/revoke, run traces, worker leases and machine-readable exit codes. |
| `agent-runner` | 独立 Worker 进程，只通过 Agent Gateway 执行受控 Tool，不挂载数据库或 MinIO 凭证；Run 按 `worker_id + lease_token` 绑定。<br>A standalone worker that executes controlled tools only through the Agent Gateway, with no database or MinIO credentials mounted; runs are bound by `worker_id + lease_token`. |
| Agent 可观测性 `/agent-metrics`<br>Agent observability | JSON 与 Prometheus 指标、跨 Run Planner 用量、Token 与成本状态；价格未配置时明确标记 unavailable，不臆造成本。<br>JSON and Prometheus metrics, cross-run planner usage, token and cost status. When pricing is not configured, cost is explicitly marked unavailable rather than invented. |

### 七、平台与治理 / Platform & Governance

| 功能 / Feature | 说明 / Description |
|---|---|
| 多租户与数据范围<br>Multi-tenancy & data scope | 法人（`legal_entity_id`）行级过滤，扩展到区域 / 品牌 / 门店范围。<br>Row-level filtering by legal entity, extended to region / brand / store scope. |
| 认证与会话<br>Auth & sessions | JWT access / refresh，refresh session 哈希持久化、一次性轮换、设备会话列表与撤销、过期自动清理。<br>JWT access/refresh with hashed refresh sessions, single-use rotation, device session listing and revocation, and automatic expiry cleanup. |
| Working / Official 双模式<br>Dual reporting mode | Working 可含草稿与待审批数据；Official 只含已审批正式数据。<br>Working may contain drafts and pending items; Official contains only approved, formal data. |
| ERP / 总账集成<br>ERP & GL integration | 会计分录 CSV 导出与 ERP 凭证号回写。<br>Journal-entry CSV export and ERP voucher-number write-back. |
| 审计留痕 `/audit-logs`<br>Audit trail | 合同、事件、审批、月结、锁账等关键动作全链路留痕。<br>End-to-end trail across contracts, events, approvals, month-end close and ledger locking. |

---

## 五条不可突破的底线 / Five Non-Negotiable Boundaries

零售转型期间，任何改动都不得降低以下五条。它们在 MAX-001 → MAX-009 每一轮评审中被独立验证。

No change made during the retail transformation may weaken these five. Each was independently verified in every MAX-001 → MAX-009 review round.

| # | 底线 / Boundary | 含义 / What it means |
|---|---|---|
| 1 | 跨法人隔离<br>Cross-entity isolation | 法人 A 的账号无法读取或写入法人 B 的门店、事实、行动、Run 或 Artifact。<br>An account in entity A can neither read nor write entity B's stores, facts, actions, runs or artifacts. |
| 2 | 模拟 / 正式数据区分<br>Simulated vs production | 模拟数据在库、API、导出、UI 全程带标识，且永不进入 Official。<br>Simulated data is labelled everywhere and can never reach Official. |
| 3 | 来源追溯<br>Source provenance | 每条事实都可追溯到来源系统、批次、版本与 as-of 时点。<br>Every fact traces back to a source system, batch, version and as-of timestamp. |
| 4 | 重复导入保护<br>Duplicate-import protection | 请求级与业务级幂等，重放不产生第二条记录。<br>Request-level and business-level idempotency; a replay never creates a second record. |
| 5 | IFRS 16 正式台账隔离<br>Official ledger isolation | 经营分析与 AI 路径对 IFRS 16 正式表零写入。<br>The analytics and AI paths perform zero writes against the IFRS 16 official tables. |

---

## 技术栈 / Tech Stack

| 层 / Layer | 技术 / Technology |
|---|---|
| 前端 / Frontend | Next.js 14 + TypeScript + Ant Design + Recharts |
| 核心后端 / Core backend | Go 1.25 + Gin |
| 数据访问 / Data access | pgx（手写 SQL / hand-written SQL） |
| 数据库 / Database | PostgreSQL 16 |
| AI 服务 / AI service | ~~Python 3.11 + FastAPI~~（**已退役并删除 / retired and removed**，ADR-0023/0024，W6）；模型调用 `internal/llm`，文档解析 `internal/docparse` + `internal/aiintake` |
| OCR / 文档结构化 | PaddleOCR-VL-1.5（AI Studio 异步 API）+ anydoc（`internal/docparse`，ADR-0024 分流）；AGPL 版 PDF 依赖已删除 |
| 报表产出 / Report output | excelize（xlsx，含活公式）+ 确定性 docx 渲染器 |
| 大模型 / LLM | DeepSeek API（默认 / default）、OpenAI API（备用 / fallback） |
| 对象存储 / Object storage | MinIO |
| 认证授权 / Auth | 自建 JWT + RBAC + 多租户行级过滤；Agent Gateway 支持短时效、Run 绑定的 Capability Token |
| 部署 / Deployment | Docker Compose |

---

## 项目结构 / Repository Layout

```text
<repo>/                              # GitHub: retail_performance_workstation
├── db/
│   ├── init/                      # PostgreSQL 首次初始化 schema / first-run schema
│   └── migrations/                # 增量迁移 SQL / incremental migrations
├── core-service/                  # Go + Gin 核心 API
│   ├── cmd/api/                   # API 服务入口
│   ├── cmd/ifrs16-regression/     # IFRS 16 回归报告生成命令
│   ├── cmd/lease-agent/           # 只调用 Agent Gateway 的 CLI Adapter
│   ├── cmd/agent-runner/          # Pi-like Worker 进程入口（无 DB/MinIO 凭证）
│   └── internal/
│       ├── agentcore/             # 纯 Go Agent 循环内核（ADR-0022）
│       ├── agentartifact/         # Artifact / Evidence 协议
│       ├── agentskill/            # Skill Registry
│       ├── agenttools/            # Tool Registry / Runtime / Policy（lease.* / retail.* / fpna.*）
│       ├── agentseval/            # L1 评测 harness 与不变量用例
│       ├── docparse/              # CSV / anydoc / PaddleOCR 解析层
│       ├── workingpaper/          # 单元格级 provenance、fail-closed lint、xlsx/docx 渲染
│       ├── finmodel/              # 三表模型：纯函数引擎 + 勾稽 T1–T16
│       │   ├── template/          #   模板值对象与白名单公式 DSL
│       │   ├── opening/           #   期初三道闸
│       │   ├── persist/           #   唯一写入口、发布谱系、勾稽落队列
│       │   ├── adapter/           #   四个生产端口（事实 / 计量 / 付款计划 / TB）
│       │   ├── suggestion/        #   AI 假设草稿（draft-only）
│       │   └── memo/              #   四层差异备忘录
│       ├── storepnl/              # 单店利润表投影、占用成本拆分、活公式 xlsx
│       ├── handlers/              # HTTP handlers（含 retail_* / finmodel / store_pnl）
│       ├── middleware/            # JWT、tenant、CORS
│       ├── repository/            # pgx 数据访问层
│       └── services/
│           ├── retailkpi/         # retail-kpi-v1 指标语义层
│           ├── retailpulse/       # 经营脉搏聚合
│           ├── retailstore360/    # 门店 360 与同群对比
│           ├── retailscenario/    # 确定性情景计算
│           ├── retailsimulation/  # 固定 seed 模拟数据生成器
│           └── ifrs16/            # IFRS 16 计量
# ai-service/ 已删除（W6）：解析、LLM、上传全部迁入 core-service（ADR-0023/0024）
├── contracts/ai-intake.v1/        # 抽取契约 JSON Schema（实现方可换，契约不变）
├── web/                           # Next.js 前端
│   └── app/
│       ├── operating-pulse/       # 经营脉搏 / Operating Pulse
│       ├── store-360/             # 门店 360 / Store 360
│       ├── scenario-workbench/    # 情景工作台 / Scenario Workbench
│       ├── store-pnl/             # 单店利润表 / Store P&L
│       ├── financial-model/       # 三表模型工作台 / Statement model workbench
│       ├── fpna-workbench/        # FP&A 版本与差异工作台
│       ├── ai-chat/               # AI 录入与经营问答
│       ├── contracts/             # 合同台账与详情
│       ├── monthly-closing/       # 月结跑批
│       ├── reports/               # 报表
│       ├── performance/           # 经营驾驶舱
│       ├── portfolio/             # 组合分析
│       ├── sensitivity/           # 敏感性分析
│       └── standards/             # 多准则对比
├── docs/                          # 需求、架构、转型规划、执行看板、验收证据
├── ops/                           # Prometheus 规则与采集模板
├── scripts/
├── docker-compose.yml
├── Makefile
└── AGENTS.md
```

---

## 快速开始 / Getting Started

### 1. 准备环境 / Prerequisites

Docker / Docker Compose、Make、Go 1.25+、Node.js 20+（Python 已随 ai-service 退役移出）

### 2. 配置环境变量 / Configure environment

```bash
make setup
```

编辑 `.env`，重点配置 / Then edit `.env`, in particular:

- `DEEPSEEK_API_KEY`
- `OPENAI_API_KEY`（备用 / fallback）
- `PADDLEOCR_ACCESS_TOKEN`（启用 PaddleOCR 时 / when PaddleOCR is enabled）

### 3. 启动服务 / Start services

```bash
docker compose up -d --build
```

如需启动 Pi-like Worker，先把一个短时效、受限的 Gateway JWT 放入当前 shell，再启动 `worker` profile。
To run the Pi-like worker, put a short-lived, scoped Gateway JWT into your shell first, then start the `worker` profile:

```bash
export AGENT_CORE_URL="${AGENT_CORE_URL:-http://localhost:8080}"
export AGENT_GATEWAY_TOKEN="$(curl -fsS -X POST "$AGENT_CORE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"<worker-user>","password":"<password>"}' | jq -r '.token')"
docker compose --profile worker up -d --build agent-runner
```

本地如遇 8080 / 8081 被占用，可在 `.env` 改 `CORE_PORT` / `AI_PORT`；容器内部地址仍固定为 Core `8080`、AI `8000`。**不要把 Worker JWT 写入 Git 或 `.env.example`。**
If 8080 / 8081 are taken locally, change `CORE_PORT` / `AI_PORT` in `.env`; in-container addresses stay at Core `8080` and AI `8000`. **Never commit a worker JWT to Git or `.env.example`.**

服务地址 / Service endpoints:

| 服务 / Service | 地址 / Address |
|------|------|
| Web | http://localhost:3000 |
| Core Service | http://localhost:8080 |
| AI Service | http://localhost:8081 |
| MinIO Console | http://localhost:9001 |
| PostgreSQL | localhost:5432 |

### 4. 测试账号 / Test accounts

> ⚠️ **仓库不种任何用户**：`db/init/01_init.sql`、`db/migrations/` 和 `scripts/` 中都没有创建用户的语句。下表列的是既有开发环境里手工建出来的账号，**新环境不会有**。公开注册（`POST /api/v1/auth/register`）保持关闭。

**全新环境引导 / First-admin bootstrap**

`make reset-db` 或 `docker compose down -v` 重置数据库后，用 `bootstrap-admin` 创建首个管理员（唯一的引导入口；凭据只从环境变量读取，不会写入仓库）：

```bash
# 本地编译运行
cd core-service
BOOTSTRAP_ADMIN_USERNAME=admin_user \
BOOTSTRAP_ADMIN_EMAIL=admin@example.com \
BOOTSTRAP_ADMIN_PASSWORD=<你的强密码> \
GOCACHE=$(pwd)/.gocache go run ./cmd/bootstrap-admin

# 或在容器内运行（镜像已包含该二进制）
docker compose up -d --build core-service
docker compose exec core-service env \
  BOOTSTRAP_ADMIN_USERNAME=admin_user \
  BOOTSTRAP_ADMIN_EMAIL=admin@example.com \
  BOOTSTRAP_ADMIN_PASSWORD=<你的强密码> \
  ./bootstrap-admin
```

退出码 / Exit codes：`0` 成功；`2` 缺少必需环境变量（未触库）；`3` 系统已存在用户、拒绝执行（可安全重复运行，不会提权）；`4` 数据库错误。

> The repository seeds no users. After `make reset-db` or `docker compose down -v`, run `bootstrap-admin` (environment-variable credentials only) to create the first administrator; public registration stays disabled.

| 用户名 / Username | 密码 / Password | 角色 / Role | 说明 / Notes |
|--------|------|------|------|
| `admin_user` | `password123` | admin | 管理员，跨租户 / cross-tenant admin（历史环境账号） |
| `testuser` | `password123` | user | 普通测试用户 / general test user（历史环境账号） |

需要按法人隔离测试时，用管理员通过 `POST /api/v1/admin/users` 新建带 `legal_entity_id` 的用户（该字段对非管理员角色现为必填）。
To test legal-entity isolation, create a user with a `legal_entity_id` through `POST /api/v1/admin/users` as an admin; that field is now required for non-admin roles.

### 5. 生成演示数据并跑一次经营晨检 / Generate demo data and run a morning check

以管理员登录后打开 `/operating-pulse`。当前法人若还没有模拟数据集，页面会提示「生成固定演示数据」，一键生成 60 店 / 181 天、含六类固定异常的可复演数据集，然后按 **经营脉搏 → 门店 360 → 情景工作台 → 行动草稿** 走完整条链路。

Sign in as an admin and open `/operating-pulse`. If the legal entity has no simulated dataset yet, the page offers **生成固定演示数据**, which creates a reproducible 60-store / 181-day dataset containing six fixed anomalies. Then walk the full chain: **Operating Pulse → Store 360 → Scenario Workbench → action draft**.

零售 MVP（MAX-001 → MAX-009）的演示脚本、发布检查清单与端到端验收证据已随交付完成删除 —— 结论见上方「当前状态」，原文在 git 历史里。
The retail MVP's demo script, release checklist and end-to-end acceptance evidence were deleted once delivered; the conclusions live in Current Status above and the originals in git history.

### 6. 数据库迁移 / Database migrations

`db/init/01_init.sql` 只在 PostgreSQL volume 首次为空时执行。已有旧 volume 时需要手动跑增量迁移。
`db/init/01_init.sql` runs only when the PostgreSQL volume is empty. Existing volumes need the incremental migrations applied by hand.

零售转型相关的迁移 / Migrations introduced by the retail transformation:

```bash
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/037_budget_versions_source_backfill.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/038_retail_store_day_facts.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/039_retail_simulation_datasets.sql
docker exec -i lease-postgres psql -U lease -d lease < db/migrations/040_agent_artifact_retail_proposal.sql
```

早期迁移（005–036）见 [`db/migrations/`](db/migrations/)；要清空并按最新 schema 重建：
For the earlier migrations (005–036) see [`db/migrations/`](db/migrations/). To wipe and rebuild on the latest schema:

```bash
make reset-db
make up
```

---

## 关键页面 / Key Screens

| 页面 / Screen | 路径 / Path | 说明 / Description |
|------|------|------|
| 经营脉搏 / Operating Pulse | `/operating-pulse` | 每日经营晨检、KPI、趋势、优先门店与数据可信度 |
| 门店 360 / Store 360 | `/store-360` | 单店趋势、同群对比、驱动拆解与证据链 |
| 情景工作台 / Scenario Workbench | `/scenario-workbench` | 七杠杆 What-if、贡献变化桥与行动草稿 |
| 单店利润表 / Store P&L | `/store-pnl` | 期间视图、双口径并排、三级下钻、同群列、多店汇总与活公式导出 |
| 三表模型工作台 / Statement model | `/financial-model` | 法人级 IS/BS/CF 联动、期初三道闸、T1–T16 勾稽门禁、异步 Run 与折叠导出 |
| FP&A 工作台 / FP&A workbench | `/fpna-workbench` | 计划版本、冻结与差异归因 |
| 经营驾驶舱 / Performance | `/performance` | 四墙利润、租售比与效率指标 |
| AI 助手 / AI assistant | `/ai-chat` | 文件录入、草稿确认、经营问答与行动建议 |
| 待办 / To-do | `/todo` | 关键日期、审批与异常待办 |
| 合同台账 / Contracts | `/contracts` | 合同列表、搜索、筛选、排序、余额与在租金额 |
| 月结跑批 / Month-end close | `/monthly-closing` | 生成、审批、过账、锁账、ERP 导出与回写 |
| 报表 / Reports | `/reports` | Working / Official 报表、披露报表包、预算差异与 CSV 导出 |
| 组合分析 / Portfolio | `/portfolio` | 资产类型、范围、租金承诺与到期分布 |
| 开店前测算 / Pre-deal | `/pre-deal` | 新店经济性测算 |
| 方案比价 / Deal compare | `/deal-compare` | 多方案有效租金比较 |
| 现金流预测 / Cash-flow forecast | `/cashflow-forecast` | 租赁现金流出预测 |
| 敏感性分析 / Sensitivity | `/sensitivity` | 折现率冲击分析 |
| 多准则对比 / Standards | `/standards` | IFRS 16 / ASC 842 / 本地准则对比 |
| ROI 测算 / ROI | `/roi` | 人力工时与成本节省测算 |
| Agent 运营 / Agent metrics | `/agent-metrics` | Planner 用量、Token 与成本（管理员 / 审计） |
| 审计日志 / Audit logs | `/audit-logs` | 全链路操作留痕查询 |

---

## 验证命令 / Verification

```bash
cd core-service
GOCACHE=$(pwd)/.gocache go test ./...
go vet ./...

cd ../web
npm run type-check
npm run build
npm test

cd ..
make ifrs16-regression
```

真实 PostgreSQL 集成测试需要设置 `TEST_DATABASE_URL`，未设置时相关用例会正常 skip。
The real-PostgreSQL integration tests require `TEST_DATABASE_URL`; without it those cases skip cleanly.

常用命令 / Common commands:

```bash
make help                 # 查看命令 / list commands
make up                   # 启动服务 / start
make down                 # 停止服务 / stop
make restart              # 重启 / restart
make logs                 # 日志 / logs
make db                   # 进入 PostgreSQL
make ifrs16-regression    # IFRS 16 回归 / regression report
```

---

## 开发约束 / Development Constraints

中文：

- 转型采用增量叠加：不得删除、重命名、隐藏或破坏任何既有功能、页面、导航入口或 API。
- 所有重大变更必须通过事件表处理，不得手工覆盖合同金额或日期。
- AI 识别结果不得直接写入正式台账，必须进入草稿层并人工确认。
- AI 不得猜测折现率，缺失时必须触发 human-in-the-loop。
- 先付租金、后付租金、变量租金、非租赁成分必须严格区分。
- Working Report 可包含草稿 / 待审批数据；Official Report 仅包含已审批正式数据。
- 模拟数据必须在数据库、API、导出和 UI 中保持标识，不得进入 Official 过账链路。
- 所有数据库变更必须同时提供增量迁移和 `db/init/01_init.sql` 空库初始化版本。
- 经营口径的占用成本不等于 IFRS 16 折旧、利息、ROU 或租赁负债变动，两者不可互相替代。

English:

- The transformation is additive: never delete, rename, hide or break an existing feature, page, navigation entry or API.
- Material changes go through the event tables; contract amounts and dates are never overwritten by hand.
- AI extraction results never land directly in the official ledger — they enter the draft layer and require human confirmation.
- The AI must not guess a discount rate; a missing one triggers human-in-the-loop.
- Rent in advance, rent in arrears, variable rent and non-lease components must stay strictly separated.
- Working reports may include drafts and pending items; official reports contain approved data only.
- Simulated data stays labelled across database, API, exports and UI, and never enters the Official posting chain.
- Every database change ships both an incremental migration and an updated `db/init/01_init.sql`.
- Operating-basis occupancy cost is not IFRS 16 depreciation, interest, ROU or lease-liability movement; the two are never substituted for each other.

---

## 关键文档 / Key Documents

**转型 / Transformation**

- [转型实施任务清单与验收标准](docs/线下零售经营工作站_转型实施任务清单与验收标准.md)
- [精简 MVP 路线与延后治理清单](docs/execution/精简MVP路线与延后治理清单.md) — **HARD-001~012 延后治理项与其重新进入条件**
- [线下零售经营分析工作站 · 外部研究](docs/research/线下零售经营分析工作站_外部研究.md)

**UI / UX**

- [DESIGN.md](DESIGN.md) — 设计系统与前端约束（写前端代码前先读这份）
- [UIUX 改善方案](docs/UIUX改善方案.md) — 当前设计系统诊断、合并排期与分阶段改进计划

**财务模型 / Financial modelling**

- [PRD：三表财务模型与单店利润表](docs/PRD_三表财务模型与单店利润表.md) — 规格基准。附录 B 是勾稽 T1–T16，附录 E 是逐项落地状态
- [CodebaseDesign：三表模型与单店利润表模块深化](docs/CodebaseDesign_三表模型与单店利润表_模块深化.md) — SM1–SM8 模块接口与 D-S1~S9 决策留痕
- [PRD：财务 BP 与 FP&A 岗位支撑补齐方案](docs/PRD_财务BP与FPA岗位支撑补齐方案.md)

**租赁与 IFRS 16 / Lease & IFRS 16**

- [IFRS 16 计量方法与准则映射白皮书](docs/IFRS16_计量方法与准则映射白皮书.md)
- 计量回归对数报告：**不入库**，跑 `make ifrs16-regression` 生成到 `docs/IFRS16_计量回归对数报告.md`（已 gitignore）

**Agent 与运维 / Agent & Operations**

- [AGENTS.md](AGENTS.md)
- [AI 文档索引与现行决策](docs/AI_文档索引与现行决策.md) — **AI 相关文档的入口**，标注每份文档是否仍然有效
- [AI Agent 运行运维手册](docs/AI_Agent_运行运维手册.md)
- [ops/prometheus/README.md](ops/prometheus/README.md)

> 历史文档（已被取代或已完结）统一归档在 `docs/archive/`，均带 ARCHIVED 横幅，**不作为现行依据**。

---

## License

Internal Use Only
