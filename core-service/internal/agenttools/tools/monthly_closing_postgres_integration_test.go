package tools

// B-2 月结只读工具带库集成测试（底线 1）：跨法人隔离、锁账状态回显、
// Working 口径信封、零写入。TEST_DATABASE_URL 指向已加载 db/init/01_init.sql
// 的 Postgres；未设则 skip。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestMonthlyClosingToolsPostgresIsolationNoWrites(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	// t.Cleanup 后进先出：先注册关池，让它晚于数据清理执行。
	t.Cleanup(func() { pool.Close() })

	runToken := uuid.NewString()[:12]
	runPattern := "AGENT-MC-" + runToken + "%"
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v (%s)", err, sql)
		}
	}

	var entityA, entityB, storeA, landlordA, contractA1, contractA2 string

	cleanup := func() {
		cctx := context.Background()
		statements := []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM journal_entries je USING lease_contracts lc WHERE je.contract_id=lc.id AND lc.contract_number LIKE $1`, []any{runPattern}},
			{`DELETE FROM period_locks WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runPattern}},
			{`DELETE FROM monthly_closing_batches WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runPattern}},
			{`DELETE FROM measurement_results m USING lease_contracts lc WHERE m.contract_id=lc.id AND lc.contract_number LIKE $1`, []any{runPattern}},
			{`DELETE FROM lease_contracts WHERE contract_number LIKE $1`, []any{runPattern}},
			{`DELETE FROM stores WHERE code = $1`, []any{"AGENTS-" + runToken}},
			{`DELETE FROM landlords WHERE code = $1`, []any{"AGENTL-" + runToken}},
			{`DELETE FROM legal_entities WHERE code LIKE $1`, []any{runPattern}},
		}
		for _, statement := range statements {
			if _, err := pool.Exec(cctx, statement.sql, statement.args...); err != nil {
				t.Errorf("cleanup: %v (%s)", err, statement.sql)
			}
		}
	}
	t.Cleanup(cleanup)

	// 业务表零写入断言：只统计本测试种下的实体，避免共享库上的并发噪声。
	scopedCounts := map[string]string{
		"journal_entries":         `SELECT COUNT(*) FROM journal_entries je JOIN lease_contracts lc ON lc.id=je.contract_id WHERE lc.contract_number LIKE $1`,
		"monthly_closing_batches": `SELECT COUNT(*) FROM monthly_closing_batches WHERE batch_number LIKE $1`,
		"period_locks":            `SELECT COUNT(*) FROM period_locks pl JOIN legal_entities le ON le.id=pl.legal_entity_id WHERE le.code LIKE $1`,
	}
	tableCounts := func() map[string]int64 {
		counts := map[string]int64{}
		for table, query := range scopedCounts {
			var count int64
			if err := pool.QueryRow(ctx, query, runPattern).Scan(&count); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			counts[table] = count
		}
		return counts
	}
	before := tableCounts()

	// 种子：法人 A（合同×2、分录 2026-06 三条 / 2026-05 两条、2026-06 锁账、一个批次）+ 法人 B（空）。
	entityA, entityB = uuid.NewString(), uuid.NewString()
	storeA, landlordA = uuid.NewString(), uuid.NewString()
	contractA1, contractA2 = uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES
		($1,$2,$3,'CN','CNY'), ($4,$5,$6,'CN','CNY')`,
		entityA, "AGENT-MC-"+runToken+"-a", "MC A "+runToken,
		entityB, "AGENT-MC-"+runToken+"-b", "MC B "+runToken)
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES ($1,$2,'MC store',$3,'east','b1')`,
		storeA, "AGENTS-"+runToken, entityA)
	exec(`INSERT INTO landlords (id, code, name) VALUES ($1,$2,'MC landlord')`,
		landlordA, "AGENTL-"+runToken)
	exec(`INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, currency, commencement_date, lease_start_date, lease_end_date, status, lease_scope) VALUES
		($1,$2,'MC c1',$3,$4,$5,'property','CNY','2024-01-01','2024-01-01','2030-12-31','approved','in_scope'),
		($6,$7,'MC c2',$3,$4,$5,'property','CNY','2024-01-01','2024-01-01','2030-12-31','approved','in_scope')`,
		contractA1, "AGENT-MC-"+runToken+"-C1", entityA, storeA, landlordA,
		contractA2, "AGENT-MC-"+runToken+"-C2")
	exec(`INSERT INTO period_locks (accounting_period, legal_entity_id, is_locked) VALUES ('2026-06',$1,true)`, entityA)
	var batchA string
	if err := pool.QueryRow(ctx, `INSERT INTO monthly_closing_batches (batch_number, accounting_period, legal_entity_id, status, total_contracts, processed_contracts, total_entries) VALUES ($1,'2026-06',$2,'completed',2,2,3) RETURNING id`,
		"AGENT-MC-"+runToken+"-B", entityA).Scan(&batchA); err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	for _, e := range []struct {
		contract, period, status string
		day                      int
		withBatch                bool
	}{
		{contractA1, "2026-06", "draft", 10, true},
		{contractA2, "2026-06", "approved", 11, true},
		{contractA1, "2026-06", "posted", 28, true},
		{contractA1, "2026-05", "draft", 10, false},
		{contractA2, "2026-05", "draft", 11, false},
	} {
		// 2026-06 的三条挂在批次下（GetBatches 的访问校验经 batch_id 关联）；
		// 2026-05 两条不入批（期间级分录可独立于批次存在）。
		if e.withBatch {
			exec(`INSERT INTO journal_entries (contract_id, accounting_period, entry_date, entry_type, debit_account, credit_account, amount, currency, posting_status, batch_id) VALUES ($1,$2,$3::date,'interest','dr','cr',100,'CNY',$4,$5)`,
				e.contract, e.period, fmt.Sprintf("%s-%02d", e.period, e.day), e.status, batchA)
		} else {
			exec(`INSERT INTO journal_entries (contract_id, accounting_period, entry_date, entry_type, debit_account, credit_account, amount, currency, posting_status) VALUES ($1,$2,$3::date,'interest','dr','cr',100,'CNY',$4)`,
				e.contract, e.period, fmt.Sprintf("%s-%02d", e.period, e.day), e.status)
		}
	}

	before = tableCounts() // seed 后重定基线：断言的是「工具调用不改变行数」

	repo := repository.NewMonthlyClosingRepository(pool)
	batchesDef := NewMonthlyClosingBatchesDefinition(repo)
	entriesDef := NewMonthlyClosingEntriesPreviewDefinition(repo)
	periodsDef := NewMonthlyClosingPeriodsDefinition(repo)
	lockDef := NewMonthlyClosingLockStatusDefinition(repo)

	newCtx := func(legalEntityID string) context.Context {
		scope := access.Scope{LegalEntityID: legalEntityID}
		base := access.WithScope(context.Background(), scope)
		return agenttools.WithExecutionContext(base, agenttools.ExecutionContext{
			Principal: agenttools.Principal{UserID: "agent-mc", Permissions: []string{"monthly_closing:read"}, Scope: scope},
			RunID:     "agent-mc",
		})
	}
	ctxA, ctxB := newCtx(entityA), newCtx(entityB)
	invoke := func(callCtx context.Context, def agenttools.ToolDefinition, args string) agenttools.ToolResult {
		t.Helper()
		result, err := def.Handler(callCtx, agenttools.ToolCall{
			CallID: "c", RunID: "agent-mc", ToolName: def.Descriptor.Name, ToolVersion: "v1",
			Arguments: json.RawMessage(args),
		})
		if err != nil {
			t.Fatalf("%s handler error: %v", def.Descriptor.Name, err)
		}
		return result
	}

	// 1. 法人 A：分录预览 → completed，Working 信封 + 锁账状态。
	result := invoke(ctxA, entriesDef, `{"period":"2026-06"}`)
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("entries preview failed: %+v", result.Error)
	}
	preview := result.Data.(MonthlyClosingEntriesPreviewToolData)
	if preview.ReportBasis != "working" || preview.IsOfficialVersion {
		t.Fatalf("envelope must declare working/non-official: %+v", preview)
	}
	if !preview.PeriodLocked {
		t.Fatal("2026-06 must surface as locked for entity A")
	}
	if preview.ApprovalStatus.Total != 3 || preview.ApprovalStatus.DraftCount != 1 || preview.ApprovalStatus.ApprovedCount != 1 || preview.ApprovalStatus.PostedCount != 1 {
		t.Fatalf("approval summary wrong: %+v", preview.ApprovalStatus)
	}
	if len(preview.Items) != 3 {
		t.Fatalf("items wrong: %d", len(preview.Items))
	}
	entryIDsA := map[string]bool{}
	for _, item := range preview.Items {
		entryIDsA[item.ID] = true
		if item.PostingStatus == "" {
			t.Fatal("each item must carry posting_status")
		}
	}

	// 2. status 过滤：posted 只剩一条。
	result = invoke(ctxA, entriesDef, `{"period":"2026-06","status":"posted"}`)
	preview = result.Data.(MonthlyClosingEntriesPreviewToolData)
	if preview.ApprovalStatus.Total != 1 || len(preview.Items) != 1 || preview.Items[0].PostingStatus != "posted" {
		t.Fatalf("status filter wrong: %+v", preview)
	}

	// 3. 跑批状态 + 单期间锁账状态（法人 A）。
	result = invoke(ctxA, batchesDef, `{"period":"2026-06"}`)
	batches := result.Data.(MonthlyClosingBatchesToolData)
	if batches.Total != 1 || batches.PeriodLocked == nil || !*batches.PeriodLocked {
		t.Fatalf("batches wrong: %+v", batches)
	}
	result = invoke(ctxA, lockDef, `{"period":"2026-06"}`)
	if locked := result.Data.(MonthlyClosingLockStatusToolData); !locked.IsLocked {
		t.Fatalf("lock status wrong: %+v", locked)
	}
	result = invoke(ctxA, lockDef, `{"period":"2026-05"}`)
	if locked := result.Data.(MonthlyClosingLockStatusToolData); locked.IsLocked {
		t.Fatalf("2026-05 must be unlocked for A: %+v", locked)
	}

	// 4. 有分录期间列表：两期都出现且带 is_locked。
	result = invoke(ctxA, periodsDef, `{}`)
	periods := result.Data.(MonthlyClosingPeriodsToolData)
	if periods.Total < 2 {
		t.Fatalf("periods wrong: %+v", periods)
	}
	lockedByPeriod := map[string]bool{}
	for _, p := range periods.Periods {
		lockedByPeriod[p.AccountingPeriod] = p.IsLocked
	}
	if !lockedByPeriod["2026-06"] || lockedByPeriod["2026-05"] {
		t.Fatalf("per-period lock flags wrong: %+v", lockedByPeriod)
	}

	// 5. 跨法人隔离（底线 1）：法人 B 读同名期间 → 零条 A 的分录、零批次、
	// A 的实体级锁不可见；异法人合同 ID 过滤得空集。
	foreign := invoke(ctxB, entriesDef, `{"period":"2026-06"}`)
	foreignPreview := foreign.Data.(MonthlyClosingEntriesPreviewToolData)
	if foreignPreview.ApprovalStatus.Total != 0 || len(foreignPreview.Items) != 0 {
		t.Fatalf("entity B saw entity A entries: %+v", foreignPreview)
	}
	for _, item := range foreignPreview.Items {
		if entryIDsA[item.ID] {
			t.Fatalf("entity A entry %s leaked to entity B", item.ID)
		}
	}
	foreignBatches := invoke(ctxB, batchesDef, `{"period":"2026-06"}`).Data.(MonthlyClosingBatchesToolData)
	if foreignBatches.Total != 0 {
		t.Fatalf("entity B saw entity A batches: %+v", foreignBatches)
	}
	foreignLock := invoke(ctxB, lockDef, `{"period":"2026-06"}`).Data.(MonthlyClosingLockStatusToolData)
	if foreignLock.IsLocked {
		t.Fatal("entity A's entity-scoped lock leaked to entity B")
	}
	foreignContract := invoke(ctxB, entriesDef, fmt.Sprintf(`{"period":"2026-06","contract_id":%q}`, contractA1))
	if fp := foreignContract.Data.(MonthlyClosingEntriesPreviewToolData); fp.ApprovalStatus.Total != 0 {
		t.Fatalf("foreign contract_id leaked entries: %+v", fp)
	}

	// 6. 零写入：工具调用前后业务表行数不变。
	after := tableCounts()
	for table, count := range after {
		if before[table] != count {
			t.Fatalf("read tools changed %s: %d -> %d", table, before[table], count)
		}
	}
}
