package retailexport

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func sampleEnvelope() Envelope {
	return Envelope{
		Basis: "Working", DataClassification: "simulated", DatasetVersion: "planA-v1",
		PeriodLabel: "近 14 天", AsOf: "2026-08-16", FormulaVersion: "retail-kpi-v1",
		SourceSystems: []string{"retail_simulator"}, GeneratedAt: time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC),
	}
}

func readCSV(t *testing.T, content []byte) [][]string {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(content), "\uFEFF")))
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestDescriptorRegistryCoversTheThreeRetailKinds(t *testing.T) {
	all := Descriptors()
	for _, kind := range []ExportKind{KindOperatingPulse, KindStoreDiagnostics, KindScenario} {
		if _, err := Descriptor(kind); err != nil {
			t.Fatalf("kind %q: %v", kind, err)
		}
		if len(all[string(kind)].Columns) == 0 {
			t.Fatalf("kind %q has no columns", kind)
		}
	}
	if _, err := Descriptor(ExportKind("unknown")); err == nil {
		t.Fatal("unknown kind accepted")
	}
	// Formula declarations exist for the computed columns the workbook side
	// derives — delta on diagnostics and scenario.
	diagnostics, _ := Descriptor(KindStoreDiagnostics)
	scenario, _ := Descriptor(KindScenario)
	if diagnostics.Columns[4].Formula == nil || diagnostics.Columns[4].Formula.Kind != FormulaDelta {
		t.Fatalf("diagnostics change formula missing: %+v", diagnostics.Columns[4])
	}
	if scenario.Columns[4].Formula == nil || scenario.Columns[4].Formula.Kind != FormulaDelta {
		t.Fatalf("scenario delta formula missing: %+v", scenario.Columns[4])
	}
}

func TestExportCSVCarriesProvenanceHeaderAndRows(t *testing.T) {
	descriptor, _ := Descriptor(KindStoreDiagnostics)
	filename, content, err := ExportCSV(descriptor, sampleEnvelope(), []Row{
		{"metric": "营业额", "unit": "CNY", "current": "120", "comparison": "80", "change": "", "status": "complete"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filename, "store_diagnostics_simulated_") || !strings.HasSuffix(filename, ".csv") {
		t.Fatalf("filename=%q", filename)
	}
	rows := readCSV(t, content)
	if len(rows) != 5 { // 3 provenance lines + header + 1 data row
		t.Fatalf("row count=%d rows=%v", len(rows), rows)
	}
	if !strings.Contains(rows[0][0], "模拟数据（simulated）") || !strings.Contains(rows[1][0], "period=近 14 天") {
		t.Fatalf("provenance=%v", rows[:2])
	}
	if rows[3][0] != "指标" || rows[4][0] != "营业额" || rows[4][2] != "120" {
		t.Fatalf("body=%v", rows[3:])
	}
	if !strings.HasPrefix(string(content), "\uFEFF") {
		t.Fatal("BOM missing")
	}
}

func TestExportCSVEscapesFormulaInjection(t *testing.T) {
	descriptor, _ := Descriptor(KindStoreDiagnostics)
	_, content, err := ExportCSV(descriptor, sampleEnvelope(), []Row{
		{"metric": "=SUM(A1:A9)", "unit": "+CMD('malicious')", "current": "@redirect", "comparison": "-1+1", "change": "", "status": "normal"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := readCSV(t, content)
	data := rows[len(rows)-1]
	for _, cell := range data[:4] {
		if !strings.HasPrefix(cell, "'") {
			t.Fatalf("unescaped injection cell %q in %v", cell, data)
		}
	}
	if data[5] != "normal" {
		t.Fatalf("normal cell must not be escaped: %v", data)
	}
}

func TestWorkingMarkDistinguishesClassifications(t *testing.T) {
	if !strings.Contains(workingMark("simulated"), "模拟") {
		t.Fatalf("simulated mark=%q", workingMark("simulated"))
	}
	if !strings.Contains(workingMark("production"), "Working") {
		t.Fatalf("production mark=%q", workingMark("production"))
	}
	if !strings.Contains(workingMark("mixed"), "mixed") {
		t.Fatalf("mixed mark=%q", workingMark("mixed"))
	}
}
