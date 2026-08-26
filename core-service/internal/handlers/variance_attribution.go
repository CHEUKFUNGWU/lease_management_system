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
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
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

// aggregateVarianceWindow 委托给 varianceattribution.AggregateWindow——
// 窗口聚合的唯一实现在服务层，HTTP 与 Agent 诊断链共用（Ch1 BG1 接线）。
func aggregateVarianceWindow(facts []retailkpi.DailyFact) (varianceattribution.PeriodFacts, []string) {
	return varianceattribution.AggregateWindow(facts)
}

func currencyOf(facts []retailkpi.DailyFact) string {
	for _, f := range facts {
		if f.Currency != "" {
			return f.Currency
		}
	}
	return ""
}
