package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

func TestRetailKPIPostgresGoldenVersionSourceCurrencyAndTenantBoundaries(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "kpi-a")
	entityB, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "kpi-b")
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		cleanup := func(statement string, args ...interface{}) {
			if _, err := pool.Exec(cleanupCtx, statement, args...); err != nil {
				t.Errorf("MAX-003 PostgreSQL cleanup failed: %v", err)
			}
		}
		// Facts reference stores; datasets reference batches; batches and
		// stores reference legal entities. Keep this order so a failed test
		// cannot silently leak its tenant or registry rows.
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM fpna_plan_versions WHERE legal_entity_id IN ($1,$2) AND source='retail_simulator_budget'`, entityA, entityB)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2) AND source_system='retail_simulator'`, entityA, entityB)
		cleanup(`DELETE FROM stores WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
	})
	repo := NewRetailSimulationRepository(pool)
	payloadA, inputA, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planA, err := retailsimulation.Build(entityA, inputA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityA, nil, "kpi-generate-a", payloadA, planA); err != nil {
		t.Fatal(err)
	}
	payloadB, inputB, err := retailsimulation.PayloadSHA256(entityB, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planB, err := retailsimulation.Build(entityB, inputB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityB, nil, "kpi-generate-b", payloadB, planB); err != nil {
		t.Fatal(err)
	}

	kpiRepo := NewRetailKPIRepository(pool)
	scopedA := access.WithScope(ctx, access.Scope{LegalEntityID: entityA})
	set, err := kpiRepo.QueryFacts(scopedA, entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Facts) != 10860 || set.ExpectedStoreCount != 60 {
		t.Fatalf("A facts=%d expected stores=%d", len(set.Facts), set.ExpectedStoreCount)
	}
	from, _ := parseKPIDate("2026-01-01")
	to, _ := parseKPIDate("2026-06-30")
	rows, coverage, err := retailkpi.AggregateFacts(set.Facts, retailkpi.Request{DateFrom: from, DateTo: to, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-06-30", GroupBy: "total", ExpectedStoreCount: set.ExpectedStoreCount})
	if err != nil {
		t.Fatal(err)
	}
	assertKPIIntegrationValue(t, rows[0], "revenue", 44645082.59)
	assertKPIIntegrationValue(t, rows[0], "gross_profit", 13828135.35)
	assertKPIIntegrationValue(t, rows[0], "occupancy_cash_cost", 4244221.96)
	assertKPIIntegrationValue(t, rows[0], "store_contribution", 1881556.30)
	if coverage.ObservedStoreDays != 10860 || coverage.ExpectedStoreDays != 10860 || coverage.CoverageRate == nil || *coverage.CoverageRate != 100 {
		t.Fatalf("coverage=%+v", coverage)
	}

	otherTenant, err := kpiRepo.QueryFacts(scopedA, entityB, "2026-01-01", "2026-06-30", "simulated", planB.DatasetVersion, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherTenant.Facts) != 0 || otherTenant.ExpectedStoreCount != 0 {
		t.Fatalf("cross-tenant visibility facts=%d stores=%d", len(otherTenant.Facts), otherTenant.ExpectedStoreCount)
	}
	regionSet, err := kpiRepo.QueryFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{"North"}}), entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "retail_simulator", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(regionSet.Facts) != 15*181 || regionSet.ExpectedStoreCount != 15 {
		t.Fatalf("region scope facts=%d stores=%d", len(regionSet.Facts), regionSet.ExpectedStoreCount)
	}
	brandSet, err := kpiRepo.QueryFacts(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{"Northwind"}}), entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "retail_simulator", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(brandSet.Facts) != 15*181 || brandSet.ExpectedStoreCount != 15 {
		t.Fatalf("brand scope facts=%d stores=%d", len(brandSet.Facts), brandSet.ExpectedStoreCount)
	}

	// A higher version replaces the selected row without changing the fact count.
	var storeID string
	if err := pool.QueryRow(ctx, `SELECT id FROM stores WHERE legal_entity_id=$1 AND data_classification='simulated' AND simulation_dataset_version=$2 ORDER BY code LIMIT 1`, entityA, planA.DatasetVersion).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification,simulation_dataset_version) VALUES ($1,'2026-01-01','CNY',99999,30000,1000,2000,100,10000,1000,100,100,100,'retail_simulator',2,'unreconciled','mapped','valid','simulated',$2)`, storeID, planA.DatasetVersion); err != nil {
		t.Fatal(err)
	}
	set, err = kpiRepo.QueryFacts(scopedA, entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "retail_simulator", nil)
	if err != nil {
		t.Fatal(err)
	}
	foundHigher := false
	for _, fact := range set.Facts {
		if fact.StoreID == storeID && fact.BusinessDate.Format("2006-01-02") == "2026-01-01" {
			foundHigher = fact.Version == 2
			break
		}
	}
	if len(set.Facts) != 10860 || !foundHigher {
		t.Fatalf("highest version not selected count=%d found=%v", len(set.Facts), foundHigher)
	}

	// A second source for the same store-day is rejected unless explicitly selected.
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification,simulation_dataset_version) VALUES ($1,'2026-01-02','CNY',100,50,2,4,100,10,1,1,1,1,'other_source',1,'unreconciled','mapped','valid','simulated',$2)`, storeID, planA.DatasetVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := kpiRepo.QueryFacts(scopedA, entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "", nil); !errors.Is(err, ErrRetailKPISourceConflict) {
		t.Fatalf("source conflict=%v", err)
	}
	if _, err := kpiRepo.QueryFacts(scopedA, entityA, "2026-01-01", "2026-06-30", "simulated", planA.DatasetVersion, "retail_simulator", nil); err != nil {
		t.Fatalf("explicit source should succeed: %v", err)
	}
	production, err := kpiRepo.QueryFacts(scopedA, entityA, "2026-01-01", "2026-06-30", "production", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(production.Facts) != 0 {
		t.Fatalf("simulated data leaked into production query: %d", len(production.Facts))
	}
}

func parseKPIDate(value string) (time.Time, error) { return time.Parse("2006-01-02", value) }

func assertKPIIntegrationValue(t *testing.T, row retailkpi.Aggregate, code string, want float64) {
	t.Helper()
	got := row.KPIs[code].Value
	if got == nil || *got < want-0.011 || *got > want+0.011 {
		t.Fatalf("%s expected %.2f got %v", code, want, got)
	}
}
