package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

const maxRetailKPIDateRange = 366

type retailKPIReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
}

// RetailKPIHandler exposes only additive, protected daily KPI endpoints. It
// intentionally has no write path and never touches IFRS16 tables.
type RetailKPIHandler struct{ repo retailKPIReader }

func NewRetailKPIHandler(repo retailKPIReader) *RetailKPIHandler {
	return &RetailKPIHandler{repo: repo}
}

func (h *RetailKPIHandler) Definitions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "formula_version": retailkpi.FormulaVersion, "definitions": retailkpi.Definitions()})
}

func (h *RetailKPIHandler) StoreDays(c *gin.Context) {
	fromText, toText := strings.TrimSpace(c.Query("date_from")), strings.TrimSpace(c.Query("date_to"))
	from, err := time.Parse("2006-01-02", fromText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_from must be an ISO date (YYYY-MM-DD)"})
		return
	}
	to, err := time.Parse("2006-01-02", toText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_to must be an ISO date (YYYY-MM-DD)"})
		return
	}
	if to.Before(from) || int(to.Sub(from).Hours()/24)+1 > maxRetailKPIDateRange {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range must be 1-366 days"})
		return
	}
	classification := strings.TrimSpace(c.Query("data_classification"))
	if classification != "production" && classification != "simulated" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data_classification must be explicitly production or simulated"})
		return
	}
	datasetVersion := strings.TrimSpace(c.Query("dataset_version"))
	if datasetVersion == "" {
		datasetVersion = strings.TrimSpace(c.Query("simulation_dataset_version"))
	}
	if classification == "simulated" && datasetVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is required for simulated data"})
		return
	}
	if classification == "production" && datasetVersion != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is not allowed for production data"})
		return
	}
	groupBy := strings.TrimSpace(c.DefaultQuery("group_by", "total"))
	if groupBy != "total" && groupBy != "region" && groupBy != "brand" && groupBy != "store" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_by must be one of total, region, brand, store"})
		return
	}
	storeIDs, storeErr := parseRetailKPIStoreIDs(c.QueryArray("store_id"))
	if storeErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": storeErr})
		return
	}
	legalEntityID := middleware.GetTenantID(c)
	if strings.TrimSpace(legalEntityID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal entity scope is required"})
		return
	}
	result, err := h.repo.QueryFacts(c.Request.Context(), legalEntityID, fromText, toText, classification, datasetVersion, strings.TrimSpace(c.Query("source_system")), storeIDs)
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	aggregates, coverage, err := retailkpi.AggregateFacts(result.Facts, retailkpi.Request{DateFrom: from, DateTo: to, RequestedDateFrom: fromText, RequestedDateTo: toText, GroupBy: groupBy, ExpectedStoreCount: result.ExpectedStoreCount})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var asOf any
	if !result.HighestAsOf.IsZero() {
		asOf = result.HighestAsOf
	}
	response := gin.H{
		"basis": "Working", "formula_version": retailkpi.FormulaVersion,
		"data_classification": classification, "dataset_version": datasetVersion, "simulation_dataset_versions": result.DatasetVersions,
		"requested_scope": gin.H{"legal_entity_id": legalEntityID, "store_ids": storeIDs},
		"group_by":        groupBy, "as_of": asOf, "source_system": strings.TrimSpace(c.Query("source_system")),
		"coverage": coverage, "multi_currency": len(uniqueCurrencies(result.Facts)) > 1, "total_rows": len(aggregates),
		"fact_version_min": result.MinFactVersion, "fact_version_max": result.MaxFactVersion,
		"data": aggregates, "definition_url": "/api/v1/retail/kpis/definitions",
		"source": gin.H{"table": "retail_store_day_facts", "source_systems": result.SourceSystems, "dataset_versions": result.DatasetVersions, "selected_fact_count": len(result.Facts), "fact_version_range": gin.H{"min": result.MinFactVersion, "max": result.MaxFactVersion}, "highest_as_of": asOf},
	}
	c.JSON(http.StatusOK, response)
}

func parseRetailKPIStoreIDs(values []string) ([]string, string) {
	result := make([]string, 0)
	for _, raw := range values {
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				if _, err := uuid.Parse(item); err != nil {
					return nil, "store_id must be a UUID"
				}
				result = append(result, item)
			}
		}
	}
	return result, ""
}

func uniqueCurrencies(facts []retailkpi.DailyFact) map[string]bool {
	result := map[string]bool{}
	for _, f := range facts {
		result[f.Currency] = true
	}
	return result
}
