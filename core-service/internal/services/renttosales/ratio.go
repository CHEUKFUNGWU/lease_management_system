// Package renttosales turns rent and sales into the ratio a retail lease
// decision actually turns on.
//
// The measure is simple arithmetic; what earns its keep is refusing to state it
// when the inputs do not support it. Three cases are kept apart, because
// collapsing them is how a report ends up lying:
//
//   - No sales reported. The ratio is unknown. It is not zero, and the store is
//     not healthy by default.
//   - Sales reported as zero. The ratio is undefined, and the store is in
//     trouble in a way no percentage conveys.
//   - Sales and rent in different currencies. The division is meaningless, and
//     translating it here would invent a rate nobody agreed to.
package renttosales

import (
	"fmt"
	"math"
	"sort"
)

// Status of one store's ratio.
const (
	StatusHealthy       = "healthy"
	StatusWatch         = "watch"
	StatusOverThreshold = "over_threshold"
	StatusNoRevenue     = "no_revenue"
	StatusZeroRevenue   = "zero_revenue"
	StatusCurrencyClash = "currency_mismatch"
	StatusNoRent        = "no_rent"
)

// StoreInput is one store's rent and, if it was reported, its sales.
type StoreInput struct {
	StoreID   string
	StoreCode string
	StoreName string
	Brand     string
	Region    string

	// CashRent is nil when no approved rent schedule covers the period. That
	// is different from a store paying no rent, and the report keeps the two
	// apart (the same rule Revenue already follows).
	CashRent     *float64
	RentCurrency string

	Revenue         *float64
	RevenueCurrency string
	RevenueVersion  *int
	RevenueSource   string

	AreaSqm *float64
}

// StoreRatio is one store's answer.
type StoreRatio struct {
	StoreID   string `json:"store_id"`
	StoreCode string `json:"store_code"`
	StoreName string `json:"store_name"`
	Brand     string `json:"brand"`
	Region    string `json:"region"`

	CashRent     float64  `json:"cash_rent"`
	Revenue      *float64 `json:"revenue"`
	Currency     string   `json:"currency"`
	SalesPerSqm  *float64 `json:"sales_per_sqm"`
	RentToSales  *float64 `json:"rent_to_sales_percent"`
	Status       string   `json:"status"`
	StatusReason string   `json:"status_reason"`

	RevenueVersion *int   `json:"revenue_version"`
	RevenueSource  string `json:"revenue_source"`
}

// Result is the period's picture.
type Result struct {
	Period          string       `json:"period"`
	HealthyCeiling  float64      `json:"healthy_ceiling_percent"`
	WarningCeiling  float64      `json:"warning_ceiling_percent"`
	Stores          []StoreRatio `json:"stores"`
	StoresOverLine  int          `json:"stores_over_line"`
	StoresNoRevenue int          `json:"stores_without_revenue"`

	// PortfolioRatio is stated only when every included store shares one
	// currency and reported sales. A portfolio ratio computed over a partial
	// set reads as if it covered everything.
	PortfolioRatio    *float64 `json:"portfolio_rent_to_sales_percent"`
	PortfolioCaveat   string   `json:"portfolio_caveat"`
	CoverageStatement string   `json:"coverage_statement"`
}

// Input is a period's rows and the thresholds to judge them by.
type Input struct {
	Period         string
	HealthyCeiling float64
	WarningCeiling float64
	Stores         []StoreInput
}

// Calculate produces the ratios and the warnings.
func Calculate(input Input) (Result, error) {
	if input.Period == "" {
		return Result{}, fmt.Errorf("请指定期间")
	}
	healthy := input.HealthyCeiling
	warning := input.WarningCeiling
	if healthy <= 0 || warning <= 0 {
		return Result{}, fmt.Errorf("租售比健康线和预警线必须通过政策配置提供")
	}
	if warning < healthy {
		return Result{}, fmt.Errorf("预警线 %.2f%% 低于健康线 %.2f%%", warning, healthy)
	}

	result := Result{
		Period:         input.Period,
		HealthyCeiling: healthy,
		WarningCeiling: warning,
		Stores:         make([]StoreRatio, 0, len(input.Stores)),
	}

	var ratedRent, ratedRevenue float64
	ratedCurrency := ""
	portfolioUsable := true

	for _, store := range input.Stores {
		ratio := StoreRatio{
			StoreID: store.StoreID, StoreCode: store.StoreCode, StoreName: store.StoreName,
			Brand: store.Brand, Region: store.Region,
			Revenue: store.Revenue,
			Currency: store.RentCurrency, RevenueVersion: store.RevenueVersion,
			RevenueSource: store.RevenueSource,
		}
		if store.CashRent != nil {
			ratio.CashRent = round2(*store.CashRent)
		}

		switch {
		case store.Revenue == nil:
			ratio.Status = StatusNoRevenue
			ratio.StatusReason = "尚未上传该期间营收"
			result.StoresNoRevenue++
			portfolioUsable = false

		case store.CashRent == nil:
			// 门店在期间内没有已审批的租金付款计划：租金未知，不是零租金。
			ratio.Status = StatusNoRent
			ratio.StatusReason = "该期间未匹配到已审批的租金付款计划，租金未知（并非零租金）"
			portfolioUsable = false

		case store.RentCurrency != "" && store.RevenueCurrency != "" &&
			store.RentCurrency != store.RevenueCurrency:
			// Dividing one currency by another is not a ratio. Translating it
			// here would put a rate into a business measure that nobody agreed.
			ratio.Status = StatusCurrencyClash
			ratio.StatusReason = fmt.Sprintf("租金以 %s 计价、营收以 %s 计价，口径不一致无法计算",
				store.RentCurrency, store.RevenueCurrency)
			portfolioUsable = false

		case *store.Revenue == 0:
			ratio.Status = StatusZeroRevenue
			ratio.StatusReason = "该期间营收为零，租售比无意义"
			portfolioUsable = false

		default:
			percent := round2(*store.CashRent / *store.Revenue * 100)
			ratio.RentToSales = &percent
			switch {
			case percent > warning:
				ratio.Status = StatusOverThreshold
				ratio.StatusReason = fmt.Sprintf("超过预警线 %.1f%%", warning)
				result.StoresOverLine++
			case percent > healthy:
				ratio.Status = StatusWatch
				ratio.StatusReason = fmt.Sprintf("高于健康线 %.1f%%，未超预警线", healthy)
			default:
				ratio.Status = StatusHealthy
			}

			ratedRent += *store.CashRent
			ratedRevenue += *store.Revenue
			if ratedCurrency == "" {
				ratedCurrency = store.RentCurrency
			} else if ratedCurrency != store.RentCurrency {
				portfolioUsable = false
			}

			if store.AreaSqm != nil && *store.AreaSqm > 0 {
				perSqm := round2(*store.Revenue / *store.AreaSqm)
				ratio.SalesPerSqm = &perSqm
			}
		}

		result.Stores = append(result.Stores, ratio)
	}

	// Worst first: that is the order the list gets read in. Stores without an
	// answer sort last, since there is nothing to act on yet.
	sort.SliceStable(result.Stores, func(i, j int) bool {
		left, right := result.Stores[i].RentToSales, result.Stores[j].RentToSales
		if left == nil && right == nil {
			return result.Stores[i].StoreCode < result.Stores[j].StoreCode
		}
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return *left > *right
	})

	if portfolioUsable && ratedRevenue > 0 {
		portfolio := round2(ratedRent / ratedRevenue * 100)
		result.PortfolioRatio = &portfolio
	} else {
		result.PortfolioCaveat = "部分门店缺营收、营收为零或币种不一致，组合级租售比无法代表全部门店，故不给出"
	}

	rated := len(result.Stores) - result.StoresNoRevenue
	result.CoverageStatement = fmt.Sprintf(
		"本期 %d 家门店中 %d 家已报营收；分析基于客户提供的营收口径，权威源为客户的 POS/ERP/BI 系统。",
		len(result.Stores), rated)

	return result, nil
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
