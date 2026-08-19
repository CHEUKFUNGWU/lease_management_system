package storepnl

import (
	"github.com/xuri/excelize/v2"
)

// RenderXLSX exports a store P&L projection as a formula workbook (S1-8):
// variance/pct columns are live formulas, subtotal cells of each block are
// live SUM expressions over their child rows (with the template's sign
// convention), and every block carries the basis label — so the file's
// arithmetic, not just its numbers, is auditable.
func RenderXLSX(pnl *StorePnl) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", sheetFor(pnl.Operating.Basis)); err != nil {
		return nil, err
	}
	if err := writeBlock(f, sheetFor(pnl.Operating.Basis), pnl.Operating); err != nil {
		return nil, err
	}
	if pnl.Ifrs16 != nil {
		if _, err := f.NewSheet(sheetFor(pnl.Ifrs16.Basis)); err != nil {
			return nil, err
		}
		if err := writeBlock(f, sheetFor(pnl.Ifrs16.Basis), pnl.Ifrs16); err != nil {
			return nil, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sheetFor(basis string) string {
	if basis == "" || basis == "operating_basis" || basis == "ifrs16_basis" {
		name := basis
		if name == "" {
			name = "block"
		}
		return name
	}
	return basis
}

func writeBlock(f *excelize.File, sheet string, block *Block) error {
	if block == nil {
		block = &Block{Basis: sheet, Rows: nil}
	}
	headers := []string{"科目", "口径 basis", "Actual", "对比列", "差异额", "差异率"}
	for j, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		if err := f.SetCellStr(sheet, cell, header); err != nil {
			return err
		}
	}
	rowIdx := map[string]int{}
	for i, row := range block.Rows {
		rowIdx[row.Key] = i + 2
	}
	for i, row := range block.Rows {
		r := i + 2
		labelCell, _ := excelize.CoordinatesToCellName(1, r)
		basisCell, _ := excelize.CoordinatesToCellName(2, r)
		_ = f.SetCellStr(sheet, labelCell, row.Label)
		_ = f.SetCellStr(sheet, basisCell, block.Basis)

		actualCell, _ := excelize.CoordinatesToCellName(3, r)
		otherCell, _ := excelize.CoordinatesToCellName(4, r)
		varCell, _ := excelize.CoordinatesToCellName(5, r)
		pctCell, _ := excelize.CoordinatesToCellName(6, r)

		if row.Kind == "subtotal" && len(row.Children) > 0 {
			_ = f.SetCellFormula(sheet, actualCell, subtotalFormula(row, 3, rowIdx))
			_ = f.SetCellFormula(sheet, otherCell, subtotalFormula(row, 4, rowIdx))
		} else {
			writeNumeric(f, sheet, actualCell, row.Actual)
			writeNumeric(f, sheet, otherCell, row.Other)
		}
		_ = f.SetCellFormula(sheet, varCell, formulaCol(3, 4, r))
		_ = f.SetCellFormula(sheet, pctCell, pctFormula(3, 4, r))
	}
	return nil
}

func writeNumeric(f *excelize.File, sheet, cell string, value *float64) {
	if value == nil {
		_ = f.SetCellStr(sheet, cell, "—")
		return
	}
	_ = f.SetCellValue(sheet, cell, *value)
}

func formulaCol(a, b, r int) string {
	ca, _ := excelize.CoordinatesToCellName(a, r)
	cb, _ := excelize.CoordinatesToCellName(b, r)
	return "=" + ca + "-" + cb
}

func pctFormula(a, b, r int) string {
	ca, _ := excelize.CoordinatesToCellName(a, r)
	cb, _ := excelize.CoordinatesToCellName(b, r)
	return `=IF(` + cb + `=0,"",(` + ca + `-` + cb + `)/ABS(` + cb + `))`
}

func subtotalFormula(row RowValue, col int, rowIdx map[string]int) string {
	expr := ""
	for _, child := range row.Children {
		idx, ok := rowIdx[child]
		if !ok {
			continue
		}
		cell, _ := excelize.CoordinatesToCellName(col, idx)
		sign := "+"
		for _, sub := range row.Subtracted {
			if sub == child {
				sign = "-"
			}
		}
		if expr == "" && sign == "-" {
			expr = "-" + cell
		} else {
			expr += sign + cell
		}
	}
	if expr == "" {
		return ""
	}
	return "=" + expr
}
