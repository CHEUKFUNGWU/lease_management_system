# Retire the first-party Python AI service

Status: Accepted

## Context

`ai-service` is a Python 3.11 FastAPI process of roughly 2 500 lines. Reading it
by responsibility rather than by file shows that very little of it is Python for
a reason:

| Component | Lines | What it actually is |
|---|---|---|
| `services/llm.py` | 144 | HTTP client for two providers |
| `routers/agent_plan.py` | 170 | JSON in, JSON out, one model call |
| `routers/chat.py` | 117 | HTTP passthrough |
| `routers/files.py`, `services/storage.py` | 176 | MinIO glue |
| `services/paddleocr.py` | 446 | Submit, poll, walk JSON |
| `intake/` | ~400 | Schema definition and prompt assembly |
| `document_extractor.py` — Excel | — | `openpyxl` |
| `document_extractor.py` — PDF | — | `PyMuPDF` |

Roughly nine tenths is HTTP glue and pure logic that Go expresses at least as
well. Two parts are not: Excel handling and PDF text extraction.

Excel turns out to favour Go. `excelize` both reads and writes, and the writing
half is a capability the product currently lacks entirely — every export today
is CSV (`handlers/monthly_closing.go`, `handlers/retail_export.go`), which is
why the working-paper deliverable in
`docs/AI_底稿与Paperwork_Agent设计方案.md` has no output format.

PDF is the only genuine obstacle, and it is resolved separately by ADR-0024.

Two further facts weigh on the decision. `agent_plan.py` exists only because the
brain lives outside Core; once the agent core is in-process (ADR-0022), that
endpoint and its shared `AGENT_PLANNER_TOKEN` are pure overhead. And the
alternative direction on the table — adding a tau container — would have moved
the stack the opposite way, to two Python services.

## Decision

### 1. `ai-service` is retired; no first-party Python remains

Backend runtimes reduce to Go. Node remains for the frontend only.

### 2. Responsibilities are relocated as follows

| From | To |
|---|---|
| `services/llm.py` | `internal/llm` (ADR-0022 §4) |
| `routers/agent_plan.py`, `routers/chat.py` | `internal/agentcore` loop, in-process |
| `routers/files.py`, `services/storage.py` | `minio-go` inside core-service |
| `services/paddleocr.py` | `internal/docparse/paddleocr.go` |
| `intake/` | `internal/aiintake` (package already exists) |
| Excel read and write | `excelize` |
| Office family, text-layer PDF | `anydoc` (ADR-0024) |
| Scanned PDF, evidence coordinates | PaddleOCR API (ADR-0024) |

### 3. A third-party container is not first-party Python

If a vendor's official container is later adopted for document processing, it is
a dependency of the same kind as PostgreSQL or MinIO. The commitment here is
that *this team maintains no Python*, not that no Python exists anywhere in the
deployment.

### 4. Retirement is the last wave, not the first

Migration order is W1–W3 (agent core, no external behaviour change), W4
(`internal/llm`, retire `chat.py` and `agent_plan.py`), W5 (`internal/docparse`,
retire the rest). Each wave must pass `agent-evaluation.v1` and the skill
contract replay. Extraction accuracy at W5 must not regress against the CORR-2
baseline.

## Consequences

**One less service, one less hop, one less shared secret.** The planner HTTP
round trip and `AGENT_PLANNER_TOKEN` disappear with `agent_plan.py`.

**The xlsx writer arrives as a side effect.** Adopting `excelize` for the Excel
path supplies the working-paper output format for free. This is the largest
incidental benefit of the migration and should be scheduled to land with W5, not
after it.

**Extraction accuracy carries the risk.** Everything else in this migration is
mechanical; the parser swap is not. W5 does not pass on "it compiles" — it
passes on the labelled corpus.

**Python expertise stops being on the critical path** for hiring and on-call.

**`controlledxlsx` is not deleted by default.** The dependency-free reader for
the controlled template is a virtue, not duplication. Whether it merges into the
`excelize` path is left open in `docs/Agent_Core_Go设计_对齐pi架构.md` §12.

## Non-goals

- Not a rewrite of `agenttools`, the Gateway contract, or the CLI surface.
- Not a change to the PaddleOCR provider relationship — only to which process
  calls it.
- Not a claim that Go is a better language than Python. The claim is narrower:
  this particular service is glue, and glue belongs where the callers are.
