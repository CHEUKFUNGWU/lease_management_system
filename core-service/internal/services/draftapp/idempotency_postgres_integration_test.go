package draftapp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// TestPostgresIdempotencyLockAndReplay pins the advisory-lock + replay seam
// against a real database.
//
// 回归背景：锁键曾用 operation+"\x00"+key 拼接，Postgres 文本参数拒绝 0x00
// （SQLSTATE 22021），导致每次 LookupIdempotency 都直接报错——这条路径没有
// 集成覆盖，bug 一直潜伏。本用例先证它 RUN 过且绿：同一幂等键提交两次，
// 第一次 created、第二次 replayed，lease_contracts 只有一条。
func TestPostgresIdempotencyLockAndReplay(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	ctx := context.Background()

	suffix := t.Name()
	var entityID, storeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"DRAFTAPP-LE-"+suffix[:8], "draftapp idempotency tenant").Scan(&entityID); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_contracts WHERE legal_entity_id=$1`, entityID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_draft_idempotency WHERE idempotency_key=$1`, "draftapp-it:"+suffix)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id=$1`, storeID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id=$1`, entityID)
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, 'brand', 'region', true) RETURNING id`,
		"DRAFTAPP-ST-"+suffix[:8], "draftapp store", entityID).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	var reviewerID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"draftapp-it-"+suffix[:8], "draftapp-it-"+suffix[:8]+"@test.local", entityID).Scan(&reviewerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, reviewerID) })

	contractRepo := repository.NewContractRepository(pool)
	uow := NewPostgresUnitOfWork(pool, contractRepo, repository.NewPaymentScheduleRepository(pool))
	service := NewService(uow, PostgresContractReader{repo: contractRepo})

	// Create 的 scope 校验走 ctx scope：带法人 scope 模拟真实调用方。
	scopedCtx := func() context.Context {
		return access.WithScope(ctx, access.Scope{LegalEntityID: entityID, StoreIDs: []string{storeID}})
	}
	var landlordID string
	if err := pool.QueryRow(ctx, `INSERT INTO landlords (code, name) VALUES ($1, 'draftapp 出租方') RETURNING id`,
		"DRAFTAPP-LL-"+suffix[:8]).Scan(&landlordID); err != nil {
		t.Fatalf("seed landlord: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM landlords WHERE id=$1`, landlordID) })

	contract := &repository.Contract{
		ContractNumber: suffix, ContractName: "idempotency probe",
		LegalEntityID: &entityID, StoreID: &storeID, LandlordID: &landlordID,
		LesseeName: "甲", LessorName: "乙", Currency: "CNY",
		AssetType: "store", LeaseScope: "in_scope",
		CommencementDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseStartDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseEndDate:     time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	first := service.CreateContractDraft(scopedCtx(), ContractDraftCommand{
		IdempotencyKey: "draftapp-it:" + suffix, ActorID: reviewerID, Contract: contract,
	})
	if first.Status != ItemCreated {
		t.Fatalf("first submit = %+v; want created", first)
	}
	second := service.CreateContractDraft(scopedCtx(), ContractDraftCommand{
		IdempotencyKey: "draftapp-it:" + suffix, ActorID: reviewerID, Contract: contract,
	})
	if second.Status != ItemReplayed {
		t.Fatalf("second submit = %+v; want replayed", second)
	}

	var formalCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lease_contracts WHERE contract_number=$1`, suffix).Scan(&formalCount); err != nil || formalCount != 1 {
		t.Fatalf("formal records = %d (err %v); want exactly 1", formalCount, err)
	}
}
