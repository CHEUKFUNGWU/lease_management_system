# CodebaseDesign：AI 阶段 0（产物底座）与 W1（agentcore 内核抽取）模块深化

> 文档状态：Draft for Review
> 编制日期：2026-08-19
> 上游依据：[Agent Core（Go）设计](Agent_Core_Go设计_对齐pi架构.md)（W1–W6 波次与 ACORE-1~9）、[AI 底稿与 Paperwork Agent 设计方案](AI_底稿与Paperwork_Agent设计方案.md)（阶段 0 交付物、不变量 I1–I8、G0 门）、ADR-0022 / 0023 / 0024 / 0025。
> 本文是实施级设计：把两份方案文档翻译成**包结构、接口、seam 与验收映射**，按 [codebase-design 技能](https://github.com/cheukfungwu/skills) 的深模块语言编写（Interface / Implementation / Seam / Depth / 删除测试）。

---

## 1. 范围与边界

### 1.1 本阶段做什么（W1 + 阶段 0）

| # | 交付物 | 来源 |
|---|---|---|
| M1 | `internal/agentcore`：纯循环内核（State / Event / Loop / Agent / queue / 闸点类型） | Agent Core 设计 §4–5（W1） |
| M2 | `internal/agenttools/protected_measures.go`：10 项受保护度量 + 中英词法探针 + 请求期路由函数 + 产物期 lint | 底稿方案 §4.1–4.3（阶段 0 实现，W2 接中间件） |
| M3 | `internal/workingpaper`：WorkingPaper 协议 + fail-closed lint（I1/I2/I3/I6）+ 封面页 + xlsx/docx 渲染器 | 底稿方案 §7（阶段 0） |
| M4 | `internal/docparse`：DocumentParser 接口 + CSV 确定性解析 + anydoc 子进程适配器 + PaddleOCR Go 客户端 + 四类解析错误分类 | 底稿方案 §6.2、ADR-0024（阶段 0 骨架，W5 完成切流） |
| M5 | `doc.triage` 工具 + `aiagent` 文件路由改走 triage，**移除 `return "contract"` 兜底** | 底稿方案 §6.1（阶段 0） |
| M6 | `working_paper` artifact 类型 + xlsx/docx 导出端点 | 底稿方案 §7.2、G5 |
| M7 | CLI 三层命令：`retail import preview\|commit`、`scenario evaluate\|save` 领域捷径 + `--format json\|table\|ndjson` | 附录 A.4、lark-cli 三层借鉴 |
| M8 | Web：移除 `inferUploadTaskType` 关键词猜测；前端消费 `tool_start`；`working_paper` artifact 渲染 + 导出链接 | 底稿方案 §10、改造清单〔0〕 |
| M9 | 评测扩展：`provenance`（working_paper_lint）与 `triage_refusal` 两类用例注入既有 `agent-evaluation.v1.json` | 底稿方案 §12.7.2 L1 |

### 1.2 本阶段刻意不做（防蔓延）

- **W2 中间件链的完整装配**（TenantScope/CapabilityCheck/BudgetGuard 等 hook 实现留到 W2；本阶段只做闸点类型 + Chain 组合子 + ProtectedMeasure 的判定函数）。
- **W3 订阅者持久化改造**、**W4 internal/llm**、**W5 解析栈切流与 ai-service 退役**、**W6 agentrunner 收敛**。
- **沙箱 / Tier B / Router 完整判定器**（阶段 4）。
- **三个 Web 入口的完整运行时归一**（BriefColumn/Drawer 迁到 SSE 会话运行时是后续波次；本阶段只做共享事件契约与 `tool_start` 消费，不做大搬家）。
- **任何对外行为变更**：W1 不接前端、不换 planner，行为等价由既有 `agent-evaluation.v1` + skill contract replay 平价门保证。

---

## 2. 现状核查结论（设计的事实基础）

> 以下为对 main 分支代码的实际核查（2026-08-19），与文档描述不同的地方以代码为准。

1. **`ToolResultMessage` 类型不存在**；工具级进度回调（onUpdate）**不存在**——`ToolHandler` 只有 `func(context.Context, ToolCall) (ToolResult, error)`。agentcore 的 `UpdateFunc` 是新增面。
2. **没有真正的工具调用循环**：aichat 侧是 one-shot（一次 LLM + 最多一次解析工具，零售路径是硬编码 3 步顺序）；真正的循环在 `agentrunner.Runner.Run`（planner + 限额 + checkpoint + steer + 租约）。
3. **23 处 `Status:"pending"` 静态卡片**确认存在（`agent.go` buildPerformanceRunbook 15 处 + 其他 runbook 8 处；retail 另有 3 处 plan step）。
4. **文件路由兜底确认是静默的**：`detectFileParseTool`（agent.go:2530-2540）关键词不命中 → `"parse_contract"`。前端另有 `inferUploadTaskType`（page.tsx:1702-1722）关键词猜测。
5. **前端三入口各写各的**：`/ai-chat` 用 `useAIChatRuntime`（SSE + localStorage）；主页 `BriefColumn` 与 `RetailAIDrawer` 是一次性 JSON（无流、无会话）。前端 reducer **丢弃 `tool_start`**（后端已发）。
6. **`go.mod` 仅 6 个直接依赖**，无 excelize / docx 库 / minio-go。`internal/controlledxlsx`（零依赖 XLSX 读取器）已存在，本阶段保留（待决策 #7 倾向保留）。
7. **三个对外协议是硬约束**：`ai-intake.v1`（core 的 `aiintake` 包严格校验）、`/agent/plan` 的 tool_calls 协议（`http_planner.go` 已定义对端结构）、`llm-usage.v1` usage 元数据。本阶段不动它们。
8. **零售导入 preview/commit API 已存在**：`POST /retail/operating-facts/store-days/import/preview|commit`（permission `master_data:manage`）——CLI 捷径可直接映射，不需要新建业务 API。
9. **CLI（cmd/lease-agent）是薄 HTTP 适配器**：全 JSON 输出、无本地凭证存储（env/flag 传 token）、业务命令硬编码工具名映射。无表格输出、无 SSE 消费。
10. **评测 harness 现况**：`agentskill.Evaluate` 只测技能路由（14 个 case，6 个 category）；`EvaluationCase` 没有可表达「不变量」的字段——M9 需要扩展。

---

## 3. 目标包结构与依赖方向

```
internal/agentcore/              # 纯循环内核。禁止 import database/sql、net/http、internal/repository、minio
├─ state.go   event.go   tool.go   queue.go
├─ hooks.go   loop.go    agent.go    # 闸点类型（ChainBefore/ChainAfter）就位；六个治理中间件实现留 W2
internal/workingpaper/           # 底稿产物层：协议 + lint + 封面 + 渲染
├─ protocol.go   lint.go
├─ render_xlsx.go   render_docx.go
internal/docparse/               # 文档解析层
├─ docparse.go   csv.go   anydoc.go   paddleocr.go
internal/agentseval/             # 不变量与 triage 的 L1 用例（embed 数据集）
├─ invariants.go   testdata/agent-invariants.v1.json
internal/agenttools/             # 既有包，增量：
├─ protected_measures.go         # 受保护度量数据 + 词法探针 + 路由/ lint 函数（M2）
└─ tools/doc_triage.go           # doc.triage 工具（M5）
internal/agentartifact/          # 既有包，增量：working_paper 类型
internal/aiagent/                # 既有包，增量：文件路由改走 triage
internal/handlers/               # 既有包，增量：artifact 导出端点
cmd/lease-agent/                 # 既有包，增量：三层命令
```

**依赖方向**（单向，与 Agent Core 设计 §4 一致）：

```
agentcore ← agenttools（类型复用）
workingpaper ← agenttools（protected_measures）+ excelize
docparse ← （无业务依赖，仅 stdlib + net/http + minio 凭据由调用方注入）
aiagent ← agentcore（未来）+ agenttools + docparse（triage）
handlers ← workingpaper + agentartifact
```

`agentcore` 不认识任何上层包；`hooks` 只依赖 `agentcore` + `agenttools`。

---

## 4. 模块设计

以下每个模块给出：**Interface**（调用者必须知道的全部）、**Implementation**（藏在后面的复杂度）、**Seam**（接口活在哪里）、**Depth**（为什么值得深）、**删除测试**（凭什么它配存在）。

### M1 · `internal/agentcore` —— 纯循环内核（W1）

**Interface**

```go
// State：有状态的消息/工具/流式态容器。对外只读，赋值复制顶层切片。
type State struct{ /* mu + SystemPrompt/Model/ThinkingLevel/tools/messages/streaming/pendingToolCalls/lastError */ }
func (s *State) Tools() []Tool
func (s *State) SetTools(t []Tool)
func (s *State) Messages() []Message
func (s *State) SetMessages(m []Message)
func (s *State) IsStreaming() bool
func (s *State) PendingToolCalls() []string

// Message：会话消息（role/content/annotations），Event：AgentStart/TurnStart/MessageStart/MessageUpdate/MessageEnd/
// ToolExecutionStart/ToolExecutionUpdate/ToolExecutionEnd/TurnEnd/AgentEnd 联合。
type Event interface{ isAgentEvent() }

// Tool：存量 agenttools.ToolDefinition 的超集——补 ExecutionMode 与 onUpdate。
type UpdateFunc func(partial any)
type Tool interface {
    Descriptor() agenttools.ToolDescriptor
    Execute(ctx context.Context, call agenttools.ToolCall, onUpdate UpdateFunc) (agenttools.ToolResult, error)
    ExecutionMode() ExecutionMode   // Sequential | Parallel
}
func FromDefinition(d agenttools.ToolDefinition, mode ExecutionMode) (Tool, error)  // 存量工具零改动接入；非法 mode 在注册期失败

// 闸点类型（W1 只做类型 + Chain；W2 装六个实现）：
type BeforeToolCall func(context.Context, BeforeContext) (BeforeResult, error)
type AfterToolCall  func(context.Context, AfterContext) (AfterResult, error)
func ChainBefore(hooks ...BeforeToolCall) BeforeToolCall  // 顺序执行，首个 Block 即止
func ChainAfter(hooks ...AfterToolCall) AfterToolCall     // 全部执行，错误聚合

// 纯循环：唯一 LLM I/O 出口是注入的 Stream；Emit 是唯一事件出口。
type Deps struct {
    Stream      StreamFunc
    Before      BeforeToolCall
    After       AfterToolCall
    ShouldStop  func(context.Context, *State) bool
    PrepareNext func(context.Context, *State) error
    Emit        func(Event)
}
func Loop(ctx context.Context, s *State, d Deps) error
func LoopContinue(ctx context.Context, s *State, d Deps) error

// Agent：有状态包装——两条队列（steer/followUp，mode All|OneAtATime）、订阅者、
// abort、WaitForIdle 结算语义（AgentEnd 后等所有订阅者返回）。
type Agent struct{ /* State + queues + subscribers + abort */ }
func New(opts Options) *Agent
func (a *Agent) Prompt(ctx, msg ...Message) error
func (a *Agent) Continue(ctx) error
func (a *Agent) Steer(msg Message) / FollowUp(msg Message) / ClearQueues() / HasQueued() bool
func (a *Agent) Subscribe(fn func(context.Context, Event) error) (unsubscribe func())
func (a *Agent) Abort()
func (a *Agent) WaitForIdle(ctx context.Context) error
func (a *Agent) Reset()
func (a *Agent) State() StateView
```

**Implementation（藏在后面）**：循环状态机（turn 推进、工具调用 round-trip、pendingToolCalls 去重、流式消息合并）；队列语义（steering 先于 follow-up 排空、follow-up 仅将停止时注入）；订阅者结算（AgentEnd 后 `WaitForIdle` 等所有订阅者返回，审计类失败 → run 失败、推送类失败 → 告警不失败，注册时显式声明类别）；Abort 的中断传播（无新工具调用、已开始的收 ctx 取消）。

**Seam**：`StreamFunc` 是 LLM 的**唯一** I/O 出口（对齐 pi 的 `streamFn`）；`Emit` 是事件的唯一出口；Before/After 是策略的唯一挂载点；订阅者是旁路消费者的唯一挂载点。**没有**数据库、**没有** net/http、**没有** repository。

**Depth**：调用者学一个 `Deps` + 一个 `Agent`，得到完整的多轮循环、中断、队列、结算语义。循环能脱离数据库在纯内存里测试——这是它存在的一半理由。

**删除测试**：删掉它，循环/队列/结算逻辑会在 agentrunner、aichat 每个未来调用点重新生长 N 份，且每份都要独自证明审计不丢事件。复杂度不会消失，只会扩散。

**硬规则（ACORE-5）**：`Descriptor().Level != LevelRead` 的工具声明 `Parallel` → 注册期失败（启动期之前），不是运行期。理由：并行会打乱幂等与审计顺序。

**验收锚点**：ACORE-1（import 图纯度——本包测试用 `go list` 依赖断言固化为 CI 用例）、ACORE-5、ACORE-6（Abort 语义）、ACORE-8（队列语义）；平价门为既有 `agent-evaluation.v1` 全绿 + skill contract replay（本阶段不动 planner，天然满足，作为回归基线跑通）。

### M2 · `internal/agenttools/protected_measures.go` —— 受保护度量（阶段 0 数据面）

**Interface**

```go
type ProtectedMeasure struct{ ID, ZH, EN string }
func ProtectedMeasures() []ProtectedMeasure          // 10 项，随 ADR-0025 §2 定稿
func IsProtected(measureID string) bool
func MatchLexicalProbe(label string) (measureID string, hit bool)   // 中英词法探针
func RouteMeasures(measureIDs []string) RouteDecision               // TierA | Reject（W2 的 ProtectedMeasure 中间件直接调用）
func LintCell(measureID, label string, basis workingpaper.Basis) []Violation  // I3 + 词法探针兜底
```

**Implementation**：10 项度量表（lease_liability / rou_asset / discount_rate_applied / interest_expense / rou_depreciation / amortization_schedule_row / journal_amount / disclosure_maturity_bucket / weighted_average_discount_rate / remeasurement_adjustment）；中英词法探针表（租赁负债/lease liability/使用权资产/ROU asset/折现率/discount rate/摊销表/amortization/分录/journal entry/重计量/remeasurement/…）；归一化匹配（大小写、空白、全半角）。

**Seam**：纯函数、纯数据——`workingpaper` 依赖它做产物期 lint，W2 的中间件依赖 `RouteMeasures` 做请求期路由。两处调用同一份事实源。

**删除测试**：删掉它，「受保护度量」的判定会变成每个调用点的字符串比较，且 W2 上线时请求期/产物期必然各写一份互相漂移的表。

**决策留痕（D15 单人协议要求的带日期决策日志）**：清单 10 项按 ADR-0025 §2 原文定稿，2026-08-19 复核未新增度量（减值、售后租回相关度量在 ADR-0025 定稿时已讨论，当前引擎不产出这些度量，暂不纳入；引擎新增该类输出时先补清单再发布工具——此为破坏性变更预警，记录在案）。**清单上线后变更 = 破坏性变更，必须走 ADR。**

### M3 · `internal/workingpaper` —— 底稿产物层（阶段 0 核心）

**Interface**

```go
type Basis string  // SystemFact | Certified | Exploratory | HumanInput
type Provenance struct {
    Basis Basis; ToolCallID string; EngineVersion string; InputHash string
    SandboxRunID string; CodeHash string; ImageDigest string
    SourceTable string; SourceRecordID string; DataVersion string
    ConfirmedBy string; ConfirmedAt string
}
type Cell struct{ Ref, Label, MeasureID, Value, Unit, Currency string; Provenance Provenance }
type Section struct{ ID, Title string; Kind SectionKind; Cells []Cell; Narrative string }
type Paper struct {
    Title, Period, LegalEntityScope string
    Sections []Section
    DataGaps []string
    UnexplainedResidual string
    OpenQuestions []string
    ReviewState ReviewState  // draft | needs_review | confirmed
    Cover CoverPage          // 组装时计算
}

func Build(p Paper) (Paper, error)                                  // 规范化 + 计算封面（I6 的一致性由渲染重算保证）
func Lint(ctx context.Context, p Paper, audits AuditLookup) LintReport  // fail-closed：任一 violation 即不可生成
func RenderXLSX(p Paper, now time.Time) ([]byte, error)             // 确定性输出（I7：注入 now）
func RenderDOCX(p Paper, now time.Time) ([]byte, error)
type AuditLookup interface{ CompletedToolCall(callID string) bool } // I2 的交叉比对缝
```

**Implementation**：

- **Lint（fail-closed）**：I1 每个 Cell 有非空 Provenance 且 Basis 合法；I2 `Certified` 的 `ToolCallID` 经 `AuditLookup` 命中且 completed；I3 `IsProtected(MeasureID)` 的 Cell 的 Basis 必须 ∈ {Certified, SystemFact}；词法探针兜底（MeasureID 缺失但 Label 命中探针 + Exploratory → 拒绝并记疑似绕过）；I6 封面统计与单元格实际统计重算一致。
- **封面页**：期间/法人/门店范围、数据/口径/假设版本、引擎版本 + 镜像 digest、生成时间与 run id、Certified/Exploratory 占比（I6）、未解释残差、数据缺口清单、复核状态。封面是 `Build` 的产物，不靠调用方手填。
- **RenderXLSX**（excelize）：每 Section 一个 sheet 区、Certified 数字写值并加单元格批注 `tool@version / call_id`、Exploratory 统一底色标出、末尾固定 `_来源` sheet 列全部 provenance。输出确定性：不写入当前时间戳以外的可变元数据。
- **RenderDOCX**（stdlib `archive/zip` + OOXML，**不引第三方 docx 库**）：封面页 + 章节表格 + provenance 附录。docx 是 zip 包一层 XML，手写最小 OOXML（约 200 行）比引入无人维护的 docx 库更符合依赖纪律（go.mod 只多 excelize 一个直接依赖）。

**Seam**：`AuditLookup` 把「谁算的」的查证留在调用方（agenttool 审计表 / 未来 agentcore 的 ArtifactCollector）；渲染器内部 seam 为 `xlsx` / `docx` 两个实现，屏内渲染留待后续。**lint 必须先于任何渲染**，顺序在 `Build`/`Lint` 之外不可绕过——导出端点只认 `Lint` 干净的 Paper。

**Depth**：调用方只交 Cell + Provenance；模块负责所有不变量、封面一致性、批注追溯、确定性输出。不变量被违反时**模块拒绝产出**而不是静默降级。

**删除测试**：删掉它，I1–I3、I6、I7 的每一条都要在导出端点、artifact 序列化、未来沙箱产物三条路径上各实现一遍，且没有一条能保证三处一致。

**验收锚点**：PROV-1（I1）、PROV-2（I2）、PROV-6（I6）、PROV-7（I7，xlsx 两次渲染字节一致）、PROV-8（批注与 `_来源` sheet，人工抽检位 + 自动化抽查批注存在性）、I3 的产物期部分（请求期部分随 W2）。

### M4 · `internal/docparse` —— 文档解析层（阶段 0 骨架，W5 完成切流）

**Interface**

```go
type EvidenceMode string  // Quote | Coordinate | Unavailable
type ParsedDocument struct {
    Markdown string; Format string; EvidenceMode EvidenceMode; Warnings []string
}
type Source struct{ Data []byte; Filename string; SizeLimit int64 }
type DocumentParser interface{ Parse(ctx context.Context, src Source) (ParsedDocument, error) }

// 错误分类（绝不静默成功）：
var ErrParseUnsupported, ErrFileEncrypted, ErrFileTooLarge, ErrParserUnavailable error
func ClassifyParseError(err error) string  // 稳定错误码：parse_unsupported | file_encrypted | file_too_large | parser_unavailable | parse_failed

// 适配器：
func CSV() DocumentParser                      // stdlib，确定性：分隔符嗅探（, \t），绝不交给模型
func AnyDoc(binPath string, timeout time.Duration) DocumentParser  // Rust CLI 子进程
func PaddleOCR(cfg PaddleOCRConfig) *PaddleOCRClient              // 异步 job：submit → poll → 下载 jsonUrl → markdown + 块级 locator
```

**Implementation**：

- **CSV**：`encoding/csv` + 分隔符嗅探 + BOM 剥离，输出确定性的 GFM 表格文本。CSV 是 ERP 默认导出格式，这条路径**永不经过 LLM**。
- **anydoc**：`exec.CommandContext` 子进程调用；二进制缺失 → `ErrParserUnavailable`（D8 诚实降级：产出不可得就不产出，**不得声称任何坐标**）；输出 GFM，`EvidenceMode=Quote`（office，无坐标）或 `Unavailable`（PDF 首轮）。
- **PaddleOCR 客户端**：按 Python 版 `paddleocr.py` 语义逐项移植——multipart 提交（**不用 base64 JSON**，Python 侧已踩过 500 的坑）、轮询 `state`（done/failed/超时）、下载 `resultUrl.jsonUrl`、`layoutParsingResults[].markdown.text` 取文本、`prunedResult` 递归识别 `{block_content|content|text|rec_text|label}` + `{block_bbox|bbox|box|coordinate|coordinates|poly}` 归一化为 `[x0,y0,x1,y1]` 的块级 `{page, coordinates, quote}`。凭据与可用性开关（`paddleocr_enabled`）从 env 读。
- **格式探测**：扩展名 + 内容嗅探；超限 → `ErrFileTooLarge`；加密/损坏 → `ErrFileEncrypted` / `ErrParseUnsupported`。

**Seam**：`DocumentParser` 接口是所有文件入口的唯一形状；调用方（triage 链路、未来 aiintake Go 化）不感知 anydoc/PaddleOCR/CSV 的差异。**证据模式的取值把 ADR-0024 §4 的诚实降级编码进了类型**——这是本契约的核心。

**删除测试**：删掉它，格式探测、四类错误分类、证据模式判定会扩散回每个上传/解析调用点，且「有文本就当有证据」的静默降级（踩 R3 红线）会重新出现。

**验收锚点**：G5（excelize 在本包落地 xlsx 写入）、CORR-7 的解析侧失败语义（每类错误码都有故障注入用例）；完整切流与 PyMuPDF 删除留 W5。

### M5 · `doc.triage` 工具 —— 文件分诊（阶段 0）

**Interface**

```go
// 工具：lease.file.triage，LevelRead
// Input:  { file_id, object_name, content_type, user_message }
// Output: { doc_class, confidence, detected_entities, reason }
type DocClass string  // lease_contract | rent_schedule | amendment | contract_ledger
                      // | operating_data | financial_statement | invoice
                      // | meeting_minutes | unknown
type TriageClassifier interface {
    Classify(ctx context.Context, req TriageRequest) (TriageResult, error)
}
func DeterministicTriage(req TriageRequest) TriageResult  // 关键词判定，但绝不兜底为 contract
```

**Implementation**：LLM 分类器走既有 `callLLM` 通道（`response_format: json_object` + 严格 schema 校验，文档内容一律当数据不当指令）；确定性回退在 LLM 不可用时给出 `doc_class=unknown` + 候选列表，**不是**「关键词不命中 → contract」。`aiagent` 的文件路由（`detectFileParseTool` 的 default 分支）改为：先 triage → `unknown` 或置信度低于阈值 → 停下问用户并列候选（前端渲染候选按钮）；命中 → 走对应解析路径。**`return "contract"` 兜底从代码中删除。**

**Seam**：`TriageClassifier` 接口分隔 LLM 实现与确定性实现——测试用后者（零网络），生产用前者，失败语义一致。

**删除测试**：删掉它，「静默兜底为合同」是历史上已被证实的最危险失效模式（门店 P&L 被当合同解析），CORR-6 的核心判据「域外文件被误分类为 lease_contract 的数量 = 0」没有它就没有承载点。

**验收锚点**：CORR-6（确定性回退部分进 CI；LLM 准确率部分需要 ≥50 份语料，属 L2 nightly，本阶段建语料框架并标注为剩余工作）、CORR-7 的「Triage 置信度低 → 停下来问用户，不得兜底」故障注入。

### M6 · artifact 类型与导出端点（阶段 0）

- `agentartifact` 增加 `working_paper` 到 `knownArtifactType`；data 为 `workingpaper.Paper` JSON。
- 新端点 `GET /api/v1/ai/chat/artifacts/:id/export?format=xlsx|docx`（permission `ai_chat:use`）：读 artifact → 解析 Paper → **先 `Lint`（fail-closed）** → 渲染 → `Content-Disposition` 附件。Lint 不过 → 400 + 逐条 violation（不允许「降级输出」）。
- 前端 `ArtifactSummaryPanel` 对 `working_paper` 渲染封面统计卡 + 章节摘要 + 导出链接。

**验收锚点**：PROV-1/6 在导出路径上的复用（同一 `Lint` 代码），OPS-3 的可观测（导出记录走既有审计动作）。

### M7 · CLI 三层命令（cmd/lease-agent 增量）

对齐 lark-cli 三层，但**不引入代码生成器**——我们的「API 元数据」就是编译期可枚举的 `agenttools.Registry`：

| 层 | 我们 | 现状 → 本阶段 |
|---|---|---|
| 3 Raw | `lease-agent execute --tool ...`（Gateway 直通） | 已有，不动 |
| 2 API | 注册表逐工具命令 | 已有 `tools`（Describe）+ `execute`；本阶段加 `--format table\|ndjson` 输出 |
| 1 Shortcuts | 领域捷径：`retail import preview\|commit <file>`、`scenario evaluate\|save` | **本阶段新增**，映射既有 API（`/retail/operating-facts/store-days/import/preview\|commit`、scenario 工具） |

**人机边界编码**（本阶段的关键设计，附录 A.4）：`commit` 类命令强制人类 JWT（**拒绝 capability token**——capability token 是 agent 面的凭证）、默认 `--dry-run` 预览、执行前回显 scope 摘要、显式 `--confirm`。服务端网关仍按既有权限/幂等/审计闸口执行——CLI 的 flag 是 UX，服务端是真正的闸。退出码沿用既有约定（0/2/3/4/5），错误信封 stdout/stderr 分离（数据 stdout、进度与错误 stderr）。

**验收锚点**：ACORE-10 的 CLI 侧可观察性（agent capability token 调 commit 被拒，错误码 `scope_denied` 类保持原因不软化）、附录 A.5 端到端链路（`retail import preview` → 页面 commit 由人触发）。

### M8 · Web 三入口（阶段 0 增量，不做大搬家）

1. **删除 `inferUploadTaskType`**（page.tsx:1702-1722）：上传不再猜 task_type，后端 `doc.triage` 决定；前端渲染 triage 的候选选择按钮（unknown/低置信度时）。
2. **前端消费 `tool_start`**（state.ts reducer 增加分支）：运行中工具状态可见（现状后端已发但被丢弃）。
3. **`working_paper` artifact 渲染**：封面统计卡（Certified/Exploratory 占比、数据缺口、复核状态）+ 导出 xlsx/docx 链接。
4. 新 UI 一律遵守 DESIGN.md §13（无硬编码颜色、无字面边框），复用既有 ToolChip/ConfidenceBadge/DataTrustBar。

**验收锚点**：OPS-5 的一部分（事件流契约统一，`tool_start`/`tool_end`/`working_paper` 三入口共享同一 reducer 语义）、SEC-6 的前端呈现部分（triage 候选高亮而非静默）。

### M9 · 评测扩展（L1 注入既有 harness）

- **落位调整（2026-08-19 实施时修正）**：新 category 不放进 `agentskill.EvaluationCase`（那会把 workingpaper 的依赖图拖进纯注册表包），而是新增 `internal/agentseval` 包 + `testdata/agent-invariants.v1.json`（embed 随包发布），`cmd/agent-evaluation` 增加第三份报告段 `invariants`——仍是「同一 harness、同一评测入口」，退出码对不变量失败同样非零。
- 用例分两类：`provenance`（workingpaper.Lint 断言，期望 violation 集合精确匹配）与 `triage_refusal`（确定性 triage 断言 doc_class）。**不改变既有 case 的判定语义**（回归基线：现有 14 个 routing/role/permission/refusal/review_gate/high_risk_refusal case 保持全绿）。
- 数据集自带用例：干净底稿通过、缺失 provenance 拒绝（I1）、无 tool_call_id 的 Certified 拒绝（I2）、受保护度量 Exploratory 拒绝（I3）、词法探针兜底、发票/劳动合同/宣传册不得分类为 lease_contract（CORR-6 确定性部分）。
- L2 语料（≥50 份文件）与 nightly 入口：**本阶段只建了确定性骨架，不产语料**——语料必须含真实/仿真文件，属阶段 1 门之前的持续工作，标注为剩余项。

**验收锚点**：`make` 验证命令中 `agent-evaluation` 全绿 + 新增 category 全绿；PROV-1~8 的断言以「同一评测集的新 category」形态存在。

---

## 5. 关键决策记录（带日期）

| # | 决策 | 理由 | 日期 |
|---|---|---|---|
| D-A | 新增唯一第三方依赖 **excelize**（xlsx 读写）；docx 用 stdlib 手写最小 OOXML | go.mod 现仅 6 个直接依赖；docx 库生态（baliance 系列）无人维护，OOXML 最小面约 200 行可控；`controlledxlsx` 保留为受控模板读取路径（待决策 #7 倾向保留的落地） | 2026-08-19 |
| D-B | `workingpaper` 的 lint 是**唯一**出口前闸口：导出端点、artifact 序列化、未来沙箱产物都只认 `Lint` 干净的 Paper | 底稿方案 §4.3 fail-closed 原话：任一项失败整份不予生成 | 2026-08-19 |
| D-C | triage 确定性回退的失败语义 = `unknown` + 候选列表，**永不**是「默认 contract」 | R3 红线；`return "contract"` 是本项目已被证实的最危险行为模式 | 2026-08-19 |
| D-D | protected_measures 10 项按 ADR-0025 §2 定稿；后续新增引擎输出涉及受保护度量时，先扩清单再发布工具 | 索引 §5 未决项 3 的 D15 单人替代（带日期决策日志）；清单变更 = 破坏性变更 | 2026-08-19 |
| D-E | W1 的 agentcore 只做「类型 + 循环 + 测试」，不接前端、不换 planner；平价门 = 既有 agent-evaluation.v1 全绿 + skill contract replay | Agent Core 设计 §10：W1–W3 不改变任何对外行为 | 2026-08-19 |
| D-F | CLI 不引入 keychain（现状无本地凭证存储，token 走 env/flag）；`--format table\|ndjson` 先行，keychain 列为 W4 之后的治理项 | 现状已无明文落盘风险（好于预期）；keychain 的价值在「多人共享机器」场景，当前单人 + AI 团队下优先级低于 commit 边界 | 2026-08-19 |
| D-G | 三入口 Web 归一（BriefColumn/Drawer 迁 SSE 运行时）**不进本阶段**，本阶段只统一事件消费契约（tool_start + working_paper 渲染语义） | 改造清单〔0〕只要求「两平面事件流合并渲染」；大搬家风险高于收益，留两平面汇流落地波次 | 2026-08-19 |

---

## 6. 任务清单与验收映射

| 序 | 任务 | 产出 | 验收锚点 |
|---|---|---|---|
| 1 | agentcore 全包 + 单测 | state/event/tool/queue/hooks/loop/agent | ACORE-1/5/6/8 + 回归基线（agent-evaluation 全绿） |
| 2 | protected_measures.go | 度量表 + 探针 + Route/Lint | I3 产物期用例；W2 接线预留 |
| 3 | workingpaper | 协议 + lint + 封面 + xlsx/docx | PROV-1/2/6/7/8 |
| 4 | agentartifact + 导出端点 | working_paper 类型 + export 端点 | PROV-1/6 复用；端点测试 |
| 5 | docparse | 接口 + CSV/anydoc/PaddleOCR + 错误分类 | CORR-7 解析侧故障注入 |
| 6 | doc.triage 工具 + aiagent 路由改造 | 工具 + 删除 `return "contract"` | CORR-6 确定性部分 + CORR-7 triage 故障 |
| 7 | CLI 三层 | retail import/scenario 捷径 + 格式 + commit 边界 | ACORE-10 CLI 侧 + A.5 链路 |
| 8 | Web 三入口增量 | 去关键词猜测 + tool_start + working_paper 渲染 | OPS-5 部分 + DESIGN.md §13 |
| 9 | 评测扩展 | provenance/triage 两 category | M9 全绿 + 旧 case 不回归 |
| 10 | 全量验证 | `go test ./... && go vet`、web type-check/build/test | AGENTS.md 验证节 |

**G0 门对应**：PROV-1（任务 3/4）、PROV-2（3）、PROV-6（3）、PROV-7（3）、PROV-8（3）、CORR-6 确定性部分（6/9）、CORR-7 解析+triage 部分（5/6）、OPS-3~6 部分（4/8）。SEC-6 的注入红队与 CORR-6 的 LLM 准确率语料为**本阶段明确剩余项**，随 L2 语料建设补齐。

## 7. 落地顺序

依赖驱动的串行顺序：**1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10**。每步跑 `go test ./...`（web 步骤跑 type-check + jest），提交粒度按任务切分。
