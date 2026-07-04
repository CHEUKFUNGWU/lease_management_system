# Use a versioned AI intake seam

The AI service and core service communicate through a versioned `ai-intake.v1` contract rather than implicit JSON maps. Each intake response contains a typed draft, source evidence, confidence scores, and an explicit review gate. Assist Mode always requires human review; missing field locators must be represented as incomplete evidence with a reason rather than fabricated coordinates or quotations.

Payment-schedule, single-contract, and contract-batch extraction use this seam. A consumer must reject unknown schema versions, unsafe modes, mismatched evidence identities, invalid confidence values, count mismatches, and responses that attempt to disable the Assist Mode review gate.

Evidence is complete only when the producing adapter explicitly marks it complete and supplies a locator covering every draft record. Deterministic Excel rows can satisfy this rule with sheet and cell-range locators. Document and LLM adapters that cannot produce truthful field locations must return incomplete evidence with a reason; the presence of an unrelated or partial locator never upgrades the bundle to complete.

Shared JSON fixtures under `contracts/ai-intake.v1/` are executable compatibility examples consumed by both Python producer tests and Go consumer tests. Additive fields may be introduced within v1 when old consumers can safely ignore them; breaking semantic or structural changes require a new schema version and parallel adapter support during migration.
