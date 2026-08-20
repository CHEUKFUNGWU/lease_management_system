package persist

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// TestPublishLineageAndReplayPostgres locks S2-7 against the real schema:
// a tied-out run publishes a forecast plan_version with group-grain lines,
// the next publish links prior_version_id to it (lineage chain), the tie-out
// gate refuses unpassed runs, and republishing the same run is idempotent.
func TestPublishLineageAndReplayPostgres(t *testing.T) {
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
		entity, "PUB-E-"+suffix, "Publish "+suffix)
	tmplID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'approved','{"rows":[]}'::jsonb)`, tmplID, entity, "PUB-TPL-"+suffix)
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "PUB-DEF-"+suffix, tmplID)

	makeRun := func(tieOut, classification string) string {
		id := uuid.NewString()
		exec(`INSERT INTO fin_model_runs (id, legal_entity_id, model_definition_id, model_definition_version, status, tie_out_status, data_classification, input_snapshot, idempotency_key)
			VALUES ($1,$2,$3,1,'completed',$4,$5,'{"currency":"CNY"}'::jsonb,$6)`,
			id, entity, defID, tieOut, classification, "pub-run-"+id[:8])
		exec(`INSERT INTO fin_model_run_lines (run_id, row_key, period, value, provenance) VALUES
			($1,'rev','2026-07',100,'{}'::jsonb),
			($1,'gp','2026-07',40,'{}'::jsonb),
			($1,'operating_ebitda','2026-07',25,'{}'::jsonb),
			($1,'custom_ratio','2026-07',0.05,'{}'::jsonb),
			($1,'rev','2026-08',110,'{}'::jsonb)`, id)
		return id
	}

	runs := repository.NewFinModelRepository(pool)
	plans := repository.NewFPnAGovernanceRepository(pool)
	writer := NewPublishWriter(runs, plans)

	// 门：未通过勾稽的 run 不得发布。
	failedRun := makeRun("failed", "production")
	if _, err := writer.Publish(ctx, failedRun, nil, ""); err == nil {
		t.Fatal("unpassed tie-outs must refuse publish (S2-6 gate)")
	}

	// 底线 2：模拟 / 混合 run 即使勾稽通过也不得发布为 plan version。
	for _, simulated := range []string{"simulated", "mixed"} {
		if _, err := writer.Publish(ctx, makeRun("passed", simulated), nil, ""); !errors.Is(err, ErrSimulatedPublish) {
			t.Fatalf("classification %q must be refused at publish, got %v", simulated, err)
		}
	}

	// 发布 1：谱系为空。
	runA := makeRun("passed", "production")
	first, err := writer.Publish(ctx, runA, nil, "")
	if err != nil {
		t.Fatalf("publish runA: %v", err)
	}
	if first.LineCount != 2 {
		t.Fatalf("runA lines = %d, want 2 periods", first.LineCount)
	}
	if len(first.UnmappedRows) != 1 || first.UnmappedRows[0] != "custom_ratio" {
		t.Fatalf("custom row must be reported unmapped, got %v", first.UnmappedRows)
	}
	v1, err := plans.FindPlanVersionBySource(ctx, publishSourcePrefix+runA)
	if err != nil {
		t.Fatalf("find runA version: %v", err)
	}
	if v1.PriorVersionID != nil {
		t.Fatalf("first publish must have no prior, got %v", v1.PriorVersionID)
	}
	lines, err := plans.ListPlanLines(ctx, v1.ID, access.GlobalEntityFilter(), "", "group")
	if err != nil || len(lines) != 2 {
		t.Fatalf("runA plan lines = %d (%v)", len(lines), err)
	}
	if lines[0].Revenue == nil || *lines[0].Revenue != 100 || !lines[0].ForecastFlag || lines[0].ActualFlag {
		t.Fatalf("plan line must carry forecast revenue, got %+v", lines[0])
	}

	// 发布 2：prior 指向发布 1。
	runB := makeRun("passed", "production")
	if _, err := writer.Publish(ctx, runB, nil, "upside"); err != nil {
		t.Fatalf("publish runB: %v", err)
	}
	v2, err := plans.FindPlanVersionBySource(ctx, publishSourcePrefix+runB)
	if err != nil {
		t.Fatal(err)
	}
	if v2.PriorVersionID == nil || *v2.PriorVersionID != v1.ID {
		t.Fatalf("lineage must chain runB → runA, got %v", v2.PriorVersionID)
	}
	if v2.ScenarioType != "upside" {
		t.Fatalf("scenario_type must ride the published version, got %q", v2.ScenarioType)
	}

	// 非法情景拒绝，不静默归为 baseline。
	if _, err := writer.Publish(ctx, makeRun("passed", "production"), nil, "optimistic"); err == nil {
		t.Fatal("an unknown scenario_type must be refused")
	}

	// 幂等：同一 run 重复发布 = 同一版本，不产生第二条谱系。
	replay, err := writer.Publish(ctx, runB, nil, "upside")
	if err != nil || replay.VersionID != v2.ID {
		t.Fatalf("replay must return the same version, got %+v err=%v", replay, err)
	}
}
