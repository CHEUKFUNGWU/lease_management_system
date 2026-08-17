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
		"xl/sharedStrings.xml":     `<sst><si><t>营业额</t></si><si><t>123.45</t></si></sst>`,
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

// The previous implementation picked whichever worksheet Go's map iteration
// happened to yield first, so this test failed roughly three runs in five and
// the importer could read the wrong sheet in production. Tab order — not file
// name, and certainly not map order — decides which sheet is first.
func TestReadFollowsWorkbookTabOrderNotFileName(t *testing.T) {
	data := buildXLSX(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Facts" sheetId="7" r:id="rId9"/><sheet name="Notes" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId9" Target="worksheets/sheet3.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>notes</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet3.xml":   `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>facts</t></is></c></row></sheetData></worksheet>`,
	})
	for attempt := 0; attempt < 20; attempt++ {
		rows, err := Read(data)
		if err != nil {
			t.Fatal(err)
		}
		if rows[0][0] != "facts" {
			t.Fatalf("attempt %d read the wrong tab: %v", attempt, rows)
		}
	}
}

// Without a workbook part the fallback sorts numerically, so sheet2 wins over
// sheet10 — a plain string sort would put "sheet10.xml" first.
func TestReadFallbackSortsSheetNumbersNumerically(t *testing.T) {
	data := buildXLSX(t, map[string]string{
		"xl/worksheets/sheet10.xml": `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>ten</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml":  `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>two</t></is></c></row></sheetData></worksheet>`,
	})
	for attempt := 0; attempt < 20; attempt++ {
		rows, err := Read(data)
		if err != nil {
			t.Fatal(err)
		}
		if rows[0][0] != "two" {
			t.Fatalf("attempt %d: %v", attempt, rows)
		}
	}
}
