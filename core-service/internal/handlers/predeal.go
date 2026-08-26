package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/services/leasescenario"
)

// PreDealHandler prices a lease before it is signed. Nothing is read from or
// written to the ledger: the terms arrive with the request, which is the whole
// point — the question is asked before there is anything to record.
type PreDealHandler struct{}

func NewPreDealHandler() *PreDealHandler { return &PreDealHandler{} }

type BriefingRequest struct {
	Name                    string  `json:"name"`
	CommencementDate        string  `json:"commencement_date" binding:"required"`
	TermMonths              int     `json:"term_months" binding:"required"`
	MonthlyRent             float64 `json:"monthly_rent"`
	RentFreeMonths          int     `json:"rent_free_months"`
	AnnualEscalationPercent float64 `json:"annual_escalation_percent"`
	DiscountRate            float64 `json:"discount_rate"`
	Currency                string  `json:"currency"`
	InitialDirectCost       float64 `json:"initial_direct_cost"`
	EarlyExitPenaltyMonths  float64 `json:"early_exit_penalty_months"`
}

// Briefing produces the pre-signature decision briefing.
// POST /deals/briefing
func (h *PreDealHandler) Briefing(c *gin.Context) {
	var req BriefingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	commencement, err := time.Parse("2006-01-02", req.CommencementDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "起租日格式应为 YYYY-MM-DD"})
		return
	}

	briefing, err := leasescenario.Build(leasescenario.Draft{
		Name: req.Name, CommencementDate: commencement, TermMonths: req.TermMonths,
		MonthlyRent: req.MonthlyRent, RentFreeMonths: req.RentFreeMonths,
		AnnualEscalationPercent: req.AnnualEscalationPercent,
		DiscountRate:            req.DiscountRate, Currency: req.Currency,
		InitialDirectCost:      req.InitialDirectCost,
		EarlyExitPenaltyMonths: req.EarlyExitPenaltyMonths,
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, briefing)
}
