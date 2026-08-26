package repository_test

// 电商独立站模式带库集成四件套（spec §6 测试缝 4；skip 不构成证据）：
//
//	R-E1-3 幂等：同一批次重放两次行数不变
//	R-E2-2 重述：v2 覆盖读路径、v1 仍在库、原期间 restated
//	底线 1 跨法人隔离：站点属法人、行级过滤
//	R-E4-3 口径门禁：未对平期间不得进入 Approved（写侧拒绝 + 差异入队由 handler 完成）
//
// 跑法：make test-integration ARGS="./internal/repository/ -run TestEcom"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/ecomintake"
	"github.com/lease-management-system/core-service/internal/services/settlement"
)

func ecomPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func ecomSeedEntities(t *testing.T, pool *pgxpool.Pool) (leA, leB, siteA, siteB string) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano() % 1e9
	// 批次/请求清理必须最先注册（LIFO 最后执行）——先删 site（级联 facts）释放
	// operating_fact_batches 的 FK，才能删 batch；顺序反了 batch 删除会被 FK 挡住，
	// 残留 completed batch 让下一轮 BeginBatch 误判 replay（间歇性 0 行的根因）。
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM operating_fact_batches WHERE idempotency_key LIKE 'ecom:%'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ecommerce_ingest_requests WHERE idempotency_key LIKE 'ecom-%'`)
	})
	leA = uuid.NewString()
	leB = uuid.NewString()
	leNameA := "ecom-le-a-" + itoa9(suffix)
	leNameB := "ecom-le-b-" + itoa9(suffix)
	for _, pair := range [][2]string{{leA, leNameA}, {leB, leNameB}} {
		if _, err := pool.Exec(ctx, `INSERT INTO legal_entities (id, code, name, currency) VALUES ($1, $2, $2, 'USD')`, pair[0], pair[1]); err != nil {
			t.Fatalf("insert legal entity: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id = $1`, pair[0]) })
	}
	siteA = uuid.NewString()
	siteB = uuid.NewString()
	for i, le := range []string{leA, leB} {
		id := []string{siteA, siteB}[i]
		if _, err := pool.Exec(ctx, `
			INSERT INTO storefronts (id, legal_entity_id, code, name, currency, platform, status)
			VALUES ($1, $2, $3, $4, 'USD', 'shopify', 'active')`, id, le, "st-"+"ecom-"+[]string{"a", "b"}[i]+"-"+itoa9(suffix), id); err != nil {
			t.Fatalf("insert storefront: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM storefronts WHERE id = $1`, id) })
	}
	return leA, leB, siteA, siteB
}

func itoa9(n int64) string {
	return fmt.Sprintf("%09d", n)
}

func ecomEnvelope() ecomintake.EnvelopeSpec {
	return ecomintake.EnvelopeSpec{
		SourceSystem:       "shopify",
		AsOfAt:             time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		DataClassification: "production",
	}
}

func TestEcomIngestIdempotentReplay(t *testing.T) {
	pool := ecomPool(t)
	leA, _, siteA, _ := ecomSeedEntities(t, pool)
	ctx := context.Background()
	repo := repository.NewEcommerceRepository(pool)
	sink := repo

	csv := "business_date,channel,sku,currency,gmv_amount\n2026-08-01,direct,,USD,1000\n2026-08-02,direct,,USD,2000\n"
	spec := ecomintake.ImportSpec{
		LegalEntityID: leA, StorefrontID: siteA, UserID: nil,
		Filename: "orders.csv", Data: []byte(csv),
		Source: ecomintake.SourceShopify, TemplateVersion: "1",
		IdempotencyKey: "ecom-it-replay-1", Envelope: ecomEnvelope(),
	}
	for i := 0; i < 2; i++ {
		result, err := ecomintake.IngestBatch(ctx, spec, sink)
		if err != nil {
			t.Fatalf("ingest #%d: %v", i+1, err)
		}
		t.Logf("ingest #%d report=%+v batch=%+v", i+1, result.Report, result.Batch)
		if i == 1 && !result.IdempotentReplay {
			t.Fatal("第二次重放必须短路为 IdempotentReplay")
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM storefront_day_facts WHERE storefront_id::text=$1`, siteA).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("幂等重放后行数必须不变：期望 2 实际 %d", count)
	}
}

func TestEcomRestatementKeepsV1AndReadsV2(t *testing.T) {
	pool := ecomPool(t)
	leA, _, siteA, _ := ecomSeedEntities(t, pool)
	ctx := context.Background()
	repo := repository.NewEcommerceRepository(pool)

	day := "2026-08-10"

	// v1：无退款
	first := ecomintake.ImportSpec{
		LegalEntityID: leA, StorefrontID: siteA, Filename: "v1.csv",
		Data: []byte("business_date,channel,sku,currency,gmv_amount,refund_amount\n"), Source: ecomintake.SourceShopify,
		TemplateVersion: "1", IdempotencyKey: "ecom-it-restate-v1", Envelope: ecomEnvelope(),
	}
	firstRows := "business_date,channel,sku,currency,gmv_amount,refund_amount\n" + day + ",direct,,USD,1000,0\n"
	first.Data = []byte(firstRows)
	if _, err := ecomintake.IngestBatch(ctx, first, repo); err != nil {
		t.Fatalf("v1 ingest: %v", err)
	}

	// v2：退款 100 到达（重述）
	second := first
	second.IdempotencyKey = "ecom-it-restate-v2"
	second.Data = []byte("business_date,channel,sku,currency,gmv_amount,refund_amount\n" + day + ",direct,,USD,1000,100\n")
	if _, err := ecomintake.IngestBatch(ctx, second, repo); err != nil {
		t.Fatalf("v2 ingest: %v", err)
	}

	// v1 仍在库
	var versions []int
	if rows, err := pool.Query(ctx, `SELECT fact_version FROM storefront_day_facts
		WHERE storefront_id::text=$1 AND business_date=$2::date ORDER BY fact_version`, siteA, day); err != nil {
		t.Fatalf("versions query: %v", err)
	} else {
		for rows.Next() {
			var v int
			if err := rows.Scan(&v); err != nil {
				t.Fatalf("scan: %v", err)
			}
			versions = append(versions, v)
		}
		rows.Close()
	}
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 2 {
		t.Fatalf("v1 与 v2 必须都留在库：%v", versions)
	}
	var restated bool
	if err := pool.QueryRow(ctx, `SELECT restated FROM storefront_day_facts
		WHERE storefront_id::text=$1 AND business_date=$2::date AND fact_version=2`, siteA, day).Scan(&restated); err != nil {
		t.Fatalf("restated flag: %v", err)
	}
	if !restated {
		t.Fatal("v2 必须带 restated 修订标记（R-E2-2）")
	}

	// 读路径只返回最高版本且退款已生效
	entity, _ := access.EntityFilterFor(leA)
	facts, err := repo.StorefrontDays(ctx, ecomfact.StorefrontFilter{Entity: entity, StorefrontIDs: []string{siteA}},
		ecomfact.Window{From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("读路径必须只返回最高版本一行：%d", len(facts))
	}
	if facts[0].SourceEnvelope.FactVersion != 2 || facts[0].RefundAmount == nil || *facts[0].RefundAmount != 100 {
		t.Fatalf("读路径必须返回 v2（退款 100）：%+v", facts[0])
	}
	if !facts[0].SourceEnvelope.Restated {
		t.Fatal("读出的最高版本必须带 restated 标记")
	}
}

func TestEcomCrossLegalEntityIsolation(t *testing.T) {
	pool := ecomPool(t)
	leA, leB, siteA, siteB := ecomSeedEntities(t, pool)
	ctx := context.Background()
	repo := repository.NewEcommerceRepository(pool)

	// A 站点导入
	specA := ecomintake.ImportSpec{
		LegalEntityID: leA, StorefrontID: siteA, Filename: "a.csv",
		Data: []byte("business_date,channel,sku,currency,gmv_amount\n2026-08-11,direct,,USD,1000\n"),
		Source: ecomintake.SourceShopify, TemplateVersion: "1",
		IdempotencyKey: "ecom-it-iso-a", Envelope: ecomEnvelope(),
	}
	if resA, err := ecomintake.IngestBatch(ctx, specA, repo); err != nil {
		t.Fatalf("ingest A: %v", err)
	} else {
		t.Logf("ingest A report=%+v", resA.Report)
	}
	// B 站点导入（法人归属也必须切到 B——落库以站点归属为准的防线由产品层兜底）
	specB := specA
	specB.LegalEntityID = leB
	specB.StorefrontID = siteB
	specB.IdempotencyKey = "ecom-it-iso-b"
	if resB, err := ecomintake.IngestBatch(ctx, specB, repo); err != nil {
		t.Fatalf("ingest B: %v", err)
	} else {
		t.Logf("ingest B report=%+v", resB.Report)
	}

	win := ecomfact.Window{From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}

	// 法人 A 视角读不到 B 的站点日事实
	entityA, _ := access.EntityFilterFor(leA)
	all := ecomfact.StorefrontFilter{Entity: entityA}
	factsA, err := repo.StorefrontDays(ctx, all, win)
	if err != nil {
		t.Fatalf("read A: %v", err)
	}
	for _, f := range factsA {
		if f.StorefrontID == siteB {
			t.Fatalf("法人 A 读到了法人 B 的事实（底线 1 破坏）：%+v", f)
		}
	}
	// 法人 A 拿 B 的站点 ID 找站点 → not found（无存在性泄漏）
	if _, err := repo.GetStorefront(ctx, entityA, siteB); err != repository.ErrEcomNotFound {
		t.Fatalf("越权取站必须 not found：%v", err)
	}
	// 法人 B 视角能看到自己的
	entityB, _ := access.EntityFilterFor(leB)
	var rawCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM storefront_day_facts WHERE legal_entity_id::text=$1`, leB).Scan(&rawCount); err != nil {
		t.Fatalf("raw count B: %v", err)
	}
	var allB, allA int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM storefront_day_facts`).Scan(&allB)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM operating_fact_batches WHERE idempotency_key LIKE 'ecom:%'`).Scan(&allA)
	var dbName, dbUser string
	_ = pool.QueryRow(ctx, `SELECT current_database(), current_user`).Scan(&dbName, &dbUser)
	var leOfFacts string
	_ = pool.QueryRow(ctx, `SELECT legal_entity_id::text FROM storefront_day_facts LIMIT 1`).Scan(&leOfFacts)
	t.Logf("raw B facts=%d siteB=%s all-facts=%d ecom-batches=%d db=%s user=%s leB=%s leOfFacts=%s", rawCount, siteB, allB, allA, dbName, dbUser, leB, leOfFacts)
	var manualB int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM storefront_day_facts
		WHERE (cardinality($1::text[]) = 0 OR storefront_id::text = ANY($1::text[]))
		  AND legal_entity_id::text = $2
		  AND business_date BETWEEN $3::date AND $4::date`,
		[]string{}, leB, "2026-08-01", "2026-08-31").Scan(&manualB)
	t.Logf("manual SQL with leB count=%d", manualB)
	var distinctB int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT ON (storefront_id::text, business_date, channel, sku, source_system)
				storefront_id::text, fact_version
			FROM storefront_day_facts
			WHERE (cardinality($1::text[]) = 0 OR storefront_id::text = ANY($1::text[]))
			  AND legal_entity_id::text::text = $2
			  AND business_date BETWEEN $3::date AND $4::date
			ORDER BY storefront_id::text, business_date, channel, sku, source_system, fact_version DESC
		) t`, []string{}, leB, "2026-08-01", "2026-08-31").Scan(&distinctB)
	t.Logf("distinct-on SQL with leB count=%d", distinctB)
	factsB, err := repo.StorefrontDays(ctx, ecomfact.StorefrontFilter{Entity: entityB}, win)
	if err != nil {
		t.Fatalf("read B: %v", err)
	}
	if len(factsB) != 1 || factsB[0].StorefrontID != siteB {
		t.Fatalf("法人 B 应只看到自己的 1 行：%+v", factsB)
	}
}

func TestEcomSettlementGateBlocksApproval(t *testing.T) {
	pool := ecomPool(t)
	leA, _, siteA, _ := ecomSeedEntities(t, pool)
	ctx := context.Background()
	repo := repository.NewEcommerceRepository(pool)

	// 写入 1 条 payout 但没有任何银行行 → 在途差异
	day := "2026-08-12"
	batch := ecomintake.ImportSpec{
		LegalEntityID: leA, StorefrontID: siteA, Filename: "settle.csv",
		Data: []byte("provider,payout_id,payout_date,currency,gross_amount,fee_amount,net_amount\n"),
		Source: ecomintake.SourceSettlement, TemplateVersion: "1",
		IdempotencyKey: "ecom-it-gate-1", Envelope: ecomEnvelope(),
	}
	batch.Data = []byte("provider,payout_id,payout_date,currency,gross_amount,fee_amount,net_amount\nshopify_payments,PO-G1," + day + ",USD,1000,30,970\n")
	if res, err := ecomintake.IngestBatch(ctx, batch, repo); err != nil {
		t.Fatalf("ingest payout: %v", err)
	} else {
		t.Logf("ingest payout report=%+v", res.Report)
	}

	entity, _ := access.EntityFilterFor(leA)
	from, _ := time.Parse("2006-01", "2026-08")
	to := from.AddDate(0, 1, 0).AddDate(0, 0, -1)
	payoutRows, err := repo.ListPayoutLines(ctx, entity, siteA, from, to)
	if err != nil {
		t.Fatalf("list payouts: %v", err)
	}
	bankRows, err := repo.ListBankLines(ctx, entity, siteA, from, to)
	if err != nil {
		t.Fatalf("list banks: %v", err)
	}
	if len(payoutRows) != 1 || len(bankRows) != 0 {
		t.Fatalf("fixture 出错：payouts=%d banks=%d", len(payoutRows), len(bankRows))
	}
	payouts := []settlement.PayoutLine{{Provider: payoutRows[0].Provider, PayoutID: payoutRows[0].PayoutID,
		PayoutDate: payoutRows[0].PayoutDate, Currency: payoutRows[0].Currency, GrossAmount: payoutRows[0].GrossAmount,
		FeeAmount: payoutRows[0].FeeAmount, NetAmount: payoutRows[0].NetAmount}}
	results := settlement.Match(payouts, nil, nil, settlement.MatchPolicy{})
	verdict := settlement.ApprovalGate("2026-08", results)
	if verdict.Verdict != "deny" {
		t.Fatalf("未对平期间门禁必须 deny：%+v", verdict)
	}

	// 门禁 deny 的 run 在签认管线里无法 approve：任意状态迁移到 approved 都被拒
	run := &repository.SettlementRun{
		LegalEntityID: leA, StorefrontID: siteA, Period: "2026-08", Currency: "USD",
		GateVerdict: strPtrVal2("deny"), Results: mustJSON2(results), Differences: mustJSON2(results),
		IdempotencyKey: strPtrVal2("ecom-it-gate-run"),
	}
	created, _, err := repo.CreateSettlementRun(ctx, run)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	// draft → prepared → pending
	if _, err := repo.TransitionSettlementRun(ctx, entity, created.ID, "prepare", "", ""); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := repo.TransitionSettlementRun(ctx, entity, created.ID, "submit", "", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err = repo.TransitionSettlementRun(ctx, entity, created.ID, "approve", "", "")
	if err == nil {
		t.Fatal("gate=deny 的 run 不得进入 approved（R-E4-3）")
	}
}

func strPtrVal2(s string) *string { return &s }

func mustJSON2(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
