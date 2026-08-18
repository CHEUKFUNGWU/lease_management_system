# Separate certified engine output from exploratory analysis, and fence the protected measures

Status: Accepted

## Context

The product is being extended from answering questions to producing working
papers — deliverables a finance user hands to an approver or an auditor. That
raises a question the current architecture has no answer for: **for any number
on the page, who computed it?**

Today every number an agent surfaces comes from a deterministic service
(`internal/services/predeal`, `dealcompare`, `renewaldecision`, `cashflow`,
`operating`) or from a database fact, so the question has a trivial answer. That
stops being true the moment the agent can run code, and it must be able to run
code: customer working papers contain bespoke measures that no registered tool
will ever cover, and `docs/FP&A与Finance_BP经营决策及AI辅助需求清单.md` treats
those as in scope.

Two failure modes bracket the design. Allowing free code execution would put
IFRS 16 figures on a path that the 22-case / 148-assertion measurement
regression does not protect — surrendering the one asset an auditor currently
accepts. Refusing code execution entirely leaves the long tail permanently
unserved, which is the thing FP&A and Finance BP users are actually asking for.

## Decision

### 1. Two execution tiers, distinguished at the cell level

| | Tier A — Certified | Tier B — Exploratory |
|---|---|---|
| Source | Registered deterministic tools, database facts | Sandboxed Python |
| Provenance | `tool_call_id`, `engine_version`, `input_hash` | `sandbox_run_id`, `code_hash`, `image_digest`, `input_snapshot_hash` |
| May enter an official draft | Yes, subject to the Review Gate | **No** |
| Standing in review | Admissible as a basis for approval | Analysis only, visually marked |

Provenance is recorded per cell, not per document. A single working paper
routinely mixes both tiers, and a document-level label would be a lie about half
of it.

### 2. A named set of protected measures may never originate in Tier B

```
lease_liability, rou_asset, discount_rate_applied, interest_expense,
rou_depreciation, amortization_schedule_row, journal_amount,
disclosure_maturity_bucket, weighted_average_discount_rate,
remeasurement_adjustment
```

The set is defined by **measure semantics, not by tool identity**, so that
registering a new tool cannot silently widen it.

The list is a governance artefact, not an implementation detail. This project
has a single human participant, so "sign-off" cannot mean a second person's
approval; it means a **dated decision-log entry recording the change and its
rationale**, committed alongside the change. The function of sign-off is
traceability of who decided what and why, and a solo operator satisfies that
completely. What is not acceptable is widening the set as a side effect of
implementing something else.

### 3. The fence is enforced twice, in code

**Request time.** A request whose target measures intersect the protected set
routes to Tier A if a certified tool can satisfy it, and is **refused** if none
can. Falling back to Tier B is prohibited — not discouraged. Refusal must state
which inputs would make the calculation possible; an unhelpful refusal is a
defect.

This gate is the `ProtectedMeasure` middleware in the ADR-0022 chain, placed
before the budget guard so that a request rejected on principle does not consume
the user's quota.

**Artifact time.** Before a working paper serialises, a lint pass rejects the
whole document if any cell whose `measure_id` is protected carries
`basis=Exploratory`, and additionally if a cell's *label* matches a
Chinese/English lexical probe for a protected measure while carrying
`basis=Exploratory`. The lexical probe exists to catch a missing `measure_id`,
not to replace it.

Both gates fail closed. A working paper that cannot be proven clean is not
emitted in degraded form; it is not emitted.

### 4. Tier B is a discovery mechanism with an exit

When the same bespoke measure or mapping recurs across runs (initially: three),
the system raises a hardening candidate. Hardening means a documented measure
definition, a deterministic Go service, unit tests, at least three regression
assertions against real customer data, descriptor registration, and allowlist
update — after which the measure is served from Tier A.

**The trigger must be mechanical, not a calendar commitment.** On the third
recurrence the system files an issue by itself, carrying the sandbox code, the
input fingerprints and the run references. A one-person team will not reliably
run a quarterly review; it will reliably work through an issue queue. Any
control here that depends on someone remembering is a control that does not
exist.

This is what stops Tier B accumulating as debt: the tail that customers actually
use is converted into moat on a schedule, and the tail that no one repeats stays
where it is.

### 5. Sandbox construction is deferred, its constraints are not

No customer exists yet, therefore no bespoke measures exist yet, therefore the
sandbox is scheduled last. What is *not* deferred is the data model:
`measure_id`, the four-valued `basis`, the per-cell provenance record and the
artifact lint are built from the start, because retrofitting them into a
serialised artifact format is a breaking change.

When the sandbox is built, its isolation tier is set by the first lighthouse
customer's compliance requirements, behind a `SandboxProvider` interface. Its
non-negotiable properties — egress deny-all, zero credentials, ephemeral
environments, resource quotas, reproducibility, filesystem isolation — are
recorded in `docs/AI_底稿与Paperwork_Agent设计方案.md` §5.3 and are not subject
to per-customer negotiation.

## Consequences

**"Who computed this" becomes answerable for every number**, which is the
precondition for a working paper being usable in review at all.

**The measurement regression keeps its meaning.** No path exists by which an
IFRS 16 figure reaches a deliverable without passing the certified engines.

**Some legitimate requests get refused.** A user asking for a protected measure
the engines cannot yet compute is told no, and told what is missing. This is the
intended behaviour and the intended cost.

**Two gates means two chances to be wrong in the same direction.** The lexical
probe will need periodic extension as models phrase labels in new ways; §12 of
the working-paper document records this as accepted residual risk.

**Governance acquires a recurring obligation, and it must be automated.** Tier B
becomes permanent technical debt unless hardening candidates actually get
worked. With a single human participant, the enforcement mechanism is the
auto-filed issue in §4 plus a visible count of open candidates — not a
scheduled review. The same reasoning applies to every control in this ADR: the
two fences in §3 are valuable precisely because they are code that fails closed,
rather than something a reviewer is expected to notice.

## Non-goals

- Not a general-purpose code execution platform. The sandbox serves working
  paper generation only.
- Not permission for AI to create official accounting records. Everything
  produced is a draft behind the Review Gate; ADR-0019 is unchanged.
- Not a claim that Tier B output is untrustworthy. It is unattested, which is a
  different and narrower statement.
