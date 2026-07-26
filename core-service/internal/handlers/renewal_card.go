package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/dealcompare"
	"github.com/lease-management-system/core-service/internal/services/renttosales"
)

// RenewalCardHandler turns a critical date from a reminder into a decision.
// Knowing a lease expires in ninety days is not the hard part; deciding whether
// to renew it, and on what terms, is.
type RenewalCardHandler struct {
	contractRepo     *repository.ContractRepository
	psRepo           *repository.PaymentScheduleRepository
	storeMetricsRepo *repository.StoreMetricsRepository
}

func NewRenewalCardHandler(
	contractRepo *repository.ContractRepository,
	psRepo *repository.PaymentScheduleRepository,
	storeMetricsRepo *repository.StoreMetricsRepository,
) *RenewalCardHandler {
	return &RenewalCardHandler{contractRepo: contractRepo, psRepo: psRepo, storeMetricsRepo: storeMetricsRepo}
}

// Card returns everything needed to decide a renewal.
// GET /contracts/:id/renewal-card?renewal_term_months=36&uplift_percent=5&discount_rate=0.05
func (h *RenewalCardHandler) Card(c *gin.Context) {
	contractID := c.Param("id")
	ctx := c.Request.Context()

	contract, err := h.contractRepo.GetByID(ctx, contractID, middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)

	asOf := time.Now()
	var remainingCommitment, lastRent float64
	for _, payment := range payments {
		if payment.Date.Before(asOf) || payment.Type == "variable" {
			continue
		}
		remainingCommitment += payment.Amount
		if payment.Amount > 0 {
			lastRent = payment.Amount
		}
	}

	renewalMonths := intQuery(c, "renewal_term_months", 36)
	uplift := floatQuery(c, "uplift_percent", 5)
	discountRate := floatQuery(c, "discount_rate", 0)
	if discountRate <= 0 && contract.DiscountRateValue != nil {
		discountRate = *contract.DiscountRateValue
	}
	if discountRate <= 0 {
		// The comparison is a present-value comparison, so a rate is required
		// rather than assumed — the same rule the rest of the product follows.
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":                 "该合同尚未确认折现率，无法进行续租现值对比",
			"discount_rate_missing": true,
		})
		return
	}

	response := gin.H{
		"contract_id":          contract.ID,
		"contract_number":      contract.ContractNumber,
		"contract_name":        contract.ContractName,
		"currency":             contract.Currency,
		"lease_end_date":       contract.LeaseEndDate.Format("2006-01-02"),
		"days_to_expiry":       int(contract.LeaseEndDate.Sub(asOf).Hours() / 24),
		"remaining_commitment": round2Handler(remainingCommitment),
		"discount_rate":        discountRate,
	}

	// The comparison worth putting in front of a negotiator is not "renew
	// versus walk away" — walking away always costs less in rent, which tells
	// nobody anything. It is what the landlord's asking uplift costs against
	// holding the current rent, because that is the number being negotiated.
	if lastRent > 0 && renewalMonths > 0 {
		comparison, err := dealcompare.Compare(dealcompare.Input{
			DiscountRate: discountRate,
			Currency:     contract.Currency,
			Offers: []dealcompare.Offer{
				{
					Name: "按现租金续租", TermMonths: renewalMonths,
					BaseMonthlyRent: lastRent,
					AreaSqm:         valueOrZero(contract.AreaSqm),
				},
				{
					Name: fmt.Sprintf("按上浮 %.1f%% 续租", uplift), TermMonths: renewalMonths,
					BaseMonthlyRent: lastRent * (1 + uplift/100),
					AreaSqm:         valueOrZero(contract.AreaSqm),
				},
			},
		})
		if err == nil {
			response["renewal_comparison"] = comparison
			response["current_monthly_rent"] = round2Handler(lastRent)
			response["assumed_renewal_rent"] = round2Handler(lastRent * (1 + uplift/100))
			// What the uplift costs over the renewal term, in cash.
			response["uplift_cost_over_term"] = round2Handler(
				lastRent * (uplift / 100) * float64(renewalMonths))
			response["renewal_term_months"] = renewalMonths
			response["uplift_percent"] = uplift
		}
	}

	// The store's trading is what makes the decision a business one rather than
	// an accounting one. It is optional: many contracts have no store, and many
	// stores have no revenue reported.
	if contract.StoreID != nil {
		if health := h.storeHealth(ctx, *contract.StoreID, contract, lastRent); health != nil {
			response["store_health"] = health
		}
	}

	c.JSON(http.StatusOK, response)
}

// storeHealth pairs the store's most recent reported revenue with its rent.
func (h *RenewalCardHandler) storeHealth(ctx context.Context, storeID string, contract *repository.Contract, monthlyRent float64) gin.H {
	metrics, err := h.storeMetricsRepo.List(ctx, "", "", storeID)
	if err != nil || len(metrics) == 0 {
		return nil
	}
	latest := metrics[0]

	result, err := renttosales.Calculate(renttosales.Input{
		Period: latest.Period,
		Stores: []renttosales.StoreInput{{
			StoreID: storeID, StoreCode: latest.StoreCode, StoreName: latest.StoreName,
			CashRent: monthlyRent, RentCurrency: contract.Currency,
			Revenue: &latest.Revenue, RevenueCurrency: latest.Currency,
			RevenueSource: latest.Source, AreaSqm: contract.AreaSqm,
		}},
	})
	if err != nil || len(result.Stores) == 0 {
		return nil
	}
	store := result.Stores[0]

	return gin.H{
		"store_name":            store.StoreName,
		"period":                latest.Period,
		"revenue":               latest.Revenue,
		"rent_to_sales_percent": store.RentToSales,
		"sales_per_sqm":         store.SalesPerSqm,
		"status":                store.Status,
		"status_reason":         store.StatusReason,
		"revenue_source":        latest.Source,
	}
}

func intQuery(c *gin.Context, name string, fallback int) int {
	if value, err := strconv.Atoi(c.Query(name)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func floatQuery(c *gin.Context, name string, fallback float64) float64 {
	if value, err := strconv.ParseFloat(c.Query(name), 64); err == nil {
		return value
	}
	return fallback
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func round2Handler(value float64) float64 {
	return math.Round(value*100) / 100
}
