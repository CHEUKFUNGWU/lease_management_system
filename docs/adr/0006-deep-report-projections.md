# Derive reports through one projection interface after the controlled snapshot

Report handlers decode HTTP input, acquire one access-scoped snapshot, call `reporting.Project`, and encode its JSON or CSV result. IFRS calculations, sensitivity scenarios, standard comparisons, time and currency buckets, tag and portfolio aggregation, response shapes, and CSV record construction belong to the report projection module under `core-service/internal/services/reporting/`.

The projection interface is `Project(snapshot, request)`. Its request selects the projection kind and supplies only decoded options; its result contains the complete response payload or CSV document. Tests use in-memory snapshots through this same seam. Handlers must not call the IFRS engine, aggregate source facts, convert currencies, split tags, or independently shape report rows.

The snapshot remains the source authority. Contracts, controlled payment schedules, resolved discount rates, and event adjustments are loaded before projection. Event adjustments are batch-loaded with the snapshot so amortization does not perform per-contract reads or silently combine facts from different points in time.

Liability JSON and CSV use the same controlled population and projection policy. Sensitivity and standards also resolve contracts, payments, and discount-rate precedence from the snapshot rather than bypassing it with repository reads. Projection responses include snapshot trace metadata; existing report fields remain compatible.
