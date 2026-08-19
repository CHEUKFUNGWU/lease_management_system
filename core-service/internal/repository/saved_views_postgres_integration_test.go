package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func execFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("fixture exec: %v", err)
	}
}

// TestTemplateGovernanceTransitionsPostgres locks the S3-4 state machine:
// approve-before-review is refused, review stamps the reviewer, a rejected
// review returns to draft, approval stamps the approver and a second
// approval is refused — all enforced by the FROM-status guards in SQL.
func TestTemplateGovernanceTransitionsPostgres(t *testing.T) {
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

	suffix := uuid.NewString()[:8]
	entityID := uuid.NewString()
	creatorID := uuid.NewString()
	reviewerID := uuid.NewString()
	approverID := uuid.NewString()
	execFixture(t, ctx, pool, `INSERT INTO legal_entities (id, code, name, country, currency)
		VALUES ($1, $2, $3, 'CN', 'CNY')`, entityID, "TG-E-"+suffix, "Governance "+suffix)
	execFixture(t, ctx, pool, `INSERT INTO users (id, username, email, password_hash, legal_entity_id)
		VALUES ($1,$2,$3,'integration-only',$4), ($5,$6,$7,'integration-only',$4), ($8,$9,$10,'integration-only',$4)`,
		creatorID, "tg-creator-"+suffix, "tg-creator-"+suffix+"@example.com", entityID,
		reviewerID, "tg-reviewer-"+suffix, "tg-reviewer-"+suffix+"@example.com",
		approverID, "tg-approver-"+suffix, "tg-approver-"+suffix+"@example.com")

	templateID := uuid.NewString()
	execFixture(t, ctx, pool, `INSERT INTO fin_statement_templates
		(id, legal_entity_id, name, version, status, rows, created_by)
		VALUES ($1, $2, $3, 1, 'draft', '{"rows":[]}'::jsonb, $4)`,
		templateID, entityID, "TG Template "+suffix, creatorID)

	repo := NewFinModelRepository(pool)

	if err := repo.ApproveStatementTemplate(ctx, templateID, approverID); !errors.Is(err, ErrInvalidWorkflowTransition) {
		t.Fatalf("approve before review = %v, want ErrInvalidWorkflowTransition", err)
	}
	if err := repo.ReviewStatementTemplate(ctx, templateID, reviewerID, true); err != nil {
		t.Fatalf("review approve: %v", err)
	}
	row, err := repo.GetStatementTemplate(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "review" || row.ReviewedBy == nil || *row.ReviewedBy != reviewerID {
		t.Fatalf("after review: status=%s reviewed_by=%v", row.Status, row.ReviewedBy)
	}
	if err := repo.ReviewStatementTemplate(ctx, templateID, reviewerID, false); err != nil {
		t.Fatalf("review reject: %v", err)
	}
	row, _ = repo.GetStatementTemplate(ctx, templateID)
	if row.Status != "draft" {
		t.Fatalf("after review reject: status=%s, want draft", row.Status)
	}
	if err := repo.ReviewStatementTemplate(ctx, templateID, reviewerID, true); err != nil {
		t.Fatalf("re-review approve: %v", err)
	}
	if err := repo.ApproveStatementTemplate(ctx, templateID, approverID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	row, _ = repo.GetStatementTemplate(ctx, templateID)
	if row.Status != "approved" || row.ApprovedBy == nil || *row.ApprovedBy != approverID {
		t.Fatalf("after approve: status=%s approved_by=%v", row.Status, row.ApprovedBy)
	}
	if err := repo.ApproveStatementTemplate(ctx, templateID, approverID); !errors.Is(err, ErrInvalidWorkflowTransition) {
		t.Fatalf("second approve = %v, want ErrInvalidWorkflowTransition", err)
	}
}

// TestSavedViewsOwnershipPostgres locks the S3-5 access rules: personal
// views are invisible to other users until shared, mutations are
// owner-only even when shared, the default slot is per-owner and strictly
// unique, and sharing never moves data because the row holds config only.
func TestSavedViewsOwnershipPostgres(t *testing.T) {
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

	suffix := uuid.NewString()[:8]
	entityID := uuid.NewString()
	user1ID := uuid.NewString()
	user2ID := uuid.NewString()
	execFixture(t, ctx, pool, `INSERT INTO legal_entities (id, code, name, country, currency)
		VALUES ($1, $2, $3, 'CN', 'CNY')`, entityID, "SV-E-"+suffix, "Views "+suffix)
	execFixture(t, ctx, pool, `INSERT INTO users (id, username, email, password_hash, legal_entity_id)
		VALUES ($1,$2,$3,'integration-only',$4), ($5,$6,$7,'integration-only',$4)`,
		user1ID, "sv-u1-"+suffix, "sv-u1-"+suffix+"@example.com", entityID,
		user2ID, "sv-u2-"+suffix, "sv-u2-"+suffix+"@example.com")

	repo := NewFinModelRepository(pool)
	cfg, _ := json.Marshal(map[string]any{"basis_mode": "working", "period_from": "2026-01", "period_to": "2026-12"})

	viewA := &SavedView{
		ID: uuid.NewString(), LegalEntityID: entityID, Kind: "store_pnl",
		Name: "我的默认视图", Config: cfg, CreatedBy: &user1ID, IsDefault: true,
	}
	if err := repo.CreateSavedView(ctx, viewA); err != nil {
		t.Fatalf("create view A: %v", err)
	}
	viewB := &SavedView{
		ID: uuid.NewString(), LegalEntityID: entityID, Kind: "store_pnl",
		Name: "第二个视图", Config: cfg, CreatedBy: &user1ID, IsDefault: true,
	}
	if err := repo.CreateSavedView(ctx, viewB); err != nil {
		t.Fatalf("create view B: %v", err)
	}
	// The partial unique index guarantees only one default per owner.
	var defaults int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fin_saved_views WHERE created_by=$1 AND is_default`, user1ID).Scan(&defaults); err != nil {
		t.Fatal(err)
	}
	if defaults != 1 {
		t.Fatalf("defaults for user1 = %d, want exactly 1", defaults)
	}
	if a, _ := repo.GetSavedViewForUser(ctx, viewA.ID, &entityID, user1ID); a == nil || a.IsDefault {
		t.Fatalf("expected view A to have lost its default flag, got %+v", a)
	}

	// Personal views are invisible to other users until shared.
	if _, err := repo.GetSavedViewForUser(ctx, viewB.ID, &entityID, user2ID); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatalf("unshared view visible to user2: %v", err)
	}
	if err := repo.ShareSavedView(ctx, viewB.ID, user1ID, true); err != nil {
		t.Fatalf("share: %v", err)
	}
	seen, err := repo.GetSavedViewForUser(ctx, viewB.ID, &entityID, user2ID)
	if err != nil {
		t.Fatalf("shared view not visible to user2: %v", err)
	}
	if seen.Name != "第二个视图" {
		t.Fatalf("shared view name mangled: %q", seen.Name)
	}

	// Mutations stay owner-only even when shared.
	if err := repo.UpdateSavedView(ctx, viewB.ID, user2ID, strPtr("劫持"), cfg); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatal("user2 updated a shared view they do not own")
	}
	if err := repo.SetDefaultSavedView(ctx, viewB.ID, "store_pnl", user2ID); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatal("user2 defaulted a shared view they do not own")
	}
	if err := repo.DeleteSavedView(ctx, viewB.ID, user2ID); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatal("user2 deleted a shared view they do not own")
	}

	// Owner update and delete both apply.
	if err := repo.UpdateSavedView(ctx, viewB.ID, user1ID, strPtr("改名"), cfg); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	after, _ := repo.GetSavedViewForUser(ctx, viewB.ID, &entityID, user1ID)
	if after.Name != "改名" {
		t.Fatalf("updated name = %q", after.Name)
	}
	if err := repo.DeleteSavedView(ctx, viewB.ID, user1ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, err := repo.GetSavedViewForUser(ctx, viewB.ID, &entityID, user2ID); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatal("deleted view still visible")
	}

	// Entity isolation: another tenant cannot see the shared view's rows.
	otherEntity := uuid.NewString()
	execFixture(t, ctx, pool, `INSERT INTO legal_entities (id, code, name, country, currency)
		VALUES ($1, $2, $3, 'CN', 'CNY')`, otherEntity, "SV-F-"+suffix, "ViewsOther "+suffix)
	if _, err := repo.GetSavedViewForUser(ctx, viewA.ID, &otherEntity, user2ID); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatal("cross-tenant view access succeeded")
	}
}

func strPtr(s string) *string { return &s }
