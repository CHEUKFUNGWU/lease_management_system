package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
)

func retailStoreDayFactsPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedRetailStoreDayFactsTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (string, string) {
	t.Helper()
	suffix := uuid.NewString()[:8]
	var legalEntityID, storeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "DAY-LE-"+label+"-"+suffix, "Store-day tenant "+label).Scan(&legalEntityID); err != nil {
		t.Fatalf("seed legal entity %s: %v", label, err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id
	`, "DAY-ST-"+label+"-"+suffix, "Store-day store "+label, legalEntityID, "Brand-"+label, "Region-"+label).Scan(&storeID); err != nil {
		t.Fatalf("seed store %s: %v", label, err)
	}
	return legalEntityID, storeID
}

func seedRetailStoreDayFactsBatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, legalEntityID, label string) string {
	t.Helper()
	var batchID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO operating_fact_batches (legal_entity_id, source_system, status)
		VALUES ($1, $2, 'received') RETURNING id
	`, legalEntityID, "store-day-batch-"+label).Scan(&batchID); err != nil {
		t.Fatalf("seed import batch %s: %v", label, err)
	}
	return batchID
}

func TestRetailStoreDayFactsPostgresConstraintsIdempotencyAndTenantScope(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, storeA := seedRetailStoreDayFactsTenant(t, ctx, pool, "a")
	entityB, storeB := seedRetailStoreDayFactsTenant(t, ctx, pool, "b")
	batchA := seedRetailStoreDayFactsBatch(t, ctx, pool, entityA, "a")
	batchB := seedRetailStoreDayFactsBatch(t, ctx, pool, entityB, "b")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_fact_requests WHERE scope_key IN ($1, $2)`, entityA, entityB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE legal_entity_id IN ($1, $2)`, entityA, entityB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts WHERE store_id IN ($1, $2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM operating_fact_batches WHERE id IN ($1, $2)`, batchA, batchB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id IN ($1, $2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id IN ($1, $2)`, entityA, entityB)
	})

	repo := NewOperatingFactsRepository(pool)
	production := &RetailStoreDayFact{
		StoreID: storeA, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: 100,
		SourceSystem: "postgres-test", DataClassification: "production", Version: 1,
	}
	first, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityA), production)
	if err != nil {
		t.Fatalf("first production upsert: %v", err)
	}
	second := *production
	second.ID = ""
	second.Revenue = 200
	replayed, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityA), &second)
	if err != nil {
		t.Fatalf("replayed production upsert: %v", err)
	}
	if replayed.ID != first.ID {
		t.Fatalf("replayed fact id = %s, first id = %s; business key was not idempotent", replayed.ID, first.ID)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE store_id = $1 AND business_date = DATE '2026-08-01' AND version = 1 AND source_system = 'postgres-test'`, storeA).Scan(&count); err != nil {
		t.Fatalf("count idempotent rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("idempotent row count = %d, want 1", count)
	}

	rowsA, err := repo.ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), mustEntityFilter(t, entityA), "2026-08-01", "2026-08-01", nil, "")
	if err != nil {
		t.Fatalf("list tenant A facts: %v", err)
	}
	if len(rowsA) != 1 || rowsA[0].StoreID != storeA || rowsA[0].StoreCode == "" || rowsA[0].Brand == "" || rowsA[0].Region == "" {
		t.Fatalf("tenant A rows = %+v; expected one row with store metadata", rowsA)
	}
	rowsB, err := repo.ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}), mustEntityFilter(t, entityB), "2026-08-01", "2026-08-01", nil, "")
	if err != nil {
		t.Fatalf("list tenant B facts: %v", err)
	}
	if len(rowsB) != 0 {
		t.Fatalf("tenant B saw tenant A facts: %+v", rowsB)
	}
	productionB := &RetailStoreDayFact{
		StoreID: storeB, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: 300,
		SourceSystem: "postgres-test-b", DataClassification: "production", Version: 1,
	}
	if _, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityB), productionB); err != nil {
		t.Fatalf("tenant B own production upsert: %v", err)
	}
	rowsAAfterB, err := repo.ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), mustEntityFilter(t, entityA), "2026-08-01", "2026-08-01", nil, "")
	if err != nil {
		t.Fatalf("list tenant A after tenant B write: %v", err)
	}
	if len(rowsAAfterB) != 1 || rowsAAfterB[0].StoreID != storeA {
		t.Fatalf("tenant A isolation after tenant B write failed: %+v", rowsAAfterB)
	}
	rowsBOwn, err := repo.ListRetailStoreDayFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}), mustEntityFilter(t, entityB), "2026-08-01", "2026-08-01", nil, "")
	if err != nil || len(rowsBOwn) != 1 || rowsBOwn[0].StoreID != storeB {
		t.Fatalf("tenant B own fact visibility = %+v, err=%v", rowsBOwn, err)
	}
	if _, err := repo.UpsertRetailStoreDayFact(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}), mustEntityFilter(t, entityB), &RetailStoreDayFact{StoreID: storeA, BusinessDate: "2026-08-02", Currency: "CNY", Revenue: 10, SourceSystem: "postgres-test", DataClassification: "production"}); err == nil {
		t.Fatal("tenant B write to tenant A store unexpectedly succeeded")
	}
	if _, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityA), &RetailStoreDayFact{StoreID: storeA, BusinessDate: "2026-08-06", Currency: "CNY", Revenue: 10, SourceSystem: "postgres-test-batch", ImportBatchID: &batchB, DataClassification: "production"}); err == nil {
		t.Fatal("cross-entity import batch unexpectedly succeeded")
	}
	validBatchFact := &RetailStoreDayFact{StoreID: storeA, BusinessDate: "2026-08-07", Currency: "CNY", Revenue: 11, SourceSystem: "postgres-test-batch", ImportBatchID: &batchA, DataClassification: "production"}
	if _, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityA), validBatchFact); err != nil {
		t.Fatalf("same-entity import batch rejected: %v", err)
	}
	var productionRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE store_id=$1 AND data_classification='production' AND simulation_dataset_version IS NULL`, storeA).Scan(&productionRows); err != nil || productionRows < 2 {
		t.Fatalf("production-only facts check = count %d, err %v", productionRows, err)
	}

	// These writes bypass the Go validation deliberately: the database must
	// remain the final guard for every ingestion path.
	invalidSimulated, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (store_id, business_date, currency, revenue, source_system, data_classification)
		VALUES ($1, DATE '2026-08-03', 'CNY', 1, 'postgres-test-invalid', 'simulated')
	`, storeA)
	if err == nil || invalidSimulated.RowsAffected() != 0 {
		t.Fatal("simulated row without dataset version was accepted")
	}
	invalidProduction, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (store_id, business_date, currency, revenue, source_system, data_classification, simulation_dataset_version)
		VALUES ($1, DATE '2026-08-04', 'CNY', 1, 'postgres-test-invalid', 'production', 'seed-v1')
	`, storeA)
	if err == nil || invalidProduction.RowsAffected() != 0 {
		t.Fatal("production row with dataset version was accepted")
	}
	negative, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (store_id, business_date, currency, revenue, transactions, source_system, data_classification)
		VALUES ($1, DATE '2026-08-05', 'CNY', 1, -1, 'postgres-test-invalid', 'production')
	`, storeA)
	if err == nil || negative.RowsAffected() != 0 {
		t.Fatal("negative quantity row was accepted")
	}

	var latestAsOf time.Time
	if err := pool.QueryRow(ctx, `SELECT as_of_at FROM retail_store_day_facts WHERE id = $1`, first.ID).Scan(&latestAsOf); err != nil {
		t.Fatalf("read as_of_at: %v", err)
	}
	if latestAsOf.IsZero() {
		t.Fatal("as_of_at was not stored")
	}
}

func TestRetailStoreDayFactsPostgresSchemaHasLookupIndex(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	var exists bool
	if err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = current_schema()
			  AND tablename = 'retail_store_day_facts'
			  AND indexname = 'idx_retail_store_day_facts_lookup'
		)
	`).Scan(&exists); err != nil {
		t.Fatalf("check store-day lookup index: %v", err)
	}
	if !exists {
		t.Fatal("retail store-day lookup index is missing")
	}
}

func TestRetailStoreDayFactsPostgresAtomicAuditIdempotencyAndRollback(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityID, storeID := seedRetailStoreDayFactsTenant(t, ctx, pool, "atomic")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_fact_requests WHERE scope_key=$1`, entityID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE legal_entity_id=$1`, entityID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts WHERE store_id=$1`, storeID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id=$1`, storeID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id=$1`, entityID)
	})

	auditRepo := NewAuditRepository(pool)
	var auditCalls int
	audit := func(fail bool) RetailStoreDayFactAuditFunc {
		return func(ctx context.Context, tx DBTX, oldFact, newFact *RetailStoreDayFact) error {
			auditCalls++
			if fail {
				return errors.New("forced audit failure")
			}
			newJSON, _ := json.Marshal(newFact)
			log := &AuditLog{TableName: "retail_store_day_facts", RecordID: newFact.ID, LegalEntityID: &entityID, Action: "upsert", NewValues: stringPtr(string(newJSON)), ChangedAt: time.Now().UTC()}
			if oldFact != nil {
				oldJSON, _ := json.Marshal(oldFact)
				log.OldValues = stringPtr(string(oldJSON))
			}
			return auditRepo.WithTx(tx).Create(ctx, log)
		}
	}

	repo := NewOperatingFactsRepository(pool)
	first := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-10", Currency: "CNY", Revenue: 100, SourceSystem: "atomic-test", DataClassification: "production"}
	result, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{first}, "", "", nil, audit(false))
	if err != nil || result.Replayed || len(result.Facts) != 1 {
		t.Fatalf("atomic first write = result %+v, err %v", result, err)
	}
	updated := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-10", Currency: "CNY", Revenue: 200, SourceSystem: "atomic-test", DataClassification: "production"}
	if _, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{updated}, "", "", nil, audit(false)); err != nil {
		t.Fatalf("atomic update: %v", err)
	}
	var oldRevenue, newRevenue float64
	if err := pool.QueryRow(ctx, `
		SELECT (old_values->>'revenue')::numeric, (new_values->>'revenue')::numeric
		FROM audit_logs WHERE table_name='retail_store_day_facts' AND record_id=$1
		ORDER BY changed_at DESC LIMIT 1`, updated.ID).Scan(&oldRevenue, &newRevenue); err != nil {
		t.Fatalf("read atomic old/new audit: %v", err)
	}
	if oldRevenue != 100 || newRevenue != 200 {
		t.Fatalf("atomic old/new audit = %.2f/%.2f, want 100/200", oldRevenue, newRevenue)
	}

	failed := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-11", Currency: "CNY", Revenue: 300, SourceSystem: "atomic-failure", DataClassification: "production"}
	if _, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{failed}, "", "", nil, audit(true)); err == nil {
		t.Fatal("audit failure unexpectedly committed")
	}
	var factCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE store_id=$1 AND business_date=DATE '2026-08-11'`, storeID).Scan(&factCount); err != nil {
		t.Fatalf("count rolled-back fact: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE table_name='retail_store_day_facts' AND record_id=$1`, failed.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count rolled-back audit: %v", err)
	}
	if factCount != 0 || auditCount != 0 {
		t.Fatalf("audit rollback counts = fact %d/audit %d, want 0/0", factCount, auditCount)
	}

	key := "atomic-idem-" + uuid.NewString()
	hash := strings.Repeat("a", 64)
	idem := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-12", Currency: "CNY", Revenue: 123, SourceSystem: "atomic-idem", DataClassification: "production"}
	firstIdem, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{idem}, key, hash, nil, audit(false))
	if err != nil || firstIdem.Replayed {
		t.Fatalf("first idempotent write = %+v, err %v", firstIdem, err)
	}
	replay := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-12", Currency: "CNY", Revenue: 999, SourceSystem: "atomic-idem", DataClassification: "production"}
	beforeReplayAudits := auditCalls
	replayed, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{replay}, key, hash, nil, audit(false))
	if err != nil || !replayed.Replayed || auditCalls != beforeReplayAudits {
		t.Fatalf("idempotent replay = %+v, err %v, audit calls %d->%d", replayed, err, beforeReplayAudits, auditCalls)
	}
	var storedRevenue float64
	if err := pool.QueryRow(ctx, `SELECT revenue FROM retail_store_day_facts WHERE id=$1`, firstIdem.Facts[0].ID).Scan(&storedRevenue); err != nil {
		t.Fatalf("read idempotent replay fact: %v", err)
	}
	if storedRevenue != 123 {
		t.Fatalf("idempotent replay changed revenue to %.2f", storedRevenue)
	}
	if _, err := repo.UpsertRetailStoreDayFactsAtomic(ctx, mustEntityFilter(t, entityID), []*RetailStoreDayFact{replay}, key, strings.Repeat("b", 64), nil, audit(false)); !errors.Is(err, ErrRetailStoreDayFactIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v, want %v", err, ErrRetailStoreDayFactIdempotencyConflict)
	}
}

func TestRetailStoreDayFactsPostgresPaginationHasReliableTotal(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityID, storeID := seedRetailStoreDayFactsTenant(t, ctx, pool, "page")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts WHERE store_id=$1`, storeID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id=$1`, storeID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id=$1`, entityID)
	})
	repo := NewOperatingFactsRepository(pool)
	for index, date := range []string{"2026-08-20", "2026-08-21", "2026-08-22"} {
		fact := &RetailStoreDayFact{StoreID: storeID, BusinessDate: date, Currency: "CNY", Revenue: float64(index + 1), SourceSystem: "pagination-test", DataClassification: "production"}
		if _, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityID), fact); err != nil {
			t.Fatalf("seed page fact %s: %v", date, err)
		}
	}
	first, err := repo.ListRetailStoreDayFactsPage(ctx, mustEntityFilter(t, entityID), "2026-08-20", "2026-08-22", nil, "", 2, 0)
	if err != nil || first.Total != 3 || first.Returned != 2 {
		t.Fatalf("first page = %+v, err %v", first, err)
	}
	second, err := repo.ListRetailStoreDayFactsPage(ctx, mustEntityFilter(t, entityID), "2026-08-20", "2026-08-22", nil, "", 2, 2)
	if err != nil || second.Total != 3 || second.Returned != 1 {
		t.Fatalf("second page = %+v, err %v", second, err)
	}
}

func TestRetailStoreDayFactsPostgresAuditDimensionScope(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityID, storeA := seedRetailStoreDayFactsTenant(t, ctx, pool, "scope")
	var storeB string
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code,name,legal_entity_id,brand,region,is_active)
		VALUES ($1,'Store-day scope B',$2,'Brand-scope-b','Region-scope-b',true) RETURNING id
	`, "DAY-ST-scope-b-"+uuid.NewString()[:8], entityID).Scan(&storeB); err != nil {
		t.Fatalf("seed scope store B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE legal_entity_id=$1`, entityID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts WHERE store_id IN ($1,$2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id IN ($1,$2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id=$1`, entityID)
	})
	repo := NewOperatingFactsRepository(pool)
	auditRepo := NewAuditRepository(pool)
	for index, storeID := range []string{storeA, storeB} {
		fact := &RetailStoreDayFact{StoreID: storeID, BusinessDate: "2026-08-30", Currency: "CNY", Revenue: float64(index + 1), SourceSystem: "scope-test", DataClassification: "production"}
		written, err := repo.UpsertRetailStoreDayFact(ctx, mustEntityFilter(t, entityID), fact)
		if err != nil {
			t.Fatalf("seed scoped fact: %v", err)
		}
		newJSON, _ := json.Marshal(written)
		if err := auditRepo.Create(ctx, &AuditLog{TableName: "retail_store_day_facts", RecordID: written.ID, LegalEntityID: &entityID, Action: "upsert", NewValues: stringPtr(string(newJSON)), ChangedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("seed scoped audit: %v", err)
		}
	}
	checks := []struct {
		name  string
		scope access.Scope
		want  int
	}{
		{name: "store", scope: access.Scope{LegalEntityID: entityID, StoreIDs: []string{storeA}}, want: 1},
		{name: "region", scope: access.Scope{LegalEntityID: entityID, Regions: []string{"Region-scope-b"}}, want: 1},
		{name: "brand", scope: access.Scope{LegalEntityID: entityID, Brands: []string{"Brand-scope-b"}}, want: 1},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			logs, total, err := auditRepo.List(access.WithScope(ctx, check.scope), AuditLogFilter{TableName: "retail_store_day_facts", Limit: 50})
			if err != nil || total != check.want || len(logs) != check.want {
				t.Fatalf("audit scope logs=%d total=%d err=%v, want %d", len(logs), total, err, check.want)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
