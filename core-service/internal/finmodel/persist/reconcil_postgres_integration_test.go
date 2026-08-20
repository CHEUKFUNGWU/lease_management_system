package persist

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/repository"
)

// TestRecordReconciliationIssuesPostgres locks P1-1: a run with a failed T13
// (Actual vs facts mismatch) and a failed opening gate③ land in
// fpna_data_quality_items as category=reconciliation rows carrying their
// source table / record / data version — the mismatch stays visible even
// though the failing run never publishes.
func TestRecordReconciliationIssuesPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v", err)
		}
	}
	suffix := uuid.NewString()[:8]
	entity := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "REC-E-"+suffix, "Reconcil "+suffix)
	repo := repository.NewFinModelRepository(pool)
	sourceID := "run-" + uuid.NewString()[:8]

	// T13 failed（占用成本 mismatch）→ 一条 reconciliation 行。
	tieOuts := []finmodel.TieOutResult{
		{CheckCode: "T13", Period: "2026-07", Expected: pf(200), Actual: pf(233), Diff: pf(33), Status: "failed"},
		{CheckCode: "T13", Period: "2026-08", Expected: pf(210), Actual: pf(210), Diff: pf(0), Status: "passed"},
		{CheckCode: "T1", Period: "2026-07", Status: "passed"},
	}
	if err := RecordReconciliationIssues(ctx, repo, entity, sourceID, "ds-1", tieOuts); err != nil {
		t.Fatalf("record: %v", err)
	}
	var category, sourceTable, sourceRecordID, dataVersion string
	var period string
	err = pool.QueryRow(ctx, `SELECT category, source_table, source_record_id, data_version, period
		FROM fpna_data_quality_items WHERE legal_entity_id=$1 AND category='reconciliation' ORDER BY created_at DESC LIMIT 1`, entity).
		Scan(&category, &sourceTable, &sourceRecordID, &dataVersion, &period)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if category != "reconciliation" || sourceTable != "fin_model_tie_outs" || sourceRecordID != sourceID ||
		dataVersion != "ds-1" || period != "2026-07" {
		t.Fatalf("reconciliation row broken: %s/%s/%s/%s/%s", category, sourceTable, sourceRecordID, dataVersion, period)
	}

	// opening gate③ failure → 一条 reconciliation 行（PRD S2-3：不符进队列）。
	gateFailures := []opening.GateFailure{
		{Gate: "3", Period: "2026-07", Diff: 25, Detail: "合同 C1 期初租赁负债 475 ≠ 引擎 500"},
	}
	if err := RecordOpeningGateIssues(ctx, repo, entity, sourceID, gateFailures); err != nil {
		t.Fatalf("record opening: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM fpna_data_quality_items
		WHERE legal_entity_id=$1 AND category='reconciliation' AND source_table='fin_model_opening'`, entity).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("opening gate issue must be queued, count=%d", count)
	}
}
