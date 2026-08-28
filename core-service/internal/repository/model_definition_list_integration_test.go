package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestListModelDefinitionsPostgres locks the definitions listing surface
// (FP&A 反馈 2026-08-27 P0-3 前置：GET /financial-model/definitions 曾是
// 返回空数组的桩): rows come back tenant-scoped, admin (nil tenant) sees
// every entity. Bottom line 1 evidence lives here, not in unit tests.
func TestListModelDefinitionsPostgres(t *testing.T) {
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
	entityA := uuid.NewString()
	entityB := uuid.NewString()
	for _, e := range []struct{ id, tag string }{{entityA, "LMD-A-"}, {entityB, "LMD-B-"}} {
		exec(`INSERT INTO legal_entities (id, code, name, country, currency)
			VALUES ($1,$2,$3,'CN','CNY')`, e.id, e.tag+suffix, e.tag+suffix)
	}
	templateID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'draft','{"rows":[]}'::jsonb)`, templateID, entityA, "LMD-TPL-"+suffix)
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, status)
		VALUES ($1,$2,$3,1,$4,'draft')`, defID, entityA, "LMD-DEF-"+suffix, templateID)

	repo := NewFinModelRepository(pool)

	scoped, err := repo.ListModelDefinitions(ctx, &entityA)
	if err != nil {
		t.Fatalf("scoped list: %v", err)
	}
	if len(scoped) != 1 || scoped[0].ID != defID || scoped[0].LegalEntityID != entityA {
		t.Fatalf("scoped list = %+v", scoped)
	}

	otherScoped, err := repo.ListModelDefinitions(ctx, &entityB)
	if err != nil {
		t.Fatalf("other-entity list: %v", err)
	}
	for _, row := range otherScoped {
		if row.LegalEntityID == entityA && row.ID == defID {
			t.Fatalf("entity B leaked entity A's definition: %+v", row)
		}
	}

	adminView, err := repo.ListModelDefinitions(ctx, nil)
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	found := false
	for _, row := range adminView {
		if row.ID == defID {
			found = true
		}
	}
	if !found {
		t.Fatalf("admin view lost the seeded definition (%d rows scanned)", len(adminView))
	}
}
