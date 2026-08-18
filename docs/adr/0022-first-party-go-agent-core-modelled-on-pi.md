# Build a first-party Go agent core modelled on pi, not a third-party brain

Status: Accepted

Supersedes the tau decision in
`docs/AI_Agent_填表升级_tau_anydoc_实施计划.md` (D1, D2, D4).

## Context

The agent brain is currently split across two planes that never meet. The AI
Chat plane (`/api/v1/ai/chat`) decides what to do with a static runbook in
`internal/aiagent`; it executes six read tools and four file-parse tools and
nothing else. The Agent Gateway plane (`/api/v1/agent/runs`) plans over the full
descriptor set with an LLM, but is reachable only from `cmd/agent-runner` and
the CLI, and that worker sits behind a Compose profile that is off by default.
The consequence is visible in the product: `lease.predeal.simulate`,
`lease.cashflow.scenario` and `lease.store.scenario.simulate` render as
`Status: "pending"` cards that no user request ever executes.

Three replacement options were considered.

**tau** (`huggingface/tau`, Python 3.12) was the previously recorded choice. It
requires a new Python container, because `ai-service` is pinned at 3.11 and is
not being upgraded. That directly contradicts ADR-0023, which retires
first-party Python.

**pi** (`earendil-works/pi`, TypeScript, MIT) has the better runtime design but
would introduce Node as a *backend* runtime. Node already exists in the stack
for the Next.js frontend, so this is not a new language for the team, but it is
a new backend service holding credentials, with its own network egress and
patch surface. `pi-ai` is genuinely attractive as a provider abstraction — 30+
providers including the DeepSeek and China-region endpoints this product needs —
but adopting a TypeScript library for that alone does not justify the runtime.

**A first-party Go core** reuses `agenttools`, `agentcapability`, `agentskill`,
`agentartifact` and the checkpoint logic unchanged, and keeps the whole agent
path inside one runtime.

The decisive observation is that none of the three options closes any current
gap. Triage, working-paper output and plane convergence are unaffected by which
brain runs the loop. Brain replacement is therefore a structural investment, and
should be judged on structure alone — which argues for the option with the
lowest runtime cost and the highest reuse.

## Decision

### 1. The agent core is first-party Go, in `internal/agentcore`

Not tau, not pi, not any third-party runtime.

### 2. Its architecture is modelled on pi-agent-core

Three structural choices are adopted deliberately:

**The loop is pure.** `Loop()` and `LoopContinue()` take state and an injected
`Deps` struct and perform no I/O of their own. `internal/agentcore` may not
import `database/sql`, `net/http`, `internal/repository` or the MinIO client.
This is enforced by an import guard in CI, in the same manner as the existing
`internal/services/ifrs16` guard required by ADR-0021.

**Tool policy attaches at two gates, not six places.** `BeforeToolCall` and
`AfterToolCall` become the only policy attachment points. See ADR-0019
Addendum A for the ordered chain and the reasoning behind the order.

**Persistence is a subscriber, not a step in the loop.** A run is not settled
until every subscriber has returned, which is what keeps audit from being lost
when the loop finishes first. This replaces the current arrangement in
`internal/aichat`, where Postgres writes are interleaved with loop control.

### 3. pi's trust model is not adopted

pi has no permission system, no persistence, and an open extension ecosystem.
All three are wrong for a multi-tenant financial system. Capability tokens,
scope intersection, the Review Gate, checkpointed resume and the controlled tool
registry are retained and, in the case of policy, strengthened by being
centralised.

### 4. Provider abstraction is reimplemented in Go, not imported

`internal/llm` takes the *shape* of pi-ai — a provider collection owning its own
auth, model catalogue and streaming behaviour — without taking the dependency.
The current `ai-service/app/services/llm.py` hardcodes two providers; the Go
replacement must support China-region endpoints as first-class.

## Consequences

**The two planes converge.** Multi-step work routes to the agent core in-process;
the `agent_plan.py` HTTP hop and the `AGENT_PLANNER_TOKEN` it requires both
disappear. Runbook cards stop being decorative and become execution records.

**Policy becomes auditable as a unit.** A reviewer can read one ordered chain and
know every gate a tool call passes. This is testable by mutation: removing any
middleware must turn some test red (ACORE-2). A middleware that can be removed
silently was never covered.

**`cmd/agent-runner` becomes a driver, not a second implementation.** Its
checkpoint and lease-recovery logic — which pi does not have and which this
product does need — is retained and moved behind the core.

**Migration is staged and reversible.** Waves W1–W3 change no external
behaviour; W4 onward do. Every wave must pass `agent-evaluation.v1` and the
skill contract replay before the next begins.

**The cost is real.** This is a rewrite of the orchestration layer, not a
library swap. It is justified only because the same work removes the second
plane, retires Python (ADR-0023), and provides the attachment point that
ADR-0025's protected-measure routing requires.

## Non-goals

- No open extension ecosystem. Tool registration stays controlled.
- No session tree or branching UI beyond the existing linear checkpoint model.
- No context compaction in the initial waves. It is deferred until a real
  context overflow is observed in working-paper sessions.
- No change to the Gateway wire contract or to `agenttools` descriptors.
