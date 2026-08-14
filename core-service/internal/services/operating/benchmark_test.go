package operating

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

func benchmarkFixture() []*repository.StoreOperatingFact {
	p := func(v float64) *float64 { return &v }
	base := func(store, code, brand, region, currency string, revenue float64) *repository.StoreOperatingFact {
		return &repository.StoreOperatingFact{
			StoreID: store, StoreCode: code, StoreName: "门店" + code, Brand: brand, Region: region,
			Period: "2026-06", Currency: currency, Revenue: revenue,
			GrossProfit: p(30000), AreaSqm: p(100), LaborCost: p(20000), FixedRent: p(8000),
			VariableRent: p(2000), NonLeaseCost: p(1000), OtherControllableCost: p(3000),
			SourceSystem: "pos", Version: 1, ReconciliationStatus: "matched", MappingStatus: "mapped",
			AsOfAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		}
	}
	facts := make([]*repository.StoreOperatingFact, 0, 8)
	// five CNY stores in 品牌A/华东 -> each has 4 peers (>= MinimumPeerCount)
	for i := 0; i < 5; i++ {
		facts = append(facts, base("store-a"+string(rune('0'+i)), "A00"+string(rune('1'+i)), "品牌A", "华东", "CNY", 100000))
	}
	// two USD stores in the same brand+region -> each has 1 peer (below minimum)
	facts = append(facts, base("store-a5", "A006", "品牌A", "华东", "USD", 100000))
	facts = append(facts, base("store-a6", "A007", "品牌A", "华东", "USD", 100000))
	// one solo store in another region -> 0 peers
	facts = append(facts, base("store-a7", "A008", "品牌A", "华北", "CNY", 100000))
	return facts
}

// K4: Peer Cohort follows the retail-kpi-v1 rule — same brand + region +
// currency, minimum 3 peers, otherwise no benchmark; cohorts never span
// currencies.
func TestBenchmarkStoresCohortRules(t *testing.T) {
	benchmarks := BenchmarkStores(benchmarkFixture())
	byStore := map[string]StoreBenchmark{}
	for _, b := range benchmarks {
		byStore[b.StoreCode] = b
	}
	for _, code := range []string{"A001", "A002", "A003", "A004", "A005"} {
		b, ok := byStore[code]
		if !ok {
			t.Fatalf("store %s missing from benchmarks", code)
		}
		if b.PeerCount != 4 || b.PeerAverage == nil || b.Percentile == nil {
			t.Fatalf("store %s peers=%d average=%v percentile=%v, want 4 peers with benchmark", code, b.PeerCount, b.PeerAverage, b.Percentile)
		}
	}
	// USD pair: only 1 peer each — below the minimum, no benchmark produced.
	for _, code := range []string{"A006", "A007"} {
		b, ok := byStore[code]
		if !ok {
			t.Fatalf("store %s missing from benchmarks", code)
		}
		if b.PeerCount != 1 || b.PeerAverage != nil || b.Percentile != nil {
			t.Fatalf("store %s peers=%d average=%v percentile=%v, want no benchmark below minimum", code, b.PeerCount, b.PeerAverage, b.Percentile)
		}
	}
	// Solo store: 0 peers.
	b, ok := byStore["A008"]
	if !ok {
		t.Fatal("store A008 missing from benchmarks")
	}
	if b.PeerCount != 0 || b.PeerAverage != nil || b.Percentile != nil {
		t.Fatalf("store A008 peers=%d average=%v percentile=%v, want no benchmark", b.PeerCount, b.PeerAverage, b.Percentile)
	}
	// The rule constant is the shared one.
	if retailkpi.MinimumPeerCount != 3 {
		t.Fatalf("MinimumPeerCount = %d, want 3", retailkpi.MinimumPeerCount)
	}
}
