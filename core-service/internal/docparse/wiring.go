package docparse

import (
	"context"
)

// Router is the assembly point W5-1 wires into the Agent. It implements the
// ADR-0024 §2 routing table: CSV/TSV go to the deterministic CSV adapter, PDF
// with an evidence request goes to OCR, and office-family text extraction goes
// to anydoc. When OCR is unavailable the router degrades to anydoc text and
// marks evidence Unavailable (D8) — it never claims coordinates it does not
// have.
type Router struct {
	CSV    DocumentParser // CSV/TSV deterministic parser
	Office DocumentParser // anydoc subprocess adapter (office family + text-PDF first pass)
	OCR    DocumentParser // coordinate-capable OCR (PDF/images on demand)
	// ocrAvailable reports whether the OCR adapter is configured and usable.
	ocrAvailable func() bool
}

// NewRouter assembles the default production parser stack. The anydoc binary
// path is injected by the caller; its supply-chain pin lives in the Dockerfile
// and docker-compose.
func NewRouter(csvParser DocumentParser, office DocumentParser, ocr DocumentParser, ocrAvailable func() bool) *Router {
	if csvParser == nil {
		csvParser = CSV()
	}
	if office == nil {
		office = AnyDoc("", 0)
	}
	return &Router{CSV: csvParser, Office: office, OCR: ocr, ocrAvailable: ocrAvailable}
}

// Parse routes the source to the adapter that can honestly serve it.
func (r *Router) Parse(ctx context.Context, src Source) (ParsedDocument, error) {
	if r == nil {
		return ParsedDocument{}, ErrParserUnavailable
	}
	format := DetectFormat(src.Filename, src.Data)
	switch {
	case format == "csv" || format == "tsv":
		return r.CSV.Parse(ctx, src)
	case (format == "pdf" || format == "image") && src.NeedEvidence:
		// D7 lazy evidence: OCR runs only when the caller actually needs it.
		return r.ocrOrOffice(ctx, src)
	case format == "image":
		// Images have no text layer; without OCR there is nothing to extract.
		return r.ocrOrOffice(ctx, src)
	default:
		// Office family and text PDF first pass: anydoc produces text without
		// claiming coordinates.
		return r.Office.Parse(ctx, src)
	}
}

func (r *Router) ocrOrOffice(ctx context.Context, src Source) (ParsedDocument, error) {
	if r.OCR != nil && r.ocrAvailable != nil && r.ocrAvailable() {
		return r.OCR.Parse(ctx, src)
	}
	doc, err := r.Office.Parse(ctx, src)
	if err != nil {
		return doc, err
	}
	// D8: text is not evidence. Without OCR there is no coordinate to claim.
	doc.EvidenceMode = EvidenceUnavailable
	return doc, nil
}

// OCREnabled returns a predicate for NewRouter that reports whether the given
// OCR client is configured and usable.
func OCREnabled(c *PaddleOCRClient) func() bool {
	return func() bool { return c != nil && c.Available() }
}
