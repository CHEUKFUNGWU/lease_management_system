package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/promotionattribution"
)

type PromotionHandler struct {
	repo *repository.PromotionRepository
}

func NewPromotionHandler(repo *repository.PromotionRepository) *PromotionHandler {
	return &PromotionHandler{repo: repo}
}

func (h *PromotionHandler) ListPromotions(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	status := c.Query("status")
	list, err := h.repo.ListPromotions(c.Request.Context(), tenantID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []repository.Promotion{}
	}
	c.JSON(http.StatusOK, gin.H{"promotions": list})
}

func (h *PromotionHandler) GetPromotion(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")
	p, err := h.repo.GetPromotion(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "promotion not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

type createPromotionReq struct {
	PromoCode      string   `json:"promo_code" binding:"required"`
	Name           string   `json:"name" binding:"required"`
	PromoType      string   `json:"promo_type" binding:"required"`
	StartDate      string   `json:"start_date" binding:"required"`
	EndDate        string   `json:"end_date" binding:"required"`
	TargetScope    string   `json:"target_scope"`
	ScopeValues    []string `json:"scope_values"`
	Currency       string   `json:"currency"`
	BudgetAmount   float64  `json:"budget_amount"`
	ApprovalStatus string   `json:"approval_status"`
	Owner          *string  `json:"owner"`
	Description    *string  `json:"description"`
}

func (h *PromotionHandler) CreatePromotion(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req createPromotionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TargetScope == "" {
		req.TargetScope = "all"
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}
	if req.ApprovalStatus == "" {
		req.ApprovalStatus = "draft"
	}
	if req.ScopeValues == nil {
		req.ScopeValues = []string{}
	}

	p := &repository.Promotion{
		LegalEntityID:  tenantID,
		PromoCode:      req.PromoCode,
		Name:           req.Name,
		PromoType:      req.PromoType,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		TargetScope:    req.TargetScope,
		ScopeValues:    req.ScopeValues,
		Currency:       req.Currency,
		BudgetAmount:   req.BudgetAmount,
		ApprovalStatus: req.ApprovalStatus,
		Owner:          req.Owner,
		Description:    req.Description,
	}

	if err := h.repo.CreatePromotion(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, p)
}

func (h *PromotionHandler) UpdatePromotion(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")
	var req createPromotionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p := &repository.Promotion{
		ID:             id,
		LegalEntityID:  tenantID,
		PromoCode:      req.PromoCode,
		Name:           req.Name,
		PromoType:      req.PromoType,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		TargetScope:    req.TargetScope,
		ScopeValues:    req.ScopeValues,
		Currency:       req.Currency,
		BudgetAmount:   req.BudgetAmount,
		ApprovalStatus: req.ApprovalStatus,
		Owner:          req.Owner,
		Description:    req.Description,
	}

	if err := h.repo.UpdatePromotion(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, p)
}

type addCostReq struct {
	Period       string  `json:"period" binding:"required"`
	CostCategory string  `json:"cost_category" binding:"required"`
	Amount       float64 `json:"amount" binding:"required"`
	Currency     string  `json:"currency"`
	Notes        *string `json:"notes"`
}

func (h *PromotionHandler) ListPromotionCosts(c *gin.Context) {
	promoID := c.Param("id")
	costs, err := h.repo.ListPromotionCosts(c.Request.Context(), promoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if costs == nil {
		costs = []repository.PromotionCost{}
	}
	c.JSON(http.StatusOK, gin.H{"costs": costs})
}

func (h *PromotionHandler) AddPromotionCost(c *gin.Context) {
	promoID := c.Param("id")
	var req addCostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}

	cost := &repository.PromotionCost{
		PromotionID:  promoID,
		Period:       req.Period,
		CostCategory: req.CostCategory,
		Amount:       req.Amount,
		Currency:     req.Currency,
		Notes:        req.Notes,
	}

	if err := h.repo.AddPromotionCost(c.Request.Context(), cost); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cost)
}

func (h *PromotionHandler) EvaluateROI(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	promoID := c.Param("id")

	p, err := h.repo.GetPromotion(c.Request.Context(), tenantID, promoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "promotion not found"})
		return
	}

	// 1. Fetch costs
	costs, err := h.repo.ListPromotionCosts(c.Request.Context(), p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. Fetch overlapping promotions
	overlaps, err := h.repo.GetOverlappingPromotions(c.Request.Context(), tenantID, p.ID, p.StartDate, p.EndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Fetch actual facts for promo window
	actuals, err := h.repo.GetPromotionActualFacts(c.Request.Context(), tenantID, p.StartDate, p.EndDate, p.ScopeValues)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. Compute baseline run-rate from actual facts or estimate
	// Calculate baseline from 14 days prior
	st, _ := time.Parse("2006-01-02", p.StartDate)
	baseStart := st.AddDate(0, 0, -14).Format("2006-01-02")
	baseEnd := st.AddDate(0, 0, -1).Format("2006-01-02")

	baseFacts, _ := h.repo.GetPromotionActualFacts(c.Request.Context(), tenantID, baseStart, baseEnd, p.ScopeValues)

	var baseDailyRev, baseDailyGP, baseDailyTx float64
	if len(baseFacts) > 0 {
		var bRev, bGP float64
		var bTx int
		bDays := make(map[string]struct{})
		for _, bf := range baseFacts {
			bRev += bf.Revenue
			bGP += bf.GrossProfit
			bTx += bf.Transactions
			bDays[bf.BusinessDate] = struct{}{}
		}
		dayCount := float64(len(bDays))
		if dayCount > 0 {
			baseDailyRev = bRev / dayCount
			baseDailyGP = bGP / dayCount
			baseDailyTx = float64(bTx) / dayCount
		}
	} else if len(actuals) > 0 {
		// Fallback: estimate run-rate as 85% of actual promo performance
		var aRev, aGP float64
		aDays := make(map[string]struct{})
		for _, af := range actuals {
			aRev += af.Revenue
			aGP += af.GrossProfit
			aDays[af.BusinessDate] = struct{}{}
		}
		dayCount := float64(len(aDays))
		if dayCount > 0 {
			baseDailyRev = (aRev / dayCount) * 0.85
			baseDailyGP = (aGP / dayCount) * 0.85
		}
	}

	baseline := promotionattribution.RunRate{
		DailyRevenue:      baseDailyRev,
		DailyGrossProfit:  baseDailyGP,
		DailyTransactions: baseDailyTx,
	}

	// 5. Convert promotion costs
	promoCosts := make([]promotionattribution.PromotionCost, len(costs))
	for i, c := range costs {
		promoCosts[i] = promotionattribution.PromotionCost{
			Category: c.CostCategory,
			Amount:   c.Amount,
			Currency: c.Currency,
			Period:   c.Period,
		}
	}

	// Convert overlaps
	overlapPromos := make([]promotionattribution.Promotion, len(overlaps))
	for i, o := range overlaps {
		overlapPromos[i] = promotionattribution.Promotion{
			ID:          o.ID,
			PromoCode:   o.PromoCode,
			Name:        o.Name,
			StartDate:   o.StartDate,
			EndDate:     o.EndDate,
			TargetScope: o.TargetScope,
		}
	}

	res := promotionattribution.Attribute(
		promotionattribution.Promotion{
			ID:             p.ID,
			LegalEntityID:  p.LegalEntityID,
			PromoCode:      p.PromoCode,
			Name:           p.Name,
			PromoType:      p.PromoType,
			StartDate:      p.StartDate,
			EndDate:        p.EndDate,
			TargetScope:    p.TargetScope,
			ScopeValues:    p.ScopeValues,
			Currency:       p.Currency,
			BudgetAmount:   p.BudgetAmount,
			ApprovalStatus: p.ApprovalStatus,
		},
		promoCosts,
		actuals,
		baseline,
		overlapPromos,
	)

	c.JSON(http.StatusOK, res)
}
