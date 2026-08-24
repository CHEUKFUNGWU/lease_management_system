# Extend the picoclaw adoption from the agent loop to the whole runtime

Status: Accepted

Extends ADR-0027. That ADR's §1 through §6 stand unchanged; only its Non-goals are narrowed, because they scoped the adoption to `pkg/agent` alone.

Companion documents: [Spec C1](../specs/agent-runtime-overhaul-c1.md) (D-C0 to D-C10), [module design](../CodebaseDesign_AgentRuntime升级_模块深化.md) (AR1 to AR6, D-C11 to D-C20).

## Context

ADR-0027 adopted picoclaw's `pkg/agent` and listed `providers`, `identity`, `auth` and `credential` as non-goals. It said nothing about `bus`, `session`, `memory` or `routing`. On 2026-08-23 the owner set a wider goal: bring in everything an agent runtime should have, naming session management and context engineering, while taking what is useful from picoclaw and leaving what is redundant. ADR-0027's Non-goals now contradict that scope, so this ADR replaces them.

Three findings from surveying both codebases shaped the decision.

**`internal/agentcore` has never run in production.** `agentcore.New` appears in no non-test file. The production chat path runs `aichat.Runtime[T]`, and `aiagent/agent.go` imports agentcore only for its types and hooks. The 1,819-line pure loop and the governance chain that ACORE-2 locks with nine mutation tests protect nothing that serves a request today. This matches the open gap G1, "two agent planes have not converged", recorded since ADR-0022 and never closed.

**Context engineering does not exist.** No trimming, no windowing, no budget, and no token counting anywhere in the repository. `State.Messages()` returns the whole conversation. ADR-0022's Non-goals deferred compaction "until a real context overflow is observed", but nothing in the system can observe one: without a token count there is no measurable quantity, so the re-entry condition can never be met.

**Sessions have storage but no owner.** Seven tables exist. Lifecycle logic is spread across six files in `handlers`, `aichat` and `agentrunner`. "What state is this session in, can it resume, should it be evicted, what happens when two messages arrive at once" has no single place that answers it.

## Decision

### 1. Scope is measured in capabilities, not packages

picoclaw has 36 packages. Seven target embedded hardware (`audio`, `media`, `devices`, `netbind`, `pid`, `seahorse`, plus `isolation`, whose sandbox was deferred pending a real customer). Two are self-update (`updater`, `evolution`), which do not fit this product's release and audit model. Copying those adds maintenance surface and upstream drift without adding capability.

The inventory that governs scope is the 21-row capability table in Spec C1 D-C0. Every row is marked present, storage-without-owner, or absent, and every non-present row has a disposition. Absent and undisposed is the only unacceptable state.

Taking a package because upstream has it, and skipping a gap because "we have something similar", are the two failure modes this rule exists to prevent. The second is how `State.Messages()` came to count as context management.

### 2. Runtime isolation is stateless sharing, not per-account instances

The blueprint describes per-user runtime instances with `AgentManager.GetOrCreate()`, hot and cold state, and per-instance history and steering channels. This ADR rejects that shape.

The blueprint's four requirements all hold under stateless sharing, and one of them holds better. Per-instance tool filtering fixes a user's tool set when the instance is built; filtering per call through `Runtime.Describe` means a permission change takes effect on the next request instead of on the next instance rebuild.

Per-account instances would add lifecycle, eviction, memory ceilings and hot/cold consistency, four things to get right, while isolation would still depend on whether the lookup key is complete. An extra layer of instances does not strengthen isolation. It adds a place to leak.

This constrains the adaptation: `agentcore.Agent` holds five mutable fields (`state`, `steering`, `followUp`, `cancel`, `running`) and picoclaw has `instance.go`. Adapting means pushing that state into call parameters. Reusing upstream's instance model as-is produces a shared object holding per-run state, which is the one shape that leaks. An architecture test enforces this rather than review.

### 3. `ContextKey` is the isolation primitive, and it carries five dimensions

Context pollution and data access are separate axes. Scope plumbing governs which data a tool call can read. It says nothing about whose messages reach the prompt, because that happens before any tool runs.

Every cause of pollution reduces to an incomplete or wrong lookup key. Today the identifiers are bare strings, so omitting one, transposing two, or passing an empty one all compile and none report an error at runtime. They surface only as a model reading somebody else's context.

`ContextKey` makes those three errors unrepresentable: unexported fields, a single constructor, and that constructor takes a resolved `Principal` rather than three strings. Holding a key therefore proves the caller passed through permission resolution.

It carries `legal_entity_id`, `user_id`, `session_id`, a scope fingerprint and a data classification.

**The scope fingerprint** exists because `access.Scope` has seven dimensions and changes over time. After a store reassignment or a permission withdrawal, a cache keyed only on entity and user still holds stores the user may no longer see, and serving it triggers no permission check: the reader is the same person and the entity has not changed. Changing the scope changes the key, so stale context stops matching without any invalidation logic.

**The data classification** is 底线 2 landing in the context layer. That rule has enforcement points in the database, the API, exports and the interface, but never had one in context, because until now nothing carried context between sessions. Memory and cross-session summaries do exactly that. A conversation grounded in simulated data whose memory reaches a production conversation puts simulated figures in front of the model under a formal framing, and again no permission check fires.

Within-session history is fetched by `session_id`, which is stable, so a conversation that escalates to `mixed` does not orphan its own earlier turns. Cross-session carriers use the full key. Memory is written under the session's current classification, so a `mixed` session's memory is never readable by a purely `production` one.

### 4. Context engineering is counting, then budgeting, then compaction, in that order

Compaction without a budget is guesswork. A budget without a count has nothing to compare against. Building them out of order produces a system that trims by feel.

Token counting must use the tokenizer of the model in play. When that tokenizer is unavailable the count fails rather than falling back to a character estimate. Estimation error is largest at the budget boundary, which is the only place the number matters: too low and the request still overflows, too high and context is discarded for nothing. An approximation that fails precisely where it is consulted is worse than none, because it looks like it is working.

Establishing the count also settles ADR-0022's deferral clause. That clause is discharged here, not ignored.

Compaction never removes rows from `ai_chat_messages`. It shapes the sequence sent to the model, and the `Dropped` references it returns must resolve back to the stored rows, which makes "compaction did not delete anything" an executable assertion instead of a claim.

Audit-bearing content is protected structurally rather than by rule. A pure `classify` splits messages into preserved and compactable, and the compactor's signature accepts only the compactable half. It cannot discard tool calls, arguments, results, artifact references, approvals or `scope_denied` conclusions, because it never receives them. Rules can be edited wrongly by a later author. Types cannot.

### 5. Convergence is part of the kernel swap, not a follow-up

Replacing a kernel that no request reaches would swap one piece of dead code for another. The production chat path moves onto the new kernel in the same wave, and an assertion proves it does.

G1 stayed open for months precisely because no such assertion existed. Nobody noticed that `agentcore.New` had zero callers by reading the code. Machine checks find that class of gap. People do not.

ACORE-2's nine mutations must each fail before they pass on the new attachment points. Any one that cannot be re-established stops the migration rather than allowing a weakened chain through. Tests rewritten to match a new implementation do not count as re-established; the standard is whether removing the middleware turns a test red.

### 6. Subturn delegation narrows, never widens

Delegation is the one new capability that also adds attack surface, so its constraints are the tightest in this batch.

A child turn does not inherit the parent's clearance. Every tool call inside a child runs the full governance chain, because a parent that passed the Review Gate says nothing about what the child may do. A child's `Principal` is produced only by an unexported `narrow` that intersects the parent's scope with the request, so widening has no code path. Depth and concurrency are capped and configurable. Child tool calls carry an audit link to the parent.

A child's context is the task description the parent passed plus the child's own turns, not a copy of the parent's conversation. Copying it would both pollute across topics within one account and let the parent's history consume the child's budget.

### 7. What stays first-party, and why

`internal/llm` stays because it already treats China-region endpoints as first class and that works today. If upstream's provider layer proves stronger on that point, this is reopenable. The test is measured capability, not precedent.

`access` and `middleware` stay because cross-entity isolation is one of the five product floors. picoclaw is a personal assistant with no tenancy; `tool_allowlist.go` is a do-not-disturb list, the same limitation ADR-0027 §4 addresses on the channel side. Adopting it could only weaken what exists.

`aiagent` (6,603 lines) and `agentrunner` (2,455 lines) stay because upstream has no counterpart. The first is domain wiring for a product picoclaw knows nothing about. The second is checkpointing and lease recovery, which long-running work here actually needs.

## Consequences

**The upgrade is measured in capability, not in lines replaced.** Roughly 18,200 lines sit on the agent side. The two largest packages are the two that stay. What changes is the 1,819-line kernel plus nine capabilities that are currently absent.

**The vendored surface now includes the critical path.** ADR-0027 accepted drift risk for two channel packages. Extending vendoring to the kernel raises the stakes, and a pinned commit with a no-edits-in-place rule is necessary but not sufficient. A divergence budget should be reviewed before the wave after this one.

**One column changes.** `ai_chat_sessions.data_classification`, with the incremental migration and the empty-database version both supplied. Nothing else in the schema moves; compaction deletes no rows and therefore needs no new table.

**AR1 must land first.** It is the only module in this batch whose late arrival forces rework at every call site. Build the key as a type and each later module grows with the protection already attached. Build it last and every module first grows a bare-string signature that then has to be unwound.

**Two documents now disagree with reality by design.** The architecture blueprint still describes webhook transport, DOMPurify, per-user runtime instances and a 47-tool count. Those statements predate ADR-0026, ADR-0027 and this ADR. The owner chose on 2026-08-23 to leave the blueprint as written; readers should treat these ADRs as current where they conflict.

## Correction (2026-08-24, C1 batch review)

The phrase "kernel swap" overstates what AR5b/c/d actually shipped. The audited facts:

- First-party code references only picoclaw's **hook symbols** (`HookManager` / `ToolInterceptor` / `HookDecision` and siblings). The vendored `pipeline`, `providers`, `events`, `bus`, `routing` and `session` packages have zero first-party callers.
- `internal/agentcore` still exists (~1,370 lines), and `agentcorehooks.Governance` remains the guard for every plane that has not converged — so ACORE-2's nine controls now have **two implementations** in the tree.
- What is real and shipped: the governance chain runs on production chat traffic for the first time (the convergence assertion with its reverse probe proves it), and tool-call dispatch moved onto picoclaw's `HookManager`. That is "governance chain first production run + hook-based dispatch", not a kernel replacement.

Read this ADR's Decision §5 and the spec's 第一层 framing subject to that correction.

## Non-goals

- Not an adoption of `providers`, `identity`, `auth` or `credential`. §7 gives the reasons, and the first of them is reopenable on evidence.
- Not an adoption of the embedded-hardware or self-update packages.
- Not an open extension ecosystem. MCP arrives behind the governance chain, off by default and requiring registration. A tool that runs without being registered is a path around permissions, audit and the Review Gate.
- Not a decision to enable compaction unconditionally. It is gated on tests pinning its behaviour against this repository's own session shapes, and where it conflicts with working-paper provenance, provenance wins.
- Not a schema redesign. One column changes.
