# 任务指令：ai-service 退役（W4 + W5）与 agentcore 接管循环

> 状态：待执行 · 编制：2026-08-20 · 分支基线：`docs/retail-bp-workstation-prd` @ `045a864`
> 依据：ADR-0022（自研 Go Agent Core）、ADR-0023（退役 Python ai-service）、ADR-0024（删除 AGPL 的 PyMuPDF）
> **完成后本文可删除** —— 它是 transient 执行工单，不是长期文档。结论请回写 ADR 与 `docs/AI_文档索引与现行决策.md`。

## 0. 你接手的是什么

三份 ADR 在 2026-08 定了「后端只剩 Go」。W1–W3 落了地，**W4 与 W5 一行没动**，但多份文档在描述现状时把它们当成了已完成事实。你要做的是把这个差距真正补上。

动手前必读（按序）：

1. [AGENTS.md](../../AGENTS.md) —— 五条底线、GUARD-001 验收规则、验证命令、双交付纪律
2. [docs/AI_文档索引与现行决策.md](../AI_文档索引与现行决策.md) —— §2 决策登记 D1–D21、§3 缺口表。**与本文冲突时以决策登记为准**
3. [docs/Agent_Core_Go设计_对齐pi架构.md](../Agent_Core_Go设计_对齐pi架构.md) —— §8.2 anydoc 契约、§11 ACORE 验收、§12 未决项
4. [docs/adr/0023-retire-the-first-party-python-ai-service.md](../adr/0023-retire-the-first-party-python-ai-service.md) 与 [0024](../adr/0024-remove-the-agpl-pdf-dependency.md)
5. [CONTEXT.md](../../CONTEXT.md) —— 命名与口径纪律

### 0.1 核实过的现状（2026-08-20，与文档叙述有出入，以本表为准）

| 组件 | 文档曾暗示 | 代码实际 |
|---|---|---|
| `internal/agentcore` 根包 | W1 已交付、内核在跑 | **零生产调用**。`Agent`/`Loop`/`Queue`/`State` 有测试有 import guard，但没有任何非测试代码 import 它 |
| `internal/agentcore/hooks` | 同上 | ✅ **真的在跑**。经 `agenttools.ExecutionGuard` 接进 aiagent 全路径（`aiagent/agent.go:250-255`） |
| `internal/docparse` | 阶段 0 已交付 | **全仓零引用**。846 行生产码 + 327 行测试完全孤立；`anydoc` 二进制也未安装 |
| `internal/llm`（W4） | ADR-0023 映射表列了它 | **不存在**。Go 每一次 LLM 调用都走 Python `/api/v1/chat`，API key 只配在 ai-service |
| `intake/` 迁移体量 | ADR-0023 估 ~400 行 | **1,973 行**（`producer.py` 1263 + `models.py` 398 + `adapters.py` 312） |
| `workingpaper` | 阶段 0 已交付 | ✅ **真的在跑**，13 处生产调用，provenance + fail-closed lint + xlsx/docx 齐全 |
| `lease.file.triage` | G2 部分解决 | ✅ 在跑，但是**纯元数据分诊**（文件名 + 用户消息），不读文件内容，因此用不上 docparse |

**ADR-0023 §Context 那张「九成是 HTTP glue」的表低估了 intake。** 那 1,973 行里不是 glue，是业务规则：折现率缺失判定、币种校验、关键字段检查、`lease_scope` 归一、置信度净化、证据引文比对、付款计划校验与兜底文本解析。**迁移风险全部集中在这里**，W5-3 因此单列并配语料验收。

### 0.2 Go 侧现存的全部 Python 依赖点

| # | 位置 | 调用的 Python 端点 | 归属波次 |
|---|---|---|---|
| 1 | `aiagent/agent.go:2046` `callLLM` | `/api/v1/chat` | W4-1 |
| 2 | `aiagent/agent.go:2104` `callLLMWithTools` | `/api/v1/chat` | W4-1 |
| 3 | `cmd/agent-runner/main.go:48-49` | `/api/v1/agent/plan` | W4-3 |
| 4 | `aiagent/agent.go:2238` `parseFile` | `/api/v1/parse/contract` | W5-3 |
| 5 | `aiagent/agent.go:2306` `parsePaymentSchedule` | `/api/v1/parse/payment-schedule` | W5-3 |
| 6 | `aiagent/agent.go:2463` `parseContractBatch` | `/api/v1/parse/contract-batch` | W5-3 |
| 7 | `aiagent/event_file_parser.go:33` `parseEvent` | `/api/v1/parse/event` | W5-3 |
| 8 | `handlers/retail_mapping_ai.go:24` | `/api/v1/suggest-mapping` | W5-4 |
| 9 | `web/next.config.js:28` → `/api/ai/files/upload` | `/api/v1/files/upload` | W5-5 |

九条清空，ai-service 才能删。**任何一条留着，W6 都不许执行。**

## 1. 顺序与它的理由

```
W4  internal/llm（含 StreamFunc 形状）→ 退 chat.py + agent_plan.py
     ↓  ← 这里是 agentcore 的接缝
W5  docparse 接线 → intake 迁 Go → mapping/files → Python 只剩空壳
     ↓
C   agentcore 接管循环（此时只是薄接线）
     ↓
W6  删 ai-service + PyMuPDF + CI Python job
```

**为什么 agentcore 排在 W4 之后而不是单独立项：** `agentcore.Loop` 只缺一样东西才能跑 —— 一个 `StreamFunc`（`func(ctx, *State) (StreamResult, error)`，见 `agentcore/loop.go:14`），即「给我一轮模型输出加它要调的工具」。而 W4 本来就要重写 `callLLM` 与 `callLLMWithTools` 这两个函数。**把 `internal/llm` 直接做成 `StreamFunc` 的形状，agentcore 接管就从「重写 2500 行」降级为一次薄接线；按老形状写完则要再造一遍接缝。** 这是本指令唯一一处「顺序即成本」的地方，不要调换。

**为什么不删 agentcore 也不删 docparse：** 两者都不是重复实现。`aiagent` 没有循环（无 `for` 轮次、工具结果不回灌模型、`callLLMWithTools` 只调一次且只取 `toolCalls[0]`），`agentcore.Loop` 提供的多轮与流式事件序列是它没有的能力；`docparse` 是 W5 的地基，只是提前建好了。**它们是没接上电的部件，不是垃圾。**

## 2. 工作约定

- 每项改动必须带测试；**替换类改动适用 GUARD-001**：证明「新的真的生效」，只证明「旧的消失了」不算数
- **迁移类改动必须先建平价门**：新旧两条路径对同一输入产出一致的对照测试，先绿再切换，切换后删旧路径
- 完成标准：`cd core-service && GOCACHE=$(pwd)/.gocache go test ./... && go vet ./...`；`cd web && npm run type-check && npm run build && npm test`；`make ifrs16-regression` 参考值不变（初始负债 ¥3,255,676.79）
- **每个波次结束时 `agent-evaluation.v1` harness 必须全绿**（`internal/agentseval`，当前 14 用例 5 category）
- 不得顺手重命名 `lease_*` 命名空间；不得破坏既有页面/API
- 凡动 schema：增量迁移 + `db/init/01_init.sql` 空库版本**双交付**

---

# 波次 W4 —— LLM 进 Go，退两个 Python 端点

## W4-1 `internal/llm`：Go 的 LLM 客户端，形状对齐 `agentcore.StreamFunc`

- **位置**：新建 `core-service/internal/llm/`；替换 `aiagent/agent.go:2045-2236` 两个函数
- **对照实现**：`ai-service/app/services/llm.py`（144 行，两个 provider 的 HTTP 客户端 + `usage_metadata` 解析）
- **要做**：
  - Provider 抽象覆盖现有两家：DeepSeek（默认）与 OpenAI（备用），沿用现有环境变量名（`DEEPSEEK_API_KEY` / `DEEPSEEK_BASE_URL` / `DEEPSEEK_MODEL` / `OPENAI_*`），**不要新造名字**，否则 `.env` 与 compose 要连带改
  - 保留 `usage_metadata` 的 token 用量解析 —— `/agent-metrics` 页面依赖它，缺了会让成本显示变成 unavailable
  - **接口形状必须能直接充当 `agentcore.StreamFunc`**：产出 `StreamResult{Start, Updates, End, ToolCalls, Terminate}`。当前 Python 不支持流式，本波次允许「一次返回、Updates 为空」的退化实现，但**类型形状要对**，让 C 波次不必改签名
  - API key 从 ai-service 挪到 core-service 的环境变量；compose 与 `.env.example` 同步
- **验收**：
  - provider 双分支的单元测试（含 fallback 触发条件）
  - token 用量解析的对照测试，`/agent-metrics` 回归不变
  - **平价门**：同一 prompt 经新旧两条路径，回答文本与 model name 一致的对照测试（可用录制的 fixture）
  - 一个断言 `internal/llm` 的输出类型满足 `agentcore.StreamFunc` 契约的编译期测试

## W4-2 切换 aiagent 的两个调用点并删除 `/chat` 依赖

- **位置**：`aiagent/agent.go:2045`（`callLLM`）、`:2103`（`callLLMWithTools`）
- **要做**：两个函数改为调用 `internal/llm`；删除 `AI_SERVICE_URL` 分支与 `http://ai-service:8000` 默认值；`ai-service/app/routers/chat.py` 与其测试一并删除
- **注意**：`callLLMWithTools` 现在只取 `toolCalls[0]`（`agent.go:902` 附近）。**本波次保持这个行为不变**，多轮是 C 波次的事 —— 一次只改一件事
- **验收**：`grep -rn "AI_SERVICE_URL" core-service --include="*.go"` 在 aiagent 里只剩 parse 系列；聊天端到端回归绿；`agent-evaluation.v1` 全绿

## W4-3 退役 planner，拆掉 `AGENT_PLANNER_TOKEN`

- **位置**：`cmd/agent-runner/main.go:48-49, 93, 155, 239-241`；`internal/agentrunner` 的 `NewHTTPPlanner`；`ai-service/app/routers/agent_plan.py`（170 行）
- **背景**：ADR-0023 说得明白 —— `agent_plan.py` 存在只因为大脑在 Core 之外。W4 之后大脑在进程内，这个端点与它的共享密钥是纯开销
- **要做**：
  - planner 改为进程内实现，复用 W4-1 的 `internal/llm`；`_normalize_plan` 的工具白名单校验逻辑（`agent_plan.py:55-93`）一并迁 Go —— **它是安全边界，不是格式化**，模型返回未注册工具必须拒绝
  - 删 `--planner-url` / `--planner-token` 两个 flag 与 `AGENT_PLANNER_URL` / `AGENT_PLANNER_TOKEN` 环境变量（`docker-compose.yml` 三处）
  - 保留 `agent-runner` 本身与它的 checkpoint / 租约恢复（决策 D14，**不删**）
- **验收**：
  - 空 plan 必须 fail-closed 的测试（现有 `http_planner_test.go:43` 的语义要保住）
  - **模型返回未注册工具名 → 拒绝**的反向测试（先证红）
  - `grep -rn "AGENT_PLANNER" .` 除 git 历史外零命中
  - `agent-runner` 的 worker 租约与 run trace 回归绿

---

# 波次 W5 —— 解析进 Go，Python 只剩空壳

## W5-1 接线 `internal/docparse`（它已经写好了，只是没人调）

- **位置**：`core-service/internal/docparse/`（`docparse.go:56` 的 `DocumentParser` 接口是唯一形状）
- **要做**：
  - 在 `cmd/api/main.go` 构造 `DocumentParser`（CSV / anydoc / PaddleOCR 三个适配器按 ADR-0024 的分流规则装配），注入到需要它的调用方
  - **`anydoc` 二进制的供应链**（索引文档未决项 6）：版本 + checksum 钉死方式本波次必须定死并写进 `docker-compose.yml` 与 Dockerfile。**不要留「装一下就行」的口头约定**
  - PaddleOCR 客户端已在 `docparse/paddleocr.go`（378 行），与 Python 的 `paddleocr.py`（446 行）行为对齐验证
- **验收**：
  - `grep -rln "internal/docparse" core-service --include="*.go" | grep -v _test` **非空**（这是本票的存在理由）
  - 三个适配器各自的端到端解析测试
  - **惰性证据（D7）**：首轮 anydoc 出文本、点「查看证据」才跑 OCR 并缓存的测试
  - **诚实降级（D8）**：OCR 不可用时降级 anydoc 且证据标 `unavailable`、**不得声称坐标**的反向测试

## W5-2 建立抽取准确率基线（**做 W5-3 之前必须先有这个**）

- **背景**：ADR-0023 说得对 ——「W5 不是靠『它编译过了』通过，是靠标注语料」。但基线现在不存在
- **要做**：
  - 用现有 Python 路径对一批真实/仿真文件跑一遍，固化为 CORR-2 基线：合同、租金表、事件、批量台账四类各 ≥10 份
  - 逐字段记录：抽取值、置信度、证据定位、`discount_rate_missing` / `currency_missing` / 关键字段缺失的判定结果
  - 落进 `internal/agentseval/testdata/`，纳入 harness
- **验收**：基线可复演（同输入同输出）；harness 新增 category 且全绿
- **注意**：这一票**不改任何生产代码**，它是 W5-3 的验收标尺。**不做这一票就做 W5-3，等于用「能跑」冒充「对」**

## W5-3 迁移 intake producer（1,973 行，本指令风险最高的一票）

- **位置**：`ai-service/app/intake/{producer.py,models.py,adapters.py}` → `core-service/internal/aiintake/`
- **注意**：Go 的 `internal/aiintake` 现在是**消费侧**（校验到达的草稿是否满足 `ai-intake.v1`），你要加的是**生产侧**。两者共用同一批类型，不要另起包名
- **必须逐条迁移的业务规则**（这些不是 glue，漏一条就是行为回归）：

  | Python 位置 | 规则 |
  |---|---|
  | `producer.py:1013` | `_check_discount_rate_missing` —— **AI 不得猜折现率**（AGENTS.md 硬约束） |
  | `producer.py:1026` | `_check_currency_missing` |
  | `producer.py:1045` | `_check_critical_fields` |
  | `producer.py:1064` | `_normalize_lease_scope` —— 范围闸门前置到计量引擎的入口 |
  | `producer.py:1090` | `_sanitize_confidence_scores` |
  | `producer.py:703-780` | 证据解析与引文比对（`_evidence_quote_matches` / `_normalize_evidence_text`） |
  | `producer.py:837` | `_validate_payment_schedules` |
  | `producer.py:893` | `_fallback_parse_payment_schedule_text` —— LLM 失败时的确定性兜底 |
  | `producer.py:1225` | `_apply_excel_evidence_safety_checks` |
  | `producer.py:508-702` | 四个 prompt 模板（合同/付款/事件/批量）|

- **Excel 读取**：Python 用 `openpyxl`，Go 用 `excelize`（`go.mod` 已有 v2.11.0，`storepnl` / `workingpaper` / `finmodel_export` 三处在用）。**`controlledxlsx` 不默认删除**（ADR-0023 §Consequences），去留在本票结束时给结论并回写索引文档未决项 7
- **表格送模型的纪律**：决策 D13 —— **送列画像不送原始值**，`RetailMappingAI` 的现有做法是规范，迁移不得放宽
- **验收**：
  - **CORR-2 语料不得回归**：W5-2 的基线逐字段对照，准确率不低于 Python 路径。**这一条不通过就不许合并**，「编译过了」「测试绿了」都不算
  - 上表十条规则各自的单元测试，其中 `discount_rate_missing` 与 `lease_scope` 必须有反向测试
  - `contracts/ai-intake.v1/*.json` 四份契约 fixture 继续通过（**契约不变，只换实现方** —— 决策登记 §1.3）
  - Assist Mode review gate 未被绕过的测试

## W5-4 迁移 `/suggest-mapping`

- **位置**：`handlers/retail_mapping_ai.go:24` → 改调 `internal/llm`；删 `ai-service/app/routers/mapping.py`
- **要做**：列画像构造逻辑保持不变（D13），只换 LLM 通道
- **验收**：现有 `RetailMappingAI` 测试全绿；列映射建议的输出形状不变（前端导入页消费它）

## W5-5 迁移 `/files/upload`

- **位置**：`ai-service/app/routers/files.py` + `services/storage.py`（共 166 行）→ core-service 的 `minio-go`
- **注意**：`internal/miniostore` 已存在（只读接缝，服务 page-fill）。本票要加写入侧，**沿用同一个包**，不要另起
- **前端连带改动**：`web/app/ai-chat/page.tsx:1769` 的 `/api/ai/files/upload` 与 `web/next.config.js:28` 的 `serverAiUrl` rewrite 一并指向 core
- **验收**：上传 → MinIO → 解析链路端到端绿；`web/next.config.js` 中 `SERVER_AI_URL` 及其 rewrite 删除

---

# 波次 C —— agentcore 接管循环

> 前置：W4 完成。此时 `internal/llm` 已是 `StreamFunc` 形状，本波次是接线不是重写。

## C-1 让 `agentcore.Loop` 驱动聊天

- **位置**：`aiagent/agent.go:432` `Execute()` / `:872` `executeChatRequest()`；`agentcore/loop.go:60`
- **要做**：
  - `Execute()` 改为构造 `agentcore.Deps{Stream: <W4-1 的客户端>, Before/After: <现有 ExecutionGuard 的两条链>, Emit: <接到现有事件流>}` 并调 `Loop`
  - **aiagent 的确定性分支保留为循环前的快速路径**：`executeS1Paper`、`executeRetailIngestFill`、底稿触发、intake 这些是业务资产，不进循环、不删除
  - 事件序列必须满足 `loop.go:52-54` 的有序契约，且与前端现有 reducer 兼容（`tool_start` / `tool_end` / `working_paper` 三入口共享语义）
- **验收**：
  - **平价门**：切换前后，现有全部聊天测试与 `agent-evaluation.v1` 保持全绿，一条不改
  - 多轮能力的新测试：工具结果回灌模型后产生第二轮的用例（这是本波次唯一的**新能力**，必须有正面证据）
  - `grep -rn 'agentcore"' core-service --include="*.go" | grep -v _test` 出现 aiagent（现在只有 hooks 子包）
  - ACORE-1 import guard 仍绿（循环不得 import 数据库/HTTP/repository/对象存储）

## C-2 持久化改订阅者（决策 D4）

- **位置**：`aichat/runtime.go`（451 行，现在在循环内同步写库）
- **要做**：改为 `agentcore` 的订阅者，run 直到所有订阅者返回才结算
- **验收**：订阅者失败时 run 不静默成功的反向测试；现有 run 状态流转测试全绿

---

# 波次 W6 —— 清除

## W6-1 删除 ai-service 与 PyMuPDF

- **前置**：§0.2 九条依赖点全部清零，`grep -rn "AI_SERVICE_URL\|ai-service:8000" core-service web --include="*.go" --include="*.js" --include="*.ts" --include="*.tsx"` 零命中
- **要做**：
  - 删 `ai-service/` 整个目录（含 `pymupdf==1.23.0` —— **这才是 ADR-0024 / 缺口 G6 真正关闭的时刻**）
  - `docker-compose.yml`：删 `ai-service` 服务定义、两处 `depends_on`、`SERVER_AI_URL`、`AGENT_PLANNER_*`；PaddleOCR 与 LLM 的环境变量移到 `core-service`
  - `.github/workflows/ci.yml:116-136`：删 Python job
  - `Makefile:67` 的 `make ai` 删除；`make help` 同步
  - `.env.example` 清理
- **验收**：
  - `docker-compose up` 起 4 个服务（PostgreSQL / MinIO / Core / Web），全链路冒烟通过
  - 全仓 `grep -rin "pymupdf\|fitz"` 零命中
  - CI 绿

## W6-2 文档回写（**不做这一票等于前面白做**）

- `docs/AI_文档索引与现行决策.md`：缺口 **G6 / G7 标记为已解决**；§1.2 的 AI Agent 运行运维手册去掉「待随 W4/W5 修订」；决策 D21 更新
- `docs/AI_Agent_运行运维手册.md`：ai-service 与 `AGENT_PLANNER_TOKEN` 相关章节重写
- `AGENTS.md`：删除「当前工程事实」里那段 **ai-service 尚未退役**的说明；架构分层的 AI Service 一行删除；Compose 服务数改 4
- `README.md`：技术栈删 Python / PyMuPDF 行；仓库结构删 `ai-service/`；「当前状态」表里两行 ⚠ 改 ✅
- `docs/adr/0023`、`0024`：加 Addendum 记录实际完成日期与 intake 体量的修正（ADR 正文不改，**"supersedes" 与 Addendum 才是 ADR 的机制**）
- `docs/Agent_Core_Go设计_对齐pi架构.md` §12：未决项 6（anydoc 供应链）、7（`controlledxlsx` 去留）给结论

---

# 不在本范围

| 项 | 为什么排除 |
|---|---|
| **G1 两个 Agent 平面汇流**（33 处静态 `Status: "pending"`，`aiagent/agent.go:404-419` 等） | 与 ai-service 退役无技术依赖。混进来会让「聊天回归失败」无法归因是 LLM 迁移还是平面汇流。**另开工单**，建议排在 C 波次之后 —— 那时循环已就位，静态卡片接真实执行会容易得多 |
| G2 的 LLM 分类器与 ≥50 份语料 | 需要 L2 语料建设，独立节奏 |
| G4 代码执行 / 沙箱 | ADR-0025 §5 刻意后置到阶段 4，进入条件是「已有 ≥1 真实客户且合规要求明确」 |
| 财务三表模型相关 | 已交付，不在本范围 |

---

# 完成定义（Definition of Done）

1. §0.2 的**九条 Python 依赖点全部清零**，逐条给出「文件:行号 / 改成了什么 / 测试」
2. **W5-2 的 CORR-2 基线先于 W5-3 建立**，且 W5-3 的准确率不低于基线 —— 这条是本指令唯一的非机械验收，不通过不合并
3. 每个波次结束时 `agent-evaluation.v1` harness 全绿；`go test ./... && go vet ./...`、`npm run type-check && npm run build && npm test` 全绿；`make ifrs16-regression` 参考值不变
4. 五条底线逐项给证据：跨法人正反测试、模拟标识、幂等重放、**IFRS 16 正式表零写入前后计数**、来源追溯（证据坐标在降级路径下不得伪造）
5. **W6-2 文档回写完成** —— 六份文档逐一更新，缺口表 G6/G7 关闭
6. 输出交付报告：每票一行「编号 / 文件 / 改了什么 / 测试 / 证据」，贴回本分支

## 给评审者（Claude）的提醒

本轮最容易出现的假绿有三种，评审时优先查：

1. **「迁移完成」但旧路径还在** —— 只加了 Go 实现、没删 Python 调用，两条路径并存且默认走旧的。查 §0.2 九条是否真的零命中，不看声明看 grep
2. **平价门是空的** —— 对照测试用的 fixture 恰好是新实现产出的，等于自己证自己。查基线是否在改代码**之前**固化
3. **诚实降级被偷偷改成静默成功** —— OCR 不可用时声称有坐标、置信度缺失时填默认值、模型返回未注册工具时忽略而非拒绝。这三处都要有反向测试，且反向测试要能证红
