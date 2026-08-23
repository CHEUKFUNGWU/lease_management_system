package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
)

// ── Ch2 草稿复核工作台 · 集成证据 ──────────────────────────────────────────
//
// 这些断言证明的是底线 1（跨法人隔离、NULL 行 fail-closed）在真实 Postgres
// 上成立——单元测试里的 fake 只能模拟 SQL 语义，不能替代这里。skip 掉的集
// 成测试不构成任何证据：确认输出里这些用例真的 RUN 过。

type draftReviewFixture struct {
	pair        tenantPair
	taskA       string
	draftA      string // 法人 A 的草稿
	draftB      string // 法人 B 的草稿
	draftLegacy string // legal_entity_id 为 NULL 的存量行
	userID      string // 真实用户，供 reviewed_by 外键
}

const draftReviewPayload = `{"contract_number":"DRAFTREV-CT-A","lessee_name":"甲","lessor_name":"乙","currency":"CNY","commencement_date":"2026-01-01T00:00:00Z","lease_start_date":"2026-01-01T00:00:00Z","lease_end_date":"2027-12-31T00:00:00Z"}`

func TestDraftReviewTenantIsolation(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "draftrev")
	fixture := seedDraftReviewRows(t, ctx, pool, pair)
	// Cleanup LIFO：先清本夹具的草稿/任务/用户，最后清 pair（否则删实体时
	// 草稿外键报非致命错）。
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })
	t.Cleanup(func() { cleanupDraftReviewRows(t, ctx, pool, fixture) })

	repo := NewContractRepository(pool)
	filterA := mustEntityFilter(t, pair.entityA)
	filterB := mustEntityFilter(t, pair.entityB)

	// 验收 1：legal_entity_id 为 NULL 的存量行对任何账号都不可见。
	for name, filter := range map[string]access.EntityFilter{"entity-a": filterA, "entity-b": filterB} {
		rows, err := repo.ListDraftsForReview(ctx, filter, "", 200)
		if err != nil {
			t.Fatalf("%s list: %v", name, err)
		}
		for _, row := range rows {
			if row.ID == fixture.draftLegacy {
				t.Fatalf("%s listed the NULL-entity legacy draft %s", name, fixture.draftLegacy)
			}
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_contract_drafts WHERE id=$1`, fixture.draftLegacy).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy draft vanished (seed bug): count=%d err=%v", count, err)
	}
	legacyRowStillThere := count == 1

	// 验收 2：global（无法人）filter 一行都拿不到——这个接缝没有「管理员全量」。
	globalRows, err := repo.ListDraftsForReview(ctx, access.GlobalEntityFilter(), "", 200)
	if err != nil || len(globalRows) != 0 {
		t.Fatalf("global filter listed %d drafts (err=%v); this seam must stay fail-closed", len(globalRows), err)
	}
	if _, err := repo.GetDraftForReview(ctx, access.GlobalEntityFilter(), fixture.draftA); err != pgx.ErrNoRows {
		t.Fatalf("global Get = %v; want ErrNoRows", err)
	}

	// 验收 3：法人 A 列得到自己的草稿，列不到法人 B 的。
	rowsA, err := repo.ListDraftsForReview(ctx, filterA, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	sawOwn := false
	for _, row := range rowsA {
		switch row.ID {
		case fixture.draftA:
			sawOwn = true
		case fixture.draftB:
			t.Fatal("entity A listed entity B's draft")
		case fixture.draftLegacy:
			t.Fatal("entity A listed the NULL-entity legacy draft")
		}
	}
	if !sawOwn {
		t.Fatal("entity A cannot see its own draft")
	}
	rowsB, err := repo.ListDraftsForReview(ctx, filterB, "", 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rowsB {
		if row.ID == fixture.draftA || row.ID == fixture.draftLegacy {
			t.Fatalf("entity B sees foreign/legacy draft %s", row.ID)
		}
	}

	// 验收 4（repository 层的形状）：异法人、不存在、NULL 行都是 pgx.ErrNoRows。
	for name, id := range map[string]string{"foreign": fixture.draftB, "missing": uuid.NewString(), "legacy": fixture.draftLegacy} {
		if _, err := repo.GetDraftForReview(ctx, filterA, id); err != pgx.ErrNoRows {
			t.Fatalf("%s Get = %v; want ErrNoRows", name, err)
		}
	}

	// 正面：自己的草稿可见，data_classification 带出（底线 2）。
	row, err := repo.GetDraftForReview(ctx, filterA, fixture.draftA)
	if err != nil {
		t.Fatalf("entity A cannot see own draft: %v", err)
	}
	if row.DataClassification != "production" {
		t.Fatalf("data_classification = %q; want production", row.DataClassification)
	}

	// 风险点 2：reviewed_by 是 users(id) 外键——真实用户可写；伪造 ID 报外键错。
	if !legacyRowStillThere {
		t.Fatal("invariant broken before reviewer update")
	}
	if err := repo.UpdateDraftReview(ctx, fixture.draftA, UpdateDraftReviewInput{
		Status: "prepared", ReviewerUserID: fixture.userID,
	}); err != nil {
		t.Fatalf("update with real reviewer id failed: %v", err)
	}
	fabricatedErr := repo.UpdateDraftReview(ctx, fixture.draftA, UpdateDraftReviewInput{
		Status: "prepared", ReviewerUserID: uuid.NewString(),
	})
	if fabricatedErr == nil || (!strings.Contains(fabricatedErr.Error(), "fk_") && !strings.Contains(fabricatedErr.Error(), "reviewed_by")) {
		t.Fatalf("fabricated reviewer id should violate the FK, got: %v", fabricatedErr)
	}

	// SaveDraftEdits 落 human_edits 子对象且不覆盖 AI 原值（差异留痕）。
	if err := repo.SaveDraftEdits(ctx, fixture.draftA,
		[]byte(`{"lessee_name":{"value":"人工终值","confirmed":true}}`)); err != nil {
		t.Fatalf("save edits: %v", err)
	}
	row, err = repo.GetDraftForReview(ctx, filterA, fixture.draftA)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		ContractNumber string         `json:"contract_number"`
		HumanEdits     map[string]any `json:"human_edits"`
	}
	if err := json.Unmarshal(row.ContractData, &stored); err != nil {
		t.Fatalf("decode stored contract_data: %v", err)
	}
	if stored.ContractNumber != "DRAFTREV-CT-A" {
		t.Fatalf("ai value overwritten: %s", stored.ContractNumber)
	}
	if stored.HumanEdits["lessee_name"] == nil {
		t.Fatalf("human layer missing after save: %v", stored.HumanEdits)
	}
}

// TestMigration060MatchesInitBaseline proves 增量迁移与空库基线一致：
// 01_init.sql 加载出的库已具备 060 声明的每一列与约束；在它之上重放 060 是
// 干净的幂等 no-op。
func TestMigration060MatchesInitBaseline(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()

	// 约束必须先由 01_init 自身建立——这正是「迁移漏合并进空库基线」的
	// 检查点（曾真实漂移过：约束只在 060 里，空库永远拿不到）。
	var checkBefore int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname='ai_contract_drafts_classification_check'`,
	).Scan(&checkBefore); err != nil || checkBefore != 1 {
		t.Fatalf("constraint missing from the init baseline (migration/init drift): count=%d err=%v", checkBefore, err)
	}

	wantColumns := map[string]string{
		"legal_entity_id":     "uuid",
		"data_classification": "character varying",
	}
	rows, err := pool.Query(ctx, `SELECT column_name, data_type FROM information_schema.columns
		WHERE table_name='ai_contract_drafts'`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			t.Fatal(err)
		}
		got[name] = dataType
	}
	rows.Close()
	for column, dataType := range wantColumns {
		if got[column] != dataType {
			t.Fatalf("column %s = %q; want %q (01_init.sql 与 migration 060 漂移)", column, got[column], dataType)
		}
	}

	migrationPath := filepath.Join("..", "..", "..", "db", "migrations", "060_draft_review_isolation.sql")
	raw, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatalf("replaying 060 onto the init baseline failed: %v", err)
	}

	var checkCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname='ai_contract_drafts_classification_check'
		  AND pg_get_constraintdef(oid) LIKE '%production%'`,
	).Scan(&checkCount); err != nil || checkCount != 1 {
		t.Fatalf("classification check constraint missing/mismatched: count=%d err=%v", checkCount, err)
	}
}

// ── seeding ────────────────────────────────────────────────────────────────

func seedDraftReviewRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pair tenantPair) draftReviewFixture {
	t.Helper()
	fixture := draftReviewFixture{pair: pair}

	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_tasks (task_type, status) VALUES ('contract_parse', 'completed') RETURNING id
	`).Scan(&fixture.taskA); err != nil {
		t.Fatalf("seed ai_task: %v", err)
	}
	insertDraft := func(entityID *string) string {
		t.Helper()
		var id string
		args := []any{fixture.taskA, draftReviewPayload, `{"currency":0.9}`}
		if entityID == nil {
			if err := pool.QueryRow(ctx, `
				INSERT INTO ai_contract_drafts (task_id, contract_data, confidence_scores, status, data_classification)
				VALUES ($1::uuid, $2::jsonb, $3::jsonb, 'pending', 'production') RETURNING id`, args...).Scan(&id); err != nil {
				t.Fatalf("seed legacy draft: %v", err)
			}
			return id
		}
		args = append(args, *entityID)
		if err := pool.QueryRow(ctx, `
			INSERT INTO ai_contract_drafts (task_id, contract_data, confidence_scores, status, legal_entity_id, data_classification)
			VALUES ($1::uuid, $2::jsonb, $3::jsonb, 'pending', $4::uuid, 'production') RETURNING id`, args...).Scan(&id); err != nil {
			t.Fatalf("seed draft: %v", err)
		}
		return id
	}
	fixture.draftA = insertDraft(&pair.entityA)
	fixture.draftB = insertDraft(&pair.entityB)
	fixture.draftLegacy = insertDraft(nil)

	suffix := uuid.NewString()[:8]
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"draftrev-"+suffix, "draftrev-"+suffix+"@test.local", pair.entityA,
	).Scan(&fixture.userID); err != nil {
		t.Fatalf("seed reviewer user: %v", err)
	}
	return fixture
}

func cleanupDraftReviewRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture draftReviewFixture) {
	t.Helper()
	statements := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM ai_contract_drafts WHERE id = ANY($1::uuid[])`,
			[]any{[]string{fixture.draftA, fixture.draftB, fixture.draftLegacy}}},
		{`DELETE FROM agent_draft_idempotency WHERE operation='draftreview.approve' AND idempotency_key = ANY($1::text[])`,
			[]any{[]string{"contract-draft:" + fixture.draftA, "contract-draft:" + fixture.draftB, "contract-draft:" + fixture.draftLegacy}}},
		{`DELETE FROM lease_contracts WHERE contract_number LIKE 'DRAFTREV-%'`, nil},
		{`DELETE FROM ai_tasks WHERE id=$1`, []any{fixture.taskA}},
		{`DELETE FROM users WHERE id=$1 AND username LIKE 'draftrev-%'`, []any{fixture.userID}},
	}
	for _, stmt := range statements {
		if _, err := pool.Exec(context.Background(), stmt.sql, stmt.args...); err != nil {
			t.Logf("cleanup statement failed (non-fatal): %v", err)
		}
	}
}
