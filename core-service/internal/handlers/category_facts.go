package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/categoryreconciliation"
	"github.com/lease-management-system/core-service/internal/services/margindecomposition"
)

type CategoryHandler struct {
	repo *repository.CategoryRepository
}

func NewCategoryHandler(repo *repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

func (h *CategoryHandler) ListCategories(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	list, err := h.repo.ListCategories(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []repository.RetailCategory{}
	}
	c.JSON(http.StatusOK, gin.H{"categories": list})
}

type createCategoryReq struct {
	CategoryCode  string  `json:"category_code" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	ParentCode    *string `json:"parent_code"`
	EffectiveFrom string  `json:"effective_from" binding:"required"`
	EffectiveTo   *string `json:"effective_to"`
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req createCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cat := &repository.RetailCategory{
		LegalEntityID: tenantID,
		CategoryCode:  req.CategoryCode,
		Name:          req.Name,
		ParentCode:    req.ParentCode,
		EffectiveFrom: req.EffectiveFrom,
		EffectiveTo:   req.EffectiveTo,
	}

	if err := h.repo.UpsertCategory(c.Request.Context(), cat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cat)
}

func (h *CategoryHandler) ListCategoryFacts(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	storeID := c.Query("store_id")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")
	classification := c.DefaultQuery("data_classification", "production")

	filter := repository.CategoryFactFilter{
		LegalEntityID:      tenantID,
		FromDate:           fromDate,
		ToDate:             toDate,
		DataClassification: classification,
	}
	if strings.TrimSpace(storeID) != "" {
		filter.StoreIDs = []string{storeID}
	}

	facts, err := h.repo.ListCategoryFacts(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if facts == nil {
		facts = []repository.RetailStoreDayCategoryFact{}
	}

	c.JSON(http.StatusOK, gin.H{"facts": facts})
}

type reconcileCategoryReq struct {
	StoreIDs           []string `json:"store_ids"`
	FromDate           string   `json:"from_date"`
	ToDate             string   `json:"to_date"`
	DataClassification string   `json:"data_classification"`
}

func (h *CategoryHandler) ReconcileCategoryFacts(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req reconcileCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DataClassification == "" {
		req.DataClassification = "production"
	}

	details, summaries, err := h.repo.GetCategoryReconciliationData(
		c.Request.Context(), tenantID, req.StoreIDs, req.FromDate, req.ToDate, req.DataClassification,
	)
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	res := categoryreconciliation.Reconcile(details, summaries, categoryreconciliation.DefaultTolerance())
	c.JSON(http.StatusOK, res)
}

func (h *CategoryHandler) GetMarginDecomposition(c *gin.Context) {
	var req margindecomposition.DecompositionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Currency == "" {
		req.Currency = "CNY"
	}

	res := margindecomposition.Decompose(req)
	c.JSON(http.StatusOK, res)
}
