package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type retailSimulationStore interface {
	Generate(ctx context.Context, legalEntityID string, createdBy *string, idempotencyKey, payloadSHA256 string, plan *retailsimulation.Plan) (*repository.RetailSimulationGenerateResult, error)
}

type retailSimulationLatestStore interface {
	LatestCompleted(ctx context.Context, legalEntityID string) (*repository.RetailSimulationDataset, error)
}

type RetailSimulationHandler struct {
	repo  retailSimulationStore
	audit *audit.Logger
}

func NewRetailSimulationHandler(repo retailSimulationStore, auditLogger *audit.Logger) *RetailSimulationHandler {
	return &RetailSimulationHandler{repo: repo, audit: auditLogger}
}

type retailSimulationRequest struct {
	Seed       *int64  `json:"seed"`
	DateFrom   *string `json:"date_from"`
	DateTo     *string `json:"date_to"`
	StoreCount *int    `json:"store_count"`
}

func (h *RetailSimulationHandler) GenerateStoreDays(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "tenant legal entity is required; global admin context cannot generate a dataset", nil)
		return
	}
	var request retailSimulationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "invalid JSON request: "+err.Error(), nil)
		return
	}
	input := retailsimulation.Input{}
	if request.Seed != nil {
		input.Seed = *request.Seed
	}
	if request.DateFrom != nil {
		input.DateFrom = *request.DateFrom
	}
	if request.DateTo != nil {
		input.DateTo = *request.DateTo
	}
	if request.StoreCount != nil {
		input.StoreCount = *request.StoreCount
	}
	payloadSHA256, normalized, err := retailsimulation.PayloadSHA256(legalEntityID, input)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	plan, err := retailsimulation.Build(legalEntityID, normalized)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idempotencyKey) > 255 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "Idempotency-Key exceeds 255 characters", nil)
		return
	}
	userID := userIDFromContext(c)
	result, err := h.repo.Generate(c.Request.Context(), legalEntityID, optionalString(userID), idempotencyKey, payloadSHA256, plan)
	if err != nil {
		if errors.Is(err, repository.ErrRetailSimulationIdempotencyConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), nil)
			return
		}
		writeSystemFailure(c, http.StatusUnprocessableEntity, err)
		return
	}
	if h.audit != nil && !result.Replayed {
		_ = h.audit.Log(c.Request.Context(), "retail_simulation_datasets", result.Dataset.ID, "generate", nil, result.Dataset, userID, c)
	}
	response := gin.H{
		"basis": "Working", "data_classification": "simulated", "source_system": "retail_simulator",
		"source":          gin.H{"type": "retail_simulator", "grain": "store_day", "dataset_id": result.Dataset.ID, "created_at": result.Dataset.CreatedAt},
		"dataset_id":      result.Dataset.ID,
		"dataset_version": result.Dataset.DatasetVersion, "generator_version": result.Dataset.GeneratorVersion,
		"seed": result.Dataset.Seed, "date_from": result.Dataset.DateFrom, "date_to": result.Dataset.DateTo,
		"store_count": result.Dataset.StoreCount, "fact_count": result.Dataset.FactCount,
		"parameters": result.Dataset.Parameters, "anomaly_manifest": result.Dataset.AnomalyManifest,
		"payload_sha256": result.Dataset.PayloadSHA256, "business_sha256": result.Dataset.BusinessSHA256,
		"import_batch_id": result.Dataset.ImportBatchID, "idempotent_replay": result.Replayed,
	}
	c.JSON(http.StatusOK, response)
}

// LatestStoreDays is a stable, read-only discovery envelope for the newest
// completed simulated dataset in the current tenant.  A missing dataset is a
// normal first-run state, not a 404 page.
func (h *RetailSimulationHandler) LatestStoreDays(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "tenant legal entity is required", nil)
		return
	}
	latestStore, ok := h.repo.(retailSimulationLatestStore)
	if !ok {
		writeCodedError(c, http.StatusInternalServerError, errcontract.CodeSystemFailure, "latest simulation dataset reader is unavailable", nil)
		return
	}
	dataset, err := latestStore.LatestCompleted(c.Request.Context(), legalEntityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusOK, retailSimulationLatestEnvelope(nil))
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, retailSimulationLatestEnvelope(dataset))
}

func retailSimulationLatestEnvelope(dataset *repository.RetailSimulationDataset) gin.H {
	response := gin.H{
		"basis":               "Working",
		"data_classification": "simulated",
		"source_system":       "retail_simulator",
		"data":                nil,
	}
	if dataset == nil {
		return response
	}
	response["data"] = gin.H{
		"id":                dataset.ID,
		"dataset_version":   dataset.DatasetVersion,
		"generator_version": dataset.GeneratorVersion,
		"seed":              dataset.Seed,
		"date_from":         dataset.DateFrom,
		"date_to":           dataset.DateTo,
		"store_count":       dataset.StoreCount,
		"fact_count":        dataset.FactCount,
		"status":            dataset.Status,
		"anomaly_manifest":  dataset.AnomalyManifest,
		"completed_at":      dataset.CompletedAt,
		"created_at":        dataset.CreatedAt,
	}
	return response
}
