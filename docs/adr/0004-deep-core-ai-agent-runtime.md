# Put core AI agent orchestration behind one runtime interface

Core AI agent run policy belongs to `internal/aichat`. Its command-oriented interface starts synchronous or asynchronous runs, continues from run/message/artifact/action targets, records review actions, and inspects ordered events. The module owns queued/running/terminal transitions, messages, artifacts, persistence failures, history compression, parent linkage, and contract-context resolution.

`internal/aiagent` is the owned execution adapter. It plans skills, retrieves permission-scoped lease context, calls the AI provider and versioned AI intake seam, and returns a projected result. It emits intermediate tool events but cannot directly create runs, messages, artifacts, review actions, or terminal events.

HTTP handlers are transport adapters. They may perform request decoding, authentication-context translation, read-side listing, and SSE encoding; they must not coordinate write ordering or reproduce runtime policy. The persistence seam is private to the runtime implementation, with PostgreSQL in production and an in-memory adapter in runtime tests.

---

## Addendum A — Extract a pure loop and demote persistence to a subscriber (2026-08-18)

Status: Accepted. Extends the decision above; the module responsibilities it
assigns remain correct, but their packaging changes.

### Why

Two problems emerged in use.

First, `internal/aichat.Runtime` owns run policy *and* performs persistence
inline — `prepare`, `complete`, `fail`, `persistAssistantMessage` and
`appendEvent` interleave Postgres writes with loop control. The loop therefore
cannot be exercised without a database, and a persistence fault becomes a
business fault.

Second, run policy exists twice. `internal/aichat` + `internal/aiagent` decide
via a static runbook, while `internal/agentrunner` decides via an LLM planner
over the full descriptor set. The two never meet, which is why simulation tools
render as `Status: "pending"` cards that no chat request executes.

### Decision

`internal/agentcore` holds the loop as pure functions — `Loop` and
`LoopContinue` — taking state plus an injected `Deps`. The package may not
import `database/sql`, `net/http`, `internal/repository` or the MinIO client;
an import guard in CI enforces this, in the same manner as the ADR-0021 guard
on `internal/services/ifrs16`.

Persistence, SSE broadcast, checkpointing and usage recording become
subscribers registered on the agent. A run is not settled until every subscriber
has returned. Audit-class subscribers failing fails the run; delivery-class
subscribers failing raises an alert and does not.

`internal/aiagent`'s role as the owned execution adapter is unchanged, and it
still may not create runs, messages, artifacts, review actions or terminal
events directly. Its static runbook is replaced by planned execution, so its
tool cards become execution records rather than presentation.

`internal/agentrunner` converges into a driver over the same core, retaining its
checkpoint and lease-recovery logic, which the core does not otherwise provide.

### Unchanged

HTTP handlers remain transport adapters. The persistence seam remains private
to the runtime, with PostgreSQL in production and an in-memory adapter in tests.
The command-oriented interface — start, continue, record review action, inspect
ordered events — is preserved; only its implementation is repackaged.

See ADR-0022 and `docs/Agent_Core_Go设计_对齐pi架构.md` §4–§7 and §10 for the
package layout and the staged migration.
