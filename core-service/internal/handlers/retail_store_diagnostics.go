package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal_entity_id is required"})
		return
	}
	classification, datasetVersion, ok := parseRetailStore360Classification(c)
	if !ok {
		return
	}
	stores, err := h.repo.ListStorePopulation(c.Request.Context(), legalEntityID, classification, datasetVersion, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal_entity_id is required"})
		return
	}
	storeID := strings.TrimSpace(c.Param("store_id"))
	if _, err := uuid.Parse(storeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "store_id must be a UUID"})
		return
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("as_of")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be an ISO date (YYYY-MM-DD)"})
		return
	}
	windowDays := 14
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		windowDays, err = strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "window_days must be one of 7, 14 or 28"})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, retailstore360.ErrStoreNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, repository.ErrRetailKPISourceConflict):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "reason": "source_conflict"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "data_classification must be explicitly production or simulated"})
		return "", "", false
	}
	if classification == "simulated" && datasetVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is required for simulated data"})
		return "", "", false
	}
	if classification == "production" && datasetVersion != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is not allowed for production data"})
		return "", "", false
	}
	return classification, datasetVersion, true
}
