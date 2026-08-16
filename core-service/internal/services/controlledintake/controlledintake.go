// Package controlledintake is the deep core behind every controlled
// file importer (design: improvement-review candidate 1). Three importers —
// store-day facts, FP&A plan versions, GL trial balances — shared one shape:
// raw upload → controlled template rows → row-level errors → partial
// success → compensation on failure. Each hand-rolled its own file reading,
// size cap, parsing and error shape; this module concentrates that shape
// behind a small interface so the fourth importer only writes domain logic.
//
// Interface: Parse (file → rows), Reporter (the one row-error contract),
// Compensator (created-entity rollback discipline). Everything else is
// hidden. Raw cell values never leave the deterministic path.
package controlledintake

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/controlledxlsx"
)

// MaxSourceSize caps every controlled upload (10MB); one place to change.
const MaxSourceSize = 10 << 20

// Format identifies the controlled template container.
type Format string

const (
	FormatCSV  Format = "csv"
	FormatXLSX Format = "xlsx"
)

// Source is one raw upload. The module owns size caps and format
// detection; callers own acquiring the bytes.
type Source struct {
	Filename string
	Data     []byte
}

// Parse reads the controlled template behind one entry: size cap, format
// detection by extension, CSV/XLSX parsing through the shared reader,
// header/row splitting and empty-row dropping. Every importer crosses this
// seam, so template semantics are defined once.
func Parse(source Source) (headers []string, rows [][]string, err error) {
	if len(source.Data) > MaxSourceSize {
		return nil, nil, fmt.Errorf("file exceeds the %dMB import limit", MaxSourceSize>>20)
	}
	format := detectFormat(source.Filename)
	var grid [][]string
	switch format {
	case FormatXLSX:
		grid, err = controlledxlsx.Read(source.Data)
		if err != nil {
			return nil, nil, err
		}
	case FormatCSV:
		reader := csv.NewReader(strings.NewReader(string(source.Data)))
		reader.TrimLeadingSpace = true
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true
		grid, err = reader.ReadAll()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid CSV: %w", err)
		}
	default:
		return nil, nil, errors.New("unsupported template format")
	}
	if len(grid) == 0 {
		return nil, nil, errors.New("template contains no header row")
	}
	trimmed := make([][]string, 0, len(grid))
	for _, row := range grid {
		values := make([]string, len(row))
		for i, value := range row {
			values[i] = strings.TrimSpace(value)
		}
		trimmed = append(trimmed, values)
	}
	lastMeaningful := -1
	for i, header := range trimmed[0] {
		if header != "" {
			lastMeaningful = i
		}
	}
	if lastMeaningful < 0 {
		return nil, nil, errors.New("template header row is empty")
	}
	headers = trimmed[0][:lastMeaningful+1]
	for _, row := range trimmed[1:] {
		if rowEmpty(row) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, nil, errors.New("template contains no data rows")
	}
	return headers, rows, nil
}

// detectFormat maps the filename extension; anything non-xlsx is CSV.
func detectFormat(filename string) Format {
	if strings.EqualFold(filename[strings.LastIndex(filename, ".")+1:], "xlsx") {
		return FormatXLSX
	}
	return FormatCSV
}

func rowEmpty(row []string) bool {
	for _, value := range row {
		if value != "" {
			return false
		}
	}
	return true
}

// RowError is the single row-level error contract shared by every importer
// (1-based data row index, machine code, human message, optional column
// context). UIs render one shape; exporters persist one shape.
type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Reporter accumulates row errors in row order.
type Reporter struct {
	errors []RowError
}

func NewReporter() *Reporter { return &Reporter{} }

func (r *Reporter) Add(row int, code, message string) {
	r.errors = append(r.errors, RowError{Row: row, Code: code, Message: message})
}

func (r *Reporter) Errors() []RowError {
	return r.errors
}

func (r *Reporter) Count() int { return len(r.errors) }

// Compensator owns the created-entity rollback discipline: a commit that
// fails midway must delete what it created. Success() seals the commit;
// Fail() runs the rollback exactly once. Two real adapters today (plan
// version delete, trial balance version delete) make the seam real.
type Compensator struct {
	rollback func() error
	done     bool
}

func NewCompensator(rollback func() error) *Compensator {
	return &Compensator{rollback: rollback}
}

func (c *Compensator) Success() {
	if c == nil || c.done {
		return
	}
	c.done = true
}

// Fail rolls the created entity back; a nil Compensator is a no-op (nothing
// was created to compensate).
func (c *Compensator) Fail() error {
	if c == nil || c.done || c.rollback == nil {
		return nil
	}
	c.done = true
	return c.rollback()
}
