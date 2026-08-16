package handlers

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// FPnAPlanImportHandler imports a budget/forecast/scenario plan version from
// the controlled Excel/CSV template (PRD P5-3, spec fpna-version-lifecycle):
// one version per upload, store-grain monthly lines, row-level errors with
// partial success, and business-level idempotency on (entity, name,
// as_of_period) — versions freeze on creation, never edit in place.
type FPnAPlanImportHandler struct {
	population retailIngestPopulationReader
	plans      planImportStore
}

type planImportStore interface {
	CreatePlanVersion(context.Context, *repository.FPnAPlanVersion) (*repository.FPnAPlanVersion, error)
	ListPlanVersions(context.Context, access.EntityFilter, string) ([]*repository.FPnAPlanVersion, error)
	CreatePlanLine(context.Context, *repository.FPnAPlanLine) (*repository.FPnAPlanLine, error)
	DeletePlanVersion(context.Context, string, access.EntityFilter) error
}

func NewFPnAPlanImportHandler(population retailIngestPopulationReader, plans planImportStore) *FPnAPlanImportHandler {
	return &FPnAPlanImportHandler{population: population, plans: plans}
}

var planPeriodPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func (h *FPnAPlanImportHandler) Import(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file is required", nil)
		return
	}
	if fileHeader.Size > maxRetailIngestFileSize {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file exceeds the 10MB import limit", nil)
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	versionType := strings.TrimSpace(c.PostForm("version_type"))
	if name == "" || !stringIn(versionType, "budget", "forecast", "scenario") {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "name and version_type (budget|forecast|scenario) are required", nil)
		return
	}
	source := strings.TrimSpace(c.PostForm("source"))
	asOfPeriod := strings.TrimSpace(c.PostForm("as_of_period"))
	fromPeriod := strings.TrimSpace(c.PostForm("from_period"))
	toPeriod := strings.TrimSpace(c.PostForm("to_period"))
	if !planPeriodPattern.MatchString(asOfPeriod) || !planPeriodPattern.MatchString(fromPeriod) || !planPeriodPattern.MatchString(toPeriod) || fromPeriod > toPeriod {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of_period/from_period/to_period must be YYYY-MM and from_period <= to_period", nil)
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(c.PostForm("currency")))
	isOfficial := strings.EqualFold(strings.TrimSpace(c.PostForm("is_official")), "true")
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}

	opened, err := fileHeader.Open()
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file cannot be opened", nil)
		return
	}
	defer opened.Close()
	data, err := io.ReadAll(opened)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file cannot be read", nil)
		return
	}
	headers, rows, err := controlledintake.Parse(controlledintake.Source{Filename: fileHeader.Filename, Data: data})
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	headerMap := map[string]int{}
	for index, value := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(value))] = index
	}

	// Business-level idempotency: (entity, name, as_of_period) is unique;
	// a replay of the same version returns the existing one untouched.
	existing, err := h.plans.ListPlanVersions(c.Request.Context(), entity, "")
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	for _, version := range existing {
		if version.Name == name && version.AsOfPeriod == asOfPeriod {
			c.JSON(http.StatusOK, gin.H{"basis": "Working", "version": version, "accepted_rows": 0, "rejected_rows": 0, "idempotent_replay": true, "errors": []controlledintake.RowError{}})
			return
		}
	}

	// Store resolution: store_code against the entity's store master.
	population, err := h.population.ListStorePopulation(c.Request.Context(), legalEntityID, "production", "", nil)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	storeByCode := map[string]retailkpi.StorePopulation{}
	for _, store := range population {
		storeByCode[strings.ToLower(strings.TrimSpace(store.StoreCode))] = store
	}

	version := &repository.FPnAPlanVersion{
		LegalEntityID: &legalEntityID, Name: name, VersionType: versionType, Source: source,
		CoverageScope: jsonRawScope(legalEntityID, fromPeriod, toPeriod), Currency: currency,
		AsOfPeriod: asOfPeriod, FromPeriod: fromPeriod, ToPeriod: toPeriod,
		IsOfficial: isOfficial, CreatedBy: optionalString(userIDFromContext(c)),
	}
	version, err = h.plans.CreatePlanVersion(c.Request.Context(), version)
	if err != nil {
		writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
		return
	}

	rowErrors := make([]controlledintake.RowError, 0)
	accepted := 0
	for index, row := range rows {
		line, rowErr := planImportLine(row, index, headerMap, storeByCode, version, currency)
		if rowErr != nil {
			rowErrors = append(rowErrors, *rowErr)
			continue
		}
		if _, err := h.plans.CreatePlanLine(c.Request.Context(), line); err != nil {
			// P1-2: a partial write must not leave an orphan version behind
			// — compensate by deleting the just-created version (cascade).
			_ = h.plans.DeletePlanVersion(c.Request.Context(), version.ID, entity)
			writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
			return
		}
		accepted++
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "version": version, "accepted_rows": accepted,
		"rejected_rows": len(rowErrors), "idempotent_replay": false, "errors": rowErrors,
	})
}

func jsonRawScope(legalEntityID, fromPeriod, toPeriod string) []byte {
	return []byte(`{"legal_entity_id":"` + legalEntityID + `","from_period":"` + fromPeriod + `","to_period":"` + toPeriod + `"}`)
}

// planHeaderIndex normalizes the controlled template header case for the
// fixed column contract.
func planHeaderIndex(headers []string, key string) (int, bool) {
	for index, value := range headers {
		if strings.EqualFold(strings.TrimSpace(value), key) {
			return index, true
		}
	}
	return -1, false
}

func planImportLine(row []string, index int, headers map[string]int, storeByCode map[string]retailkpi.StorePopulation, version *repository.FPnAPlanVersion, defaultCurrency string) (*repository.FPnAPlanLine, *controlledintake.RowError) {
	get := func(key string) string {
		position, ok := headers[key]
		if !ok || position >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[position])
	}
	storeCode := get("store_code")
	store, ok := storeByCode[strings.ToLower(storeCode)]
	if !ok {
		return nil, &controlledintake.RowError{Row: index + 2, Code: "unmatched_store", Message: "store_code " + storeCode + " is not in this entity's store master"}
	}
	period := get("period")
	if !planPeriodPattern.MatchString(period) {
		return nil, &controlledintake.RowError{Row: index + 2, Code: "bad_period", Message: "period must be YYYY-MM"}
	}
	if period < version.FromPeriod || period > version.ToPeriod {
		return nil, &controlledintake.RowError{Row: index + 2, Code: "period_out_of_range", Message: "period " + period + " is outside the version range"}
	}
	currency := strings.ToUpper(get("currency"))
	if currency == "" {
		currency = strings.ToUpper(defaultCurrency)
	}
	if currency == "" {
		return nil, &controlledintake.RowError{Row: index + 2, Code: "missing_currency", Message: "currency is required (row or version default)"}
	}
	numeric := func(key string) (*float64, *controlledintake.RowError) {
		raw := get(key)
		if raw == "" {
			return nil, nil
		}
		if raw == "" {
			return nil, nil
		}
		value := parseCSVFloat(raw)
		if value == nil || *value < 0 {
			return nil, &controlledintake.RowError{Row: index + 2, Code: "bad_number", Message: key + " must be a non-negative number"}
		}
		return value, nil
	}
	revenue, rowErr := numeric("revenue")
	if rowErr != nil {
		return nil, rowErr
	}
	grossProfit, rowErr := numeric("gross_profit")
	if rowErr != nil {
		return nil, rowErr
	}
	laborCost, rowErr := numeric("labor_cost")
	if rowErr != nil {
		return nil, rowErr
	}
	fixedRent, rowErr := numeric("fixed_rent")
	if rowErr != nil {
		return nil, rowErr
	}
	variableRent, rowErr := numeric("variable_rent")
	if rowErr != nil {
		return nil, rowErr
	}
	nonLeaseCost, rowErr := numeric("non_lease_cost")
	if rowErr != nil {
		return nil, rowErr
	}
	fourWallEBITDA, rowErr := numeric("four_wall_ebitda")
	if rowErr != nil {
		return nil, rowErr
	}
	storeID := store.StoreID
	return &repository.FPnAPlanLine{
		PlanVersionID: version.ID, Period: period, Grain: "store", StoreID: &storeID,
		Currency: currency, Revenue: revenue, GrossProfit: grossProfit, LaborCost: laborCost,
		FixedRent: fixedRent, VariableRent: variableRent, NonLeaseCost: nonLeaseCost,
		FourWallEBITDA: fourWallEBITDA, SourceSystem: version.Source,
		SourceRecordID: strings.TrimSpace(get("source_record_id")), AsOfAt: nowUTC(),
	}, nil
}
