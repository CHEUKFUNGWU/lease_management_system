# Adopt picoclaw's agent core and port the governance chain onto its hook points

Status: Accepted

Supersedes ADR-0022 §1 (Decision) only. ADR-0022 §2, §3 and §4 stand and are
reinforced by this decision. Amends ADR-0026 §2.

## Context

ADR-0022 decided that the agent core would be first-party Go in
`internal/agentcore`: "Not tau, not pi, not any third-party runtime." That
decision has been delivered — W1 and W2 shipped a pure loop with an import
guard, an ordered governance chain (six before-hooks, three after-hooks) and
ACORE-2 mutation tests locking nine behaviours. It is roughly 1,370 lines of
production code with 1,089 lines of test.

Three facts have emerged since that are not reflected in ADR-0022.

**First, ADR-0022 never evaluated picoclaw.** It evaluated three candidates:
tau (`huggingface/tau`, Python), pi (`earendil-works/pi`, TypeScript) and a
first-party Go core. Its recorded reason for rejecting pi is specific and
runtime-shaped:

> would introduce Node as a *backend* runtime […] a new backend service holding
> credentials, with its own network egress and patch surface.

**That reasoning does not transfer to picoclaw.** `sipeed/picoclaw` is Go 1.25 —
the same language and the same minor version as `core-service` — under MIT. The
objection ADR-0022 raised against pi is, for picoclaw, empty. Treating
ADR-0022's rejection of pi as a rejection of picoclaw is a category error: the
two projects were never compared.

**Second, the lineage is direct.** picoclaw is a Go rewrite of openclaw's
kernel, and openclaw is built on the pi base. ADR-0022 §2 chose to model the
architecture on pi-agent-core. picoclaw *is* that architecture, in Go, matured
through a second generation. Adopting it therefore upholds ADR-0022's
architectural decision rather than reversing it; only the sourcing clause in §1
changes.

The hook surface confirms the shared descent. ADR-0022 §2 requires that "tool
policy attaches at two gates, not six places". picoclaw exposes exactly that:

```go
type ToolInterceptor interface {
    BeforeTool(ctx context.Context, call *ToolCallHookRequest) (*ToolCallHookRequest, HookDecision, error)
    AfterTool(ctx context.Context, result *ToolResultHookResponse) (*ToolResultHookResponse, HookDecision, error)
}
type ToolApprover interface {
    ApproveTool(ctx context.Context, req *ToolApprovalRequest) (ApprovalDecision, error)
}
```

`BeforeTool` can deny a call (`HookActionDenyTool`); `AfterTool` can modify a
result (`HookActionModify`). Those are the two gates, plus a dedicated
authorisation seam. The existing governance chain can attach here.

**Third, picoclaw carries capabilities `agentcore` does not have.** A file-level
survey of `pkg/agent` (130+ files, 40+ test files) against `internal/agentcore`:

| | `internal/agentcore` | picoclaw `pkg/agent` |
|---|---|---|
| Pure loop, no direct I/O | yes, import-guarded | yes, via `adapters/` + `interfaces/` |
| Streaming events | yes (11 event types incl. `AssistantDelta`) | yes (`pipeline_streaming.go`) |
| Steering / abort | yes | yes (`steering.go`, `agent_stop.go`) |
| Governance middleware | **yes — six before / three after, ACORE-2 locks nine** | no equivalent; `tool_allowlist.go` only |
| Context budget & compaction | **none** | `context_manager.go`, `context_budget.go`, `context_usage.go` |
| MCP | none anywhere in the repository | `agent_mcp.go` |
| Subturn delegation | none | `subturn.go`, `turn_coord.go` |
| Memory / instance persistence | none (`State` is in-memory only) | `memory.go`, `instance.go` |

The absence of context compaction must be stated accurately. It is **not an
oversight**: ADR-0022's Non-goals record it as a deliberate deferral —

> No context compaction in the initial waves. It is deferred until a real
> context overflow is observed in working-paper sessions.

No such observation has been recorded. So compaction is a *deferred item whose
re-entry condition has not been demonstrated to be met*, not a live production
failure. What changes the calculation is cost, not urgency: under ADR-0022 this
item was future first-party work; adopting picoclaw supplies it, along with MCP
and subturn delegation, as part of the same move.

## Decision

### 1. The agent core is picoclaw's, vendored and adapted

`internal/agentcore` is replaced by an adaptation of picoclaw's `pkg/agent`.
Vendoring follows the mechanism ADR-0026 §1 already established for the channel
packages: upstream path, commit SHA and MIT notice per file;
`THIRD_PARTY_NOTICES` at the repository root; no business logic inside the
vendor directory.

This supersedes ADR-0022 §1 and nothing else in that ADR.

### 2. ADR-0022 §2, §3 and §4 are upheld

**§2 — the architecture.** The loop stays pure and the import guard stays: the
adapted core may not import `database/sql`, `net/http`, `internal/repository` or
the MinIO client. Upstream's `adapters/` + `interfaces/` split is compatible
with this; where it is not, the guard wins and the offending code is wrapped.
Tool policy continues to attach at two gates. Persistence remains a subscriber,
not a loop step.

**§3 — the trust model is still not adopted.** This is the load-bearing clause.
picoclaw is a *personal* assistant: no permission system, no tenancy, and
`tool_allowlist.go` is a do-not-disturb list, not authorisation — structurally
the same limitation as the `IsAllowed` filter ADR-0026 §4 addresses on the
channel side. Capability tokens, scope intersection, the Review Gate,
checkpointed resume and the controlled tool registry are retained. Upstream's
allow-list may run as a coarse first pass and establishes nothing.

**§4 — `internal/llm` stays.** picoclaw's `pkg/providers` is not adopted.
ADR-0022 §4 required China-region endpoints as first-class and that requirement
is met today; swapping the provider layer would put it at risk for no gain.

### 3. The governance chain is ported, not rewritten, and ACORE-2 is the gate

The six before-hooks and three after-hooks move to implementations of
`ToolInterceptor` / `LLMInterceptor` / `ToolApprover`. Their semantics do not
change.

**All nine ACORE-2 mutation cases must be re-proved against the new attachment
points — each one red before green, none dropped.** ADR-0022's own consequence
clause is the standard: "removing any middleware must turn some test red. A
middleware that can be removed silently was never covered."

A ported chain that passes because its tests were rewritten to match the new
implementation is a silent loss of compliance capability, and is exactly the
failure mode that let `fpna.assumptions.suggest` ship unregistered for months:
green tests over a path nothing exercised.

### 4. Context compaction is re-scoped, not assumed

ADR-0022's Non-goal deferred compaction pending an observed overflow. Adopting
upstream's `context_manager` / `context_budget` / `context_usage` makes the
capability available earlier than that condition required. It is enabled only
after its behaviour is pinned by tests on this repository's own session shapes —
in particular that compaction never drops audit-bearing content, and that a
compacted run remains replayable from checkpoint.

Availability is not the same as adoption. Where upstream compaction conflicts
with working-paper provenance, provenance wins.

## Consequences

**Roughly 1,370 lines of delivered production code are retired**, with their
1,089 lines of tests. This is the real cost and it should not be softened. The
offset is that the retired code is the part with the *fewest* product-specific
decisions in it — a loop, a queue, a state struct — while the part that encodes
this product's actual requirements, the governance chain, is preserved and
carried across.

**Two ADRs move from "decided" to "partly superseded".** ADR-0022's §1 is
replaced here; ADR-0026 §2 ("The agent loop is not vendored") is now wrong and
is amended by this decision. Both must be updated in
`docs/AI_文档索引与现行决策.md`, whose decision register currently records D1 as
"不引入 pi 本体".

**MCP becomes reachable.** The repository has no MCP support anywhere today.
This is a capability gain that was not on any roadmap and was not the reason for
this decision; it should not be treated as validated until something actually
uses it.

**Upstream drift now touches the critical path.** ADR-0026 accepted drift risk
for two channel packages. Extending vendoring to the agent core raises the
stakes: a pinned SHA and a no-in-place-edits rule are necessary but not
sufficient, and a divergence budget should be reviewed before the next wave.

**The migration must be stageable.** ADR-0022 required every wave to pass
`agent-evaluation.v1` and the skill contract replay before the next began. That
requirement carries over unchanged. If the ported chain cannot pass ACORE-2 at
full strength, the migration stops rather than proceeding with a weakened chain.

## Non-goals

- Not an adoption of picoclaw's `pkg/providers`, `pkg/identity`, `pkg/auth` or
  `pkg/credential`. ADR-0022 §4 and ADR-0026 §3 govern those.
- Not an open extension ecosystem. Tool registration stays controlled
  (ADR-0022 Non-goals, unchanged).
- Not a decision to enable context compaction by default. §4 above gates it.
- Not a claim that upstream's agent is better *for this product* on every axis.
  It is not: it has no governance chain, and that gap is why §3 exists.
