package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
)

type RetailScenarioHandler struct {
	service *retailscenario.Service
	actions *repository.OperatingFactsRepository
}

func NewRetailScenarioHandler(reader retailscenario.FactReader, actions *repository.OperatingFactsRepository) *RetailScenarioHandler {
	return &RetailScenarioHandler{service: retailscenario.NewService(reader), actions: actions}
}

type scenarioActionRequest struct {
	HorizonMonths      int                          `json:"horizon_months"`
	SelectedScenario   retailscenario.ScenarioInput `json:"selected_scenario"`
	Title              string                       `json:"title"`
	PlannedAction      string                       `json:"planned_action"`
	OwnerName          string                       `json:"owner_name"`
	DueDate            *string                      `json:"due_date"`
	VerificationPeriod string                       `json:"verification_period"`
}

func (h *RetailScenarioHandler) Evaluate(c *gin.Context) {
	legalEntityID, query, ok := h.parseQuery(c)
	if !ok {
		return
	}
	var request retailscenario.EvaluateRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	query.LegalEntityID = legalEntityID
	result, err := h.service.Evaluate(c.Request.Context(), query, request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RetailScenarioHandler) SaveAction(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "Idempotency-Key is required", nil)
		return
	}
	query, ok := h.parseQueryOnly(c)
	if !ok {
		return
	}
	query.LegalEntityID = ""
	if scopedID, idErr := entity.LegalEntityID(); idErr == nil {
		query.LegalEntityID = scopedID
	}
	var request scenarioActionRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	if strings.TrimSpace(request.Title) == "" || strings.TrimSpace(request.PlannedAction) == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "title and planned_action are required", nil)
		return
	}
	if len(request.Title) > 200 || len(request.PlannedAction) > 2000 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "title or planned_action is too long", nil)
		return
	}
	if request.VerificationPeriod != "" && !operatingPeriodPattern.MatchString(request.VerificationPeriod) {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "verification_period must be YYYY-MM", nil)
		return
	}
	if request.DueDate != nil && strings.TrimSpace(*request.DueDate) != "" {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(*request.DueDate)); err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "due_date must be YYYY-MM-DD", nil)
			return
		}
	}
	evaluateRequest := retailscenario.EvaluateRequest{HorizonMonths: request.HorizonMonths, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, request.SelectedScenario}}
	if request.SelectedScenario.Key == "baseline" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "selected_scenario must be a non-baseline plan", nil)
		return
	}
	result, err := h.service.Evaluate(c.Request.Context(), query, evaluateRequest)
	if err != nil {
		h.writeError(c, err)
		return
	}
	var selected *retailscenario.ScenarioResult
	for i := range result.Scenarios {
		if result.Scenarios[i].Key == request.SelectedScenario.Key {
			selected = &result.Scenarios[i]
			break
		}
	}
	if selected == nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "selected_scenario was not evaluated", nil)
		return
	}
	fingerprint := retailscenario.RequestFingerprint(query, evaluateRequest, request.Title, request.PlannedAction, request.OwnerName, stringValue(request.DueDate), request.VerificationPeriod)
	if existing, lookupErr := h.actions.GetActionByIdempotency(c.Request.Context(), entity, key); lookupErr != nil {
		writeSystemFailure(c, http.StatusInternalServerError, lookupErr)
		return
	} else if existing != nil {
		if existingFingerprint(existing.Evidence) != fingerprint {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, "Idempotency-Key payload conflict", gin.H{"reason": "idempotency_payload_conflict"})
			return
		}
		existing.Replayed = true
		c.JSON(http.StatusOK, gin.H{"basis": "Scenario", "formal_execution": false, "review_required": true, "data": existing, "idempotent_replay": true})
		return
	}
	var legalEntityPayload *string
	if scopedID, idErr := entity.LegalEntityID(); idErr == nil {
		legalEntityPayload = &scopedID
	}
	dueDate := parseDatePointer(request.DueDate)
	evidence := map[string]interface{}{"store": result.Store, "data_classification": result.DataClassification, "dataset_version": result.DatasetVersion, "source_system": result.SourceSystem, "current": result.Current, "fact_version_min": result.Evidence.FactVersionMin, "fact_version_max": result.Evidence.FactVersionMax, "scenario_version": result.ScenarioVersion, "formula_version": result.FormulaVersion, "baseline": result.Baseline, "selected_scenario": selected, "kpi_drilldown_url": result.Evidence.KPIDrilldownURL, "formal_execution": false, "request_fingerprint": fingerprint}
	evidenceJSON, _ := json.Marshal(evidence)
	baselineContribution := metricValue(selectedMetric(result.Baseline, "store_contribution"))
	planContribution := metricValue(selectedMetric(*selected, "store_contribution"))
	benefit := selected.HorizonContributionChange
	item := &repository.FPnAActionItem{LegalEntityID: legalEntityPayload, Period: request.VerificationPeriod, Category: "retail_store_scenario", Severity: "medium", Status: "open", Title: request.Title, Description: "Scenario plan is a deterministic working draft; verify before action.", RuleCode: retailscenario.ActionRuleCode(fingerprint, key), SourceTable: "retail_store_day_facts", SourceRecordID: query.StoreID, DataVersion: result.ScenarioVersion, IdempotencyKey: key, Currency: result.Currency, OwnerName: request.OwnerName, DueDate: dueDate, BaselineAmount: baselineContribution, TargetAmount: planContribution, ExpectedBenefit: benefit, VerificationPeriod: request.VerificationPeriod, VerificationStatus: "not_due", PlannedAction: request.PlannedAction, Evidence: evidenceJSON}
	created, replayed, createErr := h.actions.CreateScenarioAction(c.Request.Context(), item)
	if createErr != nil {
		if errors.Is(createErr, repository.ErrRetailScenarioActionScopeConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, createErr.Error(), gin.H{"reason": "scenario_action_scope_conflict"})
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, createErr)
		return
	}
	if replayed {
		if existingFingerprint(created.Evidence) != fingerprint {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, "Idempotency-Key payload conflict", gin.H{"reason": "idempotency_payload_conflict"})
			return
		}
		created.Replayed = true
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Scenario", "formal_execution": false, "review_required": true, "data": created, "idempotent_replay": replayed})
}

func (h *RetailScenarioHandler) parseQuery(c *gin.Context) (string, retailscenario.Query, bool) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return "", retailscenario.Query{}, false
	}
	query, ok := h.parseQueryOnly(c)
	return legalEntityID, query, ok
}

func (h *RetailScenarioHandler) parseQueryOnly(c *gin.Context) (retailscenario.Query, bool) {
	storeID := strings.TrimSpace(c.Param("store_id"))
	if _, err := uuid.Parse(storeID); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "store_id must be a UUID", nil)
		return retailscenario.Query{}, false
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("as_of")))
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of must be an ISO date (YYYY-MM-DD)", nil)
		return retailscenario.Query{}, false
	}
	windowDays := 14
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		parsed, scanErr := strconv.Atoi(raw)
		if scanErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
			return retailscenario.Query{}, false
		}
		windowDays = parsed
	}
	classification, datasetVersion, ok := parseRetailStore360Classification(c)
	if !ok {
		return retailscenario.Query{}, false
	}
	return retailscenario.Query{StoreID: storeID, AsOf: asOf, WindowDays: windowDays, Classification: classification, DatasetVersion: datasetVersion, SourceSystem: strings.TrimSpace(c.Query("source_system"))}, true
}

func (h *RetailScenarioHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, retailscenario.ErrInvalidRequest):
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
	case errors.Is(err, retailscenario.ErrStoreNotFound):
		writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, err.Error(), nil)
	case errors.Is(err, repository.ErrRetailKPISourceConflict):
		writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
	case errors.Is(err, retailscenario.ErrDataUnavailable):
		var evidenceErr *retailscenario.ScenarioEvidenceError
		if errors.As(err, &evidenceErr) {
			writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeDataUnavailable, err.Error(), gin.H{"reason": evidenceErr.Reason, "evidence": evidenceErr.Evidence})
			return
		}
		// DataUnavailableError is currently never constructed; keep the
		// branch honest instead of dereferencing a nil reason (the nil
		// deref flagged in the architecture review is gone with this shape).
		var dataErr *retailscenario.DataUnavailableError
		if errors.As(err, &dataErr) {
			writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeDataUnavailable, err.Error(), gin.H{"reason": dataErr.Reason})
			return
		}
		writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeDataUnavailable, err.Error(), nil)
	default:
		writeSystemFailure(c, http.StatusInternalServerError, err)
	}
}

func existingFingerprint(evidence json.RawMessage) string {
	var value struct {
		RequestFingerprint string `json:"request_fingerprint"`
	}
	_ = json.Unmarshal(evidence, &value)
	return value.RequestFingerprint
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
func metricValue(metric retailscenario.Metric) *float64 { return metric.Result }
func selectedMetric(result retailscenario.ScenarioResult, code string) retailscenario.Metric {
	return result.Metrics[code]
}
