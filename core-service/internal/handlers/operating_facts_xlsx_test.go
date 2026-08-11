package handlers

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestReadControlledXLSXSupportsInlineCells(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte(`<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>store_id</t></is></c><c r="B1" t="inlineStr"><is><t>period</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>store-1</t></is></c><c r="B2" t="inlineStr"><is><t>2026-08</t></is></c></row></sheetData></worksheet>`))
	_ = archive.Close()
	rows, err := readControlledXLSX(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "store-1" || rows[1][1] != "2026-08" {
		t.Fatalf("rows=%v", rows)
	}
}
