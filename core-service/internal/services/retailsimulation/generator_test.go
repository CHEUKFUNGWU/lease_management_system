package retailsimulation

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strconv"
	"testing"
	"time"
)

func TestBuildDefaultsAndGolden(t *testing.T) {
	plan, err := Build("entity-a", Input{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Seed != DefaultSeed || plan.DateFrom != DefaultDateFrom || plan.DateTo != DefaultDateTo || plan.StoreCount != DefaultStoreCount {
		t.Fatalf("defaults = seed %d dates %s..%s stores %d", plan.Seed, plan.DateFrom, plan.DateTo, plan.StoreCount)
	}
	if plan.FactCount != 10860 || len(plan.Facts) != 10860 || len(plan.Stores) != 60 || len(plan.Anomalies) != 6 {
		t.Fatalf("scale = stores %d facts %d anomalies %d", len(plan.Stores), len(plan.Facts), len(plan.Anomalies))
	}
	if plan.BusinessSHA256 == "" || plan.DatasetVersion == "" {
		t.Fatal("golden hashes are empty")
	}
	if plan.BusinessSHA256 != "8782919c8e8712afeae9142322dc453b6b5e1ce5fee4002c4613eade775a699f" {
		t.Fatalf("business golden hash = %s", plan.BusinessSHA256)
	}
	second, err := Build("entity-a", Input{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.DatasetVersion != second.DatasetVersion || plan.BusinessSHA256 != second.BusinessSHA256 {
		t.Fatalf("same input is not deterministic: %s/%s vs %s/%s", plan.DatasetVersion, plan.BusinessSHA256, second.DatasetVersion, second.BusinessSHA256)
	}
	if plan.Stores[0].Code != second.Stores[0].Code || plan.Facts[0] != second.Facts[0] || plan.Anomalies[0] != second.Anomalies[0] {
		t.Fatal("deterministic sample changed")
	}
	// This fixed digest protects the canonical first fact and is intentionally
	// independent of database UUIDs and timestamps.
	golden := sha256.Sum256([]byte(plan.DatasetVersion + "|" + plan.Stores[0].Code + "|" + plan.Facts[0].BusinessDate + "|" + formatGoldenFact(plan.Facts[0])))
	if got := hex.EncodeToString(golden[:]); got != "f3ec6ebf6dd21d46279e8eecc91384525b7cbe66d12e07ab9335adfbb5f32389" {
		t.Fatalf("golden sample hash = %s", got)
	}
}

func formatGoldenFact(f FactPlan) string {
	return f.Currency + "|" + formatFloat(f.Revenue) + "|" + formatFloat(f.GrossProfit) + "|" + formatFloat(f.Transactions) + "|" + formatFloat(f.Footfall) + "|" + formatFloat(f.AreaSqm) + "|" + formatFloat(f.LaborCost) + "|" + formatFloat(f.FixedRent)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func TestBuildSeedAndEntityChangeDataset(t *testing.T) {
	base, err := Build("entity-a", Input{Seed: 1, DateFrom: "2026-01-01", DateTo: "2026-01-28", StoreCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	differentSeed, err := Build("entity-a", Input{Seed: 2, DateFrom: "2026-01-01", DateTo: "2026-01-28", StoreCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if base.DatasetVersion == differentSeed.DatasetVersion || base.BusinessSHA256 == differentSeed.BusinessSHA256 {
		t.Fatal("changing seed did not change dataset")
	}
	otherEntity, err := Build("entity-b", Input{Seed: 1, DateFrom: "2026-01-01", DateTo: "2026-01-28", StoreCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if base.DatasetVersion == otherEntity.DatasetVersion || base.Stores[0].Code == otherEntity.Stores[0].Code {
		t.Fatal("different entities are not isolated in deterministic identifiers")
	}
}

func TestBuildRelationshipsAndExplicitAnomalies(t *testing.T) {
	plan, err := Build("entity-a", Input{Seed: 42, DateFrom: "2026-01-01", DateTo: "2026-02-28", StoreCount: 12})
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range plan.Facts {
		if fact.Transactions > fact.Footfall || fact.GrossProfit > fact.Revenue || fact.Revenue < 0 || fact.GrossProfit < 0 || fact.LaborCost < 0 || fact.FixedRent < 0 {
			t.Fatalf("invalid relationship in %+v", fact)
		}
	}
	for _, anomaly := range plan.Anomalies {
		if anomaly.StoreCode == plan.Stores[0].Code || anomaly.DateFrom == "" || anomaly.DateTo == "" || anomaly.ExpectedDirection == "" {
			t.Fatalf("invalid anomaly manifest %+v", anomaly)
		}
	}
	for anomalyIndex, anomaly := range plan.Anomalies {
		store := plan.Stores[anomalyIndex+1]
		start, _ := time.Parse("2006-01-02", anomaly.DateFrom)
		end, _ := time.Parse("2006-01-02", anomaly.DateTo)
		var previousFootfall float64
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			dayIndex := int(date.Sub(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
			actual, ok := findFact(plan.Facts, store.Code, date.Format("2006-01-02"))
			if !ok {
				t.Fatalf("missing anomaly fact for %s %s", store.Code, date.Format("2006-01-02"))
			}
			baseline := baselineFact(plan.Seed, store.Index, store, date, dayIndex)
			switch anomaly.Type {
			case "footfall_continuous_decline":
				if actual.Footfall > baseline.Footfall || (previousFootfall > 0 && actual.Footfall >= previousFootfall) {
					t.Fatalf("footfall anomaly is not strictly declining: actual=%+v baseline=%+v previous=%.0f", actual, baseline, previousFootfall)
				}
				previousFootfall = actual.Footfall
			case "conversion_rate_drop":
				if actual.Transactions >= baseline.Transactions || actual.Footfall != baseline.Footfall {
					t.Fatalf("conversion anomaly direction mismatch: actual=%+v baseline=%+v", actual, baseline)
				}
			case "average_ticket_drop":
				if actual.Revenue/actual.Transactions >= baseline.Revenue/baseline.Transactions {
					t.Fatalf("ticket anomaly did not decline: actual=%+v baseline=%+v", actual, baseline)
				}
			case "gross_margin_compression":
				if actual.GrossProfit/actual.Revenue >= baseline.GrossProfit/baseline.Revenue {
					t.Fatalf("margin anomaly did not compress: actual=%+v baseline=%+v", actual, baseline)
				}
			case "labor_cost_spike":
				if actual.LaborCost/actual.Revenue <= baseline.LaborCost/baseline.Revenue {
					t.Fatalf("labor anomaly did not rise: actual=%+v baseline=%+v", actual, baseline)
				}
			case "occupancy_cost_burden":
				if actual.FixedRent/actual.AreaSqm <= baseline.FixedRent/baseline.AreaSqm {
					t.Fatalf("occupancy anomaly did not rise: actual=%+v baseline=%+v", actual, baseline)
				}
			}
			if anomaly.Type != "gross_margin_compression" && !approximatelyEqual(safeRatio(actual.GrossProfit, actual.Revenue), safeRatio(baseline.GrossProfit, baseline.Revenue), 0.0002) {
				t.Fatalf("non-margin anomaly changed gross margin ratio: type=%s actual=%+v baseline=%+v", anomaly.Type, actual, baseline)
			}
			if anomaly.Type != "labor_cost_spike" && !approximatelyEqual(safeRatio(actual.LaborCost, actual.Revenue), safeRatio(baseline.LaborCost, baseline.Revenue), 0.0002) {
				t.Fatalf("non-labor anomaly changed labor ratio: type=%s actual=%+v baseline=%+v", anomaly.Type, actual, baseline)
			}
			if anomaly.Type != "occupancy_cost_burden" && actual.FixedRent != baseline.FixedRent {
				t.Fatalf("non-occupancy anomaly changed fixed rent: type=%s actual=%+v baseline=%+v", anomaly.Type, actual, baseline)
			}
			if !approximatelyEqual(safeRatio(actual.VariableRent, actual.Revenue), 0.012, 0.0002) || !approximatelyEqual(safeRatio(actual.NonLeaseCost, actual.Revenue), 0.009, 0.0002) || !approximatelyEqual(safeRatio(actual.OtherControllableCost, actual.Revenue), 0.055, 0.0002) {
				t.Fatalf("anomaly changed non-target income cost ratios: type=%s actual=%+v", anomaly.Type, actual)
			}
			if anomaly.Type == "footfall_continuous_decline" || anomaly.Type == "conversion_rate_drop" || anomaly.Type == "average_ticket_drop" {
				if !approximatelyEqual(safeRatio(actual.GrossProfit, actual.Revenue), safeRatio(baseline.GrossProfit, baseline.Revenue), 0.0002) {
					t.Fatalf("sales anomaly changed gross margin ratio: actual=%+v baseline=%+v", actual, baseline)
				}
				if !approximatelyEqual(safeRatio(actual.LaborCost, actual.Revenue), safeRatio(baseline.LaborCost, baseline.Revenue), 0.0002) || !approximatelyEqual(safeRatio(actual.VariableRent, actual.Revenue), 0.012, 0.0002) || !approximatelyEqual(safeRatio(actual.NonLeaseCost, actual.Revenue), 0.009, 0.0002) || !approximatelyEqual(safeRatio(actual.OtherControllableCost, actual.Revenue), 0.055, 0.0002) {
					t.Fatalf("sales anomaly changed income-proportional cost ratios: actual=%+v", actual)
				}
			}
		}
	}
	control := plan.Stores[0].Code
	for _, fact := range plan.Facts {
		if fact.StoreCode == control && fact.BusinessDate >= plan.Anomalies[0].DateFrom && fact.BusinessDate <= plan.Anomalies[0].DateTo {
			// The first store is a no-injection control, so its series remains valid
			// without appearing in the manifest.
			if fact.Revenue == 0 || fact.Footfall == 0 {
				t.Fatal("control store unexpectedly lost baseline values")
			}
		}
	}
}

func findFact(facts []FactPlan, storeCode, date string) (FactPlan, bool) {
	for _, fact := range facts {
		if fact.StoreCode == storeCode && fact.BusinessDate == date {
			return fact, true
		}
	}
	return FactPlan{}, false
}

func approximatelyEqual(left, right, tolerance float64) bool {
	return math.Abs(left-right) <= tolerance
}
