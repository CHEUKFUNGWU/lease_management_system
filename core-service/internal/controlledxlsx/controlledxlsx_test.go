package controlledxlsx

import (
	"archive/zip"
	"bytes"
	"testing"
)

func buildXLSX(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range entries {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// GUARD-001 evidence: the shared reader replaced the monthly importer's
// hand-rolled zip/xml parser — these cases prove the replacement works on
// inline cells, shared strings, column gaps and multi-sheet packages.
func TestReadInlineCellsWithColumnGaps(t *testing.T) {
	data := buildXLSX(t, map[string]string{
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>store_code</t></is></c><c r="C1" t="inlineStr"><is><t>period</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>S001</t></is></c><c r="C2" t="inlineStr"><is><t>2026-07</t></is></c></row></sheetData></worksheet>`,
	})
	rows, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	// Missing column B stays an empty cell — the grid is rectangular.
	if rows[0][0] != "store_code" || rows[0][1] != "" || rows[0][2] != "period" {
		t.Fatalf("header=%v", rows[0])
	}
	if rows[1][0] != "S001" || rows[1][2] != "2026-07" {
		t.Fatalf("data=%v", rows[1])
	}
}

func TestReadSharedStrings(t *testing.T) {
	data := buildXLSX(t, map[string]string{
		"xl/sharedStrings.xml": `<sst><si><t>营业额</t></si><si><t>123.45</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c r="A1" t="s"><v>0</v></c></row><row><c r="A2" t="s"><v>1</v></c></row></sheetData></worksheet>`,
	})
	rows, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0][0] != "营业额" || rows[1][0] != "123.45" {
		t.Fatalf("shared strings=%v", rows)
	}
}

func TestReadPrefersFirstWorksheetAndRejectsBrokenPackages(t *testing.T) {
	data := buildXLSX(t, map[string]string{
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>first</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>second</t></is></c></row></sheetData></worksheet>`,
	})
	rows, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "first" {
		t.Fatalf("first worksheet=%v", rows)
	}
	if _, err := Read([]byte("not a zip")); err == nil {
		t.Fatal("broken package accepted")
	}
	empty := buildXLSX(t, map[string]string{})
	if _, err := Read(empty); err == nil {
		t.Fatal("worksheet-less package accepted")
	}
}
