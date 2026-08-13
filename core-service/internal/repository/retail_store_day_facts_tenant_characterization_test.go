package repository

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
)

// TestRetailStoreDayFactsTenantIsolationByIDsCharacterization pins down the
// two legal-entity filtering paths of the store-day repository that the
// existing integration test does not probe directly: the by-IDs list behind
// idempotent replay, and the store filter of the paged list when the caller
// passes another entity's store ids.
func TestRetailStoreDayFactsTenantIsolationByIDsCharacterization(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "retail-ids")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	repo := NewOperatingFactsRepository(pool)
	fact := &RetailStoreDayFact{
		StoreID: pair.storeA, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: 100,
		SourceSystem: "char-test", DataClassification: "production",
	}
	if _, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, pair.entityA), fact); err != nil {
		t.Fatalf("seed store-day fact for entity A: %v", err)
	}

	// The paged list scopes by the caller's legal entity even when the caller
	// supplies another entity's store ids.
	pageB, err := repo.ListRetailStoreDayFactsPage(ctx, mustEntityFilter(t, pair.entityB), "2026-08-01", "2026-08-01", []string{pair.storeA}, 10, 0)
	if err != nil || pageB.Total != 0 || len(pageB.Data) != 0 {
		t.Fatalf("entity B page with entity A store ids = total %d rows %d, err %v; want 0/0", pageB.Total, len(pageB.Data), err)
	}
	pageA, err := repo.ListRetailStoreDayFactsPage(ctx, mustEntityFilter(t, pair.entityA), "2026-08-01", "2026-08-01", []string{pair.storeA}, 10, 0)
	if err != nil || pageA.Total != 1 || len(pageA.Data) != 1 {
		t.Fatalf("entity A page with own store ids = total %d rows %d, err %v; want 1/1", pageA.Total, len(pageA.Data), err)
	}

	// An idempotent atomic replay under entity B must not surface entity A's
	// stored fact ids: the write path resolves the store through the caller's
	// tenant first and must refuse the cross-entity store.
	replay := &RetailStoreDayFact{
		StoreID: pair.storeA, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: 200,
		SourceSystem: "char-test", DataClassification: "production",
	}
	if _, err := repo.UpsertRetailStoreDayFactsAtomic(access.WithScope(ctx, access.Scope{LegalEntityID: pair.entityB}), mustEntityFilter(t, pair.entityB), []*RetailStoreDayFact{replay}, "char-idem-b", "hash-b", nil, nil); err == nil {
		t.Fatal("entity B atomic replay on entity A store unexpectedly succeeded")
	}
}
