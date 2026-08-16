# Do not MIT the platform; stage a three-tier licence with a deferred trigger

Status: Accepted

## Context

The question raised was whether to release this repository under MIT. It is an
architecture decision and not only a commercial one, because the answer
determines whether the IFRS 16 measurement engine has to be extractable as a
standalone module with no inward dependency on the platform.

Three facts about the product constrain the answer.

**The buyer is not a developer.** Per
`docs/GTM_零售经营工作站_中国大陆与香港市场进入策略.md`, the economic buyer is a
CFO / COO / 财务总监 and the daily users are BP, 区域经理 and 租赁会计. The growth
mechanism open source actually provides — a developer self-hosts, uses it,
spreads it upward inside the company — has no working link in that chain. The
same GTM document already establishes on independent grounds that this product
cannot run product-led growth: the data is too sensitive to upload without a
signed DPA, and the product is an empty shell until integration is done. Open
source is a form of PLG. If PLG does not hold, open source as a distribution
strategy does not hold either.

**The moat is domain knowledge, and the code is its encoding.** The
differentiation identified in the feasibility report is not query speed or chart
count; it is the three parallel measurement bases (财务报告口径 / 经营占用口径 /
决策经济口径), the treatment of turnover rent, remeasurement triggers, and the
governed write path. These are hard to *know* and easy to *copy once written*.
MIT grants unrestricted commercial redistribution with no reciprocity, which
hands the encoded knowledge to any systems integrator who wants to fork it and
sell private deployments — a likely outcome in the mainland market, not a
theoretical one, given that §4.2 of the GTM document already records
私有化部署 as a near-mandatory delivery mode there.

**In mainland procurement, open source suppresses price.** "开源 = 免费" is close
to the default reading. With no logos, no brand premium and a 6–12 month sales
cycle already assumed, an open repository supplies the procurement counterparty
with its opening argument. What this product needs at this stage is pricing
power, not reach.

### The argument on the other side

One counter-argument is real and should not be waved away. For a financial
calculation engine, **auditability is a purchase criterion**. Controllers and
audit firms do ask how discounting and remeasurement are computed and how they
can verify it. A black-box measurement engine is a deduction in this category,
not a neutral. And with zero customers and zero brand, an open component is one
of the few ways to manufacture credibility from nothing.

That argument supports opening *a narrow, verifiable component*. It does not
support opening the platform.

## Decision

### 1. Not MIT, and not for the platform

The platform — multi-tenancy, data scope, Working/Official versioning, approval
and audit, the Agent Runtime with its Review Gate and Capability Token — is not
released under a permissive licence.

### 2. Three tiers

| Tier | Contents | Licence | Purpose |
|---|---|---|---|
| Open | Standalone single-contract IFRS 16 / CAS 21 measurement library: discounting, amortisation schedule, remeasurement, modification, sale-and-leaseback | **Apache-2.0** | Professional credibility; lets an auditor re-run the numbers; pairs with the whitepaper content strategy |
| Source-available | Platform proper | **BSL / Fair Source**, converting to Apache-2.0 after N years | Customer can self-audit, evaluate and self-host; a competitor cannot resell |
| Commercial | Retail operating modules, Agent skills, connectors, implementation and support | Closed | Revenue |

### 3. Apache-2.0 rather than MIT for the open tier

Apache-2.0 carries an express patent grant and a clearer contributor rights
definition. Enterprise and Hong Kong legal review passes it more readily than
MIT. The cost of choosing it over MIT is approximately zero and the benefit is
concrete.

### 4. The open component is the measurement library, not the platform

The library is the right object because it is simultaneously the thing that
answers the auditability objection, the showcase for standards knowledge, and
**not the moat**. The defensible assets are multi-tenant governance, the linkage
between lease facts and retail operating facts, and the action loop — none of
which a single-contract calculator reproduces.

This imposes one standing engineering constraint: **the measurement engine must
stay extractable.** It may not acquire an inward dependency on tenancy, RBAC,
persistence or the Agent Runtime.

That constraint is currently satisfied, and by a wider margin than expected.
`internal/services/ifrs16` imports exactly one internal package,
`internal/money`, and `internal/money` imports none. The pair is already a
self-contained numeric core with no reach into the platform — so the open tier
is executable today as a mechanical extraction, and the cost in §5 is
documentation, packaging and maintenance rather than decoupling work. The
constraint's job is therefore to *preserve* a property that already holds, not
to create one.

### 5. Execute later, on a trigger, not now

Nothing is published yet. The reasoning is sequencing, not reversal:

- The bottleneck is customer access, not distribution. One 公众号 article on
  turnover rent treatment reaches more CFOs than a GitHub repository will.
- Extraction itself is cheap (see §4), but a *publishable* library is not: it
  needs standards-referenced documentation, a worked example set, an English
  README, and a professional review of the accounting claims before anything
  carrying standards assertions goes public. That work competes directly with
  interviews and design-partner recruitment, which are the binding constraint.
- Open source is a standing cost: issues, PRs, compatibility, security
  response. A zero-customer company cannot carry it.
- **The direction is asymmetric.** Closing an open project damages credibility;
  opening later costs nothing. Where an option is one-way, take it late.

**Trigger to revisit:** the first design partner is signed *and* the measurement
engine has survived one real audit review. At that point the library is "a
measurement library validated in an audit", which travels; today it would be
"part of a project nobody has heard of", which does not.

### 6. Prefer contractual source access over public source

Mainland enterprise customers commonly require private deployment plus source
escrow or a scoped source licence. That is a different instrument from public
open source: bounded scope, under contract, with consideration. Making
"auditable and escrowable source" a commercial term converts the same
transparency into revenue rather than away from it.

## Consequences

**Gained.** The encoded standards knowledge is not transferable to a competitor
for free. Pricing power in the mainland market is not undercut before the first
deal. The auditability objection has a designated answer. The one-way door stays
closed until there is evidence to open it.

**Paid.** No community contribution, no inbound developer awareness, and no
open-source credibility substitute during exactly the period when the company
has no logos — which is the period it would help most. Keeping the measurement
engine extractable is an ongoing constraint on where code may be placed, and it
will occasionally be the less convenient option. BSL is less familiar to
mainland legal reviewers than MIT or Apache and will cost some explanation in
procurement.

**Explicitly not gained.** This decision does not make the measurements
audit-endorsed; per ADR 0020 the regression suite remains
`pending_third_party_review`, and the audit review named in the trigger above
has not happened. It does not establish that a scoped source licence is
acceptable to mainland customers — that is an untested assumption inherited from
the GTM document, whose stated confidence is low. And it does not select the
BSL change date (N), the additional-use grant, or the library's package
boundary; those are deferred to the point of execution.

## Verification

- `internal/services/ifrs16` imports no internal package other than
  `internal/money`, and `internal/money` imports no internal package at all.
  Verified 2026-08-16; this is the property the open tier depends on.
- A guard fails on any new inward dependency from `internal/services/ifrs16` or
  `internal/money` to platform packages. Not yet implemented — until it is, the
  property above is held by habit rather than enforcement.
- The extracted library passes the 148 regression assertions standalone, outside
  the core-service module.
- The trigger in §5 is checked at each Phase 0 milestone review rather than
  continuously; absence of a signed design partner is sufficient to defer
  without further discussion.
- Reversal signal: if ≥ 2 of the first 10 customer interviews independently cite
  a closed measurement engine as a blocking objection, the open tier moves ahead
  of the trigger.
