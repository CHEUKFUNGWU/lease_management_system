package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

type ExchangeRateHandler struct {
	repo        *repository.ExchangeRateRepository
	auditLogger *audit.Logger
}

func NewExchangeRateHandler(repo *repository.ExchangeRateRepository, auditLogger *audit.Logger) *ExchangeRateHandler {
	return &ExchangeRateHandler{repo: repo, auditLogger: auditLogger}
}

type upsertExchangeRateRequest struct {
	FromCurrency string  `json:"from_currency" binding:"required"`
	ToCurrency   string  `json:"to_currency" binding:"required"`
	RateDate     string  `json:"rate_date" binding:"required"`
	RateType     string  `json:"rate_type"`
	Rate         float64 `json:"rate" binding:"required,gt=0"`
	Source       string  `json:"source"`
}

// List returns published rates, optionally filtered to one currency pair.
func (h *ExchangeRateHandler) List(c *gin.Context) {
	rates, err := h.repo.List(c.Request.Context(),
		strings.ToUpper(c.Query("from_currency")), strings.ToUpper(c.Query("to_currency")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rates, "total": len(rates)})
}

// Upsert publishes a rate, replacing any existing rate for the same pair, date
// and type. Re-publishing a corrected rate is audited like any other accounting
// input change.
func (h *ExchangeRateHandler) Upsert(c *gin.Context) {
	var req upsertExchangeRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写完整的汇率信息(币种、日期、汇率)"})
		return
	}
	rateDate, err := time.Parse("2006-01-02", req.RateDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "汇率日期格式应为 YYYY-MM-DD"})
		return
	}
	from := strings.ToUpper(strings.TrimSpace(req.FromCurrency))
	to := strings.ToUpper(strings.TrimSpace(req.ToCurrency))
	if from == to {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原币与目标币种不能相同"})
		return
	}
	rateType := req.RateType
	if rateType == "" {
		rateType = repository.RateTypeClosing
	}
	if rateType != repository.RateTypeClosing && rateType != repository.RateTypeAverage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "汇率类型只能是 closing(期末收盘价)或 average(当期平均价)"})
		return
	}

	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)
	var createdBy *string
	if uidStr != "" {
		createdBy = &uidStr
	}
	var source *string
	if trimmed := strings.TrimSpace(req.Source); trimmed != "" {
		source = &trimmed
	}

	saved, err := h.repo.Upsert(c.Request.Context(), &repository.ExchangeRate{
		FromCurrency: from, ToCurrency: to, RateDate: rateDate,
		RateType: rateType, Rate: req.Rate, Source: source, CreatedBy: createdBy,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "exchange_rates", saved.ID, "upsert", nil, saved, uidStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "汇率已保存", "data": saved})
}
