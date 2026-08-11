package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/fpna"
	"github.com/lease-management-system/core-service/internal/services/operating"
)

var operatingPeriodPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type OperatingFactsHandler struct {
	repo        *repository.OperatingFactsRepository
	auditLogger *audit.Logger
	governance  *repository.FPnAGovernanceRepository
}

func NewOperatingFactsHandler(repo *repository.OperatingFactsRepository, auditLogger *audit.Logger, governance ...*repository.FPnAGovernanceRepository) *OperatingFactsHandler {
	var gov *repository.FPnAGovernanceRepository
	if len(governance) > 0 {
		gov = governance[0]
	}
	return &OperatingFactsHandler{repo: repo, auditLogger: auditLogger, governance: gov}
}

type storeOperatingFactInput struct {
	StoreID               string   `json:"store_id"`
	StoreExternalID       string   `json:"store_external_id"`
	StoreAlias            string   `json:"store_alias"`
	ExternalSystem        string   `json:"external_system"`
	Period                string   `json:"period" binding:"required"`
	PeriodBasis           string   `json:"period_basis" binding:"required"`
	Currency              string   `json:"currency" binding:"required"`
	Revenue               *float64 `json:"revenue"`
	GrossProfit           *float64 `json:"gross_profit"`
	Transactions          *float64 `json:"transactions"`
	Footfall              *float64 `json:"footfall"`
	AreaSqm               *float64 `json:"area_sqm"`
	LaborCost             *float64 `json:"labor_cost"`
	FixedRent             *float64 `json:"fixed_rent"`
	VariableRent          *float64 `json:"variable_rent"`
	NonLeaseCost          *float64 `json:"non_lease_cost"`
	OtherControllableCost *float64 `json:"other_controllable_cost"`
	SourceSystem          string   `json:"source_system" binding:"required"`
	SourceRecordID        string   `json:"source_record_id"`
	AsOfAt                string   `json:"as_of_at"`
	Version               int      `json:"version"`
	ReconciliationStatus  string   `json:"reconciliation_status"`
	MappingStatus         string   `json:"mapping_status"`
	BusinessSegment       string   `json:"business_segment"`
	FiscalYear            string   `json:"fiscal_year"`
	StoreAgeMonths        *int     `json:"store_age_months"`
	CohortCode            string   `json:"cohort_code"`
	DataQualityStatus     string   `json:"data_quality_status"`
	Note                  *string  `json:"note"`
}

type storeOperatingFactRequest struct {
	Items        []storeOperatingFactInput `json:"items" binding:"required,min=1"`
	SourceFile   string                    `json:"source_file"`
	SourceSystem string                    `json:"source_system"`
}

func (h *OperatingFactsHandler) UpsertStores(c *gin.Context) {
	var req storeOperatingFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.upsertStores(c, req)
}

func (h *OperatingFactsHandler) upsertStores(c *gin.Context, req storeOperatingFactRequest) {
	legalEntityID := middleware.GetTenantID(c)
	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)
	var entity *string
	if legalEntityID != "" {
		entity = &legalEntityID
	}
	sourceSystem := defaultString(req.SourceSystem, "api")
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	batch := &repository.OperatingFactBatch{LegalEntityID: entity, SourceSystem: sourceSystem, SourceFile: req.SourceFile, TotalRows: len(req.Items), AsOfAt: nowUTC(), CreatedBy: optionalString(userID), ReconciliationStatus: "unreconciled", IdempotencyKey: idempotencyKey, FactVersion: defaultString(c.GetHeader("X-Fact-Version"), nowUTC().Format(time.RFC3339))}
	if _, err := h.repo.CreateBatch(c.Request.Context(), batch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if idempotencyKey != "" && (batch.Status == "completed" || batch.Status == "failed") && batch.AcceptedRows+batch.RejectedRows > 0 {
		c.JSON(http.StatusOK, gin.H{"batch": batch, "saved_count": batch.AcceptedRows, "failed_count": batch.RejectedRows, "failures": batch.ErrorSummary, "data": []any{}, "idempotent_replay": true})
		return
	}
	saved := make([]*repository.StoreOperatingFact, 0, len(req.Items))
	failures := make([]gin.H, 0)
	allRowsReady := true
	for i, item := range req.Items {
		if strings.TrimSpace(item.StoreID) == "" && h.governance != nil {
			key := defaultString(item.StoreExternalID, item.StoreAlias)
			mapping, resolveErr := h.governance.ResolveMapping(c.Request.Context(), legalEntityID, "store", item.ExternalSystem, key, strings.TrimSpace(item.Period)+"-01")
			if resolveErr != nil {
				failures = append(failures, gin.H{"index": i, "error": resolveErr.Error()})
				h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "ambiguous_mapping", resolveErr.Error(), i)
				continue
			}
			if mapping != nil && mapping.TargetID != nil {
				item.StoreID = *mapping.TargetID
			} else if mapping != nil && mapping.TargetCode != "" {
				item.StoreID, _ = h.repo.ResolveStoreIDByCode(c.Request.Context(), legalEntityID, mapping.TargetCode)
			}
		}
		if _, parseErr := uuid.Parse(strings.TrimSpace(item.StoreID)); parseErr != nil {
			failures = append(failures, gin.H{"index": i, "error": "store_id or governed store mapping is required"})
			h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "unmapped", "store_id or governed store mapping is required", i)
			continue
		}
		if !operatingPeriodPattern.MatchString(strings.TrimSpace(item.Period)) {
			failures = append(failures, gin.H{"index": i, "error": "period must be YYYY-MM"})
			h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "invalid", "period must be YYYY-MM", i)
			continue
		}
		if item.Revenue == nil {
			failures = append(failures, gin.H{"index": i, "error": "revenue is required; use 0 explicitly for a confirmed zero"})
			h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "missing", "revenue is missing; zero must be explicit", i)
			continue
		}
		if *item.Revenue < 0 {
			failures = append(failures, gin.H{"index": i, "error": "revenue cannot be negative"})
			h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "invalid", "revenue cannot be negative", i)
			continue
		}
		fact := &repository.StoreOperatingFact{StoreID: item.StoreID, Period: strings.TrimSpace(item.Period), PeriodBasis: strings.TrimSpace(item.PeriodBasis), Currency: strings.TrimSpace(item.Currency), Revenue: *item.Revenue, GrossProfit: item.GrossProfit, Transactions: item.Transactions, Footfall: item.Footfall, AreaSqm: item.AreaSqm, LaborCost: item.LaborCost, FixedRent: item.FixedRent, VariableRent: item.VariableRent, NonLeaseCost: item.NonLeaseCost, OtherControllableCost: item.OtherControllableCost, SourceSystem: defaultString(item.SourceSystem, sourceSystem), SourceRecordID: item.SourceRecordID, Version: item.Version, ReconciliationStatus: item.ReconciliationStatus, MappingStatus: item.MappingStatus, BusinessSegment: item.BusinessSegment, FiscalYear: item.FiscalYear, StoreAgeMonths: item.StoreAgeMonths, CohortCode: item.CohortCode, DataQualityStatus: item.DataQualityStatus, Note: item.Note, CreatedBy: optionalString(userID)}
		if item.AsOfAt != "" {
			if parsed, err := parseRFC3339(item.AsOfAt); err == nil {
				fact.AsOfAt = parsed
			} else {
				failures = append(failures, gin.H{"index": i, "error": "as_of_at must be RFC3339"})
				h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, "invalid", "as_of_at must be RFC3339", i)
				continue
			}
		}
		batchID := batch.ID
		fact.ImportBatchID = &batchID
		result, err := h.repo.UpsertStore(c.Request.Context(), fact)
		if err != nil {
			failures = append(failures, gin.H{"index": i, "error": err.Error()})
			category := "invalid"
			if strings.Contains(strings.ToLower(err.Error()), "store not found") {
				category = "unmapped"
			}
			h.recordDataQuality(c, batch, item.SourceRecordID, item.Period, category, err.Error(), i)
			continue
		}
		saved = append(saved, result)
		if result.ReconciliationStatus != "matched" || result.MappingStatus != "mapped" {
			allRowsReady = false
			category := "reconciliation"
			if result.MappingStatus != "mapped" {
				category = "unmapped"
			}
			h.recordDataQuality(c, batch, result.SourceRecordID, result.Period, category, "operating fact requires review before decision use", i)
		}
		if h.auditLogger != nil {
			h.auditLogger.Log(c.Request.Context(), "store_operating_facts", result.ID, "upsert", nil, result, userID, c)
		}
	}
	batchStatus := "completed"
	reconciliationStatus := "matched"
	if len(failures) > 0 || !allRowsReady {
		reconciliationStatus = "warning"
	}
	if len(saved) == 0 {
		batchStatus = "failed"
		reconciliationStatus = "failed"
	}
	errorSummary, _ := json.Marshal(failures)
	if finalized, finalizeErr := h.repo.FinalizeBatch(c.Request.Context(), batch.ID, len(saved), len(failures), batchStatus, reconciliationStatus, errorSummary); finalizeErr == nil {
		batch = finalized
	}
	status := http.StatusOK
	if len(saved) == 0 {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"batch": batch, "saved_count": len(saved), "failed_count": len(failures), "failures": failures, "data": saved})
}

func (h *OperatingFactsHandler) recordDataQuality(c *gin.Context, batch *repository.OperatingFactBatch, sourceRecordID, period, category, description string, rowIndex int) {
	if h.governance == nil {
		return
	}
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	batchID := batch.ID
	_, _ = h.governance.CreateDataQuality(c.Request.Context(), &repository.FPnADataQualityItem{LegalEntityID: entity, BatchID: &batchID, Period: period, Dimension: "store", Category: category, Severity: "medium", SourceTable: "store_operating_facts", SourceRecordID: defaultString(sourceRecordID, fmt.Sprintf("row-%d", rowIndex)), DataVersion: batch.ID, Description: description, Evidence: json.RawMessage(fmt.Sprintf(`{"row_index":%d}`, rowIndex)), CreatedBy: optionalString(userIDFromContext(c))})
}

// ImportStoresCSV is the controlled-template path for customers that are not
// yet connected to POS/BI APIs. It uses the same row isolation and batch
// finalisation as the JSON endpoint.
func (h *OperatingFactsHandler) ImportStoresCSV(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart file field 'file' is required"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded CSV"})
		return
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV header is required"})
		return
	}
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = index
	}
	get := func(row []string, name string) string {
		index, ok := columns[name]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}
	items := make([]storeOperatingFactInput, 0)
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CSV row cannot be read", "detail": readErr.Error()})
			return
		}
		item := storeOperatingFactInput{StoreID: get(row, "store_id"), StoreExternalID: get(row, "store_external_id"), StoreAlias: get(row, "store_alias"), ExternalSystem: get(row, "external_system"), Period: get(row, "period"), PeriodBasis: get(row, "period_basis"), Currency: get(row, "currency"), SourceSystem: get(row, "source_system"), SourceRecordID: get(row, "source_record_id"), ReconciliationStatus: get(row, "reconciliation_status"), MappingStatus: get(row, "mapping_status"), BusinessSegment: get(row, "business_segment"), FiscalYear: get(row, "fiscal_year"), CohortCode: get(row, "cohort_code"), DataQualityStatus: get(row, "data_quality_status")}
		if value := get(row, "store_age_months"); value != "" {
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
				item.StoreAgeMonths = &parsed
			}
		}
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
		items = append(items, item)
	}
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV contains no data rows"})
		return
	}
	h.upsertStores(c, storeOperatingFactRequest{Items: items, SourceFile: fileHeader.Filename, SourceSystem: c.PostForm("source_system")})
}

func (h *OperatingFactsHandler) StoreCSVTemplate(c *gin.Context) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="store-operating-facts-template.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"store_id", "store_external_id", "store_alias", "external_system", "period", "period_basis", "currency", "revenue", "gross_profit", "transactions", "footfall", "area_sqm", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost", "business_segment", "fiscal_year", "store_age_months", "cohort_code", "data_quality_status", "source_system", "source_record_id", "reconciliation_status", "mapping_status"})
	w.Flush()
}

func parseCSVFloat(value string) *float64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func (h *OperatingFactsHandler) ListStores(c *gin.Context) {
	rows, err := h.repo.ListStores(c.Request.Context(), middleware.GetTenantID(c), c.Query("period"), c.Query("store_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

func (h *OperatingFactsHandler) ListBatches(c *gin.Context) {
	rows, err := h.repo.ListBatches(c.Request.Context(), middleware.GetTenantID(c), c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

func (h *OperatingFactsHandler) StorePerformance(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	rows, err := h.repo.ListStores(c.Request.Context(), middleware.GetTenantID(c), period, c.Query("store_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	latest := make(map[string]*repository.StoreOperatingFact)
	for _, row := range rows {
		if current := latest[row.StoreID]; current == nil || row.Version > current.Version {
			latest[row.StoreID] = row
		}
	}
	result := make([]operating.FourWall, 0, len(latest))
	for _, row := range latest {
		result = append(result, operating.CalculateFourWall(*row))
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "basis": "Working", "data": result, "total": len(result), "source": gin.H{"type": "store_operating_facts", "as_of": "versioned", "coverage": "store-period"}})
}

func (h *OperatingFactsHandler) StoreBenchmarks(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	rows, err := h.repo.ListStores(c.Request.Context(), middleware.GetTenantID(c), period, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	exchangeRateVersion, ok := requiredExchangeRateVersion(c, storeCurrencies(rows))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "basis": "Working", "data": operating.BenchmarkStores(rows), "peer_definition": "region+brand", "exchange_rate_version": exchangeRateVersion, "source": gin.H{"type": "store_operating_facts", "coverage": "permission_scoped"}})
}

func (h *OperatingFactsHandler) StoreCohorts(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	rows, err := h.repo.ListStores(c.Request.Context(), middleware.GetTenantID(c), period, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	exchangeRateVersion, ok := requiredExchangeRateVersion(c, storeCurrencies(rows))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "basis": "Working", "data": operating.SummarizeStoreCohorts(rows), "exchange_rate_version": exchangeRateVersion, "source": gin.H{"type": "store_operating_facts", "coverage": "permission_scoped"}})
}

func storeCurrencies(rows []*repository.StoreOperatingFact) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil && strings.TrimSpace(row.Currency) != "" {
			values = append(values, strings.ToUpper(strings.TrimSpace(row.Currency)))
		}
	}
	return values
}

func requiredExchangeRateVersion(c *gin.Context, currencies []string) (string, bool) {
	seen := map[string]struct{}{}
	for _, currency := range currencies {
		seen[currency] = struct{}{}
	}
	version := strings.TrimSpace(c.Query("exchange_rate_version"))
	if len(seen) > 1 && version == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "mixed currencies require exchange_rate_version", "review_required": true})
		return "", false
	}
	return version, true
}

func (h *OperatingFactsHandler) StorePromotionROI(c *gin.Context) {
	var input operating.PromotionROIInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.BaselineSales < 0 || input.PromotedSales < 0 || input.GrossMarginPct < 0 || input.GrossMarginPct > 100 || input.TurnoverRentPct < 0 || input.TurnoverRentPct > 100 || input.PromotionCost < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "promotion ROI assumptions are outside allowed ranges"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Scenario", "data": operating.EvaluatePromotionROI(input), "side_effects": false, "review_required": true})
}

type equipmentAssetInput struct {
	LegalEntityID      string   `json:"legal_entity_id"`
	PlantCode          string   `json:"plant_code" binding:"required"`
	ProductionLineCode string   `json:"production_line_code"`
	EquipmentCode      string   `json:"equipment_code" binding:"required"`
	EquipmentName      string   `json:"equipment_name" binding:"required"`
	CostCenter         string   `json:"cost_center"`
	AssetIdentifier    string   `json:"asset_identifier"`
	ContractID         *string  `json:"contract_id"`
	AssetType          string   `json:"asset_type"`
	Capacity           *float64 `json:"capacity"`
	CapacityUnit       string   `json:"capacity_unit"`
	Currency           string   `json:"currency"`
	ExternalSystem     string   `json:"external_system"`
	ExternalID         string   `json:"external_id"`
	EffectiveFrom      *string  `json:"effective_from"`
	EffectiveTo        *string  `json:"effective_to"`
	Active             *bool    `json:"active"`
}

func (h *OperatingFactsHandler) UpsertEquipment(c *gin.Context) {
	var input equipmentAssetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	} else if strings.TrimSpace(input.LegalEntityID) != "" {
		// Global administrators may choose the target entity; ordinary users
		// never reach this branch because TenantMiddleware supplies a tenant.
		entity = optionalString(strings.TrimSpace(input.LegalEntityID))
	}
	if entity == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal_entity_id is required for global equipment imports"})
		return
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	asset := &repository.EquipmentAsset{LegalEntityID: entity, PlantCode: strings.TrimSpace(input.PlantCode), ProductionLineCode: strings.TrimSpace(input.ProductionLineCode), EquipmentCode: strings.TrimSpace(input.EquipmentCode), EquipmentName: strings.TrimSpace(input.EquipmentName), CostCenter: input.CostCenter, AssetIdentifier: input.AssetIdentifier, ContractID: input.ContractID, AssetType: input.AssetType, Capacity: input.Capacity, CapacityUnit: input.CapacityUnit, Currency: input.Currency, ExternalSystem: input.ExternalSystem, ExternalID: input.ExternalID, EffectiveFrom: input.EffectiveFrom, EffectiveTo: input.EffectiveTo, Active: active}
	result, err := h.repo.UpsertEquipment(c.Request.Context(), asset)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
func (h *OperatingFactsHandler) ListEquipment(c *gin.Context) {
	rows, err := h.repo.ListEquipment(c.Request.Context(), middleware.GetTenantID(c), c.Query("plant"), c.Query("line"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type equipmentFactInput struct {
	EquipmentID           string   `json:"equipment_id" binding:"required,uuid"`
	Period                string   `json:"period" binding:"required"`
	Currency              string   `json:"currency" binding:"required"`
	OutputQty             *float64 `json:"output_qty"`
	YieldPct              *float64 `json:"yield_pct"`
	ScrapQty              *float64 `json:"scrap_qty"`
	DowntimeHours         *float64 `json:"downtime_hours"`
	OEEPct                *float64 `json:"oee_pct"`
	UtilizationPct        *float64 `json:"utilization_pct"`
	LaborCost             *float64 `json:"labor_cost"`
	EnergyCost            *float64 `json:"energy_cost"`
	MaintenanceCost       *float64 `json:"maintenance_cost"`
	StandardCost          *float64 `json:"standard_cost"`
	ActualCost            *float64 `json:"actual_cost"`
	MaterialUsageCost     *float64 `json:"material_usage_cost"`
	OverheadAbsorption    *float64 `json:"overhead_absorption"`
	PurchasePrice         *float64 `json:"purchase_price"`
	PurchasePriceVariance *float64 `json:"purchase_price_variance"`
	CapacityAvailable     *float64 `json:"capacity_available"`
	LeaseCost             *float64 `json:"lease_cost"`
	ContractualRent       *float64 `json:"contractual_rent"`
	DataQualityStatus     string   `json:"data_quality_status"`
	SourceSystem          string   `json:"source_system" binding:"required"`
	SourceRecordID        string   `json:"source_record_id"`
	Version               int      `json:"version"`
	ReconciliationStatus  string   `json:"reconciliation_status"`
}

func (h *OperatingFactsHandler) UpsertEquipmentFact(c *gin.Context) {
	var input equipmentFactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !operatingPeriodPattern.MatchString(input.Period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be YYYY-MM"})
		return
	}
	f := &repository.EquipmentOperatingFact{EquipmentID: input.EquipmentID, Period: input.Period, Currency: input.Currency, OutputQty: input.OutputQty, YieldPct: input.YieldPct, ScrapQty: input.ScrapQty, DowntimeHours: input.DowntimeHours, OEEPct: input.OEEPct, UtilizationPct: input.UtilizationPct, LaborCost: input.LaborCost, EnergyCost: input.EnergyCost, MaintenanceCost: input.MaintenanceCost, StandardCost: input.StandardCost, ActualCost: input.ActualCost, MaterialUsageCost: input.MaterialUsageCost, OverheadAbsorption: input.OverheadAbsorption, PurchasePrice: input.PurchasePrice, PurchasePriceVariance: input.PurchasePriceVariance, CapacityAvailable: input.CapacityAvailable, LeaseCost: input.LeaseCost, ContractualRent: input.ContractualRent, DataQualityStatus: input.DataQualityStatus, SourceSystem: input.SourceSystem, SourceRecordID: input.SourceRecordID, Version: input.Version, ReconciliationStatus: input.ReconciliationStatus, CreatedBy: optionalString(userIDFromContext(c))}
	result, err := h.repo.UpsertEquipmentFact(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
func (h *OperatingFactsHandler) EquipmentPerformance(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	rows, err := h.repo.ListEquipmentFacts(c.Request.Context(), middleware.GetTenantID(c), period, c.Query("plant"), c.Query("line"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		Fact            *repository.EquipmentOperatingFact `json:"fact"`
		Bridge          *operating.CostBridge              `json:"bridge,omitempty"`
		Missing         []string                           `json:"missing,omitempty"`
		UnderAbsorption *float64                           `json:"fixed_lease_under_absorption,omitempty"`
		RiskFlags       []string                           `json:"risk_flags,omitempty"`
	}
	result := make([]item, 0, len(rows))
	for _, row := range rows {
		v := item{Fact: row}
		bridge, err := operating.CalculateCostBridge(*row)
		if err == nil {
			v.Bridge = &bridge
		} else {
			v.Missing = []string{"standard_cost", "actual_cost"}
		}
		if row.CapacityAvailable != nil && *row.CapacityAvailable > 0 && row.Capacity != nil && *row.Capacity > 0 && row.LeaseCost != nil {
			under := (*row.LeaseCost) * (1 - (*row.CapacityAvailable / *row.Capacity))
			if under > 0 {
				value := math.Round(under*100) / 100
				v.UnderAbsorption = &value
				v.RiskFlags = append(v.RiskFlags, "fixed_lease_cost_under_absorption")
			}
		}
		if row.UtilizationPct != nil && *row.UtilizationPct < 50 {
			v.RiskFlags = append(v.RiskFlags, "low_utilization")
		}
		if row.OutputQty == nil || row.YieldPct == nil {
			v.RiskFlags = append(v.RiskFlags, "operating_evidence_incomplete")
		}
		result = append(result, v)
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "basis": "Working", "data": result, "total": len(result)})
}

func (h *OperatingFactsHandler) EquipmentCandidates(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	assets, err := h.repo.ListEquipment(c.Request.Context(), middleware.GetTenantID(c), c.Query("plant"), c.Query("line"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	facts, err := h.repo.ListEquipmentFacts(c.Request.Context(), middleware.GetTenantID(c), period, c.Query("plant"), c.Query("line"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	factByEquipment := make(map[string]*repository.EquipmentOperatingFact, len(facts))
	for _, fact := range facts {
		factByEquipment[fact.EquipmentID] = fact
	}
	type candidate struct {
		Asset          *repository.EquipmentAsset         `json:"asset"`
		Fact           *repository.EquipmentOperatingFact `json:"fact,omitempty"`
		Reasons        []string                           `json:"reasons"`
		ReviewRequired bool                               `json:"review_required"`
	}
	result := make([]candidate, 0)
	now := nowUTC()
	within := 90
	if parsed, parseErr := strconv.Atoi(c.Query("within_days")); parseErr == nil && parsed > 0 {
		within = parsed
	}
	for _, asset := range assets {
		fact := factByEquipment[asset.ID]
		reasons := make([]string, 0)
		if fact == nil {
			reasons = append(reasons, "missing_operating_fact")
		} else {
			if fact.UtilizationPct != nil && *fact.UtilizationPct < 50 {
				reasons = append(reasons, "low_utilization")
			}
			if fact.CapacityAvailable != nil && fact.Capacity != nil && *fact.Capacity > 0 && *fact.CapacityAvailable < *fact.Capacity*0.5 {
				reasons = append(reasons, "capacity_under_absorption")
			}
			if fact.UtilizationPct == nil || fact.Capacity == nil {
				reasons = append(reasons, "insufficient_capacity_evidence")
			}
		}
		if asset.EffectiveTo != nil {
			if expiry, parseErr := time.Parse("2006-01-02", *asset.EffectiveTo); parseErr == nil && expiry.After(now) && expiry.Before(now.AddDate(0, 0, within)) {
				reasons = append(reasons, "near_expiry")
			}
		}
		if len(reasons) > 0 {
			result = append(result, candidate{Asset: asset, Fact: fact, Reasons: reasons, ReviewRequired: true})
		}
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "basis": "Working", "data": result, "total": len(result), "review_required": true, "recommendation": "human_review_required"})
}

func (h *OperatingFactsHandler) Overview(c *gin.Context) {
	period := c.Query("period")
	if period != "" && !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be YYYY-MM"})
		return
	}
	result, err := h.repo.Overview(c.Request.Context(), middleware.GetTenantID(c), period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *OperatingFactsHandler) ManagementBrief(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	cadence := defaultString(c.Query("cadence"), "mbr")
	if cadence != "wbr" && cadence != "mbr" && cadence != "qbr" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cadence must be wbr, mbr or qbr"})
		return
	}
	ctx := c.Request.Context()
	legal := middleware.GetTenantID(c)
	overview, err := h.repo.Overview(ctx, legal, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actions, err := h.repo.ListActions(ctx, legal, period, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stores, err := h.repo.ListStores(ctx, legal, period, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	equipment, err := h.repo.ListEquipmentFacts(ctx, legal, period, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	deadlineWindow := 90
	if cadence == "wbr" {
		deadlineWindow = 30
	} else if cadence == "qbr" {
		deadlineWindow = 180
	}
	leaseDeadlines, err := h.repo.ListCriticalDateBrief(ctx, legal, period, deadlineWindow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(actions) > 20 {
		actions = actions[:20]
	}
	for _, action := range actions {
		rank := fpna.RankAction(*action, nowUTC())
		action.PriorityScore = rank.Score
		action.PriorityReasons = rank.Reasons
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].PriorityScore > actions[j].PriorityScore })
	storeMetrics := make([]operating.FourWall, 0, len(stores))
	dataGaps := make([]map[string]any, 0)
	for _, row := range stores {
		metric := operating.CalculateFourWall(*row)
		storeMetrics = append(storeMetrics, metric)
		if len(metric.DataGaps) > 0 {
			dataGaps = append(dataGaps, map[string]any{"store_id": metric.StoreID, "store_code": metric.StoreCode, "gaps": metric.DataGaps})
		}
	}
	overdue := make([]*repository.FPnAActionItem, 0)
	varianceBridges := make([]map[string]any, 0)
	cashRisks := make([]map[string]any, 0)
	now := nowUTC()
	for _, action := range actions {
		if action.DueDate != nil && action.DueDate.Before(now) && action.Status != "completed" {
			overdue = append(overdue, action)
		}
		category := strings.ToLower(action.Category + " " + action.RuleCode)
		if strings.Contains(category, "variance") || strings.Contains(category, "bridge") || strings.Contains(category, "差异") {
			varianceBridges = append(varianceBridges, map[string]any{"action_id": action.ID, "title": action.Title, "impact_amount": action.ImpactAmount, "currency": action.Currency, "rule_code": action.RuleCode, "source_record_id": action.SourceRecordID, "status": action.Status})
		}
		if strings.Contains(category, "cash") || strings.Contains(category, "liquidity") || strings.Contains(category, "payment") || strings.Contains(category, "现金") {
			cashRisks = append(cashRisks, map[string]any{"action_id": action.ID, "title": action.Title, "impact_amount": action.ImpactAmount, "currency": action.Currency, "rule_code": action.RuleCode, "status": action.Status})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"report_type": cadence, "period": period, "basis": "Working", "generated_at": nowUTC(),
		"source":   gin.H{"coverage": "permission_scoped", "data_version": "versioned_operating_facts", "official": false},
		"overview": overview, "top_actions": actions, "overdue_actions": overdue, "retail_four_wall": storeMetrics, "manufacturing_equipment": equipment,
		"variance_bridges": varianceBridges, "cash_risks": cashRisks, "lease_deadlines": leaseDeadlines, "data_gaps": dataGaps,
		"no_material_change": len(actions) == 0 && len(dataGaps) == 0 && len(leaseDeadlines) == 0,
		"narrative_status":   "AI narrative requires human confirmation",
	})
}

type actionInput struct {
	Period             string         `json:"period"`
	Category           string         `json:"category" binding:"required"`
	Severity           string         `json:"severity"`
	Status             string         `json:"status"`
	Title              string         `json:"title" binding:"required"`
	Description        string         `json:"description"`
	RuleCode           string         `json:"rule_code" binding:"required"`
	SourceTable        string         `json:"source_table" binding:"required"`
	SourceRecordID     string         `json:"source_record_id" binding:"required"`
	DataVersion        string         `json:"data_version"`
	ImpactAmount       *float64       `json:"impact_amount"`
	Currency           string         `json:"currency"`
	OwnerName          string         `json:"owner_name"`
	DueDate            *string        `json:"due_date"`
	BaselineAmount     *float64       `json:"baseline_amount"`
	TargetAmount       *float64       `json:"target_amount"`
	ExpectedBenefit    *float64       `json:"expected_benefit"`
	VerificationPeriod string         `json:"verification_period"`
	VerifiedAmount     *float64       `json:"verified_amount"`
	HumanRootCause     string         `json:"human_root_cause"`
	PlannedAction      string         `json:"planned_action"`
	AISuggestion       string         `json:"ai_suggestion"`
	Evidence           map[string]any `json:"evidence"`
}

func (h *OperatingFactsHandler) ListActions(c *gin.Context) {
	rows, err := h.repo.ListActions(c.Request.Context(), middleware.GetTenantID(c), c.Query("period"), c.Query("status"), c.Query("category"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().UTC()
	for _, row := range rows {
		rank := fpna.RankAction(*row, now)
		row.PriorityScore = rank.Score
		row.PriorityReasons = rank.Reasons
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].PriorityScore > rows[j].PriorityScore })
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows), "ranking": "impact_control_risk_deadline_recurrence_fixability_verification"})
}
func (h *OperatingFactsHandler) CreateAction(c *gin.Context) {
	var input actionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Period != "" && !operatingPeriodPattern.MatchString(input.Period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be YYYY-MM"})
		return
	}
	if input.Status != "" && !isActionCreateStatus(input.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new action status must be open, acknowledged, in_progress or completed"})
		return
	}
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	uid := userIDFromContext(c)
	evidence, _ := jsonMarshal(input.Evidence)
	verificationStatus := ""
	if input.Status == "completed" {
		verificationStatus = "pending"
	}
	item := &repository.FPnAActionItem{LegalEntityID: entity, Period: input.Period, Category: input.Category, Severity: defaultString(input.Severity, "medium"), Status: defaultString(input.Status, "open"), VerificationStatus: verificationStatus, Title: input.Title, Description: input.Description, RuleCode: input.RuleCode, SourceTable: input.SourceTable, SourceRecordID: input.SourceRecordID, DataVersion: input.DataVersion, IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")), ImpactAmount: input.ImpactAmount, Currency: input.Currency, OwnerName: input.OwnerName, DueDate: parseDatePointer(input.DueDate), BaselineAmount: input.BaselineAmount, TargetAmount: input.TargetAmount, ExpectedBenefit: input.ExpectedBenefit, VerificationPeriod: input.VerificationPeriod, HumanRootCause: input.HumanRootCause, PlannedAction: input.PlannedAction, AISuggestion: input.AISuggestion, Evidence: evidence, CreatedBy: optionalString(uid), UpdatedBy: optionalString(uid)}
	result, err := h.repo.CreateAction(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil && !result.Replayed {
		h.auditLogger.Log(c.Request.Context(), "fpna_action_items", result.ID, "create", nil, result, uid, c)
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "idempotent_replay": result.Replayed})
}
func (h *OperatingFactsHandler) UpdateAction(c *gin.Context) {
	var input actionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status == "verified" && (input.VerificationPeriod == "" || input.VerifiedAmount == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verified action requires verification_period and verified_amount"})
		return
	}
	if input.Status != "" && !isActionUpdateStatus(input.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action status is invalid"})
		return
	}
	if input.VerificationPeriod != "" && !operatingPeriodPattern.MatchString(input.VerificationPeriod) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification_period must be YYYY-MM"})
		return
	}
	verificationStatus := ""
	if input.Status == "verified" {
		verificationStatus = "verified"
	} else if input.Status == "completed" {
		verificationStatus = "pending"
	}
	result, err := h.repo.UpdateAction(c.Request.Context(), c.Param("id"), middleware.GetTenantID(c), userIDFromContext(c), repository.FPnAActionItem{Status: input.Status, OwnerName: input.OwnerName, DueDate: parseDatePointer(input.DueDate), HumanRootCause: input.HumanRootCause, PlannedAction: input.PlannedAction, ExpectedBenefit: input.ExpectedBenefit, VerificationPeriod: input.VerificationPeriod, VerifiedAmount: input.VerifiedAmount, VerificationStatus: verificationStatus})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_action_items", result.ID, "transition", nil, result, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

type bulkActionInput struct {
	IDs       []string `json:"ids" binding:"required,min=1"`
	Status    string   `json:"status"`
	OwnerName string   `json:"owner_name"`
	DueDate   *string  `json:"due_date"`
}

func (h *OperatingFactsHandler) BulkUpdateActions(c *gin.Context) {
	var input bulkActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status != "" && !isActionUpdateStatus(input.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action status is invalid"})
		return
	}
	if input.Status == "verified" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bulk verification requires action-specific actual amount and period"})
		return
	}
	updated := make([]*repository.FPnAActionItem, 0, len(input.IDs))
	for _, id := range input.IDs {
		item, err := h.repo.UpdateAction(c.Request.Context(), id, middleware.GetTenantID(c), userIDFromContext(c), repository.FPnAActionItem{Status: input.Status, OwnerName: input.OwnerName, DueDate: parseDatePointer(input.DueDate), VerificationStatus: func() string {
			if input.Status == "completed" {
				return "pending"
			}
			return ""
		}()})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "updated_count": len(updated)})
			return
		}
		if item != nil {
			updated = append(updated, item)
			if h.auditLogger != nil {
				h.auditLogger.Log(c.Request.Context(), "fpna_action_items", item.ID, "bulk_transition", nil, item, userIDFromContext(c), c)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": updated, "updated_count": len(updated)})
}

func (h *OperatingFactsHandler) ExportActions(c *gin.Context) {
	rows, err := h.repo.ListActions(c.Request.Context(), middleware.GetTenantID(c), c.Query("period"), c.Query("status"), c.Query("category"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="fpna-actions-working.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"basis", "id", "period", "category", "severity", "status", "title", "impact_amount", "currency", "owner_name", "due_date", "verification_status", "source_table", "source_record_id"})
	for _, row := range rows {
		due := ""
		if row.DueDate != nil {
			due = row.DueDate.Format("2006-01-02")
		}
		impact := ""
		if row.ImpactAmount != nil {
			impact = fmt.Sprintf("%.2f", *row.ImpactAmount)
		}
		_ = w.Write([]string{"Working", row.ID, row.Period, row.Category, row.Severity, row.Status, row.Title, impact, row.Currency, row.OwnerName, due, row.VerificationStatus, row.SourceTable, row.SourceRecordID})
	}
	w.Flush()
}

func isActionCreateStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "open", "acknowledged", "in_progress", "completed":
		return true
	default:
		return false
	}
}

func isActionUpdateStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "open", "acknowledged", "in_progress", "completed", "verified", "accepted", "dismissed":
		return true
	default:
		return false
	}
}

type assumptionInput struct {
	AssumptionKey string         `json:"assumption_key" binding:"required"`
	Category      string         `json:"category" binding:"required"`
	Value         map[string]any `json:"value" binding:"required"`
	Unit          string         `json:"unit"`
	Source        string         `json:"source" binding:"required"`
	OwnerName     string         `json:"owner_name"`
	EffectiveFrom string         `json:"effective_from" binding:"required"`
	EffectiveTo   string         `json:"effective_to"`
	Version       int            `json:"version"`
}

func (h *OperatingFactsHandler) ListAssumptions(c *gin.Context) {
	rows, err := h.repo.ListAssumptions(c.Request.Context(), middleware.GetTenantID(c), c.Query("key"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

func (h *OperatingFactsHandler) CreateAssumption(c *gin.Context) {
	var input assumptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	from, err := time.Parse("2006-01-02", input.EffectiveFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "effective_from must be YYYY-MM-DD"})
		return
	}
	var to *time.Time
	if strings.TrimSpace(input.EffectiveTo) != "" {
		parsed, parseErr := time.Parse("2006-01-02", input.EffectiveTo)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "effective_to must be YYYY-MM-DD"})
			return
		}
		to = &parsed
	}
	value, marshalErr := json.Marshal(input.Value)
	if marshalErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value must be JSON"})
		return
	}
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	uid := userIDFromContext(c)
	result, createErr := h.repo.CreateAssumption(c.Request.Context(), &repository.FPnAAssumptionVersion{LegalEntityID: entity, AssumptionKey: input.AssumptionKey, Category: input.Category, Value: value, Unit: input.Unit, Source: input.Source, OwnerName: input.OwnerName, EffectiveFrom: from, EffectiveTo: to, Version: input.Version, CreatedBy: optionalString(uid)})
	if createErr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": createErr.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_assumption_versions", result.ID, "create", nil, result, uid, c)
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "review_required": true})
}

func optionalString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}
func userIDFromContext(c *gin.Context) string { v, _ := c.Get("user_id"); s, _ := v.(string); return s }
func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}
func nowUTC() time.Time                        { return time.Now().UTC() }
func parseRFC3339(v string) (time.Time, error) { return time.Parse(time.RFC3339, v) }
func parseDatePointer(v *string) *time.Time {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*v))
	if err != nil {
		return nil
	}
	return &parsed
}
func jsonMarshal(v map[string]any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage(`{}`), nil
	}
	data, err := json.Marshal(v)
	return json.RawMessage(data), err
}
