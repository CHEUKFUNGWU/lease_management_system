package handlers

// storePnlAgentReader bridges the Agent Tool seam (agenttooldefs.StorePnlReader)
// onto the production StorePnlHandler wiring — one projection implementation
// serves both the HTTP surface and the Agent. The seam never computes a KPI
// and never writes a business table; the projection core (storepnl.Project)
// and the S1 port adapters are the same objects the /stores/:id/pnl route
// uses.

import (
	"context"
	"slices"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// The gate returns the tools' exported sentinels so error classification is
// single-sourced in storePnlErrorCode. A wrong tenant reads as not-found
// (no existence leak, 底线 1); an out-of-dimension-scope store reads as
// scope_denied with the reason kept (never softened to "no data").

// storePnlAgentReader implements agenttooldefs.StorePnlReader over the
// production handler wiring.
type storePnlAgentReader struct {
	h *StorePnlHandler
}

// NewStorePnlAgentReader builds the seam. A nil handler keeps the tool
// honest (unavailable at call time) — the wiring never registers a nil
// version unconditionally (P0-8).
func NewStorePnlAgentReader(h *StorePnlHandler) agenttooldefs.StorePnlReader {
	return storePnlAgentReader{h: h}
}

// Project implements the seam: scope-gate the store, resolve the optional
// plan-version reader, then run the same projection core as the HTTP route.
func (r storePnlAgentReader) Project(ctx context.Context, q agenttooldefs.StorePnlQuery) (*storepnl.StorePnl, error) {
	h := r.h
	if h == nil {
		return nil, agenttooldefs.ErrStoreMasterDataUnavailable()
	}
	// 法人维度：执行上下文已把 Principal.Scope 投影进 ctx（agenttools
	// WithExecutionContext）；这里用它作为调用方 tenant，而不是从参数里
	// 信任任何值。scope_denied 原因保留。
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped {
		return nil, agenttooldefs.ErrStoreMasterDataUnavailable()
	}
	tenantID := scope.LegalEntityID

	if _, err := resolveScopedStore(ctx, h.stores, tenantID, scope, q.Ref.StoreID); err != nil {
		return nil, err
	}

	planReader := h.plan
	if q.PlanVersionID != "" && h.planRepo != nil {
		planReader = SetStorePnlPlanReader(h.planRepo, q.PlanVersionID)
	}
	return storepnl.Project(ctx, h.tmpl, q.Ref, q.Period, q.Pair, q.Basis, storepnl.Readers{
		KPI:       h.kpi,
		Plan:      planReader,
		Lease:     h.lease,
		Peer:      h.peer,
		Occupancy: h.occupancy,
		Governed:  h.governedRowsCtx(ctx),
	})
}

// resolveScopedStore is the shared store-scope gate (底线 1) used by both
// the gin route and the Agent seam. Store must belong to the caller's legal
// entity; when the caller carries an explicit store/region/brand scope the
// store must sit inside it. A wrong tenant is indistinguishable from a
// missing store (no existence leak); an out-of-dimension-scope store keeps
// the scope_denied reason (never softened to "no data").
func resolveScopedStore(ctx context.Context, stores StoreLookup, tenantID string, scope access.Scope, storeID string) (repository.StoreOption, error) {
	if stores == nil {
		return repository.StoreOption{}, agenttooldefs.ErrStoreMasterDataUnavailable()
	}
	store, err := stores.GetStoreByID(ctx, storeID)
	if err != nil {
		return repository.StoreOption{}, err
	}
	if store.ID == "" {
		return repository.StoreOption{}, agenttooldefs.ErrStoreNotFound()
	}
	if tenantID != "" && store.LegalEntityID != tenantID {
		return repository.StoreOption{}, agenttooldefs.ErrStoreNotFound() // 异法人视同不存在：无存在性泄漏
	}
	if !scope.Global {
		if len(scope.StoreIDs) > 0 && !slices.Contains(scope.StoreIDs, storeID) {
			return repository.StoreOption{}, agenttooldefs.ErrStoreScopeDenied()
		}
		if len(scope.Regions) > 0 && !slices.Contains(scope.Regions, deref(store.Region)) {
			return repository.StoreOption{}, agenttooldefs.ErrStoreScopeDenied()
		}
		if len(scope.Brands) > 0 && !slices.Contains(scope.Brands, deref(store.Brand)) {
			return repository.StoreOption{}, agenttooldefs.ErrStoreScopeDenied()
		}
	}
	return store, nil
}

// governedRowsCtx lists the approved metric-definition row keys — the ctx
// variant of the gin handler's governedRows. A nil repo yields an empty set:
// every formula row is marked 未经指标治理, the fail-closed presumption.
func (h *StorePnlHandler) governedRowsCtx(ctx context.Context) map[string]bool {
	set := map[string]bool{}
	if h == nil || h.planRepo == nil {
		return set
	}
	defs, err := h.planRepo.ListMetricDefinitions(ctx, "")
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
