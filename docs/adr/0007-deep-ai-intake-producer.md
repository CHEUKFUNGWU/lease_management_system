# Put AI intake policy behind one producer interface

Contract, contract-batch, and payment-schedule routes select a source adapter, construct an `IntakeCommand`, and call `AIIntakeProducer.produce(command, source_adapter, llm_adapter)`. Routes own HTTP decoding and error mapping only. The producer owns pipeline selection, LLM fallback, normalization, confidence sanitization, missing-field detection, evidence truth, and construction of the mandatory Assist Mode review gate.

Source adapters return raw text plus only the deterministic records and locators they can truthfully establish. The stored-document adapter downloads and extracts document text; the stored-Excel adapter unfolds workbook cells and supplies row records with cell-range locators; the provided-text adapter preserves the existing direct-text path. The LLM adapter returns untrusted model output. These adapters do not decide whether evidence is complete or whether a draft may bypass review.

The producer is the test surface. In-memory source and LLM adapters exercise the same interface as production. Confidence values are clamped to the `ai-intake.v1` range, payment timing is never guessed, missing accounting fields generate review reasons, and all modes other than `assist` are rejected before adapters run.

Deterministic Excel fallback may mark evidence complete only when its locators cover every emitted record. LLM-derived document or workbook drafts remain incomplete evidence unless a future adapter produces locators tied to those exact records. Unrelated deterministic row locators must never be attached to LLM output.

The versioned `ai-intake.v1` response models and shared cross-language fixtures remain the external seam. Any breaking schema or review-policy change still requires a new schema version under ADR 0002.
