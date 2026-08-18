package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/cashplan"
)

type CashPlanHandler struct {
	repo *repository.CashPlanRepository
}

func NewCashPlanHandler(repo *repository.CashPlanRepository) *CashPlanHandler {
	return &CashPlanHandler{repo: repo}
}

type CashPlanComposeRequest struct {
	FromPeriod         string   `json:"from_period" binding:"required"`
	ToPeriod           string   `json:"to_period" binding:"required"`
	DataClassification string   `json:"data_classification"`
	DatasetVersion     string   `json:"dataset_version"`
	StoreIDs           []string `json:"store_ids"`
}

// Compose handles POST /cashflow/plan/compose
func (h *CashPlanHandler) Compose(c *gin.Context) {
	var req CashPlanComposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entityID := middleware.GetTenantID(c)
	if entityID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}

	if req.DataClassification == "" {
		req.DataClassification = "production"
	}

	planReq := cashplan.Request{
		LegalEntityID:      entityID,
		FromPeriod:         req.FromPeriod,
		ToPeriod:           req.ToPeriod,
		DataClassification: req.DataClassification,
		DatasetVersion:     req.DatasetVersion,
		StoreIDs:           req.StoreIDs,
	}

	sources := cashplan.Sources{
		Operating: h.repo,
		Lease:     h.repo,
		Capex:     h.repo,
	}

	result, err := cashplan.Compose(c.Request.Context(), planReq, sources)
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type ExchangeRateVersionHandler struct {
	repo *repository.ExchangeRateRepository
}

func NewExchangeRateVersionHandler(repo *repository.ExchangeRateRepository) *ExchangeRateVersionHandler {
	return &ExchangeRateVersionHandler{repo: repo}
}

// ListVersions handles GET /exchange-rates/versions
func (h *ExchangeRateVersionHandler) ListVersions(c *gin.Context) {
	versions, err := h.repo.ListVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

type CreateExchangeRateVersionRequest struct {
	Name          string `json:"name" binding:"required"`
	VersionType   string `json:"version_type" binding:"required"`
	EffectiveFrom string `json:"effective_from" binding:"required"`
	EffectiveTo   string `json:"effective_to"`
	Source        string `json:"source" binding:"required"`
	Status        string `json:"status"`
}

// CreateVersion handles POST /exchange-rates/versions
func (h *ExchangeRateVersionHandler) CreateVersion(c *gin.Context) {
	var req CreateExchangeRateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	effFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "effective_from must be YYYY-MM-DD"})
		return
	}
	var effTo *time.Time
	if req.EffectiveTo != "" {
		parsed, err := time.Parse("2006-01-02", req.EffectiveTo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "effective_to must be YYYY-MM-DD"})
			return
		}
		effTo = &parsed
	}

	rawUserID, _ := c.Get("user_id")
	userID, _ := rawUserID.(string)
	var userPtr *string
	if strings.TrimSpace(userID) != "" {
		userPtr = &userID
	}

	v := &repository.ExchangeRateVersion{
		Name:          req.Name,
		VersionType:   req.VersionType,
		EffectiveFrom: effFrom,
		EffectiveTo:   effTo,
		Source:        req.Source,
		Status:        req.Status,
		CreatedBy:     userPtr,
	}

	created, err := h.repo.CreateVersion(c.Request.Context(), v)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"version": created})
}
