# AGENTS.md — 线下零售经营分析工作站

> 面向线下连锁零售承租方的经营分析工作站。把门店销售、毛利、客流、人工、占用成本与承租合同连接起来，完成「发现问题 — 解释原因 — 模拟方案 — 形成行动」的闭环。IFRS 16 租赁会计是其中一个高价值合规模块，不再是产品全部。

## 本文档的范围

**AGENTS.md 只放「改这个仓库的代码时必须遵守的约束」。** 产品能力清单、快速开始、目录结构、测试账号、部署方式一律不在此重复 —— 那些在 [README.md](README.md)，重复一份只会产生两个互相矛盾的真相。

修改本文档时请守住这条线：**如果一段内容不会改变一个 agent 写代码的方式，它就不属于这里。**

### 文档索引

| 想知道什么 | 看哪份 |
|---|---|
| 产品有什么功能、怎么跑起来、目录结构、测试账号 | [README.md](README.md) |
| **改前端代码必须遵守的设计与样式约束** | **[DESIGN.md](DESIGN.md)** |
| **一个业务词到底是什么意思、该叫什么** | **[CONTEXT.md](CONTEXT.md)** — 领域语言，命名与措辞以它为准 |
| 为什么做这次转型、边界在哪、ICP 与产品蓝图 | [docs/线下零售经营工作站_转型实施任务清单与验收标准.md](docs/线下零售经营工作站_转型实施任务清单与验收标准.md)、[docs/PRD_零售经营分析工作站_BP日常支撑完善.md](docs/PRD_零售经营分析工作站_BP日常支撑完善.md) |
| **三表财务模型与单店利润表的规格与模块设计** | [docs/PRD_三表财务模型与单店利润表.md](docs/PRD_三表财务模型与单店利润表.md)（附录 B 勾稽 T1–T16、附录 E 落地状态）、[docs/CodebaseDesign_三表模型与单店利润表_模块深化.md](docs/CodebaseDesign_三表模型与单店利润表_模块深化.md)（SM1–SM8、D-S1~S9） |
| 进入哪个市场、怎么找到第一批客户 | [docs/GTM_零售经营工作站_中国大陆与香港市场进入策略.md](docs/GTM_零售经营工作站_中国大陆与香港市场进入策略.md) |
| 还没做的延后治理项（HARD-001~012）与其重新进入条件 | [docs/execution/精简MVP路线与延后治理清单.md](docs/execution/精简MVP路线与延后治理清单.md) |
| UI/UX 现状诊断与改进计划 | [docs/UIUX改善方案.md](docs/UIUX改善方案.md) |
| 架构决策记录 | [docs/adr/](docs/adr/) |
| 现行功能规格 | [docs/specs/](docs/specs/) |
| IFRS 16 计量口径（**模块级参考，不是产品定位**） | [docs/IFRS16_计量方法与准则映射白皮书.md](docs/IFRS16_计量方法与准则映射白皮书.md)；回归对数报告不入库，跑 `make ifrs16-regression` 生成 |
| Agent 运行时的运维 | [docs/AI_Agent_运行运维手册.md](docs/AI_Agent_运行运维手册.md) |
| **AI / Agent 的现行设计与决策** | [docs/AI_文档索引与现行决策.md](docs/AI_文档索引与现行决策.md) — **动 AI 相关的东西之前先读这份**，它标着每份文档还算不算数 |

变更历史不在本文维护，用 `git log`。

## 当前工程事实

> 这一节是**易腐事实**。数字变了就改，不要在别处再抄一份。核对命令见本文末「验证」。

- `db/init/01_init.sql`：90 张业务表 + `schema_migrations`（空库基线自动标记全部迁移已应用）；增量迁移到 `059_assumption_draft_idempotency_batch.sql`
- `core-service/internal/`：28 个包。零售经营分析在 `services/retail*`（9 个）；**财务三表模型在 `finmodel/`**（子包 `template` / `opening` / `persist` / `adapter` / `suggestion` / `memo` / `view`），**单店利润表在 `storepnl/`**；Agent 侧为 `agentcore` / `agenttools` / `agentskill` / `agentseval` / `workingpaper` / `docparse` / `pagefill` / `miniostore`
- R 批次（2026-08-23）新增两个纯函数服务包：`services/varianceattribution`（利润差异归因，连环替代）、`services/newstorefeasibility`（新店可行性，纯函数 + Ports，**禁 import `ifrs16`**）；`services/promotionattribution` 增投前保本（同包，与投后共用 `RunRate`）
- `web/app/`：31 个页面。零售主线 `/operating-pulse`、`/store-360`、`/scenario-workbench`；**财务主线 `/store-pnl`（单店利润表）、`/financial-model`（法人级三表模型工作台）**
- Agent Tool：`retail.*` 5 个（含 `retail.working_paper.store.generate`、`retail.store_days.import.preview`）、**`fpna.*` 6 个**（`statement_model.read` / `statement_model.evaluate` / `working_paper.finmodel.generate` / `assumptions.suggest` / `assumptions.suggest_batch` / `memos.model_diff.draft`）。**`lease.*` 的数量没有可靠的静态核对方式**——工具有多种定义写法，三种 grep 口径分别给出 31 / 21 / 19，此前记的「36 个」无从复核。要准确数字请在运行时枚举 `agenttools.Registry`，不要引用一个记在文档里的数
- Docker Compose 默认 4 个服务（PostgreSQL、MinIO、Core、Web），`worker` profile 可另起 `agent-runner`
- Go 1.25（`go.mod` 与两个 Dockerfile 一致）；前端 Next.js 14 + Node 20
- IFRS 16 回归：22 用例 / 148 断言通过，但标准答案仍为 `pending_third_party_review`，未经第三方会计师复核，**不得对外表述为审计背书**
- 零售 MVP 与三表模型均只完成固定 seed / 构造数据的内部验证；**无真实 POS/ERP/GL 联调，无客户验证**，相关结论一律保持 `unvalidated`

**ai-service 已退役（W6，2026-08-20 完成 W4+W5）。** Python 自研已从仓库删除：LLM 走 `internal/llm`、解析走 `internal/docparse` + `internal/aiintake` 生产侧、上传走 `miniostore` 写入侧、planner 走进程内 `agentrunner.PlannerLLM`。ADR-0023/0024 自此为真；CORR-2 基线（门 A + 门 B）已脱离 Python。

**命名现状：** GitHub 仓库已于 2026-08 改名为 `retail_performance_workstation`，**但代码命名空间没有跟着改，也不打算改**：容器、数据库、MinIO bucket、JWT、Go module（`github.com/lease-management-system/core-service`）、CLI 与 Compose 项目名仍是 `lease_*`。

**不要顺手重命名。** 三处尤其危险：

| 位置 | 后果 |
|---|---|
| `docker-compose.yml` 的 `name:` | 改了会**孤立现有 postgres 卷与网络**，等于清库 |
| `core-service/go.mod` 的 module 路径 | 要改全仓每一个 import |
| `docs/archive/**` 里的旧名 | 归档是历史记录，本就该保留当时的事实 |

显示名（`app.title`、Logo、页面文案）与代码命名空间是两件事；2026-05 已经做过一次物理重命名，第二次被刻意推迟到内部验证门槛通过之后。

## 转型是增量叠加

零售经营分析是**叠加**上去的，不是替换。

- **不得删除、重命名、隐藏或破坏**任何既有功能、页面、导航入口、API、合同管理、IFRS 16、月结、报表或既有 Agent 能力
- 新增页面沿用既有 `AppLayout`、栅格与交互范式；视觉与样式约束见 [DESIGN.md](DESIGN.md)，其 §13 止血条款对新代码强制生效
- 所有原路由继续可访问，改动需提供兼容回归证据

## 五条不可突破的底线

任何改动都不得降低这五条，每一条在零售 MVP 的每轮评审中都被独立验证过。

| # | 底线 | 含义 |
|---|---|---|
| 1 | 跨法人隔离 | 法人 A 的账号无法读写法人 B 的门店、事实、行动、Run 或 Artifact |
| 2 | 模拟 / 正式数据区分 | 模拟数据在库、API、导出、UI 全程带标识，**永不进入 Official 过账链路** |
| 3 | 来源追溯 | 每条事实可追溯到来源系统、批次、版本与 as-of 时点 |
| 4 | 重复导入保护 | 请求级与业务级幂等，重放不产生第二条记录 |
| 5 | IFRS 16 正式台账隔离 | 经营分析与 AI 路径对 IFRS 16 正式表**零写入** |

## 架构分层

- **Web** (Next.js + TypeScript + Ant Design + Recharts)：经营脉搏、门店 360、租金谈判测算、单店利润表、三表模型工作台、合同台账、AI 录入、月结、报表、组合分析等 31 个页面
- **Core Service** (Go + Gin)：权限、合同主数据、付款计划、事件、IFRS 16 计量、月结、ERP 导出/回写、审计日志、零售 KPI 语义层与经营分析服务、**三表财务模型引擎与单店利润表投影**、Agent Gateway、Agent Core（`agentcore` 纯循环内核 + 治理中间件链）
- ~~**AI Service** (Python + FastAPI)~~：**按 ADR-0023/0024 已退役并删除（W6）**。文件解析、PaddleOCR、LLM 字段抽取、草稿生成、置信度与原文定位均已由 Go 侧 `internal/docparse` / `internal/aiintake` / `internal/llm` 承担；任何代码不得再新增对已退役 Python 端点的调用
- **PostgreSQL**：正式业务数据、经营事实、AI 草稿、任务状态、审核记录
- **MinIO**：原始上传文件、合同附件、OCR 中间产物、解析结果

**分析引擎边界：** PostgreSQL 承担控制平面、工作流、版本与审计。若进入数百门店 × 日 × SKU 规模，不应继续把分析事实堆进当前宽表；Agent 只调用 Core Tool，**不直接查询原始仓库**。

## 领域核心约束（必须遵守）

### 数据模型

- 租赁侧以 **合同 + 门店/资产 + 付款计划 + 事件** 为核心
- 经营侧以 **经营单元（法人 → 区域 → 门店 → 日期）** 为核心，合同是门店经济性的一类成本来源，不再是所有分析的根对象
- 会计引擎与业务录入分离，业务变化必须通过**事件驱动重算**
- 所有计算结果必须可追溯到输入字段、参数版本和重算批次

### 零售经营分析约束

- **粒度是 store-day。** 月粒度的 `store_operating_facts` 无法支撑日/周经营分析，新增经营指标一律走 store-day 事实
- **口径冲突只降级不换算。** 一个数值在某展示语境下要么可用、要么显示「—」加原因，**没有第三条路**。设备口径的数不得贴零售标题，判定点只有 `web/app/lib/displayBasis.ts` 的 `resolveBasis`。给估算/近似/改名留口子就会有人走——`/performance` 曾用 `oee_pct` 渲染「同群平均坪效达成率」、把 `plant_code` 空值兜底成「核心商圈」，i18n 层六个键被整体零售化改名（连英文一起改）
- **指标露出走受校验清单。** 「哪些指标露出到哪个页面」用 `retailkpi.Surface`，清单里的 code 必须在 `Definitions` 里存在，否则启动即失败。**中文名只有 `retailkpi` 一个真相源**，不得在消费包里再建第二张 labels map
- **不得反推事实。** 缺 `labor_hours` 就是 nil，不许用 `labor_cost ÷ 假定时薪` 倒推。反推会造出类型上是事实、语义上是猜测的值，绕过整套覆盖率门槛与 `decision_ready` 判定
- **连环替代法与「有意义的残差」互斥。** 精确连环替代下各步贡献是望远镜和，求和恒等于总差异、残差恒为浮点噪声；残差只在隔离替代法下非零，而那种方法与顺序无关。选了连环替代，守恒等式就是**构造性质不是检查对象**（风险红线 12），真正的检查是中间值序列逐格锁定 + 顺序敏感性，且 `DecompositionOrder` 必须回显
- **新店测算不得长出第二套租赁计算。** `services/newstorefeasibility` 禁 import `ifrs16`，租赁与 ROU 只经 `LeaseProjectionReader` 从 `measurement_results` 只读投影取；import guard **遍历全部子包**（风险红线 14 的第二个落点，第一个是 `finmodel`）
- **折现率缺失是部分降级不是整体拒绝。** IRR / NPV / 动态回本期返回具名 Gap，静态回本期与盈亏平衡销售额照常返回——后两者不依赖折现率。**任何情况下不得使用默认折现率**
- **前端不复刻后端的 DSL 解析器，一次本地校验也不做**（含括号配对这类「显然安全」的检查）。公式校验一律走 `POST /api/v1/financial-model/templates/validate`，复用 `template.Compile` 同一条路径。开了本地校验的口子它会长大，然后与后端分叉
- **指标语义走 `retail-kpi-v1`。** 严格 null / 零分母语义、覆盖率门槛、最高事实版本选择、来源冲突返回 409、多币种分区。**指标在后端计算，前端不得重算评分或排序**
- **不用 0 填补缺失。** 覆盖不足时降级为 `decision_ready = false` 并说明原因，不得编造数据点
- **同群对比样本不足或币种混杂时必须显式降级**，不得给出看似确定的对比结论
- **经营占用口径 ≠ IFRS 16 口径。** 经营侧算基本租金 + 服务费 + 当期变量租金；IFRS 16 侧算 ROU、负债、利息、折旧。两者并列维护，**不可互相替代**

### 财务三表模型与单店利润表约束（`finmodel/` + `storepnl/`）

规格在 [PRD](docs/PRD_三表财务模型与单店利润表.md)（附录 B = 勾稽 T1–T16），模块接口与 D-S1~S9 在 [模块深化](docs/CodebaseDesign_三表模型与单店利润表_模块深化.md)。以下八条是**已由测试锁定的架构约束**，改代码时不得绕开：

- **`finmodel` 不得 import `ifrs16`。** 模型里不存在第二套租赁计算——租赁附表行 100% 来自 `measurement_results` 的只读投影。守卫是 `finmodel/importguard_test.go`，它遍历 `finmodel/...` 全部子包，不只根包
- **`fin_model_runs` 只有一条写入口。** 同步与异步路径都必须经 `finmodel/persist`；架构测试在 `finmodel/writeguard_test.go`。不要为异步流程再开一条 `CreateModelRun`
- **引擎是纯函数。** `finmodel/engine.go` 不做 IO；一切外部数据经 Ports（FactReader / LeaseRollforwardReader / 付款计划 + capex / TB）注入。**端口未接线时降级为具名 Gap，不 panic、不填 0、不产出数字**
- **Actual 冻结线左侧读事实，右侧才跑驱动公式。** `def.ActualCutoffPeriod` 之前的期间，声明了 `actual_source` 的行从 FactReader 取值；用假设推导 Actual 期毛利会让 T13 在真实数据上必然失败（P0-3 教训）
- **期初有两种失败，方向相反。** `openingAbsent`（未提供期初）→ BS/CF 降级为不可判定、run 继续；`openingRejected`（提供了但三道闸任一不过）→ **Run 报错、不落库**。把后者当成前者等于让带病期初污染后面每一期
- **勾稽必须是两条独立路径的对照，不能恒真。** 若某条按构造必然成立，就把它改写为构造断言并同步修订 PRD 附录 B——不留「假装在检查」的检查。T1–T16 每条都要有反向测试（先证红再证绿）；T13 与期初闸③不符时写 `fpna_data_quality_items`（category=`reconciliation`）
- **模拟标识贯穿到底。** `data_classification` 必须出现在 xlsx / CSV 导出口径头；**simulated / mixed 的 run 不得 publish 为计划版本**（底线 2）
- **底稿对勾稽失败 fail-closed。** tie-out 未全绿的 run 请求底稿 → `workingpaper/lint.go` 的 `tie_out_unpassed` 规则拒绝产出 artifact，不是标红了事

页面侧：`/store-pnl` 与 `/financial-model` 与零售三页同一底座——取数经 `useRetailQuery`，状态呈现经 `classifyDataState`/`StateBlock`，新枚举登记进 `code-lists-contract.test.ts`（CONTRACT-001），容器遵守 DESIGN.md §8.1。**公式行与小计行由后端同一 AST 求值，前端不得重算任何模型行。**

### IFRS 16 模块的关键会计区分（常见错误点）

> 以下三节（会计区分、事件驱动、月结锁账）是 **IFRS 16 合规模块内部**的约束，不是产品主线的描述。写零售经营分析或三表模型代码时它们通常不适用——除非你正在动计量引擎、月结或租赁附表投影。

- **先付 vs 后付租金**：先付租金通常不形成未来融资成本，作为已支付租赁付款额影响使用权资产初始成本；后付租金纳入贴现及后续利息摊销
- **固定 vs 变量租金**：turnover rent / sales-based rent 必须**当期费用化**，不得资本化计入租赁负债
- **租赁 vs 非租赁成分**：CAM、管理费、服务费、清洁费、保安费、维修费、税费需按政策拆分或适用 practical expedient

### 事件驱动架构（禁止手工改表）

- 所有重大变更必须通过**事件表**处理，不得直接修改合同金额或日期
- 事件类型至少包括：新合同录入、新店开业、租赁开始、合同续签、提前终止、面积调整、固定租金变更、指数更新、续租/终止选择权判断变化、lease modification、reassessment、index/rate change、闭店、转租开始/结束、减值触发、恢复成本估计变化
- modification（范围/对价变化）与 reassessment（选择权判断变化导致租期变化）有独立的会计处理逻辑

### 月结与锁账

- 支持按法人、期间、区域、品牌批量跑批
- 分录预览 → 复核 → 审批 → 过账 → 总账回写过账状态
- 严格的锁账与重开期间控制，已关账期间不得覆盖

### AI Agent 约束

- AI 识别结果**不得直接写入正式台账**，必须通过草稿层（合同草稿、事件草稿、付款计划草稿）
- 必须包含字段级置信度、低置信度人工必审、原文定位高亮、差异留痕
- **AI 默认运行在 Assist Mode**：只负责识别、建议、草稿生成，正式入库需人工审批。Auto-Post Mode 需另行定义适用范围与阈值
- AI 问答必须基于权限范围检索，展示引用来源，区分「系统正式数据」与「AI 推断/建议」
- **零售与 FP&A 的 Agent Tool 分两类，边界要守住**：读类（`retail.operating_pulse.read`、`retail.store_diagnostics.read`、`retail.store.scenario.evaluate`、`fpna.statement_model.read/evaluate`）只读不写；写类**一律只写 draft 层**（`fpna.assumptions.suggest[_batch]` 落 `fpna_assumption_versions` 且 `source=ai_suggestion`，`fpna.memos.model_diff.draft` 落 `fpna_decision_memos`）。**approved-only 的读取路径永不回采 draft**——这条有测试锁定
- **底稿类 Tool（`*.working_paper.*.generate`）产出 Artifact，走 LevelDraft + Review Gate**，不落业务表
- **端口未接线时工具必须诚实拒绝**，返回 unavailable，不得产出数字。反过来：**不要用 nil 端口无条件注册工具**，那会让重名注册把真实端口挡在外面（P0-8 教训，`aiagent/agent.go` 现在按 `finModelRepo == nil` 二选一分支注册）
- **权限拒绝必须保持原因。** `scope_denied` 不得被改写成「无数据」之类的软化表述 —— 这会掩盖权限问题，触及底线 1

### 报表双模式

- **Working Report**：可含 Draft / Pending Approval 数据，用于内部试算
- **Official Report**：仅含 Approved 数据，用于正式财务与审计
- 报表需显示 `approval_status`、`is_official_version`、生成时间与版本；导出文件须标注 Draft / Pending Approval / Official

### Discount Rate 人机协同

- AI **不得猜测折现率**，缺失时必须触发 human-in-the-loop
- 处理顺序：检查合同文本 → 系统政策库匹配 → 无法唯一确定则人工确认
- 缺失时标记 `discount_rate_missing = true`，并记录来源、确认人、确认时间

## 角色与权限模型

| 角色 | 代码 | 主要职责 |
|------|------|----------|
| System Admin | `admin` | 用户/角色管理、主数据配置、系统参数 |
| Finance Editor | `editor` | 上传合同、维护草稿、录入台账 |
| Finance Reviewer | `reviewer` | 复核合同/付款计划/事件草稿 |
| Finance Approver | `approver` | 审批正式入库、关键会计处理 |
| Auditor Readonly | `auditor` | 只读查看、导出审计资料 |
| Business Readonly | `readonly` | 查看授权范围内合同和报表 |

**数据权限维度：** 法人 (`legal_entity`)、门店 (`store`)、区域 (`region`)、品牌 (`brand`)。

**审批流程：** `Agent 生成草稿 → Editor 修改确认 → Reviewer 复核 → Approver 审批 → 正式入库`。MVP 阶段 Editor 与 Reviewer 可为同一人；未批准数据可保留计算，但必须区分 Working / Official。

**关键动作日志：** 新增、修改、删除、导入、导出、重算、审批、锁账/解锁。

> 已知缺口：现有六角色无法准确表达区域经理、门店经理、经营分析师等零售角色，需要把「角色」与「数据范围/动作权限」解耦。改动前先看转型可行性报告 §2.3 缺口四。

## 风险红线（设计时必须规避）

1–7 是 **IFRS 16 模块**的红线；8–11 跨全产品；12–14 是**财务三表模型**的红线。

1. **会计政策未先统一**：续租/终止判断标准、CAM 拆分政策、turnover rent 口径、折现率政策、闭店减值触发口径必须在开发前固化
2. **合同数据质量不足**：关键日期缺失、付款计划不完整、先付/后付未明确、非租赁成分未拆分
3. **变更做成手工覆盖**：必须事件驱动，否则历史不可追溯、审计无法复演
4. **忽视先付与后付逻辑差异**：直接导致初始负债、使用权资产、利息摊销表、首期分录错误
5. **忽视变量租金与指数调整**：不应资本化的变量租金被计入负债，需重估的指数租金未更新
6. **接口和锁账控制不足**：台账与总账不一致、付款与负债滚动不一致、已关账期间被覆盖
7. **过度依赖手工调整**：标准场景自动化、高频场景批量化、例外场景可审批可追溯
8. **AI 识别结果直接入正式台账**：必须经草稿层，强制人工确认与审批
9. **OCR 精度不足**：需低置信度标识、原文定位高亮、字段级人工修正、人工转录兜底
10. **AI 问答越权或引用错误**：必须基于权限范围检索、展示引用来源、区分正式数据与 AI 推断
11. **用模拟数据冒充经营结论**：模拟验证不能替代商业验证，不得表述为客户采用、真实 ROI 或 PMF
12. **恒真的勾稽**：拿别名比自己、拿定义式倒减自己、返回 `not_applicable` 的桩，都是「假装在检查」。勾稽必须是两条独立路径的对照，否则改写成构造断言并同步改规格
13. **假设污染 Actual**：Actual 冻结线左侧任何一行用假设推导，都会让真实数据的 run 永远过不了发布门禁
14. **模型里长出第二套租赁计算**：租赁附表只能是计量引擎结果的只读投影；一旦 `finmodel` 自己算 ROU/利息，两个数字必然分叉且无人知道该信哪个

## 文档归档规则

- **`docs/archive/**` 是历史记录，永不作为现行依据。** 里面的方案、计划、评审结论均已被取代或已完结，照着它做事会产出错误的工作。
- **要找现行结论，读 `docs/AI_文档索引与现行决策.md`。** 那份维护着每份文档的状态（Current / Historical / Partially Superseded / Superseded）与现行决策登记。
- **归档是单向的。** 现行文档不得链接进 `docs/archive/`；需要引用归档结论时，把结论重述在现行文档里。豁免：文档索引、`docs/adr/**`（"supersedes X" 正是 ADR 的功能）、archive 内部互引。
- 以上两条由 `scripts/check_docs_archive.sh` 在 CI 强制，不靠人记。

## 文件与数据规范

**合同头**：合同编号、名称、法人主体、门店编号/名称、出租方、资产/物业类别、`asset_type`、币种、签约日期、commencement date、lease start/end date、原始不可撤销期、续租/终止选择权描述及判断结果、折现率类型/版本、`lease_scope`、豁免/排除原因、scope 来源与置信度、合同状态

**付款计划**：付款计划编号、合同编号、生效起止日、覆盖期间起止日、应付日期、实际支付日期、付款时点（先付/后付）、金额/币种/税额、金额类型（固定/指数调整/变量/服务费/押金/税金）、会计属性（租赁/非租赁/变量/税费）、是否进入负债现值计算

**事件**：事件编号、合同编号、事件类型、生效日期、申请/审批日期、原值/新值、变更原因、判断依据、事件状态、重算批次号

**租赁管理**：文档类型、版本号、文件哈希、MinIO object name、关键日期类型、目标日期、提醒天数、负责人、提醒状态、义务类型、责任方、义务状态、原文引用、结构化条款值

**计量结果**：合同编号、会计期间、期初/期末租赁负债、本期新增/利息/付款/重估调整、期初/期末使用权资产、本期新增/折旧/减值/终止处置

**Store-day 经营事实**：法人、门店、日期、币种、营收、毛利、交易数、客流、人工成本、固定租金、变量租金、非租赁成本、其他成本、面积；来源信封按 038/048/052 实际列名写：`data_classification`（production/simulated/mixed）、`source_system`、`import_batch_id`、`as_of_at`、`version`（= fact version）、`simulation_dataset_version`

**三表模型 Run**：模型定义编号（法人 + 期间范围 + `actual_cutoff_period`）、模板版本、假设版本、政策快照（循环引用政策、利息列报政策、计息方法）、状态（`queued`/`running`/`completed`/`failed`/`cancelled`）、进度与行数、`data_classification`、勾稽结论（T1–T16 逐条 status + diff）、缺口清单（具名 Gap，不是 0）、五条版本线（事实版本 / 计量版本 / 汇率版本 / 指标定义版本 / 模板版本）、发布谱系（`fpna_plan_versions` 的 prior + `scenario_type`）

**审计追溯**：数据版本号、合同版本号、利率版本号、计算规则版本号、导入批次号、创建/更新人及时间、审批人、附件索引

**AI 识别与入库**：AI 任务编号、文件编号/类型/哈希、上传人/时间、识别/OCR 状态、文档分类结果、识别出的合同/门店/事件、字段名称、AI 提取值/最终确认值、置信度评分、scope 初判、原文页码/位置坐标、差异原因、审核人/时间、入库结果状态

## 关键设计决策

| 决策 | 原因 |
|------|------|
| `db/init/01_init.sql` 合并所有迁移（无 goose 标记） | PostgreSQL 容器自动初始化更简单 |
| 数据库变更必须同时提供增量迁移和 `01_init.sql` 空库版本 | 已有 volume 不会重跑 init，两者缺一就会出现环境漂移 |
| Docker Compose 移除 `volumes: ./xxx:/app` 挂载 | 避免覆盖构建产物 |
| PaddleOCR 用 multipart form data 而非 base64 JSON | base64 JSON 方式返回 500，multipart 正常 |
| PaddleOCR 结果只解析 `jsonUrl` | API 当前只返回 `jsonUrl`，从 `result.layoutParsingResults[].markdown.text` 提取 |
| 多租户用 `legal_entity_id` 行级过滤 | MVP 更简单可控；admin 空 `legal_entity_id` 不加过滤 |
| 公开注册关闭 | 内部系统，管理员通过 `/api/v1/admin/users` 创建账号 |
| 租赁范围闸门前置到计量引擎 | 避免短期租赁、低价值资产、非租赁合同被错误资本化 |
| AI 运行在 Assist Mode | AI 只生成建议和草稿，入库、计量、过账仍由审批和锁账控制 |
| `/ai-chat` 作为默认录入入口，`/upload` 保留 | 降低录入摩擦，同时保留批量上传备用路径 |
| ERP 集成先做 CSV 导出 + 凭证回写 | 先跑通最小可演示链路，再按客户 ERP 做字段映射 |
| 模拟数据用固定 seed 生成 | 无设计伙伴时能稳定复演经营场景与异常，且可与正式数据严格区分 |
| 零售 Agent 只给只读 Tool | 在权限、版本、审批、审计不被破坏的前提下使用 AI |
| 三表模型引擎写成纯函数 + Ports | 引擎可以在没有库的情况下被 golden 测试逐格锁定；接线缺失退化为具名 Gap 而不是崩溃或假数字 |
| 租赁附表只做计量引擎的只读投影（`finmodel` 禁 import `ifrs16`） | 一套租赁数字只能有一个来源；用 import guard 而不是 code review 来保证 |
| 期初「未提供」与「闸未过」走相反路径 | 前者是信息不足（可降级），后者是数据有毒（必须挡住），当成同一件事会让错误期初污染全部后续期间 |
| 勾稽失败写 `fpna_data_quality_items` 而不只是标红 | 单人团队里「有人会看到红色」不成立；进队列才有人处理 |
| 假设建议只写 draft，approved-only 读取不回采 | AI 可以提议，不可以自我批准 |

## 关键集成接口

- **ERP/总账**：输出 IFRS 16 会计分录，回传凭证号与过账状态，支持冲销/重过账
- **AP/付款系统**：获取实际付款信息，比对差异，识别预付/欠付/日期偏差
- **门店主数据**：门店新增、闭店、区域/品牌调整
- **销售系统 / POS**：门店销售额与交易数，支撑 turnover rent 计算与 store-day 经营事实
- **固定资产/工程系统**：装修完工、恢复义务、减值线索
- **审批与身份系统**：单点登录、组织架构同步、审批流程集成

> 现状：以上均**未完成真实联调**，目前只有 CSV/XLSX 导入与 API 合同。真实 POS/ERP 接入是试点第一阻塞项。

## 验证

改完代码至少跑通：

```bash
cd core-service && GOCACHE=$(pwd)/.gocache go test ./... && go vet ./...
cd ../web && npm run type-check && npm run build && npm test
```

真实 PostgreSQL 集成测试需 `TEST_DATABASE_URL`，未设置时正常 skip —— **跨法人隔离、幂等与勾稽落库的证据大多在集成测试里**，只跑单元测试证明不了底线 1 和底线 4，改这些路径时必须带库跑。IFRS 16 回归用 `make ifrs16-regression`。

核对本文「当前工程事实」用：

```bash
grep -c '^CREATE TABLE' db/init/01_init.sql        # 表数
ls db/migrations | tail -1                          # 最新迁移
ls core-service/internal | wc -l                    # 包数
find web/app -name page.tsx | wc -l                 # 页面数
```

**替换类改动的验收规则（GUARD-001，缺失即退回）**：凡是「把 A 换成 B」的改动——内联样式换成类名、组件 A 换成组件 B、手写逻辑换成接缝函数——验收必须证明 **B 真的生效**，只证明 A 消失了不算数。判定「B 生效」的机械手段，按改动类型二选一或并用：

1. **运行时实测**：报告给出 `getComputedStyle` / `getBoundingClientRect` 的改动前后对照（例如 Modal 标题 line-height 修复前 768px → 修复后 32px；卡片高度 82px → 41px）。FIX-019 / FIX-005 是已落地的先例。
2. **精确规则体断言**：测试用 `ruleBody()` 一类的辅助函数取目标选择器的规则体做断言，禁止 `expect(css).toMatch(/A[\s\S]*?B/)` 全文跨规则正则（FIX-021 教训：懒惰通配会跨行命中无关规则，断言恒真）。先例：`web/app/home/chatLayout.test.ts`、`web/app/lib/kpi-card-height.test.ts`。

**构建期守卫要单独验，它们会被 Docker layer cache 掩盖（2026-08-23 教训）。** `Dockerfile` 里那段 anydoc SRI 校验把 `$ANYDOC_INTEGRITY` 无引号插进 JS，生成 `if(got!=sha512-…==)`，node 每次都 SyntaxError——**从引入起就没比较过任何东西**，因为那一层一直命中缓存、从不重跑。基础镜像 digest 变了才暴露。单元测试碰不到构建期守卫，CI 若也命中缓存就同样看不见。加或改这类守卫时，用错误输入实跑一次证明它拦得住：

```bash
docker build --build-arg ANYDOC_INTEGRITY=sha512-WRONG== --target anydoc-install -t probe -f core-service/Dockerfile core-service
# 应中止并打印 anydoc integrity mismatch: got … want …
```

样式重构类改动（内联样式 → 类名）尤其如此：`class-coverage.test.ts` 只能证明「类有规则」，不能证明「规则挂在了正确的元素上、值正确」（STY-005 教训：31 个类补上规则时 44 处替换点必须逐个核对同元素同值，报告给「文件/行号/原内联值/现类名/规则值/是否同元素」表）。自检句：**把 B 的规则删掉或改错，这条测试会不会红？不会红就是没写对。**

启动方式、端口、测试账号见 [README.md](README.md)。

**IFRS 16 计量参考值**（改计量引擎后用于快速自检）：初始负债 ¥3,255,676.79 / 36 个月摊销；月结 2024-01 参考分录利息 ¥13,318、折旧 ¥92,170、付款 ¥50,000。
