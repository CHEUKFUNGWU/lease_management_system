# Domain vocabulary

- **AI intake**: The versioned Assist Mode process that converts source documents into typed drafts with explicit evidence, confidence, and a mandatory human-review gate. AI intake output is never formal ledger data and cannot bypass normal approval, measurement, or posting controls.
- **AI chat runtime**: The client-side module that owns chat sessions, persistence and server hydration, streamed run transitions, artifact projection, and review-action history. Views observe its state and invoke its commands; they do not interpret transport events themselves.
- **AI agent runtime**: The core-side module that owns run creation, execution dispatch, ordered events, terminal success/failure, continuations, artifacts, and review actions. HTTP adapters translate requests; the AI agent implementation plans and executes work without controlling persistence transitions.
