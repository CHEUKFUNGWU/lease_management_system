# CodebaseDesign：AI 阶段 1（S1 签约前底稿）与 W2（治理中间件链）模块深化

> 文档状态：Draft for Review
> 编制日期：2026-08-19
> 上游依据：[Agent Core（Go）设计](Agent_Core_Go设计_对齐pi架构.md)（W2 波次与 ACORE-2）、[AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md)（阶段 1、G1 门、CORR-1/2）、[第一阶段设计](CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md)（agentcore 内核与 workingpaper 协议已交付）。
> 本文是实施级设计，沿用同一套深模块语言（Interface / Implementation / Seam / Depth / 删除测试）。

---

## 1. 范围与边界

### 1.1 本阶段做什么（W2 + 阶段 1）

| # | 交付物 | 来源 |
|---|---|---|
| M1 | `internal/agentcore/hooks/`：六个 before 中间件（TenantScope → CapabilityCheck → ProtectedMeasure → BudgetGuard → IdempotencyGuard → ReviewGate）+ 三个 after（AuditRecorder / ArtifactCollector / MetricsRecorder）+ `Governance` 链装配 | Agent Core 设计 §6（W2） |
| M2 | `agenttools.Runtime` 接入链：Execute 在注册表解析后走 before 链（行为等价于现 policy.Evaluate），执行后走 after 链的 ArtifactCollector | W2 平价门：既有全量测试 + ACORE-2 |
| M3 | `lease.report.sensitivity` 工具（LevelRead）：包现有 reporting 投影 `projectSensitivity`，补 `/sensitivity` 与 Agent 的断链 | 改造清单〔1〕 |
| M4 | `internal/workingpaper/s1`：S1 底稿构建器——predeal / dealcompare / 折现率冲击重跑 → 带 Certified provenance 的 Paper | 阶段 1 链路 |
| M5 | `lease.working_paper.s1.generate` 工具（LevelDraft，Review.Required）：Agent 产 S1 底稿的正式入口 | 阶段 1 链路 + 不变量 I2/I5 |
| M6 | aiagent 接线：注册两个新工具；`ProjectResult` 把 WorkingPaper 映射为 `working_paper` artifact；确定性触发路径（消息含报价假设 JSON → 调 S1 工具） | 底稿方案 §10、改造清单〔1〕 |
| M7 | 评测扩展：`s1_engine_consistency` category（CORR-1 确定性半边：底稿单元格 == 引擎直接输出）+ 演示语料（仿真报价单 fixtures） | 底稿方案 §12.3 CORR-1 |

### 1.2 本阶段刻意不做

- **W3 订阅者持久化、W4 internal/llm、W5 解析栈切流、W6 agentrunner 收敛**。
- **Draft 级 ReviewGate 的执行前短路**：现行为是「draft 工具先执行、后强制 needs_review」（草稿要先产出再复核）；设计 §6 的「Draft 短路」与现行为冲突，平价优先——**本阶段 ReviewGate hook 只对 LevelCommand 短路，Draft 保持现状**，待 W6 runner 重连时再统一。此偏差记入决策 D-B2。
- **CORR-2 条款抽取语料（≥20 份报价单金标准标注）**：需要人工时间隔离双标注，本阶段只建评测机制与仿真 fixtures，标注协议按底稿方案 §12.7.4 由唯一人类参与者执行，列为剩余项。
- **I2 的 agent_tool_audit 数据库读回**：`internal/repository` 暂无该表读接口（写入侧已有），本阶段 S1 单元格的 `ToolCallID` 指向 S1 工具自身的已审计调用，lint 的 AuditLookup 缝保留；DB 读回接线随 W3 订阅者落地。
- **UI 改动 ≈ 零**：`working_paper` artifact 面板与 xlsx/docx 导出在第一阶段已交付，S1 底稿复用即可；sensitivity 页面本身已存在，本阶段只补 Agent 工具。

---

## 2. 现状核查结论（2026-08-19）

1. **治理现在散在四处**（与设计 §2.2 一致）：`agenttools.Runtime.execute` 内联 `policy.Evaluate`（level/capability/permission/dry-run/幂等键/review）、`recordAudit`（审计+指标）、各 handler 内 `RequireContractAccess`（合同级 scope）。「一次工具调用过哪些闸」不可见。
2. **policy.Evaluate 的精确语义**（平价基准，hooks 必须逐条复刻）：descriptor/call Validate → 上下文必须存在 → 名称/版本一致 → level 启用 → capability 活跃则必须持有该工具 → LevelCommand 额外要求 AllowCommand + capability → permissions 逐一检查 → dry_run 不支持即拒 → **非 read 工具必须有幂等键** → Review.Required 或 (Draft + RequireDraftReview) → RequiresReview。
3. **Review 执行语义**：LevelCommand + RequiresReview → 执行前短路 needs_review；Draft → 执行后状态覆写 needs_review 并回填 ConfirmAction。
4. **predeal 是纯函数包**：`predeal.Build(Draft) (Briefing, error)`，不触库。输出 `balance_sheet.initial_liability/initial_rou/undiscounted_commitment/discounting_effect`、`yearly[]`（interest/depreciation/cash_rent/straight_line_rent/ifrs16_expense/closing_*）、`ebitda_bridge[]`、`exit_curve[]`。**无押金字段、无内建敏感性**——S1 的敏感性节用「折现率冲击重跑 predeal」实现（确定性，无需已签约合同）。
5. **dealcompare 是纯函数包**：`dealcompare.Compare(Input)` → 各 offer 的有效租金/现值/排名/conclusion。
6. **sensitivity 在 reporting 投影层**（`projectSensitivity`，按合同重算 shock 行），`GET /reports/sensitivity` 存在，**Agent 侧零工具**（grep 无命中）。`RouteMeasures` 目前无生产调用方。
7. **决策工具已注册**：`lease.predeal.simulate` / `lease.deal.simulate` / `lease.renewal.simulate`（LevelRead，reports:read），输出 `Data.basis = "Scenario"`（与 workingpaper 的 `Certified` 是两套词汇，S1 构建器负责映射）。
8. **`agent_tool_audit` 只有写入接口**，无 repository 读回方法。
9. **ProjectResult 已有 7 类 artifact 映射范式**（Type/Title/Data/ReviewRequired/…），`working_paper` 映射按同一范式追加即可。

---

## 3. 目标包结构

```
internal/agentcore/hooks/        # W2：治理中间件（本阶段新增）
├─ tenant.go   capability.go   protected_measure.go
├─ budget.go   idempotency.go   review.go
├─ audit.go    artifact.go     metrics.go
└─ governance.go                # Governance(...) → (before, after)，顺序固定
internal/workingpaper/s1/       # 阶段 1：S1 底稿构建器（本阶段新增）
├─ s1.go                        # Build(Input) (workingpaper.Paper, error)
internal/agenttools/tools/       # 增量：sensitivity.go、s1_generate.go
internal/aiagent/                # 增量：工具注册 + 触发路径 + ProjectResult 映射
internal/agentseval/             # 增量：s1_engine_consistency category + 仿真报价 fixtures
```

依赖方向：`hooks` 只依赖 `agentcore` + `agenttools`；`workingpaper/s1` 依赖 `workingpaper` + `services/predeal` + `services/dealcompare`；`agenttools` 不依赖 `hooks`（由 `Runtime` 组合，见 M2）。

---

## 4. 模块设计

### M1 · `internal/agentcore/hooks` —— 治理中间件（W2 核心）

**Interface**

```go
// 每个 before hook 是一个构造器，注入窄依赖，返回 agentcore.BeforeToolCall：
func TenantScope(requirePrincipal bool) agentcore.BeforeToolCall
func CapabilityCheck(policy agenttools.Policy) agentcore.BeforeToolCall
func ProtectedMeasure(resolver MeasureResolver) agentcore.BeforeToolCall
type MeasureResolver interface {
    MeasuresFor(toolName string) []string          // 该工具产出的度量语义
    CertifiedTools() map[string]bool               // 确定性引擎工具白名单
}
func BudgetGuard(b *Budget) agentcore.BeforeToolCall   // 调用数/时长配额；nil 即禁用
func IdempotencyGuard(replay ReplayStore) agentcore.BeforeToolCall  // 非 read 强制键；命中则 Short
type ReplayStore interface { Lookup(ctx, key string) (*agenttools.ToolResult, bool) }
func ReviewGate(requireDraftReview bool) agentcore.BeforeToolCall  // 仅 LevelCommand 短路（D-B2）

// after 钩子：
func AuditRecorder(rec agenttools.AuditRecorder) agentcore.AfterToolCall
func ArtifactCollector(sink ArtifactSink) agentcore.AfterToolCall
type ArtifactSink interface { RecordCertified(ctx, CertifiedCall) }  // tool_call_id/engine_version/input_hash
func MetricsRecorder(m *agenttools.RuntimeMetrics) agentcore.AfterToolCall

// 装配——顺序固定，是本设计的可读性产出：
func Governance(d Deps) (before agentcore.BeforeToolCall, after agentcore.AfterToolCall)
```

**Implementation**：`CapabilityCheck` 内部精确复刻 `policy.Evaluate` 的 level/capability/permission/dry-run 判定（返回同一批 `agenttools` 哨兵错误，Runtime 的错误映射 `policyErrorCode` 不用改）；`TenantScope` 复刻「已认证上下文必须存在」；`IdempotencyGuard` 复刻「非 read 必须带幂等键」+ 可选的 replay 短路；`ProtectedMeasure` 调 `agenttools.RouteMeasures`（阶段 0 已交付）——resolver 无命中即放行，命中受保护度量且工具不在 Certified 白名单 → Block。`Governance` 把六个 before 按设计顺序串成 `ChainBefore`，三个 after 串成 `ChainAfter`。

**Seam**：hook 构造器是注入点（Policy、MeasureResolver、Budget、ReplayStore、Sink 全部由调用方给）；hook 本身无数据库、无 HTTP。**顺序只在 `Governance` 一个函数里可见**——这就是 W2 的产出：新增治理项的挂点只有这里。

**Depth**：调用方学一个 `Governance(deps)`，得到「一次工具调用过哪些闸」的结构性保证；新增一个闸 = 加一个 hook + 一条变异测试，而不是在五个文件里找挂点。

**删除测试**：删掉它，六项治理重新散落回 descriptor/scope/audit/Limits/allowlist 五处，且「少挂一项没人发现」的失效模式复活——ACORE-2 变异测试正是为锁死这一点而存在的。

**验收锚点**：ACORE-2（逐项从链中移除 → 至少一条用例变红）；行为等价 = 既有 agenttools/aiagent/handlers 全量测试保持全绿（M2 接链后）。

### M2 · `agenttools.Runtime` 接链（行为等价）

`RuntimeOptions` 增加 `Governance *agentcore.Governance`（含 before/after）；`NewRuntime` 未显式注入时**内部自组装**默认链（policy 来自 options.Policy、replay 无、budget 无、resolver 无命中），保证生产路径默认走链——既有测试即平价门。`execute()` 中 `policy.Evaluate` 调用点替换为 before 链；`recordAudit` 保持原样（它覆盖所有结局含早期拒绝，与 after 链的 AuditRecorder 不重复装配——after 链在本装配里只含 ArtifactCollector）。**审计/metrics 的单一事实源仍在 Runtime**，after hook 版本供 W3 订阅者路径使用，二者行为一致但不同时挂。

**验收锚点**：`go test ./internal/agenttools/... ./internal/aiagent/...` 全绿（平价）；hooks 包变异测试全绿。

### M3 · `lease.report.sensitivity` 工具（补断链）

**Interface**：输入 `{contract_id, base_rate?, shocks[]}`（与既有 HTTP 接口一致）；LevelRead、reports:read。Handler 经 `SensitivityReader` 缝调用 reporting 投影，输出 `SensitivityRow[]` + `basis: "Certified"` + sources（contract_id/base_rate）。

**Seam**：`SensitivityReader` 接口把 reporting 投影与工具隔离（测试用假实现）。**不新建计算逻辑**——确定性投影已存在，工具只是把它挂进 Agent 白名单。

**删除测试**：删掉它，`/sensitivity` 页面与 Agent 的断链维持原状，Agent 无法解释敏感性数字——改造清单〔1〕明确点名的缺口。

### M4 · `internal/workingpaper/s1` —— S1 底稿构建器

**Interface**

```go
type Input struct {
    Draft          predeal.Draft          // 关键假设（折现率等必须人工确认后传入）
    Offers         []dealcompare.Offer    // 可比报价（可选，≥2 时出对比节）
    ShocksPercent  []float64              // 折现率敏感性冲击（可选）
    ConfirmedBy    string                 // HumanInput provenance 需要
    ConfirmedAt    string
    ToolCallID     string                 // 本次 S1 工具调用的已审计 call id（I2 锚点）
    EngineVersion  string                 // "lease.working_paper.s1.generate@v1"
}
func Build(in Input) (workingpaper.Paper, error)
```

**Implementation**：

- **IFRS 16 影响节**：`predeal.Build(in.Draft)` → 单元格映射（初始负债→`lease_liability`、初始 ROU→`rou_asset`、折现率→`discount_rate_applied`、各年 interest→`interest_expense`、depreciation→`rou_depreciation`、期末余额→`lease_liability`/`rou_asset`），全部 `BasisCertified` + `ToolCallID=in.ToolCallID` + `EngineVersion`。**引擎输出即单元格值，构建器不重算、不舍入**（CORR-1 的确定性半边）。
- **EBITDA 桥节**、**退出曲线节**：同源映射。
- **敏感性节**：对每个 shock 用 `predeal.Build(draft±shock)` 重跑 → 初始负债行（Certified，engine_version 注明 "predeal-shock"）。**不用 /reports/sensitivity**（那需要已签约合同；签约前场景用引擎重跑，是同一口径的确定性计算）。
- **假设清单节**：`BasisHumanInput`（ConfirmedBy/ConfirmedAt）+ 假设本身；**折现率缺失 → Build 直接报错**（predeal 校验已强制 DiscountRate>0，延续「AI 不得猜折现率」）。
- **数据缺口**：诚实列出——押金未建模（predeal.Draft 无此字段）、变量租金未建模、组合级加权平均折现率不适用等。
- **Exploratory 单元格 = 0**：S1 是纯 Tier A，全表无 Exploratory——不变量 I3/I5 天然满足，lint 仍全量执行（fail-closed 不因「应该干净」而跳过）。

**Seam**：引擎是注入的（`BuildWith(engines)` 内部 seam，测试可替换为 stub 验证「构建器不重算」）；对外只有 `Build`。**I2 锚点**：所有 Certified 单元格共用一个 `ToolCallID`（S1 工具自身的已审计调用）——审计表里该调用 completed 即可通过 lint 交叉比对。

**Depth**：调用方给一份已确认的假设，得到一份每个数字可追溯到引擎版本与审计调用的完整底稿；字段映射、provenance 装配、缺口诚实声明全部藏在后面。

**删除测试**：删掉它，predeal 字段→受保护度量的映射、provenance 装配、I3 合规会在每个未来调用点（Agent 工具、导出、DEMO）各写一份且必然漂移。

### M5 · `lease.working_paper.s1.generate` 工具

LevelDraft、Review.Required（草稿级产物，进 Review Gate）、SupportsIdempotency。Handler：严格解码 `s1.Input` → `s1.Build` → `ToolResult{Data: {paper, side_effects:false}}`。**不写任何业务表**（写的是 artifact，走 aichat 的 artifact 入库 + 人工复核）。

**验收锚点**：Review Gate 生效（tool 执行后 status=needs_review）；`basis=Exploratory` 单元格为 0（I5 无违规面）；幂等键必填。

### M6 · aiagent 接线

1. `newAgent` 注册 `lease.report.sensitivity`（需 reporting 投影依赖）与 `lease.working_paper.s1.generate`。
2. **确定性触发路径**：消息含报价假设 JSON（`"draft"` 键且 `"commencement_date"` 出现）→ 直接调 S1 工具（不走 LLM 猜测），tool_end 事件照常发出——延续零售路径的确定性 fallback 纪律。
3. `ProjectResult`：`response.WorkingPaper != nil` → `ArtifactDraft{Type: working_paper, Title: "S1 签约前决策底稿", ReviewRequired: true, Data: paper, RuleVersion: "s1-paper-rule.v1"}`。前端复用第一阶段的面板与导出。
4. `Response` 增加 `WorkingPaper *workingpaper.Paper json:"working_paper,omitempty"`。

**验收锚点**：一条「签约前底稿」消息走通：工具执行 → needs_review → artifact 面板渲染 → 导出 xlsx 通过 lint（OPS-5 的部分语义）。

### M7 · 评测扩展

- `agentseval` 新增 category `s1_engine_consistency`：用例携带 S1 Input → `s1.Build` 的单元格 vs `predeal.Build`/`dealcompare.Compare` 直接输出逐字段相等（CORR-1 的确定性半边，纯 Go 断言，进 CI）。
- 仿真报价 fixtures（2 份，公开招租信息风格的 JSON）入库 `agentseval/testdata/`，作为 DEMO-1 素材与回归夹具；DEMO-1~3 的人工演示仍属人类参与者工作。
- CORR-2 标注语料：**剩余项**（§1.2）。

---

## 5. 关键决策记录

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D-B1 | W2 平价优先：ReviewGate 只对 LevelCommand 短路；Draft 维持「先执行后 needs_review」 | 草稿必须先产出才能复核；设计 §6 的 Draft 短路与现行为冲突，等 W6 统一 | 2026-08-19 |
| D-B2 | Runtime 接链时 after 链只挂 ArtifactCollector；审计/metrics 仍走 Runtime 原路径 | 审计必须覆盖早期拒绝结局，链式 after 只到得了「已执行」分支；双挂会重复记录 | 2026-08-19 |
| D-B3 | S1 敏感性节用「折现率冲击重跑 predeal」而非 `/reports/sensitivity` | 后者需要已签约合同；签约前场景重跑同一引擎 = 同口径确定性计算 | 2026-08-19 |
| D-B4 | 所有 Certified 单元格共用 S1 工具调用的 CallID 作 I2 锚点；EngineVersion 用工具名@版本 | predeal 在工具内部执行，审计表只记录 S1 工具调用；锚点指向审计事实，不虚构子调用 | 2026-08-19 |
| D-B5 | `agent_tool_audit` 读回接口不在本阶段建（写入侧已有，读侧随 W3） | 阶段 1 无 Tier B、审计表读回仅服务于 I2 校验，AuditLookup 缝已隔离成本 | 2026-08-19 |

---

## 6. 任务清单与验收映射

| 序 | 任务 | 产出 | 验收锚点 |
|---|---|---|---|
| 1 | hooks 六前 + 三后 + Governance | hooks 包 + 单测 | 每个 hook 独立用例 |
| 2 | Runtime 接链 | RuntimeOptions.Governance + execute 改造 | 既有 agenttools/aiagent/handlers 全量测试绿（平价门） |
| 3 | ACORE-2 变异测试 | 逐项移除 hook → 用例变红 | 9 个 hook 全部覆盖 |
| 4 | sensitivity 工具 | tools/sensitivity.go + 测试 | 断链补齐；LevelRead |
| 5 | s1 构建器 | workingpaper/s1 + 测试 | CORR-1 半边；I3 全 Certified |
| 6 | s1 generate 工具 | tools/s1_generate.go + 测试 | Review Gate；幂等键 |
| 7 | aiagent 接线 | 注册/触发/ProjectResult + 测试 | 端到端 needs_review + artifact |
| 8 | 评测扩展 | s1_engine_consistency + fixtures | 新 category 全绿 |
| 9 | 全量验证 | go test/vet + web type-check/build/test | AGENTS.md 验证节 |

## 7. 落地顺序

依赖驱动：**1 → 3 → 2 → 4 → 5 → 6 → 7 → 8 → 9**（先建 hook 与变异测试再接线，平价门随时可退）。
