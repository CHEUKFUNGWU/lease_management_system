# Remove the AGPL PDF dependency and split document parsing by evidence need

Status: Accepted

## Context

`ai-service/requirements.txt` pins `pymupdf==1.23.0`, used in
`app/services/document_extractor.py` for PDF text extraction and for the
evidence-locator fallback.

PyMuPDF is dual-licensed: AGPL-3.0, or a commercial licence sold by Artifex as
the exclusive agent for MuPDF. The `fitz` import name is an alias and carries
identical obligations.

This conflicts directly with the posture ADR-0021 already adopted. That decision
places the platform under BSL / Fair Source and the standalone measurement
library under Apache-2.0, on the reasoning that the encoded domain knowledge is
the moat and that mainland delivery is frequently 私有化部署. A strong-copyleft
network-triggered dependency inside a source-available platform is not
compatible with that position, and ADR-0021 §4.2 makes on-premise delivery the
common case rather than the exception — which is precisely the scenario AGPL
distribution terms bite hardest.

Separately, examining what the current code actually produces changes the
cost-benefit. `extract_text_with_evidence` computes its bounding box as the
min/max over every word on the page and labels the locator
`document.pages[i]`. The stored coordinates therefore describe the whole page.
The evidence is page-level with a precise-looking box around it.

Meanwhile the PaddleOCR path already returns better evidence.
`_structured_locators` (`app/services/paddleocr.py:253`) walks
`prunedResult` / `parsing_res_list` and emits block-level
`{page, coordinates, quote}`, and it is deliberately conservative: it accepts a
locator only when both text and a box are present, and refuses to promote a
Markdown-only result into a coordinate claim.

So removing PyMuPDF removes the weaker of the two evidence sources, not the
stronger one.

Two replacements were evaluated. **Docling** (IBM, MIT) returns element-level
`prov` with `page_no`, `bbox` and `charspan`, includes OCR, and runs offline —
but it is Python with ML model weights, which contradicts ADR-0023, and its
model prefetch and image size are real operational costs. **anydoc**
(Firecrawl, MIT, pure Rust, no ML, no network) covers the office family plus
text PDF with a CLI, Node/Python bindings and WASM — but returns Markdown with
no positional information and no OCR.

## Decision

### 1. PyMuPDF is removed

No AGPL dependency ships in the platform. This applies to transitive
replacements as well: `gen2brain/go-fitz` binds the same MuPDF core under the
same terms and is equally excluded.

### 2. Parsing is split by whether evidence coordinates are required

| Input | Parser | Evidence |
|---|---|---|
| Office family (docx, pptx, xlsx, odf, rtf, epub, csv) | **anydoc** (Rust CLI, subprocess) | none claimed |
| Text-layer PDF, first pass | **anydoc** | none claimed |
| Scanned PDF and images | **PaddleOCR API** | block-level `{page, coordinates, quote}` |
| PDF where a user requests evidence | **PaddleOCR API** | block-level |
| Excel read and write | **excelize** | n/a |

Docling is not adopted. It is recorded here as the fallback if PaddleOCR's
Chinese-layout advantage does not hold, or if a customer requires fully offline
OCR.

### 3. Evidence is fetched lazily

A first pass runs anydoc: fast, local, free, sufficient to extract fields and
produce a draft. OCR runs only when a user opens evidence for a specific
document, and the resulting locators are cached and reused.

This moves OCR cost from per-file to per-file-a-user-actually-inspects, and
removes the OCR round trip from the first-response path.

### 4. Absent evidence is stated, never inferred

The failure rule is explicit and must be enforced in code rather than by
convention:

> **OCR unavailable → fall back to anydoc, return text, mark evidence status
> `unavailable`, and claim no coordinates.**

This replaces the previous rule ("fall back to the PyMuPDF text layer"), which
is no longer implementable. It is a direct application of R3 in
`docs/AI_底稿与Paperwork_Agent设计方案.md`: text is not evidence, and having
text must never be allowed to masquerade as having a locator.

### 5. The parser seam stays swappable

`internal/docparse` defines a `DocumentParser` interface with the backend behind
it. Swapping the OCR backend — to self-hosted PaddleOCR, or to Docling — is a
configuration and adapter change, not a change to calling code.

## Consequences

**The licence conflict with ADR-0021 is resolved**, and one fewer item requires
legal review before an on-premise deployment.

**Evidence quality improves.** Page-level boxes with false precision are
replaced by block-level locators, or by an honest `unavailable`. The
working-paper drill-down described in
`docs/AI_底稿与Paperwork_Agent设计方案.md` §7.2 depends on this granularity.

**Text-layer PDFs lose their free coordinate source.** This is the real cost of
the decision, and §3 is the mitigation rather than a denial of it. Where a
workflow needs coordinates on every document, that workflow pays for OCR.

**A Rust binary joins the build.** anydoc is invoked as a subprocess rather than
linked via cgo, keeping the process boundary clean and avoiding cgo build
complexity. The binary must be pinned by version and checksum.

**PaddleOCR becomes a hard dependency for evidence**, where previously a local
fallback existed. The failure semantics in §4 are what make that acceptable:
the system degrades to "no evidence available" rather than to "evidence of
unknown quality".

## Non-goals

- Not a decision about where OCR runs. Cloud versus self-hosted PaddleOCR is
  deferred to the first lighthouse customer's requirements.
- Not a decision to remove OCR from the product. PaddleOCR remains the primary
  engine; only PyMuPDF is removed.
- Not an evaluation of anydoc for OCR. It has none, by design.
