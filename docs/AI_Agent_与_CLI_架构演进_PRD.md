# AI Agent 与 CLI 架构演进 PRD

> 文档状态：Implemented / Runtime Validation Complete
>
> 版本：v1.1
>
> 编制视角：AI Engineer / Platform Engineer
>
> 目标系统：线下零售经营分析工作站 / Retail Performance Workstation（IFRS 16 计量是其中一个合规模块）
>
> 日期：2026-08-08
>
> 最近验证：2026-08-10；Core Tool Runtime、CLI、Pi-like Worker、Docker Planner 闭环和 Web 设备会话管理已完成技术验收

## Problem Statement

当前系统已经具备 AI Chat、文件解析、合同/付款计划草稿、服务端会话、运行事件和人工复核能力，但 Agent 的执行接口还没有形成统一的业务能力层。

当前实现的主要问题是：

1. Agent 的意图路由、Runbook、Repository 查询、LLM 调用、文件解析路由和结果投影集中在同一组实现中，增加新技能时需要修改多个职责不同的部分。
2. 真正的 Function Calling 主要覆盖文件解析，其他 `ToolCalls` 更多是运行轨迹或展示元数据，并不是真正注册、授权、版本化和可测试的工具。
3. AI 草稿确认后，Web 前端直接调用合同和付款计划业务接口；未来如果增加 Pi-like Agent、CLI 或定时任务，容易形成多套写入流程。
4. Agent 需要访问合同、计量、分录、事件和报表，但这些访问必须始终带有用户身份、法人、门店/区域/品牌数据范围和资源权限。目前缺少一个统一的执行接缝来保证这一点。
5. 终端式 Agent 需要稳定的命令式入口，但直接让 Pi-like Agent 自由调用 HTTP、SQL 或 Shell 会扩大越权、Prompt Injection、重复写入和审计缺失风险。
6. 财务系统的 AI 输出必须严格区分系统事实、AI 信号、草稿、会计判断和正式控制结论。Agent 不能替代审批、过账、锁账或确定性计量规则。

## Solution

增加一个受控的 Agent Tool Runtime，作为所有 Agent 客户端访问租赁业务能力的唯一接缝。

目标调用关系为：

```text
Web AI Chat ───────────────┐
                           │
Pi-like Agent ── CLI ──────┼──> Agent Tool Runtime
                           │             │
定时任务 / 外部自动化 ───────┘             ├── Skill Registry
                                         ├── Permission / Scope Policy
                                         ├── Tool Executor
                                         ├── Artifact / Evidence Manager
                                         └── Audit / Trace
                                               │
                                   Core Application Services
                                               │
                                  Repository / PostgreSQL / MinIO
```

核心方案：

- Pi-like Agent 负责规划、多步任务、工具选择、上下文管理和用户交互。
- CLI 负责把终端命令转换为结构化 Tool Call，是一个 Adapter，不复制业务逻辑。
- Tool Runtime 负责工具发现、参数校验、权限、数据范围、Review Gate、幂等、超时、审计和结果裁剪。
- Core Application Services 负责合同、付款计划、事件、计量、月结和报表等业务规则。
- AI Service 继续负责 LLM、OCR 和文档结构化，不获得 PostgreSQL 访问权限。
- Agent 产生的写入结果默认是 Draft 或 Artifact；正式状态仍由既有审批和锁账流程产生。

本方案不会立即重写现有 AI Chat。优先复用现有 Session、Run、SSE、Artifact、Review Action 和 Assist Mode，把现有直接调用逐步迁移到 Tool Runtime。

## User Stories

### Agent 使用者

1. As a finance editor, I want to ask the Agent to search contracts within my authorized scope, so that I do not need to manually navigate multiple filters.
2. As a finance editor, I want to open a contract through the Agent, so that the Agent can explain its dates, parties, scope, discount rate and approval status using system facts.
3. As a finance editor, I want to ask for a contract's payment schedule, so that I can verify periods, amounts, timing and accounting attributes before import.
4. As a finance editor, I want to ask for a contract's measurement results, so that I can reconcile opening liability, interest, payment, depreciation and closing ROU.
5. As a finance editor, I want to ask for journal entries for a period, so that I can review the accounting impact before month-end approval.
6. As a finance reviewer, I want the Agent to show evidence references for every extracted field, so that I can review the source instead of trusting an unsupported answer.
7. As a finance reviewer, I want the Agent to identify missing discount rates, currencies, dates and payment timing, so that I can resolve data quality issues before approval.
8. As a finance reviewer, I want the Agent to distinguish fixed rent, variable rent and non-lease components, so that the liability calculation is not contaminated by incorrectly capitalized amounts.
9. As a finance reviewer, I want the Agent to highlight uncertain lease-scope judgments, so that short-term, low-value and non-lease classifications receive human review.
10. As a finance approver, I want Agent output to stop at a reviewable draft, so that no model response can silently create an approved contract or posted entry.
11. As an auditor, I want to inspect the Agent run, tool calls, source evidence, model version, rule version and review actions, so that the analysis is reproducible.
12. As an auditor, I want Working and Official data to be visibly separated, so that AI explanations do not make trial data appear authoritative.
13. As a business readonly user, I want the Agent to respect my legal entity, store, region and brand scope, so that asking a broader question does not expand my visibility.
14. As a system administrator, I want to enable or disable a skill by role and permission, so that new Agent capabilities do not automatically become available to every user.
15. As a user, I want to correct the Agent while a long task is running, so that I can avoid waiting for a task that has taken the wrong interpretation.
16. As a user, I want to continue a completed run with a follow-up instruction, so that I can refine an analysis without losing its evidence and history.

### Pi-like Agent / CLI 使用者

17. As a Pi-like Agent, I want to discover only the tools allowed for my current capability grant, so that tool planning cannot select an unauthorized operation.
18. As a Pi-like Agent, I want to call business-semantic tools instead of raw REST endpoints, so that I can work with stable domain concepts and smaller prompts.
19. As a Pi-like Agent, I want tool results to include bounded data, sources, review status and retry guidance, so that I can decide the next step safely.
20. As a Pi-like Agent, I want to create a contract draft from a reviewed extraction, so that the business system receives a traceable draft rather than an opaque mutation.
21. As a Pi-like Agent, I want to import a payment schedule idempotently, so that retrying after a timeout does not duplicate successful rows.
22. As a CLI user, I want machine-readable JSON output, so that shell workflows and tests can consume results without parsing prose.
23. As a CLI user, I want stable exit codes, so that authentication failure, permission denial, review required and business failure can be handled differently.
24. As a CLI user, I want `dry-run`, `run-id` and `idempotency-key` options, so that automation can preview and safely retry operations.
25. As a CLI user, I want diagnostic logs on stderr and results on stdout, so that pipelines remain deterministic.
26. As an operator, I want CLI calls to use short-lived capability credentials, so that a leaked token has limited scope and lifetime.
27. As an operator, I want every CLI call to create the same execution trace as a Web Agent call, so that multiple entry points remain auditable.
28. As an operator, I want the CLI to reject arbitrary SQL, URLs and Shell commands, so that a terminal Agent cannot turn the system into a general-purpose remote executor.

### 系统与控制

29. As a product owner, I want one Tool Runtime shared by Web, Pi, CLI and scheduled jobs, so that fixes to permissions, business rules and audit behavior apply to every caller.
30. As an AI engineer, I want a versioned Skill Registry, so that adding a skill does not require expanding a large keyword switch or duplicating tool chains.
31. As an AI engineer, I want structured Tool Call and Artifact schemas, so that business objects do not depend on regular-expression parsing of model prose.
32. As an AI engineer, I want separate planner, extractor and analyst model roles, so that model settings can match task risk and quality requirements.
33. As an AI engineer, I want an evaluation dataset for routing, extraction, permissions, evidence and refusal behavior, so that model changes can be compared against a stable baseline.
34. As a backend engineer, I want Tool Runtime to use application services rather than Repository access as its external seam, so that transaction rules and domain validation remain centralized.
35. As a backend engineer, I want a fake Tool Adapter seam, so that Tool Runtime and Agent orchestration can be tested without requiring PostgreSQL, MinIO or an LLM.
36. As a security engineer, I want server-side scope checks before loading any contract-linked data, so that a caller cannot use a foreign contract ID to reach measurement or event data.
37. As a security engineer, I want document content treated as untrusted evidence, so that Prompt Injection inside a contract cannot alter tool policy.
38. As a finance controller, I want high-risk commands such as approval, posting, period lock and ERP writeback to remain separately authorized, so that Agent convenience does not weaken segregation of duties.
39. As an operator, I want tool budgets, timeouts, retry rules and result limits, so that a multi-step Agent cannot exhaust system resources.
40. As an operator, I want traceable metrics for tool latency, rejection, review rate, cost and failure, so that the Agent platform can be operated as a production system.
41. As a release engineer, I want existing IFRS 16 regression tests to remain independent of Agent behavior, so that AI changes cannot change accounting calculation standards.
42. As a release engineer, I want the migration to preserve existing Web AI Chat behavior while the new Tool Runtime is introduced, so that the architecture can be rolled out incrementally.

## Implementation Decisions

### 1. 外部接缝选择

The highest useful seam is `ToolRuntime.Execute` plus the Tool descriptor contract. Web Agent, Pi-like Agent and CLI all become adapters to this seam. Repositories remain internal adapters behind application services and are not exposed to Agent clients.

The Tool Runtime interface must include not only method parameters, but also permission requirements, data-scope behavior, read/write classification, review requirements, retry semantics, timeout behavior, pagination, error categories, source references and versioning.

### 2. 运行时位置

The first implementation stays inside Core Service to preserve locality for business rules, access scope and audit. A separate Agent Runner may host the Pi-like runtime, but it must call Core Service through the Tool Gateway and must not receive database credentials.

### 3. Tool 版本与注册

Tools are registered in reviewed code or a controlled registry, not discovered from arbitrary HTTP routes. Each Tool has a stable domain name, version, JSON input schema, bounded output schema, required permissions, allowed data scope, review policy and retry policy.

The first read tools are contract search/get, payment schedule list, measurement list, journal list, event list, report preview and audit list. The first write-capable tools create drafts or reviewable Artifacts only.

### 4. Skill Registry

Skills compose Tools and define intent examples, required evidence, required page or contract context, Artifact types, review blockers and allowed roles. Skills do not bypass Tool Runtime or call repositories directly.

The first Skills are contract batch intake, contract review, payment schedule intake, audit pack preparation, monthly close explanation, report explanation and data-quality review.

### 5. Tool Call contract

Every Tool Call carries a run identifier, call identifier, Tool name, Tool version, structured arguments, optional idempotency key and dry-run flag. Identity, role, legal entity and data scope are derived on the server and cannot be supplied as trusted arguments by the Agent.

Tool results contain bounded data, status, sources, review state, retry guidance and a structured error. Large result sets use pagination and explicit limits; the full source remains in the system of record rather than being copied into an unbounded prompt.

### 6. CLI contract

The CLI exposes domain commands such as contract search, measurement list, journal list, draft creation and report preview. It translates commands to Tool Calls and does not implement contract validation, accounting logic, approval rules or SQL.

The CLI emits JSON on stdout and diagnostics on stderr, supports stable exit codes, includes run and trace identifiers, supports dry-run and idempotency options, and rejects arbitrary URL, SQL or shell arguments.

### 7. Identity and capability

Browser calls continue to use the existing user identity and permission middleware. Pi-like Agent and CLI calls use short-lived capability credentials bound to a subject, session or run, legal entity, data-scope snapshot, allowed Tool set, audience and expiry. The server recomputes authorization and never trusts a client-provided role or tenant field.

### 8. Draft and Artifact behavior

AI output that may affect business data is represented as a versioned Artifact with structured data, actions, source evidence, confidence, review reasons, model version and rule version. Confirming an Artifact invokes the same Draft Application Service for every caller.

Contract, payment schedule and event creation remain draft-first. Approval, posting, period lock, unlock and ERP writeback are not default Agent Tools and remain governed by existing permission and approval-separation controls.

### 9. Database persistence

Existing Agent session, run, message, event, Artifact, review-action and attachment records are reused. A dedicated Tool Execution record is added when querying execution history, permission decisions, idempotency, latency and error categories from event JSON would otherwise be difficult.

If external capability grants require revocation and audit, a capability-grant persistence record stores token hashes and scope snapshots but never stores plaintext credentials.

Refresh credentials are persisted only as hashes. The Core API exposes owner-scoped device-session listing and revocation, while the Web Settings page provides the operator-facing session control surface. Revoking all sessions does not grant any additional business permission and does not bypass the short-lived access-token boundary.

### 10. Data-scope enforcement

All contract-linked reads first validate the contract against the caller's legal entity and store/region/brand scope. Only after this check may the implementation load payment schedules, measurement results, journal entries, events or documents. This prevents a foreign contract identifier from becoming an indirect lookup channel.

### 11. Document and Prompt Injection handling

Uploaded documents are untrusted evidence. They may provide facts and citations but cannot modify system instructions, Tool Registry, permissions, capability grants or review policy. Tool arguments are server-validated and critical accounting fields are checked deterministically after extraction.

### 12. Model roles

Planner, extractor and analyst responsibilities are separated. Extraction uses structured output and low-temperature settings; the analyst explains bounded system facts; the planner selects only tools permitted by the server-provided registry. Model choice, temperature, token budget and retry policy are versioned per Skill.

### 13. Write transactions and idempotency

Draft creation and batch imports use server-side application services. Each external write has an idempotency key and a result per input item. Successful items are not repeated after a retry, failed items are resumable, and Artifact status reflects the business write outcome.

### 14. Event-driven execution

Tool execution emits start, update, end, review-required, Artifact-ready and error events. Existing Run and SSE mechanisms remain the transport for Web and can be reused by CLI or Pi Runner through a polling or streaming Adapter.

### 15. Testing seam

The primary behavior seam is Tool Runtime. Tests validate external behavior through Tool descriptors and Execute results. Application Services are tested with repository adapters, while Agent planning can use a fake Tool Runtime and deterministic model adapter.

## Testing Decisions

### 1. Test external behavior

Tests should assert what an Agent client can discover, execute, read, write, retry and observe. They should not assert private helper ordering or the exact internal class layout.

### 2. Tool Runtime tests

Cover:

- Tool discovery and version resolution.
- Input schema validation.
- Permission denial.
- Legal entity and fine-grained data-scope denial.
- Contract-linked read ordering: access check before related data load.
- Read result pagination and row limits.
- Review Gate and blocker generation.
- Dry-run semantics.
- Idempotency behavior.
- Timeout, retry and error classification.
- Evidence and source propagation.

### 3. Application Service tests

Cover domain behavior for contract draft creation, payment schedule draft creation, event draft creation, Artifact confirmation, partial batch failure and audit transaction boundaries. The tests should use fake Repository adapters where possible and PostgreSQL integration tests for SQL and transaction behavior.

### 4. CLI contract tests

Cover JSON output, stderr separation, stable exit codes, capability expiration, permission denial, malformed arguments, dry-run, idempotency and retry behavior. CLI tests should invoke the CLI process against a test Tool Gateway rather than mock every command parser function.

### 5. Agent planning tests

Cover Skill selection, allowed Tool filtering, maximum step budget, review-required routing, follow-up behavior and refusal of unsupported operations. Planning tests use deterministic model responses or a fake planner and do not require a live LLM.

### 6. AI evaluation tests

Maintain a versioned dataset covering routing, extraction, confidence, missing discount rate, missing currency, payment timing, variable/non-lease components, scope classification, Working/Official distinction, Prompt Injection and refusal of control bypass.

Track Tool selection accuracy, argument validity, permission refusal accuracy, evidence completeness, review-gate recall, unsupported-action refusal and extraction accuracy.

### 7. Regression and end-to-end tests

The existing IFRS 16 measurement regression remains an independent gate. End-to-end tests cover upload → parse → Artifact → review → draft creation → approval workflow, plus Pi Runner → CLI → Tool Gateway → access denial and full audit trace.

### 8. Observability tests

Verify that every Tool Call can be traced from user, session and run to Tool Execution, Artifact or business record, and audit record. Logs must be redacted and must not contain credentials or unrestricted contract content.

### 9. Prior art

Reuse existing tests for access scope, protected routes, AI chat Runtime, AI intake producer, contract batch parsing, payment schedule parsing, monthly closing, approval separation and IFRS 16 regression. New tests should cross the highest Tool Runtime seam before adding lower-level tests.

## Out of Scope

The following are explicitly excluded from the first delivery:

- Direct PostgreSQL, MinIO-admin, arbitrary SQL, arbitrary HTTP and arbitrary Shell access for Agent clients.
- An open third-party Tool marketplace or arbitrary extension installation.
- Automatic contract approval, journal posting, period lock/unlock or ERP writeback.
- Replacement of the IFRS 16 measurement engine with an LLM.
- Replacement of deterministic approval, lock-control or close-control rules with an Agent.
- A complete rewrite of the Web AI Chat UI.
- A full distributed Agent platform before Tool Runtime proves its value in Core Service.
- General-purpose coding-agent behavior unrelated to lease management.
- Automatically changing accounting policy based on model output.

## Further Notes

### Recommended delivery sequence

1. Close the contract-linked data-scope gap and add adversarial access tests.
2. Introduce Tool Runtime, Registry, Policy and Tool Execution audit while keeping existing AI Chat contracts compatible.
3. Migrate contract, measurement, journal, event and file parsing reads to Tools.
4. Unify Artifact, Evidence and Review Action behavior for Web and future external callers.
5. Move draft creation and batch import into server-side Draft Application Services with idempotency.
6. Replace keyword-only Runbook routing with a versioned Skill Registry.
7. Release a constrained `lease-agent` CLI as an Adapter to the Tool Gateway.
8. Run a Pi-like Agent in a separate Runner process without database credentials.
9. Add steer, follow-up, checkpoint, cancellation and accounting-safe compaction.
10. Evaluate limited automation only after security, audit, regression and human-review gates are proven.

### Implementation issue slicing

The implementation should be delivered as vertical slices, in this order:

1. Access-safe contract context Tool.
2. Read-only contract, measurement, journal and event Tools.
3. File parsing Tools with structured intake Artifact.
4. Server-side contract draft creation.
5. Server-side payment schedule draft creation with idempotent batch behavior.
6. Skill Registry for the first four Skills.
7. Tool Execution audit and observability queries.
8. CLI read-only commands.
9. CLI draft commands.
10. Pi-like Runner integration.
11. Streaming intervention and advanced runtime capabilities.

### Current implementation evidence

- Core, CLI and Runner use the same versioned Tool descriptors, server-derived identity/scope, review gates and audit/event contracts.
- The optional Docker Worker has completed a real Planner → Worker → Gateway → Tool → checkpoint/event run without database or MinIO credentials.
- The repository test gates include Go packages, AI intake/planner tests, Web type-check/tests, skill evaluation and IFRS 16 regression.
- Persisted `planner_usage` aggregation is exposed through the permissioned `GET /api/v1/agent/usage` endpoint and the Web `/agent-metrics` operator page; the summary derives user/legal-entity scope server-side and keeps unavailable cost explicit.
- The repository includes Prometheus recording/alert rules, a bearer-token scrape template, startup validation for the LLM price book, and an operations runbook covering health checks, LLM usage/cost semantics, release smoke tests and application/database rollback boundaries.
- The remaining environment/customer gates are enumerated in the external acceptance checklist (archived 2026-08-18) with owners, required inputs, evidence and pass criteria; they are intentionally not represented as completed by local tests.
- Remaining acceptance work is operational or customer-specific: production metrics baseline calibration and approved rollback drill, real PaddleOCR sample coverage, ERP field mapping and third-party accounting sign-off.

### Decisions requiring explicit review before high-risk expansion

- Whether any client will receive a capability that can submit a draft for review automatically.
- Whether close draft generation may be initiated by a scheduled Agent.
- What threshold, if any, permits low-risk bulk draft creation without row-level confirmation.
- How customer ERP identities map to Agent capability subjects.
- Whether external Agent execution must be isolated in a separate network segment.

### Definition of done

The architecture evolution is complete only when:

- Web, CLI and Pi-like Runner use the same Tool Runtime semantics.
- No external Agent has direct database or unrestricted command access.
- Contract-linked reads enforce data scope before loading related records.
- Draft writes are idempotent, reviewable and auditable.
- High-risk accounting controls remain outside default Agent execution.
- Tool, Skill, Artifact, Evidence, Review and capability contracts have tests.
- Existing build, type-check, Go tests and IFRS 16 regression gates remain green.
- A production operator can trace a Tool Call from user intent to final business record or review decision.
