package tools

// B-4 报表工具带库集成测试（底线 1）：跨法人隔离、三档口径的累积语义、
// draft 叠加层只在 draft 口径出现、零写入。用 make test-integration 实跑；
// TEST_DATABASE_URL 未设则 skip。

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

func TestReportToolsPostgresBasisIsolationNoWrites(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	runToken := uuid.NewString()[:12]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v (%s)", err, sql)
		}
	}

	var entityA, entityB, storeA, landlordA string
	var cApproved, cPending, cDraft string
	cleanup := func() {
		cctx := context.Background()
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM lease_payment_schedules WHERE contract_id IN (SELECT id FROM lease_contracts WHERE contract_number LIKE $1)`, []any{runToken + "%"}},
			{`DELETE FROM fpna_scenario_drafts WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runToken + "%"}},
			{`DELETE FROM fpna_plan_versions WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runToken + "%"}},
			{`DELETE FROM fpna_assumption_versions WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`, []any{runToken + "%"}},
			{`DELETE FROM measurement_results m USING lease_contracts lc WHERE m.contract_id=lc.id AND lc.contract_number LIKE $1`, []any{runToken + "%"}},
			{`DELETE FROM lease_contracts WHERE contract_number LIKE $1`, []any{runToken + "%"}},
			{`DELETE FROM stores WHERE code = $1`, []any{"AGENTS-" + runToken}},
			{`DELETE FROM landlords WHERE code = $1`, []any{"AGENTL-" + runToken}},
			{`DELETE FROM legal_entities WHERE code LIKE $1`, []any{runToken + "%"}},
		} {
			if _, err := pool.Exec(cctx, statement.sql, statement.args...); err != nil {
				t.Errorf("cleanup: %v (%s)", err, statement.sql)
			}
		}
	}
	t.Cleanup(cleanup)

	scopedCounts := map[string]string{
		"lease_payment_schedules": `SELECT COUNT(*) FROM lease_payment_schedules s JOIN lease_contracts c ON c.id=s.contract_id WHERE c.contract_number LIKE $1`,
		"lease_contracts":         `SELECT COUNT(*) FROM lease_contracts WHERE contract_number LIKE $1`,
	}
	tableCounts := func() map[string]int64 {
		counts := map[string]int64{}
		for table, query := range scopedCounts {
			var count int64
			if err := pool.QueryRow(ctx, query, runToken+"%").Scan(&count); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			counts[table] = count
		}
		return counts
	}
	before := tableCounts()

	// 种子：法人 A 三份合同（approved / submitted / draft）各一条付款计划，
	// 加 draft 叠加层三种用户自建内容；法人 B 空。
	entityA, entityB = uuid.NewString(), uuid.NewString()
	storeA, landlordA = uuid.NewString(), uuid.NewString()
	cApproved, cPending, cDraft = uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES
		($1,$2,$3,'CN','CNY'), ($4,$5,$6,'CN','CNY')`,
		entityA, runToken+"-a", "B4 A "+runToken,
		entityB, runToken+"-b", "B4 B "+runToken)
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES ($1,$2,'B4 store',$3,'east','b1')`,
		storeA, "AGENTS-"+runToken, entityA)
	exec(`INSERT INTO landlords (id, code, name) VALUES ($1,$2,'B4 landlord')`,
		landlordA, "AGENTL-"+runToken)
	exec(`INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, currency, commencement_date, lease_start_date, lease_end_date, status, approval_status, lease_scope, discount_rate_value) VALUES
		($1,$2,'B4 approved',$3,$4,$5,'property','CNY','2024-01-01','2024-01-01','2030-12-31','approved','approved','in_scope',0.05),
		($6,$7,'B4 submitted',$3,$4,$5,'property','CNY','2024-01-01','2024-01-01','2030-12-31','submitted','submitted','in_scope',0.05),
		($8,$9,'B4 draft',$3,$4,$5,'property','CNY','2024-01-01','2024-01-01','2030-12-31','draft','draft','in_scope',0.05)`,
		cApproved, runToken+"-C-approved", entityA, storeA, landlordA,
		cPending, runToken+"-C-submitted",
		cDraft, runToken+"-C-draft")
	for _, sc := range []struct {
		contract, status string
	}{
		{cApproved, "approved"}, {cPending, "submitted"}, {cDraft, "draft"},
	} {
		exec(`INSERT INTO lease_payment_schedules
			(contract_id, effective_start_date, effective_end_date, coverage_start_date, coverage_end_date, due_date,
			 payment_timing, amount, currency, amount_type, is_fixed, approval_status)
			VALUES ($1,'2024-02-01','2030-12-01','2024-02-01','2030-12-31','2024-02-01',
			 'postpaid',50000,'CNY','fixed',true,$2)`,
			sc.contract, sc.status)
	}
	exec(`INSERT INTO fpna_scenario_drafts (legal_entity_id, scenario_type, name, assumptions, data_version, status)
		VALUES ($1,'renewal','B4 客流下降情景','{"revenue_change_pct":-10}','sim-v1','draft')`, entityA)
	exec(`INSERT INTO fpna_plan_versions (legal_entity_id, name, version_type, scenario_type, source, as_of_period, from_period, to_period, status)
		VALUES ($1,'B4 custom plan','scenario','custom','agent-b4','2026-06','2026-07','2026-12','draft')`, entityA)
	exec(`INSERT INTO fpna_assumption_versions (legal_entity_id, assumption_key, category, value, source, effective_from, version, status)
		VALUES ($1,'global_discount_rate','discount_rate','{"value":0.05}','agent-b4','2026-01-01',1,'draft')`, entityA)

	before = tableCounts() // 种子后重定基线

	builder := reporting.NewSnapshotBuilder(
		repository.NewContractRepository(pool),
		repository.NewPaymentScheduleRepository(pool),
		repository.NewSystemSettingRepository(pool),
		repository.NewMonthlyClosingRepository(pool),
	).WithStores(repository.NewMasterDataRepository(pool))
	reader := NewReportSnapshotReader(builder, repository.NewFPnAGovernanceRepository(pool), repository.NewOperatingFactsRepository(pool), nil, repository.NewMonthlyClosingRepository(pool))
	scheduleDef := NewReportScheduleDefinition(reader)

	newCtx := func(legalEntityID string) context.Context {
		scope := access.Scope{LegalEntityID: legalEntityID}
		base := access.WithScope(context.Background(), scope)
		return agenttools.WithExecutionContext(base, agenttools.ExecutionContext{
			Principal: agenttools.Principal{UserID: "agent-b4", Permissions: []string{"reports:read"}, Scope: scope},
			RunID:     "agent-b4", SkillID: "fpna_copilot", SkillVersion: "v1",
		})
	}
	ctxA, ctxB := newCtx(entityA), newCtx(entityB)

	rowsFor := func(callCtx context.Context, basis string) map[string]bool {
		t.Helper()
		result, err := scheduleDef.Handler(callCtx, agenttools.ToolCall{
			CallID: "c", RunID: "agent-b4", ToolName: scheduleDef.Descriptor.Name, ToolVersion: "v1",
			Arguments: json.RawMessage(`{"kind":"liability_rolling","report_basis":"` + basis + `"}`),
		})
		if err != nil {
			t.Fatalf("liability_rolling %s handler error: %v", basis, err)
		}
		if result.Status != agenttools.StatusCompleted {
			t.Fatalf("liability_rolling %s failed: %+v", basis, result.Error)
		}
		data := result.Data.(ReportProjectionToolData)
		if data.ReportBasis != basis || data.SideEffects {
			t.Fatalf("envelope wrong for %s: %+v", basis, data)
		}
		raw, _ := json.Marshal(data.Payload["data"])
		var rows []struct {
			ContractNumber    string `json:"contract_number"`
			ApprovalStatus    string `json:"approval_status"`
			IsOfficialVersion bool   `json:"is_official_version"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("payload rows: %v", err)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			seen[row.ContractNumber] = true
			if row.ApprovalStatus == "" {
				t.Fatal("each row must carry approval_status")
			}
		}
		return seen
	}

	// 1. 累积口径：approved ⊂ pending ⊂ draft。
	approvedRows := rowsFor(ctxA, "approved")
	pendingRows := rowsFor(ctxA, "pending")
	draftRows := rowsFor(ctxA, "draft")
	if len(approvedRows) != 1 || !approvedRows[runToken+"-C-approved"] {
		t.Fatalf("approved basis must contain only the approved contract: %v", approvedRows)
	}
	if len(pendingRows) != 2 || !pendingRows[runToken+"-C-submitted"] || !pendingRows[runToken+"-C-approved"] {
		t.Fatalf("pending basis must be Pending+Approved: %v", pendingRows)
	}
	if len(draftRows) != 3 || !draftRows[runToken+"-C-draft"] {
		t.Fatalf("draft basis must admit all three: %v", draftRows)
	}

	// 2. 叠加层只在 draft 出现，且内容经过筛选。
	for _, basis := range []string{"approved", "pending"} {
		result, err := scheduleDef.Handler(ctxA, agenttools.ToolCall{
			CallID: "c", RunID: "agent-b4", ToolName: scheduleDef.Descriptor.Name, ToolVersion: "v1",
			Arguments: json.RawMessage(`{"kind":"liability_rolling","report_basis":"` + basis + `"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		data := result.Data.(ReportProjectionToolData)
		if data.UserOverlays != nil {
			t.Fatalf("basis %q must never read drafts back", basis)
		}
	}
	draftResult, err := scheduleDef.Handler(ctxA, agenttools.ToolCall{
		CallID: "c", RunID: "agent-b4", ToolName: scheduleDef.Descriptor.Name, ToolVersion: "v1",
		Arguments: json.RawMessage(`{"kind":"liability_rolling","report_basis":"draft"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	overlays := draftResult.Data.(ReportProjectionToolData).UserOverlays
	if overlays == nil {
		t.Fatal("draft basis must surface user overlays")
	}
	draftsJSON, _ := json.Marshal(overlays["scenario_drafts"])
	if !strings.Contains(string(draftsJSON), "B4 客流下降情景") {
		t.Fatalf("overlay scenario drafts wrong: %s", draftsJSON)
	}
	plansJSON, _ := json.Marshal(overlays["custom_plan_versions"])
	if !strings.Contains(string(plansJSON), "B4 custom plan") {
		t.Fatalf("overlay custom plans wrong: %s", plansJSON)
	}
	assumptionsJSON, _ := json.Marshal(overlays["draft_assumptions"])
	if !strings.Contains(string(assumptionsJSON), "global_discount_rate") {
		t.Fatalf("overlay draft assumptions wrong: %s", assumptionsJSON)
	}

	// 3. 跨法人隔离（底线 1）：三个口径下法人 B 全部为空。
	for _, basis := range []string{"approved", "pending", "draft"} {
		foreign := rowsFor(ctxB, basis)
		if len(foreign) != 0 {
			t.Fatalf("entity B saw entity A contracts under %s basis: %v", basis, foreign)
		}
	}

	// 4. 零写入。
	after := tableCounts()
	for table, count := range after {
		if before[table] != count {
			t.Fatalf("read tools changed %s: %d -> %d", table, before[table], count)
		}
	}
}
