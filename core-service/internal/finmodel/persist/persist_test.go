package persist_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/persist"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestWriterRefusesFailedTieOuts(t *testing.T) {
	writer := persist.NewRunWriter(nil)
	result := &finmodel.RunResult{TieOutStatus: "failed"}
	err := writer.Persist(context.Background(), finmodel.ModelDef{}, result, "model-1", "k1", nil)
	if !errors.Is(err, persist.ErrTieOutFailed) {
		t.Fatalf("failed tie-outs must block Persist, got %v", err)
	}
}

// TestPersistIntoCompletesQueuedRunPostgres locks the async completion
// half (S2-5): a queued run row plus a tie-out-passed result becomes
// completed with persisted lines and tie-outs — the exact contract the
// worker's PersistInto call relies on.
func TestPersistIntoCompletesQueuedRunPostgres(t *testing.T) {
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
		entity, "PI-E-"+suffix, "PersistInto "+suffix)
	tmplID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'approved','{"rows":[]}'::jsonb)`, tmplID, entity, "PI-TPL-"+suffix)
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "PI-DEF-"+suffix, tmplID)
	runID := uuid.NewString()
	exec(`UPDATE fin_statement_templates SET rows='{"rows":[]}'::jsonb WHERE id=$1`, tmplID)
	exec(`INSERT INTO fin_model_runs (id, legal_entity_id, model_definition_id, model_definition_version, status, tie_out_status, input_snapshot, idempotency_key)
		VALUES ($1,$2,$3,1,'queued','pending','{"currency":"CNY"}'::jsonb,$4)`, runID, entity, defID, "pi-run-"+suffix)

	failed := &finmodel.RunResult{TieOutStatus: "failed", Versions: finmodel.VersionSet{}}
	if err := persist.NewRunWriter(repository.NewFinModelRepository(pool)).PersistInto(ctx, runID, failed); err == nil {
		t.Fatal("a failed tie-out result must be refused even on the async path")
	}

	passed := &finmodel.RunResult{
		TieOutStatus: "passed", Versions: finmodel.VersionSet{Data: "d1"},
		Lines: []finmodel.LineValue{
			{RowKey: "rev", Period: "2026-07", Value: fpv(120), Provenance: finmodel.Provenance{}},
			{RowKey: "rev", Period: "2026-08", Value: fpv(130), Provenance: finmodel.Provenance{}},
		},
		TieOuts: []finmodel.TieOutResult{
			{CheckCode: "T1", Period: "2026-07", Expected: fpv(10), Actual: fpv(10), Diff: fpv(0), Status: "passed"},
		},
	}
	if err := persist.NewRunWriter(repository.NewFinModelRepository(pool)).PersistInto(ctx, runID, passed); err != nil {
		t.Fatalf("PersistInto: %v", err)
	}
	run, err := repository.NewFinModelRepository(pool).GetModelRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || run.TieOutStatus != "passed" || run.CompletedAt == nil {
		t.Fatalf("run after PersistInto = %+v", run)
	}
	lines, err := repository.NewFinModelRepository(pool).ListRunLines(ctx, runID)
	if err != nil || len(lines) != 2 {
		t.Fatalf("persisted lines = %d (%v)", len(lines), err)
	}
}

func fpv(v float64) *float64 { return &v }
