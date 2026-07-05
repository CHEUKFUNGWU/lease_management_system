# Put core AI agent orchestration behind one runtime interface

Core AI agent run policy belongs to `internal/aichat`. Its command-oriented interface starts synchronous or asynchronous runs, continues from run/message/artifact/action targets, records review actions, and inspects ordered events. The module owns queued/running/terminal transitions, messages, artifacts, persistence failures, history compression, parent linkage, and contract-context resolution.

`internal/aiagent` is the owned execution adapter. It plans skills, retrieves permission-scoped lease context, calls the AI provider and versioned AI intake seam, and returns a projected result. It emits intermediate tool events but cannot directly create runs, messages, artifacts, review actions, or terminal events.

HTTP handlers are transport adapters. They may perform request decoding, authentication-context translation, read-side listing, and SSE encoding; they must not coordinate write ordering or reproduce runtime policy. The persistence seam is private to the runtime implementation, with PostgreSQL in production and an in-memory adapter in runtime tests.
