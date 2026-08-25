# AI 文档索引与现行决策

> 末次统一：2026-08-20
> 用途：**读任何一份 AI 文档之前先看这里**，确认它是否仍然有效。
> 维护规则：任何改变下表 §2 决策的变更，必须同时写 ADR 并更新本文；只改方案文档不算数。

---

## 0. 产品定位（所有 AI 工作的前提）

**产品是「线下零售门店经营分析工作站」。主线用户是零售 Finance BP 与集团 FP&A。**

```
主线（产品是什么）              次要（value-added 模块）
──────────────────────      ────────────────────────
门店 store-day 经营事实      →  IFRS 16 / CAS 21 租赁计量
四墙损益 · 租售比 · 坪效     →  月结、分录、披露、审计包
异常下钻 · 情景 · 行动闭环    →  合同台账与关键日期
单店利润表 · 法人级三表模型
FP&A 版本治理与滚动预测
```

**2026-08-20 补充：财务建模层已进入主线。** 单店利润表（`/store-pnl`）与法人级三表模型（`/financial-model`）是「四墙损益」那一行的完整形态，服务的是同一批用户（零售 Finance BP + 集团 FP&A）。它**消费**经营事实与租赁计量，**不自产**任何一方——租赁附表只能是计量引擎的只读投影（`finmodel` 禁 import `ifrs16`，有 import guard 锁定）。这条不是风格偏好：模型里一旦长出第二套租赁计算，两个数字必然分叉，而没有人能说清该信哪个。

**IFRS 16 是次要的 value-added 合规能力，不是产品定位。** 它的战略价值在两处：作为合规刚需的销售入口，以及让租约条款、面积、门店主数据先进系统——**这些数据不在客户的湖仓里，任何通用 BI 都分析不了不存在的数据**。

对 AI 工作的三条直接约束：

1. **Agent 的主战场是经营分析与财务建模**，不是合同录入。技能路由、评测集、底稿场景以零售经营为第一优先、财务建模（`fpna.*`）为第二优先，合同录入排最后。
2. **IFRS 16 在 AI 设计中的角色是「被保护对象」**——见 [ADR-0025](adr/0025-separate-certified-engine-output-from-exploratory-analysis.md) 的受保护度量清单。保护它，不是围绕它做产品。
3. **制造 / 设备 / 工厂维度不在当前范围。** 相关内容在需求清单 §9 保留供将来评估，但不进排期、不作为架构约束。

写文档、定技能、排优先级时若与本节冲突，以本节为准。

---

## 1. 文档状态总表

状态定义：**Current** 现行有效 · **Historical** 已交付的历史记录，正文不再更新 · **Partially Superseded** 部分作废 · **Superseded** 整体被取代。

### 1.1 设计与方案

| 文档 | 状态 | 说明 |
|---|---|---|
| [UIUX与产品能力全面改善建议书](UIUX与产品能力全面改善建议书.md) | **Partially Superseded** | 缺口清单已按代码实测重写，见 Spec（retail-workstation-honesty-and-capability-r1）；本文保留为需求来源记录。§0.5 复核记录列出六项被代码推翻的判断与三项与既有约束冲突的动作。根目录旧副本已删除，仅存 docs/ 这份 |
| [Spec：经营工作站的诚实性止血与能力补齐](specs/retail-workstation-honesty-and-capability-r1.md) | **Current** | R 批次口径来源：D-R1~D-R9。诚实性止血（设备口径冒充零售指标）、人效上架、五项真缺口（投前保本/新店可行性/差异归因/滚动滚入/公式编辑器）|
| [CodebaseDesign：经营工作站诚实性与能力补齐模块深化](CodebaseDesign_经营工作站诚实性与能力补齐_模块深化.md) | **Current** | 实施级设计：RH1~RH8，决策留痕 D-R10~D-R18；Metric Surface 收敛标签表、Display Basis Guard 口径判定、保本与新店可行性纯函数、模板校验端点与编辑器前端零本地校验 |
| [Agent Core（Go）设计 —— 对齐 pi 架构](Agent_Core_Go设计_对齐pi架构.md) | **Current** | **内核层现行依据**。纯循环 + 中间件链 + 订阅者；ai-service 退役映射 |
| [AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md) | **Current** | **能力层现行依据**。双轨执行、WorkingPaper、不变量、分阶段与验收 |
| [CodebaseDesign：AI 阶段 0 与 W1 模块深化](CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md) | **Current** | **实施级设计**。W1（agentcore）+ 阶段 0（workingpaper / docparse / triage / CLI / Web / 评测）的深模块接口、seam、决策留痕 D-A~G、验收映射 |
| [CodebaseDesign：AI 阶段 1 与 W2 模块深化](CodebaseDesign_AI阶段1与W2_模块深化.md) | **Current** | **实施级设计**。W2（治理中间件链 + ACORE-2 变异测试）+ 阶段 1（sensitivity 工具、S1 签约前底稿构建器与工具、aiagent 接线、CORR-1 评测）的接口与决策留痕 D-B1~B5 |
| [CodebaseDesign：AI 阶段 3 零售经营底稿模块深化](CodebaseDesign_AI阶段3零售经营底稿_模块深化.md) | **Current** | **实施级设计**。零售经营底稿（收尾底稿主线的产品决策 D-C1~C5）：retailpulse/store360/scenario 引擎 → 全 Certified 经营底稿 |
| [CodebaseDesign：AI 阶段 4 PageFill 填表缝模块深化](CodebaseDesign_AI阶段4_PageFill填表缝_模块深化.md) | **Current** | page_fill 协议（Exploratory 结构性不入 payload，I5/ACORE-12）、`retail.store_days.import.preview` 工具、导入页 `?fill=` 消费；D-D1~D3 |
| [CodebaseDesign：AI 阶段 5 MinIO 接线与三入口汇流模块深化](CodebaseDesign_AI阶段5_MinIO接线与三入口汇流_模块深化.md) | **Current** | MinIO 读取接线（page_fill 点亮）、三入口统一、G1 两平面汇流；D-E1~E4。**M1 已交付，M2/M3 进行中** |
| [Spec：Agent Runtime 完整升级（C1 批次）](specs/agent-runtime-overhaul-c1.md) | **Current** | **C1 批次口径来源**：D-C0~D-C10，Story 1–43。范围按能力清单判不按包判（D32）；三层推进各自可独立回退 |
| [CodebaseDesign：Agent Runtime 升级模块深化](CodebaseDesign_AgentRuntime升级_模块深化.md) | **Current** | **C1 实施级设计**：AR1 ContextKey（已交付）、AR2 Session Manager（已交付）、AR3 上下文工程（已交付并接线，flag 默认关，见 D38）、AR4 Subturn 委派、AR5 汇流与治理链上生产（b/c/d 已交付；「内核置换」口径订正见 ADR-0028 Correction）、AR6 Memory；决策留痕 D-C11~D-C20 与守卫 AR*-G* |
| [Research：picoclaw agent 闭包分析](research/AR5_picoclaw_agent闭包分析_2026-08-24.md) | **Historical** | AR5a 的只读分析：最小闭包清单、挂点对齐表、ACORE-2 九项可移植性判定（含审计传播三选一裁决）、形状 A 可行性、拆单建议。其裁决已被 AR5b/c/d 落实，作为决策依据留档 |
| [FP&A 与 Finance BP 经营决策及 AI 辅助需求清单](FP&A与Finance_BP经营决策及AI辅助需求清单.md) | Current | 业务需求有效。**§9 制造功能已标注为不在当前范围**；§12 的工具勾选表已移除，实现状态以代码为准 |
| [PRD：三表财务模型与单店利润表](PRD_三表财务模型与单店利润表.md) | **Current** | **业务需求现行依据**（财务经理视角）：S1 单店利润表页、S2 法人级三表模型、S3 受治理模板自定义、S4 AI 填数与报表生成、S5 集团合并视图。AI 相关诉求均声明沿用 ADR-0019/0025 与既有 WorkingPaper 先例，未改动 §2 任何决策。**附录 E（2026-08-20）全部 ✅、无 ⚠/❌ 行**；附录 B 的勾稽表述已随 P0-4 重设计同步修订 |
| [CodebaseDesign：三表财务模型与单店利润表模块深化](CodebaseDesign_三表模型与单店利润表_模块深化.md) | **Current** | **实施级设计**（上述 PRD 的配套，SM1–SM8 已全量落地）：SM1 模板/DSL、SM2 纯函数引擎（Run/Persist）、SM3 门店利润表投影、SM4 期初三道闸、SM5 AI 假设草稿、SM6 finmodel 底稿构建器、SM7 Agent 工具（现为 `fpna.*` 六个）、SM8 集团视图（克制）；决策留痕 D-S1~S9；三条架构测试：`finmodel` 不得 import `ifrs16`（遍历全部子包）、`fin_model_runs` 唯一写入口、底稿 lint 的 `tie_out_unpassed` fail-closed |
| ~~PRD：租赁经营决策与 AI Copilot 平台~~ | **已归档** `archive/superseded-prds-2026-08/` | 编于零售转型前，标题与内容以「租赁」为主体；产品边界已由零售 PRD (P0–P5) 与财务BP/FP&A PRD (F0–F9) 取代 |
| ~~AI Agent 填表升级（tau + anydoc）实施计划~~ | **已归档** `archive/ai-runtime-2026-08/` | tau 作废（ADR-0022）；anydoc 与填表缝**已迁入** Agent Core 设计 §8.2 与附录 A |
| ~~CodebaseDesign：Agent 填表升级模块深化~~ | **已归档** 同上 | 同上（M1 → §8.2，M4 → 附录 A） |
| ~~AI Chat 升级方案（参考 Pi Coding Agent）~~ | **已归档** 同上 | 早期 pi 借鉴，P0 建议已交付；精确映射见 Agent Core 设计 |

### 1.2 已交付的运行时文档

| 文档 | 状态 | 说明 |
|---|---|---|
| [AI Agent 与 CLI 架构演进 PRD](AI_Agent_与_CLI_架构演进_PRD.md) | **Historical** | AG-001~035 已交付（`b1532b4`）。Tool Runtime / Gateway / Capability 契约**仍然有效且不变** |
| ~~AI Agent 与 CLI 架构演进实施计划~~ | **已归档** `archive/ai-runtime-2026-08/` | 交付记录，1816 行；对外契约以保留的 PRD 为准 |
| [AI Agent 运行运维手册](AI_Agent_运行运维手册.md) | Current | chat / parser / planner / upload 均已内迁 core-service（`internal/llm` / `internal/aiintake` / `agentrunner.PlannerLLM` / `miniostore`），ai-service 已退役 |
| ~~AI Agent 外部验收清单~~ | **已归档** 同上 | 后续验收以 Agent Core 设计 §11 与底稿方案 §12 为准 |

### 1.3 合规、契约与评测

| 文档 | 状态 | 说明 |
|---|---|---|
| [Agent 有限自动化风险登记册 v1](agent-automation-risk-register.v1.md) | Current | 不授权任何自动过账/审批/锁账 |
| [Agent Evaluation Dataset v1](agent-evaluation.v1.md) | Current（已扩容） | 定位为三层评测的 **L1**；新增 provenance / protected_measure / middleware_chain / triage_refusal 四个 category |
| `contracts/ai-intake.v1/*.json` | Current | 合同 / 台账 / 租金表 / 事件的抽取契约。**W5 迁 Go 后契约不变**，仅实现方变更 |

### 1.4 ADR

| ADR | 主题 | 状态 |
|---|---|---|
| [0002](adr/0002-versioned-ai-intake-seam.md) | 版本化 AI intake 缝 | Accepted |
| [0003](adr/0003-deep-web-ai-chat-runtime.md) | Web AI Chat 运行时 | Accepted |
| [0004](adr/0004-deep-core-ai-agent-runtime.md) | Core AI Agent 运行时 | Accepted **+ Addendum A（纯循环 + 订阅者）** |
| [0007](adr/0007-deep-ai-intake-producer.md) | AI intake producer | Accepted |
| [0012](adr/0012-separate-agent-signals-from-control-conclusions.md) | Agent 信号与控制结论分离 | Accepted |
| [0019](adr/0019-agent-tool-runtime-policy-and-threat-model.md) | Tool Runtime 政策与威胁模型 | Accepted **+ Addendum A（治理中间件链）** |
| [0021](adr/0021-licensing-and-open-source-posture.md) | 许可证姿态（非 MIT，三层） | Accepted |
| [0022](adr/0022-first-party-go-agent-core-modelled-on-pi.md) | 自研 Go Agent Core，架构对齐 pi | **Partially Superseded**（§1 被 ADR-0027 取代；§2–4 仍现行） |
| [**0023**](adr/0023-retire-the-first-party-python-ai-service.md) | **退役自研 Python ai-service** | **Accepted（新）** |
| [**0024**](adr/0024-remove-the-agpl-pdf-dependency.md) | **移除 AGPL 的 PyMuPDF，解析按证据需求分流** | **Accepted（新）** |
| [**0025**](adr/0025-separate-certified-engine-output-from-exploratory-analysis.md) | **双轨执行与受保护度量红线** | **Accepted（新）** |
| [**0026**](adr/0026-vendor-picoclaw-im-channels.md) | **vendor picoclaw 的飞书/企微渠道（MIT）** | **Accepted（新）**；§2 被 ADR-0027 修订 |
| [**0027**](adr/0027-adopt-picoclaw-agent-core-keep-the-governance-chain.md) | **采用 picoclaw agent 内核，治理链移植到其 hook 挂点** | **Accepted**；Non-goals 被 ADR-0028 收窄 |
| [**0028**](adr/0028-extend-picoclaw-adoption-to-the-whole-runtime.md) | **把采用范围从 agent loop 扩到整个 runtime**（session / 上下文工程 / subturn / MCP 等） | **Accepted（新）** |

### 1.5 归档区

全部历史文档在 `docs/archive/`，**均带 ARCHIVED 横幅，永不作为现行依据**（规则见 AGENTS.md「文档归档规则」，由 `scripts/check_docs_archive.sh` 在 CI 强制）。

| 目录 | 份数 | 内容 |
|---|---|---|
| `ai-runtime-2026-08/` | 5 | AI/CLI 实施计划、外部验收清单、pi 参考方案、tau 计划与其模块深化 |
| `strategy-inputs-2026-08/` | 2 | 零售转型可行性报告、FP&A 产品战略分析 |
| `uiux-reviews-2026-08/` | 3 | UIUX 评估报告、零售 UIUX 评审、架构改善方案 |
| `ifrs16-mvp-2026-05/` | 2 | IFRS16 IT 需求文档、MVP 技术架构方案 |
| `pre-retail-roadmap/` | 3 | 零售转型前的路线图与任务清单 |
| `superseded-prds-2026-08/` | 1 | 租赁经营决策与 AI Copilot 平台 PRD |

**2026-08-18 的约定**：此前 `docs/archive-local/` 被 gitignore、只存本机。现改为跟踪式归档——本地归档有三个代价（机器丢失即消失、其他 clone 与 worktree 看不到、git 历史里的删除文件没人会想到去找），而它防的那件事由**横幅 + AGENTS.md 规则 + CI 守卫**替代。

**2026-08-20 的收缩（本次）**：归档从 30 份 / 25,025 行减到 17 份 / 7,863 行。删掉的是**已完结的交付流程工单**——`ui-upgrade-2026-05/`（4）、`uiux-overhaul-2026-08/`（3）、`retail-mvp-execution-2026-08/`（含 tasks/，MAX-001~009 工单与演示脚本）、`project-history/`（1）、`superseded-lists/`（3）。理由：跟踪式归档解决的是「副本会丢」，解决不了「语料量本身压垮上下文」；一份已完结、结论已被现行文档吸收的工单，留着只增加 agent 误读的面积。**保留的是「被取代但推理仍有价值」的方案文档**——它们回答「为什么不这么做」，这个问题现行文档答不了。

另删除 `docs/IFRS16_计量回归对数报告.md`（841 行）：它是 `make ifrs16-regression` 的生成产物，现已 gitignore，需要时一条命令重生成。

---

## 2. 现行决策登记

每条决策标注**决定了什么**、**推翻了什么**、**留痕在哪**。

| # | 决策 | 推翻的旧结论 | 留痕 |
|---|---|---|---|
| ~~**D1**~~ | ~~Agent 内核自研 Go（`internal/agentcore`），不引入 pi 本体~~ **已被 D28 取代** | 填表计划 D1/D2/D4「引入 tau 作为大脑」 | ADR-0022 §1 |
| **D2** | 借鉴 pi 的**运行时抽象**（纯循环 + 注入依赖 + 两个闸点 + 订阅者结算），**不借鉴其信任模型**（无权限、无持久化、开放生态） | — | ADR-0022 §2–3 |
| **D3** | 治理收拢为**有序中间件链**：TenantScope → CapabilityCheck → ProtectedMeasure → BudgetGuard → IdempotencyGuard → ReviewGate | 治理散落在 descriptor / scope.go / audit.go / Limits / allowlist 五处 | ADR-0019 Addendum A |
| **D4** | 持久化改为**订阅者**；run 直到所有订阅者返回才结算 | `aichat.Runtime` 在循环内同步写库 | ADR-0004 Addendum A |
| **D5** | **退役自研 Python**。ai-service 全部职责迁 Go，后端只剩 Go；Node 仅前端 | 「ai-service 是 3.11，另开 3.12 容器」 | ADR-0023 |
| **D6** | **删除 PyMuPDF**（AGPL 与 ADR-0021 冲突）。解析按**是否需要证据坐标**分流：anydoc / PaddleOCR / excelize | 「PyMuPDF 做 PDF 文本层 + 证据 fallback」 | ADR-0024 |
| **D7** | **惰性证据**：首轮 anydoc 出文本，用户点「查看证据」才跑 OCR 并缓存 | 每份文件都跑 OCR | ADR-0024 §3 |
| **D8** | OCR 不可用时**降级 anydoc 但证据标 `unavailable`，不得声称坐标** | 「降级到 PyMuPDF 文本层」 | ADR-0024 §4 |
| **D9** | **双轨执行**：Tier A Certified / Tier B Exploratory，**provenance 到单元格级** | — | ADR-0025 §1 |
| **D10** | **10 项 protected_measures 永不出自 Tier B**，请求期 + 产物期双重 fail-closed 拦截 | — | ADR-0025 §2–3 |
| **D11** | **沙箱整体后置到阶段 4**，进入条件为「已有 ≥1 真实客户且合规要求明确」；但数据模型（measure_id / basis / provenance / lint）从阶段 0 就建 | — | ADR-0025 §5 |
| **D12** | 当前**无真实客户**，BIZ 验收改用 DEMO-1~3 演示转化替代判据，并须如实标注 | — | 底稿方案 §12.5.1 |
| **D13** | **表格交给模型判断时，送列画像不送原始值**（现有 `RetailMappingAI` 的做法，升格为规范） | — | 底稿方案 §1.2 G3 |
| **D14** | `agent-runner` **收敛为 agentcore 的 driver**，保留 checkpoint 与租约恢复，不删除 | 填表计划「tau 平价后退役 agent-runner」 | ADR-0022 §Consequences |
| **D15** | 团队为**单人 + AI**，无第二位人类。多人控制项一律「标准不降、执行方式替换、残余风险声明」，并优先改造为 CI 可强制的机器检查 | 各文档中默认多人组织的签字/双评/第三方复核条款 | 本文 §6、ADR-0025 §2/§4、底稿方案 §12.7.4 |
| **D16** | 产品定位为**线下零售门店经营分析工作站**，主线用户零售 Finance BP + FP&A；**IFRS 16 降为次要 value-added 合规模块**；制造/设备维度移出当前范围 | 多份文档中「租赁管理系统」「租赁及相关经营资产的决策平台」等以租赁为主体的定位表述 | 本文 §0 |
| **D17** | **财务建模层进主线**：`/store-pnl` 与 `/financial-model` 与零售三页同级。`finmodel` **禁 import `ifrs16`**（import guard 遍历全部子包），租赁附表只能是 `measurement_results` 的只读投影 | 「三表模型自己算一套租赁数」的任何实现路径 | ADR-0025 精神 + 模块深化 D-S3；`finmodel/importguard_test.go` |
| **D18** | **勾稽不得恒真**。拿别名比自己、拿定义式倒减自己、返回 `not_applicable` 的桩都不算检查；按构造必然成立的关系改写为构造断言并同步改规格。T1–T16 每条必须有先红后绿的反向测试 | 初版 T2/T4/T11 的恒真实现与桩 | PRD 附录 B（已随 P0-4 修订）；`finmodel/tieouts.go` |
| **D19** | **期初的两种失败走相反路径**：`openingAbsent` 降级、`openingRejected` 阻止 run 落库 | 初版把闸失败一律降级为「未提供期初」 | 模块深化 SM4；`finmodel/engine.go` |
| **D20** | **AI 写类工具只写 draft 层**，`source=ai_suggestion`；approved-only 读取永不回采 draft。**端口未接线的工具诚实拒绝，但不得用 nil 端口无条件注册**（会让重名注册把真实端口挡在外面） | 「工具注册了就算接线了」的隐含假设（1d67dd6 的声明曾因此落空） | ADR-0025；`aiagent/agent.go` 的 `finModelRepo == nil` 二选一分支 |
| **D21** | **ADR-0023 的退役已全部完成（W4–W6，2026-08-20）。** ai-service 目录已删除，任何代码不得再调用已退役的 Python 端点；现状以 AGENTS.md「当前工程事实」与本文 G6/G7 已解决为准 | 多份设计文档中「ai-service 退役映射」被读成已完成事实（现已成真） | 本文 §3 缺口 G7 → ✅ |
| **D22** | **口径冲突只降级不换算。** 一个数值在某展示语境下要么可用、要么显示「—」加原因，**不存在第三条路**（估算、近似、改名）。设备口径的数不得贴零售标题，`resolveBasis` 是唯一判定点 | `/performance` 用 `oee_pct` 渲染「同群平均坪效达成率」、`plant_code` 空值兜底成「核心商圈」；i18n 层六个键被系统性零售化改名（连英文一起改） | Spec D-R1；模块深化 D-R11；`web/app/lib/displayBasis.ts` |
| **D23** | **指标露出做成受校验清单，标签表收敛为一处。** 清单里的每个 code 必须在 `retailkpi.Definitions` 里存在，否则启动即失败；中文名只有 `retailkpi` 一个真相源 | `retailstore360` 包内第二张 labels map（加指标要改四处，漏一处的表现是前端显示指标码） | 模块深化 D-R10；`retailkpi/surface.go` |
| **D24** | **连环替代法与「有意义的残差」互斥，二选一。** 精确连环替代下各步贡献是望远镜和，求和恒等于总差异，残差恒为浮点噪声；残差只在隔离替代法下非零，而那种方法与顺序无关。选连环替代则守恒等式是构造性质**不是检查对象**，真正的检查是中间值序列 + 顺序敏感性 | 本文 D18 的精神在差异归因上被误用：Spec D-R5 初版同时要求两者，会留下一条永远绿的守恒断言 | Spec D-R5 更正节；`varianceattribution/attribution_test.go` |
| **D25** | **折现率缺失是部分降级不是整体拒绝。** IRR / NPV / 动态回本期返回具名 Gap，静态回本期与盈亏平衡销售额照常返回——后两者不依赖折现率，一起挡掉是过度保守。**任何情况下不得使用默认折现率** | 「缺一个输入就整体不可用」的一刀切 | 模块深化 D-R14；`newstorefeasibility/feasibility.go` |
| **D26** | **新店测算不得长出第二套租赁计算。** `newstorefeasibility` 禁 import `ifrs16`，租赁与 ROU 只经 `LeaseProjectionReader` 从 `measurement_results` 只读投影取；import guard **遍历全部子包** | 风险红线 14 在 finmodel 之外的第二个落点 | 模块深化 D-R13；`newstorefeasibility/importguard_test.go` |
| **D27** | **前端不复刻后端的 DSL 解析器，一次本地校验也不做**（含括号配对这类「显然安全」的检查）。公式校验一律走 `POST /financial-model/templates/validate`，复用 `template.Compile` 同一条路径 | 「先在前端挡一下明显错误」的直觉——开了口子它会长大，然后与后端分叉，届时「界面说没问题、保存时报错」比现在难查 | 模块深化 D-R16；`web/app/financial-model/formula-editor.test.tsx` |
| **D28** | **Agent 内核改用 picoclaw 的 `pkg/agent`**（vendor + 适配）。ADR-0022 从未评估过 picoclaw——它否决的 pi 是 TypeScript 的 `earendil-works/pi`，理由是 Node 运行时成本，对 Go 的 picoclaw 不成立；且 picoclaw ← openclaw ← pi 同源，采用它是**落实**而非推翻 ADR-0022 §2 | D1「不引入 pi 本体」 | ADR-0027 §1 |
| **D29** | **治理链移植不重写**，六前三后接到 `ToolInterceptor`/`LLMInterceptor`/`ToolApprover`；**ACORE-2 九项变异必须逐项重新证红证绿**，一项不少，否则迁移中止 | — | ADR-0027 §3 |
| **D30** | **picoclaw 的信任模型仍不采纳**（ADR-0022 §3 不变）。`tool_allowlist.go` 是防打扰清单不是授权，与渠道侧 `IsAllowed` 同一性质 | — | ADR-0027 §2 |
| **D31** | **上下文压缩改为「可得但需门禁」**。ADR-0022 Non-goals 曾推迟它至「观察到真实溢出」，该条件**未被证明满足**；采用上游 `context_manager`/`budget`/`usage` 后仍须先用本仓会话形状钉住行为：压缩不得丢审计内容、压缩后的 run 仍可从 checkpoint 重放。与底稿溯源冲突时**溯源优先** | ADR-0022 Non-goals | ADR-0027 §4 |
| **D32** | **范围按能力清单判，不按包判。** picoclaw 36 个包里 7 个是边缘硬件、2 个是自更新，搬进来不增加能力只增加漂移风险；反过来「我们已经有类似的」也不能用来跳过真实缺口——`State.Messages()` 曾因此被当作「有上下文管理」 | — | ADR-0028 §1 · Spec C1 D-C0 |
| **D33** | **runtime 隔离取无状态共享，不做每账号实例。** 蓝图 §三 描述的是每账号实例，不采用：其四条要求无状态共享全部满足，且按权限过滤工具改为逐次进行后**权限变更立即生效**。硬约束：`agentcore.Agent` 的五个可变字段必须推到调用参数 | 蓝图 §三 | ADR-0028 §2 · D-C19 |
| **D34** | **`ContextKey` 是隔离原语，五个维度**：法人 / 用户 / 会话 / scope 指纹 / 数据分类。字段非导出、唯一构造器吃 `Principal`——漏传、传错顺序、传空串三类错误变成编译期不可表达 | — | ADR-0028 §3 · D-C11/12/20 |
| **D35** | **上下文工程顺序不可颠倒**：先计数、再预算、最后压缩。分词器不可得时**拒绝而非估算**（误差在预算边界最大，而边界是唯一重要处）。审计承载内容靠 `classify` 先切两段、压缩器签名只收 compactable 来保护，不靠规则 | ADR-0022 Non-goals 的压缩推迟条款 | ADR-0028 §4 · D-C14/15 |
| **D36** | **汇流是内核置换的一部分，不是后续项。** `agentcore.New` 生产零调用，换一个没在跑的内核等于死代码换死代码。G1 悬置数月正因为从没有一条断言说「生产路径确实经过内核」 | G1「两平面未汇流」 | ADR-0028 §5 |
| **D37** | **token 计数取 pi 双轨形态**：provider 实测 usage（llm.ParseUsage 同源字段）为主真值，未发送尾部消息 chars/4 兜底、下一轮被真值覆盖；分词器注册制与「缺失即拒绝」不采用。中文密度问题因此消解（历史计数与语言无关）。残余风险：纯中文尾部低估，已由真值主轨封顶；除数可一行调整 | D35/D-C15 的注册制方案 | 用户裁决 2026-08-24；contextassembler.PiStyleEstimator |
| **D38** | **AR3 接线完成（2026-08-24）：chat history 来源唯一换轨**。`executeChatRequest` 在 callLLM 前 Assemble，只换 history 参数来源，路由/上下文加载/prompt 构建零改动；flag `CONTEXT_ASSEMBLER_ENABLED`（默认关）= 进程接线层 nil 注入，回滚即重启。usage 真值经 callLLM 第 4 返回值回填 `ai_chat_messages.measured_tokens`（迁移 063，0=未测量哨兵）。压缩发生时 emit `context_compacted`（Dropped refs 可解析）。Assemble 失败响亮终止；全局 admin 无法人身份自动走 legacy 路径。预算默认 deepseek-v4-flash 1M / gpt-4o 128K、reserve 4096，env 可覆盖 | D37 | 移交单 `tmp/handover-AR3wiring.md`；Summarizer drop-only 上线（摘要模型待裁决）、Story 18 延后、工具消息进历史前 Kind 分类对存量历史不触发（登记不扩表） |

---

## 3. 当前缺口（对代码的核查结论，2026-08-18，2026-08-19 更新）

| 缺口 | 状态 | 归属 |
|---|---|---|
| G1 两个 Agent 平面未汇流（23 处 `Status: "pending"` 静态卡片） | ✅ **已解决（2026-08-24，`b2f532e` AR5d）。** 生产 chat 接线（`handlers/ai_chat.go`）的 executor 换为内核适配器 `agentkernel/chatexec.Executor`：每回合绑定 vendored HookManager 并挂 ACORE-2 九控制治理链。汇流断言 `ExecutorKind()` 双层锁定（类型层 + 判别器反向对照）；形状 A 守卫 `TestExecutorHoldsNoPerRunMutableState` 防回归 | ADR-0028 §5 / Spec C1 D36 |
| G2 无文件分诊，兜底 `return "contract"` | 🟡 **部分解决**：`lease.file.triage` 确定性分诊落地、`return "contract"` 兜底已删除、域外文件（发票/劳动合同/宣传册）显式拒绝并问用户；LLM 分类器与 ≥50 份语料的 CORR-6 完整判据待 L2 语料建设 | 底稿方案 §6.1，阶段 0 |
| G3 经营数据语义映射 | ✅ **已解决**（Profile 复用与漂移检测未做） | — / 阶段 3 |
| G4 无代码执行能力 | 未解决（**刻意后置**） | ADR-0025 §5，阶段 4 |
| G5 无 xlsx/docx 产出（导出仍是 CSV） | 🟡 **部分解决**：`workingpaper` xlsx/docx 确定性渲染器 + lint 门 + `GET /ai/chat/artifacts/:id/export` 端点已落地；端到端 WorkingPaper 生成链路（S1 底稿）未接线 | 阶段 0 → 阶段 1 |
| **G6 PyMuPDF 的 AGPL 风险** | ✅ **已解决（W6，2026-08-20）。** `ai-service/` 整目录已删除，该 AGPL PDF 依赖不再存在于仓库代码；解析改由 `internal/docparse`（anydoc / PaddleOCR）承担 | ADR-0024 |
| **G7 ai-service 未退役（新登记，2026-08-20）** | ✅ **已解决（W6，2026-08-20）。** W4+W5 全部落地：chat（`/chat` ×2）与 planner（`/api/v1/agent/plan`）迁入 `internal/llm` / `agentrunner.PlannerLLM`；四个 `/parse/*` 迁入 `internal/aiintake` 生产侧 + `internal/docparse`；`/suggest-mapping` 迁入 `internal/llm`；`/files/upload` 迁入 `miniostore` 写入侧。§0.2 九条依赖点清零，ai-service 整目录删除 | ADR-0023 W4–W6 |
| **G9 sessionmanager 零调用方（AR2 已交付但未接线）** | ✅ **全部接线完成（RT1-B，2026-08-25）**：chat 平面（SI1 Part B）、**gateway + runner 平面（RT1-B）**——gateway `CreateSession` 经同一 `SessionOwner`（agentrunner 唯一会话路径是 gateway HTTP 入口，接好即覆盖），机器断言 + 反向对照见 `TestAgentGatewayWithSessionOwnerReportsAdapterKind`。worker 租约轴不加法人参数（机器身份信任模型）：断言钉住的是「worker 只能访问自己持租约的那个 run、claim 不改所有权」——**不是**「worker 池不跨法人」（worker 池从共享队列取队，不按法人分区，是部署级信任的选择，不是结构保证）。**worker→法人绑定关系为独立开放决策，见新登记 RT1-OPEN-1，不随 G9 关闭。** G9 关闭，不再有到期日 | C1 批次评审 + SI1 + RT1 |
| **G10 会话状态三值承诺未兑现（新登记，RT1-L3-A，2026-08-26）** | 🟡 **如实登记，未解决（刻意）**：`Session.Status` 注释与 `ai_chat_sessions.status` 的 CHECK 均声明 active/archived/closed 三值，但全仓唯一写路径是创建时的 active——注释与 CHECK 是未兑现的承诺。不得据此暗示系统具备归档或关闭语义；引入迁移语义时按未决 #9 的重开条件一并兑现，届时此条关闭 | Spec C1 Story 18；AI 文档索引 §5 #9 |

> W1 + 阶段 0 已按 [CodebaseDesign 模块深化](CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md) 交付：`internal/agentcore`（纯循环内核 + ACORE-1/5/6/8 测试）、`protected_measures`（10 项 + 词法探针）、`internal/workingpaper`（I1/I2/I3/I6 lint + 封面 + 渲染）、`internal/docparse`（CSV/anydoc/PaddleOCR）、`internal/agentseval`（不变量与 triage 用例，harness 第三段 `invariants`）、CLI 三层命令（commit 只对人）、Web（去关键词猜测、tool_start 消费、working_paper 渲染）。
>
> **W2 + 阶段 1 已按 [CodebaseDesign：AI 阶段 1 与 W2](CodebaseDesign_AI阶段1与W2_模块深化.md) 交付（2026-08-19）**：`agentcore/hooks` 六前 + 三后治理中间件与 `Governance` 固定顺序链（**注：该治理链已被 AR5c 移植版取代并于 RT1-D-2 删除，见下方 D-2 登记**）、ACORE-2 变异测试（9 项全锁）、`agenttools.ExecutionGuard` seam + aiagent 全路径接链（平价门：既有测试保持全绿）、`lease.report.sensitivity` 工具（补 /sensitivity 断链，共用同一 reporting 投影）、`workingpaper/s1` 构建器（predeal/dealcompare/shock 重跑 → 全 Certified 单元格，I2 锚点为工具自身已审计调用）、`lease.working_paper.s1.generate`（LevelDraft + Review Gate）、aiagent 确定性触发（消息带确认假设块 → 底稿 → working_paper artifact）、评测新增 `s1_engine_consistency` category（CORR-1 确定性半边 + 2 份仿真报价 fixtures，harness 12/12）。
>
> **三表财务模型与单店利润表已全量交付（2026-08-20）**：SM1–SM8 按 [模块深化](CodebaseDesign_三表模型与单店利润表_模块深化.md) 落地，PRD 附录 E 全部 ✅。随后的**双轴评审 19 项修复**（P0 九项 / P1 五项 / P2 五项）已合并，其中改变结论的五项值得记住：跨法人校验补进 definition 与门店两条入口（底线 1，两处泄露）；出厂模板 Actual 区改为读事实而非假设推导（否则真实数据的 run 永远过不了发布门禁）；T2/T4/T11 三条恒真/桩勾稽重设计 + 十六条反向测试补齐；期初闸失败改为阻止 run 而非降级；`fpna.*` 工具的 nil 注册挡住生产端口（1d67dd6 的接线声明曾因此落空）。修复工单已完成删除，逐条结论已吸收进本表、AGENTS.md 与 PRD 附录 E。
>
> **阶段 3（零售经营底稿，产品主线的底稿）已按 [CodebaseDesign：AI 阶段 3](CodebaseDesign_AI阶段3零售经营底稿_模块深化.md) 交付（2026-08-19）**：底稿主线切回零售（D-C1：S4/S3/S2 后移，S1 保留）；`workingpaper/retail` 构建器（pulse/store360/scenario → 全 Certified/SystemFact 单元格，1:1 保值断言锁定、nil 跳格不填 0、覆盖不足/多币种/模拟标识/抑制信号一一进 DataGaps、残差显式保留）；`retail.working_paper.store.generate` 工具（LevelDraft + Review Gate，复用 scopedRetailReader 权限过滤，情景镜像聊天阻断语义 D-C3）；aiagent「底稿 + filters」确定性触发 → working_paper artifact（复用面板与 xlsx/docx 导出）；评测新增 `retail_paper_sanctity` category（harness 13/13）；CLI `run events --format table|ndjson`。core-service `go test ./...` + `go vet ./...`、web 回归全绿。
>
> **C1 批次已交付（2026-08-24，截至 `192234e`）**：AR1 ContextKey（`agentcontext`，五维隔离键，`f2dcaf0`）；AR5b vendor picoclaw 内核切片（`agentkernel/third_party/picoclaw`，回合循环仍在 `picoclaw_agent_core` build tag 后，启用是独立票，`e0b4546`）；AR5c 治理链移植到 HookManager 挂点（九控制 + 装配序显式优先级，`9b700d0`）；AR5d 汇流（生产 chat executor 切到 `chatexec.Executor`，G1 关闭，`b2f532e`）；AR2 Session Manager（`sessionmanager`，两方法接口 + Store 端口 + 引用计数防孤儿租约 + migration 062，`192234e`）。评审修复随票吸收：admin 空法人回归（identityComplete 接受 Global）、Save upsert 归属谓词、挂载优先级显式化。
>
> **RT1-D-2 旧链删除（2026-08-26）**：`internal/agentcore/hooks` 整包删除（W2 六前+三后实现）。事实基础：生产装配中它只剩一个挂点（`aiagent` 的 base runtime），而 chat/gateway 均以 WithGuard 覆盖、唯一不被覆盖的 `Agent.Execute` 无生产调用方——旧链已是死代码。语义选择：**base 回落 Evaluate**（不换新链）——逐行比对证实其判定与 Evaluate 同构、码走同一映射器；三边比对（新链/旧链/Evaluate，9 场景含 TenantScope 不对称行）证实删除零漂移。等价性的适用范围有明确边界：ProtectedMeasure / BudgetGuard 不在 Evaluate 内（两条路径的生产 Deps 里本就是 nil），TenantScope 身份完整性链侧严于 Evaluate（AR5d 决策）——锚测试 `governance/dualpath_rc1_test.go` 注释与 `TestDualPathKnownAsymmetryTenantScopeCompleteness` 锁定。`AgentToolRuntime()`（零生产调用方，仅靠一条测试存活）一并删除，反向对照改为直接构造 base runtime。`internal/agentcore` 本体保留（llm/stream.go 在用）。

---

## 4. 阅读路径

- **想知道 Agent 怎么跑** → [Agent Core（Go）设计](Agent_Core_Go设计_对齐pi架构.md) → ADR-0022 / 0019 Addendum A
- **想知道 Agent 产出什么** → [AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md) → ADR-0025
- **想知道文件怎么解析** → ADR-0024 → 底稿方案 §6
- **想知道为什么不用 tau / pi 本体** → ADR-0022 Context
- **想知道验收怎么判** → 底稿方案 §12（41 项）+ Agent Core 设计 §11（ACORE-1~9）
- **想知道评测怎么跑** → [Agent Evaluation Dataset v1](agent-evaluation.v1.md) + 底稿方案 §12.7

---

## 5. 未决事项汇总

集中列出，避免散落在各文档尾部而被遗忘。

| # | 待决 | 阻塞什么 | 归属文档 |
|---|---|---|---|
| 1 | 沙箱选型与隔离档位（挂首个标杆客户合规要求） | 阶段 4 | 底稿方案 §15.1 |
| 2 | 国内客户的沙箱落地方案 | 阶段 4 | 底稿方案 §15.2 |
| 3 | protected_measures 清单首次定稿 | ~~阶段 0~~ **不阻塞**——写一条带日期和理由的决策日志条目即可（D15） | ADR-0025 §2 |
| 4 | Tier B 产物能否导出给外部审计师 | 阶段 4 | 底稿方案 §15.5 |
| 5 | ~~固化通道的季度人力承诺~~ → 改为**自动建 issue** | 不阻塞 | ADR-0025 §4 |
| 6 | anydoc 二进制的供应链管理（版本 + checksum 钉死方式） | ✅ **已定（W5-1）且自 2026-08-23 起真正生效**：`@firecrawl/anydoc@0.2.0` + npm SRI integrity（sha512）钉死进 `Dockerfile` / `Dockerfile.agent-runner` 的 `anydoc-install` 阶段，`ANYDOC_BIN_PATH` 由 compose 传入。**订正：W5-1 到 2026-08-23 之间这条校验从未执行过**——`$ANYDOC_INTEGRITY` 无引号插进 JS 源码导致 SyntaxError，一直被 Docker layer cache 掩盖；`5e1e6f0` 修好并用错误哈希实证它拦得住。详见下方 §5.1 | ADR-0024；`5e1e6f0` |
| 7 | `controlledxlsx` 与 excelize 的去留 | ✅ **已定（W5-3）**：`controlledxlsx` **保留**——受控模板路径的零依赖优点成立，不与 excelize 合并；intake 的 Excel 确定性读取统一走 `internal/aiintake/excel.go`（excelize），`internal/controlledxlsx` 仅服务受控模板导出 | Agent Core 设计 §12.1 |
| 8 | 上下文压缩是否进 W1–W6 | W6 | Agent Core 设计 §12.2 |
| 9 | **Story 18 会话状态迁移可追溯**（spec C1）→ **延期（2026-08-26，前提缺失）**。实测：会话状态机当前单态——status 唯一写入是创建时的 active（`manager.go:235`；PostgresStore.Save 经 `EXCLUDED.status` 透传同值），archived/closed 全仓零写路径（两条 UPDATE ai_chat_sessions 均不碰 status，`ai_chat_runtime.go:292/:641`；aichat/review.go:82 的 archived 是 artifact 状态非会话状态）。工单原写「迁移发生在三处」有误已订正：仅创建一处是状态写入，Close 是缓存结算不改 Status，evictExpired 纯缓存淘汰不触 Store。现在实现 tracing = 给不存在的迁移造永远为空的日志，与正常工作的无法区分。**重开条件（带牙齿）**：任何引入 archived/closed 写路径的改动必须同票携带 (a) 迁移留痕——经 Store 端口扩展或新窄端口（D-C4 不做 IO），(b) 留痕不含 legal_entity_id/user_id（key.go D-C11），(c) 反向测试 | ~~C1 第二层验收完整性~~（第二层已验收） | Spec C1 Story 18；`tmp/delivery-L3-A.md` |
| 10 | **Session Manager 接线票约束两条**：release 不落盘 / Close 落盘的不对称；`Session` 导出字段跨持有者共享可变锚点。接线定调用契约时一并处理（flush 语义或只读视图二选一） | 六处调用方接线票 | `tmp/delivery-AR2.md` §4 |
| 11 | **AR5c 离线测试按字母序派发**（NamedHook 默认 Priority=0）：单控制变异不受影响但顺序未被那些测试锁定；生产绑定已用显式优先级并有顺序断言。建议 governance_test 补显式优先级 | 无阻塞；防未来误读 | AR5 双轴评审 Standards #3 |
| 12 | **picoclaw 完整回合循环启用**（去掉 `picoclaw_agent_core` build tag 需补 ~6,300 行闭包）与 AR5e（steering/abort + 流式事件接 Web 层） | **只阻塞 AR4 子任务委派**（`turn_coord`/`subturn` 在 tagged 文件里，L3 范围裁决 2026-08-25 已将 AR4 推迟）。**当前不阻塞任何在途工作**：治理链只消费 hook 符号（`ToolInterceptor`/`HookDecision`，未挂 tag 的 `hooks.go`/`events.go`/`turn_context.go`），AR3 已交付接线且不碰回合循环；十个文件里七个默认不编译。不要顺手去 tag，那是独立票 | `tmp/blocked-AR5b.md`；AR5a §5 |
| 13 | 定期用错误输入跑构建期守卫的检查尚未进 CI | 见下方 §5.1 | 本文 §5.1 |
| 14 | **RT1-OPEN-1：worker→法人的绑定关系**——worker 池从共享队列取队、不按法人分区（部署级信任的选择，不是结构保证；RT1-B 已如实登记并订正）。若需要分租户 worker（worker 只服务特定法人），需新授权策略（worker→法人绑定表）。**独立开放决策，不随 G9 关闭。** | 分租户部署 | `tmp/delivery-RT1.md` |

**当前没有阻塞阶段 0 的未决项。** 阶段 0 + 阶段 1 可以立即开工。

### 5.1 构建期守卫会被 layer cache 掩盖（2026-08-23 教训）

上表第 6 项的订正值得单列，因为它是一类问题不是一次事故。

W5-1 把 anydoc 的 SRI 校验写进 Dockerfile，本文当时记为「✅ 已定」。实际上那段 `RUN node -e "..."` 把 `$ANYDOC_INTEGRITY` **无引号**插进 JS 源码，生成的是 `if(got!=sha512-Hm4x…==)`——不是字符串字面量，node 每次都 SyntaxError 退出 1。也就是说这个控制项**从引入起就没有比较过任何东西**。

它没被发现的原因是 Docker layer cache：这一层在首次（失败前的某个版本）之后一直命中缓存，从不重跑。2026-08-23 重建镜像时 `node:20-alpine` 的 digest 变了、缓存失效，它才第一次真的执行并暴露。

**这是风险红线 12「假装在检查」长在构建基础设施上的形态。** 勾稽侧我们已经有纪律（恒真即改写、每条要有反向测试），构建期守卫没有对应纪律，而它们更隐蔽——单元测试跑不到，CI 若也命中缓存就同样看不见。

现行结论：

- 修复见 `5e1e6f0`，把期望值先赋给 JS 变量再比较；`Dockerfile.agent-runner` 的失败分支引用未定义变量 `helping` 一并改掉（它 fail-closed 正确，但拿不到 got/want 对照）。
- 钉死的哈希经实证与 0.2.0 tarball 相符，**不存在供应链问题**，只是校验没跑。
- **未决**：需要一条定期用错误输入跑构建期守卫的检查，确认它们还会失败。手动做法已验证可行（`docker build --build-arg ANYDOC_INTEGRITY=sha512-WRONG… --target anydoc-install`，应中止并打印 got/want）。尚未进 CI。

---

## 6. 团队构成对控制设计的约束

本项目只有**一位人类参与者**，其余为 AI 协作者。这不是花絮，它决定了哪类控制是真的、哪类是摆设。

**原则：标准不降，执行方式替换，残余风险如实声明。** 因为"只有一个人"就删掉控制项，等于制造合规假象——比不写更糟。

| 原本依赖第二个人的控制 | 单人替代 |
|---|---|
| 财务负责人签字 | **带日期与理由的决策日志条目**。签字的功能是可追溯，不是人数 |
| 第二人独立标注复核 | **间隔 ≥1 天的盲评自我复核**；一致率报本人两遍之间 |
| AI 交叉标注 | 用**不同模型**独立标注，结果**不计入一致率**，只进待仲裁清单——AI 与实现者失效模式相关，不构成独立性 |
| 第三方复核错误数 | 自评 + 声明"非独立"及偏差方向（**会低估自己的错误**，准确率按上界解读） |
| L3 人工双评分 | 单评分 + rubric + 时间隔离；**AI 作 challenger 专门找问题，不打分** |
| 法务审 AGPL | **不需要**——ADR-0024 直接删依赖。规避比咨询便宜 |
| 季度人力承诺做口径固化 | **第 3 次复用自动建 issue**（ADR-0025 §4） |

**这个事实印证了本套设计的方向。** 单人团队里，任何依赖"有人会注意到"的控制都是脆的；任何进 CI 的控制在你睡觉时也生效。因此这三项的优先级应高于其表面成本：

- **ACORE-2 变异测试** — 替代"同事 review 时会发现你漏挂了中间件"
- **artifact lint fail-closed** — 替代"有人会看出这个数字来源可疑"
- **agentcore import guard** — 替代"code review 会拦住这个 import"

**唯一无法替代的**：你验证不了自己没有系统性盲区。时间隔离抓粗心，抓不住"从一开始就理解错了某个会计口径"——两遍会错得一样。对此唯一实际的缓解是 DEMO 环节：**把底稿拿给真实从业者并主动请他们挑错**。因此 DEMO-1/DEMO-3 不只是业务验收，是本项目**唯一的外部认知纠偏通道**。
