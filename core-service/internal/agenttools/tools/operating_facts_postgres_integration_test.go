package tools

// B-3 经营事实工具带库集成测试（底线 1 + 底线 3）：跨法人隔离、来源信封
// 逐字段回显、零写入。用 make test-integration 起一次性库实跑；
// TEST_DATABASE_URL 未设则 skip。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestOperatingFactsToolsPostgresIsolationEnvelopeNoWrites(t *testing.T) {
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
	runPattern := "AGENT-B3-" + runToken + "%"
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v (%s)", err, sql)
		}
	}

	var entityA, entityB, storeA string
	cleanup := func() {
		cctx := context.Background()
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM retail_store_day_facts WHERE store_id IN (SELECT id FROM stores WHERE code = $1)`, []any{"AGENTS-" + runToken}},
			{`DELETE FROM store_operating_facts WHERE store_id IN (SELECT id FROM stores WHERE code = $1)`, []any{"AGENTS-" + runToken}},
			{`DELETE FROM operating_fact_batches WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runPattern}},
			{`DELETE FROM stores WHERE code = $1`, []any{"AGENTS-" + runToken}},
			{`DELETE FROM legal_entities WHERE code LIKE $1`, []any{runPattern}},
		} {
			if _, err := pool.Exec(cctx, statement.sql, statement.args...); err != nil {
				t.Errorf("cleanup: %v (%s)", err, statement.sql)
			}
		}
	}
	t.Cleanup(cleanup)

	writeCounts := map[string]string{
		"retail_store_day_facts": `SELECT COUNT(*) FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id WHERE s.code = $1`,
		"store_operating_facts":  `SELECT COUNT(*) FROM store_operating_facts f JOIN stores s ON s.id=f.store_id WHERE s.code = $1`,
	}
	tableCounts := func() map[string]int64 {
		counts := map[string]int64{}
		for table, query := range writeCounts {
			var count int64
			if err := pool.QueryRow(ctx, query, "AGENTS-"+runToken).Scan(&count); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			counts[table] = count
		}
		return counts
	}
	before := tableCounts()
	_ = before // seed 后重定基线（下方），断言的是「工具调用不改变行数」

	// 种子：法人 A 一个门店、5 天 production 事实（信封字段各不相同）+ 法人 B（空）。
	entityA, entityB, storeA = uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES
		($1,$2,$3,'CN','CNY'), ($4,$5,$6,'CN','CNY')`,
		entityA, "AGENT-B3-"+runToken+"-a", "B3 A "+runToken,
		entityB, "AGENT-B3-"+runToken+"-b", "B3 B "+runToken)
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES ($1,$2,'B3 store',$3,'east','b1')`,
		storeA, "AGENTS-"+runToken, entityA)
	var batchJune string
	if err := pool.QueryRow(ctx, `INSERT INTO operating_fact_batches (legal_entity_id, source_system, status, total_rows, accepted_rows, reconciliation_status) VALUES ($1,'agent-b3-fixture','completed',5,5,'matched') RETURNING id`,
		entityA).Scan(&batchJune); err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	// 月粒度 store_operating_facts 一行：/operating-facts/stores 的数据源，
	// 同样携带五个信封字段。
	exec(`INSERT INTO store_operating_facts
		(store_id, period, period_basis, currency, revenue, gross_profit,
		 source_system, import_batch_id, as_of_at, version, data_quality_status)
		VALUES ($1,'2026-06','calendar_month','CNY',5000,1500,'agent-b3-fixture',$2,$3::timestamptz,3,'valid')`,
		storeA, batchJune, time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC))
	for day := 3; day <= 7; day++ {
		version := day - 1 // version 2..6，验证范围回显
		asOf := time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC)
		exec(`INSERT INTO retail_store_day_facts
			(store_id, business_date, currency, revenue, gross_profit, transactions, footfall, area_sqm,
			 labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
			 source_system, source_record_id, import_batch_id, as_of_at, version,
			 reconciliation_status, mapping_status, data_quality_status, data_classification)
			VALUES ($1, $2::date, 'CNY', 1000, 300, 100, 200, 100,
			 80, 50, 20, 10, 5,
			 'agent-b3-fixture', $3, $4, $5::timestamptz, $6,
			 'matched', 'mapped', 'valid', 'production')`,
			storeA, fmt.Sprintf("2026-06-%02d", day), fmt.Sprintf("src-%02d", day), batchJune, asOf, version)
	}

	// 种子后重定基线：零写入断言的是工具调用不改变行数。
	before = tableCounts()

	repo := repository.NewOperatingFactsRepository(pool)
	storesDef := NewOperatingStoresDefinition(repo)
	daysDef := NewOperatingStoreDaysDefinition(repo)
	kpiRepo := repository.NewRetailKPIRepository(pool)
	kpiDef := NewKpiStoreDaysDefinition(kpiRepo)

	newCtx := func(legalEntityID string) context.Context {
		scope := access.Scope{LegalEntityID: legalEntityID}
		base := access.WithScope(context.Background(), scope)
		return agenttools.WithExecutionContext(base, agenttools.ExecutionContext{
			Principal: agenttools.Principal{UserID: "agent-b3", Permissions: []string{"reports:read"}, Scope: scope},
			RunID:     "agent-b3", SkillID: "retail_operations", SkillVersion: "v1",
		})
	}
	ctxA, ctxB := newCtx(entityA), newCtx(entityB)
	invoke := func(callCtx context.Context, def agenttools.ToolDefinition, args string) agenttools.ToolResult {
		t.Helper()
		result, err := def.Handler(callCtx, agenttools.ToolCall{
			CallID: "c", RunID: "agent-b3", ToolName: def.Descriptor.Name, ToolVersion: "v1",
			Arguments: json.RawMessage(args),
		})
		if err != nil {
			t.Fatalf("%s handler error: %v", def.Descriptor.Name, err)
		}
		return result
	}

	// 1. 法人 A：store-day 原始事实 → 每行五个信封字段逐一回显。
	result := invoke(ctxA, daysDef, `{"date_from":"2026-06-01","date_to":"2026-06-30"}`)
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("store-days read failed: %+v", result.Error)
	}
	days := result.Data.(OperatingStoreDaysToolData)
	if days.Total != 5 || days.ReturnedCount != 5 {
		t.Fatalf("row count wrong: total=%d returned=%d", days.Total, days.ReturnedCount)
	}
	asOfMin := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	asOfMax := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	rowIDsA := map[string]bool{}
	for _, row := range days.Data {
		rowIDsA[row.ID] = true
		// 底线 3：五个字段逐个断言，不是「信封非空」。
		if row.DataClassification != "production" {
			t.Fatalf("data_classification wrong: %q", row.DataClassification)
		}
		if row.SourceSystem != "agent-b3-fixture" {
			t.Fatalf("source_system wrong: %q", row.SourceSystem)
		}
		if row.ImportBatchID == nil || *row.ImportBatchID != batchJune {
			t.Fatalf("import_batch_id wrong: %v", row.ImportBatchID)
		}
		if row.AsOfAt.Before(asOfMin) || row.AsOfAt.After(asOfMax) {
			t.Fatalf("as_of_at wrong: %v", row.AsOfAt)
		}
		if row.Version < 2 || row.Version > 6 {
			t.Fatalf("version wrong: %d", row.Version)
		}
		// 严格 null：行级缺失指标保持 nil（labor_hours 不在行类型里，
		// 在下方 KPI 视图断言）。
		if row.GrossProfit == nil {
			t.Fatal("seeded gross_profit must be echoed")
		}
	}
	// 汇总信封回显版本范围与来源系统。
	if days.Envelope.FactVersionMin != 2 || days.Envelope.FactVersionMax != 6 {
		t.Fatalf("aggregate version range wrong: %+v", days.Envelope)
	}
	if len(days.Envelope.SourceSystems) != 1 || days.Envelope.SourceSystems[0] != "agent-b3-fixture" {
		t.Fatalf("aggregate source systems wrong: %+v", days.Envelope.SourceSystems)
	}

	// 2. 法人 A：KPI store-day 聚合。先断言降级纪律：请求 30 天但只有 5 天
	// 事实 → decision_ready=false + incomplete_store_day_coverage。
	result = invoke(ctxA, kpiDef, `{"date_from":"2026-06-01","date_to":"2026-06-30","data_classification":"production"}`)
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("kpi read failed: %+v", result.Error)
	}
	degraded := result.Data.(KpiStoreDaysToolData)
	if degraded.DecisionReady || degraded.DecisionReadyReason != "incomplete_store_day_coverage" {
		t.Fatalf("insufficient coverage must degrade with reason: %+v", degraded)
	}
	// 再请求被覆盖的窗口（6/3-6/7 共 5 天）→ 覆盖完整 → decision_ready=true，
	// revenue = 5×1000。
	result = invoke(ctxA, kpiDef, `{"date_from":"2026-06-03","date_to":"2026-06-07","data_classification":"production"}`)
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("kpi read failed: %+v", result.Error)
	}
	kpis := result.Data.(KpiStoreDaysToolData)
	if len(kpis.Data) == 0 {
		t.Fatal("entity A must see its own aggregates")
	}
	if !kpis.DecisionReady {
		t.Fatalf("full single-store coverage should be decision ready: reason=%q coverage=%+v", kpis.DecisionReadyReason, kpis.Coverage)
	}
	revenue, exists := kpis.Data[0].KPIs["revenue"]
	if !exists || revenue.Value == nil || *revenue.Value != 5000 {
		t.Fatalf("revenue aggregate wrong: %+v", revenue)
	}
	// 严格 null（AGENTS.md 原例）：事实行没有 labor_hours，KPI 的 labor_hours
	// 必须是 nil，不许用工时成本÷假定时薪反推，也不许填 0。
	// 严格 null（AGENTS.md 原例）：事实行没有 labor_hours，sales_per_labor_hour
	// 的分母缺失 → value 必须是 nil，不许用工时成本÷假定时薪反推，也不许填 0。
	splh, exists := kpis.Data[0].KPIs["sales_per_labor_hour"]
	if !exists {
		t.Fatal("sales_per_labor_hour KPI entry must exist")
	}
	if splh.Value != nil {
		t.Fatalf("labor_hours denominator missing: sales_per_labor_hour must have nil value, got %v", *splh.Value)
	}

	// 3. 门店列表（法人 A）→ 至少包含种子门店。
	result = invoke(ctxA, storesDef, `{}`)
	stores := result.Data.(OperatingStoresToolData)
	if stores.Total < 1 {
		t.Fatalf("stores wrong: %+v", stores)
	}

	// 4. 跨法人隔离（底线 1）：法人 B 读同一窗口 → 全部为空，无 A 的行泄漏。
	foreignDays := invoke(ctxB, daysDef, `{"date_from":"2026-06-01","date_to":"2026-06-30"}`).Data.(OperatingStoreDaysToolData)
	if foreignDays.Total != 0 || len(foreignDays.Data) != 0 {
		t.Fatalf("entity B saw entity A facts: %+v", foreignDays.Total)
	}
	for _, row := range foreignDays.Data {
		if rowIDsA[row.ID] {
			t.Fatalf("entity A fact %s leaked to entity B", row.ID)
		}
	}
	foreignKpis := invoke(ctxB, kpiDef, `{"date_from":"2026-06-01","date_to":"2026-06-30","data_classification":"production"}`).Data.(KpiStoreDaysToolData)
	if foreignKpis.TotalRows != 0 {
		t.Fatalf("entity B saw entity A aggregates: %+v", foreignKpis)
	}
	foreignStores := invoke(ctxB, storesDef, `{}`).Data.(OperatingStoresToolData)
	if foreignStores.Total != 0 {
		t.Fatalf("entity B saw entity A stores: %+v", foreignStores)
	}

	// 5. 零写入：工具调用前后事实行数不变。
	after := tableCounts()
	for table, count := range after {
		if before[table] != count {
			t.Fatalf("read tools changed %s: %d -> %d", table, before[table], count)
		}
	}
}
