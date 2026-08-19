package adapter

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/repository"
)

func exec2(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("fixture exec: %v (%s)", err, sql)
	}
}

// TestProductionPortsPostgres locks the three S2-3 adapters against real
// rows: lease roll-forward folds the engine's measurement outputs,
// the schedule reads non-lease expense, and the trial balance folds into a
// standardized opening screen that passes the SM4 gates.
func TestProductionPortsPostgres(t *testing.T) {
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
	entity := uuid.NewString()
	exec2(t, ctx, pool, `INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "PORTS-E-"+suffix, "Ports "+suffix)
	landlord := uuid.NewString()
	exec2(t, ctx, pool, `INSERT INTO landlords (id, code, name) VALUES ($1,$2,$3)`, landlord, "PORTS-L-"+suffix, "Landlord")
	store := uuid.NewString()
	exec2(t, ctx, pool, `INSERT INTO stores (id, code, name, legal_entity_id, region, brand, is_active) VALUES ($1,$2,$3,$4,'east','b1',true)`,
		store, "PORTS-S-"+suffix, "Store", entity)
	userID := uuid.NewString()
	exec2(t, ctx, pool, `INSERT INTO users (id, username, email, password_hash, legal_entity_id) VALUES ($1,$2,$3,'integration-only',$4)`,
		userID, "ports-user-"+suffix, "ports-"+suffix+"@example.com", entity)
	for _, contract := range []string{uuid.NewString(), uuid.NewString()} {
		exec2(t, ctx, pool, `INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, commencement_date, lease_start_date, lease_end_date, currency, status, approval_status, lease_scope)
			VALUES ($1,$2,$3,$4,$5,$6,'real_estate','2025-01-01','2025-01-01','2029-12-31','CNY','active','approved','in_scope')`,
			contract, "PORTS-C-"+uuid.NewString()[:8], "Ports contract", entity, store, landlord)
	}
	contracts := []string{}
	rows, _ := pool.Query(ctx, `SELECT id::text FROM lease_contracts WHERE legal_entity_id=$1 ORDER BY id`, entity)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		contracts = append(contracts, id)
	}
	rows.Close()
	if len(contracts) != 2 {
		t.Fatalf("contract fixture broken: %d rows", len(contracts))
	}
	// 计量引擎输出（period 2026-07）：两合同，含当月新租约入账。
	exec2(t, ctx, pool, `INSERT INTO measurement_results
		(contract_id, accounting_period, period_start_date, period_end_date,
		 opening_liability, interest_expense, principal_repayment, total_payment,
		 closing_liability, opening_rou_asset, depreciation, closing_rou_asset,
		 non_lease_expense, discount_rate, is_calculated)
		VALUES
		($1,'2026-07','2026-07-01','2026-07-31',410,5,10,15,400,450,10,440,25,0.05,true),
		($2,'2026-07','2026-07-01','2026-07-31',300,4,8,12,292,320,8,312,10,0.05,true)`,
		contracts[0], contracts[1])
	// TB：2026-07 标准屏（平衡分录）。
	tbID := uuid.NewString()
	exec2(t, ctx, pool, `INSERT INTO gl_trial_balance_versions (id, legal_entity_id, name, source_system, period, functional_currency, content_sha256, total_debit, total_credit)
		VALUES ($1,$2,$3,'gl-a','2026-07','CNY',$4,700,700)`, tbID, entity, "PORTS-TB-"+suffix, uuid.NewString())
	exec2(t, ctx, pool, `INSERT INTO gl_trial_balance_lines (trial_balance_version_id, account_code, account_name, debit, credit) VALUES
		($1,'1001','货币资金',500,0),
		($1,'1601','固定资产',200,0),
		($1,'2801','租赁负债',0,500),
		($1,'4001','实收资本',0,200)`, tbID)

	measurements := repository.NewMonthlyClosingRepository(pool)
	trial := repository.NewOperatingFactsRepository(pool)

	// 1. 租赁投影。
	leaseOut, err := NewLeaseReader(measurements).Monthly(ctx, entity, "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if leaseOut.ROUAsset == nil || *leaseOut.ROUAsset != 770 || leaseOut.LeaseLiability == nil || *leaseOut.LeaseLiability != 710 ||
		leaseOut.Depreciation == nil || *leaseOut.Depreciation != 18 {
		t.Fatalf("lease fold over engine rows wrong: %+v", leaseOut)
	}

	// 2. 付款计划（非租赁成分）与 trio 适配。
	schedOut, err := NewScheduleReader(measurements, nil, nil).Monthly(ctx, entity, "2026-07")
	if err != nil || schedOut.ServiceFee == nil || *schedOut.ServiceFee != 35 {
		t.Fatalf("schedule fold wrong: %+v (%v)", schedOut, err)
	}

	// 3. 期初余额：标准化屏过三闸。
	balance, ref, engineBalances, policy, err := NewOpeningReader(trial, measurements).Get(ctx, entity)
	if err != nil {
		t.Fatal(err)
	}
	if balance == nil || len(balance.Periods) != 1 {
		t.Fatalf("balance = %+v", balance)
	}
	if failures := opening.Validate(opening.ValidateInput{Balance: *balance, LeaseRef: ref, Engine: engineBalances, Policy: policy}); len(failures) > 0 {
		t.Fatalf("standard balance must pass the gates: %+v", failures)
	}
	if len(engineBalances) != len(contracts) {
		t.Fatalf("gate-3 engine balances must cover every measured contract: %d vs %d", len(engineBalances), len(contracts))
	}

	// 4. 走正式 RunDefinition 的同一输入形状：引擎通过端口拿到租货行值。
	tmpl, err := template.DefaultStatementTemplate()
	if err != nil {
		t.Fatal(err)
	}
	inputs := finmodel.ModelInputs{
		Assumptions:        finmodel.AssumptionReader(kvAssumptions{"gross_margin_rate": 0.4, "borrow_interest_rate": 0.05, "tax_rate": 0.25, "dso": 10, "dio": 10, "dpo": 10, "days": 30, "dividend_payout_rate": 0, "sssg": 0, "labor_cost_growth": 0, "fixed_rent_growth": 0, "variable_rent_growth": 0, "non_lease_cost_growth": 0, "other_controllable_cost_growth": 0, "borrowings": 0, "share_capital": 200, "other_depreciation": 0, "capex": 0}),
		Versions:           finmodel.VersionSet{},
		DataClassification: "production",
		Facts:              nil, // 事实端口与本用例无关：行缺失即缺失
		Lease:              NewLeaseReader(measurements),
		Schedules:          NewScheduleReader(measurements, kvAssumptions{"share_capital": 200, "borrowings": 0, "other_depreciation": 0}, nil),
		Opening:            NewOpeningReader(trial, measurements),
	}
	def := finmodel.ModelDef{
		Name: "ports-test", LegalEntityID: entity, Currency: "CNY", Template: tmpl,
		PeriodStart: "2026-07", HistoricalMonths: 2, ForecastMonths: 2,
		ActualCutoffPeriod: "",
		Policy:             finmodel.ModelPolicy{Version: "v1", InterestCashFlowPresentation: "financing", InterestMethod: "opening_balance"},
	}
	result, err := finmodel.Run(ctx, def, inputs)
	if err != nil {
		t.Fatalf("Run with production ports: %v", err)
	}
	foundLeaseValue := false
	for _, line := range result.Lines {
		if line.RowKey == "lease_liability" && line.Period == "2026-07" && line.Value != nil {
			foundLeaseValue = true
		}
	}
	if !foundLeaseValue {
		t.Fatalf("the lease port must feed the run's lease rows")
	}
}

type kvAssumptions map[string]float64

func (k kvAssumptions) Value(_ context.Context, _, key, _ string) (json.RawMessage, error) {
	if value, ok := k[key]; ok {
		return json.Marshal(value)
	}
	return nil, nil
}
