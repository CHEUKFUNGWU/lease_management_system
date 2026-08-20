package adapter

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
	"github.com/lease-management-system/core-service/internal/repository"
)

// TestFactReaderPostgres locks the S2-3 production fact path: store-day
// rows in the real fact table fold through QueryFacts into the
// entity-month OperatingFacts the engine consumes, with the coverage
// rule and the currency discipline intact.
func TestFactReaderPostgres(t *testing.T) {
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
		entity, "FAC-E-"+suffix, "Facts "+suffix)
	storeA := uuid.NewString()
	storeB := uuid.NewString()
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand, is_active) VALUES
		($1,$2,'Facts A',$3,'east','b1',true), ($4,$5,'Facts B',$3,'east','b1',true)`,
		storeA, "FAC-S1-"+suffix, entity, storeB, "FAC-S2-"+suffix)
	exec(`INSERT INTO retail_store_day_facts
		(store_id, business_date, currency, revenue, labor_cost, source_system, version, data_classification)
		VALUES
		($1,'2026-07-01','CNY',100,30,'pos-a',2,'production'),
		($1,'2026-07-02','CNY',50,20,'pos-a',2,'production'),
		($2,'2026-07-01','CNY',200,60,'pos-a',2,'production')`, storeA, storeB)

	reader := NewFactReader(repository.NewRetailKPIRepository(pool))
	facts, err := reader.Operating(ctx, entity, "2026-07")
	if err != nil {
		t.Fatalf("Operating: %v", err)
	}
	if facts.Revenue == nil || *facts.Revenue != 350 || facts.LaborCost == nil || *facts.LaborCost != 110 {
		t.Fatalf("folded facts wrong: %+v", facts)
	}
	if !facts.DecisionReady {
		t.Fatalf("full coverage must be decision-ready: %q", facts.DecisionReadyReason)
	}

	// 缺一店 → 覆盖不足。
	exec(`DELETE FROM retail_store_day_facts WHERE store_id=$1`, storeB)
	partial, err := reader.Operating(ctx, entity, "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if partial.DecisionReady {
		t.Fatalf("missing store must degrade coverage: %+v", partial)
	}
}

// TestAssumptionDraftsIdempotentAndAtomicPostgres locks P0-9 (底线 4): the
// assumption suggestion batch is one transaction keyed by idempotency_key —
// a replayed key returns the SAME batch ids with no second row, and a
// mid-batch constraint failure rolls the whole batch back (no partial batch).
func TestAssumptionDraftsIdempotentAndAtomicPostgres(t *testing.T) {
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
		entity, "IDM-E-"+suffix, "Idempotency "+suffix)
	evd := func(call string) []suggestion.EvidenceRef {
		return []suggestion.EvidenceRef{{ToolCallID: call, Scope: entity, Period: "2026-07"}}
	}
	countForKey := func(key string) int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM fpna_assumption_versions WHERE legal_entity_id=$1 AND idempotency_key=$2`, entity, key).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	total := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM fpna_assumption_versions WHERE legal_entity_id=$1`, entity).Scan(&n); err != nil {
			t.Fatalf("total: %v", err)
		}
		return n
	}

	writer := NewDraftWriter(repository.NewFinModelRepository(pool))
	rows := []suggestion.SuggestionDraft{
		{AssumptionKey: "sssg", Category: "revenue", Value: []byte("0.03"), Unit: "rate", SourceTag: "ai_suggestion", Confidence: 0.7, Basis: evd("t1")},
		{AssumptionKey: "labor_cost_growth", Category: "expense", Value: []byte("0.01"), SourceTag: "ai_suggestion", Confidence: 0.6, Basis: evd("t1")},
	}
	first, err := writer.SaveDrafts(ctx, entity, rows, "idem-k-1")
	if err != nil || len(first) != 2 {
		t.Fatalf("first save = %v / %v", first, err)
	}
	if countForKey("idem-k-1") != 2 {
		t.Fatal("first batch must write exactly two draft rows")
	}
	// 重放：返回既有批次，不再落第二条。
	replay, err := writer.SaveDrafts(ctx, entity, rows, "idem-k-1")
	if err != nil || len(replay) != 2 {
		t.Fatalf("replay = %v / %v", replay, err)
	}
	// 同批 id（顺序无关：首次返按插入序、重放返按 (created_at,id) 序）。
	sort.Strings(first)
	sort.Strings(replay)
	if replay[0] != first[0] || replay[1] != first[1] {
		t.Fatalf("replay must return the same batch ids: %v vs %v", replay, first)
	}
	if countForKey("idem-k-1") != 2 {
		t.Fatal("replay must not create a second batch row")
	}

	// 批量中途失败（同批两条同 assumption_key 撞 version=1 唯一约束）→ 整批回滚。
	before := total()
	failing := []suggestion.SuggestionDraft{
		{AssumptionKey: "dso", Category: "nwc", Value: []byte("10"), SourceTag: "ai_suggestion", Confidence: 0.5, Basis: evd("t2")},
		{AssumptionKey: "dso", Category: "nwc", Value: []byte("20"), SourceTag: "ai_suggestion", Confidence: 0.5, Basis: evd("t2")},
	}
	if _, err := writer.SaveDrafts(ctx, entity, failing, "idem-k-2"); err == nil {
		t.Fatal("a duplicate assumption_key in one batch must fail")
	}
	if after := total(); after != before {
		t.Fatalf("batch must be atomic (no partial commit): %d before, %d after", before, after)
	}
}

// TestDraftWriterPostgres locks the S4-2 contract against real rows: AI
// suggestions persist as draft rows with ai_suggestion source, evidence
// and confidence — and the approved-only reader never sees them.
func TestDraftWriterPostgres(t *testing.T) {
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
		entity, "DW-E-"+suffix, "Drafts "+suffix)

	writer := NewDraftWriter(repository.NewFinModelRepository(pool))
	ids, err := writer.SaveDrafts(ctx, entity, []suggestion.SuggestionDraft{{
		AssumptionKey: "sssg", Category: "revenue", Value: []byte("0.03"),
		Unit: "rate", SourceTag: "ai_suggestion", Confidence: 0.8,
		Basis: []suggestion.EvidenceRef{{ToolCallID: "tcall-1", Scope: entity, Period: "2026-07"}},
	}}, "dw-k-1")
	if err != nil || len(ids) != 1 {
		t.Fatalf("SaveDrafts = %v/%v", ids, err)
	}

	var status, source string
	var evidence []byte
	var confidence *float64
	err = pool.QueryRow(ctx, `SELECT status, source, evidence, confidence FROM fpna_assumption_versions WHERE id=$1`, ids[0]).
		Scan(&status, &source, &evidence, &confidence)
	if err != nil {
		t.Fatal(err)
	}
	if status != "draft" || source != "ai_suggestion" || confidence == nil || *confidence != 0.8 || len(evidence) == 0 {
		t.Fatalf("draft row contract broken: status=%s source=%s conf=%v evidence=%s", status, source, confidence, evidence)
	}

	// approved-only 读取绝不可见 draft。
	approved, err := repository.NewFinModelRepository(pool).LatestApprovedAssumptions(ctx, entity, []string{"sssg"}, "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := approved["sssg"]; ok {
		t.Fatal("a draft leaked into the approved reader")
	}
}
