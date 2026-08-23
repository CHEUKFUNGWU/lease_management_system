package draftreview

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// ── 端到端集成证据（真库）──────────────────────────────────────────────────
//
// 单元测试的 fake 只能证明服务逻辑自洽；底线 4（幂等落库）、部分失败的持久
// 化效果、reviewed_by 外键，都要在真实 Postgres 上复验。TEST_DATABASE_URL
// 未设置时这些用例 skip —— skip 不构成证据。

func draftReviewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type pgDraftReviewUOW struct{ inner *draftapp.PostgresUnitOfWork }

func (u pgDraftReviewUOW) Execute(ctx context.Context, fn func(ContractStore) error) error {
	return u.inner.Execute(ctx, func(store draftapp.DraftStore) error { return fn(store) })
}

const e2ePayload = `{
	"contract_number": "DRAFTREV-E2E-1",
	"contract_name": "集成测试租约",
	"lessee_name": "承租方甲",
	"lessor_name": "出租方乙",
	"store_name": "集成测试门店",
	"lease_scope": "in_scope",
	"currency": "CNY",
	"commencement_date": "2026-01-01T00:00:00Z",
	"lease_start_date": "2026-01-01T00:00:00Z",
	"lease_end_date": "2027-12-31T00:00:00Z"
}`

func seedE2EDraft(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entityID string, payload string, confidence string) (draftID, taskID string) {
	t.Helper()
	if err := pool.QueryRow(ctx,
		`INSERT INTO ai_tasks (task_type, status) VALUES ('contract_parse', 'completed') RETURNING id`,
	).Scan(&taskID); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_contract_drafts (task_id, contract_data, confidence_scores, status, legal_entity_id, data_classification)
		VALUES ($1::uuid, $2::jsonb, $3::jsonb, 'pending', $4::uuid, 'production') RETURNING id`,
		taskID, payload, confidence, entityID).Scan(&draftID); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_draft_idempotency WHERE operation='draftreview.approve' AND idempotency_key=$1`, "contract-draft:"+draftID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_contracts WHERE contract_number LIKE 'DRAFTREV-E2E%'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_contract_drafts WHERE id=$1`, draftID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_tasks WHERE id=$1`, taskID)
	})
	return draftID, taskID
}

func newE2EService(t *testing.T, pool *pgxpool.Pool) *Service {
	t.Helper()
	contractRepo := repository.NewContractRepository(pool)
	uow := pgDraftReviewUOW{inner: draftapp.NewPostgresUnitOfWork(pool, contractRepo, repository.NewPaymentScheduleRepository(pool))}
	return NewService(contractRepo, uow)
}

// 验收 7 + 底线 4：同一草稿批准两次，正式记录只有一条。
func TestE2EDoubleApproveCreatesSingleFormalRecord(t *testing.T) {
	pool := draftReviewTestPool(t)
	ctx := context.Background()
	entityA := seedIsolatedEntity(t, ctx, pool)
	draftID, _ := seedE2EDraft(t, ctx, pool, entityA, e2ePayload, `{}`)
	service := newE2EService(t, pool)

	first, err := service.Decide(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}),
		[]Decision{{DraftID: draftID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Items[0].Verdict != "approved" {
		t.Fatalf("first approve: %+v", first.Items[0])
	}
	second, err := service.Decide(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}),
		[]Decision{{DraftID: draftID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Items[0].Verdict != "approved" {
		t.Fatalf("second approve should replay as approved: %+v", second.Items[0])
	}

	var formalCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lease_contracts WHERE contract_number='DRAFTREV-E2E-1'`).Scan(&formalCount); err != nil || formalCount != 1 {
		t.Fatalf("formal records = %d (err %v); want exactly 1", formalCount, err)
	}
	var idemCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_draft_idempotency WHERE operation='draftreview.approve' AND idempotency_key=$1`,
		"contract-draft:"+draftID).Scan(&idemCount); err != nil || idemCount != 1 {
		t.Fatalf("idempotency rows = %d (err %v); want exactly 1", idemCount, err)
	}
	var draftStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_contract_drafts WHERE id=$1`, draftID).Scan(&draftStatus); err != nil || draftStatus != "approved" {
		t.Fatalf("draft status = %q (err %v)", draftStatus, err)
	}
	// 正式记录落在草稿所属法人名下（隔离锚点在真库上成立）。
	var formalEntity string
	if err := pool.QueryRow(ctx, `SELECT legal_entity_id::text FROM lease_contracts WHERE contract_number='DRAFTREV-E2E-1'`).Scan(&formalEntity); err != nil || formalEntity != entityA {
		t.Fatalf("formal record legal_entity_id=%q want %q (err=%v)", formalEntity, entityA, err)
	}
}

// 验收 3 + 验收 5：跨法人 Decide 被拒且措辞不软化；低置信未确认被拒。
func TestE2ECrossTenantAndConfidenceGate(t *testing.T) {
	pool := draftReviewTestPool(t)
	ctx := context.Background()
	entityA := seedIsolatedEntity(t, ctx, pool)
	entityB := seedIsolatedEntity(t, ctx, pool)
	lowConfPayload := strings.Replace(e2ePayload, "DRAFTREV-E2E-1", "DRAFTREV-E2E-2", 1)
	draftID, _ := seedE2EDraft(t, ctx, pool, entityA, lowConfPayload, `{"lessee_name":0.42}`)
	service := newE2EService(t, pool)

	// 法人 B 直接调 Decide 批准法人 A 的草稿：拒绝 + scope_denied 原文。
	outcome, err := service.Decide(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}),
		[]Decision{{DraftID: draftID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Items[0].Verdict != "failed" ||
		outcome.Items[0].Error != ErrScopeDenied.Error() {
		t.Fatalf("cross-tenant decide not rejected with exact wording: %+v", outcome.Items[0])
	}
	var formalCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lease_contracts WHERE contract_number='DRAFTREV-E2E-2'`).Scan(&formalCount); err != nil || formalCount != 0 {
		t.Fatalf("cross-tenant approve created a formal record: %d", formalCount)
	}

	// 低置信字段未经 Revise 确认：法人 A 自己批准也被拒。
	gateOutcome, err := service.Decide(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}),
		[]Decision{{DraftID: draftID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if gateOutcome.Items[0].Verdict != "failed" ||
		!strings.Contains(gateOutcome.Items[0].Error, "low_confidence_fields_unconfirmed") {
		t.Fatalf("confidence gate did not hold on real DB: %+v", gateOutcome.Items[0])
	}

	// Revise 确认后同一条放行——闸的唯一出口是 Revise。
	reviewerCtx := WithReviewer(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), reviewerUserID(t, ctx, pool, entityA))
	if _, err := service.Revise(reviewerCtx, draftID, []FieldEdit{{Field: "lessee_name", Value: "承租方甲（确认）", Confirmed: true}}); err != nil {
		t.Fatalf("revise: %v", err)
	}
	finalOutcome, err := service.Decide(reviewerCtx, []Decision{{DraftID: draftID, Approve: true}})
	if err != nil || finalOutcome.Items[0].Verdict != "approved" {
		t.Fatalf("post-revision approve failed: %+v err=%v", finalOutcome.Items[0], err)
	}
	var humanLayer struct {
		HumanEdits map[string]struct {
			Value     string `json:"value"`
			Confirmed bool   `json:"confirmed"`
		} `json:"human_edits"`
		AiLessee string `json:"lessee_name"`
	}
	row, err := repository.NewContractRepository(pool).GetDraftForReview(
		access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), mustFilter(entityA), draftID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(row.ContractData, &humanLayer); err != nil {
		t.Fatal(err)
	}
	if humanLayer.AiLessee == "" || humanLayer.HumanEdits["lessee_name"].Value != "承租方甲（确认）" {
		t.Fatalf("差异留痕 broken in DB: %+v", humanLayer)
	}
}

// 验收 6：批量部分失败——第 N 条失败前 N-1 条已入库，续跑不重复。
func TestE2EBatchPartialFailureThenResume(t *testing.T) {
	pool := draftReviewTestPool(t)
	ctx := context.Background()
	entityA := seedIsolatedEntity(t, ctx, pool)
	goodID, _ := seedE2EDraft(t, ctx, pool, entityA, e2ePayload, `{}`)
	if !strings.Contains(e2ePayload, `"lessee_name": "承租方甲"`) {
		t.Fatal("e2ePayload 格式变化，替换锚点失效")
	}
	badPayload := strings.Replace(e2ePayload, `"lessee_name": "承租方甲"`, `"lessee": "AI 键名"`, 1) // 键名错位 → 校验失败
	badID, _ := seedE2EDraft(t, ctx, pool, entityA, badPayload, `{}`)
	service := newE2EService(t, pool)
	caller := access.WithScope(ctx, access.Scope{LegalEntityID: entityA})

	outcome, err := service.Decide(caller, []Decision{{DraftID: goodID, Approve: true}, {DraftID: badID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Items[0].Verdict != "approved" || outcome.Items[1].Verdict != "failed" ||
		!strings.Contains(outcome.Items[1].Error, "missing lessee_name") {
		t.Fatalf("partial failure shape wrong: %+v", outcome.Items)
	}

	resume, err := service.Decide(caller, []Decision{{DraftID: goodID, Approve: true}})
	if err != nil {
		t.Fatal(err)
	}
	if resume.Items[0].Verdict != "approved" {
		t.Fatalf("resume failed: %+v", resume.Items[0])
	}
	var goodCount, badCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lease_contracts WHERE contract_number='DRAFTREV-E2E-1'`).Scan(&goodCount); err != nil || goodCount != 1 {
		t.Fatalf("good draft formal records = %d; want 1 (resume duplicated?)", goodCount)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lease_contracts WHERE contract_number LIKE 'DRAFTREV-E2E%' AND contract_number <> 'DRAFTREV-E2E-1'`).Scan(&badCount); err != nil || badCount != 0 {
		t.Fatalf("misaligned draft leaked a formal record: %d", badCount)
	}
}

// ── seeding helpers ────────────────────────────────────────────────────────

func seedIsolatedEntity(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := uuid.NewString()[:8]
	var entityID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"DRAFTREV-E-"+suffix, "DraftReview E2E tenant "+suffix).Scan(&entityID); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id=$1`, entityID) })
	return entityID
}

func reviewerUserID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entityID string) string {
	t.Helper()
	suffix := uuid.NewString()[:8]
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"draftrev-e2e-"+suffix, "draftrev-e2e-"+suffix+"@test.local", entityID).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	return userID
}

func mustFilter(id string) access.EntityFilter {
	filter, err := access.EntityFilterFor(id)
	if err != nil {
		panic(err)
	}
	return filter
}
