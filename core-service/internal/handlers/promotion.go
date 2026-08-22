package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
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
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. Compute baseline run-rate from actual facts or estimate
	// Calculate baseline from 14 days prior
	st, _ := time.Parse("2006-01-02", p.StartDate)
	baseStart := st.AddDate(0, 0, -14).Format("2006-01-02")
	baseEnd := st.AddDate(0, 0, -1).Format("2006-01-02")

	baseFacts, err := h.repo.GetPromotionActualFacts(c.Request.Context(), tenantID, baseStart, baseEnd, p.ScopeValues)
	if err != nil {
		if errors.Is(err, repository.ErrRetailKPISourceConflict) {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("query baseline facts: %v", err)})
		return
	}

	var baseDailyRev, baseDailyGP, baseDailyTx float64
	baselineUnavailable := len(baseFacts) == 0
	if !baselineUnavailable {
		var bRev, bGP float64
		var bTx int
		bDays := make(map[string]struct{})
		for _, bf := range baseFacts {
			// NULL 字段不参与基线加总（缺失不等同于零销售）；完整字段数不足以
			// 支撑日均值时按基线缺失处理，不编造数字。
			if bf.Revenue != nil {
				bRev += *bf.Revenue
			}
			if bf.GrossProfit != nil {
				bGP += *bf.GrossProfit
			}
			if bf.Transactions != nil {
				bTx += *bf.Transactions
			}
			bDays[bf.BusinessDate] = struct{}{}
		}
		dayCount := float64(len(bDays))
		if dayCount == 0 {
			baselineUnavailable = true
		} else {
			baseDailyRev = bRev / dayCount
			baseDailyGP = bGP / dayCount
			baseDailyTx = float64(bTx) / dayCount
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

	if baselineUnavailable {
		// 基线缺失：不得用 0 或估算值充当基线，ROI 与增量数字不可作为决策依据。
		res.BaselineUnavailable = true
		res.ROI = nil
		res.Disclaimers = append(res.Disclaimers, "活动前 14 天无可用经营事实，未估算基线；增量销售、增量毛利与 ROI 不可作为决策依据。")
	}

	c.JSON(http.StatusOK, res)
}

// ─── RH3 投前保本（R2-1）─────────────────────────────────────────────

type breakevenReq struct {
	PromoID              string   `json:"promo_id"`               // 可选：给了则基线从活动前 14 天事实算（与投后复盘同一取数路径）
	Currency             string   `json:"currency"`
	EventDays            int      `json:"event_days"`             // 手输模式必填；promo 模式从活动日期推导
	BaselineDailyRevenue *float64 `json:"baseline_daily_revenue"` // 手输模式必填
	BaselineMarginRate   *float64 `json:"baseline_margin_rate"`   // 两模式都必填（0-1）
	PromoMarginRate      *float64 `json:"promo_margin_rate"`      // 必填（≤0 合法，走 unachievable）
	FixedMarketingCost   float64  `json:"fixed_marketing_cost"`
}

// EvaluateBreakeven POST /api/v1/retail/promotions/breakeven
// 投前测算：这次活动至少要多卖多少才不亏。纯计算不落库；
// promo_id 模式经租户隔离的仓库取数（跨法人 promo_id 在 GetPromotion 即 404）。
func (h *PromotionHandler) EvaluateBreakeven(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req breakevenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.PromoMarginRate == nil || req.BaselineMarginRate == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "promo_margin_rate 与 baseline_margin_rate 为必填"})
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	in := promotionattribution.BreakevenInput{
		Currency:           currency,
		EventDays:          req.EventDays,
		BaselineMarginRate: *req.BaselineMarginRate,
		PromoMarginRate:    *req.PromoMarginRate,
		FixedMarketingCost: req.FixedMarketingCost,
	}

	if req.PromoID != "" {
		p, err := h.repo.GetPromotion(c.Request.Context(), tenantID, req.PromoID)
		if err != nil {
			// 跨法人或不存在：一律 not found，不区分两种情况。
			c.JSON(http.StatusNotFound, gin.H{"error": "promotion not found"})
			return
		}
		st, err1 := time.Parse("2006-01-02", p.StartDate)
		et, err2 := time.Parse("2006-01-02", p.EndDate)
		if err1 != nil || err2 != nil || et.Before(st) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "promotion dates invalid"})
			return
		}
		in.EventDays = int(et.Sub(st).Hours()/24) + 1
		currency = p.Currency

		// 基线：与 EvaluateROI 同一窗口、同一取数、同一日均值算式。
		baseStart := st.AddDate(0, 0, -14).Format("2006-01-02")
		baseEnd := st.AddDate(0, 0, -1).Format("2006-01-02")
		baseFacts, err := h.repo.GetPromotionActualFacts(c.Request.Context(), tenantID, baseStart, baseEnd, p.ScopeValues)
		if err != nil {
			if errors.Is(err, repository.ErrRetailKPISourceConflict) {
				writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("query baseline facts: %v", err)})
			return
		}
		if len(baseFacts) == 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "活动前 14 天没有可用经营事实，无法建立基线。请改用手输模式提供基线销售额。"})
			return
		}
		var bRev, bGP float64
		bDays := make(map[string]struct{})
		for _, bf := range baseFacts {
			if bf.Revenue != nil {
				bRev += *bf.Revenue
			}
			if bf.GrossProfit != nil {
				bGP += *bf.GrossProfit
			}
			bDays[bf.BusinessDate] = struct{}{}
		}
		dayCount := float64(len(bDays))
		baseline := promotionattribution.RunRate{DailyRevenue: bRev / dayCount, DailyGrossProfit: bGP / dayCount}
		in.Baseline = baseline
		if baseline.DailyRevenue > 0 {
			rate := baseline.DailyGrossProfit / baseline.DailyRevenue
			in.BaselineMarginRate = rate
		}
	} else {
		if req.BaselineDailyRevenue == nil || req.EventDays <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "手输模式需要 baseline_daily_revenue 与正数 event_days；已有活动可传 promo_id 自动取基线"})
			return
		}
		in.Baseline = promotionattribution.RunRate{DailyRevenue: *req.BaselineDailyRevenue}
	}

	res := promotionattribution.Breakeven(in)
	res.Currency = currency
	c.JSON(http.StatusOK, res)
}
