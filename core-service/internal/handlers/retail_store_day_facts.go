package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
	auditservice "github.com/lease-management-system/core-service/internal/services/audit"
)

const (
	maxRetailStoreDayBatch        = 500
	maxRetailStoreDayRange        = 366
	defaultRetailStoreDayPageSize = 500
	maxRetailStoreDayPageSize     = 5000
)

var (
	retailBusinessDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	retailCurrencyPattern     = regexp.MustCompile(`^[A-Z]{3}$`)
)

type retailStoreDayFactStore interface {
	UpsertRetailStoreDayFactsAtomic(context.Context, access.EntityFilter, []*repository.RetailStoreDayFact, string, string, *string, repository.RetailStoreDayFactAuditFunc) (*repository.RetailStoreDayFactWriteResult, error)
	ListRetailStoreDayFactsPage(context.Context, access.EntityFilter, string, string, []string, int, int) (*repository.RetailStoreDayFactsPage, error)
}

type retailStoreDayFactAuditor interface {
	LogInTx(context.Context, repository.DBTX, string, string, string, interface{}, interface{}, string, *gin.Context) error
}

type transactionalRetailStoreDayFactAuditor struct{ logger *auditservice.Logger }

func (a transactionalRetailStoreDayFactAuditor) LogInTx(ctx context.Context, tx repository.DBTX, tableName, recordID, action string, oldValues, newValues interface{}, changedBy string, c *gin.Context) error {
	return a.logger.WithTx(tx).Log(ctx, tableName, recordID, action, oldValues, newValues, changedBy, c)
}

type RetailStoreDayFactsHandler struct {
	repo  retailStoreDayFactStore
	audit retailStoreDayFactAuditor
}

func NewRetailStoreDayFactsHandler(repo retailStoreDayFactStore, auditLogger any) *RetailStoreDayFactsHandler {
	var auditor retailStoreDayFactAuditor
	switch value := auditLogger.(type) {
	case nil:
	case retailStoreDayFactAuditor:
		auditor = value
	case *auditservice.Logger:
		auditor = transactionalRetailStoreDayFactAuditor{logger: value}
	default:
		panic("unsupported retail store-day fact audit logger")
	}
	return &RetailStoreDayFactsHandler{repo: repo, audit: auditor}
}

type retailStoreDayFactInput struct {
	StoreID                  string   `json:"store_id"`
	BusinessDate             string   `json:"business_date"`
	Currency                 string   `json:"currency"`
	Revenue                  *float64 `json:"revenue"`
	GrossProfit              *float64 `json:"gross_profit"`
	Transactions             *float64 `json:"transactions"`
	Footfall                 *float64 `json:"footfall"`
	AreaSqm                  *float64 `json:"area_sqm"`
	LaborCost                *float64 `json:"labor_cost"`
	FixedRent                *float64 `json:"fixed_rent"`
	VariableRent             *float64 `json:"variable_rent"`
	NonLeaseCost             *float64 `json:"non_lease_cost"`
	OtherControllableCost    *float64 `json:"other_controllable_cost"`
	SourceSystem             string   `json:"source_system"`
	SourceRecordID           string   `json:"source_record_id"`
	ImportBatchID            *string  `json:"import_batch_id"`
	AsOfAt                   string   `json:"as_of_at"`
	Version                  int      `json:"version"`
	ReconciliationStatus     string   `json:"reconciliation_status"`
	MappingStatus            string   `json:"mapping_status"`
	DataQualityStatus        string   `json:"data_quality_status"`
	DataClassification       string   `json:"data_classification"`
	SimulationDatasetVersion *string  `json:"simulation_dataset_version"`
}

type retailStoreDayFactRequest struct {
	Items []retailStoreDayFactInput `json:"items"`
}

func (h *RetailStoreDayFactsHandler) Upsert(c *gin.Context) {
	var request retailStoreDayFactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "invalid JSON request: "+err.Error(), nil)
		return
	}
	if len(request.Items) == 0 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "items must contain at least one fact", nil)
		return
	}
	if len(request.Items) > maxRetailStoreDayBatch {
		writeCodedError(c, http.StatusRequestEntityTooLarge, errcontract.CodeInvalidArguments, "batch exceeds maximum of 500 facts", gin.H{"max_batch_size": maxRetailStoreDayBatch})
		return
	}

	prepared := make([]*repository.RetailStoreDayFact, 0, len(request.Items))
	for index, input := range request.Items {
		fact, validationError := validateRetailStoreDayInput(input)
		if validationError != "" {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, validationError, gin.H{"index": index})
			return
		}
		prepared = append(prepared, fact)
	}

	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	payload, err := json.Marshal(prepared)
	if err != nil {
		writeCodedError(c, http.StatusInternalServerError, errcontract.CodeSystemFailure, "encode idempotency payload", nil)
		return
	}
	digest := sha256.Sum256(payload)
	payloadSHA256 := hex.EncodeToString(digest[:])
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > 255 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "Idempotency-Key exceeds 255 characters", nil)
		return
	}
	for _, fact := range prepared {
		fact.CreatedBy = optionalString(userIDFromContext(c))
	}
	var auditFn repository.RetailStoreDayFactAuditFunc
	if h.audit != nil {
		auditFn = func(ctx context.Context, tx repository.DBTX, oldFact, newFact *repository.RetailStoreDayFact) error {
			return h.audit.LogInTx(ctx, tx, "retail_store_day_facts", newFact.ID, "upsert", oldFact, newFact, userIDFromContext(c), c)
		}
	}
	result, err := h.repo.UpsertRetailStoreDayFactsAtomic(c.Request.Context(), entity, prepared, idempotencyKey, payloadSHA256, optionalString(userIDFromContext(c)), auditFn)
	if err != nil {
		if errors.Is(err, repository.ErrRetailStoreDayFactIdempotencyConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), nil)
			return
		}
		// Contract errors (scope_denied, invalid batch reference, replay
		// verification) keep their code and message; anything else is
		// sanitized as an internal failure.
		writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
		return
	}
	response := retailStoreDayEnvelope(result.Facts, "", "", nil, len(result.Facts), 1, len(result.Facts), 0)
	response["saved_count"] = len(result.Facts)
	response["idempotent_replay"] = result.Replayed
	c.JSON(http.StatusOK, response)
}

func (h *RetailStoreDayFactsHandler) List(c *gin.Context) {
	dateFrom, dateTo := strings.TrimSpace(c.Query("date_from")), strings.TrimSpace(c.Query("date_to"))
	from, err := parseRetailBusinessDate(dateFrom)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "date_from must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	to, err := parseRetailBusinessDate(dateTo)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "date_to must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	if to.Before(from) {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "date_to must not be before date_from", nil)
		return
	}
	if int(to.Sub(from).Hours()/24)+1 > maxRetailStoreDayRange {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "date range exceeds maximum of 366 days", gin.H{"max_range_days": maxRetailStoreDayRange})
		return
	}

	storeIDs, storeIDError := retailStoreIDs(c.QueryArray("store_id"))
	if storeIDError != "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, storeIDError, nil)
		return
	}
	page, pageSize, offset, paginationError := retailStoreDayPagination(c)
	if paginationError != "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, paginationError, nil)
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	result, err := h.repo.ListRetailStoreDayFactsPage(c.Request.Context(), entity, dateFrom, dateTo, storeIDs, pageSize, offset)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, retailStoreDayEnvelope(result.Data, dateFrom, dateTo, storeIDs, result.Total, page, pageSize, offset))
}

func validateRetailStoreDayInput(input retailStoreDayFactInput) (*repository.RetailStoreDayFact, string) {
	storeID := strings.TrimSpace(input.StoreID)
	if _, err := uuid.Parse(storeID); err != nil {
		return nil, "store_id must be a UUID"
	}
	businessDate := strings.TrimSpace(input.BusinessDate)
	if _, err := parseRetailBusinessDate(businessDate); err != nil {
		return nil, "business_date must be an ISO date (YYYY-MM-DD)"
	}
	currency := strings.TrimSpace(input.Currency)
	if !retailCurrencyPattern.MatchString(currency) {
		return nil, "currency must be a three-letter uppercase ISO code"
	}
	if input.Revenue == nil {
		return nil, "revenue is required; use 0 explicitly for a confirmed zero"
	}
	if field := firstNegativeRetailField(input); field != "" {
		return nil, field + " cannot be negative"
	}
	sourceSystem := strings.TrimSpace(input.SourceSystem)
	if sourceSystem == "" {
		return nil, "source_system is required"
	}
	if input.Version < 0 {
		return nil, "version cannot be negative"
	}
	reconciliation := defaultString(strings.TrimSpace(input.ReconciliationStatus), "unreconciled")
	if !stringIn(reconciliation, "unreconciled", "matched", "warning", "failed") {
		return nil, "invalid reconciliation_status"
	}
	mapping := defaultString(strings.TrimSpace(input.MappingStatus), "mapped")
	if !stringIn(mapping, "mapped", "unmapped", "ambiguous") {
		return nil, "invalid mapping_status"
	}
	quality := defaultString(strings.TrimSpace(input.DataQualityStatus), "unassessed")
	if !stringIn(quality, "unassessed", "valid", "warning", "invalid") {
		return nil, "invalid data_quality_status"
	}
	classification := strings.TrimSpace(input.DataClassification)
	if !stringIn(classification, "production", "simulated") {
		return nil, "data_classification must be production or simulated"
	}
	var simulationVersion *string
	if input.SimulationDatasetVersion != nil {
		trimmed := strings.TrimSpace(*input.SimulationDatasetVersion)
		if trimmed != "" {
			simulationVersion = &trimmed
		}
	}
	if classification == "simulated" && simulationVersion == nil {
		return nil, "simulation_dataset_version is required for simulated facts"
	}
	if classification == "production" && simulationVersion != nil {
		return nil, "simulation_dataset_version must be absent for production facts"
	}
	if input.ImportBatchID != nil {
		if _, err := uuid.Parse(strings.TrimSpace(*input.ImportBatchID)); err != nil {
			return nil, "import_batch_id must be a UUID"
		}
	}

	fact := &repository.RetailStoreDayFact{
		StoreID: storeID, BusinessDate: businessDate, Currency: currency, Revenue: *input.Revenue,
		GrossProfit: input.GrossProfit, Transactions: input.Transactions, Footfall: input.Footfall,
		AreaSqm: input.AreaSqm, LaborCost: input.LaborCost, FixedRent: input.FixedRent,
		VariableRent: input.VariableRent, NonLeaseCost: input.NonLeaseCost,
		OtherControllableCost: input.OtherControllableCost, SourceSystem: sourceSystem,
		SourceRecordID: strings.TrimSpace(input.SourceRecordID), ImportBatchID: input.ImportBatchID,
		Version: input.Version, ReconciliationStatus: reconciliation, MappingStatus: mapping,
		DataQualityStatus: quality, DataClassification: classification,
		SimulationDatasetVersion: simulationVersion,
	}
	if input.AsOfAt != "" {
		asOf, err := time.Parse(time.RFC3339, strings.TrimSpace(input.AsOfAt))
		if err != nil {
			return nil, "as_of_at must be RFC3339"
		}
		fact.AsOfAt = asOf.UTC()
	}
	return fact, ""
}

func firstNegativeRetailField(input retailStoreDayFactInput) string {
	values := []struct {
		name  string
		value *float64
	}{
		{"revenue", input.Revenue}, {"gross_profit", input.GrossProfit}, {"transactions", input.Transactions},
		{"footfall", input.Footfall}, {"area_sqm", input.AreaSqm}, {"labor_cost", input.LaborCost},
		{"fixed_rent", input.FixedRent}, {"variable_rent", input.VariableRent},
		{"non_lease_cost", input.NonLeaseCost}, {"other_controllable_cost", input.OtherControllableCost},
	}
	for _, item := range values {
		if item.value != nil && *item.value < 0 {
			return item.name
		}
	}
	return ""
}

func parseRetailBusinessDate(value string) (time.Time, error) {
	if !retailBusinessDatePattern.MatchString(value) {
		return time.Time{}, errors.New("invalid date format")
	}
	return time.Parse("2006-01-02", value)
}

func retailStoreIDs(raw []string) ([]string, string) {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, value := range raw {
		for _, candidate := range strings.Split(value, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, err := uuid.Parse(candidate); err != nil {
				return nil, "store_id must contain UUID values"
			}
			if _, exists := seen[candidate]; !exists {
				seen[candidate] = struct{}{}
				result = append(result, candidate)
			}
		}
	}
	return result, ""
}

func retailStoreDayPagination(c *gin.Context) (int, int, int, string) {
	page := 1
	pageSize := defaultRetailStoreDayPageSize
	for name, target := range map[string]*int{"page": &page, "page_size": &pageSize} {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return 0, 0, 0, name + " must be a positive integer"
		}
		*target = value
	}
	if pageSize > maxRetailStoreDayPageSize {
		return 0, 0, 0, "page_size exceeds maximum of 5000"
	}
	if page > 1_000_000 {
		return 0, 0, 0, "page exceeds maximum supported page"
	}
	offset := (page - 1) * pageSize
	return page, pageSize, offset, ""
}

func retailStoreDayEnvelope(rows []*repository.RetailStoreDayFact, dateFrom, dateTo string, requestedStoreIDs []string, total, page, pageSize, offset int) gin.H {
	classifications := make(map[string]struct{})
	versions := make(map[string]struct{})
	stores := make(map[string]struct{})
	asOf := time.Time{}
	coverageFrom, coverageTo := dateFrom, dateTo
	for _, row := range rows {
		classifications[row.DataClassification] = struct{}{}
		stores[row.StoreID] = struct{}{}
		if coverageFrom == "" || row.BusinessDate < coverageFrom {
			coverageFrom = row.BusinessDate
		}
		if coverageTo == "" || row.BusinessDate > coverageTo {
			coverageTo = row.BusinessDate
		}
		if row.SimulationDatasetVersion != nil && *row.SimulationDatasetVersion != "" {
			versions[*row.SimulationDatasetVersion] = struct{}{}
		}
		if row.AsOfAt.After(asOf) {
			asOf = row.AsOfAt
		}
	}
	classification := "none"
	if len(classifications) == 1 {
		for value := range classifications {
			classification = value
		}
	} else if len(classifications) > 1 {
		classification = "mixed"
	}
	datasetVersions := make([]string, 0, len(versions))
	for value := range versions {
		datasetVersions = append(datasetVersions, value)
	}
	sort.Strings(datasetVersions)
	if asOf.IsZero() {
		asOf = nowUTC()
	}
	returned := len(rows)
	hasMore := offset+returned < total
	return gin.H{
		"basis": "Working", "as_of": asOf, "data_classification": classification,
		"simulation_dataset_versions": datasetVersions,
		"coverage": gin.H{"date_from": coverageFrom, "date_to": coverageTo,
			"requested_store_ids": requestedStoreIDs, "returned_store_count": len(stores)},
		"source": gin.H{"type": "retail_store_day_facts", "grain": "store_day", "versioned": true},
		"total":  total, "returned_count": returned, "data": rows,
		"pagination": gin.H{"page": page, "page_size": pageSize, "offset": offset, "has_more": hasMore, "truncated": hasMore},
	}
}

func stringIn(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
