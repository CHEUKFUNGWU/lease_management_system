package controlledintake

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func buildXLSX(t *testing.T, sheetXML string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	sheet, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.Write([]byte(sheetXML)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestParseCSVTrimsDropsEmptyRowsAndSplitsHeader(t *testing.T) {
	headers, rows, err := Parse(Source{Filename: "pos.csv", Data: []byte("门店编号, 日期 ,营业额\nS001,2026-07-01,100\n\nS001,2026-07-02,101\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 3 || headers[1] != "日期" || len(rows) != 2 || rows[1][2] != "101" {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
}

func TestParseXLSXViaSharedReader(t *testing.T) {
	data := buildXLSX(t, `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>门店编号</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>S001</t></is></c></row></sheetData></worksheet>`)
	headers, rows, err := Parse(Source{Filename: "budget.xlsx", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if headers[0] != "门店编号" || rows[0][0] != "S001" {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
}

func TestParseRejectsOversizeEmptyAndBroken(t *testing.T) {
	if _, _, err := Parse(Source{Filename: "big.csv", Data: make([]byte, MaxSourceSize+1)}); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversize err=%v", err)
	}
	if _, _, err := Parse(Source{Filename: "e.csv", Data: []byte("")}); err == nil {
		t.Fatal("empty template accepted")
	}
	if _, _, err := Parse(Source{Filename: "h.csv", Data: []byte("a,b\n")}); err == nil {
		t.Fatal("header-only template accepted")
	}
	if _, _, err := Parse(Source{Filename: "b.xlsx", Data: []byte("not a zip")}); err == nil {
		t.Fatal("broken xlsx accepted")
	}
}

func TestReporterKeepsRowOrder(t *testing.T) {
	reporter := NewReporter()
	reporter.Add(3, "bad_number", "revenue must be numeric")
	reporter.Add(1, "unmatched_store", "store missing")
	errors := reporter.Errors()
	if len(errors) != 2 || errors[0].Row != 3 || errors[1].Row != 1 {
		t.Fatalf("errors=%+v", errors)
	}
}

func TestCompensatorRunsRollbackOnceAndSeals(t *testing.T) {
	calls := 0
	compensator := NewCompensator(func() error { calls++; return nil })
	compensator.Success()
	if err := compensator.Fail(); err != nil || calls != 0 {
		t.Fatalf("sealed compensator rolled back: calls=%d err=%v", calls, err)
	}
	compensator = NewCompensator(func() error { calls++; return errors.New("boom") })
	if err := compensator.Fail(); err == nil || calls != 1 {
		t.Fatalf("fail err=%v calls=%d", err, calls)
	}
	if err := compensator.Fail(); err != nil || calls != 1 {
		t.Fatalf("second fail ran again: calls=%d err=%v", calls, err)
	}
	var nilCompensator *Compensator
	if err := nilCompensator.Fail(); err != nil {
		t.Fatalf("nil compensator err=%v", err)
	}
}
