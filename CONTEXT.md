# Domain vocabulary

- **AI intake**: The versioned Assist Mode process that converts source documents into typed drafts with explicit evidence, confidence, and a mandatory human-review gate. AI intake output is never formal ledger data and cannot bypass normal approval, measurement, or posting controls.
- **AI intake producer**: The module behind `ai-intake.v1` that turns raw document, Excel, and LLM adapter output into normalized contract, contract-batch, or payment-schedule drafts. It owns confidence sanitization, missing-field policy, evidence truth, fallback selection, and the mandatory Assist Mode review gate; HTTP routes only decode input, select adapters, and encode errors/results.
- **AI chat runtime**: The client-side module that owns chat sessions, persistence and server hydration, streamed run transitions, artifact projection, and review-action history. Views observe its state and invoke its commands; they do not interpret transport events themselves.
