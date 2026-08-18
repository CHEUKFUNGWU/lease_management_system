# AI 文档索引与现行决策

> 末次统一：2026-08-18
> 用途：**读任何一份 AI 文档之前先看这里**，确认它是否仍然有效。
> 维护规则：任何改变下表 §2 决策的变更，必须同时写 ADR 并更新本文；只改方案文档不算数。

---

## 1. 文档状态总表

状态定义：**Current** 现行有效 · **Historical** 已交付的历史记录，正文不再更新 · **Partially Superseded** 部分作废 · **Superseded** 整体被取代。

### 1.1 设计与方案

| 文档 | 状态 | 说明 |
|---|---|---|
| [Agent Core（Go）设计 —— 对齐 pi 架构](Agent_Core_Go设计_对齐pi架构.md) | **Current** | **内核层现行依据**。纯循环 + 中间件链 + 订阅者；ai-service 退役映射 |
| [AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md) | **Current** | **能力层现行依据**。双轨执行、WorkingPaper、不变量、分阶段与验收 |
| [FP&A 与 Finance BP 经营决策及 AI 辅助需求清单](FP&A与Finance_BP经营决策及AI辅助需求清单.md) | Current（清单陈旧） | 业务需求仍有效；§12 的工具勾选状态已过时，以代码为准 |
| [PRD：租赁经营决策与 AI Copilot 平台](PRD_租赁经营决策与AI_Copilot平台.md) | Current | 产品边界与用户故事 |
| [AI Agent 填表升级（tau + anydoc）实施计划](AI_Agent_填表升级_tau_anydoc_实施计划.md) | **Partially Superseded** | **tau 作废**（ADR-0022）；**anydoc 保留并升格**（ADR-0024）；Wave T3/T4 填表缝保留 |
| [CodebaseDesign：Agent 填表升级模块深化](CodebaseDesign_Agent填表_tau_anydoc_模块深化.md) | **Partially Superseded** | 同上 |
| [AI Chat 升级方案（参考 Pi Coding Agent）](AI_Chat_升级方案_pi_agent_参考.md) | **Superseded** | 早期 pi 借鉴，P0 建议已交付；对 pi 的精确映射见 Agent Core 设计 |

### 1.2 已交付的运行时文档

| 文档 | 状态 | 说明 |
|---|---|---|
| [AI Agent 与 CLI 架构演进 PRD](AI_Agent_与_CLI_架构演进_PRD.md) | **Historical** | AG-001~035 已交付（`b1532b4`）。Tool Runtime / Gateway / Capability 契约**仍然有效且不变** |
| [AI Agent 与 CLI 架构演进实施计划](AI_Agent_与_CLI_架构演进实施计划.md) | **Historical** | 同上。§9.1「Runner 不经 shell 调 CLI」的旧结论已被填表计划 D1 推翻 |
| [AI Agent 运行运维手册](AI_Agent_运行运维手册.md) | Current（**待随 W4/W5 修订**） | ai-service 与 `AGENT_PLANNER_TOKEN` 退役后需重写相关章节 |
| [AI Agent 外部验收清单](AI_Agent_外部验收清单.md) | Current | 客户/生产环境签字项 |

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

---

## 3. 当前缺口（对代码的核查结论，2026-08-18）

| 缺口 | 状态 | 归属 |
|---|---|---|
| G1 两个 Agent 平面未汇流（23 处 `Status: "pending"` 静态卡片） | **未解决** | ADR-0022 / W1–W6 |
| G2 无文件分诊，兜底 `return "contract"` | **未解决** | 底稿方案 §6.1，阶段 0 |
| G3 经营数据语义映射 | ✅ **已解决**（Profile 复用与漂移检测未做） | — / 阶段 3 |
| G4 无代码执行能力 | 未解决（**刻意后置**） | ADR-0025 §5，阶段 4 |
| G5 无 xlsx/docx 产出（导出仍是 CSV） | **未解决** | 随 excelize 落地，阶段 0 |
| **G6 PyMuPDF 的 AGPL 风险** | **未解决，与部署形态无关，建议单独立项** | ADR-0024 |

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
| 3 | **protected_measures 清单的财务负责人首次确认** | **阶段 0 数据模型定型** | ADR-0025 §2 |
| 4 | Tier B 产物能否导出给外部审计师 | 阶段 4 | 底稿方案 §15.5 |
| 5 | 固化通道的季度人力承诺 | 长期 | ADR-0025 §4 |
| 6 | anydoc 二进制的供应链管理（版本 + checksum 钉死方式） | W5 | ADR-0024 |
| 7 | `controlledxlsx` 与 excelize 的去留 | W5 | Agent Core 设计 §12.1 |
| 8 | 上下文压缩是否进 W1–W6 | W6 | Agent Core 设计 §12.2 |

**第 3 项是唯一阻塞阶段 0 的**，其余都可以边做边定。
