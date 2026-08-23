package handlers

// R2-3（RH5）：利润差异归因端点的跨法人隔离证据（底线 1）。
//
// handler 的取数路径是 repo.QueryFacts(legalEntityID, ...)，租户边界在
// 仓库层且已有专项覆盖（retail_kpi_postgres_integration_test.go）。本文件
// 验的是端点全路径：法人 B 的租户查法人 A 的门店 → 窗口内零行事实 →
// 归因整体 unavailable（no_facts），绝不产出 A 的数字；法人 A 自身则拿到
// complete 且总差异与手算一致。只跑单元测试证明不了这些。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
)

func varianceITPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return url
}

func seedVarianceTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (legalEntityID, storeID string) {
	t.Helper()
	suffix := label + "-" + time.Now().UTC().Format("150405")
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "VA-LE-"+suffix, "Variance entity "+label).Scan(&legalEntityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, $4, $5, true) RETURNING id
	`, "VA-ST-"+suffix, "Variance store "+label, legalEntityID, "Brand-"+label, "Region-"+label).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	// 两个窗口各一天：基期利润 4000 的分布、当期利润 2900 的分布
	days := []struct {
		date    time.Time
		foot    float64
		tx      float64
		rev     float64
		gp      float64
		labor   float64
		fixed   float64
		varRent float64
		nonLease float64
		other   float64
	}{
		{time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), 1000, 100, 20000, 6000, 1000, 500, 200, 100, 200},
		{time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC), 1100, 99, 24750, 4950, 1200, 450, 150, 100, 150},
	}
	for _, d := range days {
		if _, err := pool.Exec(ctx, `
			INSERT INTO retail_store_day_facts
				(store_id, business_date, currency, footfall, transactions, revenue, gross_profit,
				 labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
				 source_system, data_classification)
			VALUES ($1, $2, 'CNY', $3, $4, $5, $6, $7, $8, $9, $10, $11, 'variance-itest', 'production')
		`, storeID, d.date.Format("2006-01-02"), d.foot, d.tx, d.rev, d.gp, d.labor, d.fixed, d.varRent, d.nonLease, d.other); err != nil {
			t.Fatalf("seed fact: %v", err)
		}
	}
	return legalEntityID, storeID
}

func TestStoreVarianceAttributionIsolatedAcrossLegalEntities(t *testing.T) {
	ctx := context.Background()
	pool := varianceITPool(t)
	entityA, storeA := seedVarianceTenant(t, ctx, pool, "ent-a")
	entityB, _ := seedVarianceTenant(t, ctx, pool, "ent-b")

	reader := repository.NewRetailKPIRepository(pool)
	h := NewVarianceAttributionHandler(reader)
	r := gin.New()
	r.GET("/attr", func(c *gin.Context) {
		c.Set("legal_entity_id", c.GetHeader("X-Tenant"))
		h.StoreVarianceAttribution(c)
	})

	query := "?store_id=" + storeA + "&as_of=2026-06-30&window_days=14&data_classification=production"

	// 法人 B 查法人 A 的门店：窗口内零行事实 → 整体 unavailable + no_facts，
	// 绝不出现 A 的任何数字。
	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodGet, "/attr"+query, nil)
	reqB.Header.Set("X-Tenant", entityB)
	r.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("B status = %d body=%s", recB.Code, recB.Body.String())
	}
	var resB struct {
		Status       string   `json:"status"`
		MissingFacts []string `json:"missing_facts"`
		BaseProfit   float64  `json:"base_profit"`
		Factors      []any    `json:"factors"`
	}
	if err := json.Unmarshal(recB.Body.Bytes(), &resB); err != nil {
		t.Fatal(err)
	}
	if resB.Status != "unavailable" {
		t.Fatalf("entity B must see unavailable, got %s", resB.Status)
	}
	foundNoFacts := false
	for _, m := range resB.MissingFacts {
		if m == "no_facts" {
			foundNoFacts = true
		}
	}
	if !foundNoFacts {
		t.Fatalf("B missing facts %v must contain no_facts", resB.MissingFacts)
	}
	if resB.BaseProfit != 0 || len(resB.Factors) != 0 {
		t.Fatalf("entity B must receive no numbers from entity A's facts: %+v", resB)
	}

	// 法人 A 自己查：complete，端点数字与手算 fixture 一致。
	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodGet, "/attr"+query, nil)
	reqA.Header.Set("X-Tenant", entityA)
	r.ServeHTTP(recA, reqA)
	var resA struct {
		Status        string   `json:"status"`
		MissingFacts  []string `json:"missing_facts"`
		BaseProfit    float64  `json:"base_profit"`
		CurrentProfit float64  `json:"current_profit"`
		TotalVariance float64  `json:"total_variance"`
		Factors       []any    `json:"factors"`
	}
	if err := json.Unmarshal(recA.Body.Bytes(), &resA); err != nil {
		t.Fatal(err)
	}
	if resA.Status != "complete" || resA.BaseProfit != 4000 || resA.CurrentProfit != 2900 || resA.TotalVariance != -1100 || len(resA.Factors) != 7 {
		t.Fatalf("entity A attribution wrong: %+v\nraw=%s", resA, recA.Body.String())
	}
}
