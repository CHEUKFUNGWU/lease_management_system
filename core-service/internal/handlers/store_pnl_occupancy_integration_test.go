package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

type occupancyFakeKPI map[string]storepnl.KPIAggregates

func (m occupancyFakeKPI) Operating(_ context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	return m[ref.StoreID], nil
}

// TestStorePnlOccupancyContractSplitPostgres locks S1-5 level 2 end to
// end: real contract payment rows flow through the production occupancy
// port into the store P&L's occupancy row as per-contract 基本租金 /
// 服务费 / 变量租金 — the aggregate components are derived from the
// split, and a narrower window prorates the amounts by covered days.
func TestStorePnlOccupancyContractSplitPostgres(t *testing.T) {
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
			t.Fatalf("fixture exec: %v (%s)", err, sql)
		}
	}

	suffix := uuid.NewString()[:8]
	entity := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "OCC-E-"+suffix, "Occupancy "+suffix)
	landlord := uuid.NewString()
	exec(`INSERT INTO landlords (id, code, name) VALUES ($1,$2,$3)`, landlord, "OCC-L-"+suffix, "Landlord")
	storeID := uuid.NewString()
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand, is_active) VALUES ($1,$2,$3,$4,'east','b1',true)`,
		storeID, "OCC-S-"+suffix, "Store", entity)
	contractA := uuid.NewString()
	contractB := uuid.NewString()
	for _, contract := range []string{contractA, contractB} {
		exec(`INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, commencement_date, lease_start_date, lease_end_date, currency, status, approval_status, lease_scope)
			VALUES ($1,$2,$3,$4,$5,$6,'real_estate','2025-01-01','2025-01-01','2029-12-31','CNY','active','approved','in_scope')`,
			contract, "OCC-C-"+uuid.NewString()[:8], "Occ contract", entity, storeID, landlord)
	}
	// 合同 A：基本 3000 + 服务费 300 + 变量 600；合同 B：基本 1200。
	row := func(contract, start, end, timing string, amount float64, variable, nonLease bool) {
		exec(`INSERT INTO lease_payment_schedules
			(contract_id, effective_start_date, effective_end_date, coverage_start_date, coverage_end_date, due_date, payment_timing, amount, currency, amount_type, is_fixed, is_variable, is_lease_component, is_non_lease_component, included_in_liability_pv)
			VALUES ($1,$2,$3,$2,$3,$3,$4,$5,'CNY',$6,$7,$8,$9,$10,$11)`,
			contract, start, end, timing, amount,
			map[bool]string{true: "variable", false: "service_fee"}[nonLease || variable],
			!variable, variable, !nonLease, nonLease, !variable)
	}
	row(contractA, "2026-07-01", "2026-07-31", "postpaid", 3000, false, false)
	row(contractA, "2026-07-01", "2026-07-31", "postpaid", 300, false, true)
	row(contractA, "2026-07-01", "2026-07-31", "postpaid", 600, true, false)
	row(contractB, "2026-07-01", "2026-07-31", "postpaid", 1200, false, false)

	revenue := 10000.0
	kpi := occupancyFakeKPI{storeID: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY"}}
	pnlHandler := NewStorePnlHandler(kpi, nil, nil).
		WithOccupancy(NewStorePnlOccupancyAdapter(repository.NewPaymentScheduleRepository(pool)))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/stores/:id/pnl", func(c *gin.Context) {
		c.Set("legal_entity_id", entity)
		pnlHandler.Projection(c)
	})

	type rowView struct {
		Key           string                   `json:"key"`
		Components    []storepnl.Component     `json:"components"`
		ContractSplit []storepnl.ContractSplit `json:"contract_split"`
	}
	type pnlBody struct {
		Pnl struct {
			Operating struct {
				Rows []rowView `json:"rows"`
			} `json:"operating"`
		} `json:"pnl"`
	}

	get := func(query string) (int, pnlBody) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/stores/"+storeID+"/pnl?"+query, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var body pnlBody
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse body: %v (%s)", err, w.Body.String())
		}
		return w.Code, body
	}

	// 全月窗口：拆分为两个合同，聚合组件由拆分导出。
	code, body := get("as_of=2026-08-19&period=2026-07&basis=operating")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	var occupancy *rowView
	for i := range body.Pnl.Operating.Rows {
		if body.Pnl.Operating.Rows[i].Key == "occupancy_cost" {
			occupancy = &body.Pnl.Operating.Rows[i]
		}
	}
	if occupancy == nil {
		t.Fatal("occupancy row missing")
	}
	if len(occupancy.ContractSplit) != 2 {
		t.Fatalf("contract split = %+v", occupancy.ContractSplit)
	}
	byContract := map[string]storepnl.ContractSplit{}
	for _, split := range occupancy.ContractSplit {
		byContract[split.ContractID] = split
	}
	if a := byContract[contractA]; a.BasicRent == nil || *a.BasicRent != 3000 || a.ServiceFee == nil || *a.ServiceFee != 300 || a.VariableRent == nil || *a.VariableRent != 600 {
		t.Fatalf("contract A split wrong: %+v", a)
	}
	if b := byContract[contractB]; b.BasicRent == nil || *b.BasicRent != 1200 || b.ServiceFee != nil || b.VariableRent != nil {
		t.Fatalf("contract B split wrong: %+v", b)
	}
	if len(occupancy.Components) != 3 || occupancy.Components[0].Value == nil || *occupancy.Components[0].Value != 4200 {
		t.Fatalf("aggregate components must derive from the split: %+v", occupancy.Components)
	}

	// 窄窗口：金额按覆盖天数摊配（as_of=07-10 + 滚动 10 天 → 10/31）。
	_, narrow := get("as_of=2026-07-10&window_days=10&basis=operating&primary=actual")
	for i := range narrow.Pnl.Operating.Rows {
		if narrow.Pnl.Operating.Rows[i].Key == "occupancy_cost" {
			for _, split := range narrow.Pnl.Operating.Rows[i].ContractSplit {
				if split.ContractID == contractA && split.BasicRent != nil {
					want := 3000.0 * 10 / 31
					if *split.BasicRent-want > 1e-6 || want-*split.BasicRent > 1e-6 {
						t.Fatalf("prorated basic rent = %v, want %v", *split.BasicRent, want)
					}
					return
				}
			}
		}
	}
	t.Fatal("narrow-window prorated split missing")
}
