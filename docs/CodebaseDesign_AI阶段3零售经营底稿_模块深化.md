# CodebaseDesign：AI 阶段 3（零售经营底稿）模块深化

> 文档状态：Draft for Review
> 编制日期：2026-08-19
> 上游依据：产品定位（AI 文档索引 §0：**Agent 的主战场是经营分析**，底稿场景以零售经营为第一优先）、底稿方案 §7（WorkingPaper 协议，已交付）、AGENTS.md 零售约束（store-day 粒度、retail-kpi-v1 语义、指标后端计算、不用 0 填补、覆盖不足显式降级）。
> 背景：阶段 2 曾按租赁时代的旧编排做了 S1 签约前底稿（IFRS 16 主题）。本阶段起**底稿主线切换为零售经营**——S1 机制保留为合规模块资产，不再扩展租赁主题底稿（S4/S3/S2 后移，2026-08-19 产品决策）。
> 本阶段的 Paper 全部数字来自三个已交付的零售确定性引擎（retailpulse / retailstore360 / retailscenario），零新计算逻辑、零 Exploratory 单元格。

---

## 1. 范围与边界

### 1.1 本阶段做什么

| # | 交付物 |
|---|---|
| M1 | `internal/workingpaper/retail`：零售经营底稿构建器——三个引擎 Response → Paper（全 Certified/SystemFact 单元格 + 诚实缺口） |
| M2 | `retail.working_paper.store.generate` 工具（LevelDraft + Review Gate + 幂等键），复用 scope 包装层 |
| M3 | aiagent 触发接线：消息含「底稿」意图 + 有效 filters → 工具 → working_paper artifact（复用现有面板与导出） |
| M4 | 评测：builder 1:1 保值断言（包测）+ agentseval `retail_paper_sanctity` 用例（最小脉冲 fixture：nil 值不产单元格、lint 全过、零 Exploratory） |
| M5 | CLI `run events --format table|ndjson`（阶段 2 承诺的 Lark 输出契约补件） |

### 1.2 本阶段刻意不做

- **不做任何零售口径计算**：单元格值 = 引擎 Response 字段原值（含 nil 跳过）。指标语义仍归 retail-kpi-v1 管。
- **不改三个既有零售工具**（Contract 不变，平价门为既有零售测试全绿）。
- **不做 page_fill 填表缝**（优先级 2，下一阶段）、**不做两平面汇流**（优先级 3）。
- **不触碰 protected_measures**：零售 KPI（revenue/gross_profit/footfall/occupancy 等）不在 10 项保护清单内；占用成本口径 ≠ IFRS 16 口径（AGENTS.md），底稿只呈现经营口径。
- Agent 侧仍无 CallID 传递（`AgentToolCall` 缺 CallID 字段）：纸工具的 Certified 单元格用**纸工具自身的已审计 CallID** 作统一 I2 锚点（同 S1 的决策 D-B4），engine_version 用响应里的真实版本字段（pulse_version / diagnostics_version / scenario_version）。

---

## 2. 现状核查结论（2026-08-19）

1. **共享 seam**：`RetailOperationsReader.QueryFacts(ctx, legal, from, to, class, dataset, source, storeIDs)`；tools 包内有 `scopedRetailReader`（权限 scope 过滤 + 来源元数据重建）——**纸工具必须走它**，不可绕过权限再自建。
2. **引擎输出**（§详见勘察报告）：
   - pulse：`Summary`（14 个 KPI id，Current/Comparison/ChangeValue/ChangeType/ChangeMarginPP）、`SSSG`、`Attention`、`Coverage`、`DecisionReady(+Reason)`、`Envelope`（fact version 范围、highest_as_of、semantic_version）。
   - store360：14 键 Summary（含绝对成本 3 键）、`PeerBenchmark`（p25/p50/p75）、3 个 `Bridges`（含 rounding_residual）、`Observations`（叙述）、`DataQualityIssues`、`minimum_peer_count`（=3）。
   - scenario：`Baseline`（单个 ScenarioResult）+ `Scenarios`（plan 等），每方案 12 个 metric（Baseline/Result/Delta）+ 2 个贡献变化 + Bridge；`Assumptions` 七项为唯一合法 HumanInput 数值来源；`review_required=true`、`official_impact=false`、`ifrs16_impact=false` 是红线承诺字段。
3. **诚实性信号**（→ DataGaps）：`decision_ready=false(+reason)`、`CoverageIncomplete`、`MissingFields`、`SuppressedAttention.Reasons`、`DataQualityIssues`（currency_conflict / insufficient_peer_count）、`multi_currency`、`plan==nil`、simulation 分类、`*float64==nil`（永不填 0）。
4. **既有零售工具的 SkillID 闸**（retailReadDefinition 要求 SkillID=retail_operations）：纸工具直接调 service（同包内 scope 包装），不吃这条闸。
5. **lint 契约**：`measure_id` 为空不失败；HumanInput 合规（带 ConfirmedBy/At）；零售 KPI 无 I3 触点；I2 唯一硬约束 = Certified 带 ToolCallID 且审计 completed。
6. **前端零改动**：working_paper artifact 面板 + xlsx/docx 导出在阶段 1 已交付，引擎版 S1 的 Response.WorkingPaper → ProjectResult → artifact 链路现成。

---

## 3. 模块设计

### M1 · `internal/workingpaper/retail` —— 底稿构建器（纯映射）

**Interface**

```go
type Input struct {
    Pulse           *retailpulse.Response
    Diagnostics     *retailstore360.Response
    Scenario        *retailscenario.Response
    Assumptions     retailscenario.Assumptions // plan 假设（HumanInput）
    ConfirmedBy     string
    ConfirmedAt     string
    ToolCallID      string                    // I2 统一锚点（纸工具自身已审计调用）
    AttentionLimit  int
}
func Build(in Input) (workingpaper.Paper, error)
```

**Implementation**：

- **scope 节**（table，SystemFact cells）：data_classification、dataset_version、source_systems、fact_version_min/max、highest_as_of、currency(+status)、period_label、store 身份（store_id/code/name/brand/region）、同行定义（peer_definition/minimum_peer_count）。Provenance：`SourceTable: "store_operating_facts"`、`DataVersion: dataset_version`。
- **经营脉搏节**（table，Certified，engine=response.PulseVersion）：14 个 Summary KPI × {Current、Comparison、ChangeValue（+store_contribution 的 ChangeMarginPP）}。`Value==nil` → **跳格**（不产 0）。Unit 取 KPIValue.Unit，Currency 仅当 `Response.Currency != ""` 且非 conflict/unknown 时填写。
- **同店销售节**：SSSG 存在才产（Certified）。
- **关注门店节**：Attention 前 N 条（rank/store/score/severity/top signal change），Certified。
- **诊断节**（Certified，engine=response.DiagnosticsVersion）：Summary 各键同上规则；PeerBenchmark（target/median/p25/p75/percentile, status!=complete → 该行跳 + gap）；3 个 Bridge（current/comparison/total_change/items/rounding_residual）；Observations 的 statement/reference 进 Narrative。
- **情景节**（Certified，engine=response.ScenarioVersion）：Baseline 各 metric 的 Result；每个 plan 方案的 metric Result+Delta + monthly/horizon contribution + bridge items；**假设清单**（七项 Assumptions，HumanInput + Confirmed）单独一节。
- **DataGaps 组装**（诚实大于好看）：
  - `decision_ready=false` → gap 附 reason（pulse 与 diagnostics 各自的）
  - `CoverageIncomplete` / `MissingFields` / `SuppressedAttention.Reasons` / `DataQualityIssues` 逐条
  - `multi_currency` 或 mixed_currency_stores>0 → "多币种数据未经折算，底稿不得跨币种加总"
  - classification=simulated → "数据标记为模拟（SIMULATED），不得用作正式结论"
  - plan==nil → "该期间无 FP&A 计划版本，计划对比缺失"
  - 无 store_id → "仅组合经营脉搏；门店诊断与情景需 store_id"
  - scenario 缺失（被阻断语义跳过）→ 附原因
- **UnexplainedResidual**：bridge 的 `rounding_residual` 逐桥列出（残差显式保留，不允许分摊）。

**Seam**：构建器只吃规范化的 Response 结构（值对象），不 import repository/agenttools/tools——纯映射可单测锁死。**输入值就是输出值**：任何重算、舍入、汇率折算都会让 1:1 断言变红。

**Depth**：调用方给三个 Response + 确认信息，得到一份「每个数字能追到引擎版本与审计调用、每个缺口都被点名」的完整底稿。字段映射、nil 纪律、缺口组装、I3 合规全部藏在后面。

**删除测试**：删掉它，pulse/360/scenario 三套字段映射与诚实性规则会在 Agent 工具、导出、DEMO 三处各写一份且必然互相漂移。

### M2 · `retail.working_paper.store.generate` 工具

- 输入：`retailContextArguments`（as_of/window_days/classification/dataset_version/source_system/store_ids/attention_limit）+ `store_id` + `horizon_months` + `assumptions` + `confirmed_by/at`（缺则由 auth principal 补）。
- Handler：`retailQuery` 复用校验 → scopedRetailReader 下依次调 pulse（store_ids 收窄为 [store_id] 或原样）→ 若 pulse 不充分且带 store_id 仍调 diagnostics（记录缺口）→ **仅当 pulse 充分且 diagnostics decision_ready 时调 scenario**（镜像聊天路径的阻断语义，原因写入 gap）→ stamp ToolCallID/Confirmed → `retail.Build` → **先 Lint（own call）再出 data**（fail-closed）。
- Descriptor：LevelDraft、Review.Required（reasons: retail_paper_review / assumptions_human_confirmed）、Permissions `reports:read`、SupportsIdempotency、Timeout 60s。

### M3 · aiagent 触发

- `extractRetailPaperIntent(message)`: 消息含「底稿」「经营底稿」「出个底稿」任一 + 存在合法 filters（as_of + data_classification + window_days）→ 执行。
- filters 复用既有 `retailFilters`（page_context + 消息正则兜底）——不改零售工具。
- 响应：`Response.WorkingPaper` + `ReviewPrompts`；`ProjectResult` 的 working_paper 映射复用现成。
- store-360 场景（page_context 带 store_id）自然命中 → FPGA/BP 顺手拿店级底稿。

### M4 · 评测

- 包测 1:1 保值：构造三类 Response（含 nil 字段、currency_conflict、suppressed attention、bridges residual）→ 断言单元格值 === 输入值、nil 不产格、gaps 全出现、lint 全过、零 Exploratory。
- agentseval 新用例 `retail_paper_sanctity`：最小 pulse fixture JSON → builder → 同上断言（Category `retail_paper`，CI 常驻）。

### M5 · CLI 输出契约补件

`lease-agent run events` 增加 `--format json|table|ndjson`：table 用 tabwriter 列 No/事件类型/终态；ndjson 每行一个事件 JSON。其余命令保持 JSON。commit 边界不变。

---

## 4. 关键决策记录

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D-C1 | 底稿主线切换零售：S4/S3/S2 租赁主题底稿后移；S1 保留不移除 | 产品定位（索引 §0）：底稿场景以零售经营为第一优先；用户拍板 | 2026-08-19 |
| D-C2 | 零售底稿单元格 basis：引擎计算值=Certified（engine_version 用响应真实版本字段），范围/版本/覆盖元数据=SystemFact，情景假设=HumanInput | I2 锚点唯一、不虚构子调用；版本随引擎响应走，杜绝硬编码漂移 | 2026-08-19 |
| D-C3 | 场景执行镜像聊天阻断语义（pulse 充分且 diagnostics ready 才跑 scenario），但底稿不失败——缺什么记什么 | 数字口径与聊天一致（OPS-5/CORR-1），底稿是记录文档，诚实降级优于拒绝 | 2026-08-19 |
| D-C4 | 纸工具直接调三个 service（复用 scopedRetailReader），不经过既有零售工具的 SkillID 闸 | scope 过滤不能绕；SkillID 闸是 chat-plane 语义，纸工具是独立入口 | 2026-08-19 |
| D-C5 | 本阶段不建 page_fill 与两平面汇流 | 已定的优先级 2/3，避免同阶段范围爆炸 | 2026-08-19 |

---

## 5. 任务-验收映射

| 序 | 任务 | 验收锚点 |
|---|---|---|
| 1 | workingpaper/retail 构建器 + 包测 | 1:1 保值、nil 纪律、gaps、lint、零 Exploratory（CORR-1 风格断言） |
| 2 | retail.working_paper.store.generate 工具 + 测试 | Review Gate、幂等键、scope 过滤、fail-closed lint |
| 3 | aiagent 触发 + 注册 + 测试 | 消息「底稿」+ filters → WorkingPaper → artifact；无 store_id 也出组合底稿 |
| 4 | agentseval `retail_paper_sanctity` + CLI --format | 新用例全绿；CLI 测试 |
| 5 | 全量验证 | core-service test/vet、web 回归、评测 harness 全绿 |

## 6. 落地顺序

**1 → 2 → 3 → 4 → 5**
