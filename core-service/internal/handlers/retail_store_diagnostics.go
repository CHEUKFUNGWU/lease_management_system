package handlers

import (
	"context"
	"fmt"
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
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailexport"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

type retailStoreDiagnosticsReader interface {
	retailKPIReader
	ListStorePopulation(context.Context, string, string, string, []string) ([]retailkpi.StorePopulation, error)
}

type RetailStoreDiagnosticsHandler struct {
	service *retailstore360.Service
	repo    retailStoreDiagnosticsReader
}

func NewRetailStoreDiagnosticsHandler(repo retailStoreDiagnosticsReader) *RetailStoreDiagnosticsHandler {
	return &RetailStoreDiagnosticsHandler{service: retailstore360.NewService(repo), repo: repo}
}

func (h *RetailStoreDiagnosticsHandler) StoreOptions(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	classification, datasetVersion, ok := parseRetailStore360Classification(c)
	if !ok {
		return
	}
	stores, err := h.repo.ListStorePopulation(c.Request.Context(), legalEntityID, classification, datasetVersion, nil)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	data := make([]gin.H, 0, len(stores))
	for _, store := range stores {
		data = append(data, gin.H{"store_id": store.StoreID, "store_code": store.StoreCode, "store_name": store.StoreName, "brand": store.Brand, "region": store.Region})
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "data_classification": classification, "dataset_version": datasetVersion,
		"data": data,
	})
}

func (h *RetailStoreDiagnosticsHandler) Diagnostics(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	storeID := strings.TrimSpace(c.Param("store_id"))
	if _, err := uuid.Parse(storeID); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "store_id must be a UUID", nil)
		return
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("as_of")))
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	windowDays := 14
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		windowDays, err = strconv.Atoi(raw)
		if err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
			return
		}
	}
	classification, datasetVersion, ok := parseRetailStore360Classification(c)
	if !ok {
		return
	}
	result, err := h.service.Build(c.Request.Context(), retailstore360.Query{
		LegalEntityID: legalEntityID, StoreID: storeID, AsOf: asOf, WindowDays: windowDays,
		Classification: classification, DatasetVersion: datasetVersion,
		SourceSystem: strings.TrimSpace(c.Query("source_system")),
	})
	if err != nil {
		switch {
		case errors.Is(err, retailstore360.ErrInvalidQuery):
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		case errors.Is(err, retailstore360.ErrStoreNotFound):
			writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, err.Error(), nil)
		case errors.Is(err, repository.ErrRetailKPISourceConflict):
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
		default:
			writeSystemFailure(c, http.StatusInternalServerError, err)
		}
		return
	}
	if strings.TrimSpace(c.Query("format")) == "csv" {
		descriptor, descriptorErr := retailexport.Descriptor(retailexport.KindStoreDiagnostics)
		if descriptorErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, descriptorErr.Error(), nil)
			return
		}
		filename, content, exportErr := retailexport.ExportCSV(descriptor, retailexport.Envelope{
			Basis: result.Basis, DataClassification: result.DataClassification, DatasetVersion: result.DatasetVersion,
			PeriodLabel: fmt.Sprintf("%s ~ %s", result.Current.DateFrom, result.Current.DateTo), AsOf: result.Current.DateTo,
			FormulaVersion: result.FormulaVersion, SourceSystems: nil, // per-metric evidence sources are in the row payload GeneratedAt: result.GeneratedAt,
		}, DiagnosticsExportRows(result))
		if exportErr != nil {
			writeSystemFailure(c, http.StatusInternalServerError, exportErr)
			return
		}
		writeExportCSV(c, filename, content)
		return
	}
	c.JSON(http.StatusOK, result)
}

// PlFlow serves the SANKEY-001 phase-one profit-flow sankey for one store,
// with the same query contract as Diagnostics.
func (h *RetailStoreDiagnosticsHandler) PlFlow(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	storeID := strings.TrimSpace(c.Param("store_id"))
	if _, err := uuid.Parse(storeID); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "store_id must be a UUID", nil)
		return
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("as_of")))
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	windowDays := 14
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		windowDays, err = strconv.Atoi(raw)
		if err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
			return
		}
	}
	classification, datasetVersion, ok := parseRetailStore360Classification(c)
	if !ok {
		return
	}
	result, err := h.service.PlFlow(c.Request.Context(), retailstore360.Query{
		LegalEntityID: legalEntityID, StoreID: storeID, AsOf: asOf, WindowDays: windowDays,
		Classification: classification, DatasetVersion: datasetVersion,
		SourceSystem: strings.TrimSpace(c.Query("source_system")),
	})
	if err != nil {
		switch {
		case errors.Is(err, retailstore360.ErrInvalidQuery):
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		case errors.Is(err, retailstore360.ErrStoreNotFound):
			writeCodedError(c, http.StatusNotFound, errcontract.CodeNotFound, err.Error(), nil)
		case errors.Is(err, repository.ErrRetailKPISourceConflict):
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
		default:
			writeSystemFailure(c, http.StatusInternalServerError, err)
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

func parseRetailStore360Classification(c *gin.Context) (string, string, bool) {
	classification := strings.TrimSpace(c.Query("data_classification"))
	datasetVersion := strings.TrimSpace(c.Query("dataset_version"))
	if datasetVersion == "" {
		datasetVersion = strings.TrimSpace(c.Query("simulation_dataset_version"))
	}
	if classification != "production" && classification != "simulated" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "data_classification must be explicitly production or simulated", nil)
		return "", "", false
	}
	if classification == "simulated" && datasetVersion == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "dataset_version is required for simulated data", nil)
		return "", "", false
	}
	if classification == "production" && datasetVersion != "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "dataset_version is not allowed for production data", nil)
		return "", "", false
	}
	return classification, datasetVersion, true
}
