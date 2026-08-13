package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
)

type RetailPulseHandler struct {
	service *retailpulse.Service
}

func NewRetailPulseHandler(reader retailKPIReader) *RetailPulseHandler {
	return &RetailPulseHandler{service: retailpulse.NewService(reader)}
}

func (h *RetailPulseHandler) OperatingPulse(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "legal_entity_id is required"})
		return
	}
	asOfText := strings.TrimSpace(c.Query("as_of"))
	asOf, err := time.Parse("2006-01-02", asOfText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be an ISO date (YYYY-MM-DD)"})
		return
	}
	windowDays := retailpulse.DefaultWindowDays
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "window_days must be an integer between 7 and 28"})
			return
		}
		windowDays = parsed
	}
	attentionLimit := retailpulse.DefaultAttentionLimit
	if raw := strings.TrimSpace(c.Query("attention_limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "attention_limit must be an integer between 1 and 50"})
			return
		}
		attentionLimit = parsed
	}
	if windowDays < 7 || windowDays > 28 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window_days must be an integer between 7 and 28"})
		return
	}
	if attentionLimit < 1 || attentionLimit > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "attention_limit must be an integer between 1 and 50"})
		return
	}
	classification := strings.TrimSpace(c.Query("data_classification"))
	datasetVersion := strings.TrimSpace(c.Query("dataset_version"))
	if datasetVersion == "" {
		datasetVersion = strings.TrimSpace(c.Query("simulation_dataset_version"))
	}
	if classification != "production" && classification != "simulated" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data_classification must be explicitly production or simulated"})
		return
	}
	if classification == "simulated" && datasetVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is required for simulated data"})
		return
	}
	if classification == "production" && datasetVersion != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dataset_version is not allowed for production data"})
		return
	}
	storeIDs, storeErr := parseRetailKPIStoreIDs(c.QueryArray("store_id"))
	if storeErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": storeErr})
		return
	}
	result, err := h.service.Build(c.Request.Context(), retailpulse.Query{LegalEntityID: legalEntityID, AsOf: asOf, WindowDays: windowDays, Classification: classification, DatasetVersion: datasetVersion, SourceSystem: strings.TrimSpace(c.Query("source_system")), StoreIDs: storeIDs, AttentionLimit: attentionLimit})
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
