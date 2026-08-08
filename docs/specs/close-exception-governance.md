# Close Exception Governance

> 规格状态：Ready for implementation
> 对应任务：P0.2 正式异常治理
> 依赖：`close-readiness-preflight.md` 的确定性规则读取边界

## Problem Statement

Close Readiness Preflight 能告诉 Finance Analyst 哪些合同或月结批次存在问题，但它是一次性的只读诊断。若用户通过 Excel 或聊天记录自行跟踪问题，系统就无法回答：

- 这个问题来自哪一版规则、哪一次检测和哪一份投影事实。
- 同一个问题在不同用户、不同重跑之间是否还是同一个异常。
- 异常目前是正在调查、已解决，还是只是被允许带过本期间。
- 解决依据是否经过复核和审批，且处理人是否与发现人职责分离。
- 期间锁账前是否仍有未解决的 Blocking Exception。

因此需要把“诊断发现”升级为可追溯、可治理的正式 Close Exception，同时保持 Exception State 与 Closing Disposition 分离，避免“已关闭系统记录”被误解为“会计处理已经正确”。

## Solution

新增 Close Exception Governance 模块：

1. 以版本化 Close Control Rule 定义规则代码、严重级别、闸门效果和有效版本。
2. 每次运行确定性检测时写入 Detection Event，并关联 `projection_version`、规则版本、证据和稳定 fingerprint。
3. 对相同 fingerprint 维护一条 Close Exception，避免每次刷新产生重复异常。
4. 通过明确的治理动作维护 Exception State 与 Closing Disposition：分派、验证解决、会计结论、期间豁免、永久豁免和关闭。
5. 记录 owner、reviewer、approver，禁止同一人同时承担相互冲突的职责。
6. 期间锁账时检查当前法人和期间是否存在未解决的 Blocking Exception；存在时拒绝锁账并返回可操作的阻塞数量。

V1 检测复用 P0.1 的四条规则：`missing_payment_schedule`、`missing_discount_rate`、`pending_event_before_period_end`、`failed_close_batch`。V1 不自动修改合同、付款计划、事件、计量结果或分录。

## User Stories

1. As a Finance Analyst, I want to run a formal exception detection for a period, so that preflight findings become assigned work rather than transient messages.
2. As a Finance Analyst, I want each exception to show rule code/version and fingerprint, so that duplicate findings can be recognised.
3. As a Finance Analyst, I want each exception to show the detection event and projection version, so that I can reproduce the evidence used by the control.
4. As an Exception Owner, I want to be assigned an exception, so that responsibility and due follow-up are explicit.
5. As a Finance Reviewer, I want to record a verified resolution, so that a data correction is distinguished from an unreviewed comment.
6. As a Finance Reviewer, I want to record an accounting conclusion, so that an intentional accounting treatment is visible separately from data remediation.
7. As a Finance Approver, I want to grant a period waiver or standing waiver, so that exceptions are not silently ignored when business policy permits controlled acceptance.
8. As a Finance Approver, I want to close only a resolved or waived exception, so that an open issue cannot be hidden by a status shortcut.
9. As a user, I want Exception State and Closing Disposition shown separately, so that “closed” does not erase the nature of the decision.
10. As a Finance Manager, I want owner, reviewer and approver separation enforced, so that the same user cannot create an unreviewed self-approval chain.
11. As a Finance Approver, I want unresolved Blocking Exceptions to prevent period lock, so that lock status remains a controlled conclusion.
12. As an Auditor, I want the exception actions in the existing audit log, so that ownership and disposition changes are independently traceable.
13. As a scoped user, I want the exception list to follow my contract scope, so that I cannot view or act on another store or region.
14. As a Legal Entity administrator, I want a legal-entity-wide detection run, so that an exception run is not mistaken for a partial close conclusion.
15. As a product owner, I want the first version to reuse deterministic rule facts, so that AI suggestions cannot change severity, gate effect or control state.

## Implementation Decisions

### Domain objects

- `close_control_rules`: versioned rule definition. V1 stores code, version, severity, gate effect, display text, remediation and effective dates.
- `close_detection_events`: append-oriented evidence record for a detection run. It stores rule code/version, period, legal entity, subject, fingerprint, projection version, evidence JSON and detected time.
- `close_exceptions`: governed case keyed by stable fingerprint. It stores the current `exception_state` and `closing_disposition` as separate fields, plus owner/reviewer/approver and action timestamps.
- `audit_logs`: remains the immutable action trail for assign/resolve/conclude/waive/close actions.

`projection_version` V1 is `close-readiness-v1`. It names the facts/evaluation projection used by the detector; it is not a formal financial statement snapshot. A future Close Snapshot can be added without changing the exception interface.

### Rule and fingerprint policy

The fingerprint is SHA-256 over:

```text
accounting_period | legal_entity_id | rule_code | subject_type | subject_id
```

The fingerprint intentionally excludes display text and detection timestamp. A re-run updates evidence for the new Detection Event but does not create a duplicate Exception. Automatic reopening is out of scope for V1 and must be a separately approved policy.

### State and disposition policy

Exception State is one of `open`, `investigating`, `resolved`, `waived`, `closed`.

Closing Disposition is one of `unresolved`, `verified_resolution`, `accounting_conclusion`, `period_waiver`, `standing_waiver`.

Allowed V1 actions:

| Action | Result | Required actor | Separation rule |
|---|---|---|---|
| `assign` | `open` → `investigating` | owner | owner required |
| `verify_resolution` | `investigating` → `resolved` | reviewer | reviewer ≠ owner |
| `accounting_conclusion` | `investigating` → `resolved` | reviewer | reviewer ≠ owner |
| `period_waiver` | `investigating` → `waived` | approver | approver ≠ owner/reviewer |
| `standing_waiver` | `investigating` → `waived` | approver | approver ≠ owner/reviewer |
| `close` | `resolved`/`waived` → `closed` | approver | approver ≠ owner/reviewer |

Every action requires a non-empty note. `closed` requires a non-unresolved disposition. A Blocking Exception is considered unresolved for lock gating unless its state is `closed` with a valid disposition.

### Read/write seams

- `closecontrol.Service.Detect` loads the same facts source used by Close Readiness, evaluates the rules, creates Detection Events and upserts Exceptions.
- `closecontrol.Service.List` returns scope-filtered current exceptions.
- `closecontrol.Service.ApplyAction` validates the lifecycle and delegates the atomic update to the repository.
- `MonthlyClosingRepository.LockPeriod` calls one database guard query that rejects locking when unresolved blocking exceptions exist for the target Legal Entity and period.

Handlers only validate transport input, actor identity and route permissions. React does not implement lifecycle rules.

### API contract

- `POST /api/v1/monthly-closing/periods/:period/exceptions/detect`
- `GET /api/v1/monthly-closing/periods/:period/exceptions?state=...`
- `POST /api/v1/close-exceptions/:id/actions`

The action body is `{ "action": "assign|verify_resolution|accounting_conclusion|period_waiver|standing_waiver|close", "owner_id": "...", "note": "..." }`. The server derives reviewer/approver from the authenticated actor; client-supplied reviewer or approver IDs are ignored.

Detection and state-changing actions require legal-entity-wide scope in V1. Read-only listing may be partial and must return scope metadata. The existing `monthly_closing` permission family is extended with `exception_detect`, `exception_manage` and `exception_read`.

## Testing Decisions

### Domain/service tests

- fingerprint is stable for the same subject and changes when period/rule/subject changes;
- one detection run creates one exception per fingerprint;
- assignment requires owner and moves to `investigating`;
- resolution/conclusion requires a different reviewer and a note;
- period/standing waiver requires a different approver;
- close rejects unresolved or un-dispositioned exceptions;
- list output preserves state/disposition separately;
- unresolved blocking exceptions are reported to the lock guard.

### Repository integration tests

Use PostgreSQL fixtures to verify uniqueness, legal-entity isolation, contract-scope filtering, detection persistence, action updates and lock rejection/acceptance.

### HTTP verification

Verify malformed periods, missing notes, invalid transitions, insufficient scope and lock-gate conflict responses. Verify action records are written to `audit_logs`.

## Out of Scope

- automatic exception reopening;
- automatic remediation or AI-generated control conclusions;
- configurable rule authoring UI;
- due-date escalations and notifications;
- full Close Snapshot / immutable projection storage;
- exception aging analytics beyond the current list;
- changing the accounting engine or the underlying lease source data.

## Further Notes

The lock gate is deliberately implemented as a server-side database guard, not a UI warning. The UI may explain the blocker, but a caller cannot bypass it by calling the lock endpoint directly. The gate does not claim that a period is clean; it only prevents an explicitly unresolved blocking case from being locked.
