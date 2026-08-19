package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTemplateCopyAndDeletePostgres locks the S3-4 remaining rules: copying
// continues the same-name lineage at version+1 and starts a fresh lineage
// for a new name (copied_from set either way); deletion succeeds only for
// drafts no model definition ever bound — an in-use or non-draft template
// is refused with a distinguishable reason.
func TestTemplateCopyAndDeletePostgres(t *testing.T) {
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
	exec(`INSERT INTO legal_entities (id, code, name, country, currency)
		VALUES ($1,$2,$3,'CN','CNY')`, entity, "TCD-E-"+suffix, "CopyDel "+suffix)
	reviewerID := uuid.NewString()
	approverID := uuid.NewString()
	exec(`INSERT INTO users (id, username, email, password_hash, legal_entity_id)
		VALUES ($1,$2,$3,'integration-only',$4), ($5,$6,$7,'integration-only',$4)`,
		reviewerID, "tcd-reviewer-"+suffix, "tcd-reviewer-"+suffix+"@example.com", entity,
		approverID, "tcd-approver-"+suffix, "tcd-approver-"+suffix+"@example.com")
	sourceID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,2,'draft','{"rows":[]}'::jsonb)`, sourceID, entity, "TCD-TPL-"+suffix)

	repo := NewFinModelRepository(pool)

	// 同名复制：续谱系 version 2 → 3，copied_from 指向源。
	sameID := uuid.NewString()
	if err := repo.CopyStatementTemplate(ctx, sameID, "TCD-TPL-"+suffix, 3, &sourceID, nil); err != nil {
		t.Fatalf("same-name copy: %v", err)
	}
	same, err := repo.GetStatementTemplate(ctx, sameID)
	if err != nil {
		t.Fatal(err)
	}
	if same.Version != 3 || same.Status != "draft" || same.CopiedFrom == nil || *same.CopiedFrom != sourceID {
		t.Fatalf("same-name copy = %+v", same)
	}

	// 异名复制：新谱系 version 1，仍带 copied_from。
	newID := uuid.NewString()
	if err := repo.CopyStatementTemplate(ctx, newID, "TCD-NEW-"+suffix, 1, &sourceID, nil); err != nil {
		t.Fatalf("new-name copy: %v", err)
	}
	fresh, err := repo.GetStatementTemplate(ctx, newID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Version != 1 || fresh.Name != "TCD-NEW-"+suffix || fresh.CopiedFrom == nil || *fresh.CopiedFrom != sourceID {
		t.Fatalf("new-name copy = %+v", fresh)
	}

	// 删除守卫一：被模型定义引用的模板（即使 draft）拒绝删除。
	inUseID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'draft','{"rows":[]}'::jsonb)`, inUseID, entity, "TCD-INUSE-"+suffix)
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "TCD-DEF-"+suffix, inUseID)
	if err := repo.DeleteStatementTemplate(ctx, inUseID); !errors.Is(err, ErrStatementTemplateInUse) {
		t.Fatalf("in-use template deletion = %v, want ErrStatementTemplateInUse", err)
	}

	// 删除守卫二：非 draft 拒绝（先走复核/批准到 approved）。
	if err := repo.ReviewStatementTemplate(ctx, sourceID, reviewerID, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApproveStatementTemplate(ctx, sourceID, approverID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteStatementTemplate(ctx, sourceID); !errors.Is(err, ErrInvalidWorkflowTransition) {
		t.Fatalf("approved template deletion = %v, want ErrInvalidWorkflowTransition", err)
	}

	// 干净草稿可删，删除后读取即空。
	orphanID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'draft','{"rows":[]}'::jsonb)`, orphanID, entity, "TCD-ORPHAN-"+suffix)
	if err := repo.DeleteStatementTemplate(ctx, orphanID); err != nil {
		t.Fatalf("unused draft deletion: %v", err)
	}
	if _, err := repo.GetStatementTemplate(ctx, orphanID); err == nil {
		t.Fatal("deleted template still readable")
	}
}
