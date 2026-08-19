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
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// aggFakeKPI serves per-store aggregates the aggregate endpoint projects.
type aggFakeKPI map[string]storepnl.KPIAggregates

func (m aggFakeKPI) Operating(_ context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	aggregates, ok := m[ref.StoreID]
	if !ok {
		return storepnl.KPIAggregates{}, os.ErrNotExist
	}
	return aggregates, nil
}

// TestStorePnlAggregatePostgres locks S1-7 end to end: the authorized
// store set comes from the scoped master-data read (entity/region cuts),
// grouping splits by the requested dimension, and a mixed-currency group
// renders as per-currency partitions with the explicit T14 note — never a
// cross-currency total.
func TestStorePnlAggregatePostgres(t *testing.T) {
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
	entityA := uuid.NewString()
	entityB := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES
		($1,$2,$3,'CN','CNY'), ($4,$5,$6,'CN','CNY')`,
		entityA, "AGG-A-"+suffix, "Agg A "+suffix, entityB, "AGG-B-"+suffix, "Agg B "+suffix)
	storeA1 := uuid.NewString()
	storeA2 := uuid.NewString()
	storeB1 := uuid.NewString()
	exec(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand, is_active) VALUES
		($1,$2,'Agg A1',$3,'east','b1',true),
		($4,$5,'Agg A2',$3,'west','b1',true),
		($6,$7,'Agg B1',$8,'east','b1',true)`,
		storeA1, "AGG-S1-"+suffix, entityA, storeA2, "AGG-S2-"+suffix, storeB1, "AGG-S3-"+suffix, entityB)

	revenue := 100.0
	kpi := aggFakeKPI{
		storeA1: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY"},
		storeA2: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "USD"},
		storeB1: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY"},
	}
	handler := NewStorePnlHandler(kpi, nil, nil).WithMasterData(repository.NewMasterDataRepository(pool))

	gin.SetMode(gin.TestMode)
	serve := func(scope access.Scope, query string) (int, string) {
		t.Helper()
		router := gin.New()
		router.GET("/store-pnl/aggregate", func(c *gin.Context) {
			c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
			c.Set("legal_entity_id", scope.LegalEntityID)
			handler.AggregateProjection(c)
		})
		req := httptest.NewRequest(http.MethodGet, "/store-pnl/aggregate?"+query, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	type groupView struct {
		Key           string `json:"key"`
		StoreCount    int    `json:"store_count"`
		MixedCurrency bool   `json:"mixed_currency"`
		Note          string `json:"note"`
		Partitions    []struct {
			Currency string `json:"currency"`
		} `json:"partitions"`
	}
	type body struct {
		Aggregate struct {
			Groups         []groupView `json:"groups"`
			Note           string      `json:"note"`
			DegradedStores []any       `json:"degraded_stores"`
		} `json:"aggregate"`
	}

	// 1. 区域受限：仅 east 门店进入（west 与 B 法人被 Data Scope 排除）。
	code, raw := serve(access.Scope{LegalEntityID: entityA, Regions: []string{"east"}}, "group_by=region&as_of=2026-08-19")
	if code != http.StatusOK {
		t.Fatalf("scoped aggregate status %d: %s", code, raw)
	}
	var scoped body
	if err := json.Unmarshal([]byte(raw), &scoped); err != nil {
		t.Fatal(err)
	}
	if len(scoped.Aggregate.Groups) != 1 || scoped.Aggregate.Groups[0].Key != "east" || scoped.Aggregate.Groups[0].StoreCount != 1 {
		t.Fatalf("region-scoped aggregate must contain only authorized stores, got %+v", scoped.Aggregate.Groups)
	}

	// 2. 法人范围：A 的两家（east CNY + west USD）各自成组，不混币种。
	code, raw = serve(access.Scope{LegalEntityID: entityA}, "group_by=region&as_of=2026-08-19")
	if code != http.StatusOK {
		t.Fatalf("entity-scoped status %d: %s", code, raw)
	}
	var entityView body
	if err := json.Unmarshal([]byte(raw), &entityView); err != nil {
		t.Fatal(err)
	}
	if len(entityView.Aggregate.Groups) != 2 {
		t.Fatalf("entity-scoped groups = %+v", entityView.Aggregate.Groups)
	}
	for _, group := range entityView.Aggregate.Groups {
		if group.MixedCurrency || len(group.Partitions) != 1 {
			t.Fatalf("single-currency group must stay a single partition: %+v", group)
		}
	}

	// 3. 全局 + 按法人：A 组混币种 → 2 个币种分区 + T14 声明，绝不加总。
	// 集成库为共享 fixture 库：只断言本测试的门店入组且无人降级，其他
	// fixture 留下的门店/降级记录与本测试无关。
	code, raw = serve(access.Scope{Global: true}, "group_by=legal_entity&as_of=2026-08-19")
	if code != http.StatusOK {
		t.Fatalf("global status %d: %s", code, raw)
	}
	var global body
	if err := json.Unmarshal([]byte(raw), &global); err != nil {
		t.Fatal(err)
	}
	var groupA, groupB *groupView
	for i := range global.Aggregate.Groups {
		switch global.Aggregate.Groups[i].Key {
		case entityA:
			g := global.Aggregate.Groups[i]
			groupA = &g
		case entityB:
			g := global.Aggregate.Groups[i]
			groupB = &g
		}
	}
	if groupA == nil || groupA.StoreCount != 2 || !groupA.MixedCurrency || len(groupA.Partitions) != 2 {
		t.Fatalf("entity A group must be mixed-currency with two partitions: %+v", groupA)
	}
	if groupA.Note == "" || global.Aggregate.Note == "" {
		t.Fatalf("cross-currency refusal must be stated: group=%q aggregate=%q", groupA.Note, global.Aggregate.Note)
	}
	if groupB == nil || groupB.StoreCount != 1 || groupB.MixedCurrency || len(groupB.Partitions) != 1 {
		t.Fatalf("entity B group must stay a single-currency partition: %+v", groupB)
	}
	for _, degraded := range global.Aggregate.DegradedStores {
		if degraded == nil {
			continue
		}
		item, _ := degraded.(map[string]any)
		id, _ := item["store_id"].(string)
		if id == storeA1 || id == storeA2 || id == storeB1 {
			t.Fatalf("our stores must not degrade in the global view: %+v", degraded)
		}
	}
}
