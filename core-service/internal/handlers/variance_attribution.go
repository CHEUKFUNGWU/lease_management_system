package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/varianceattribution"
)

// RH5 利润差异归因端点（R2-3）。
//
// GET /api/v1/retail/store-variance-attribution
//
// 取数走 retailKPIReader.QueryFacts——租户边界在仓库层（legal_entity_id），
// 跨法人 store_id 在这里天然拿不到行。基线窗 = 当前窗之前等长的紧邻窗口。
// 归因本身是纯函数；handler 只做取数、窗口切分与聚合，不算任何因子。

type VarianceAttributionHandler struct {
	repo retailKPIReader
}

func NewVarianceAttributionHandler(repo retailKPIReader) *VarianceAttributionHandler {
	return &VarianceAttributionHandler{repo: repo}
}

func (h *VarianceAttributionHandler) StoreVarianceAttribution(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	storeID := strings.TrimSpace(c.Query("store_id"))
	if _, err := uuid.Parse(storeID); err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "store_id must be a UUID", nil)
		return
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("as_of")))
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "as_of must be an ISO date (YYYY-MM-DD)", nil)
		return
	}
	windowDays := 14
	if raw := strings.TrimSpace(c.Query("window_days")); raw != "" {
		windowDays, err = strconv.Atoi(raw)
		if err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
			return
		}
	}
	if _, rerr := retailperiod.ParseRollingDays(windowDays); rerr != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "window_days must be an integer between 7 and 28", nil)
		return
	}
	classification := c.Query("data_classification")
	if classification == "" {
		classification = "production"
	}
	datasetVersion := c.Query("dataset_version")
	sourceSystem := c.Query("source_system")

	currentFrom := asOf.AddDate(0, 0, -(windowDays - 1))
	baseTo := currentFrom.AddDate(0, 0, -1)
	baseFrom := baseTo.AddDate(0, 0, -(windowDays - 1))

	set, err := h.repo.QueryFacts(c.Request.Context(), legalEntityID, baseFrom.Format("2006-01-02"), asOf.Format("2006-01-02"), classification, datasetVersion, sourceSystem, []string{storeID})
	if err != nil {
		if err == repository.ErrRetailKPISourceConflict {
			writeCodedError(c, http.StatusConflict, errcontract.CodeConflict, err.Error(), gin.H{"reason": "source_conflict"})
			return
		}
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}

	var currentFacts, baseFacts []retailkpi.DailyFact
	for _, f := range set.Facts {
		d := f.BusinessDate
		if !d.Before(currentFrom) && !d.After(asOf) {
			currentFacts = append(currentFacts, f)
		} else if !d.Before(baseFrom) && !d.After(baseTo) {
			baseFacts = append(baseFacts, f)
		}
	}

	basePeriod, baseIssues := aggregateVarianceWindow(baseFacts)
	currentPeriod, currentIssues := aggregateVarianceWindow(currentFacts)
	result, aerr := varianceattribution.Attribute(basePeriod, currentPeriod, currencyOf(set.Facts), nil)
	if aerr != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, aerr.Error(), nil)
		return
	}
	if result.Status != "complete" {
		issues := append([]string(nil), baseIssues...)
		issues = append(issues, currentIssues...)
		result.MissingFacts = append(result.MissingFacts, issues...)
	}
	c.JSON(http.StatusOK, result)
}

// aggregateVarianceWindow 把一个窗口的日事实聚合成归因输入。
// 缺失传播：任一门店日在任一字段上为 nil，该字段的期间聚合即为缺失
// （nil），由服务层整体降级——不许拿 0 补齐后继续分解（retail-kpi-v1）。
// 币种混排同样是不可用条件之一。
func aggregateVarianceWindow(facts []retailkpi.DailyFact) (varianceattribution.PeriodFacts, []string) {
	out := varianceattribution.PeriodFacts{}
	if len(facts) == 0 {
		return out, []string{"no_facts"}
	}
	currencies := map[string]bool{}
	type acc struct {
		allPresent bool
		sum        float64
	}
	fields := map[string]*acc{
		"footfall":                {allPresent: true},
		"transactions":            {allPresent: true},
		"revenue":                 {allPresent: true},
		"gross_profit":            {allPresent: true},
		"labor_cost":              {allPresent: true},
		"occupancy_cost":          {allPresent: true},
		"other_controllable_cost": {allPresent: true},
	}
	add := func(name string, v *float64) {
		a := fields[name]
		if v == nil {
			a.allPresent = false
			return
		}
		a.sum += *v
	}
	for _, f := range facts {
		currencies[f.Currency] = true
		add("footfall", f.Footfall)
		add("transactions", f.Transactions)
		add("revenue", f.Revenue)
		add("gross_profit", f.GrossProfit)
		add("labor_cost", f.LaborCost)
		occ := sumNonNil(f.FixedRent, f.VariableRent, f.NonLeaseCost)
		if f.FixedRent == nil || f.VariableRent == nil || f.NonLeaseCost == nil {
			fields["occupancy_cost"].allPresent = false
		} else {
			fields["occupancy_cost"].sum += occ
		}
		add("other_controllable_cost", f.OtherControllableCost)
	}
	issues := make([]string, 0)
	if len(currencies) > 1 {
		issues = append(issues, "currency_conflict")
	}
	get := func(name string) *float64 {
		a := fields[name]
		if !a.allPresent {
			return nil
		}
		v := a.sum
		return &v
	}
	out.Footfall = get("footfall")
	out.Transactions = (*float64)(nil)
	if tx := get("transactions"); tx != nil {
		txInt := *tx
		out.Transactions = &txInt
	}
	out.Revenue = get("revenue")
	out.GrossProfit = get("gross_profit")
	out.LaborCost = get("labor_cost")
	out.OccupancyCost = get("occupancy_cost")
	out.OtherControllableCost = get("other_controllable_cost")

	if len(issues) > 0 || out.Footfall == nil || out.Transactions == nil || out.Revenue == nil ||
		out.GrossProfit == nil || out.LaborCost == nil || out.OccupancyCost == nil || out.OtherControllableCost == nil {
		return out, issues
	}
	return out, issues
}

func sumNonNil(vs ...*float64) float64 {
	var s float64
	for _, v := range vs {
		if v != nil {
			s += *v
		}
	}
	return s
}

func currencyOf(facts []retailkpi.DailyFact) string {
	for _, f := range facts {
		if f.Currency != "" {
			return f.Currency
		}
	}
	return ""
}
