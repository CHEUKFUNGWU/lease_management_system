package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresTestPool connects to the real test database named by
// TEST_DATABASE_URL. Tests are skipped when the variable is unset, matching
// the convention of retail_store_day_facts_postgres_integration_test.go.
func postgresTestPool(t *testing.T) *pgxpool.Pool {
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

// tenantPair carries the two legal entities and their dependent rows that the
// characterization tests seed. All rows carry the pair's unique label so a
// cleanup can remove exactly what this test created.
type tenantPair struct {
	entityA, entityB string
	storeA, storeB   string
	contractA        string
	landlordA        string
}

// seedTenantPair creates two legal entities, one store and one contract per
// entity. Entity A owns its rows; entity B is the isolation probe.
func seedTenantPair(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) tenantPair {
	t.Helper()
	suffix := uuidSuffix()
	var pair tenantPair
	for _, side := range []struct {
		label     string
		entityRef *string
		storeRef  *string
	}{
		{"a", &pair.entityA, &pair.storeA},
		{"b", &pair.entityB, &pair.storeB},
	} {
		var entityID, storeID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO legal_entities (code, name, country, currency, is_active)
			VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
		`, "CHAR-LE-"+side.label+"-"+suffix, "Characterization tenant "+side.label).Scan(&entityID); err != nil {
			t.Fatalf("seed legal entity %s: %v", side.label, err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
			VALUES ($1, $2, $3, $4, $5, true) RETURNING id
		`, "CHAR-ST-"+side.label+"-"+suffix, "Characterization store "+side.label, entityID, "Brand-"+side.label, "Region-"+side.label).Scan(&storeID); err != nil {
			t.Fatalf("seed store %s: %v", side.label, err)
		}
		*side.entityRef = entityID
		*side.storeRef = storeID
	}
	var landlordID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO landlords (code, name) VALUES ($1, $2) RETURNING id
	`, "CHAR-LL-"+suffix, "Characterization landlord").Scan(&landlordID); err != nil {
		t.Fatalf("seed landlord: %v", err)
	}
	pair.landlordA = landlordID
	if err := pool.QueryRow(ctx, `
		INSERT INTO lease_contracts (contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, currency, commencement_date, lease_start_date, lease_end_date, lease_scope)
		VALUES ($1, $2, $3, $4, $5, 'store', 'CNY', '2026-01-01', '2026-01-01', '2027-12-31', 'in_scope')
		RETURNING id
	`, "CHAR-CT-A-"+suffix, "Characterization contract A", pair.entityA, pair.storeA, landlordID).Scan(&pair.contractA); err != nil {
		t.Fatalf("seed contract A: %v", err)
	}
	return pair
}

func uuidSuffix() string {
	return uuid.NewString()[:8]
}

// cleanupTenantPair removes every row created for the pair. The order follows
// foreign-key dependencies: children before parents.
func cleanupTenantPair(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pair tenantPair) {
	t.Helper()
	entities := []string{pair.entityA, pair.entityB}
	stores := []string{pair.storeA, pair.storeB}
	contracts := []string{pair.contractA}
	statements := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM critical_dates WHERE contract_id = ANY($1::uuid[])`, []any{contracts}},
		{`DELETE FROM measurement_results WHERE contract_id = ANY($1::uuid[])`, []any{contracts}},
		{`DELETE FROM renewal_decision_snapshots WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM budget_versions WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_plan_lines WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_plan_versions WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_data_quality_items WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_master_data_mappings WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_decision_memos WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_report_artifacts WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_agent_signals WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_action_items WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM fpna_assumption_versions WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM store_operating_facts WHERE store_id = ANY($1::uuid[])`, []any{stores}},
		{`DELETE FROM equipment_operating_facts WHERE equipment_id IN (SELECT id FROM equipment_assets WHERE legal_entity_id = ANY($1::uuid[]))`, []any{entities}},
		{`DELETE FROM equipment_assets WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM operating_fact_batches WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM retail_store_day_facts WHERE store_id = ANY($1::uuid[])`, []any{stores}},
		{`DELETE FROM retail_store_day_fact_requests WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM lease_contracts WHERE legal_entity_id = ANY($1::uuid[])`, []any{entities}},
		{`DELETE FROM landlords WHERE id = $1`, []any{pair.landlordA}},
		{`DELETE FROM stores WHERE id = ANY($1::uuid[])`, []any{stores}},
		{`DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`, []any{entities}},
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(context.Background(), stmt.sql, stmt.args...); err != nil {
			t.Logf("cleanup statement failed (non-fatal): %v", err)
		}
	}
}
