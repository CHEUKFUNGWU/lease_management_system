package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/fpna"
	"github.com/lease-management-system/core-service/internal/services/operating"
)

type FPnAGovernanceHandler struct {
	repo          *repository.FPnAGovernanceRepository
	operatingRepo *repository.OperatingFactsRepository
	auditLogger   *audit.Logger
}

func NewFPnAGovernanceHandler(repo *repository.FPnAGovernanceRepository, operatingRepo *repository.OperatingFactsRepository, auditLogger *audit.Logger) *FPnAGovernanceHandler {
	return &FPnAGovernanceHandler{repo: repo, operatingRepo: operatingRepo, auditLogger: auditLogger}
}

type planVersionInput struct {
	Name                    string          `json:"name" binding:"required"`
	VersionType             string          `json:"version_type" binding:"required"`
	ScenarioType            string          `json:"scenario_type"`
	Source                  string          `json:"source" binding:"required"`
	CoverageScope           map[string]any  `json:"coverage_scope"`
	Currency                string          `json:"currency"`
	AsOfPeriod              string          `json:"as_of_period" binding:"required"`
	FromPeriod              string          `json:"from_period" binding:"required"`
	ToPeriod                string          `json:"to_period" binding:"required"`
	ActualCutoffPeriod      string          `json:"actual_cutoff_period"`
	PriorVersionID          *string         `json:"prior_version_id"`
	AssumptionVersion       string          `json:"assumption_version"`
	ExchangeRateVersion     string          `json:"exchange_rate_version"`
	MetricDefinitionVersion string          `json:"metric_definition_version"`
	Lines                   []planLineInput `json:"lines"`
}

type planLineInput struct {
	Period             string         `json:"period" binding:"required"`
	Grain              string         `json:"grain"`
	BusinessSegment    string         `json:"business_segment"`
	Brand              string         `json:"brand"`
	Region             string         `json:"region"`
	StoreID            *string        `json:"store_id"`
	PlantCode          string         `json:"plant_code"`
	ProductionLineCode string         `json:"production_line_code"`
	EquipmentID        *string        `json:"equipment_id"`
	AssetType          string         `json:"asset_type"`
	Currency           string         `json:"currency" binding:"required"`
	Revenue            *float64       `json:"revenue"`
	GrossProfit        *float64       `json:"gross_profit"`
	LaborCost          *float64       `json:"labor_cost"`
	FixedRent          *float64       `json:"fixed_rent"`
	VariableRent       *float64       `json:"variable_rent"`
	NonLeaseCost       *float64       `json:"non_lease_cost"`
	FourWallEBITDA     *float64       `json:"four_wall_ebitda"`
	CashFlow           *float64       `json:"cash_flow"`
	NetDebt            *float64       `json:"net_debt"`
	OperationalKPIs    map[string]any `json:"operational_kpis"`
	SourceSystem       string         `json:"source_system" binding:"required"`
	SourceRecordID     string         `json:"source_record_id"`
	AsOfAt             string         `json:"as_of_at"`
	ActualFlag         bool           `json:"actual_flag"`
	ForecastFlag       bool           `json:"forecast_flag"`
	ScenarioInputs     map[string]any `json:"scenario_inputs"`
}

func (h *FPnAGovernanceHandler) CreatePlanVersion(c *gin.Context) {
	var input planVersionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePeriodRange(input.AsOfPeriod, input.FromPeriod, input.ToPeriod); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.VersionType != "actual" && input.VersionType != "prior_year" && input.VersionType != "budget" && input.VersionType != "forecast" && input.VersionType != "scenario" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version_type must be actual, prior_year, budget, forecast or scenario"})
		return
	}
	if input.ScenarioType == "" {
		input.ScenarioType = "baseline"
	}
	if input.ScenarioType != "baseline" && input.ScenarioType != "upside" && input.ScenarioType != "downside" && input.ScenarioType != "custom" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scenario_type is invalid"})
		return
	}
	coverage, _ := json.Marshal(input.CoverageScope)
	if len(coverage) == 0 {
		coverage = json.RawMessage(`{}`)
	}
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	version, err := h.repo.CreatePlanVersion(c.Request.Context(), &repository.FPnAPlanVersion{LegalEntityID: entity, Name: strings.TrimSpace(input.Name), VersionType: input.VersionType, ScenarioType: input.ScenarioType, Source: strings.TrimSpace(input.Source), CoverageScope: coverage, Currency: input.Currency, AsOfPeriod: input.AsOfPeriod, FromPeriod: input.FromPeriod, ToPeriod: input.ToPeriod, ActualCutoffPeriod: input.ActualCutoffPeriod, PriorVersionID: input.PriorVersionID, AssumptionVersion: input.AssumptionVersion, ExchangeRateVersion: input.ExchangeRateVersion, MetricDefinitionVersion: input.MetricDefinitionVersion, CreatedBy: optionalString(userIDFromContext(c))})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	createdLines := 0
	for _, line := range input.Lines {
		kpis, _ := json.Marshal(line.OperationalKPIs)
		scenario, _ := json.Marshal(line.ScenarioInputs)
		asOf := nowUTC()
		if line.AsOfAt != "" {
			if parsed, e := time.Parse(time.RFC3339, line.AsOfAt); e == nil {
				asOf = parsed
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "line as_of_at must be RFC3339"})
				return
			}
		}
		_, lineErr := h.repo.CreatePlanLine(c.Request.Context(), &repository.FPnAPlanLine{PlanVersionID: version.ID, Period: line.Period, Grain: line.Grain, LegalEntityID: entity, BusinessSegment: line.BusinessSegment, Brand: line.Brand, Region: line.Region, StoreID: line.StoreID, PlantCode: line.PlantCode, ProductionLineCode: line.ProductionLineCode, EquipmentID: line.EquipmentID, AssetType: line.AssetType, Currency: line.Currency, Revenue: line.Revenue, GrossProfit: line.GrossProfit, LaborCost: line.LaborCost, FixedRent: line.FixedRent, VariableRent: line.VariableRent, NonLeaseCost: line.NonLeaseCost, FourWallEBITDA: line.FourWallEBITDA, CashFlow: line.CashFlow, NetDebt: line.NetDebt, OperationalKPIs: kpis, SourceSystem: line.SourceSystem, SourceRecordID: line.SourceRecordID, AsOfAt: asOf, ActualFlag: line.ActualFlag, ForecastFlag: line.ForecastFlag, ScenarioInputs: scenario})
		if lineErr != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": lineErr.Error(), "version": version, "created_lines": createdLines})
			return
		}
		createdLines++
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_plan_versions", version.ID, "create", nil, version, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": version, "created_lines": createdLines, "basis": strings.Title(input.VersionType), "frozen": false})
}

func (h *FPnAGovernanceHandler) ListPlanVersions(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListPlanVersions(c.Request.Context(), entity, c.Query("version_type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

func (h *FPnAGovernanceHandler) FreezePlanVersion(c *gin.Context) {
	official := strings.EqualFold(c.Query("official"), "true")
	if official && !hasGovernanceApprovalRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only approver or admin may freeze an Official plan version"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	item, err := h.repo.FreezePlanVersion(c.Request.Context(), c.Param("id"), entity, userIDFromContext(c), official)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "plan version is missing or already frozen"})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_plan_versions", item.ID, "freeze", nil, item, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "immutable": true, "basis": func() string {
		if official {
			return "Official"
		}
		return "Working"
	}()})
}

func (h *FPnAGovernanceHandler) ComparePlanVersions(c *gin.Context) {
	leftID, rightID, period := c.Query("left_id"), c.Query("right_id"), c.Query("period")
	if leftID == "" || rightID == "" || period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "left_id, right_id and period are required"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	leftVersion, err := h.repo.GetPlanVersion(c.Request.Context(), leftID, entity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rightVersion, err := h.repo.GetPlanVersion(c.Request.Context(), rightID, entity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if leftVersion == nil || rightVersion == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan version not found"})
		return
	}
	filters := planLineFilters(c)
	left, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), leftID, entity, period, c.Query("grain"), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	right, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), rightID, entity, period, c.Query("grain"), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	currencies := map[string]struct{}{}
	for _, line := range append(left, right...) {
		if strings.TrimSpace(line.Currency) != "" {
			currencies[strings.ToUpper(line.Currency)] = struct{}{}
		}
	}
	exchangeRateVersion := strings.TrimSpace(c.Query("exchange_rate_version"))
	if exchangeRateVersion == "" {
		exchangeRateVersion = firstNonEmpty(leftVersion.ExchangeRateVersion, rightVersion.ExchangeRateVersion)
	}
	if len(currencies) > 1 && exchangeRateVersion == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "mixed currencies require exchange_rate_version", "review_required": true})
		return
	}
	leftBasis := firstNonEmpty(c.Query("left_basis"), strings.Title(leftVersion.VersionType))
	rightBasis := firstNonEmpty(c.Query("right_basis"), strings.Title(rightVersion.VersionType))
	dataVersion := firstNonEmpty(c.Query("data_version"), fmt.Sprintf("plan:%s@%s|plan:%s@%s", leftID, leftVersion.AsOfPeriod, rightID, rightVersion.AsOfPeriod))
	currency := firstNonEmpty(c.Query("currency"), leftVersion.Currency, rightVersion.Currency)
	result := fpna.ComparePlanLines(period, leftBasis, rightBasis, currency, dataVersion, left, right, 0.01)
	result.Source = fmt.Sprintf("fpna_plan_lines:%s,%s", leftID, rightID)
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "exchange_rate_version": exchangeRateVersion, "coverage": result.Coverage, "source": gin.H{"left_version": leftID, "right_version": rightID, "left_as_of": leftVersion.AsOfPeriod, "right_as_of": rightVersion.AsOfPeriod, "data_version": dataVersion, "exchange_rate_version": exchangeRateVersion}, "result": result})
}

func (h *FPnAGovernanceHandler) ForecastAccuracy(c *gin.Context) {
	forecastID, actualID := c.Query("forecast_id"), c.Query("actual_id")
	if forecastID == "" || actualID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "forecast_id and actual_id are required"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	forecast, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), forecastID, entity, c.Query("period"), c.Query("grain"), planLineFilters(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actual, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), actualID, entity, c.Query("period"), c.Query("grain"), planLineFilters(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "forecast_accuracy": fpna.ForecastAccuracy(forecast, actual), "source": gin.H{"forecast_version": forecastID, "actual_version": actualID}})
}

func (h *FPnAGovernanceHandler) HybridForecast(c *gin.Context) {
	var req struct {
		ForecastID         string `json:"forecast_id" binding:"required"`
		ActualID           string `json:"actual_id" binding:"required"`
		ActualCutoffPeriod string `json:"actual_cutoff_period" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	forecast, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), req.ForecastID, entity, "", "", planLineFilters(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actual, err := h.repo.ListPlanLinesFiltered(c.Request.Context(), req.ActualID, entity, "", "", planLineFilters(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result, err := fpna.HybridForecast(forecast, actual, req.ActualCutoffPeriod)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "actual_cutoff_period": req.ActualCutoffPeriod, "data": result, "immutable": true})
}

type mappingInput struct {
	MappingType    string         `json:"mapping_type" binding:"required"`
	ExternalSystem string         `json:"external_system" binding:"required"`
	ExternalID     string         `json:"external_id" binding:"required"`
	ExternalName   string         `json:"external_name"`
	Alias          string         `json:"alias"`
	TargetID       *string        `json:"target_id"`
	TargetCode     string         `json:"target_code"`
	EffectiveFrom  string         `json:"effective_from" binding:"required"`
	EffectiveTo    string         `json:"effective_to"`
	Status         string         `json:"status"`
	Evidence       map[string]any `json:"evidence"`
}

func (h *FPnAGovernanceHandler) CreateMapping(c *gin.Context) {
	var input mappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status == "approved" && !hasGovernanceApprovalRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only approver or admin may approve a master-data mapping"})
		return
	}
	if input.Status != "" && input.Status != "draft" && input.Status != "approved" && input.Status != "retired" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mapping status must be draft, approved or retired"})
		return
	}
	from, err := time.Parse("2006-01-02", input.EffectiveFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "effective_from must be YYYY-MM-DD"})
		return
	}
	var to *time.Time
	if input.EffectiveTo != "" {
		v, e := time.Parse("2006-01-02", input.EffectiveTo)
		if e != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "effective_to must be YYYY-MM-DD"})
			return
		}
		to = &v
	}
	evidence, _ := json.Marshal(input.Evidence)
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	item, err := h.repo.CreateMapping(c.Request.Context(), &repository.FPnAMasterDataMapping{LegalEntityID: entity, MappingType: input.MappingType, ExternalSystem: input.ExternalSystem, ExternalID: input.ExternalID, ExternalName: input.ExternalName, Alias: input.Alias, TargetID: input.TargetID, TargetCode: input.TargetCode, EffectiveFrom: from, EffectiveTo: to, Status: input.Status, Evidence: evidence, CreatedBy: optionalString(userIDFromContext(c))})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "review_required": item.Status != "approved"})
}
func (h *FPnAGovernanceHandler) ListMappings(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListMappings(c.Request.Context(), entity, c.Query("mapping_type"), c.Query("effective_date"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type metricDefinitionInput struct {
	MetricKey      string         `json:"metric_key" binding:"required"`
	Version        string         `json:"version" binding:"required"`
	DisplayName    string         `json:"display_name" binding:"required"`
	Formula        string         `json:"formula" binding:"required"`
	Grain          string         `json:"grain" binding:"required"`
	CurrencyPolicy string         `json:"currency_policy" binding:"required"`
	FiscalRule     string         `json:"fiscal_period_rule" binding:"required"`
	Exclusions     map[string]any `json:"exclusions"`
	OwnerName      string         `json:"owner_name" binding:"required"`
	EffectiveFrom  string         `json:"effective_from" binding:"required"`
	EffectiveTo    string         `json:"effective_to"`
	Status         string         `json:"status"`
}

func (h *FPnAGovernanceHandler) ListMetricDefinitions(c *gin.Context) {
	rows, err := h.repo.ListMetricDefinitions(c.Request.Context(), c.Query("metric_key"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows), "versioned": true})
}

func (h *FPnAGovernanceHandler) CreateMetricDefinition(c *gin.Context) {
	var input metricDefinitionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status == "approved" && !hasGovernanceApprovalRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only approver or admin may approve a metric definition"})
		return
	}
	if input.Status != "" && input.Status != "draft" && input.Status != "approved" && input.Status != "retired" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric status must be draft, approved or retired"})
		return
	}
	from, err := time.Parse("2006-01-02", input.EffectiveFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "effective_from must be YYYY-MM-DD"})
		return
	}
	var to *time.Time
	if input.EffectiveTo != "" {
		parsed, parseErr := time.Parse("2006-01-02", input.EffectiveTo)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "effective_to must be YYYY-MM-DD"})
			return
		}
		to = &parsed
	}
	exclusions, _ := json.Marshal(input.Exclusions)
	item, err := h.repo.CreateMetricDefinition(c.Request.Context(), &repository.FPnAMetricDefinition{
		MetricKey: input.MetricKey, Version: input.Version, DisplayName: input.DisplayName,
		Formula: input.Formula, Grain: input.Grain, CurrencyPolicy: input.CurrencyPolicy,
		FiscalRule: input.FiscalRule, Exclusions: exclusions, OwnerName: input.OwnerName,
		EffectiveFrom: from, EffectiveTo: to, Status: input.Status,
		CreatedBy: optionalString(userIDFromContext(c)),
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "immutable": true})
}

func (h *FPnAGovernanceHandler) ListAgentSignals(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListAgentSignals(c.Request.Context(), entity, c.Query("period"), c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows), "signal_only": true, "does_not_change_control": true})
}

func (h *FPnAGovernanceHandler) CreateAgentSignal(c *gin.Context) {
	var input struct {
		Period         string         `json:"period"`
		RuleCode       string         `json:"rule_code"`
		Severity       string         `json:"severity"`
		SourceTable    string         `json:"source_table"`
		SourceRecordID string         `json:"source_record_id"`
		DataVersion    string         `json:"data_version"`
		Signal         map[string]any `json:"signal"`
		Status         string         `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(input.RuleCode) == "" || strings.TrimSpace(input.SourceTable) == "" || strings.TrimSpace(input.SourceRecordID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_code, source_table and source_record_id are required"})
		return
	}
	if input.Status != "" && input.Status != "open" && input.Status != "acknowledged" && input.Status != "dismissed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "signal status must be open, acknowledged or dismissed"})
		return
	}
	signal, _ := json.Marshal(input.Signal)
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	item, err := h.repo.CreateAgentSignal(c.Request.Context(), &repository.FPnAAgentSignal{LegalEntityID: entity, Period: input.Period, RuleCode: input.RuleCode, Severity: input.Severity, SourceTable: input.SourceTable, SourceRecordID: input.SourceRecordID, DataVersion: input.DataVersion, Signal: signal, Status: input.Status})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "signal_only": true, "formal_control_changed": false})
}

type dataQualityInput struct {
	BatchID        *string        `json:"batch_id"`
	Period         string         `json:"period"`
	Dimension      string         `json:"dimension" binding:"required"`
	Category       string         `json:"category" binding:"required"`
	Severity       string         `json:"severity"`
	SourceTable    string         `json:"source_table" binding:"required"`
	SourceRecordID string         `json:"source_record_id" binding:"required"`
	DataVersion    string         `json:"data_version"`
	Description    string         `json:"description" binding:"required"`
	Evidence       map[string]any `json:"evidence"`
}

func (h *FPnAGovernanceHandler) CreateDataQuality(c *gin.Context) {
	var input dataQualityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	evidence, _ := json.Marshal(input.Evidence)
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	item, err := h.repo.CreateDataQuality(c.Request.Context(), &repository.FPnADataQualityItem{LegalEntityID: entity, BatchID: input.BatchID, Period: input.Period, Dimension: input.Dimension, Category: input.Category, Severity: input.Severity, SourceTable: input.SourceTable, SourceRecordID: input.SourceRecordID, DataVersion: input.DataVersion, Description: input.Description, Evidence: evidence, CreatedBy: optionalString(userIDFromContext(c))})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}
func (h *FPnAGovernanceHandler) ListDataQuality(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListDataQuality(c.Request.Context(), entity, c.Query("period"), c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows), "coverage": gin.H{"status": "incomplete_if_open_items_exist"}})
}

func (h *FPnAGovernanceHandler) UpdateDataQualityStatus(c *gin.Context) {
	var input struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status != "acknowledged" && input.Status != "resolved" && input.Status != "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be acknowledged, resolved or accepted"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	item, err := h.repo.UpdateDataQualityStatus(c.Request.Context(), c.Param("id"), entity, input.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "data quality item not found or already closed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "audit_required": true})
}

type realizationInput struct {
	Period         string         `json:"period" binding:"required"`
	BaselineAmount *float64       `json:"baseline_amount"`
	TargetAmount   *float64       `json:"target_amount"`
	ActualAmount   *float64       `json:"actual_amount"`
	Currency       string         `json:"currency"`
	SourceTable    string         `json:"source_table" binding:"required"`
	SourceRecordID string         `json:"source_record_id" binding:"required"`
	DataVersion    string         `json:"data_version"`
	Evidence       map[string]any `json:"evidence"`
}

func (h *FPnAGovernanceHandler) CreateRealization(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	allowed, scopeErr := h.repo.ActionInScope(c.Request.Context(), c.Param("id"), entity)
	if scopeErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": scopeErr.Error()})
		return
	}
	if !allowed {
		c.JSON(http.StatusNotFound, gin.H{"error": "action not found"})
		return
	}
	var input realizationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	status, benefit := fpna.VerifyRealization(input.TargetAmount, input.BaselineAmount, input.ActualAmount, 0.01)
	evidence, _ := json.Marshal(input.Evidence)
	item, err := h.repo.CreateActionRealization(c.Request.Context(), &repository.FPnAActionRealization{ActionID: c.Param("id"), Period: input.Period, BaselineAmount: input.BaselineAmount, TargetAmount: input.TargetAmount, ActualAmount: input.ActualAmount, RealizedBenefit: benefit, Currency: input.Currency, SourceTable: input.SourceTable, SourceRecordID: input.SourceRecordID, DataVersion: input.DataVersion, Status: status, Evidence: evidence, VerifiedBy: optionalString(userIDFromContext(c)), VerifiedAt: func() *time.Time {
		if status == "verified" {
			v := nowUTC()
			return &v
		}
		return nil
	}()})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "verification_status": status, "formal_state": "action_realization"})
}
func (h *FPnAGovernanceHandler) ListRealizations(c *gin.Context) {
	rows, err := h.repo.ListActionRealizations(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type memoInput struct {
	MemoType                  string         `json:"memo_type" binding:"required"`
	Title                     string         `json:"title" binding:"required"`
	Basis                     string         `json:"basis"`
	ScenarioDraftID           *string        `json:"scenario_draft_id"`
	SystemFacts               map[string]any `json:"system_facts"`
	DeterministicCalculations map[string]any `json:"deterministic_calculations"`
	HumanInputs               map[string]any `json:"human_inputs"`
	AINarrative               map[string]any `json:"ai_narrative"`
	SourceReferences          []any          `json:"source_references"`
	DataVersion               string         `json:"data_version"`
	AssumptionVersion         string         `json:"assumption_version"`
	MetricDefinitionVersion   string         `json:"metric_definition_version"`
}

func (h *FPnAGovernanceHandler) CreateMemo(c *gin.Context) {
	var input memoInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	facts, _ := json.Marshal(input.SystemFacts)
	calcs, _ := json.Marshal(input.DeterministicCalculations)
	human, _ := json.Marshal(input.HumanInputs)
	ai, _ := json.Marshal(input.AINarrative)
	sources, _ := json.Marshal(input.SourceReferences)
	legal := middleware.GetTenantID(c)
	var entity *string
	if legal != "" {
		entity = &legal
	}
	item, err := h.repo.CreateMemo(c.Request.Context(), &repository.FPnADecisionMemo{LegalEntityID: entity, MemoType: input.MemoType, Title: input.Title, Basis: input.Basis, ScenarioDraftID: input.ScenarioDraftID, SystemFacts: facts, DeterministicCalculations: calcs, HumanInputs: human, AINarrative: ai, SourceReferences: sources, DataVersion: input.DataVersion, AssumptionVersion: input.AssumptionVersion, MetricDefinitionVersion: input.MetricDefinitionVersion, IdempotencyKey: c.GetHeader("Idempotency-Key"), CreatedBy: optionalString(userIDFromContext(c))})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_decision_memos", item.ID, "create", nil, item, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "basis": "Scenario", "review_required": true})
}
func (h *FPnAGovernanceHandler) ListMemos(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListMemos(c.Request.Context(), entity, c.Query("memo_type"), c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}
func (h *FPnAGovernanceHandler) UpdateMemoStatus(c *gin.Context) {
	var input struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Status != "review" && input.Status != "approved" && input.Status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "memo status must be review, approved or rejected"})
		return
	}
	if input.Status == "approved" && !hasGovernanceApprovalRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only approver or admin may approve a decision memo"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	item, err := h.repo.UpdateMemoStatus(c.Request.Context(), c.Param("id"), entity, userIDFromContext(c), input.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "memo not found or already immutable"})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_decision_memos", item.ID, "status_transition", nil, item, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "immutable": input.Status == "approved"})
}

func hasGovernanceApprovalRole(c *gin.Context) bool {
	role, _ := c.Get("role")
	value, _ := role.(string)
	return value == "admin" || value == "approver"
}

func (h *FPnAGovernanceHandler) GenerateReportPack(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if !operatingPeriodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required in YYYY-MM format"})
		return
	}
	reportType := strings.ToUpper(defaultString(c.Query("report_type"), "MBR"))
	if reportType != "WBR" && reportType != "MBR" && reportType != "QBR" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "report_type must be WBR, MBR or QBR"})
		return
	}
	format := defaultString(c.Query("format"), "json")
	allowed := map[string]bool{"json": true, "html": true, "csv": true, "xlsx": true, "pdf": true, "pptx": true}
	if !allowed[format] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be json, html, csv, xlsx, pdf or pptx"})
		return
	}
	basis := defaultString(c.Query("basis"), "Working")
	if basis != "Working" && basis != "Official" && basis != "Scenario" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "basis must be Working, Official or Scenario"})
		return
	}
	if basis == "Official" && strings.TrimSpace(c.Query("official_version_id")) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "official_version_id is required for Official report generation", "review_required": true})
		return
	}
	if basis == "Official" && !hasGovernanceApprovalRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only approver or admin may generate an Official report artifact"})
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	if basis == "Official" {
		version, versionErr := h.repo.GetPlanVersion(c.Request.Context(), c.Query("official_version_id"), entity)
		if versionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": versionErr.Error()})
			return
		}
		if version == nil || !version.IsOfficial {
			c.JSON(http.StatusConflict, gin.H{"error": "official_version_id must reference an Official frozen plan version"})
			return
		}
	}
	var legalEntityPayload *string
	if scopedID, idErr := entity.LegalEntityID(); idErr == nil {
		legalEntityPayload = &scopedID
	}
	overview := any(nil)
	actions := any([]*repository.FPnAActionItem{})
	priorRealizations := make([]*repository.FPnAActionRealization, 0)
	stores := any([]operating.FourWall{})
	equipment := any([]*repository.EquipmentOperatingFact{})
	if h.operatingRepo != nil {
		if v, e := h.operatingRepo.Overview(c.Request.Context(), entity, period); e == nil {
			overview = v
		}
		if v, e := h.operatingRepo.ListActions(c.Request.Context(), entity, period, "", ""); e == nil {
			actions = v
			for _, action := range v {
				if realizations, realizationErr := h.repo.ListActionRealizations(c.Request.Context(), action.ID); realizationErr == nil {
					for _, realization := range realizations {
						if realization.Period < period {
							priorRealizations = append(priorRealizations, realization)
						}
					}
				}
			}
		}
		if v, e := h.operatingRepo.ListStores(c.Request.Context(), entity, period, ""); e == nil {
			metrics := make([]operating.FourWall, 0, len(v))
			for _, row := range v {
				metrics = append(metrics, operating.CalculateFourWall(*row))
			}
			stores = metrics
		}
		if v, e := h.operatingRepo.ListEquipmentFacts(c.Request.Context(), entity, period, "", ""); e == nil {
			equipment = v
		}
	}
	payload := map[string]any{"report_type": reportType, "period": period, "basis": basis, "results": overview, "drivers": map[string]any{"residual_policy": "unexplained values remain visible"}, "risks": map[string]any{"data_quality": "review open data-quality items"}, "opportunities": []any{}, "decisions_requested": []any{}, "actions": actions, "prior_period_realization": priorRealizations, "retail": stores, "manufacturing": equipment}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	dataVersion := firstNonEmpty(c.Query("data_version"), fmt.Sprintf("operating-facts:%s", period))
	assumptionVersion := c.Query("assumption_version")
	metricDefinitionVersion := c.Query("metric_definition_version")
	source, _ := json.Marshal(map[string]any{"coverage": "permission_scoped", "generated_at": nowUTC(), "data_version": dataVersion, "reporting_mode": basis, "format": format, "official_version_id": c.Query("official_version_id"), "scenario_id": c.Query("scenario_id"), "assumption_version": assumptionVersion, "metric_definition_version": metricDefinitionVersion})
	artifactStatus := "draft"
	if basis == "Official" {
		artifactStatus = "published"
	}
	item, err := h.repo.CreateReportArtifact(c.Request.Context(), &repository.FPnAReportArtifact{LegalEntityID: legalEntityPayload, ReportType: reportType, ViewType: defaultString(c.Query("view"), "group"), Period: period, Basis: basis, Format: format, Status: artifactStatus, Payload: raw, SourceMetadata: source, ManifestSHA256: hex.EncodeToString(sum[:]), DataVersion: dataVersion, AssumptionVersion: assumptionVersion, MetricDefinitionVersion: metricDefinitionVersion, GeneratedBy: optionalString(userIDFromContext(c))})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "fpna_report_artifacts", item.ID, "generate", nil, item, userIDFromContext(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"data": item, "download": gin.H{"format": format, "basis": basis, "manifest_sha256": item.ManifestSHA256}, "review_required": basis != "Official"})
}
func (h *FPnAGovernanceHandler) ListReportPacks(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	rows, err := h.repo.ListReportArtifacts(c.Request.Context(), entity, c.Query("report_type"), c.Query("period"), c.Query("basis"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

func (h *FPnAGovernanceHandler) DownloadReportPack(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	item, err := h.repo.GetReportArtifact(c.Request.Context(), c.Param("id"), entity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report artifact not found"})
		return
	}
	body, contentType, extension := renderReportArtifact(item)
	filename := fmt.Sprintf("fpna-%s-%s.%s", strings.ToLower(item.ReportType), item.Period, extension)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-Report-Basis", item.Basis)
	c.Header("X-Manifest-SHA256", item.ManifestSHA256)
	c.Data(http.StatusOK, contentType, body)
}

func renderReportArtifact(item *repository.FPnAReportArtifact) ([]byte, string, string) {
	pretty := item.Payload
	var formatted bytes.Buffer
	if json.Indent(&formatted, item.Payload, "", "  ") == nil {
		pretty = formatted.Bytes()
	}
	switch strings.ToLower(item.Format) {
	case "html":
		body := fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s %s</title></head><body><h1>%s %s</h1><p>Basis: %s · Manifest: %s</p><pre>%s</pre></body></html>", html.EscapeString(item.ReportType), html.EscapeString(item.Period), html.EscapeString(item.ReportType), html.EscapeString(item.Period), html.EscapeString(item.Basis), html.EscapeString(item.ManifestSHA256), html.EscapeString(string(pretty)))
		return []byte(body), "text/html; charset=utf-8", "html"
	case "csv":
		body := fmt.Sprintf("report_type,period,basis,manifest_sha256,data_version\n%s,%s,%s,%s,%s\n", item.ReportType, item.Period, item.Basis, item.ManifestSHA256, item.DataVersion)
		return []byte(body), "text/csv; charset=utf-8", "csv"
	case "xlsx":
		return buildReportXLSX(item, pretty), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
	case "pdf":
		return buildReportPDF(item, pretty), "application/pdf", "pdf"
	case "pptx":
		return buildReportPPTX(item, pretty), "application/vnd.openxmlformats-officedocument.presentationml.presentation", "pptx"
	default:
		return pretty, "application/json; charset=utf-8", "json"
	}
}

func buildReportXLSX(item *repository.FPnAReportArtifact, payload []byte) []byte {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Report" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
	}
	cell := html.EscapeString(fmt.Sprintf("%s %s | %s | %s", item.ReportType, item.Period, item.Basis, string(payload)))
	files["xl/worksheets/sheet1.xml"] = fmt.Sprintf(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>%s</t></is></c></row></sheetData></worksheet>`, cell)
	for name, content := range files {
		writer, _ := archive.Create(name)
		_, _ = writer.Write([]byte(content))
	}
	_ = archive.Close()
	return buffer.Bytes()
}

func buildReportPDF(item *repository.FPnAReportArtifact, payload []byte) []byte {
	text := strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("FP&A %s %s | %s | %s", item.ReportType, item.Period, item.Basis, string(payload)), "(", "["), ")", "]")
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>", fmt.Sprintf("<< /Length %d >>\nstream\nBT /F1 10 Tf 40 740 Td (%s) Tj ET\nendstream", len(text)+35, text), "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for index, object := range objects {
		offsets = append(offsets, buffer.Len())
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buffer.Bytes()
}

func buildReportPPTX(item *repository.FPnAReportArtifact, payload []byte) []byte {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	title := html.EscapeString(fmt.Sprintf("%s %s · %s", item.ReportType, item.Period, item.Basis))
	text := html.EscapeString(string(payload))
	files := map[string]string{
		"[Content_Types].xml":                          `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/><Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/><Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/><Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst><p:sldSz cx="12192000" cy="6858000" type="screen4x3"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        fmt.Sprintf(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN"/><a:t>%s</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="Payload"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN"/><a:t>%s</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`, title, text),
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="title" preserve="1"><p:cSld name="Title Slide"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld name="Master"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld><p:clrMap accent1="accent1" accent2="accent2" bg1="lt1" bg2="lt2" folHlink="folHlink" hlink="hlink" tx1="dk1" tx2="dk2" xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"/><p:sldLayoutIdLst><p:sldLayoutId id="1" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/></Relationships>`,
		"ppt/theme/theme1.xml":                         `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F1F1F"/></a:dk2><a:lt2><a:srgbClr val="FFFFFF"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Aptos"/></a:majorFont><a:minorFont><a:latin typeface="Aptos"/></a:minorFont></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst/><a:lnStyleLst/><a:effectStyleLst/><a:bgFillStyleLst/></a:fmtScheme></a:themeElements></a:theme>`,
	}
	for name, content := range files {
		writer, _ := archive.Create(name)
		_, _ = writer.Write([]byte(content))
	}
	_ = archive.Close()
	return buffer.Bytes()
}

func validatePeriodRange(asOf, from, to string) error {
	for _, value := range []string{asOf, from, to} {
		if _, err := time.Parse("2006-01", value); err != nil {
			return fmt.Errorf("period must be YYYY-MM")
		}
	}
	if from > to {
		return fmt.Errorf("to_period must not be before from_period")
	}
	return nil
}

func planLineFilters(c *gin.Context) map[string]string {
	filters := make(map[string]string)
	for _, key := range []string{"business_segment", "brand", "region", "store_id", "plant", "line", "equipment_id", "asset_type", "currency"} {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			filters[key] = value
		}
	}
	return filters
}
