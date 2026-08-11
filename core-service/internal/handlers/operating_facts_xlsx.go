package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ImportStoresXLSX accepts the controlled store-facts workbook contract. It
// intentionally supports the common inline/shared-string cells emitted by
// Excel and keeps all row validation, idempotency and draft/Working controls
// in the same path as CSV imports.
func (h *OperatingFactsHandler) ImportStoresXLSX(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "XLSX cannot be opened"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "XLSX cannot be read"})
		return
	}
	rows, err := readControlledXLSX(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(rows) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "XLSX contains no data rows"})
		return
	}
	header := make(map[string]int)
	for index, value := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(value))] = index
	}
	get := func(row []string, key string) string {
		index, ok := header[key]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}
	items := make([]storeOperatingFactInput, 0, len(rows)-1)
	for _, row := range rows[1:] {
		item := storeOperatingFactInput{StoreID: get(row, "store_id"), StoreExternalID: get(row, "store_external_id"), StoreAlias: get(row, "store_alias"), ExternalSystem: get(row, "external_system"), Period: get(row, "period"), PeriodBasis: get(row, "period_basis"), Currency: get(row, "currency"), SourceSystem: get(row, "source_system"), SourceRecordID: get(row, "source_record_id"), ReconciliationStatus: get(row, "reconciliation_status"), MappingStatus: get(row, "mapping_status"), BusinessSegment: get(row, "business_segment"), FiscalYear: get(row, "fiscal_year"), CohortCode: get(row, "cohort_code"), DataQualityStatus: get(row, "data_quality_status")}
		item.Revenue = parseCSVFloat(get(row, "revenue"))
		item.GrossProfit = parseCSVFloat(get(row, "gross_profit"))
		item.Transactions = parseCSVFloat(get(row, "transactions"))
		item.Footfall = parseCSVFloat(get(row, "footfall"))
		item.AreaSqm = parseCSVFloat(get(row, "area_sqm"))
		item.LaborCost = parseCSVFloat(get(row, "labor_cost"))
		item.FixedRent = parseCSVFloat(get(row, "fixed_rent"))
		item.VariableRent = parseCSVFloat(get(row, "variable_rent"))
		item.NonLeaseCost = parseCSVFloat(get(row, "non_lease_cost"))
		item.OtherControllableCost = parseCSVFloat(get(row, "other_controllable_cost"))
		if value := get(row, "store_age_months"); value != "" {
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
				item.StoreAgeMonths = &parsed
			}
		}
		items = append(items, item)
	}
	h.upsertStores(c, storeOperatingFactRequest{Items: items, SourceFile: fileHeader.Filename, SourceSystem: c.PostForm("source_system")})
}

type xlsxCell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Value  string `xml:"v"`
	Inline string `xml:"is>t"`
}
type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}
type xlsxSheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}
type xlsxSharedString struct {
	Text []string `xml:"t"`
}
type xlsxSharedStrings struct {
	Items []xlsxSharedString `xml:"si"`
}

func readControlledXLSX(data []byte) ([][]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid XLSX package: %w", err)
	}
	files := make(map[string][]byte)
	for _, file := range archive.File {
		reader, openErr := file.Open()
		if openErr != nil {
			return nil, openErr
		}
		content, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		files[file.Name] = content
	}
	shared := xlsxSharedStrings{}
	if raw, ok := files["xl/sharedStrings.xml"]; ok {
		if err := xml.Unmarshal(raw, &shared); err != nil {
			return nil, fmt.Errorf("invalid shared strings: %w", err)
		}
	}
	var sheetRaw []byte
	for name, content := range files {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			sheetRaw = content
			break
		}
	}
	if len(sheetRaw) == 0 {
		return nil, fmt.Errorf("XLSX worksheet is missing")
	}
	sheet := xlsxSheet{}
	if err := xml.Unmarshal(sheetRaw, &sheet); err != nil {
		return nil, fmt.Errorf("invalid worksheet: %w", err)
	}
	rows := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := make([]string, 0)
		for _, cell := range row.Cells {
			index := xlsxColumnIndex(cell.Ref)
			for len(values) <= index {
				values = append(values, "")
			}
			value := cell.Inline
			if value == "" {
				value = cell.Value
				if cell.Type == "s" {
					if parsed, parseErr := strconv.Atoi(value); parseErr == nil && parsed >= 0 && parsed < len(shared.Items) {
						value = strings.Join(shared.Items[parsed].Text, "")
					}
				}
			}
			values[index] = value
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func xlsxColumnIndex(reference string) int {
	letters := strings.TrimRight(reference, "0123456789")
	index := 0
	for _, char := range strings.ToUpper(letters) {
		if char >= 'A' && char <= 'Z' {
			index = index*26 + int(char-'A'+1)
		}
	}
	if index == 0 {
		return 0
	}
	return index - 1
}
