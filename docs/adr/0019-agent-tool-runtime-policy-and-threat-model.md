# Establish a controlled Agent Tool Runtime and threat model

Status: Accepted

## Context

The Web AI Chat already has an Agent execution path, but future Web, CLI and
Pi-like Runner clients must not gain separate access rules or direct database,
HTTP or Shell capabilities. A model-generated `contract_id` is untrusted input;
it must never be enough to load payment schedules, measurements, journals,
events, documents or reports.

The first implementation slice therefore needs one server-owned seam with a
small interface, explicit Tool levels, authenticated execution context and a
scope check before contract-linked reads.

## Threat model and control matrix

| Threat | Entry point | Required control | Verification |
|---|---|---|---|
| Cross legal-entity `contract_id` | Web AI Chat, future CLI, follow-up | Resolve contract attributes under the request scope before linked reads; return not-found semantics | `agenttools` scope tests and repository access-scope integration tests |
| Same legal entity, different store/region/brand | Page context, Session, Agent arguments | Apply `access.Scope` to contract, payment, measurement, journal and event queries | scope predicate tests and Postgres access tests |
| Forged identity or role fields | ToolCall JSON | ToolCall contains no identity fields; identity comes from `ExecutionContext` | protocol and runtime tests |
| Prompt Injection in uploaded document | OCR/LLM evidence | Treat document text as evidence only; server validates Tool arguments and policy | AI intake contract tests and Tool policy tests |
| Arbitrary SQL/HTTP/Shell | Pi-like Runner or CLI | No generic executor is registered; only server-side ToolDefinitions can execute | Registry registration and forbidden-action tests |
| High-risk accounting transition | Model Tool proposal | Command level is disabled by default and requires capability plus review | runtime command rejection/review tests |
| Replay or duplicate draft write | Retry, CLI reconnect, Agent loop | Draft/write Tools must declare idempotency support and receive an idempotency key | descriptor/call validation tests |
| Scope loss in asynchronous execution | Web request cancellation | Detached Agent Run preserves authenticated context values while applying a runtime timeout | AI Chat runtime scope propagation test |

## Decision

Create `core-service/internal/agenttools` as a deep module. Its external
interface is:

```go
type ToolRuntime interface {
    Describe(context.Context, ToolFilter) ([]ToolDescriptor, error)
    Execute(context.Context, ToolCall) (ToolResult, error)
}
```

The module owns:

- versioned server-side registration and exact Tool resolution;
- descriptor and argument validation;
- permission, capability and Tool-level decisions;
- authenticated execution context and access scope propagation;
- review gating for draft/command Tools;
- timeout, cancellation and stable structured error categories.

The Core Service remains the business authority. Repository adapters are
internal to Tool handlers and are never exposed to Agent clients. The default
policy permits Level 0 read Tools and reviewable Level 1 draft Tools; Level 2
commands require explicit policy, capability and human review.

## Consequences

Positive:

- Web, CLI and Pi-like Runner can share one execution contract.
- Scope and permission fixes have one seam and one test surface.
- Tool names cannot dynamically map to arbitrary Go functions or URLs.
- Existing AI Chat behavior can migrate incrementally.

Trade-offs:

- Every new business capability needs a descriptor, handler adapter, policy
  declaration and contract test.
- Existing direct Agent repository reads remain during migration and must be
  replaced by registered Tools in later tasks.
- Capability tokens and an external Gateway are deliberately deferred until
  the in-process runtime proves the model.

## Non-goals

- This ADR does not authorize automatic approval, posting, period locking,
  unlocking or ERP writeback.
- This ADR does not expose PostgreSQL, MinIO-admin, arbitrary HTTP or Shell.
- This ADR does not replace the IFRS 16 calculation engine with an LLM.


---

## Addendum A — Centralise policy into an ordered middleware chain (2026-08-18)

Status: Accepted. Extends the Decision above; does not revise it.

### Why

The Decision lists what `agenttools` owns, but the implementation scattered
those responsibilities: permissions and review live on `ToolDescriptor`,
idempotency partly on the descriptor and partly inside handlers, tenant scope in
`agenttools/scope.go` and again in each handler, audit in `agenttools/audit.go`,
budget in the runner's `Limits`, and the skill allowlist in
`agentskill/registry.go`. Each is individually correct. Together they make it
impossible to read one place and know which gates a tool call passes, and easy
to omit one when registering a new tool.

ADR-0022 introduces `BeforeToolCall` / `AfterToolCall` as the only policy
attachment points. This addendum fixes what attaches, and in what order.

### Decision

```
before := ChainBefore(
    TenantScope,        // 1
    CapabilityCheck,    // 2
    ProtectedMeasure,   // 3
    BudgetGuard,        // 4
    IdempotencyGuard,   // 5
    ReviewGate,         // 6
)

after := ChainAfter(
    AuditRecorder,      // not skippable
    ArtifactCollector,
    MetricsRecorder,
)
```

The order is load-bearing:

| # | Placement rationale |
|---|---|
| 1 | Cheapest and most severe. An out-of-scope request must not consume any further resource. |
| 2 | Identity precedes policy; every later decision is meaningless under an invalid token. |
| 3 | **Before** the budget guard, so a request refused on principle (ADR-0025) does not consume the user's quota. |
| 4 | Before any real cost is incurred. |
| 5 | **Before** the Review Gate, so replaying an already-confirmed call returns the stored result rather than demanding a second confirmation. |
| 6 | Last. Short-circuits to `needs_review` and returns control to a human. |

`ChainBefore` short-circuits on the first block; `ChainAfter` runs every hook and
aggregates errors. The asymmetry is intentional — blocking should be as early as
possible, while audit-side hooks must always run even if an earlier one fails.

`ProtectedMeasure` is the request-time half of the ADR-0025 fence.
`ArtifactCollector` records the `tool_call_id` / `engine_version` / `input_hash`
that per-cell Certified provenance requires, at the moment of the call rather
than by later reconstruction.

### Enforcement

Coverage is verified by mutation, not by inspection: removing any single
middleware from the chain must turn at least one test red. A middleware whose
removal leaves the suite green is not protecting anything and either needs a
test or should not exist. This is acceptance item ACORE-2 in
`docs/Agent_Core_Go设计_对齐pi架构.md` §11.

### Unchanged

The threat model, the `ToolRuntime` interface, the level model, and every
Non-goal above remain in force. This addendum relocates enforcement; it does not
relax it.
