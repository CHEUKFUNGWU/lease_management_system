# Put contract-detail behavior behind one workspace interface

The contract-detail route renders one observable contract workspace and sends user intent through `dispatch` and `execute`. Loading the contract aggregate, mapping form values to command payloads, applying workflow rules, choosing targeted refreshes, tracking command state, and reporting failures belong to the contract workspace module under `web/app/contracts/[id]/workspace/`.

The workspace exposes `getSnapshot`, `subscribe`, `load`, `dispatch`, and `execute`. Its HTTP transport is an adapter behind that seam; tests use an in-memory transport and assert behavior through the same public interface. A failed aggregate loader does not discard independently loaded collections, and successful mutations refresh only the affected collection.

The Next.js route remains client-rendered because it depends on browser authentication and interactive Ant Design forms. The route and its rendering modules may own form instances, layout, translation, role-based button visibility, and navigation intent. They must not call contract-detail HTTP adapters directly, own duplicate workflow state, guess accounting fields, or decide mutation refresh ordering.

AI-assisted payment intake is entered through `/ai-chat`. The removed contract-page draft panel had no producer and must not be reintroduced as parallel state; future intake UI consumes the versioned `ai-intake.v1` review flow before creating payment-schedule drafts.
