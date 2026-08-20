package storepnl

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/xuri/excelize/v2"
)

func TestRenderXLSXLiveFormulas(t *testing.T) {
	tmpl, err := template.DefaultStorePnlTemplate()
	if err != nil {
		t.Fatal(err)
	}
	pnl, err := Project(context.Background(), tmpl, StoreRef{StoreID: "S1", AsOf: "2026-08-18", WindowDays: 7, Classification: "production"}, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisSideBySide, Readers{
		KPI: memKPI{facts: testFacts()}, Plan: memPlan{values: map[string]*float64{}},
		Lease: memLease{lease: LeaseMonthValues{ROUDepreciation: pf(55), LeaseInterest: pf(10), OtherDepreciation: pf(8)}},
		Peer:  memPeer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderXLSX(pnl)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("exported workbook must reopen: %v", err)
	}
	// 差异列与差异率列必须是活公式（不是数字落盘）。表头下有口径头行，
	// 首行数据在 headerRow+1 = 3。
	variance, err := f.GetCellFormula("operating_basis", "E3")
	if err != nil || !strings.HasPrefix(variance, "=C3-D3") {
		t.Fatalf("variance cell must be a live formula, got %q err=%v", variance, err)
	}
	pct, err := f.GetCellFormula("operating_basis", "F3")
	if err != nil || !strings.Contains(pct, "ABS(D3)") {
		t.Fatalf("pct cell must be a live formula, got %q err=%v", pct, err)
	}
	// 口径头必须带分类标识（T16：模拟标识贯穿导出）。
	header, _ := f.GetCellValue("operating_basis", "A1")
	if !strings.Contains(header, "data_classification: production") {
		t.Fatalf("口径头 must name the classification, got %q", header)
	}
	// 小计行（毛利合计）必须是 SUM 表达式。
	foundSubtotal := false
	for r := 3; r <= 31; r++ {
		cell := "C" + string(rune('0'+r/10)) + string(rune('0'+r%10))
		if r < 10 {
			cell = "C" + string(rune('0'+r))
		}
		formula, _ := f.GetCellFormula("operating_basis", cell)
		isSubtotal := strings.HasPrefix(formula, "=C") || strings.HasPrefix(formula, "=+C") || strings.HasPrefix(formula, "=-C")
		if isSubtotal && (strings.Contains(formula, "-C") || strings.Contains(formula, "+C")) {
			foundSubtotal = true
			break
		}
	}
	if !foundSubtotal {
		t.Fatal("subtotal rows must carry live formula expressions")
	}
	// 双口径分 sheet、各自 basis 标签。
	if _, err := f.GetSheetIndex("ifrs16_basis"); err != nil {
		t.Fatal("ifrs16 block must render on its own sheet")
	}
}

func TestRenderXLSXCarriesUngovernedMarker(t *testing.T) {
	pnl := &StorePnl{
		StoreID: "S1", BasisMode: BasisOperating,
		Operating: &Block{Basis: "operating_basis", Rows: []RowValue{
			{Key: "custom_ratio", Label: "其他费用率", Kind: "formula", Basis: "operating_basis", Ungoverned: true, Actual: pf(0.1)},
			{Key: "revenue", Label: "营业收入", Kind: "link", Basis: "operating_basis", Actual: pf(100)},
		}},
	}
	out, err := RenderXLSX(pnl)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	label, err := f.GetCellValue("operating_basis", "A3")
	if err != nil || !strings.Contains(label, "未经指标治理") {
		t.Fatalf("ungoverned row must carry the marker in its label cell, got %q err=%v", label, err)
	}
	plain, err := f.GetCellValue("operating_basis", "A4")
	if err != nil || strings.Contains(plain, "未经指标治理") {
		t.Fatalf("governed link row must not carry the marker, got %q err=%v", plain, err)
	}
}

// P0-7/T16: 模拟导出口径头标识——simulated projection 的 workbook 顶部必须
// 出现醒目模拟声明，production 不出现。
func TestRenderXLSXCarriesSimulatedMarker(t *testing.T) {
	production := &StorePnl{
		StoreID: "S1", BasisMode: BasisOperating, Classification: "production", DatasetVersion: "ds-1", AsOf: "2026-08-18",
		Operating: &Block{Basis: "operating_basis", Rows: []RowValue{{Key: "revenue", Label: "营收", Kind: "link", Basis: "operating_basis", Actual: pf(100)}}},
	}
	simulated := &StorePnl{
		StoreID: "S1", BasisMode: BasisOperating, Classification: "simulated", DatasetVersion: "sim-ds-1", AsOf: "2026-08-18",
		Operating: &Block{Basis: "operating_basis", Rows: []RowValue{{Key: "revenue", Label: "营收", Kind: "link", Basis: "operating_basis", Actual: pf(100)}}},
	}
	out, err := RenderXLSX(production)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(out))
	if !strings.Contains(mustCellValue(f, "operating_basis", "A1"), "data_classification: production") {
		t.Fatal("production export must name data_classification production")
	}
	if strings.Contains(mustCellValue(f, "operating_basis", "A1"), "SIMULATED") {
		t.Fatal("production export must not carry the simulated banner")
	}

	outSim, err := RenderXLSX(simulated)
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := excelize.OpenReader(bytes.NewReader(outSim))
	if !strings.Contains(mustCellValue(fs, "operating_basis", "A1"), "data_classification: simulated") {
		t.Fatal("simulated export must name data_classification simulated")
	}
	if !strings.Contains(mustCellValue(fs, "operating_basis", "A2"), "SIMULATED") {
		t.Fatal("simulated export must carry the醒目模拟声明（永不进入正式链路）")
	}
}

func mustCellValue(f *excelize.File, sheet, cell string) string {
	v, _ := f.GetCellValue(sheet, cell)
	return v
}

func TestRenderXLSXAppliesRowFormats(t *testing.T) {
	pnl := &StorePnl{
		StoreID: "S1", BasisMode: BasisOperating,
		Operating: &Block{Basis: "operating_basis", Rows: []RowValue{
			{Key: "margin", Label: "毛利率", Kind: "formula", Basis: "operating_basis",
				Actual: pf(1234567.89),
				Format: template.Format{Scale: template.ScaleTenThousand, NegStyle: template.NegParens, Bold: true, Indent: 2}},
		}},
	}
	out, err := RenderXLSX(pnl)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	styleID, err := f.GetCellStyle("operating_basis", "C3")
	if err != nil {
		t.Fatal(err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	if style.CustomNumFmt == nil || !strings.Contains(*style.CustomNumFmt, "万") || !strings.Contains(*style.CustomNumFmt, ";(") {
		t.Fatalf("scaled + parens number format must land in the workbook, got %v", style.CustomNumFmt)
	}
	if style.Font == nil || !style.Font.Bold {
		t.Fatal("bold must land in the row style")
	}
	if style.Alignment == nil || style.Alignment.Indent != 2 {
		t.Fatalf("indent must land in the row style, got %+v", style.Alignment)
	}
}
