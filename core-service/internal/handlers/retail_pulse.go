package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
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
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	asOfText := strings.TrimSpace(c.Query("as_of"))
	asOf, err := time.Parse("2006-01-02", asOfText)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	windowDays := retailpulse.DefaultWindowDays
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
			return
		}
		windowDays = parsed
	}
	attentionLimit := retailpulse.DefaultAttentionLimit
	if raw := strings.TrimSpace(c.Query("attention_limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "attention_limit must be an integer between 1 and 50", nil)
			return
		}
		attentionLimit = parsed
	}
	if windowDays < 7 || windowDays > 28 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
		return
	}
	if attentionLimit < 1 || attentionLimit > 50 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "attention_limit must be an integer between 1 and 50", nil)
		return
	}
	classification := strings.TrimSpace(c.Query("data_classification"))
	datasetVersion := strings.TrimSpace(c.Query("dataset_version"))
	if datasetVersion == "" {
		datasetVersion = strings.TrimSpace(c.Query("simulation_dataset_version"))
	}
	if classification != "production" && classification != "simulated" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "data_classification must be explicitly production or simulated", nil)
		return
	}
	if classification == "simulated" && datasetVersion == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "dataset_version is required for simulated data", nil)
		return
	}
	if classification == "production" && datasetVersion != "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "dataset_version is not allowed for production data", nil)
		return
	}
	storeIDs, storeErr := parseRetailKPIStoreIDs(c.QueryArray("store_id"))
	if storeErr != "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, storeErr, nil)
		return
	}
	result, err := h.service.Build(c.Request.Context(), retailpulse.Query{LegalEntityID: legalEntityID, AsOf: asOf, WindowDays: windowDays, Classification: classification, DatasetVersion: datasetVersion, SourceSystem: strings.TrimSpace(c.Query("source_system")), StoreIDs: storeIDs, AttentionLimit: attentionLimit})
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
