# AI Agent 与 CLI 架构演进实施计划

> 文档状态：Proposed
>
> 版本：v1.0
>
> 编制视角：AI Engineer / Platform Engineer
>
> 适用项目：租赁管理系统（IFRS 16）
>
> 创建日期：2026-08-08

## 1. 执行摘要

本项目不建议直接把一个 Pi-like Agent 放在 Core Service 外部，然后让它通过 CLI 自由调用几十个 HTTP API、执行任意命令或访问数据库。

推荐的架构演进方向是：

> 在 Core Service 与 Agent 之间建立一个受控的 Agent Tool Runtime；Pi-like Agent 负责规划和多步执行，CLI 只是一个 Adapter，所有真正的业务能力仍由 Core Service 的业务模块执行。

目标架构如下：

```text
Web AI Chat ───────────────┐
                           │
Pi-like Agent ── CLI ──────┼──> Agent Tool Runtime
                           │             │
定时任务 / 外部自动化 ───────┘             │
                                         ├── Skill Registry
                                         ├── Policy / Permission Guard
                                         ├── Tool Executor
                                         ├── Evidence / Artifact Manager
                                         └── Audit / Trace
                                               │
                                   Core Application Services
                                               │
                                  Repository / PostgreSQL / MinIO
```

这不是一次“大重写”。现有的 `ai_chat_sessions`、`ai_chat_runs`、SSE 运行事件、Artifact 和 Assist Mode 应继续复用；主要工作是把当前散落在 Agent、前端和 Repository 之间的调用逻辑收口为一个稳定、可审计、可测试的工具接缝。

## 2. 设计结论

### 2.1 需要增加的是 Tool Runtime，不是第二套业务系统

Pi-like Agent 可以提供优秀的 Agent 运行时能力，例如：

- 多轮任务规划
- 工具调用循环
- 中途纠偏和 follow-up
- 会话恢复
- 工具执行事件
- 上下文压缩
- CLI / RPC 接入

但 Pi-like Agent 不应拥有：

- PostgreSQL 账号
- MinIO 管理权限
- 任意 SQL 执行能力
- 任意 HTTP URL 调用能力
- 审批、过账、锁账的默认执行权
- 绕过 Core Service 的业务规则

### 2.2 CLI 是 Adapter，不是新的业务接口实现

CLI 的价值是让 Pi-like Agent 以稳定、可观察的方式调用系统能力，尤其适合：

- 终端式 Agent
- 本地开发和调试
- 定时批处理
- 客户环境自动化
- 人工复核脚本

CLI 不应复制合同、付款计划、月结等业务逻辑。CLI 的实现应调用 Core Service 的 Agent Tool Gateway，或者在同一进程内调用同一个 Tool Runtime 接口。

### 2.3 Agent 永远不是会计控制的权威来源

Agent 可以产生：

- 识别结果
- 风险提示
- 数据质量信号
- 草稿
- 分析建议
- 审计包目录建议

Agent 不可以直接产生：

- 正式会计结论
- Official 报表结论
- 已审批合同
- 已过账分录
- 已锁账期间状态
- 未经批准的计量结果覆盖

这与项目现有的 Assist Mode、审批流、锁账控制，以及 `Agent Signal` 与 `Control Conclusion` 分离原则保持一致。

## 3. 当前实现基线

截至 2026-08-08，项目已经具备一部分真正的 Agent Runtime 基础，不应按“从零开始”设计。

### 3.1 已有能力

- Core Service 内已有 `Agent`、Planner、Executor、Projector 和 Runtime。
- 已有服务端 `ai_chat_sessions`、`ai_chat_runs`、`ai_chat_messages`、`ai_chat_run_events`、`ai_chat_artifacts`、`ai_chat_review_actions`、`ai_chat_attachments`。
- 已有 SSE 运行事件流。
- 已有文件解析 Function Calling：`parse_contract`、`parse_contract_batch`、`parse_payment_schedule`。
- 已有合同、付款计划、计量、分录、事件查询上下文。
- 已有 AI 草稿和人工确认动作。
- 已有 Assist Mode 和置信度/缺失字段/复核提示。
- Core Service 已负责 JWT、RBAC、法人隔离和数据范围。

### 3.2 当前架构的主要问题

#### 问题 A：工具接口不统一

`core-service/internal/aiagent/agent.go` 同时承担：

- 意图判断
- Runbook 选择
- 文件工具路由
- Repository 数据读取
- 上下文拼装
- LLM 调用
- 来源抽取
- 草稿结果转换

这使 Agent 模块变浅：调用方需要理解大量内部细节，未来增加 CLI、定时任务或第二个 Agent 时会重复接入。

#### 问题 B：部分 ToolCall 只是展示元数据

当前 `core.contract_repository`、`lease.audit_pack_builder` 等名称主要出现在 Runbook 和前端工具轨迹中，并不是统一注册、统一授权、统一执行的工具。

实际 Function Calling 主要集中在文件解析工具。

#### 问题 C：写入链路没有完全收口

当前 AI 草稿确认后，Web 前端直接调用：

- `/api/v1/contracts/batch`
- `/api/v1/contracts/:id/payment-schedules`

这在浏览器场景可以工作，但会导致：

- Web、Pi CLI、定时任务可能各自实现一套写入流程。
- Agent Runtime 看不到完整的写入事务。
- 幂等、批量失败恢复、Artifact 状态和业务写入状态可能不一致。

#### 问题 D：合同上下文的权限校验需要统一

合同基础信息查询已有法人和数据范围过滤，但计量结果和事件查询需要在 Agent Tool Runtime 入口统一校验合同访问权限，不能只依赖调用方传入的合同 ID。

#### 问题 E：缺少面向外部 Agent 的稳定能力协议

目前没有统一定义以下内容：

- 工具版本
- 输入/输出 Schema
- 权限要求
- 人工确认要求
- 幂等语义
- 分页和数据上限
- 错误码
- 工具执行审计
- 证据引用格式

## 4. 目标与非目标

### 4.1 目标

1. 让 Web Agent、Pi-like Agent、CLI、定时任务共享同一套业务工具接口。
2. 让工具调用天然带有用户身份、法人、数据范围、权限、Run ID 和 Trace ID。
3. 让读取、草稿写入和高风险操作有明确的权限等级。
4. 让每一次工具调用都可重放、可审计、可观测。
5. 让新增技能只需注册 Skill 和 Tool，而不是继续扩展一个大型关键词 `switch`。
6. 让 Agent 可以多步工作，但不能绕过审批、过账、锁账和会计规则。
7. 让 CLI 适合作为自动化入口，但不变成任意 Shell 执行器。

### 4.2 非目标

本计划不包括：

- 把 PostgreSQL 直接暴露给 Agent。
- 把所有 Core Service HTTP API 自动转换成 Agent 工具。
- 立即引入开放式第三方 Tool Marketplace。
- 立即开放自动审批、自动过账或自动锁账。
- 用 Pi-like Agent 替换 IFRS 16 计量引擎。
- 用 LLM 替换确定性的审批和月结控制。
- 一次性重写现有 AI Chat 前端。

## 5. 架构选型

### 5.1 方案比较

| 方案 | 说明 | 优点 | 主要风险 | 结论 |
|---|---|---|---|---|
| A. 保持现状 | Agent 继续直接读 Repository，Web 直接写业务 API | 改动最小 | 多入口重复、权限和审计分散 | 只能作为短期过渡 |
| B. Pi Agent 直接调用所有 HTTP API | CLI 只做 HTTP 包装 | 上手快 | API 泄漏、权限分散、流程漂移、难审计 | 不推荐 |
| C. Tool Runtime + CLI Adapter | Core Service 统一工具执行，CLI 只是接入层 | 权限、审计和业务规则集中 | 需要设计工具协议和迁移 | 推荐 |
| D. 完全独立 Agent Service | Pi Agent、工具、状态全部拆成新服务 | 独立扩展能力强 | 引入分布式一致性和大量运维成本 | 后期再考虑 |

### 5.2 推荐方案

选择方案 C，并保留未来升级到方案 D 的可能性：

```text
                    ┌──────────────────────────┐
                    │ Agent Clients            │
                    │ Web / Pi / CLI / Jobs    │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │ Agent Tool Gateway       │
                    │ Auth / Scope / Policy    │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │ Tool Runtime              │
                    │ Registry / Executor       │
                    │ Artifact / Evidence      │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │ Core Application Services │
                    │ Contract / Lease / Close  │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │ PostgreSQL / MinIO        │
                    └──────────────────────────┘
```

## 6. 模块设计

本计划使用深模块设计原则：对外提供少量稳定接口，把权限、校验、事务、审计、重试和错误转换隐藏在实现内部。

### 6.1 `ToolRuntime`：主要外部接缝

建议在 Core Service 内增加：

```text
core-service/internal/agenttools/
├── registry.go
├── runtime.go
├── policy.go
├── context.go
├── schema.go
├── result.go
├── audit.go
└── tools/
    ├── contract.go
    ├── payment_schedule.go
    ├── measurement.go
    ├── journal.go
    ├── event.go
    ├── report.go
    └── audit_pack.go
```

对外接口应保持小而稳定：

```go
type ToolRuntime interface {
    Describe(ctx context.Context, filter ToolFilter) ([]ToolDescriptor, error)
    Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}
```

这里的接口不仅包括函数参数，还包括：

- 调用者必须提供的身份上下文。
- 工具是否只读。
- 是否需要人工确认。
- 是否支持幂等键。
- 结果是否分页。
- 超时和重试语义。
- 错误码和可恢复性。

### 6.2 `ToolCall`

```json
{
  "call_id": "tc_01J...",
  "run_id": "run_01J...",
  "tool_name": "lease.contract.search",
  "tool_version": "v1",
  "arguments": {
    "search": "上海门店",
    "approval_status": "approved",
    "page": 1,
    "page_size": 20
  },
  "idempotency_key": "optional-client-key",
  "dry_run": true
}
```

工具不得从 `arguments` 中信任以下字段：

- `user_id`
- `role`
- `legal_entity_id`
- `permissions`
- `is_admin`
- `approval_status_override`

这些字段必须由服务器根据 Session、JWT 或短期 Capability Token 解析。

### 6.3 `ToolResult`

```json
{
  "call_id": "tc_01J...",
  "status": "completed",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "has_more": false
  },
  "sources": [
    {
      "type": "contract",
      "id": "contract-id",
      "title": "合同名称",
      "locator": "lease_contracts:contract-id"
    }
  ],
  "review": {
    "required": false,
    "reasons": []
  },
  "error": null
}
```

模型可读的是经过裁剪和脱敏的 `data`，系统保留完整结果和审计引用，但不能把无限量数据库结果直接塞进 prompt。

## 7. Tool 分层和权限模型

### 7.1 Level 0：只读事实工具

第一阶段开放：

| Tool | 用途 | 数据来源 |
|---|---|---|
| `lease.contract.search` | 合同列表、筛选、分页 | `lease_contracts` |
| `lease.contract.get` | 合同详情和关键字段 | `lease_contracts` |
| `lease.payment_schedule.list` | 付款计划查询 | `payment_schedules` |
| `lease.measurement.list` | 计量结果查询 | `measurement_results` |
| `lease.journal.list` | 分录查询 | `journal_entries` |
| `lease.event.list` | 事件查询 | `lease_events` |
| `lease.report.preview` | Working/Official 报表预览 | Reporting Service |
| `lease.audit.list` | 审计记录查询 | `audit_logs` |

所有 Tool 必须在 Repository 查询前验证合同和数据范围。

### 7.2 Level 1：草稿生成工具

第二阶段开放：

| Tool | 输出 | 是否写正式台账 |
|---|---|---|
| `lease.contract.create_draft` | 合同草稿 Artifact | 否，或只写 Draft |
| `lease.payment_schedule.create_draft` | 付款计划草稿 Artifact | 否 |
| `lease.event.create_draft` | 事件草稿 Artifact | 否 |
| `lease.audit_pack.prepare` | 审计包目录建议 | 否 |
| `lease.close.explain` | 月结异常解释 | 否 |

草稿工具必须返回：

- 字段级置信度
- 缺失字段
- 原文证据引用
- 规则版本
- 模型版本
- `requires_human_confirmation`
- `review_reasons`

### 7.3 Level 2：受控业务命令

后续可评估：

- `lease.contract.submit_for_review`
- `lease.event.submit_for_review`
- `lease.calculation.preview`
- `lease.monthly_close.generate_draft`

这些工具只能触发标准流程，不能直接改变最终状态。

### 7.4 默认禁止工具

Agent 默认不开放：

- 任意 SQL
- 任意 HTTP 请求
- 任意 Shell 命令
- 直接修改 `lease_contracts` 任意字段
- 直接批准合同
- 直接过账分录
- 直接锁账或解锁期间
- 直接 ERP 回写
- 直接覆盖已锁账数据

如果未来开放高风险命令，必须使用单独的 Policy、人工确认和审批分离控制。

## 8. Skill Registry 设计

当前 `buildAgentRunbook()` 的关键词分支应逐步迁移成受控 Skill Registry。

### 8.1 Skill 定义

```yaml
id: payment_schedule_intake
version: v1
display_name: 租金表导入
intent_examples:
  - 解析这份租金表
  - 导入付款计划
required_context:
  - evidence_file
optional_context:
  - contract_id
allowed_tools:
  - lease.evidence.read
  - lease.payment_schedule.create_draft
artifact_types:
  - payment_schedule_draft
review_policy:
  required: true
  blockers:
    - missing_currency
    - low_confidence_amount
    - unresolved_payment_timing
    - variable_or_non_lease_component
```

### 8.2 首批 Skill

1. `contract_batch_intake`
2. `contract_review`
3. `payment_schedule_intake`
4. `audit_pack`
5. `monthly_close_explainer`
6. `report_explainer`
7. `data_quality_review`

Skill 只负责组合工具和定义复核策略，不能自己绕过 Tool Runtime 直接访问数据库。

## 9. Pi-like Agent 与 CLI 设计

### 9.1 推荐部署形态

建议先将 Pi-like Agent 作为独立的 Agent Runner 运行：

```text
pi-agent-runner
    └── 调用 lease-agent CLI
            └── HTTP / RPC
                    └── Core Service Agent Tool Gateway
```

这样可以让 Pi-like Agent 的运行时快速迭代，同时让 Core Service 保持系统的权限和数据权威。

### 9.2 CLI 命令规范

CLI 应提供面向业务能力的命令，而不是面向底层 REST URL 的命令：

```bash
# 只读查询
lease-agent contract search --search "上海" --status approved --json
lease-agent contract get --id CONTRACT_ID --json
lease-agent measurement list --contract-id CONTRACT_ID --json
lease-agent journal list --contract-id CONTRACT_ID --period 2024-01 --json

# 草稿操作
lease-agent contract draft-create --input contract-draft.json --json
lease-agent payment-schedule draft-create --contract-id CONTRACT_ID --file-id FILE_ID --json

# 预览操作
lease-agent calculation preview --contract-id CONTRACT_ID --json
lease-agent close explain --period 2024-01 --mode official --json
```

### 9.3 CLI 输入输出约束

- `stdout` 只输出机器可读结果。
- `stderr` 输出诊断信息和调试日志。
- 默认支持 JSON；另提供人类可读格式。
- 支持 `--dry-run`，但 `dry-run` 不能代表已审批。
- 支持 `--run-id` 和 `--idempotency-key`。
- 返回稳定退出码：认证失败、权限不足、参数错误、需要复核、业务失败、系统失败应区分。
- 所有输出包含 `sources`、`review`、`trace_id` 和 `tool_call_id`。
- CLI 不接受任意 URL、SQL 或脚本参数。

### 9.4 CLI 认证

不使用长期管理员 Token。推荐使用短期 Capability Token，至少包含：

```text
subject / user_id
session_id
run_id
legal_entity_id
data_scope_hash
allowed_tools
expires_at
audience = lease-management-agent-gateway
```

服务器仍然根据登录用户和服务身份重新计算权限，不能只信任 Token 内的工具白名单。

## 10. Agent 执行生命周期

标准执行流程：

```text
1. 创建或恢复 Session
2. 创建 Run
3. 解析用户目标和 Skill
4. 构造 Agent Context
5. LLM 提议 Tool Call
6. Tool Runtime 校验权限、范围和参数
7. 执行 Tool
8. 记录 tool_execution 和 evidence
9. 将结果返回 Agent
10. 继续下一步、生成 Artifact 或请求人工确认
11. 完成 Run 或进入 waiting_review
```

### 10.1 执行预算

每个 Run 必须有：

- 最大 Tool Call 数
- 最大运行时间
- 最大模型 Token 数
- 最大结果行数
- 最大上传文件大小
- 最大重试次数
- 最大并发 Tool 数

默认建议：所有写工具串行执行，避免同一合同发生并发更新。

### 10.2 重试和幂等

工具必须区分：

- 可安全重试的读取操作
- 只能幂等重试的草稿写入
- 不允许自动重试的高风险命令

批量合同和付款计划导入必须支持：

- `idempotency_key`
- 单行成功/失败结果
- 可恢复批次
- 已成功行不重复写入
- Artifact 状态与业务写入结果一致

## 11. Artifact、Evidence 和 Review 设计

### 11.1 Artifact 统一协议

当前合同草稿和付款计划草稿应统一为：

```json
{
  "artifact_id": "artifact-id",
  "artifact_type": "contract_draft",
  "schema_version": "v1",
  "status": "ready",
  "data": {},
  "actions": [
    "confirm",
    "edit",
    "skip",
    "reject"
  ],
  "evidence_refs": [],
  "review_required": true,
  "review_reasons": []
}
```

### 11.2 Evidence 统一协议

每个 AI 识别字段应尽量带：

- `source_file_id`
- `object_name`
- `page`
- `sheet`
- `cell_range`
- `text_quote` 或文本偏移量
- `extraction_method`
- `model_version`
- `confidence`

对于没有可靠原文定位的结果，必须标记 `evidence_complete=false`，不能伪造定位信息。

### 11.3 Review Gate

以下情况必须进入 `waiting_review`：

- 折现率缺失
- 币种缺失
- 关键日期缺失或逻辑异常
- 先付/后付不明确
- 变量租金或非租赁成分未确认
- `lease_scope` 非 `in_scope` 或置信度不足
- 合同主体无法唯一匹配
- 文档证据不完整
- 批量导入存在部分失败

## 12. 数据库与持久化计划

### 12.1 复用现有表

优先复用并扩展：

- `ai_chat_sessions`
- `ai_chat_runs`
- `ai_chat_messages`
- `ai_chat_run_events`
- `ai_chat_artifacts`
- `ai_chat_review_actions`
- `ai_chat_attachments`
- `ai_contract_drafts`
- `ai_payment_schedule_drafts`

不要为了 Pi-like Agent 再复制一套平行会话表，除非外部 Agent 有完全不同的生命周期。

### 12.2 建议新增表

#### `ai_tool_executions`

用于查询和审计每次工具执行，不把所有信息都埋在 JSONB 事件中。

建议字段：

```text
id
run_id
parent_tool_execution_id
tool_name
tool_version
skill_id
call_id
input_redacted JSONB
output_summary JSONB
status
permission_decision
review_required
idempotency_key
trace_id
started_at
completed_at
error_code
error_message
created_by
```

#### `ai_capability_grants`

如果 CLI / Pi-like Agent 使用短期 Capability Token，可持久化 Token 摘要和撤销状态：

```text
id
subject_type
subject_id
session_id
run_id
legal_entity_id
scope_snapshot JSONB
allowed_tools JSONB
token_hash
expires_at
revoked_at
created_by
```

数据库中不保存明文 Token。

### 12.3 业务表写入原则

Agent Tool 不直接拼 SQL 写业务表。写入必须经过：

```text
Tool Runtime
    → Application Service
        → Domain validation
            → Repository / transaction
                → Audit log
```

合同、付款计划、事件和月结的写入都要继续遵守现有审批和锁账控制。

## 13. 权限和安全方案

### 13.1 权限决策位置

权限决策必须在 Core Service Tool Runtime 中完成，而不是：

- 由 Prompt 告诉模型“你只能看某法人”。
- 由 CLI 自己过滤数据。
- 由前端隐藏按钮。
- 由 Tool 描述文字提醒模型。

### 13.2 多租户校验

任何以 `contract_id` 为输入的 Tool，必须执行：

1. 合同存在性检查。
2. `legal_entity_id` 检查。
3. Store / Region / Brand 数据范围检查。
4. 当前用户或 Capability 的资源权限检查。
5. 再查询合同关联的付款计划、计量、分录和事件。

不能先按 ID 查询关联表，再事后判断合同权限。

### 13.3 文档 Prompt Injection 防护

上传的合同、Excel、PDF 文本属于不可信输入。必须：

- 把文档内容标记为 evidence，不当作系统指令。
- 禁止文档内容修改工具白名单或权限。
- 工具参数由服务器 Schema 校验。
- 关键字段由确定性规则重新校验。
- LLM 不得根据文件中的“请执行 SQL”或“忽略审批”指令行动。

### 13.4 敏感信息处理

- Prompt 中只放最小必要字段。
- 日志中的输入参数要脱敏。
- 不记录完整合同正文到普通运行日志。
- API Key、数据库密码、MinIO Secret 不能进入模型上下文。
- 工具结果默认分页、限量和字段裁剪。

## 14. API / Gateway 设计

CLI 可以通过 Core Service 增加一组受控 Agent Gateway 路由：

```text
POST /api/v1/agent/sessions
POST /api/v1/agent/runs
GET  /api/v1/agent/runs/:id/events
GET  /api/v1/agent/tools
POST /api/v1/agent/tool-calls
POST /api/v1/agent/runs/:id/steer
POST /api/v1/agent/runs/:id/follow-up
POST /api/v1/agent/artifacts/:id/actions
```

设计要求：

- Gateway 只暴露注册过的 Tool。
- Tool 名称和版本服务端解析，不允许客户端传入任意函数名映射到 Go 方法。
- 所有请求带 `run_id`、`trace_id` 和身份上下文。
- 事件流复用现有 SSE 机制。
- 外部 CLI 和 Web Runtime 使用相同的 Tool Runtime，不复制执行逻辑。

## 15. Core Service 代码迁移策略

### 15.1 第一阶段：建立内部接缝

新增 `internal/agenttools`，先不改变外部接口：

```text
aiagent.Agent
    → agenttools.ToolRuntime
        → application services
            → repositories
```

先把以下直接调用收口：

- 合同查询
- 计量查询
- 分录查询
- 事件查询
- 文件解析路由

### 15.2 第二阶段：迁移草稿写入

把当前前端直接调用正式业务写入 API 的逻辑改为：

```text
Review Action
    → Tool Runtime command
        → Draft Application Service
            → business API / repository
```

前端仍然可以调用同一个 Review Action API，但不能自行拼装一套独立的导入事务。

### 15.3 第三阶段：引入 CLI Adapter

CLI 只需实现：

```text
命令解析
    → ToolCall JSON
        → Agent Gateway
            → ToolRuntime.Execute
```

不应在 CLI 中重复实现租赁会计、字段规范化、审批条件或数据范围逻辑。

## 16. AI Engineering 运行策略

### 16.1 规划模型和执行模型分离

建议区分：

- Planner：判断目标、Skill 和下一步 Tool。
- Extractor：从合同/Excel/PDF 提取结构化字段。
- Analyst：解释数据、生成风险提示和审计建议。

不同任务可以使用不同模型和温度，不要用一个通用 Prompt 处理所有事务。

### 16.2 LLM 不是权限系统

Prompt 只能约束回答风格和会计谨慎原则；真正的权限、工具白名单、字段可见性、审批门槛必须由确定性代码执行。

### 16.3 结构化输出优先

所有需要进入 Artifact 的输出优先使用 JSON Schema：

- 合同字段
- 付款计划行
- Review Prompt
- Evidence Reference
- Tool 参数
- 审计包目录

文本回答可以自由生成，但业务对象不应依赖正则解析自然语言。

### 16.4 模型评估集

新增 Agent Evaluation Dataset，至少覆盖：

- 正常合同问答
- 跨法人合同 ID 攻击
- 缺少折现率
- 币种缺失
- 先付/后付混淆
- turnover rent / CAM / service fee
- 非租赁合同
- Working / Official 混淆
- 用户要求绕过审批
- 文档 Prompt Injection
- 批量导入部分失败

评估指标包括：

- Tool 选择准确率
- 参数 Schema 合法率
- 权限拒绝正确率
- 证据引用完整率
- 人工复核触发召回率
- 不应执行动作的拒绝率
- 业务字段抽取准确率

## 17. 测试计划

### 17.1 Tool Runtime 单元测试

- Tool 注册和版本解析
- 参数 Schema 校验
- 缺少权限时拒绝
- 跨法人合同拒绝
- Store / Region / Brand 范围拒绝
- 结果分页和行数限制
- 幂等键重复调用
- 工具超时和错误转换
- Review Gate 生成

### 17.2 Tool Contract Test

对每个 Tool 固定验证：

- 输入示例
- 输出示例
- 需要的权限
- 只读/写入属性
- 证据来源格式
- 失败错误码
- 是否允许自动重试

### 17.3 CLI Contract Test

- `--json` 输出可被机器解析。
- `stdout` 不混入日志。
- 退出码稳定。
- 参数错误不会触发业务写入。
- 重复调用不会重复创建草稿。
- Token 过期后不会使用缓存权限继续执行。

### 17.4 端到端测试

至少覆盖：

```text
上传合同
  → Agent 选择解析 Tool
  → 返回合同 Artifact
  → 人工确认
  → 创建 draft 合同
  → 正常提交审批
```

以及：

```text
Pi Agent
  → CLI
  → Tool Gateway
  → 跨法人请求被拒绝
  → 审计记录完整
```

### 17.5 回归测试

继续保持现有 IFRS 16 计量回归测试不被 Agent 代码改变。Agent 只能调用计量能力或读取结果，不能改变计量算法标准答案。

## 18. 观测和运维

### 18.1 需要观测的实体

- Agent Session
- Agent Run
- Skill
- Tool Execution
- Artifact
- Review Action
- Business Transaction

### 18.2 关键指标

- Run 成功率、失败率、取消率
- 平均工具调用数
- Tool 超时率
- Tool 权限拒绝率
- 人工复核率
- 草稿确认率、跳过率、退回率
- 草稿转正式审批的成功率
- LLM Token 和成本
- 单次任务耗时
- 跨法人拒绝次数
- 重复幂等命中次数

### 18.3 Trace 关联

每次调用都应能通过以下链路回溯：

```text
user_id
  → session_id
    → run_id
      → tool_execution_id
        → artifact_id / business_record_id
          → audit_log_id
```

## 19. 分阶段实施计划

### Phase 0：基线和安全修复

目标：在增加新 Agent 之前，先把现有调用链的风险封住。

任务：

- 统一合同、付款计划、计量、分录、事件的数据范围校验。
- 修复 Agent 使用合同 ID 查询关联表时的访问校验顺序。
- 增加跨法人和跨数据范围测试。
- 明确 AI Chat、CLI、Pi Runner 的身份模型。
- 固化 Agent 禁止动作清单。

完成标准：

- 任意不在权限范围内的合同 ID 都不会产生上下文、来源或 Artifact。
- 关键读工具均有权限测试。
- 高风险业务 API 仍不能由 AI Chat 权限直接触发。

### Phase 1：抽取 Tool Runtime

目标：将当前 Agent 的直接 Repository 调用和工具路由收口。

任务：

- 新增 `internal/agenttools`。
- 实现 `ToolRuntime`、`ToolRegistry`、`ToolPolicy`、`ToolResult`。
- 迁移合同、计量、分录、事件、付款计划查询。
- 将文件解析工具接入 Registry。
- 记录 `ai_tool_executions` 或等价事件。
- 保持现有 `/api/v1/ai/chat` 行为兼容。

完成标准：

- Web Agent 的读操作不再直接调用 Repository。
- 每个工具都有版本、权限、数据范围和测试。
- 现有 AI Chat 回归测试全部通过。

### Phase 2：统一 Artifact 和草稿写入

目标：消除前端、CLI、Agent 各自实现写入流程的问题。

任务：

- 统一合同草稿和付款计划草稿 Artifact Schema。
- 增加 Evidence Reference Schema。
- 将 Review Action 与 Draft Application Service 连接。
- 将批量合同导入和付款计划导入改造成服务端可恢复操作。
- 增加幂等键、批次结果和部分失败处理。

完成标准：

- Web 和未来 CLI 使用同一个草稿写入工具。
- Agent 不能绕过 Review Gate 直接创建 Approved 数据。
- 导入失败可以重试而不会重复成功行。

### Phase 3：Skill Registry

目标：减少 `buildAgentRunbook()` 的硬编码分支。

任务：

- 建立 Skill 定义和版本。
- 将合同批量导入、合同复核、租金表导入、审计包迁移到 Registry。
- Skill 定义允许工具白名单、必需上下文和 Review Policy。
- 增加 Skill Contract Test。

完成标准：

- 新增一个 Skill 不需要修改多个 Handler 分支。
- Skill 的工具权限和复核策略可被测试和审计。

### Phase 4：CLI Adapter

目标：让 Pi-like Agent 和定时任务调用受控系统能力。

任务：

- 创建 `lease-agent` CLI。
- 实现 `contract`、`payment-schedule`、`measurement`、`journal`、`event`、`report` 子命令。
- 接入短期 Capability Token。
- 支持 JSON 输出、退出码、Trace ID 和幂等键。
- 增加 CLI Contract Test。
- 在受控环境中运行 Pi-like Agent Runner。

完成标准：

- CLI 不包含业务规则复制。
- CLI 无任意 URL、SQL、Shell 执行能力。
- CLI 调用产生同样的 Tool Execution 和 Audit Trace。

### Phase 5：Pi-like Agent 高级运行时

目标：提升多步任务体验，而不是扩大业务写权限。

任务：

- `steer` 和 `follow-up`
- Run checkpoint
- 会话恢复
- 会计安全的上下文压缩
- 工具执行预算
- 可取消和可恢复任务
- 任务队列和后台执行

完成标准：

- Agent 中断、重试和恢复不会破坏业务状态。
- 压缩上下文后仍保留金额、期间、合同编号和证据引用。
- 所有写操作仍停在 Draft 或 Review Gate。

### Phase 6：有限自动化

目标：在明确的风险边界内启用部分自动化。

候选范围：

- 只读审计包准备
- 数据质量扫描
- 月结异常解释
- 非正式报告摘要
- 低风险、可回滚的草稿生成

暂不自动化：

- 合同正式审批
- 会计分录过账
- 期间锁账
- ERP 凭证回写
- 重大租赁范围或折现率判断

## 20. 任务拆分清单

### Architecture

- [ ] ADR：决定 Tool Runtime 作为 Core Service 内部深模块。
- [ ] ADR：决定 CLI 是 Adapter，不复制业务逻辑。
- [ ] ADR：定义 Agent Capability Token 和身份模型。
- [ ] ADR：定义 Agent Tool 分级和禁止动作。
- [ ] ADR：定义 Artifact / Evidence / Review 协议。

### Core Service

- [ ] 创建 `internal/agenttools` 目录和接口。
- [ ] 实现 Tool Registry。
- [ ] 实现 Tool Policy。
- [ ] 实现统一数据范围检查。
- [ ] 迁移合同查询 Tool。
- [ ] 迁移计量查询 Tool。
- [ ] 迁移分录查询 Tool。
- [ ] 迁移事件查询 Tool。
- [ ] 迁移文件解析 Tool。
- [ ] 统一 Tool Execution 审计。
- [ ] 统一 Artifact 创建和状态流转。
- [ ] 统一草稿导入事务和幂等。
- [ ] 增加 Agent Gateway 路由。

### AI Service / Agent Runner

- [ ] 保持 AI Service 只负责模型和文档解析。
- [ ] 增加结构化 Tool Call 适配。
- [ ] 定义 Pi-like Agent Runner 的 Session / Run 接口。
- [ ] 实现 Agent Runner 的预算、取消和恢复。
- [ ] 禁止 Runner 直接连接 PostgreSQL 和 MinIO 管理端。

### CLI

- [ ] 初始化 `lease-agent` 命令。
- [ ] 定义 JSON 输入输出协议。
- [ ] 定义退出码。
- [ ] 实现认证和 Token 刷新。
- [ ] 实现 Tool 发现命令。
- [ ] 实现只读 Tool。
- [ ] 实现草稿 Tool。
- [ ] 增加 `--dry-run`、`--run-id`、`--idempotency-key`。
- [ ] 增加 CLI Contract Test。

### Web

- [ ] 使用统一 Artifact Schema。
- [ ] 使用统一 Review Action。
- [ ] 将导入动作迁移到服务端 Tool / Draft Service。
- [ ] 增加 Evidence 面板。
- [ ] 展示 Tool Execution、Trace ID 和 Review Gate。
- [ ] 支持 Run 取消、Steer 和 Follow-up。

### Testing / Operations

- [ ] 跨法人越权测试。
- [ ] Prompt Injection 测试。
- [ ] 幂等和部分失败测试。
- [ ] Tool 超时和重试测试。
- [ ] 计量回归测试。
- [ ] CLI E2E 测试。
- [ ] Tool 成本和延迟监控。
- [ ] Audit Trace 查询能力。

## 21. 验收标准

### 功能验收

- Web Agent 和 CLI 可以调用同一套 Tool。
- 同一个 Tool 在不同入口返回相同的业务语义和错误码。
- Agent 可以完成合同查询、付款计划草稿、合同草稿和审计包准备。
- Agent 可以中途暂停、继续和追加指令。

### 安全验收

- Agent 无法执行任意 SQL、HTTP 或 Shell。
- Agent 无法读取跨法人、跨门店或跨区域数据。
- Agent 无法绕过合同审批、事件审批、月结审批和锁账控制。
- 文件中的 Prompt Injection 不会改变工具白名单或权限。
- Capability Token 过期、撤销或超出范围后立即失效。

### 可审计验收

- 每个 Tool Call 都有执行人、Run、时间、版本、权限决定、输入摘要、输出摘要和来源。
- 每个 Draft 都能追溯到原始文件、证据定位、模型版本和规则版本。
- 每个正式业务记录都能追溯到对应的人工 Review Action。

### 工程验收

- Tool Runtime 可使用 Fake Adapter 进行单元测试。
- Web、CLI 和 Agent Runner 不需要复制业务规则。
- 新增一个只读 Tool 不需要修改多个 Handler。
- 现有 `go test ./...`、前端类型检查、构建和 IFRS 16 回归测试继续通过。

## 22. 主要风险与缓解措施

| 风险 | 表现 | 缓解措施 |
|---|---|---|
| 增加一层后变复杂 | Agent、CLI、Gateway、Core 多层重复 | Tool Runtime 做成深模块，CLI 只做 Adapter |
| 权限漂移 | CLI 和 Web 规则不同 | 权限只在 Core Tool Runtime 决策 |
| 工具泛滥 | 所有 API 都暴露给模型 | 只注册业务级深 Tool，拒绝 REST 自动映射 |
| 写入重复 | 重试造成重复合同/付款计划 | 幂等键、批次事务、成功行记录 |
| 模型幻觉 | AI 生成不存在的字段或金额 | Schema、证据、确定性校验和 Review Gate |
| Prompt Injection | 合同文本诱导 Agent 执行危险动作 | 文档只作为 evidence，服务器校验 Tool Call |
| 运行成本上升 | 多步工具循环增加 Token 和延迟 | Tool 预算、模型分级、缓存和分页 |
| 旧链路不一致 | Web 直接写入，CLI 走新链路 | Phase 2 统一 Draft Service，设置迁移截止点 |
| 运行态难恢复 | CLI 进程退出导致任务状态不明 | Server-side Run、checkpoint、幂等和事件持久化 |

## 23. 关键设计决策

### 决策 1：Core Service 保留业务权威

Agent、Pi、CLI 都不能成为第二个业务后端。Core Service 的业务服务、权限、审批、锁账和审计仍是唯一权威。

### 决策 2：Tool 采用业务语义，不采用 REST 镜像

优先提供 `lease.contract.search`，而不是把 `/api/v1/contracts` 原样暴露给 Agent。业务语义工具可以隐藏分页、字段裁剪、权限和来源组装，接口更深，调用方更简单。

### 决策 3：先内嵌 Tool Runtime，再考虑独立 Agent Service

当前规模下，先在 Core Service 内建立接缝，可以获得足够的复用和测试收益。只有当外部 Agent 数量、任务队列或隔离需求明显增加时，才拆成独立 Agent Gateway。

### 决策 4：CLI 不等于 Auto-Post

CLI 只是调用方式。是否允许写入、审批或过账，必须由工具等级和权限策略决定，不能因为来自终端就获得更高权限。

## 24. 最终推荐顺序

建议实际开发顺序为：

```text
P0 统一合同/关联数据权限校验
  → P1 Tool Runtime + Registry
  → P1 Artifact / Evidence / Review 统一
  → P1 草稿写入服务端收口
  → P2 Skill Registry
  → P2 CLI Adapter
  → P2 Pi-like Agent Runner
  → P3 steer / follow-up / checkpoint / compaction
  → P4 有限自动化
```

如果只能先做一项，优先做：

> `Tool Runtime + 统一权限校验 + Tool Execution 审计`

这是未来 Web Agent、Pi-like Agent、CLI、定时任务和外部 ERP 自动化共同依赖的唯一深模块接缝。
