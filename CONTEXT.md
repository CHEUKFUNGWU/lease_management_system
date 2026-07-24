# Lease Management System

This context defines the business language used to manage lease records, accounting workflows, and controlled access to them.

## Deep Modules

**AI Intake**:
The versioned Assist Mode process that converts source documents into typed drafts with explicit evidence, confidence, and a mandatory human-review gate. AI intake output is never formal ledger data and cannot bypass normal approval, measurement, or posting controls.

**AI Intake Producer**:
The module behind `ai-intake.v1` that turns raw document, Excel, and LLM adapter output into normalized contract, contract-batch, or payment-schedule drafts. It owns confidence sanitization, missing-field policy, evidence truth, fallback selection, and the mandatory Assist Mode review gate; HTTP routes only decode input, select adapters, and encode errors/results.

**AI Chat Runtime**:
The client-side module that owns chat sessions, persistence and server hydration, streamed run transitions, artifact projection, and review-action history. Views observe its state and invoke its commands; they do not interpret transport events themselves.

**AI Agent Runtime**:
The core-side module that owns run creation, execution dispatch, ordered events, terminal success/failure, continuations, artifacts, and review actions. HTTP adapters translate requests; the AI agent implementation plans and executes work without controlling persistence transitions.

**Contract Workspace**:
The client-side module that owns the contract-detail aggregate: contract, payment schedules, events, critical dates, documents, obligations, calculation results, workflow state, dialogs, commands, and targeted refresh policy. Views observe one snapshot and send user intent; HTTP calls and form-to-payload mapping remain behind the workspace seam.

**Report Projection**:
A deterministic view derived from one controlled report snapshot. Projection policy owns IFRS calculations, time and currency buckets, tag and portfolio aggregation, sensitivity scenarios, response shapes, and export records; HTTP handlers only decode requests, acquire the snapshot, and encode the result.

## Access Control

**User**:
A person who accesses the lease management system and may hold multiple roles concurrently.
_Avoid_: Account, operator

**Role**:
A named collection of permissions representing one responsibility in the lease management workflow. A user's effective permissions are the union of every role assigned to that user.
_Avoid_: User type, access level

**Role Assignment**:
The association granting a role to a user. Role assignments are authoritative when determining a user's roles.
_Avoid_: Primary role, role string

**Permission**:
Authorization to perform a named action on a category of lease management information.
_Avoid_: Capability, privilege

**Legal Entity Access**:
The maximum set of lease information a non-administrator may access, determined by the legal entity assigned to that user. Missing narrower scopes grant access to the full assigned legal entity.
_Avoid_: Tenant permission, default scope

**Data Scope**:
An optional store, region, or brand restriction that narrows a user's Legal Entity Access. A Data Scope never expands access beyond the assigned legal entity.
_Avoid_: Data permission, tenant

**Segregation of Duties**:
The requirement that final approval be performed by a different User from the creator or reviewer. Editing and review may be performed by the same User during the MVP stage.
_Avoid_: Role separation, four-eyes rule

**Administrative Override**:
An exceptional, reasoned, and audited bypass of Segregation of Duties by a System Admin.
_Avoid_: Admin exemption, superuser approval

## Month-End Close

**Close Period**:
An accounting period progressing through preparation, calculation, review, posting, ERP writeback, and period lock for one Legal Entity.
_Avoid_: Month, batch

**Close Batch**:
A partial execution unit within a Close Period that may cover selected contracts, regions, or brands but cannot independently establish Close Readiness or authorize period lock.
_Avoid_: Close Period, lock batch

**Close Population**:
The complete set of lease contracts and related subjects required to be evaluated for one Close Period under its applicable scope and policies.
_Avoid_: Batch contracts, selected contracts

**Evaluation Coverage**:
The explicit comparison of the Close Population with the subjects actually evaluated, including a reason for every subject that was Not Evaluated.
_Avoid_: Processed count, batch completion

**Close Exception**:
A specific lease-subledger problem discovered during a Close Period that requires investigation and a recorded disposition before the period can be completed.
_Avoid_: Error, alert, failure

**Exception Fingerprint**:
The stable identity of a Close Exception across repeated evaluations, based on its Legal Entity, Close Period, control rule, affected subject, Reconciliation Group, and Functional Currency, but never on changing amounts, severity, or projection versions.
_Avoid_: Exception ID, deduplication key

**Detection Event**:
The immutable result and evidence produced when one control rule evaluates one subject against one Projection Version, linked to the continuing Close Exception when the result fails.
_Avoid_: Exception occurrence, run log

**Close Control Rule**:
A versioned, deterministic control definition that exclusively produces authoritative pass, fail, or Not Evaluated results and governs the resulting exception severity and Resolution Mode.
_Avoid_: Agent check, validation prompt

**Core Control Rule**:
A Close Control Rule whose classification and minimum control strength are system-governed and cannot be disabled, downgraded, or reclassified through customer policy configuration.
_Avoid_: Default rule, mandatory setting

**Policy Control Rule**:
A Close Control Rule whose approved thresholds or severity may vary by customer policy within system and group governance limits.
_Avoid_: Optional rule, configurable check

**Close Control Profile**:
An effective-dated, approved set of Policy Control Rule settings selected for a Close Period, consisting of group defaults and permitted Legal Entity overrides.
_Avoid_: Rule configuration, settings

**Group Control Ceiling**:
The maximum permitted weakening of a Policy Control Rule beyond which no Legal Entity override may pass, even with local approval.
_Avoid_: Default threshold, hardcoded limit

**Legal Entity Control Override**:
An approved deviation from a group default for one Legal Entity that remains within the Group Control Ceiling.
_Avoid_: Local setting, entity exception

**Emergency Control Change**:
An expedited, approved, time-limited policy change that never bypasses Segregation of Duties and expires back to the applicable approved default.
_Avoid_: Admin override, temporary setting

**Frozen Control Profile**:
The exact Close Control Profile designated with the Frozen Projection and Close Snapshot for reproducible evaluation of a locked Close Period.
_Avoid_: Current profile, copied settings

**Control Conclusion**:
The authoritative outcome produced by a Close Control Rule, distinct from any explanation, suggestion, confidence score, or Agent Signal.
_Avoid_: AI conclusion, recommendation

**Agent Signal**:
A traceable, probabilistic indication from non-structured evidence that a human accounting judgment may be required; it is input to a Close Control Rule and is never itself a Control Conclusion.
_Avoid_: Finding, exception, AI decision

**Agent Suggestion**:
Non-authoritative assistance explaining evidence or proposing assignment, investigation, or remediation without changing a Control Conclusion or accounting record.
_Avoid_: Resolution, approval, instruction

**Blocking Exception**:
A Close Exception that prevents period lock until it is closed with a Verified Resolution, Accounting Conclusion, or Approved Waiver.
_Avoid_: Fatal error, hard warning

**Warning Exception**:
A Close Exception that does not prevent posting or period lock after an authorized User acknowledges how it will be carried or addressed.
_Avoid_: Soft error, informational alert

**Gate Effect**:
The controlled close action that a failed Close Control Rule prevents, independently of the exception's severity, such as formal calculation, journal approval, posting, or period lock.
_Avoid_: Severity, status restriction

**Diagnostic Result**:
A retained projection, journal preview, reconciliation result, or other output that failed a control and remains available for investigation but cannot advance into a formal accounting state.
_Avoid_: Failed data, draft posting

**Exception Evidence**:
The traceable contract, payment schedule, event, calculation row, journal entry, reconciliation detail, or external response supporting a Close Exception and its disposition.
_Avoid_: Attachment, context

**Exception Assignment**:
The accountable User designated to investigate and advance a Close Exception to a terminal disposition.
_Avoid_: Notification recipient, watcher

**Exception Owner**:
The User accountable for investigating a Close Exception, correcting source data through authorized workflows, and submitting evidence or a proposed disposition.
_Avoid_: Assignee, approver

**Exception Reviewer**:
The User accountable for independently evaluating the Exception Owner's evidence and recommending a high-risk closing disposition.
_Avoid_: Watcher, second owner

**Exception Approver**:
The User with authority to approve a high-risk closing disposition and who must never be the Exception Owner for that exception.
_Avoid_: System Admin, reviewer

**High-Risk Disposition**:
An Accounting Conclusion, Period Waiver, or Standing Waiver that can close an exception without an automatically verified correction and therefore requires stronger Segregation of Duties.
_Avoid_: Manual close, special approval

**Group Finance Approval**:
Approval by an authorized finance role above the Legal Entity, required when local staffing cannot satisfy a control or when a Standing Waiver demands elevated authority.
_Avoid_: Admin approval, escalation bypass

**Verified Resolution**:
A closing disposition meaning the underlying deterministic problem was corrected and an Explicitly Evaluated passing event confirmed that it no longer exists.
_Avoid_: Fixed, dismissed

**Accounting Conclusion**:
A closing disposition documenting an approved human judgment on a judgmental Close Exception, including its evidence, rationale, and any required accounting action.
_Avoid_: Agent conclusion, reviewer comment

**Approved Waiver**:
A closing disposition accepting a Blocking Exception without correcting it, supported by a reason and explicit approval.
_Avoid_: Ignore, override

**Period Waiver**:
An Approved Waiver effective only for the Close Period in which it was granted and not inherited by a later period.
_Avoid_: Temporary waiver, carried waiver

**Standing Waiver**:
An exceptional Approved Waiver explicitly authorized for reuse under defined conditions in later Close Periods.
_Avoid_: Permanent exception, default waiver

**Resolution Mode**:
The fixed property of a control rule specifying whether a failed evaluation can be resolved by explicit successful revalidation or requires a human accounting conclusion.
_Avoid_: Auto-close setting, Agent decision

**Explicitly Evaluated**:
A Detection Event confirming that the applicable subject was in scope, the required evidence was available, and the control rule actually completed with a pass or fail result.
_Avoid_: Not detected, no exception

**Not Evaluated**:
The absence of a valid control conclusion because the subject was out of scope, evidence was unavailable, or the rule did not complete; it can never resolve a Close Exception.
_Avoid_: Passed, clean

**Close Readiness**:
The conclusion that a Close Period may proceed to its next controlled stage based on its current exceptions and required acknowledgements.
_Avoid_: Health score, Agent opinion

**Closed Exception**:
A Close Exception whose investigation has ended with exactly one recorded closing disposition: Verified Resolution, Accounting Conclusion, Period Waiver, or Standing Waiver.
_Avoid_: Resolved status, archived issue

**Reopened Exception**:
A previously Closed Exception returned to active investigation after its Exception Fingerprint fails again, retaining the complete prior disposition and evidence chain.
_Avoid_: New exception, duplicate issue

**Close Snapshot**:
The immutable record of the Close Period's exception population, evidence, acknowledgements, resolutions, and waivers at the moment of period lock.
_Avoid_: Export, report copy

**Post-Close Exception**:
A Close Exception discovered after period lock that cannot alter the frozen close and must lead to an approved prospective treatment or Period Reopen Request.
_Avoid_: Late exception, reopened issue

**Materiality Assessment**:
An approved human evaluation of whether a Post-Close Exception is significant enough to justify reopening a previously reported Close Period rather than correcting it prospectively.
_Avoid_: Threshold check, Agent materiality

**Period Reopen Request**:
A high-risk request to revise a locked Close Period, supported by new evidence, impact analysis, Materiality Assessment, proposed treatment, and the period's reporting and ERP status.
_Avoid_: Unlock action, admin reopen

**Close Revision**:
An immutable successor of a previously frozen Close Period that preserves the superseded close and records corrections through new reversal and adjustment activity.
_Avoid_: Overwritten close, rerun batch

**Prospective Adjustment**:
An approved correction recorded in a later open Close Period when reopening the original period is not justified.
_Avoid_: Deferred fix, carried error

**Prior-Period Error Assessment**:
An approved human judgment on whether a Post-Close Exception affecting issued financial information requires treatment as a prior-period error, including any restatement and disclosure implications.
_Avoid_: IAS 8 flag, Agent conclusion

**Forward Tracking Item**:
A mandatory tracked obligation in the target Close Period that ensures an approved Prospective Adjustment is executed and evidenced.
_Avoid_: Reminder, comment

## General Ledger Reconciliation

**Trial Balance**:
General-ledger evidence summarized by Legal Entity, Close Period, account, and available currency dimensions, including period movements and closing balance.
_Avoid_: GL report, balance export

**GL Line Item**:
An optional general-ledger posting line identified by voucher and line number that permits reconciliation evidence to be traced below Trial Balance totals.
_Avoid_: Journal Entry, transaction

**Functional Currency**:
The primary currency in which a Legal Entity measures and presents its accounting records and in which MVP reconciliation conclusions are made.
_Avoid_: Base currency, reporting currency

**Transaction Currency Coverage**:
The additional reconciliation coverage available when both subledger and general-ledger evidence retain the original transaction currency and amount.
_Avoid_: FX reconciliation, multi-currency mode

**Reconciliation Scope**:
The dimensions actually supported by the imported general-ledger evidence for a reconciliation, explicitly distinguishing Functional Currency coverage from Transaction Currency Coverage.
_Avoid_: Import format, matching level

**Lease Account Category**:
A canonical lease-subledger accounting category, such as right-of-use asset, accumulated depreciation, lease liability, interest expense, or depreciation expense.
_Avoid_: Entry type, account type

**Lease Account Mapping**:
A customer-specific association between one or more Lease Account Categories and general-ledger accounts used to compare lease-subledger expectations with general-ledger evidence.
_Avoid_: Chart of accounts, account guess

**Reconciliation Group**:
The smallest set of Lease Account Categories and general-ledger accounts that can be compared without claiming detail the supplied evidence cannot distinguish.
_Avoid_: Account bucket, category total

**GL Evidence Version**:
An immutable imported Trial Balance or GL Line Item dataset whose source identity and content are preserved for reproducible reconciliation.
_Avoid_: Upload, latest file

## Lease Subledger Projection

**Subledger Balance Projection**:
The authoritative lease-accounting state for a Legal Entity, contract, Lease Account Category, Close Period, and currency, including closing balances and their roll-forward movements.
_Avoid_: Measurement result, journal balance

**Projection Version**:
An immutable, reproducible result of generating a Subledger Balance Projection from a defined set of contract, payment, event, policy, rate, and calculation inputs.
_Avoid_: Recalculation, latest result

**Frozen Projection**:
The Projection Version designated as the final lease-subledger state for a locked Close Period.
_Avoid_: Approved batch, final calculation

**Roll-Forward Movement**:
A categorized change explaining how a lease-subledger balance moves from opening to closing, including additions, depreciation, interest accretion, principal repayment, remeasurement, modification, and derecognition.
_Avoid_: Journal Entry, adjustment

**Current Lease Liability**:
The discounted principal expected to be settled during the twelve months following the reporting date, derived from the current amortization schedule.
_Avoid_: Next-year payments, undiscounted maturity

**Non-Current Lease Liability**:
The portion of total lease liability remaining after deducting Current Lease Liability.
_Avoid_: Long-term payments, residual liability

**Maturity Analysis**:
The disclosure of undiscounted future lease payments by time band, distinct from the discounted current and non-current liability classification.
_Avoid_: Liability split, payment schedule
