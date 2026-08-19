package docparse

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
)

// maxCSVRows caps how many data rows the deterministic parser will render.
// Beyond the cap the parse reports a warning instead of silently dropping
// rows.
const maxCSVRows = 10000

// CSV returns the deterministic CSV/TSV parser. ERP exports default to CSV,
// so this path must never touch a model: delimiter sniffing and rendering are
// pure stdlib.
func CSV() DocumentParser { return csvParser{} }

type csvParser struct{}

func (csvParser) Parse(ctx context.Context, src Source) (ParsedDocument, error) {
	if err := ctx.Err(); err != nil {
		return ParsedDocument{}, err
	}
	if err := CheckSize(src); err != nil {
		return ParsedDocument{}, err
	}

	data := stripBOM(src.Data)
	delim := sniffDelimiter(data)
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delim
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("%w: %v", ErrParseUnsupported, err)
	}
	if len(records) == 0 {
		return ParsedDocument{Markdown: "", Format: "csv", EvidenceMode: EvidenceQuote}, nil
	}

	var warnings []string
	truncated := false
	if len(records) > maxCSVRows+1 {
		records = records[:maxCSVRows+1]
		truncated = true
		warnings = append(warnings, fmt.Sprintf("CSV truncated at %d data rows", maxCSVRows))
	}

	format := "csv"
	if delim == '\t' {
		format = "tsv"
	}

	var b strings.Builder
	header := records[0]
	b.WriteString("| ")
	b.WriteString(strings.Join(header, " | "))
	b.WriteString(" |\n| ")
	for range header {
		b.WriteString("--- | ")
	}
	b.WriteString("\n")
	for _, rec := range records[1:] {
		b.WriteString("| ")
		b.WriteString(strings.Join(rec, " | "))
		b.WriteString(" |\n")
	}
	if truncated {
		b.WriteString("\n*(truncated)*\n")
	}

	return ParsedDocument{
		Markdown:     b.String(),
		Format:       format,
		EvidenceMode: EvidenceQuote,
		Warnings:     warnings,
	}, nil
}

func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// sniffDelimiter picks comma or tab by counting occurrences in the first
// non-empty line.
func sniffDelimiter(data []byte) rune {
	lineEnd := bytes.IndexAny(data, "\r\n")
	line := data
	if lineEnd >= 0 {
		line = data[:lineEnd]
	}
	commas := bytes.Count(line, []byte(","))
	tabs := bytes.Count(line, []byte("\t"))
	if tabs > commas {
		return '\t'
	}
	return ','
}
