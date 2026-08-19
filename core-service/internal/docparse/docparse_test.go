package docparse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"a.csv", nil, "csv"},
		{"a.PDF", []byte("whatever"), "pdf"},
		{"mystery", []byte("%PDF-1.7 ..."), "pdf"},
		{"mystery", []byte("PK\x03\x04..."), "zip-office"},
		{"a.xlsx", nil, "xlsx"},
		{"noext", []byte("hello"), "unknown"},
	}
	for _, c := range cases {
		if got := DetectFormat(c.name, c.data); got != c.want {
			t.Fatalf("DetectFormat(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCSVParseComma(t *testing.T) {
	doc, err := CSV().Parse(context.Background(), Source{
		Filename: "stores.csv",
		Data:     []byte("store,revenue\nS1,1200\nS2,980\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "csv" || doc.EvidenceMode != EvidenceQuote {
		t.Fatalf("wrong format/evidence: %+v", doc)
	}
	if !strings.Contains(doc.Markdown, "| store | revenue |") || !strings.Contains(doc.Markdown, "| S1 | 1200 |") {
		t.Fatalf("unexpected markdown: %q", doc.Markdown)
	}
}

func TestCSVParseTabAndBOM(t *testing.T) {
	doc, err := CSV().Parse(context.Background(), Source{
		Filename: "sales.tsv",
		Data:     []byte("\xEF\xBB\xBFstore\trevenue\nS1\t1200\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "tsv" {
		t.Fatalf("tab sniffing failed, format=%s", doc.Format)
	}
	if !strings.Contains(doc.Markdown, "| store | revenue |") {
		t.Fatalf("BOM must be stripped, got %q", doc.Markdown)
	}
}

func TestCSVSizeLimit(t *testing.T) {
	_, err := CSV().Parse(context.Background(), Source{
		Filename:  "big.csv",
		Data:      []byte("a,b\n1,2\n"),
		SizeLimit: 4,
	})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("size limit must yield ErrFileTooLarge, got %v", err)
	}
}

func TestCSVDamagedFails(t *testing.T) {
	_, err := CSV().Parse(context.Background(), Source{
		Filename: "bad.csv",
		Data:     []byte("a,b\n\"unterminated\n"),
	})
	if !errors.Is(err, ErrParseUnsupported) {
		t.Fatalf("damaged CSV must be classified parse_unsupported, got %v", err)
	}
}

func TestParseErrorCode(t *testing.T) {
	if ParseErrorCode(ErrFileTooLarge) != "file_too_large" ||
		ParseErrorCode(ErrFileEncrypted) != "file_encrypted" ||
		ParseErrorCode(ErrParserUnavailable) != "parser_unavailable" ||
		ParseErrorCode(ErrParseUnsupported) != "parse_unsupported" ||
		ParseErrorCode(errors.New("x")) != "parse_failed" {
		t.Fatal("error code mapping broken")
	}
}

func TestAnyDocUnavailableWithoutBinary(t *testing.T) {
	_, err := AnyDoc("", time.Second).Parse(context.Background(), Source{Filename: "a.docx", Data: []byte("x")})
	if !errors.Is(err, ErrParserUnavailable) {
		t.Fatalf("empty binary path must yield ErrParserUnavailable, got %v", err)
	}
	_, err = AnyDoc("/nonexistent/anydoc", time.Second).Parse(context.Background(), Source{Filename: "a.docx", Data: []byte("x")})
	if !errors.Is(err, ErrParserUnavailable) {
		t.Fatalf("missing binary must yield ErrParserUnavailable, got %v", err)
	}
}

func TestAnyDocRunsStubBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "anydoc-stub")
	script := "#!/bin/sh\ncat \"$1\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	doc, err := AnyDoc(bin, 5*time.Second).Parse(context.Background(), Source{
		Filename: "contract.docx",
		Data:     []byte("# 租赁合同\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Markdown, "# 租赁合同") {
		t.Fatalf("stub output must become markdown, got %q", doc.Markdown)
	}
	if doc.EvidenceMode != EvidenceQuote {
		t.Fatalf("office documents carry Quote evidence, got %s", doc.EvidenceMode)
	}
}

func TestAnyDocPDFIsUnavailableEvidence(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "anydoc-stub")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	doc, err := AnyDoc(bin, 5*time.Second).Parse(context.Background(), Source{
		Filename: "scan.pdf",
		Data:     []byte("%PDF-1.4 text"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.EvidenceMode != EvidenceUnavailable {
		t.Fatalf("first-round PDF text must not claim evidence, got %s", doc.EvidenceMode)
	}
}
