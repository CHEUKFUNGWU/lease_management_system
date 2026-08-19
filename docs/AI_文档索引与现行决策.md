# AI 文档索引与现行决策

> 末次统一：2026-08-18
> 用途：**读任何一份 AI 文档之前先看这里**，确认它是否仍然有效。
> 维护规则：任何改变下表 §2 决策的变更，必须同时写 ADR 并更新本文；只改方案文档不算数。

---

## 0. 产品定位（所有 AI 工作的前提）

**产品是「线下零售门店经营分析工作站」。主线用户是零售 Finance BP 与集团 FP&A。**

```
主线（产品是什么）           次要（value-added 模块）
─────────────────────    ────────────────────────
门店 store-day 经营事实   →  IFRS 16 / CAS 21 租赁计量
四墙损益 · 租售比 · 坪效  →  月结、分录、披露、审计包
异常下钻 · 情景 · 行动闭环 →  合同台账与关键日期
FP&A 版本治理与滚动预测
```

**IFRS 16 是次要的 value-added 合规能力，不是产品定位。** 它的战略价值在两处：作为合规刚需的销售入口，以及让租约条款、面积、门店主数据先进系统——**这些数据不在客户的湖仓里，任何通用 BI 都分析不了不存在的数据**。

对 AI 工作的三条直接约束：

1. **Agent 的主战场是经营分析**，不是合同录入。技能路由、评测集、底稿场景都以零售经营为第一优先。
2. **IFRS 16 在 AI 设计中的角色是「被保护对象」**——见 [ADR-0025](adr/0025-separate-certified-engine-output-from-exploratory-analysis.md) 的受保护度量清单。保护它，不是围绕它做产品。
3. **制造 / 设备 / 工厂维度不在当前范围。** 相关内容在需求清单 §9 保留供将来评估，但不进排期、不作为架构约束。

写文档、定技能、排优先级时若与本节冲突，以本节为准。

---

## 1. 文档状态总表

状态定义：**Current** 现行有效 · **Historical** 已交付的历史记录，正文不再更新 · **Partially Superseded** 部分作废 · **Superseded** 整体被取代。

### 1.1 设计与方案

| 文档 | 状态 | 说明 |
|---|---|---|
| [Agent Core（Go）设计 —— 对齐 pi 架构](Agent_Core_Go设计_对齐pi架构.md) | **Current** | **内核层现行依据**。纯循环 + 中间件链 + 订阅者；ai-service 退役映射 |
| [AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md) | **Current** | **能力层现行依据**。双轨执行、WorkingPaper、不变量、分阶段与验收 |
| [CodebaseDesign：AI 阶段 0 与 W1 模块深化](CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md) | **Current** | **实施级设计**。W1（agentcore）+ 阶段 0（workingpaper / docparse / triage / CLI / Web / 评测）的深模块接口、seam、决策留痕 D-A~G、验收映射 |
| [CodebaseDesign：AI 阶段 1 与 W2 模块深化](CodebaseDesign_AI阶段1与W2_模块深化.md) | **Current** | **实施级设计**。W2（治理中间件链 + ACORE-2 变异测试）+ 阶段 1（sensitivity 工具、S1 签约前底稿构建器与工具、aiagent 接线、CORR-1 评测）的接口与决策留痕 D-B1~B5 |
| [CodebaseDesign：AI 阶段 3 零售经营底稿模块深化](CodebaseDesign_AI阶段3零售经营底稿_模块深化.md) | **Current** | **实施级设计**。零售经营底稿（收尾底稿主线的产品决策 D-C1~C5）：retailpulse/store360/scenario 引擎 → 全 Certified 经营底稿 |
| [FP&A 与 Finance BP 经营决策及 AI 辅助需求清单](FP&A与Finance_BP经营决策及AI辅助需求清单.md) | Current | 业务需求有效。**§9 制造功能已标注为不在当前范围**；§12 的工具勾选表已移除，实现状态以代码为准 |
| ~~PRD：租赁经营决策与 AI Copilot 平台~~ | **已归档** `archive/superseded-prds-2026-08/` | 编于零售转型前，标题与内容以「租赁」为主体；产品边界已由零售 PRD (P0–P5) 与财务BP/FP&A PRD (F0–F9) 取代 |
| ~~AI Agent 填表升级（tau + anydoc）实施计划~~ | **已归档** `archive/ai-runtime-2026-08/` | tau 作废（ADR-0022）；anydoc 与填表缝**已迁入** Agent Core 设计 §8.2 与附录 A |
| ~~CodebaseDesign：Agent 填表升级模块深化~~ | **已归档** 同上 | 同上（M1 → §8.2，M4 → 附录 A） |
| ~~AI Chat 升级方案（参考 Pi Coding Agent）~~ | **已归档** 同上 | 早期 pi 借鉴，P0 建议已交付；精确映射见 Agent Core 设计 |

### 1.2 已交付的运行时文档

| 文档 | 状态 | 说明 |
|---|---|---|
| [AI Agent 与 CLI 架构演进 PRD](AI_Agent_与_CLI_架构演进_PRD.md) | **Historical** | AG-001~035 已交付（`b1532b4`）。Tool Runtime / Gateway / Capability 契约**仍然有效且不变** |
| ~~AI Agent 与 CLI 架构演进实施计划~~ | **已归档** `archive/ai-runtime-2026-08/` | 交付记录，1816 行；对外契约以保留的 PRD 为准 |
| [AI Agent 运行运维手册](AI_Agent_运行运维手册.md) | Current（**待随 W4/W5 修订**） | ai-service 与 `AGENT_PLANNER_TOKEN` 退役后需重写相关章节 |
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
| [**0022**](adr/0022-first-party-go-agent-core-modelled-on-pi.md) | **自研 Go Agent Core，架构对齐 pi** | **Accepted（新）** |
| [**0023**](adr/0023-retire-the-first-party-python-ai-service.md) | **退役自研 Python ai-service** | **Accepted（新）** |
| [**0024**](adr/0024-remove-the-agpl-pdf-dependency.md) | **移除 AGPL 的 PyMuPDF，解析按证据需求分流** | **Accepted（新）** |
| [**0025**](adr/0025-separate-certified-engine-output-from-exploratory-analysis.md) | **双轨执行与受保护度量红线** | **Accepted（新）** |

### 1.5 归档区

全部历史文档在 `docs/archive/`，**均带 ARCHIVED 横幅，永不作为现行依据**（规则见 AGENTS.md「文档归档规则」，由 `scripts/check_docs_archive.sh` 在 CI 强制）。

| 目录 | 份数 | 内容 |
|---|---|---|
| `ai-runtime-2026-08/` | 5 | AI/CLI 实施计划、外部验收清单、pi 参考方案、tau 计划与其模块深化 |
| `strategy-inputs-2026-08/` | 2 | 零售转型可行性报告、FP&A 产品战略分析 |
| `uiux-reviews-2026-08/` | 3 | UIUX 评估报告、零售 UIUX 评审、架构改善方案 |
| `ifrs16-mvp-2026-05/` | 2 | IFRS16 IT 需求文档、MVP 技术架构方案 |
| `pre-retail-roadmap/` | 3 | 零售转型前的路线图与任务清单 |
| `uiux-overhaul-2026-08/` | 3 | 已完结的 UIUX 交付流程工单 |
| `superseded-lists/` | 3 | 被取代的清单与 AGENTS 历史章节 |
| `ui-upgrade-2026-05/` | 4 | 2026-05 的 UI 升级 |
| `project-history/` | 1 | 2026-05 缺陷清单 |

**2026-08-18 的约定变更**：此前 `docs/archive-local/` 被 gitignore、只存本机，目的是不让过期文档污染协作者上下文。现已全部迁回跟踪式归档——本地归档有三个代价（机器丢失即消失、其他 clone 与 worktree 看不到、git 历史里的删除文件没人会想到去找），而它防的那件事由**横幅 + AGENTS.md 规则 + CI 守卫**替代，效果更好且不丢副本。

---

## 2. 现行决策登记

每条决策标注**决定了什么**、**推翻了什么**、**留痕在哪**。

| # | 决策 | 推翻的旧结论 | 留痕 |
|---|---|---|---|
| **D1** | Agent 内核自研 Go（`internal/agentcore`），**架构对齐 pi**，不引入 tau、不引入 pi 本体 | 填表计划 D1/D2/D4「引入 tau 作为大脑」 | ADR-0022 |
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

---

## 3. 当前缺口（对代码的核查结论，2026-08-18，2026-08-19 更新）

| 缺口 | 状态 | 归属 |
|---|---|---|
| G1 两个 Agent 平面未汇流（23 处 `Status: "pending"` 静态卡片） | **未解决** | ADR-0022 / W1–W6 |
| G2 无文件分诊，兜底 `return "contract"` | 🟡 **部分解决**：`lease.file.triage` 确定性分诊落地、`return "contract"` 兜底已删除、域外文件（发票/劳动合同/宣传册）显式拒绝并问用户；LLM 分类器与 ≥50 份语料的 CORR-6 完整判据待 L2 语料建设 | 底稿方案 §6.1，阶段 0 |
| G3 经营数据语义映射 | ✅ **已解决**（Profile 复用与漂移检测未做） | — / 阶段 3 |
| G4 无代码执行能力 | 未解决（**刻意后置**） | ADR-0025 §5，阶段 4 |
| G5 无 xlsx/docx 产出（导出仍是 CSV） | 🟡 **部分解决**：`workingpaper` xlsx/docx 确定性渲染器 + lint 门 + `GET /ai/chat/artifacts/:id/export` 端点已落地；端到端 WorkingPaper 生成链路（S1 底稿）未接线 | 阶段 0 → 阶段 1 |
| **G6 PyMuPDF 的 AGPL 风险** | **未解决，与部署形态无关，建议单独立项** | ADR-0024 |

> W1 + 阶段 0 已按 [CodebaseDesign 模块深化](CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md) 交付：`internal/agentcore`（纯循环内核 + ACORE-1/5/6/8 测试）、`protected_measures`（10 项 + 词法探针）、`internal/workingpaper`（I1/I2/I3/I6 lint + 封面 + 渲染）、`internal/docparse`（CSV/anydoc/PaddleOCR）、`internal/agentseval`（不变量与 triage 用例，harness 第三段 `invariants`）、CLI 三层命令（commit 只对人）、Web（去关键词猜测、tool_start 消费、working_paper 渲染）。
>
> **W2 + 阶段 1 已按 [CodebaseDesign：AI 阶段 1 与 W2](CodebaseDesign_AI阶段1与W2_模块深化.md) 交付（2026-08-19）**：`agentcore/hooks` 六前 + 三后治理中间件与 `Governance` 固定顺序链、ACORE-2 变异测试（9 项全锁）、`agenttools.ExecutionGuard` seam + aiagent 全路径接链（平价门：既有测试保持全绿）、`lease.report.sensitivity` 工具（补 /sensitivity 断链，共用同一 reporting 投影）、`workingpaper/s1` 构建器（predeal/dealcompare/shock 重跑 → 全 Certified 单元格，I2 锚点为工具自身已审计调用）、`lease.working_paper.s1.generate`（LevelDraft + Review Gate）、aiagent 确定性触发（消息带确认假设块 → 底稿 → working_paper artifact）、评测新增 `s1_engine_consistency` category（CORR-1 确定性半边 + 2 份仿真报价 fixtures，harness 12/12）。
>
> **阶段 3（零售经营底稿，产品主线的底稿）已按 [CodebaseDesign：AI 阶段 3](CodebaseDesign_AI阶段3零售经营底稿_模块深化.md) 交付（2026-08-19）**：底稿主线切回零售（D-C1：S4/S3/S2 后移，S1 保留）；`workingpaper/retail` 构建器（pulse/store360/scenario → 全 Certified/SystemFact 单元格，1:1 保值断言锁定、nil 跳格不填 0、覆盖不足/多币种/模拟标识/抑制信号一一进 DataGaps、残差显式保留）；`retail.working_paper.store.generate` 工具（LevelDraft + Review Gate，复用 scopedRetailReader 权限过滤，情景镜像聊天阻断语义 D-C3）；aiagent「底稿 + filters」确定性触发 → working_paper artifact（复用面板与 xlsx/docx 导出）；评测新增 `retail_paper_sanctity` category（harness 13/13）；CLI `run events --format table|ndjson`。core-service `go test ./...` + `go vet ./...`、web 回归全绿。

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
| 6 | anydoc 二进制的供应链管理（版本 + checksum 钉死方式） | W5 | ADR-0024 |
| 7 | `controlledxlsx` 与 excelize 的去留 | W5 | Agent Core 设计 §12.1 |
| 8 | 上下文压缩是否进 W1–W6 | W6 | Agent Core 设计 §12.2 |

**当前没有阻塞阶段 0 的未决项。** 阶段 0 + 阶段 1 可以立即开工。

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
