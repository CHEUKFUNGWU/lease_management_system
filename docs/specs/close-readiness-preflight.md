# Close Readiness Preflight

> 规格状态：Ready for implementation
> 来源：`docs/财务业务伙伴与FP&A产品战略分析.md`  以及现有 Month-End Close、Work Queue 和领域 ADR
> 本规格对应的第一条业务垂直切片：让 Finance Analyst 在月结前看到“哪些问题会阻塞本期间工作、为什么阻塞、下一步如何处理”。

## Problem Statement

当前系统已经能够执行月结、生成计量结果和会计分录，也有一个跨合同、事件、分录及关键日期的待办队列。但 Finance Analyst 在正式点击月结前，仍然缺少一个面向“会计期间”的准备度判断。

用户需要自己逐份检查：

- 已批准且属于 IFRS 16 范围的合同是否缺少付款计划。
- 合同是否缺少已确认的折现率。
- 生效日在本期间或以前、但仍未完成审批的事件是否会影响本次月结。
- 本期间是否已经存在失败的月结批次。
- 当前用户看到的合同范围是否足以代表整个 Legal Entity 的 Close Population。

如果这些问题在生成月结之后才被发现，用户会遇到重复跑批、手工追查、错误分录、期间锁账延误和审计解释困难等问题。

这不是单纯的“错误提示”问题。根据领域模型，Close Period 由 Legal Entity 和 accounting period 共同定义；Close Batch 只能是部分执行单元，不能独立宣布整个期间清洁或授权锁账。因此系统需要提供清晰的期间级诊断，同时避免把部分用户范围误表示为完整的 Close Readiness。

## Solution

新增一个只读的 Close Readiness Preflight 能力，作为月结前的诊断入口：

1. 用户选择 accounting period。
2. 系统加载当前用户可访问的 approved lease contract population。
3. 系统按固定的、可解释的 Close Control Rule 检查关键会计前置条件。
4. 系统返回总体状态、覆盖范围、规则计数和合同级 finding。
5. 用户可以从 finding 直接跳转到合同详情、事件审批或月结页面处理。
6. 用户修正数据后可以重新运行诊断，并看到结果变化。

V1 只提供确定性的只读诊断，不创建正式 Close Exception，不写入会计数据，不改变审批、重算、过账或锁账状态。正式 Close Exception、Detection Event、Disposition、Waiver 和锁账闸门属于后续规格，必须在 Projection Version、Close Control Profile 和完整异常治理模型具备后再实现。

V1 的产品原则是：

> 让问题尽早暴露，让每个问题可解释，但不把诊断结果冒充为正式控制结论。

## User Stories

1. As a Finance Analyst, I want to select a Legal Entity accounting period, so that I can review the month-end preparation state before generating a close.
2. As a Finance Analyst, I want to see the number of contracts evaluated, so that I know whether the diagnostic covered the population I expected.
3. As a Finance Analyst, I want to see whether the current data scope is complete or limited, so that I do not mistake a regional view for a Legal Entity-wide close conclusion.
4. As a Finance Analyst, I want contracts without an approved payment schedule to be identified, so that I can correct source data before measurement.
5. As a Finance Analyst, I want contracts without a confirmed discount rate to be identified, so that the system never silently uses an unsupported rate.
6. As a Finance Analyst, I want unapproved events effective in the selected period to be identified, so that approved close calculations do not omit known lease changes.
7. As a Finance Analyst, I want failed close batches for the selected period to be visible, so that I can investigate failed subjects instead of running the same close repeatedly.
8. As a Finance Analyst, I want each finding to include a rule code and plain-language reason, so that I understand what failed without reading SQL or logs.
9. As a Finance Analyst, I want each finding to include a recommended remediation, so that the work queue points me toward the next valid workflow.
10. As a Finance Analyst, I want findings to link to the affected contract or month-end close page, so that I can resolve the issue without manually searching.
11. As a Finance Reviewer, I want to see the same deterministic findings within my authorized Legal Entity and Data Scope, so that review discussions use a common evidence set.
12. As a Finance Approver, I want the UI to distinguish diagnostics from formal approval or lock authority, so that I do not treat a green preflight as an approval decision.
13. As a Finance Business Partner, I want the preflight to surface the population and blocking count before the close, so that I can understand whether management reporting depends on incomplete lease data.
14. As an FP&A user, I want the selected period and evaluation timestamp to be visible, so that I can distinguish current diagnostic data from a frozen budget or Official report.
15. As an Auditor, I want the finding rule, source subject and diagnostic timestamp to be visible, so that I can understand how the preparation view was produced.
16. As a Legal Entity administrator, I want a legal-entity-wide user to see the full population, so that close preparation is not inferred from a partial regional or brand scope.
17. As a user, I want an invalid period to be rejected with a clear message, so that I cannot accidentally run a diagnostic for an ambiguous period.
18. As a user, I want an empty population to be clearly shown as “not evaluated” rather than “ready”, so that no-contract situations are not mistaken for a clean close.
19. As a user, I want a refresh action, so that the view reflects corrections made in the contract, payment schedule or event workflow.
20. As a product owner, I want the diagnostic rules to be deterministic and versionable, so that future formal Close Control Rules can reuse the same business vocabulary without making the Agent authoritative.

## Implementation Decisions

### Product and domain boundary

- The new capability is named Close Readiness Preflight, not Close Exception Management.
- A V1 finding is diagnostic output. It is not a persisted Close Exception, Detection Event, Control Conclusion, Verified Resolution, Accounting Conclusion, Period Waiver or Standing Waiver.
- The endpoint is read-only and cannot generate measurement results, journals, approvals, postings, ERP writeback or period locks.
- The product must explicitly show that readiness is a preparation view and that final period lock remains governed by the existing close workflow.

### Rules in V1

The preflight evaluates approved contracts whose lease term overlaps the selected accounting period. `not_a_lease` contracts are excluded from the IFRS 16 measurement population. Exempt contracts remain visible in the population policy but do not require a discount rate for capitalization.

The following rules are included:

| Rule code | Meaning | Severity | Gate effect | Remediation |
|---|---|---|---|---|
| `missing_payment_schedule` | An approved in-scope contract has no controlled payment schedule | Blocking | Formal calculation preparation | Add or import the payment schedule, then complete its review/approval workflow |
| `missing_discount_rate` | An approved in-scope contract has no confirmed contract or policy rate | Blocking | Formal calculation preparation | Confirm the rate through the discount-rate workflow; the engine must not guess |
| `pending_event_before_period_end` | A lease event effective on or before period end is not approved | Blocking | Formal calculation preparation | Review and approve, return, or reject the event through the event workflow |
| `failed_close_batch` | A close batch for the selected period completed with failed contracts | Blocking | Close preparation | Open the month-end close result, investigate failed subjects and rerun only after remediation |

The overall status is derived as follows:

- `not_run`: no evaluated population exists.
- `blocked`: at least one blocking finding exists.
- `ready`: the visible population has no V1 blocking findings and the scope is Legal Entity-wide.
- `scope_limited`: no blocking finding exists in the visible scope, but the current user does not have Legal Entity-wide scope; this must never be displayed as Legal Entity-wide readiness.

### Read-side seam

The highest useful seam is one application use case: `Evaluate Close Readiness`.

- The HTTP handler validates `period`, obtains the caller's Legal Entity and Data Scope, invokes the use case, and encodes the result.
- The repository provides a single read-side facts boundary for the approved population, schedule presence, pending event presence and close-batch failures. The business service owns rule evaluation and status derivation.
- The frontend calls one API operation and renders one preflight panel. It does not reconstruct findings from contracts, events or batches independently.
- The same service-level facts model can later be adapted to persisted Detection Events without moving rule definitions into HTTP handlers or React components.

### API contract

The read endpoint is exposed under the monthly-closing resource with a `period=YYYY-MM` query parameter.

The response includes:

- `accounting_period`.
- `evaluated_at`.
- `scope_complete`.
- `population_count`.
- `status`.
- `blocking_count`.
- `finding_count`.
- `findings`.

Each finding includes:

- `rule_code`.
- `severity`.
- `gate_effect`.
- `contract_id` when contract-specific.
- `contract_number` and `contract_name` when available.
- `title`.
- `reason`.
- `remediation`.
- `source_kind` and `source_id`.
- `target_path` for the UI navigation destination.

The endpoint must return a validation error for malformed periods and must respect the same Legal Entity and Data Scope filtering used by the existing work queue.

### UI interaction

- Add a Close Readiness card to the existing `/todo` task-oriented page.
- Default the period to the current accounting period, but allow the user to change it.
- Show status, population count, blocking count, evaluated timestamp and scope completeness.
- Show findings grouped by rule or severity, with blocking findings first.
- Link contract findings to the contract workspace and close-batch findings to the monthly-closing page.
- Provide refresh behavior after the user returns from a remediation page.
- Keep the visual distinction between diagnostic findings and approval/lock controls.
- Add translations for the existing supported languages without changing the accounting terminology.

### Access and audit boundary

- The endpoint requires the existing monthly-closing read permission.
- A user with a narrower Data Scope may receive a partial diagnostic, but the response must identify `scope_complete=false`.
- No new write audit event is needed for V1 because the operation is read-only; request and response metadata remain available through application logs as appropriate.
- The Agent may later consume this read-side output, but it must not be allowed to change its severity, gate effect or accounting state.

### Task decomposition

1. Define the Close Readiness facts and rule evaluation service.
2. Add the repository facts query with Legal Entity and Data Scope filtering.
3. Add the monthly-closing read endpoint and route permission.
4. Add the Close Readiness panel to the existing task page.
5. Add translations, links and empty/error states.
6. Add service, handler and repository tests.
7. Document the V1 boundary and the follow-up formal exception-management work.

## Testing Decisions

Tests must assert external behavior and domain outcomes rather than SQL fragments, private helper names or React implementation details.

### Service tests

Use in-memory facts to verify:

- Missing payment schedule creates one blocking finding.
- Missing discount rate blocks only in-scope contracts.
- Exempt and not-a-lease scope behavior is correct.
- Pending events block only when their effective date is on or before period end.
- Failed close batch creates a blocking finding.
- Multiple findings preserve deterministic ordering.
- Empty population returns `not_run`, not `ready`.
- Partial scope returns `scope_limited` rather than `ready`.
- A clean, complete Legal Entity-wide population returns `ready`.

Prior art: the pure service tests under `core-service/internal/services/budget` and `core-service/internal/services/monthend`.

### Handler tests

Verify:

- Invalid or missing period returns HTTP 400.
- The authenticated Legal Entity and Data Scope are passed to the use case.
- Successful output preserves the response contract.
- Repository or service failures return an appropriate server error without exposing SQL details.

Prior art: existing handler tests for reports snapshots, authentication and AI Chat runtime permissions.

### Repository integration tests

Use PostgreSQL-backed fixtures to verify:

- Approved-contract filtering.
- Lease-scope filtering.
- Payment-schedule presence detection.
- Pending-event date filtering.
- Failed-batch detection.
- Legal Entity and Data Scope isolation.

Prior art: repository PostgreSQL integration tests and access-policy integration tests.

### Frontend verification

Verify the visible behavior through the public API boundary:

- Loading state.
- `ready`, `blocked`, `scope_limited` and `not_run` states.
- Blocking findings appear before non-blocking information.
- Links route to the correct contract or monthly-closing destination.
- Empty and API-error states are actionable.

## Out of Scope

- Persisted Close Exception cases.
- Immutable Detection Events.
- Formal Close Control Profiles and effective-dated policy configuration.
- Verified Resolution, Accounting Conclusion, Period Waiver or Standing Waiver.
- Automatic exception reopening or fingerprinting.
- Period lock gating.
- Close Snapshot or Frozen Projection creation.
- Automatic generation or modification of payment schedules.
- Automatic approval, posting, ERP writeback or period unlock.
- Full Budget / Forecast / Actual model changes.
- Store revenue ingestion, rent-to-sales and renewal decision cards.
- AI agent autonomy or automatic remediation.
- Disclosure report package and audit pack generation.

## Further Notes

The analysis document identifies “月结异常工作台” as a high-value bridge between the existing accounting control plane and the future FP&A / Finance BP decision plane. This V1 intentionally starts with the analyst's immediate pre-close need because it has a narrow, high-trust seam and can be delivered without weakening the existing lock and approval model.

The next formal specification should introduce persisted Close Exception and Detection Event objects only after the Projection Version and Close Snapshot boundaries are available. That future work must preserve the ADR decisions that:

- Close Period is governed by Legal Entity and accounting period.
- Close Batch cannot independently declare a period clean.
- Exception state is separate from closing disposition.
- Agent Signals are separate from authoritative Control Conclusions.
- Unresolved Blocking Exceptions prevent period lock.
