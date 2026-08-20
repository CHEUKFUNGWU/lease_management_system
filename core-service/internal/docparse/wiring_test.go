package docparse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubParser struct {
	name string
	doc  ParsedDocument
	err  error
}

func (s stubParser) Parse(_ context.Context, src Source) (ParsedDocument, error) {
	return s.doc, s.err
}

func stubOffice() DocumentParser {
	dir, _ := os.MkdirTemp("", "docparse-router-")
	bin := filepath.Join(dir, "anydoc-stub")
	_ = os.WriteFile(bin, []byte("#!/bin/sh\ncat \"$1\"\n"), 0o700)
	return AnyDoc(bin, 5*time.Second)
}

type countingParser struct {
	n   int
	doc ParsedDocument
}

func (c *countingParser) Parse(_ context.Context, src Source) (ParsedDocument, error) {
	c.n++
	return c.doc, nil
}

// W5-1 wiring tests: the router dispatches by format and never claims evidence
// it cannot produce.

func TestRouterRoutesCSVToCSVParser(t *testing.T) {
	csv := stubParser{name: "csv", doc: ParsedDocument{Format: "csv", Markdown: "| a |", EvidenceMode: EvidenceQuote}}
	r := NewRouter(csv, stubOffice(), nil, nil)
	doc, err := r.Parse(context.Background(), Source{Filename: "stores.csv", Data: []byte("a,b\n1,2\n")})
	if err != nil || doc.Format != "csv" {
		t.Fatalf("csv routing failed: %#v err=%v", doc, err)
	}
	if len(doc.Locators) != 0 {
		t.Fatal("csv must never claim coordinates")
	}
}

func TestRouterOfficeFamilyGoesToAnyDocWithoutCoordinates(t *testing.T) {
	office := stubParser{name: "office", doc: ParsedDocument{Format: "docx", Markdown: "text", EvidenceMode: EvidenceQuote}}
	r := NewRouter(stubParser{}, office, nil, nil)

	for _, f := range []string{"a.docx", "a.pptx", "a.xlsx", "a.odt", "a.rtf", "a.epub"} {
		doc, err := r.Parse(context.Background(), Source{Filename: f, Data: []byte("x")})
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if doc.EvidenceMode == EvidenceCoordinate {
			t.Fatalf("%s must not claim coordinates without OCR: %#v", f, doc)
		}
	}
}

func TestRouterPDFFirstPassUsesAnyDocAndClaimsNoCoordinates(t *testing.T) {
	// The real AnyDoc adapter marks first-pass PDFs EvidenceUnavailable
	// (D7 lazy evidence: text yes, coordinates no).
	r := NewRouter(stubParser{}, stubOffice(), nil, nil)
	doc, err := r.Parse(context.Background(), Source{Filename: "scan.pdf", Data: []byte("%PDF-1.4 x")})
	if err != nil {
		t.Fatalf("pdf first pass must succeed via anydoc: %v", err)
	}
	if doc.EvidenceMode != EvidenceUnavailable {
		t.Fatalf("pdf first pass must not claim coordinates: %#v", doc)
	}
}

func TestRouterEvidenceRequestRunsOCR(t *testing.T) {
	ocr := &countingParser{doc: ParsedDocument{Format: "pdf", Markdown: "scan text", EvidenceMode: EvidenceCoordinate}}
	r := NewRouter(stubParser{}, stubOffice(), ocr, func() bool { return true })

	// Without an evidence request the OCR adapter must not run (D7 lazy).
	doc, err := r.Parse(context.Background(), Source{Filename: "scan.pdf", Data: []byte("%PDF-1.4 x")})
	if err != nil || doc.EvidenceMode != EvidenceUnavailable {
		t.Fatalf("first pass must degrade to unavailable evidence: %#v err=%v", doc, err)
	}
	if ocr.n != 0 {
		t.Fatalf("lazy OCR ran on first pass: calls=%d", ocr.n)
	}

	// User requests evidence -> OCR runs and coordinates are claimable.
	doc, err = r.Parse(context.Background(), Source{Filename: "scan.pdf", Data: []byte("%PDF-1.4 x"), NeedEvidence: true})
	if err != nil || doc.EvidenceMode != EvidenceCoordinate || ocr.n != 1 {
		t.Fatalf("evidence request must run OCR: calls=%d doc=%#v err=%v", ocr.n, doc, err)
	}
}

func TestRouterOCRAvailabilityPredicateGatesRouting(t *testing.T) {
	ocr := &countingParser{doc: ParsedDocument{Format: "pdf", Markdown: "scan text", EvidenceMode: EvidenceCoordinate}}
	r := NewRouter(stubParser{}, stubOffice(), ocr, func() bool { return false }) // OCR configured but disabled

	// With OCR disabled the router must degrade to anydoc and never claim
	// coordinates (D8).
	doc, err := r.Parse(context.Background(), Source{Filename: "scan.pdf", Data: []byte("%PDF-1.4 x"), NeedEvidence: true})
	if err != nil {
		t.Fatalf("disabled OCR must degrade to anydoc text: %v", err)
	}
	if doc.EvidenceMode == EvidenceCoordinate {
		t.Fatalf("disabled OCR must never claim coordinates: %#v", doc)
	}
	if ocr.n != 0 {
		t.Fatalf("disabled OCR must not run: calls=%d", ocr.n)
	}

	r2 := NewRouter(stubParser{}, AnyDoc("/nonexistent/anydoc", time.Second), nil, nil)
	_, err = r2.Parse(context.Background(), Source{Filename: "a.docx", Data: []byte("x")})
	if !errors.Is(err, ErrParserUnavailable) {
		t.Fatalf("missing anydoc binary must fail parser_unavailable: err=%v", err)
	}
}

func TestRouterImageWithoutOCRIsUnavailable(t *testing.T) {
	r := NewRouter(stubParser{}, stubOffice(), nil, nil)
	doc, err := r.Parse(context.Background(), Source{Filename: "scan.png", Data: []byte("png")})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if doc.EvidenceMode == EvidenceCoordinate {
		t.Fatalf("image without OCR must never claim coordinates: %#v", doc)
	}
}

var _ = filepath.Join
var _ = os.MkdirTemp
var _ = time.Second
