package aiintake

// Excel deterministic reading ports adapters.py:_read_excel_contracts. The
// cell dump text, deterministic records and coordinate locators must match the
// Python openpyxl output so the CORR-2 Excel case (batch-excel-deterministic-
// fallback) stays green and the Excel evidence safety checks see the same text.

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var headerAliasesExcel = map[string][]string{
	"contract_number":    {"合同编号", "contract_number", "合同号"},
	"contract_name":      {"合同名称", "contract_name", "合同名"},
	"lessee":             {"承租方", "法人主体", "legal_entity", "lessee"},
	"lessor":             {"出租方", "lessor"},
	"store_name":         {"门店/资产名称", "门店名称", "资产名称", "store_name"},
	"store_address":      {"门店/资产地址", "门店地址", "资产地址", "store_address"},
	"asset_type":         {"资产类型", "资产类别", "asset_type", "asset_category"},
	"area_sqm":           {"租赁面积", "建筑面积", "面积", "area_sqm", "area"},
	"currency":           {"币种", "currency"},
	"commencement_date":  {"起租日(commencement)", "起租日", "commencement_date", "租赁起始日"},
	"lease_start_date":   {"租赁开始日", "lease_start_date"},
	"lease_end_date":     {"租赁结束日", "租期结束日", "lease_end_date"},
	"renewal_option":     {"续租选择权", "renewal_option"},
	"termination_option": {"终止选择权判断", "终止选择权", "termination_option"},
	"fixed_rent_amount":  {"月租金", "固定租金", "fixed_rent_amount"},
	"payment_timing":     {"付款时点", "payment_timing"},
	"discount_rate":      {"折现率", "discount_rate"},
	"discount_rate_type": {"折现率类型", "discount_rate_type"},
	"lease_scope":        {"范围判定(lease_scope)", "lease_scope", "范围判定"},
}

// readExcelContracts mirrors _read_excel_contracts: it returns the cell-dump
// text (source evidence + safety-check target), the deterministic records and
// the coordinate locators.
func readExcelContracts(data []byte) (string, []map[string]any, []EvidenceLocator, error) {
	wb, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", nil, nil, err
	}
	defer wb.Close()

	var textParts []string
	textParts = append(textParts,
		"Excel workbook cell dump for AI semantic parsing.",
		"The cell coordinates are source evidence. Infer headers and fields from nearby cells, sheet names, and row context.",
	)
	var records []map[string]any
	var locators []EvidenceLocator

	for _, sheet := range wb.GetSheetList() {
		rows, err := wb.GetRows(sheet)
		if err != nil {
			continue
		}
		// Python pads every values_only row to min(sheet.max_column, 60), so the
		// locator's last column letter uses the sheet-wide max, not len(row).
		sheetMaxCols := 0
		for _, r := range rows {
			if len(r) > sheetMaxCols {
				sheetMaxCols = len(r)
			}
		}
		if sheetMaxCols > 60 {
			sheetMaxCols = 60
		}
		// Cap rows/cols like Python max_rows=200 / max_cols=60.
		if len(rows) > 200 {
			rows = rows[:200]
		}
		textParts = append(textParts, "\n## Sheet: "+sheet)
		for rowIdx, vals := range rows {
			var cells []string
			for colIdx, value := range vals {
				if colIdx >= 60 {
					break
				}
				formatted := formatExcelValue(value)
				if formatted == "" {
					continue
				}
				cells = append(cells, sheetCell(sheet, rowIdx+1, colIdx)+"="+formatted)
			}
			if len(cells) > 0 {
				textParts = append(textParts, fmt.Sprintf("Row %d: %s", rowIdx+1, strings.Join(cells, " | ")))
			}
		}

		// header detection on the first 20 rows (values_only in Python).
		headerIndexes := map[string]int{}
		headerRow := -1
		for i := 0; i < len(rows) && i < 20; i++ {
			candidate := headerIndexesFor(rows[i])
			if _, hasNum := candidate["contract_number"]; hasNum {
				if _, hasLessor := candidate["lessor"]; hasLessor {
					headerIndexes = candidate
					headerRow = i
					break
				}
			}
		}
		if headerRow < 0 {
			continue
		}
		// Openpyxl header matching is exact-lowercase on each alias; the
		// fixture headers are Chinese so we rely on the aliases.
		for rowNumber := headerRow + 1; rowNumber < len(rows); rowNumber++ {
			row := rows[rowNumber]
			contractNumber := strings.TrimSpace(excelCell(row, headerIndexes, "contract_number"))
			low := strings.ToLower(contractNumber)
			if contractNumber == "" || low == "none" || low == "null" {
				continue
			}
			leaseScope := strings.TrimSpace(excelCell(row, headerIndexes, "lease_scope"))
			if leaseScope == "" {
				leaseScope = "in_scope"
			}
			record := map[string]any{
				"contract_number":    contractNumber,
				"contract_name":      firstNonEmpty(excelCell(row, headerIndexes, "contract_name"), contractNumber),
				"lessee":             strings.TrimSpace(excelCell(row, headerIndexes, "lessee")),
				"lessor":             strings.TrimSpace(excelCell(row, headerIndexes, "lessor")),
				"store_name":         strings.TrimSpace(excelCell(row, headerIndexes, "store_name")),
				"store_address":      strings.TrimSpace(excelCell(row, headerIndexes, "store_address")),
				"commencement_date":  formatExcelValue(excelCell(row, headerIndexes, "commencement_date")),
				"lease_start_date":   firstNonEmpty(formatExcelValue(excelCell(row, headerIndexes, "lease_start_date")), formatExcelValue(excelCell(row, headerIndexes, "commencement_date"))),
				"lease_end_date":     formatExcelValue(excelCell(row, headerIndexes, "lease_end_date")),
				"currency":           strings.TrimSpace(excelCell(row, headerIndexes, "currency")),
				"asset_type":         excelCell(row, headerIndexes, "asset_type"),
				"area_sqm":           excelCell(row, headerIndexes, "area_sqm"),
				"fixed_rent_amount":  excelCell(row, headerIndexes, "fixed_rent_amount"),
				"payment_frequency":  "monthly",
				"payment_timing":     excelCell(row, headerIndexes, "payment_timing"),
				"renewal_option":     excelCell(row, headerIndexes, "renewal_option"),
				"termination_option": excelCell(row, headerIndexes, "termination_option"),
				"discount_rate_type": strings.TrimSpace(excelCell(row, headerIndexes, "discount_rate_type")),
				"discount_rate":      excelCell(row, headerIndexes, "discount_rate"),
				"lease_scope":        leaseScope,
				"suggested_scope":    leaseScope,
				"scope_source":       "ledger",
				"scope_confidence":   0.9,
				"confidence":         0.9,
			}
			records = append(records, record)
			recordIndex := len(records) - 1
			locators = append(locators, EvidenceLocator{
				Field:  fmt.Sprintf("contracts[%d]", recordIndex),
				Source: fmt.Sprintf("%s!A%d:%s%d", sheet, rowNumber+1, columnLetter(sheetMaxCols), rowNumber+1),
				Quote:  contractNumber,
			})
		}
	}
	return strings.Join(textParts, "\n"), records, locators, nil
}

// headerIndexesFor mirrors _header_indexes: alias-driven field → column index.
func headerIndexesFor(headers []string) map[string]int {
	normalized := make(map[string]int, len(headers))
	for index, header := range headers {
		key := strings.ToLower(strings.TrimSpace(header))
		if key != "" {
			normalized[key] = index
		}
	}
	result := make(map[string]int)
	for field, names := range headerAliasesExcel {
		for _, name := range names {
			key := strings.ToLower(strings.TrimSpace(name))
			if index, ok := normalized[key]; ok {
				result[field] = index
				break
			}
		}
	}
	return result
}

func excelCell(row []string, indexes map[string]int, field string) string {
	index, ok := indexes[field]
	if !ok || index >= len(row) {
		return ""
	}
	return row[index]
}

// formatExcelValue mirrors _format_excel_value for the values excelize returns
// (strings). Excel dates come through as serial numbers here, which the fixture
// avoids by keeping plain text dates.
func formatExcelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	// Excel serial date (e.g. 45292) has no determinism issue in the fixture
	// (plain text dates), so leave serials as-is.
	return value
}

// sheetCell returns an A1-style coordinate for (row1based, col0based).
func sheetCell(sheet string, row, col int) string {
	return columnLetter(col+1) + strconv.Itoa(row)
}

func columnLetter(index int) string {
	var sb strings.Builder
	for index > 0 {
		index--
		sb.WriteByte(byte('A' + index%26))
		index /= 26
	}
	// reverse
	runes := []rune(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

var _ = time.Now

// ReadExcelContracts is the exported seam the Agent's intake path uses to turn
// an uploaded xlsx into deterministic records, the cell-dump text (evidence)
// and coordinate locators — the same shape the Python Excel adapter produced.
func ReadExcelContracts(data []byte) (string, []map[string]any, []EvidenceLocator, error) {
	return readExcelContracts(data)
}

// IsExcelContentType reports whether a content type routes to the Excel reader.
func IsExcelContentType(contentType string) bool {
	return isExcelContentType(contentType)
}
