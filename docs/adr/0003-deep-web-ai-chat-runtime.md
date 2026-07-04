# Put web chat behavior behind one runtime interface

The AI chat page renders runtime state and invokes runtime commands. Session persistence and server hydration, SSE framing, run transitions, artifact projection, continuations, and review-action history belong to the AI chat runtime module under `web/app/ai-chat/runtime/`.

The runtime interface is exposed by `useAIChatRuntime`. Its implementation uses a browser-storage adapter and an HTTP/SSE transport adapter. Pure transition functions are the test surface for event and hydration behavior. Contract creation and payment-plan import remain page-level business actions; recording their review outcomes and continuing a run belong to the runtime.

Views must not parse SSE frames, merge server artifacts, or directly append review actions. New runtime event types should first be represented as a tested transition and then rendered from runtime state.
