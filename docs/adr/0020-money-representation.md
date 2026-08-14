# Represent money as decimal, not float64

Status: Accepted

## Context

Monetary amounts are stored exactly and computed inexactly.

The database is already correct: 76 columns are `DECIMAL(18,2)`, five are
`DECIMAL(18,4)` and one is `DECIMAL(18,8)`. Nothing drifts in storage.

Go is not. Every amount is read into `float64`, there is no decimal library
anywhere in the service, `round2` is defined ten times and called 137 times,
and `services/ifrs16/calculation.go:222` carries a balance from one period to
the next (`liability = entry.ClosingLiability`) for 36 or more periods.

Industry practice on this is settled: money belongs in integer minor units or a
decimal type, never in binary floating point; rounding at every step does not
repair floating point but adds double-rounding bias, so intermediates should
carry extra precision and be rounded once at the boundary; and a reconciliation
that needs a tolerance band is signalling that its representation is wrong.

### What the measurement actually shows

Two facts had to be established before deciding, because both are easy to get
backwards.

**The regression tolerance is not a float64 artifact.** `tolerance` is a single
global `1` applied to all 148 assertions. Float error at ¥3,255,676 over
roughly forty operations is on the order of `1e-9` — nine orders of magnitude
below that band. Representation is not what put it there.

**The current numbers are already exact.** Parsing all 148 assertions out of
`docs/IFRS16_计量回归对数报告.md` gives a delta of `0.00` for every one of
them. The golden values are stated to the cent, and the implementation matches
them to the cent. The `1` is defensive padding, not a measured requirement.

So the present state is: **latent risk, not manifest error.** The
representation is unsafe in principle, and on today's 22 cases it is producing
exactly right answers. That distinction sets the sequencing below — it means we
can lock in the current exactness *before* touching anything, and then let that
lock catch any regression the migration introduces.

## Decision

### 1. Decimal type, not integer minor units

Prevailing advice leans to `int64` minor units. Three properties of this
codebase point the other way:

- The columns are already `DECIMAL`. A decimal type maps one-to-one with no
  schema change and no re-definition of column semantics; integer minor units
  would require redefining 82 columns.
- `DECIMAL(18,4)` and `DECIMAL(18,8)` carry rates and index factors. Sub-minor
  precision is a real requirement, and integer minor units cannot express it —
  we would end up with a second numeric type for rates and two money models.
- The system is multi-currency. "Cents" is not one concept across CNY, USD, JPY
  (0 decimals) and KWD (3 decimals), so a scale-carrying type is a better fit
  than a scale-implied integer.

### 2. `shopspring/decimal` as the engine, a local `money` package as the seam

`shopspring/decimal` provides arbitrary precision, `sql.Scanner` and
`driver.Valuer` for `DECIMAL` interop, and the widest adoption in Go. For an
audit-facing ledger, being the boring, battle-tested choice outweighs the
allocation cost of alternatives.

Currency semantics stay ours rather than being borrowed. A thin
`internal/money` package owns the scale table, the rounding policy and the
JSON contract, expressed in this project's vocabulary (`CONTEXT.md` →
Functional Currency), with `shopspring` as the numeric engine underneath.

### 3. Half-up rounding, symmetric about zero

Round half away from zero: `0.005 → 0.01`, `-0.005 → -0.01`.

Banker's rounding reduces bias across large aggregates, but two considerations
outweigh that here. Chinese accounting practice and most ERP systems the
customer already runs use half-up, and reconciliation against those systems is
the point. And symmetry about zero means a reversal is the exact mirror of the
original — `-1 ×` an entry gives back the same magnitude — which matters for
红冲, credit notes and contra entries.

### 4. Round once, at three named boundaries

Intermediate arithmetic carries full precision. Amounts are quantised only
when:

1. persisting to a `DECIMAL` column, at that column's scale;
2. emitting a journal entry line;
3. serialising an API response.

Nothing else rounds. In particular, the per-step `round2` pattern is removed
from migrated code rather than reimplemented on the new type.

### 5. Currency scale from a table, with rejection rather than silent rounding

Default scale 2, with explicit entries for the exceptions (JPY 0; KWD, BHD, OMR
3). An amount carrying more precision than its currency allows is an error, not
something to round away quietly.

Today the seeded data is CNY only, but multi-currency is already load-bearing
in the retail line: Peer Cohort excludes cross-currency peers and Currency
Partition forbids summing across currencies. The table exists so that adding a
currency is a data change rather than a code change.

### 6. The JSON wire format stays a number

Every money field on the frontend is typed `number`, and the formatters take
`value: number | null | undefined`. `shopspring/decimal` marshals to a JSON
*string* by default, so adopting it naively would silently break the web tier —
types would be wrong and `toLocaleString` would misbehave without an error.

`money` therefore implements `MarshalJSON` emitting an unquoted number, and the
byte-for-byte equality of API responses before and after migration is an
acceptance criterion, not an assumption.

### 7. Tighten the regression tolerance to zero first, and separately

Because all 148 deltas are already `0.00`, the tolerance can go to zero
immediately, before any code moves. This is not a consequence of the migration
and must not be reported as one.

Doing it first turns the regression suite into the migration's safety net: with
a zero tolerance, any deviation the migration introduces fails instantly
instead of hiding inside a ±1 band.

The suite's `review_status` remains `pending_third_party_review`. Exact
agreement with the golden values is not third-party endorsement, and tightening
the tolerance does not change that.

### 8. Migration order

1. Tolerance to zero — no code change, establishes the net.
2. The `money` package and a guard preventing new `float64` money fields.
3. `services/ifrs16` (123 occurrences) and `services/monthend` (13) — the
   audit-facing chain, and the only place a balance carries across periods.
4. Later batches: the FP&A, retail KPI and reporting packages, guarded against
   regression in the meantime.

51 money fields and 137 `round2` calls are spread across 22 packages. Migrating
them at once would be a single unreviewable change; the guard is what makes
staging safe.

## Consequences

**Gained.** Exact arithmetic on the audit-facing chain. Reconciliation can
compare for equality instead of within a band. A stated rounding mode with
defined behaviour on negatives. Amounts that cannot silently lose precision
crossing the Go/database boundary. A regression suite that fails on any
deviation rather than absorbing it.

**Paid.** A third-party dependency in the money path. Arithmetic is more verbose
than operators on `float64`. Migration touches 22 packages over several
batches, during which two representations coexist — the guard bounds that, but
does not remove it.

**Explicitly not gained.** This does not make the measurements audit-endorsed;
`pending_third_party_review` is unchanged. It does not fix the golden values,
which remain hand-produced. And it does not by itself improve any number that
is currently correct — the case rests on removing latent risk from a chain that
compounds over 36 periods and reconciles against a general ledger, not on
repairing a present error.

## Verification

- `DECIMAL(18,2)`, `(18,4)` and `(18,8)` round-trip digit-for-digit against a
  real PostgreSQL instance.
- Rounding asserted at `.5` boundaries for both signs.
- API responses byte-identical before and after migration.
- Allocation of a split sums exactly to the total, including amounts that do
  not divide evenly.
- The existing 148 assertions continue to pass at tolerance zero.
- A guard fails on any newly introduced `float64` money field.
