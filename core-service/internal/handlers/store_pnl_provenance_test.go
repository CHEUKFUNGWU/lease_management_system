package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

type provenanceFacts struct{ set *repository.RetailKPIFactSet }

func (p provenanceFacts) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return p.set, nil
}

func pf(v float64) *float64 { return &v }

// TestStorePnlAdapterDerivesPerKPISourceEnvelope locks S1-5 level 3: the
// adapter folds the raw facts of the same window into per-KPI envelopes
// (source_system / import_batch_id / fact versions / as-of / days) and the
// store-level semantic envelope rides along. A global admin without a legal
// entity gets no trace but the projection still answers.
func TestStorePnlAdapterDerivesPerKPISourceEnvelope(t *testing.T) {
	batch := uuid.NewString()
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	set := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 1,
		ExpectedStores:     []retailkpi.StorePopulation{{StoreID: "S1", StoreCode: "S1", StoreName: "S1"}},
		Facts: []retailkpi.DailyFact{{
			StoreID: "S1", StoreCode: "S1", StoreName: "S1",
			BusinessDate: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), AsOfAt: asOf,
			Currency: "CNY", SourceSystem: "pos-a", DataClassification: "production",
			Version: 3, ImportBatchID: &batch,
			Revenue: pf(100), GrossProfit: pf(40), LaborCost: pf(20),
			FixedRent: pf(10), VariableRent: pf(2),
		}},
	}
	adapter := NewStorePnlKPIAdapter(provenanceFacts{set: set})
	aggregates, err := adapter.Operating(context.Background(), storepnl.StoreRef{
		LegalEntityID: "LE-1", StoreID: "S1", AsOf: "2026-08-19", WindowDays: 7, Classification: "production",
	})
	if err != nil {
		t.Fatalf("Operating: %v", err)
	}
	revenue, ok := aggregates.Provenance["revenue"]
	if !ok {
		t.Fatalf("revenue provenance missing: %+v", aggregates.Provenance)
	}
	if revenue.SourceDays != 1 || revenue.FactVersionMin != 3 || revenue.FactVersionMax != 3 {
		t.Fatalf("revenue envelope = %+v", revenue)
	}
	if len(revenue.SourceSystems) != 1 || revenue.SourceSystems[0] != "pos-a" {
		t.Fatalf("revenue source systems = %v", revenue.SourceSystems)
	}
	if len(revenue.ImportBatchIDs) != 1 || revenue.ImportBatchIDs[0] != batch {
		t.Fatalf("revenue batches = %v, want %s", revenue.ImportBatchIDs, batch)
	}
	if revenue.HighestAsOf == "" || revenue.DataClassification != "production" {
		t.Fatalf("revenue envelope as_of/classification = %+v", revenue)
	}
	// 无贡献事实的 KPI 不出现信封（不编造）。
	if _, ok := aggregates.Provenance["marketing"]; ok {
		t.Fatal("marketing has no fact column — it must have no envelope")
	}
	if aggregates.Envelope == nil {
		t.Fatal("store-level semantic envelope must ride along (S1-5 level 2)")
	}

	// 无事实窗口：投影仍可回答，溯源为空——不编造信封。
	emptySet := &repository.RetailKPIFactSet{
		Facts: []retailkpi.DailyFact{}, ExpectedStoreCount: 1,
		ExpectedStores: []retailkpi.StorePopulation{{StoreID: "S1", StoreCode: "S1", StoreName: "S1"}},
	}
	emptyAdapter := NewStorePnlKPIAdapter(provenanceFacts{set: emptySet})
	empty, err := emptyAdapter.Operating(context.Background(), storepnl.StoreRef{
		LegalEntityID: "LE-1", StoreID: "S1", AsOf: "2026-08-19", WindowDays: 7, Classification: "production",
	})
	if err != nil {
		t.Fatalf("Operating on an empty fact window must still answer: %v", err)
	}
	if len(empty.Provenance) != 0 {
		t.Fatalf("empty facts must yield no provenance entries, got %+v", empty.Provenance)
	}
}
