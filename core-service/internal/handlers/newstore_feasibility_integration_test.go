package handlers

// R2-2（RH4）：新店可行性端点的跨法人隔离证据（底线 1）。
//
// 租赁投影端口在构造时绑定请求租户；SQL 再按 lease_contracts.legal_entity_id
// 过滤。本文件验证：法人 B 的请求携带法人 A 的 contract_id 时，投影查不到
// 计量行 → 端口未接线语义（lease_projection_unwired Gap），绝不出现 A 的
// 租赁数字；法人 A 自身则拿到 complete 且租赁成本与种子一致。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newstoreITPool(t *testing.T) *pgxpool.Pool {
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

func seedNewStoreTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (legalEntityID, contractID string) {
	t.Helper()
	suffix := label + "-" + time.Now().UTC().Format("150405")
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "NS-LE-"+suffix, "NewStore entity "+label).Scan(&legalEntityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	var storeID, landlordID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id
	`, "NS-ST-"+suffix, "NewStore store "+label, legalEntityID, "Brand-"+label, "Region-"+label).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO landlords (code, name) VALUES ($1, $2) RETURNING id
	`, "NS-LD-"+suffix, "Landlord "+label).Scan(&landlordID); err != nil {
		t.Fatalf("seed landlord: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO lease_contracts (contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type,
		                             lease_start_date, lease_end_date, commencement_date, status, currency, lease_scope)
		VALUES ($1, $2, $3, $4, $5, 'shop', '2026-01-01', '2030-12-31', '2026-01-01', 'active', 'CNY', 'in_scope')
		RETURNING id
	`, "NS-CT-"+suffix, "NewStore contract "+label, legalEntityID, storeID, landlordID).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	// 24 个已计算计量月：现金租金各 30000（与 golden fixture 同分布）
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		period := start.AddDate(0, i, 0).Format("2006-01")
		monthEnd := start.AddDate(0, i+1, -1).Format("2006-01-02")
		if _, err := pool.Exec(ctx, `
			INSERT INTO measurement_results (contract_id, accounting_period, period_start_date, period_end_date,
			                                 total_payment, depreciation, interest_expense, discount_rate, is_calculated)
			VALUES ($1, $2, $3, $4, 30000, 20000, 5000, 0.05, true)
		`, contractID, period, period+"-01", monthEnd); err != nil {
			t.Fatalf("seed measurement: %v", err)
		}
	}
	return legalEntityID, contractID
}

func TestNewStoreFeasibilityIsolatedAcrossLegalEntities(t *testing.T) {
	ctx := context.Background()
	pool := newstoreITPool(t)
	entityA, contractA := seedNewStoreTenant(t, ctx, pool, "ent-a")
	entityB, _ := seedNewStoreTenant(t, ctx, pool, "ent-b")

	h := NewNewStoreFeasibilityHandler(pool)
	r := gin.New()
	r.POST("/feasibility", func(c *gin.Context) {
		c.Set("legal_entity_id", c.GetHeader("X-Tenant"))
		h.Evaluate(c)
	})

	body := func(contractID string) string {
		return `{
			"currency": "CNY", "start_month": "2026-01", "horizon": 24,
			"business": {"daily_area_footfall": 500, "operating_days": 30, "entry_rate": 0.2,
			             "conversion_rate": 0.5, "avg_ticket": 200, "gross_margin_rate": 0.4},
			"investment": {"fitout_and_equipment": 800000, "initial_inventory": 200000,
			               "ramp_months": 6, "ramp_factors": [0.5, 0.6, 0.7, 0.8, 0.9, 1]},
			"lease": {"contract_id": "` + contractID + `"},
			"discount_rate": 0.01
		}`
	}

	// 法人 B 携带法人 A 的 contract_id：投影查不到 A 的计量行 →
	// lease_projection_unwired Gap、租赁相关字段 nil、无任何 A 的数字。
	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/feasibility", strings.NewReader(body(contractA)))
	reqB.Header.Set("X-Tenant", entityB)
	r.ServeHTTP(recB, reqB)
	var resB struct {
		Status string `json:"status"`
		Gaps   []struct {
			Kind string `json:"kind"`
		} `json:"gaps"`
		Monthly []struct {
			LeaseCost   *float64 `json:"lease_cost"`
			NetCashFlow *float64 `json:"net_cash_flow"`
		} `json:"monthly_cash_flows"`
	}
	if err := json.Unmarshal(recB.Body.Bytes(), &resB); err != nil {
		t.Fatalf("decode B: %v body=%s", err, recB.Body.String())
	}
	foundUnwired := false
	for _, g := range resB.Gaps {
		if g.Kind == "lease_projection_unwired" {
			foundUnwired = true
		}
	}
	if !foundUnwired {
		t.Fatalf("entity B gaps %v must contain lease_projection_unwired", resB.Gaps)
	}
	for i, row := range resB.Monthly {
		if row.LeaseCost != nil || row.NetCashFlow != nil {
			t.Fatalf("month %d carries numbers leaked from entity A's measurement rows", i+1)
		}
	}

	// 法人 A 自身：complete，租赁成本与种子一致（30000），满爬坡净现金流 90000。
	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodPost, "/feasibility", strings.NewReader(body(contractA)))
	reqA.Header.Set("X-Tenant", entityA)
	r.ServeHTTP(recA, reqA)
	var resA struct {
		Status string `json:"status"`
		Gaps   []struct {
			Kind string `json:"kind"`
		} `json:"gaps"`
		Monthly []struct {
			LeaseCost   *float64 `json:"lease_cost"`
			NetCashFlow *float64 `json:"net_cash_flow"`
		} `json:"monthly_cash_flows"`
		StaticPayback *float64 `json:"static_payback_months"`
	}
	if err := json.Unmarshal(recA.Body.Bytes(), &resA); err != nil {
		t.Fatal(err)
	}
	if resA.Status != "complete" || len(resA.Gaps) != 0 {
		t.Fatalf("entity A must be complete, got %+v", resA)
	}
	if len(resA.Monthly) < 24 {
		t.Fatalf("24 months expected, got %d", len(resA.Monthly))
	}
	if resA.Monthly[0].LeaseCost == nil || *resA.Monthly[0].LeaseCost != 30000 {
		t.Fatalf("month 1 lease = %v, want 30000", resA.Monthly[0].LeaseCost)
	}
	if resA.StaticPayback == nil || *resA.StaticPayback != 14 {
		t.Fatalf("static payback = %v, want 14", resA.StaticPayback)
	}
}
