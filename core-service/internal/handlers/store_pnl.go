package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// StorePnlHandler serves the store profit-and-loss projection (PRD S1):
// the single-store page and the S1-7 multi-store aggregate.
type StorePnlHandler struct {
	kpi       storepnl.KPIReader
	plan      storepnl.PlanReader // default comparison reader (tests/reuse)
	planRepo  *repository.FPnAGovernanceRepository
	peer      storepnl.PeerReader
	lease     storepnl.LeasePort
	occupancy storepnl.OccupancyReader
	stores    StoreLookup
	tmpl      *template.Template
	templates *repository.FinModelRepository
}

// StoreLookup resolves store scope dimensions for the projection's
// entity/scope gate and the S1-7 aggregate's authorized store set (bottom
// line 1). MasterDataRepository satisfies it; the interface lets unit tests
// inject a fake without a database.
type StoreLookup interface {
	GetStoreByID(ctx context.Context, storeID string) (repository.StoreOption, error)
	ListStores(ctx context.Context, tenantID, legalEntityID string) ([]repository.StoreOption, error)
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

// WithPeer attaches the cohort-benchmark port (S1-6).
func (h *StorePnlHandler) WithPeer(peer storepnl.PeerReader) *StorePnlHandler {
	h.peer = peer
	return h
}

// WithMasterData attaches the store master-data reader the S1-7 aggregate
// and the projection's entity/scope gate use (scope filtering lives in the
// repository / gate, not here).
func (h *StorePnlHandler) WithMasterData(stores StoreLookup) *StorePnlHandler {
	h.stores = stores
	return h
}

// WithOccupancy attaches the S1-5 contract-level occupancy split port.
func (h *StorePnlHandler) WithOccupancy(occupancy storepnl.OccupancyReader) *StorePnlHandler {
	h.occupancy = occupancy
	return h
}

// WithLease attaches the S1-1 production lease port; the default remains
// the honest unavailable adapter (ROU/interest rows stay missing rather
// than fabricated) when no measurement source is wired.
func (h *StorePnlHandler) WithLease(lease storepnl.LeasePort) *StorePnlHandler {
	h.lease = lease
	return h
}

// WithTemplates attaches the template repository so a request can render
// through an explicit template version (S1-9 custom rows).
func (h *StorePnlHandler) WithTemplates(templates *repository.FinModelRepository) *StorePnlHandler {
	h.templates = templates
	return h
}

type unavailableLease struct{}

func (unavailableLease) Monthly(_ context.Context, _, _, _ string) (storepnl.LeaseMonthValues, error) {
	return storepnl.LeaseMonthValues{}, errors.New("IFRS 16 口径尚未接通：门店级 ROU/利息投影适配器待接线（诚实降级，不产出数字）")
}

// projectionParams is the resolved shared query surface both the single-
// store projection and the S1-7 aggregate consume.
type projectionParams struct {
	ref    storepnl.StoreRef
	period storepnl.Period
	basis  storepnl.BasisMode
	pair   [2]storepnl.ColumnRef
}

// parseProjectionParams resolves the period grain (S1-2 retailperiod
// forms), classification, basis and column pair. The legacy
// window_days/as_of pair stays the fallback when no ?period is given.
// ok=false means a 400 has already been answered.
func parseProjectionParams(c *gin.Context, storeID string) (projectionParams, bool) {
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

	if spec := strings.TrimSpace(c.Query("period")); spec != "" {
		anchor := time.Now()
		if asOf != "" {
			parsed, err := time.Parse("2006-01-02", asOf)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
				return projectionParams{}, false
			}
			anchor = parsed
		}
		window, err := retailperiod.Parse(spec, anchor)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return projectionParams{}, false
		}
		ref.DateFrom = window.From.Format("2006-01-02")
		ref.DateTo = window.To.Format("2006-01-02")
		ref.PeriodLabel = window.Label
		ref.PeriodKind = string(window.Period.Kind)
		period = storepnl.Period{From: ref.DateFrom, To: ref.DateTo}
	}
	return projectionParams{ref: ref, period: period, basis: basis, pair: [2]storepnl.ColumnRef{primary, secondary}}, true
}

// planReaderFor resolves the comparison-column reader for the request.
func (h *StorePnlHandler) planReaderFor(c *gin.Context) storepnl.PlanReader {
	planReader := h.plan
	if versionID := strings.TrimSpace(c.Query("plan_version_id")); versionID != "" && h.planRepo != nil {
		planReader = SetStorePnlPlanReader(h.planRepo, versionID)
	}
	return planReader
}

// Projection is GET /api/v1/stores/:id/pnl. The period grain (day/week/
// month/quarter/year, S1-2) resolves through retailperiod: a ?period spec
// (rolling days, YYYY-MM, YYYY-Qn, YYYY-Wnn, YYYY, last-month,
// this-quarter) wins; the legacy window_days/as_of pair stays as fallback.
func (h *StorePnlHandler) Projection(c *gin.Context) {
	storeID := c.Param("id")
	if err := h.requireScopedStore(c, storeID); err != nil {
		return
	}
	params, ok := parseProjectionParams(c, storeID)
	if !ok {
		return
	}
	tmpl, renderErr := h.templateFor(c)
	if renderErr != nil {
		return
	}
	pnl, err := storepnl.Project(c.Request.Context(), tmpl, params.ref, params.period, params.pair, params.basis, storepnl.Readers{
		KPI: h.kpi, Plan: h.planReaderFor(c), Lease: h.lease, Peer: h.peer, Occupancy: h.occupancy, Governed: h.governedRows(c),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.EqualFold(c.Query("format"), "xlsx") {
		out, err := storepnl.RenderXLSX(pnl)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "store-pnl-"+storeID+".xlsx"))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pnl": pnl})
}

// requireScopedStore is the single-store sibling of the aggregate's scoped
// master-data read (bottom line 1 + Data Scope): the targeted store must
// belong to the caller's tenant, and when the caller carries an explicit
// store/region/brand scope the store must sit inside it. A wrong tenant is
// indistinguishable from a missing store (no existence leak); an out-of-scope
// store keeps the scope_denied reason (never softened to "no data").
func (h *StorePnlHandler) requireScopedStore(c *gin.Context, storeID string) error {
	if h.stores == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store master data reader unavailable"})
		return errors.New("store master data reader unavailable")
	}
	store, err := h.stores.GetStoreByID(c.Request.Context(), storeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return err
	}
	if store.ID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "store not found"})
		return errors.New("store not found")
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" && store.LegalEntityID != tenant {
		c.JSON(http.StatusNotFound, gin.H{"error": "store not found"})
		return errors.New("store outside caller tenant")
	}
	if scope, scoped := access.ScopeFromContext(c.Request.Context()); scoped && !scope.Global {
		if len(scope.StoreIDs) > 0 && !slices.Contains(scope.StoreIDs, storeID) {
			writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied, "scope_denied: store outside caller data scope", nil)
			return errors.New("store outside caller data scope")
		}
		if len(scope.Regions) > 0 && !slices.Contains(scope.Regions, deref(store.Region)) {
			writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied, "scope_denied: store region outside caller data scope", nil)
			return errors.New("store region outside caller data scope")
		}
		if len(scope.Brands) > 0 && !slices.Contains(scope.Brands, deref(store.Brand)) {
			writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied, "scope_denied: store brand outside caller data scope", nil)
			return errors.New("store brand outside caller data scope")
		}
	}
	return nil
}

// AggregateProjection is GET /store-pnl/aggregate?group_by=region|brand|
// legal_entity — the S1-7 multi-store mode. The authorized store set comes
// from the scoped master-data read (bottom line 1: entity/store/region/
// brand scope is enforced inside ListStores, not here); every store is
// projected with the same period/versions and the pure aggregator groups
// the tables, partitioning by currency so mixed-currency groups never
// produce a cross-currency total (T14). Stores that fail to project are
// named in degraded_stores, never silently dropped.
func (h *StorePnlHandler) AggregateProjection(c *gin.Context) {
	groupBy := storepnl.GroupBy(strings.TrimSpace(c.Query("group_by")))
	if !storepnl.ValidGroupBy(string(groupBy)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_by is required and must be region, brand or legal_entity"})
		return
	}
	if h.stores == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "store master data reader unavailable"})
		return
	}
	params, ok := parseProjectionParams(c, "")
	if !ok {
		return
	}
	stores, err := h.stores.ListStores(c.Request.Context(), middleware.GetTenantID(c), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	planReader := h.planReaderFor(c)
	governed := h.governedRows(c)
	members := make([]storepnl.AggregateMember, 0, len(stores))
	degraded := []storepnl.DegradedStore{}
	for _, store := range stores {
		ref := params.ref
		ref.StoreID = store.ID
		pnl, err := storepnl.Project(c.Request.Context(), h.tmpl, ref, params.period, params.pair, params.basis, storepnl.Readers{
			KPI: h.kpi, Plan: planReader, Lease: h.lease, Governed: governed,
		})
		if err != nil {
			degraded = append(degraded, storepnl.DegradedStore{StoreID: store.ID, Reason: err.Error()})
			continue
		}
		members = append(members, storepnl.AggregateMember{
			StoreID: store.ID, LegalEntityID: store.LegalEntityID,
			Region: deref(store.Region), Brand: deref(store.Brand), Pnl: pnl,
		})
	}
	result, err := storepnl.Aggregate(groupBy, params.period, params.pair, members, degraded)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"aggregate": result})
}

// templateFor resolves the render template: an explicit template_id loads
// that version through Parse (visible to the caller's entity or a
// creator-owned personal draft); otherwise the factory template serves.
func (h *StorePnlHandler) templateFor(c *gin.Context) (*template.Template, error) {
	id := strings.TrimSpace(c.Query("template_id"))
	if id == "" {
		return h.tmpl, nil
	}
	if h.templates == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "template_id requested but the template repository is unavailable"})
		return nil, errors.New("template repository unavailable")
	}
	row, err := h.templates.GetStatementTemplate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return nil, err
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" {
		if row.LegalEntityID != nil && *row.LegalEntityID != tenant {
			c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
			return nil, errors.New("template outside caller scope")
		}
		if row.LegalEntityID == nil {
			userID, ok := userID(c)
			if !ok || row.CreatedBy == nil || *row.CreatedBy != userID {
				c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
				return nil, errors.New("personal template outside caller ownership")
			}
		}
	}
	tmpl, err := h.templates.LoadStatementTemplate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return nil, err
	}
	return tmpl, nil
}

func monthOf(asOf string) string {
	if len(asOf) >= 7 {
		return asOf[:7]
	}
	return asOf
}

// governedRows lists the row keys with an approved fpna_metric_definitions
// entry — the only way a template formula row escapes the 未经指标治理
// marker (S3-6). A nil repo yields an empty set: every formula row is
// marked, which is the fail-closed presumption.
func (h *StorePnlHandler) governedRows(c *gin.Context) map[string]bool {
	set := map[string]bool{}
	if h.planRepo == nil {
		return set
	}
	defs, err := h.planRepo.ListMetricDefinitions(c.Request.Context(), "")
	if err != nil {
		return set
	}
	for _, def := range defs {
		if def.Status == "approved" && strings.TrimSpace(def.MetricKey) != "" {
			set[def.MetricKey] = true
		}
	}
	return set
}
