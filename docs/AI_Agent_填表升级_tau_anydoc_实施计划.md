# AI Agent 填表升级（tau + anydoc）实施计划

> 文档状态：Draft（待评审）
>
> 版本：v0.9
>
> 编制视角：Full Stack Engineer / AI Engineer
>
> 日期：2026-08-16
>
> 上游文档：[AI Agent 与 CLI 架构演进 PRD](AI_Agent_与_CLI_架构演进_PRD.md)（v1.1，AG-001~035 已落地）、[AI Agent 与 CLI 架构演进实施计划](AI_Agent_与_CLI_架构演进实施计划.md)、[PRD_零售经营分析工作站_BP日常支撑完善.md](PRD_零售经营分析工作站_BP日常支撑完善.md)（P5-4 已决议引入 AnyDoc）

## 1. 目标

把「财务 BP / FP&A 扔一份非标准 Excel 给 Agent，Agent 识别出意图和数据，填进对应的功能页」变成可演示的端到端能力。三个动作：

1. **换脑**：引入 [huggingface/tau](https://github.com/huggingface/tau)（MIT、Python 3.12+、PyPI `tau-ai`）作为 Agent 大脑，替换当前手写的 `agent_plan` 规划器 + Go `agent-runner` 循环。
2. **补解析**：引入 [firecrawl/anydoc](https://github.com/firecrawl/anydoc)（Rust、MIT、纯本地、无 ML/无外部服务）作为 office 家族解析适配器——这是 PRD P5-4 已决议但未落地的项。
3. **补填表缝**：为每个功能页设计 fill API + CLI 命令 + `page_fill` 预填工件，Agent 通过 **CLI → Agent Gateway → Tool Runtime** 填数据。

## 2. 现状基线（为什么不是从零开始）

AG-001~035 已经建好了本方案的全部底座，本轮是「换脑 + 补解析 + 补填表」，不是重写：

| 已有资产 | 位置 | 本轮角色 |
|---|---|---|
| Tool Runtime（Describe/Execute、权限/范围/Review Gate/幂等/审计） | `internal/agenttools/` | 不变，仍是唯一工具执行面 |
| Agent Gateway + Capability Token（300s、Run 绑定、可撤销） | `/api/v1/agent/*` | 不变，tau 与人都走它 |
| `lease-agent` CLI（通用 execute + 业务友好子命令） | `cmd/lease-agent/` | **扩展**：成为 tau 的执行器 + 新增页面 fill 命令 |
| `agent-runner`（Go 手写 plan→execute→checkpoint→steer 循环） | `cmd/agent-runner/` | 被 tau 替换，平价后退役 |
| ai-service（单发端点：chat / agent/plan / parse / suggest-mapping） | `ai-service/app/` | 加 anydoc adapter；循环逻辑不进来 |
| retailingest（preview/commit，人工确认映射） | `internal/services/retailingest/` | 不变，fill 的落点 |
| FP&A plan / TB 导入 HTTP API | `/fpna/plan-versions/import`、`/gl/trial-balances/import` | 不变，fill 的落点 |

**四个缺口**：① office 解析（docx/xls/ppt/odt/rtf 等，P5-4 未落地）；② Agent 大脑是手写 Go 循环，非 tau；③ 零售事实 / 计划 / TB 的 Agent 工具与 CLI 命令缺失；④ 页面没有「从 Agent 结果预填表单」的机制。

## 3. 关键决策（默认推荐，待确认）

| # | 决策 | 推荐 | 理由 |
|---|---|---|---|
| D1 | tau 工具执行器形态 | **子进程调 `lease-agent` CLI**（用户方向） | CLI 契约已测试（AG-029），被 Agent 持续实测；安全等价（同一 Gateway、Capability、审计）。需更新实施计划 §9.1 旧结论（「Runner 不经 shell 调 CLI」）。HTTP 直连执行器作为 M3 第二适配器预留 |
| D2 | tau 部署位置 | **独立 `tau-agent` 容器**（Python 3.12） | ai-service 是 3.11-slim，不升；独立容器延续 `agent-runner` 的进程边界（无数据库凭证） |
| D3 | Agent 的 commit 权限 | **commit 不在 tau 工具白名单** | Agent 产出 preview + `page_fill` 预填；commit 由人执行（Web 按钮 / 人用 CLI）。这是 Assist Mode 的延续；Auto-commit 需另立政策（§6） |
| D4 | `agent-runner` 退役节奏 | tau 通过平价门后标记 deprecated，保留一个版本周期再删 | 平价门见 Wave T2 验收 |
| D5 | AGENTS.md 修订 | 「零售 Agent 只有三个只读 Tool」→「零售 Agent 工具 = 三个只读 + 预览/填表建议级工具；commit 类不入 Agent 白名单」 | 现状条款与本升级冲突，须同步改文档（Wave T5 执行） |

## 4. 分波实施

### Wave T1 — anydoc 解析适配器（P5-4 落地）

- ai-service 加依赖 `firecrawl-anydoc`（版本锁定 + hash）。
- 新增 `app/services/anydoc_adapter.py`：`Parse(bytes, filename) → ParsedDocument{markdown, format, evidence_mode}`。
- `parse.py` 路由分派：doc/docx/ppt/odt/rtf/epub/xls/xlsx/xlsb → anydoc；CSV 保持标准库确定性解析；PDF 路径不变（PaddleOCR 优先 + PyMuPDF fallback）。
- 错误契约：加密 / 超限 / 损坏 → 明确错误码走人工路径，不静默成功。
- 证据降级：office 无坐标体系 → quote 锚点（沿用 `ai-intake.v1` 的 `evidence_complete` 语义）。
- 测试：删除测试（删 adapter → office 解析测试红）；旧版 `.xls`（openpyxl 读不了）解析正确性；中文表格 GFM 输出断言。

**验收**：`ai-service` pytest 全绿；GUARD-001 证据 = 同一 `.docx` 的 anydoc markdown 输出 + 删除测试红 + `.xls` fixture 解析成功。

### Wave T2 — tau 大脑（tau-agent 容器）

- **Spike（0.5 天）**：PyPI `tau-ai` 的库嵌入方式（`tau_agent` 层）、自定义工具注册（schema + async executor）、provider 配置（经 `tau_ai` 接 DeepSeek/OpenAI）、headless 事件流。不可行则 fallback：保留 `agent-runner`，D4 顺延——**Gateway 契约不动，大脑永远可换**（ADR-0019 的收益）。
- 新增 Python 包 `tau-service/app/tau/`：
  - `harness`：装配 `tau_agent` 循环（消息、事件、会话原语）；
  - `schema_bridge`：Core `GET /api/v1/agent/tools` 的 ToolDescriptor → tau tool schema（单向、契约测试）；
  - `cli_executor`：子进程 `lease-agent execute --tool X --version v1 --arguments - --run-id … --capability-token …`（stdin JSON、stdout JSON、退出码映射）；
  - `event_bridge`：tau 事件 → Core run events（复用既有 HTTP EventRecorder 语义）；
  - `checkpoint_bridge`：tau 会话状态 → Core checkpoint API（Core 仍是权威持久化）。
- 容器：`python:3.12` + `lease-agent` 二进制；Capability 流转 = run create → capability issue → 每次执行携带 → 终态 revoke（沿用现有流程）。
- 预算：沿用 `agentrunner.Limits` 数值；agentguard 仍在 Gateway 生效，**不变**。
- 关键约束：**不注册** tau_coding 自带的 read/write/edit/bash 工具（AG-004 禁止任意 Shell）；工具全集 = Core descriptors ∩ skill 白名单 ∩ capability。

**平价门（替换 `agent-runner` 的前置）**：capability 流转、预算、取消、steer/follow-up、checkpoint、事件回写全部在 tau 路径上通过同一组测试；同一任务 `agent-runner` vs tau 输出与审计等价。

**验收**：docker compose 起 `tau-agent`，真实 LLM Run 走「文件 → 工具循环 → CLI → Gateway → 审计可见」；Go 全量测试保持全绿（Gateway 未动）。

### Wave T3 — 功能页 fill 缝（每页 API + CLI 命令 + page_fill）

- 新增 Agent 工具（LevelDraft / 产出预填，不落正式表）：
  - `retail.store_days.import.preview`（包装 retailingest preview + 列画像 + 映射建议，含 AI suggest-mapping）
  - `fpna.plan.import.preview`、`gl.trial_balance.import.preview`
  - `retail.scenario.action.save`（包装既有 `scenario-action-drafts` 幂等保存）
- CLI 命令（人用，含 commit；commit 工具不注册进 Agent 白名单）：
  - `retail import preview|commit`、`plan import preview|commit`、`tb import preview|commit`、`scenario evaluate|save`
- `page_fill` artifact 协议：新 artifact_type（迁移 `046_page_fill_artifact.sql`，双交付 `db/init/01_init.sql`）；字段 = `target_page / target_api / payload / sources / confidence / review_required / deep_link`。
- Web 消费端：`retail-data-import`、`scenario-workbench` 支持「从 page_fill 预填」；`ai-chat` 渲染 page_fill 卡片带「填入页面」动作。
- commit 幂等：沿用零售导入的 Idempotency-Key + payload SHA 与 plan/TB 的版本删除补偿。

**验收**：每页 fill 缝的契约测试（预填状态 = payload 断言，不是「旧入口消失」）；人用 CLI commit 一次成功、重放幂等；GUARD-001 组件受控值断言。

### Wave T4 — 意图 + 数据识别闭环

- skills 扩展（`agentskill/registry.go`）：`retail_ingest_fill`（匹配「导入/填/上传经营数据」+ HasFile）、`plan_import_fill`、`tb_import_fill`；白名单 = 文件解析工具 + 对应 preview 工具。
- tau 多步循环：附件 → anydoc 解析 → 意图（skill）→ preview 工具 → `page_fill` artifact。
- Web 全链路：ai-chat 上传 Excel → page_fill 卡片 → 「填入导入页」→ 表单预填映射 + 来源 → 人确认 commit → 经营脉搏出现 production 数据。
- 产品级 E2E 验收（对齐 P5 批次验收风格）：一份「真实 POS 风格 Excel」走通全链路，pulse 显示 production 数据与诚实覆盖率；`scope_denied` 不被软化为「无数据」（既有测试保持）。

### Wave T5 — 收口与退役

- 评估集扩充：fill 意图路由准确率、预填正确率、commit 拒绝率（Agent 白名单不含 commit 的验证）。
- `agent-runner` / `/agent/plan` 退役（D4）。
- 文档修订：AGENTS.md（D5）、实施计划 §9.1 更新、新增 ADR（tau 换脑 + CLI 子进程执行器 + Agent 填表边界）、本计划升 v1.0。
- 供应链与许可证：tau（MIT）、anydoc（MIT）为入站 permissive 依赖，ADR-0021 的对外授权姿态不受影响；锁版本、核对 wheel 平台（linux x86_64 / macOS arm64）、依赖 hash 入 requirements。

## 5. 五底线映射（每波不得降级）

| 底线 | 本升级怎么守住 |
|---|---|
| 1 跨法人隔离 | tau 身份 = Capability Token；Gateway 服务端解析身份/范围并取交集（现有 AG-025 机制不变）；CLI 只是搬运工 |
| 2 模拟 / 正式区分 | fill 载荷带 `data_classification`；production 事实仍走 retailingest 信封（source_system / batch / as_of 三件套必填） |
| 3 来源追溯 | `page_fill` 带 sources / model_version / rule_version；预填表单回写 run_id / artifact_id；批次数与版本沿用既有机制 |
| 4 重复导入保护 | commit 走既有幂等（Idempotency-Key + payload SHA / 版本删除补偿）；CLI 传递 `--idempotency-key`，重放不产生第二条记录 |
| 5 IFRS 16 正式台账隔离 | fill 缝全部落在 draft / preview / 事实导入层；Agent 工具不触计量正式表（只读工具语义不变）；commit 是人 |

## 6. 风险与缓解

| 风险 | 缓解 |
|---|---|
| tau 年轻、API 可能漂移 | Wave T2 先做 spike；Gateway 契约不动，大脑可换回 `agent-runner`；tau 版本锁定 |
| CLI 子进程开销 / 调用量 | 单 Run 工具调用数有限（预算上限），毫秒级进程开销可接受；超阈值切换 M3 的 HTTP 执行器 |
| Prompt Injection（anydoc 提取文本） | 文档只作 evidence（AG 既有决策）；字段经确定性校验；工具白名单服务端裁决 |
| Auto-commit 诉求 | D3 默认关；若产品要开，须先立政策（适用范围/阈值/审计），不在本轮 |
| 混币 / 缺失数据 | 指标语义 `retail-kpi-v1` 不变；fill 只搬运数据，不改变计算与降级规则 |

## 7. 交付物与验证

每波交付 = 代码 + 迁移（如有，双交付 01_init.sql）+ 契约测试 + GUARD-001 证据 + 看板验收条目。总验证命令（不变）：

```bash
cd core-service && GOCACHE=$(pwd)/.gocache go test ./... && go vet ./...
cd ../web && npm run type-check && npm run build && npm test
cd ../ai-service && python3 -m pytest
make ifrs16-regression        # 22 用例 / 148 断言基线不得破坏
```
