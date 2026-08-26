// Package retailexport is the platform export module (design M3, PRD
// P1-13/14/19): the export behaviour and provenance header are implemented
// once for every consumer. The Go side owns the two server-authoritative
// halves — the column/formula descriptor (single source, served to the web
// exporters) and the CSV renderer with the provenance header, Working marks
// and formula-injection escaping. The XLSX (ExcelJS) and PPTX (pptxgenjs)
// halves live on the web side and consume the same response plus the same
// descriptor, never a second column list.
package retailexport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ExportKind identifies one export surface.
type ExportKind string

const (
	KindOperatingPulse   ExportKind = "operating_pulse"
	KindStoreDiagnostics ExportKind = "store_diagnostics"
	KindScenario         ExportKind = "scenario"
)

// FormulaKind describes a spreadsheet formula the web workbook builder
// derives from the descriptor — the server never evaluates it, it only
// declares it (single source of truth for both stacks).
type FormulaKind string

const (
	FormulaSum   FormulaKind = "sum"   // totals row =SUM over the column
	FormulaDelta FormulaKind = "delta" // column = A - B (structured refs)
	FormulaRatio FormulaKind = "ratio" // column = A / B with IF guard
)

// FormulaSpec declares one computed column.
type FormulaSpec struct {
	Kind   FormulaKind `json:"kind"`
	Source []string    `json:"source,omitempty"` // operand column keys
	// Scale multiplies the computed ratio (e.g. 100 turns plan/baseline
	// into an attainment percentage); the cached result uses the same
	// scaling so the workbook value equals the response value.
	Scale int `json:"scale,omitempty"`
}

// ColumnSpec is one export column; Key addresses the row cell, Header is the
// display name, Formula optionally declares the computed-column contract.
type ColumnSpec struct {
	Key     string       `json:"key"`
	Header  string       `json:"header"`
	Formula *FormulaSpec `json:"formula,omitempty"`
	Sum     bool         `json:"sum,omitempty"` // include in the totals row
}

// ExportDescriptor is the full column contract of one export kind.
type ExportDescriptor struct {
	Kind    ExportKind   `json:"kind"`
	Title   string       `json:"title"`
	Columns []ColumnSpec `json:"columns"`
}

// Envelope carries the provenance header values shared by every format.
type Envelope struct {
	Basis              string // "Working" / "Official"
	DataClassification string // production / simulated / mixed
	DatasetVersion     string
	PeriodLabel        string // "近 14 天" / "2026-07" / "2026-Q3"
	AsOf               string
	FormulaVersion     string
	SourceSystems      []string
	GeneratedAt        time.Time
}

// Descriptors returns every registered export descriptor. The web exporters
// fetch this once — the CONTRACT-001 shape: backend whitelist, frontend
// consumes, no second list.
func Descriptors() map[string]ExportDescriptor {
	return map[string]ExportDescriptor{
		string(KindOperatingPulse):   operatingPulseDescriptor(),
		string(KindStoreDiagnostics): storeDiagnosticsDescriptor(),
		string(KindScenario):         scenarioDescriptor(),
	}
}

// Descriptor returns one descriptor or an error for unknown kinds.
func Descriptor(kind ExportKind) (ExportDescriptor, error) {
	descriptor, ok := Descriptors()[string(kind)]
	if !ok {
		return ExportDescriptor{}, fmt.Errorf("unknown export kind %q", kind)
	}
	return descriptor, nil
}

func operatingPulseDescriptor() ExportDescriptor {
	return ExportDescriptor{Kind: KindOperatingPulse, Title: "经营脉搏·关注榜", Columns: []ColumnSpec{
		{Key: "rank", Header: "优先级"},
		{Key: "identity", Header: "门店/分组"},
		{Key: "brand_region", Header: "品牌 · 区域"},
		{Key: "signals", Header: "信号"},
		{Key: "score", Header: "评分"},
		{Key: "severity", Header: "严重度"},
		{Key: "revenue", Header: "营业额"},
		{Key: "revenue_change", Header: "营业额环比%"},
		{Key: "store_contribution", Header: "门店经营利润"},
		{Key: "contribution_change", Header: "经营利润环比%"},
		{Key: "source_systems", Header: "来源系统"},
		{Key: "currency", Header: "币种"},
	}}
}

func storeDiagnosticsDescriptor() ExportDescriptor {
	return ExportDescriptor{Kind: KindStoreDiagnostics, Title: "门店360·指标摘要", Columns: []ColumnSpec{
		{Key: "metric", Header: "指标"},
		{Key: "unit", Header: "单位"},
		{Key: "current", Header: "本期", Sum: true},
		{Key: "comparison", Header: "对比期", Sum: true},
		{Key: "change", Header: "变化", Formula: &FormulaSpec{Kind: FormulaDelta, Source: []string{"current", "comparison"}}},
		{Key: "status", Header: "状态"},
	}}
}

func scenarioDescriptor() ExportDescriptor {
	return ExportDescriptor{Kind: KindScenario, Title: "租金谈判测算·情景对比", Columns: []ColumnSpec{
		{Key: "metric", Header: "指标"},
		{Key: "unit", Header: "单位"},
		{Key: "baseline", Header: "Baseline", Sum: true},
		{Key: "plan", Header: "方案", Sum: true},
		{Key: "delta", Header: "差异", Formula: &FormulaSpec{Kind: FormulaDelta, Source: []string{"plan", "baseline"}}},
		{Key: "attainment", Header: "达成率%", Formula: &FormulaSpec{Kind: FormulaRatio, Source: []string{"plan", "baseline"}, Scale: 100}},
		{Key: "status", Header: "状态"},
	}}
}

// Row is one export record addressed by column key.
type Row map[string]string

// ProvenanceLines renders the shared header block (口径头) that leads every
// export file in every format.
func ProvenanceLines(envelope Envelope) []string {
	classes := envelope.DataClassification
	if classes == "" {
		classes = "unspecified"
	}
	dataset := envelope.DatasetVersion
	if dataset == "" {
		dataset = "—"
	}
	sources := strings.Join(envelope.SourceSystems, ", ")
	if sources == "" {
		sources = "—"
	}
	return []string{
		fmt.Sprintf("%s · %s", envelope.Basis, workingMark(classes)),
		fmt.Sprintf("data_classification=%s · dataset=%s · period=%s · as_of=%s", classes, dataset, envelope.PeriodLabel, envelope.AsOf),
		fmt.Sprintf("formula=%s · sources=%s · generated_at=%s", envelope.FormulaVersion, sources, envelope.GeneratedAt.Format(time.RFC3339)),
	}
}

// workingMark keeps the simulated/Working classification visible in the file
// itself, not only the name (bottom line 2).
func workingMark(classification string) string {
	if classification == "simulated" {
		return "模拟数据（simulated）— 不得作为正式口径对外"
	}
	if classification == "production" {
		return "Working（未经关账审计）"
	}
	return "mixed — 口径混合，注意区分"
}

// Filename builds the download name with the classification mark.
func Filename(kind ExportKind, classification, periodLabel, extension string) string {
	mark := "working"
	if classification == "simulated" {
		mark = "simulated"
	}
	period := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '近', '天', '/', ':':
			return '-'
		default:
			return r
		}
	}, periodLabel)
	if period == "" {
		period = "period"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", kind, mark, period, time.Now().UTC().Format("20060102T150405Z"), extension)
}

// ExportCSV renders the provenance header, the column header and the rows as
// UTF-8 CSV with a BOM (Excel-friendly) and formula-injection escaping: any
// cell starting with = + - @ is prefixed with a single quote so spreadsheet
// apps never evaluate attacker-controlled text.
func ExportCSV(descriptor ExportDescriptor, envelope Envelope, rows []Row) (string, []byte, error) {
	if _, err := Descriptor(descriptor.Kind); err != nil {
		return "", nil, err
	}
	var buffer bytes.Buffer
	buffer.WriteString("\uFEFF")
	writer := csv.NewWriter(&buffer)
	for _, line := range ProvenanceLines(envelope) {
		provenanceRow := make([]string, len(descriptor.Columns))
		provenanceRow[0] = EscapeCell(line)
		if err := writer.Write(provenanceRow); err != nil {
			return "", nil, err
		}
	}
	header := make([]string, len(descriptor.Columns))
	for i, column := range descriptor.Columns {
		header[i] = column.Header
	}
	if err := writer.Write(header); err != nil {
		return "", nil, err
	}
	for _, row := range rows {
		record := make([]string, len(descriptor.Columns))
		for i, column := range descriptor.Columns {
			record[i] = EscapeCell(row[column.Key])
		}
		if err := writer.Write(record); err != nil {
			return "", nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", nil, err
	}
	return Filename(descriptor.Kind, envelope.DataClassification, envelope.PeriodLabel, "csv"), buffer.Bytes(), nil
}

// EscapeCell neutralizes CSV/spreadsheet formula injection. The guard is
// prefix-based (= + - @ after trimming), the standard mitigation; a leading
// tab also counts.
func EscapeCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	}
	return value
}

// SortedKeys is a small helper for deterministic map iteration in callers.
func SortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
