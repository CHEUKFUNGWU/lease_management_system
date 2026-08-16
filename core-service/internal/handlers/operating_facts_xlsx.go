package handlers

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/controlledxlsx"
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

// readControlledXLSX delegates to the shared controlled-template reader
// (internal/controlledxlsx) so the monthly and store-day importers parse the
// identical workbook contract.
func readControlledXLSX(data []byte) ([][]string, error) {
	return controlledxlsx.Read(data)
}
