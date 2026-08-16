package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailexport"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
)

type RetailPulseHandler struct {
	service            *retailpulse.Service
	planMaterialityPct func(context.Context) float64
}

func NewRetailPulseHandler(reader retailKPIReader) *RetailPulseHandler {
	return &RetailPulseHandler{service: retailpulse.NewService(reader), planMaterialityPct: func(context.Context) float64 { return 5 }}
}

// WithPlanReader enables the M4 actual-vs-plan block on every pulse read.
func (h *RetailPulseHandler) WithPlanReader(reader retailkpi.PlanReader) *RetailPulseHandler {
	h.service.WithPlanReader(reader)
	return h
}

// WithPlanMateriality sets the materiality source (system settings via the
// caller); the fallback stays 5% when unset.
func (h *RetailPulseHandler) WithPlanMateriality(getter func(context.Context) float64) *RetailPulseHandler {
	if getter != nil {
		h.planMaterialityPct = getter
	}
	return h
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
	periodSpec := strings.TrimSpace(c.Query("period"))
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		if periodSpec != "" {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "period and window_days are mutually exclusive", nil)
			return
		}
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
	if periodSpec == "" && (windowDays < 7 || windowDays > 28) {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
		return
	}
	if attentionLimit < 1 || attentionLimit > 50 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "attention_limit must be an integer between 1 and 50", nil)
		return
	}
	// M2: a period spec (rolling days, YYYY-MM, YYYY-Qn, last-month,
	// this-quarter) resolves through the shared period module; calendar
	// kinds override the rolling derivation with explicit boundaries.
	var calendarWindow *retailperiod.Window
	if periodSpec != "" {
		window, periodErr := retailperiod.Parse(periodSpec, asOf)
		if periodErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, periodErr.Error(), nil)
			return
		}
		if window.Period.Kind == retailperiod.KindRolling {
			windowDays = window.Period.Days
		} else {
			calendarWindow = &window
		}
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
	pulseQuery := retailpulse.Query{LegalEntityID: legalEntityID, AsOf: asOf, WindowDays: windowDays, Classification: classification, DatasetVersion: datasetVersion, SourceSystem: strings.TrimSpace(c.Query("source_system")), StoreIDs: storeIDs, AttentionLimit: attentionLimit, GroupBy: strings.TrimSpace(c.Query("group_by")), PlanComparison: true, PlanMaterialityThresholdPct: h.planMaterialityPct(c.Request.Context())}
	if calendarWindow != nil {
		pulseQuery.DateFrom, pulseQuery.DateTo = calendarWindow.From, calendarWindow.To
		pulseQuery.ComparisonDateFrom, pulseQuery.ComparisonDateTo = calendarWindow.CompareFrom, calendarWindow.CompareTo
		pulseQuery.PeriodLabel = calendarWindow.Label
	}
	result, err := h.service.Build(c.Request.Context(), pulseQuery)
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(c.Query("format")) == "csv" {
		descriptor, descriptorErr := retailexport.Descriptor(retailexport.KindOperatingPulse)
		if descriptorErr != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, descriptorErr.Error(), nil)
			return
		}
		filename, content, exportErr := retailexport.ExportCSV(descriptor, retailexport.Envelope{
			Basis: result.Basis, DataClassification: result.DataClassification, DatasetVersion: result.DatasetVersion,
			PeriodLabel: result.PeriodLabel, AsOf: result.Current.DateTo, FormulaVersion: retailkpi.FormulaVersion,
			SourceSystems: result.SourceSystems, GeneratedAt: result.GeneratedAt,
		}, PulseExportRows(result))
		if exportErr != nil {
			writeSystemFailure(c, http.StatusInternalServerError, exportErr)
			return
		}
		writeExportCSV(c, filename, content)
		return
	}
	c.JSON(http.StatusOK, result)
}
