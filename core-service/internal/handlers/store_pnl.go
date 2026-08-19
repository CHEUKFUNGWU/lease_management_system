package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// StorePnlHandler serves the store profit-and-loss projection (PRD S1).
type StorePnlHandler struct {
	kpi      storepnl.KPIReader
	plan     storepnl.PlanReader // default comparison reader (tests/reuse)
	planRepo *repository.FPnAGovernanceRepository
	lease    storepnl.LeasePort
	tmpl     *template.Template
}

// NewStorePnlHandler builds the handler. The lease port arrives honest:
// until the IFRS 16 per-store projection adapter lands, the ifrs16 block
// reports the gap instead of fabricating depreciation/interest.
func NewStorePnlHandler(kpi storepnl.KPIReader, plan storepnl.PlanReader, planRepo *repository.FPnAGovernanceRepository) *StorePnlHandler {
	tmpl, err := template.DefaultStorePnlTemplate()
	if err != nil {
		panic(err) // 出厂模板受包内测试锁定，失败即装配缺陷
	}
	return &StorePnlHandler{kpi: kpi, plan: plan, planRepo: planRepo, tmpl: tmpl, lease: unavailableLease{}}
}

type unavailableLease struct{}

func (unavailableLease) Monthly(_ context.Context, _, _ string) (storepnl.LeaseMonthValues, error) {
	return storepnl.LeaseMonthValues{}, errors.New("IFRS 16 口径尚未接通：门店级 ROU/利息投影适配器待接线（诚实降级，不产出数字）")
}

// Projection is GET /api/v1/stores/:id/pnl.
func (h *StorePnlHandler) Projection(c *gin.Context) {
	storeID := c.Param("id")
	asOf := strings.TrimSpace(c.Query("as_of"))
	windowDays, _ := strconv.Atoi(c.DefaultQuery("window_days", "7"))
	classification := strings.TrimSpace(c.DefaultQuery("data_classification", "production"))
	datasetVersion := strings.TrimSpace(c.Query("dataset_version"))
	basis := storepnl.BasisMode(strings.TrimSpace(c.DefaultQuery("basis", "side_by_side")))
	primary := storepnl.ColumnRef(strings.TrimSpace(c.DefaultQuery("primary", "actual")))
	secondary := storepnl.ColumnRef(strings.TrimSpace(c.DefaultQuery("secondary", "budget")))

	ref := storepnl.StoreRef{
		StoreID: storeID, AsOf: asOf, WindowDays: windowDays,
		LegalEntityID:  middleware.GetTenantID(c),
		Classification: classification, DatasetVersion: datasetVersion,
	}
	period := storepnl.Period{From: monthOf(asOf), To: monthOf(asOf)}
	planReader := h.plan
	if versionID := strings.TrimSpace(c.Query("plan_version_id")); versionID != "" && h.planRepo != nil {
		planReader = SetStorePnlPlanReader(h.planRepo, versionID)
	}
	pnl, err := storepnl.Project(c.Request.Context(), h.tmpl, ref, period, [2]storepnl.ColumnRef{primary, secondary}, basis, storepnl.Readers{
		KPI: h.kpi, Plan: planReader, Lease: h.lease,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pnl": pnl})
}

func monthOf(asOf string) string {
	if len(asOf) >= 7 {
		return asOf[:7]
	}
	return asOf
}
