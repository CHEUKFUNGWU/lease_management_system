// Package docparse is the document understanding layer: format detection,
// deterministic CSV parsing, the anydoc subprocess adapter and the PaddleOCR
// client. The EvidenceMode type encodes the honest-degradation rule from
// ADR-0024 §4: text without coordinates must never claim coordinates.
package docparse

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// EvidenceMode declares what evidence a parse can honestly claim.
type EvidenceMode string

const (
	// EvidenceQuote means the text carries quote anchors but no coordinates
	// (office documents through anydoc).
	EvidenceQuote EvidenceMode = "Quote"
	// EvidenceCoordinate means block-level {page, coordinates, quote} evidence
	// is available (PDF through PaddleOCR).
	EvidenceCoordinate EvidenceMode = "Coordinate"
	// EvidenceUnavailable means there is text but no evidence anchoring —
	// parsers must not claim any locator in this mode (ADR-0024 §4).
	EvidenceUnavailable EvidenceMode = "Unavailable"
)

// ParsedDocument is the product of a parser.
type ParsedDocument struct {
	Markdown     string       `json:"markdown"`
	Format       string       `json:"format"`
	EvidenceMode EvidenceMode `json:"evidence_mode"`
	Locators     []Locator    `json:"locators,omitempty"`
	Warnings     []string     `json:"warnings,omitempty"`
}

// Locator is one block-level evidence anchor from OCR.
type Locator struct {
	Page        int    `json:"page"`
	Coordinates []int  `json:"coordinates"` // [x0, y0, x1, y1]
	Quote       string `json:"quote"`
	Source      string `json:"source"`
}

// Source is the input a parser consumes.
type Source struct {
	Data      []byte
	Filename  string
	SizeLimit int64 // <= 0 means no limit
	// NeedEvidence marks a caller request for OCR evidence (D7 lazy
	// evidence): a first pass runs anydoc and only opens OCR when a user
	// actually inspects evidence for this document.
	NeedEvidence bool
}

// DocumentParser is the single seam every file path crosses. Adapters differ
// only in how they produce the ParsedDocument; the error classification is
// shared.
type DocumentParser interface {
	Parse(ctx context.Context, src Source) (ParsedDocument, error)
}

// Classified errors — never a silent success. Callers branch on these via
// errors.Is, and surfaces map them via ParseErrorCode.
var (
	ErrParseUnsupported  = errors.New("docparse: unsupported or damaged file")
	ErrFileEncrypted     = errors.New("docparse: file is encrypted")
	ErrFileTooLarge      = errors.New("docparse: file exceeds size limit")
	ErrParserUnavailable = errors.New("docparse: parser unavailable")
)

// ParseErrorCode maps any parse error to the stable vocabulary of the
// working-paper design §8: parse_unsupported, file_encrypted, file_too_large,
// parser_unavailable, parse_failed.
func ParseErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrFileTooLarge):
		return "file_too_large"
	case errors.Is(err, ErrFileEncrypted):
		return "file_encrypted"
	case errors.Is(err, ErrParserUnavailable):
		return "parser_unavailable"
	case errors.Is(err, ErrParseUnsupported):
		return "parse_unsupported"
	default:
		return "parse_failed"
	}
}

// CheckSize enforces the source size limit when one is set.
func CheckSize(src Source) error {
	if src.SizeLimit > 0 && int64(len(src.Data)) > src.SizeLimit {
		return ErrFileTooLarge
	}
	return nil
}

// DetectFormat returns the canonical format token for a file, from extension
// first and content magic second.
func DetectFormat(filename string, data []byte) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".doc":
		return "doc"
	case ".xlsx":
		return "xlsx"
	case ".xls":
		return "xls"
	case ".pptx":
		return "pptx"
	case ".odt":
		return "odt"
	case ".rtf":
		return "rtf"
	case ".epub":
		return "epub"
	case ".csv":
		return "csv"
	case ".tsv", ".tab":
		return "tsv"
	case ".jpg", ".jpeg", ".png", ".tiff", ".tif", ".bmp", ".webp":
		return "image"
	}
	// Content magic for files with missing or wrong extensions.
	if len(data) >= 4 && string(data[:4]) == "%PDF" {
		return "pdf"
	}
	if len(data) >= 2 && string(data[:2]) == "PK" {
		return "zip-office"
	}
	return "unknown"
}
