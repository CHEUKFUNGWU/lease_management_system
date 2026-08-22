package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

func fptr(v float64) *float64 { return &v }

// varianceFixtureDay 构造一天的事实，使窗口聚合后与 varianceattribution
// 的手算 fixture 完全同分布（基期利润 4000 / 当期利润 2900 / 总差异 −1100）。
var (
	baseDay    = time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	currentDay = time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
)

func varianceFact(storeID string, day time.Time, footfall, tx, rev, gp, labor, rent, varRent, nonLease, other *float64) retailkpi.DailyFact {
	return retailkpi.DailyFact{
		StoreID: storeID, StoreCode: "S001", StoreName: "门店", BusinessDate: day,
		Currency: "CNY", SourceSystem: "itest", DataClassification: "production",
		Footfall: footfall, Transactions: tx, Revenue: rev, GrossProfit: gp,
		LaborCost: labor, FixedRent: rent, VariableRent: varRent, NonLeaseCost: nonLease,
		OtherControllableCost: other,
	}
}

type recordingReader struct {
	set        *repository.RetailKPIFactSet
	gotEntity  string
	gotStoreIDs []string
}

func (r *recordingReader) QueryFacts(_ context.Context, legalEntityID, _, _, _, _, _ string, storeIDs []string) (*repository.RetailKPIFactSet, error) {
	r.gotEntity = legalEntityID
	r.gotStoreIDs = storeIDs
	return r.set, nil
}

func TestStoreVarianceAttributionHandlerCompletePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeID := "00000000-0000-0000-0000-000000000001"
	facts := []retailkpi.DailyFact{
		// 基期窗（1 天即可，聚合语义与多天相同）：利润 4000 的分布
		varianceFact(storeID, baseDay, fptr(1000), fptr(100), fptr(20000), fptr(6000), fptr(1000), fptr(500), fptr(200), fptr(100), fptr(200)),
		// 当期窗：利润 2900 的分布（占用 = 400+200+100）
		varianceFact(storeID, currentDay, fptr(1100), fptr(99), fptr(24750), fptr(4950), fptr(1200), fptr(450), fptr(150), fptr(100), fptr(150)),
	}
	reader := &recordingReader{set: &repository.RetailKPIFactSet{Facts: facts}}
	h := NewVarianceAttributionHandler(reader)
	r := gin.New()
	r.GET("/attr", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreVarianceAttribution(c) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/attr?store_id="+storeID+"&as_of=2026-06-30&window_days=7&data_classification=production", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if reader.gotEntity != "entity-a" {
		t.Fatalf("tenant not forwarded: %q", reader.gotEntity)
	}
	var res struct {
		Status             string   `json:"status"`
		BaseProfit         float64  `json:"base_profit"`
		CurrentProfit      float64  `json:"current_profit"`
		TotalVariance      float64  `json:"total_variance"`
		DecompositionOrder []string `json:"decomposition_order"`
		Factors            []struct {
			Factor             string  `json:"factor"`
			Effect             float64 `json:"effect"`
			IntermediateProfit float64 `json:"intermediate_profit"`
		} `json:"factors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != "complete" || len(res.Factors) != 7 || len(res.DecompositionOrder) != 7 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.BaseProfit != 4000 || res.CurrentProfit != 2900 || res.TotalVariance != -1100 {
		t.Fatalf("endpoints = %v/%v/%v, want 4000/2900/-1100", res.BaseProfit, res.CurrentProfit, res.TotalVariance)
	}
}

func TestStoreVarianceAttributionHandlerMissingFieldPropagates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeID := "00000000-0000-0000-0000-000000000001"
	facts := []retailkpi.DailyFact{
		varianceFact(storeID, baseDay, fptr(1000), fptr(100), fptr(20000), fptr(6000), nil, fptr(500), fptr(200), fptr(100), fptr(200)),
		varianceFact(storeID, currentDay, fptr(1100), fptr(99), fptr(24750), fptr(4950), fptr(1200), fptr(450), fptr(150), fptr(100), fptr(150)),
	}
	h := NewVarianceAttributionHandler(&recordingReader{set: &repository.RetailKPIFactSet{Facts: facts}})
	r := gin.New()
	r.GET("/attr", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreVarianceAttribution(c) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attr?store_id="+storeID+"&as_of=2026-06-30&window_days=7&data_classification=production", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var res struct {
		Status       string   `json:"status"`
		MissingFacts []string `json:"missing_facts"`
		Factors      []any    `json:"factors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != "unavailable" {
		t.Fatalf("status = %s, want unavailable（缺失传播到期间聚合后整体降级）", res.Status)
	}
	found := false
	for _, m := range res.MissingFacts {
		if m == "base.labor_cost" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing facts %v must contain base.labor_cost", res.MissingFacts)
	}
	if len(res.Factors) != 0 {
		t.Fatal("unavailable result must carry no factor numbers")
	}
}

func TestStoreVarianceAttributionHandlerCurrencyConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeID := "00000000-0000-0000-0000-000000000001"
	a := varianceFact(storeID, currentDay, fptr(100), fptr(10), fptr(2000), fptr(600), fptr(100), fptr(50), fptr(20), fptr(10), fptr(20))
	a.Currency = "CNY"
	b := varianceFact(storeID, currentDay.AddDate(0, 0, 1), fptr(100), fptr(10), fptr(2000), fptr(600), fptr(100), fptr(50), fptr(20), fptr(10), fptr(20))
	b.Currency = "HKD"
	h := NewVarianceAttributionHandler(&recordingReader{set: &repository.RetailKPIFactSet{Facts: []retailkpi.DailyFact{a, b}}})
	r := gin.New()
	r.GET("/attr", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreVarianceAttribution(c) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attr?store_id="+storeID+"&as_of=2026-06-30&window_days=7&data_classification=production", nil))
	var res struct {
		Status       string   `json:"status"`
		MissingFacts []string `json:"missing_facts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != "unavailable" {
		t.Fatalf("mixed currencies must degrade, got %s", res.Status)
	}
	found := false
	for _, m := range res.MissingFacts {
		if m == "currency_conflict" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing facts %v must contain currency_conflict", res.MissingFacts)
	}
}

func TestStoreVarianceAttributionHandlerEmptyWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewVarianceAttributionHandler(&recordingReader{set: &repository.RetailKPIFactSet{}})
	r := gin.New()
	r.GET("/attr", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreVarianceAttribution(c) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/attr?store_id=00000000-0000-0000-0000-000000000001&as_of=2026-06-30&window_days=7&data_classification=production", nil))
	var res struct {
		Status       string   `json:"status"`
		MissingFacts []string `json:"missing_facts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != "unavailable" {
		t.Fatalf("empty windows must be unavailable, got %s", res.Status)
	}
	found := false
	for _, m := range res.MissingFacts {
		if m == "no_facts" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing facts %v must contain no_facts", res.MissingFacts)
	}
}
