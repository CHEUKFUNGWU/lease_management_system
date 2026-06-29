# Repository Cleanup Task List

This backlog is based on direct inspection of the current repository state on 2026-06-11. It focuses on incomplete abstractions, repetitive logic, duplicate types, and comments or files that no longer carry useful intent.

## 1. Extract shared frontend domain types from page components

- Create a typed domain layer under `web/app/lib/types/` for contracts, payment schedules, monthly entries, AI draft artifacts, users, and report rows.
- Replace page-local interfaces duplicated across:
  - `web/app/contracts/page.tsx:36`
  - `web/app/contracts/[id]/page.tsx:54`
  - `web/app/ai-chat/page.tsx:57`
  - `web/app/context/AuthContext.tsx:5`
  - `web/app/admin/users/page.tsx:29`
- Rationale:
  - The same contract/user/payment shapes are redefined with slight drift.
  - The drift already shows up as `Contract` vs `ContractDetail`, local `User`, and local AI artifact types.

## 2. Split oversized frontend pages into composable feature components

- Break down large pages into feature folders with local hooks and presentational subcomponents:
  - `web/app/ai-chat/page.tsx` (3202 LOC)
  - `web/app/contracts/[id]/page.tsx` (2575 LOC)
  - `web/app/reports/page.tsx` (1040 LOC)
  - `web/app/monthly-closing/page.tsx` (1047 LOC)
  - `web/app/page.tsx` (687 LOC)
- Minimum extraction targets:
  - AI chat session sidebar, message renderer, runtime review panels, draft confirmation panels.
  - Contract detail tabs, event actions, critical dates, documents, obligations, schedules, calculations.
  - Report filter panel, amortization table builder, KPI cards, export actions.
- Rationale:
  - Current pages mix data loading, state machines, business mapping, and UI rendering in one file, making them hard to test and reuse.

## 3. Consolidate repeated status, scope, and label dictionaries

- Move shared label/color maps into dedicated constants/helpers, then reuse them across pages.
- Current duplication examples:
  - `web/app/contracts/page.tsx:56`
  - `web/app/reports/page.tsx:30`
  - `web/app/components/AppLayout.tsx` menu/breadcrumb label maps
  - `web/app/contracts/page.tsx:66` lease scope labels/colors
- Rationale:
  - Status color and translation maps are redefined per page and are already inconsistent (`reviewed` is `processing` in one place and `warning` in another).

## 4. Replace `any`-heavy frontend API and view models with typed request/response contracts

- Introduce typed request/response wrappers in `web/app/lib/api.ts`.
- Remove broad `any` usage in high-traffic pages:
  - `web/app/lib/api.ts:86`, `115`, `122`, `129`, `186`, `199`, `252`, `269`, `279`, `296`, `360`, `384`, `410`, `447`, `561`
  - `web/app/reports/page.tsx:51`, `187`, `188`
  - `web/app/page.tsx:130`, `171`, `172`
  - `web/app/monthly-closing/page.tsx:48`, `102`, `103`, `104`
  - `web/app/contracts/[id]/page.tsx:179`, `183`, `203`
- Rationale:
  - TypeScript is present but a large part of the UI currently bypasses it.
  - This blocks safe refactoring and hides schema mismatches between the Go/Python services and the UI.

## 5. Refactor `web/app/lib/api.ts` into a reusable HTTP client plus feature modules

- Extract common request concerns:
  - base URL handling
  - auth header injection
  - JSON error normalization
  - query string building
- Split `web/app/lib/api.ts` into `client.ts` plus feature modules such as `contracts.ts`, `reports.ts`, `ai-chat.ts`, `lease-admin.ts`.
- Evidence:
  - duplicated request wrapper logic in `web/app/lib/api.ts:8-34` and `36-61`
  - repeated `body: JSON.stringify(data), token` blocks throughout `web/app/lib/api.ts:63-220`
- Rationale:
  - The file is already 619 LOC and combines transport details with all business endpoints.

## 6. Remove hardcoded master data from the new contract form

- Replace hardcoded legal entities, stores, and landlords with API-driven option loaders.
- Evidence:
  - fixed UUIDs in `web/app/contracts/new/page.tsx:153-170`
- Rationale:
  - This is a broken abstraction for a multi-tenant system.
  - It couples UI behavior to seed data and will drift from production master data.

## 7. Move contract normalization and construction logic out of handlers into shared backend services

- Create a backend service or mapper for:
  - tag normalization
  - asset type normalization
  - lease scope normalization
  - discount rate normalization
  - contract request -> repository model conversion
  - legal entity / store / landlord resolution
- Evidence:
  - duplicated request structs in `core-service/internal/handlers/contract.go:23-80`
  - repeated contract construction in `core-service/internal/handlers/contract.go:193-246` and `329-380`
  - update path rebuilds the same shape again in `core-service/internal/handlers/contract.go:519-558`
- Rationale:
  - Contract creation, batch creation, and update already repeat the same transformations with small behavioral differences.

## 8. Centralize discount-rate fallback policy in backend services

- Extract a single helper/service for discount rate resolution priority and reuse it across handlers.
- Current duplication:
  - `core-service/internal/handlers/calculation.go:93-100`
  - `core-service/internal/handlers/monthly_closing.go:127-141`
  - `core-service/internal/handlers/event.go:307-311` and `617-619`
  - `core-service/internal/handlers/reports.go:413-416`, `526-529`, `1588-1593`
- Rationale:
  - The fallback chain is domain logic, not handler glue.
  - Repeating it increases the chance of accounting drift between calculation, events, reporting, and closing.

## 9. Eliminate fake fallback business behavior in monthly closing

- Remove or explicitly gate `generateMockPayments`.
- Evidence:
  - fallback path in `core-service/internal/handlers/monthly_closing.go:119-125`
  - hardcoded payments in `core-service/internal/handlers/monthly_closing.go:282-300`
- Rationale:
  - Silent mock schedules are dangerous in an accounting system.
  - Missing schedules should produce a validation failure or a draft exception, not synthetic ledger inputs.

## 10. Finish or remove incomplete backend control points

- Complete the following stubs before keeping them on request paths:
  - `core-service/internal/middleware/rbac.go:83-104` only declares `LoadUserPermissions` and then skips all work.
  - `core-service/internal/handlers/discount_rate.go:58` still says `TODO: Update contract with confirmed discount rate`.
  - `ai-service/app/routers/files.py:84-92` returns a fake pending status instead of task state.
- Rationale:
  - These are not harmless TODOs; they sit on security, data-confirmation, and workflow state boundaries.

## 11. Decompose `ai-service/app/routers/parse.py` into feature modules

- Split the 1467-line router into smaller units:
  - contract parsing
  - payment schedule parsing
  - workbook expansion
  - validation / warning generation
  - LLM response normalization
- Evidence:
  - multiple responsibilities already coexist in one file from `ai-service/app/routers/parse.py:1` onward.
- Rationale:
  - The router currently owns request models, prompt design, fallback parsing, validation, normalization, and response shaping.
  - That prevents isolated testing of parsing rules.

## 12. Introduce shared warning/validation builders for AI parsing

- Consolidate warning generation helpers around:
  - missing discount rate
  - missing currency
  - critical field validation
  - lease scope normalization
- Evidence:
  - helpers exist at `ai-service/app/routers/parse.py:52-143`
  - similar validation concepts reappear later in the same file for payment schedules and batch parsing.
- Rationale:
  - The current file repeats the same “missing fields + warnings + human confirmation” pattern across parse modes.

## 13. Replace decorative section comments with structural decomposition

- Remove banner comments that only segment giant files after the code has been modularized.
- Examples:
  - `web/app/ai-chat/page.tsx:55`, `196`
  - `web/app/reports/page.tsx:28`, `48`, `120`, `161`, `171`, `179`, `185`, `211`, `232`
  - `web/app/page.tsx:50`, `54`, `128`, `158`, `174`, `188`
  - `web/app/components/AppLayout.tsx:35`, `57`, `127`
- Rationale:
  - Many comments are only compensating for files that are too large.
  - They do not add domain intent and become stale as files change.

## 14. Remove stale or misleading comments

- Clean comments that no longer reflect current behavior:
  - `web/app/page.tsx:50` says “Mock Chart Data” but the chart is derived from live API data below.
  - `web/app/contracts/new/page.tsx:46` documents a generic fallback instead of policy-driven rate sourcing.
  - `web/app/components/AppLayout.tsx:164` says “Likely UUID”, but the heuristic is just `segment.length > 20`.
- Rationale:
  - Comments should explain domain or non-obvious tradeoffs, not narrate unstable implementation guesses.

## 15. Deduplicate translation content and split `i18n.ts`

- Break `web/app/lib/i18n.ts` (4676 LOC) into feature dictionaries.
- Remove repeated keys/content:
  - `web/app/lib/i18n.ts:1510-1518`
  - `web/app/lib/i18n.ts:2243-2256`
- Rationale:
  - Translation drift is already visible in near-duplicate strings for contract tags vs new-contract tags.
  - A single monolithic file is too large to review safely.

## 16. Remove stray Python package markers from the Go service tree

- Delete empty `__init__.py` files under `core-service/` unless a real Python workflow uses that tree.
- Current empty files:
  - `core-service/internal/__init__.py`
  - `core-service/internal/config/__init__.py`
  - `core-service/internal/handlers/__init__.py`
  - `core-service/internal/middleware/__init__.py`
  - `core-service/internal/models/__init__.py`
  - `core-service/internal/repository/__init__.py`
  - `core-service/pkg/__init__.py`
  - `core-service/pkg/utils/__init__.py`
- Rationale:
  - Empty Python package markers inside a Go service add noise with no observable value.
  - Keep the `ai-service` package markers if the Python import structure needs them.

## 17. Extract repository scan/query helpers to reduce SQL row-mapping repetition

- Introduce shared repository helpers for:
  - scanning repeated structs
  - running list queries with consistent `rows.Next()/Scan()/append()` handling
  - assembling dynamic query filters
- Evidence:
  - repeated row-scan loops in `core-service/internal/repository/monthly_closing.go:157-177`, `241-260`, `318-337`, `371-390`, `663-680`
  - repeated list-and-scan patterns in `core-service/internal/repository/lease_admin.go:107-137`, `140-188`, `232-260`, `304-321`
  - repeated permission/scope scanning in `core-service/internal/repository/role.go:31-55`, `57-79`, `81-105`, `125-147`
  - repeated user fetch scanning in `core-service/internal/repository/user.go:67-107`
- Rationale:
  - The repository layer currently duplicates the same pgx control flow in many files.
  - That makes scan changes and error-shape changes expensive and inconsistent.

## 18. Centralize “resolve or create master data” patterns in the repository layer

- Extract a reusable pattern for lookup-or-create reference data instead of repeating ad hoc insert-on-miss logic.
- Evidence:
  - `core-service/internal/repository/contract.go:85-126` legal entity resolution
  - `core-service/internal/repository/contract.go:128-160` store resolution
  - `core-service/internal/repository/contract.go:162-190` landlord resolution
- Rationale:
  - These functions all follow the same shape with slightly different defaults and SQL.
  - A generic helper or dedicated reference-data service would make the abstraction explicit and safer to extend.

## 19. Unify the role model across docs, backend validation, and frontend administration

- Define one authoritative role enum and use it everywhere.
- Evidence:
  - repository docs describe six roles in `AGENTS.md:37` and `AGENTS.md:202-206`
  - auth validation only accepts four roles in `core-service/internal/handlers/auth.go:22-28` and `49-56`
  - admin UI only renders four roles in `web/app/admin/users/page.tsx:103-115`
  - contract detail UI already checks for `editor` in `web/app/contracts/[id]/page.tsx:1054-1110`
- Rationale:
  - This is both an incomplete abstraction and a consistency bug.
  - The system currently documents, validates, and renders different role sets.

## 20. Introduce a shared discount-rate codec for percent vs decimal handling

- Create one shared rule for rate serialization/deserialization across frontend and backend.
- Evidence:
  - frontend percent conversion in `web/app/contracts/new/page.tsx:41-47`
  - frontend percent conversion in `web/app/settings/page.tsx:55-59`
  - backend normalization in `core-service/internal/handlers/contract.go:111-119`
  - backend global setting normalization in `core-service/internal/handlers/settings.go:48`
  - repository fallback normalization in `core-service/internal/repository/system_setting.go:68`
- Rationale:
  - The same domain rule is currently encoded in multiple places.
  - This invites silent inconsistencies when one side treats `5` as `5%` and another persists it as `5.0`.

## 21. Consolidate report-style table builders across analytics pages

- Extract shared analytics table builders/column factories for cashflow, amortization, and similar grouped report pages.
- Evidence:
  - amortization column builder in `web/app/reports/page.tsx:50-117`
  - cashflow column builder in `web/app/cashflow-forecast/page.tsx:29-78`
  - both pages duplicate the pattern of dimension columns + period columns + financial metric groups
- Rationale:
  - These pages are structurally similar but implement their own column DSL inline.
  - A shared abstraction would reduce drift in alignment, formatting, and export compatibility.

## 22. Remove dead or redundant AI OCR abstraction branches

- Delete or integrate `ai-service/app/services/ocr.py` if the actual extraction path is now fully owned by `document_extractor.py` + `paddleocr.py`.
- Evidence:
  - the OCR helpers are defined in `ai-service/app/services/ocr.py:23-66`
  - no other file imports or calls them in the current repository
  - the active extraction flow lives in `ai-service/app/services/document_extractor.py:28-242`
- Rationale:
  - Keeping an unused parallel OCR abstraction increases maintenance surface and confuses the true execution path.

## 23. Normalize localization and label ownership in smaller frontend pages

- Move hardcoded Chinese UI strings and repeated label maps into shared i18n/constants modules.
- Evidence:
  - asset/scope label maps and hardcoded page copy in `web/app/portfolio/page.tsx:28-48`, `97-123`, `148-205`
  - status and role label maps in `web/app/admin/users/page.tsx:103-115`
  - settings page local tag-contract reference types and modal labels in `web/app/settings/page.tsx:22-32`, `148-214`
- Rationale:
  - Smaller pages currently diverge from the main i18n approach used elsewhere.
  - This is duplication at the presentation/domain-label layer, not just text content.

## 24. Create a cleanup order instead of editing everywhere at once

- Suggested execution order:
  1. Type contracts and API client extraction.
  2. Backend contract mapping + discount-rate policy extraction.
  3. Role enum unification and admin/auth cleanup.
  4. Stub removal/completion on RBAC, discount-rate confirmation, file status.
  5. Repository scan helper extraction.
  6. Split AI parse router and remove dead OCR branches.
  7. Split large frontend pages.
  8. Delete stale comments and redundant files after the structural work lands.

## 25. Archive or remove stale planning and bug-log documents

- Reclassify old one-off working notes into `docs/archive/` or delete them if they no longer serve as reference material.
- Evidence:
  - `BUG_LOG.md` is a resolved deployment diary from 2026-05-12/13 and still describes transitional fixes such as optional OCR packages and merged init SQL creation.
  - `UI_Upgrade_Plan.md`, `findings.md`, `progress.md`, and `task_plan.md` are repository-local working artifacts from an older UI cleanup thread rather than durable project documentation.
- Rationale:
  - These files compete with `AGENTS.md`, `README.md`, and the active docs set for attention.
  - Leaving transient planning artifacts in the repo makes it harder to tell what is authoritative.

## 26. Stop tracking generated Finder and TypeScript build artifacts

- Remove tracked generated files and block them in `.gitignore`:
  - `.DS_Store`
  - `core-service/.DS_Store`
  - `db/.DS_Store`
  - `web/.DS_Store`
  - `web/tsconfig.tsbuildinfo`
- Rationale:
  - These files are machine-local output, not source.
  - Their presence adds review noise and increases the chance of meaningless diffs.

## 27. Collapse the database schema into one authoritative migration path

- Pick one supported database bootstrap story and remove the current dual-maintenance model.
- Evidence:
  - `db/init/01_init.sql` is a monolithic merged schema copy and still contains goose markers/comments from source migrations.
  - `db/migrations/001_initial_schema.sql` through `008_ai_chat_runtime.sql` define the same evolution separately.
  - `Makefile:38-43` says `make migrate` but actually loops over `db/init/*.sql`, not `db/migrations/*.sql`.
- Rationale:
  - The repo currently maintains both an append-only migration history and a handwritten merged init snapshot.
  - That guarantees drift unless regeneration is automated and clearly documented.

## 28. Decouple seed-data assumptions from UI and test helpers

- Treat seed master data as fixture/test data, not as an application contract.
- Evidence:
  - `db/migrations/002_seed_data.sql` inserts fixed UUIDs for legal entities, stores, and landlords.
  - `web/app/contracts/new/page.tsx:153-170` hardcodes the same UUIDs in form options.
  - `scripts/gen_lease_ledger.py` is also tightly bound to current seeded labels and conventions.
- Rationale:
  - This couples runtime behavior, demo data, and helper scripts around one brittle dataset.
  - Production master data should be loaded dynamically, while fixtures should be explicitly labeled as fixtures.

## 29. Reconcile authoritative docs with the implemented stack

- Update or archive docs that still describe superseded architecture choices.
- Evidence:
  - `docs/IFRS16_MVP_技术架构方案.md:27-31` still says `pgx + sqlc + goose`, `PaddleOCR`, and `OpenAI API`.
  - `docs/IFRS16_需求更新清单.md:66-70` repeats the same outdated stack assumptions.
  - The implemented stack in `AGENTS.md`, `README.md`, `.env.example`, and `docker-compose.yml` is `pgx` without `sqlc/goose`, `PaddleOCR-VL-1.5`, and DeepSeek default with OpenAI fallback.
- Rationale:
  - Architecture docs that drift from the running system undermine onboarding and change review.
  - The repo needs a clear distinction between historical design proposals and current source-of-truth documentation.

## 30. Unify auth/session contracts across backend and frontend

- Define one authoritative authenticated-user/session shape and reuse it across handlers, API wrappers, and React context.
- Evidence:
  - `core-service/internal/handlers/auth.go:30-36` defines `AuthResponse`.
  - `web/app/context/AuthContext.tsx:5-15` defines a different local `User` shape.
  - `web/app/login/page.tsx:34-40` and `web/app/admin/login/page.tsx:32-42` both manually remap login payloads into the context shape.
  - `web/app/lib/api.ts:64-78` exposes `authApi.me`, but `core-service/internal/handlers/user.go:10-18` returns only `user_id`, `username`, and `role`, omitting `legal_entity_id` even though JWT middleware stores it in `core-service/internal/middleware/auth.go:42`.
- Rationale:
  - Authentication state is a boundary contract and should not drift between login responses, `/me`, local storage, and JWT claims.
  - The current setup duplicates mapping logic and makes tenant-context behavior easy to mis-handle.

## 31. Extract a shared analysis-page scaffold for standards and sensitivity views

- Consolidate the repeated contract-selector + override-rate + chart/statistics/table layout used by management analytics pages.
- Evidence:
  - `web/app/standards/page.tsx` and `web/app/sensitivity/page.tsx` both:
    - load approved contracts through `contractApi.list(...)`
    - keep local `contracts`, `rows`, `meta`, and `loading` state
    - map percent inputs back to decimal request payloads
    - render nearly identical page scaffolding with `ProtectedRoute`, `AppLayout`, `motion.div`, `Form`, `Alert`, `Statistic`, chart, and details table
- Rationale:
  - These pages are feature variants of the same workflow, but each currently owns its own data-loading and layout choreography.
  - A shared scaffold/hook would reduce drift in filtering, error handling, and decimal-percent conversions.

## 32. Remove tracked compiled/runtime artifacts beyond Finder metadata

- Stop tracking built executables and generated fixture outputs alongside source.
- Evidence:
  - `core-service/api` is a compiled Mach-O executable, not source.
  - `scripts/gen_lease_ledger.py` writes the repository-root fixture `合同台账_测试数据.xlsx`, which is currently committed.
- Rationale:
  - Built binaries and generated workbooks should either be reproducible artifacts under an explicit fixture/testdata convention or excluded from source control.
  - Keeping them at the repo root blurs the line between maintained source and local/generated output.

## 33. Make the design system tokens single-source across TypeScript and CSS

- Collapse duplicated visual-token definitions so Ant Design theme config, animation helpers, and global CSS derive from one canonical token set.
- Evidence:
  - `web/app/design-system/tokens.ts` defines monochrome color, depth, typography, motion, radius, and layout tokens.
  - `web/app/design-system/theme.ts` maps many of the same literal values into Ant Design theme overrides.
  - `web/app/globals.css:7-55` redefines the monochrome palette, shadows, and easing values again as CSS variables.
- Rationale:
  - The visual system is currently described in parallel in TS objects and raw CSS custom properties.
  - That duplication will drift and makes theme changes more expensive than they need to be.

## 34. Remove orphan package metadata at the repository root

- Delete or justify root-level Node package artifacts that are not backed by an actual root package.
- Evidence:
  - `package-lock.json` exists at the repository root, but there is no matching root `package.json`.
  - The active frontend package is clearly `web/package.json`.
- Rationale:
  - A lockfile without a package manifest is almost always leftover tooling output.
  - It creates confusion about whether the repository is intended to have a root Node workspace.

## Notes for the next review pass

- This pass inspected the highest-risk files and repeated patterns first.
- Remaining work for a full-file sweep:
  - continue page-by-page review of smaller frontend pages
  - inspect repositories for repeated SQL fragments and scan/row mapping helpers
  - inspect docs/scripts for stale assumptions after the structural cleanup starts
