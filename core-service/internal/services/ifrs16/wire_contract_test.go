package ifrs16

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
)

// The wire JSON of the calculation chain must not change because of the money
// migration (ADR-0020 §Decision 6, MONEY-003 C6). The references in
// testdata/wire_before were captured from the float64 engine; both sides are
// normalised the same way before comparison, and the normalisation is exactly
// the two things the old engine did not guarantee:
//
//   - monthly summaries are sorted by year and month (the old engine emitted
//     them in map-iteration order, which is random on every run);
//   - numbers are quantised to the cent. The old float64 aggregation emitted
//     noise digits (e.g. 40.200000000000024 for a monthly interest sum); the
//     decimal engine emits 40.2. That is the migration's point, not a change
//     of the contract — any real value movement still fails the comparison.
//
// Everything else must be byte-identical: same structure, same field order,
// numbers unquoted, same digits at the cent.
func TestWireJSONIdenticalToPreMigration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	var payments []LeasePayment
	for month := 0; month < 36; month++ {
		monthEnd := start.AddDate(0, month+1, 0).AddDate(0, 0, -1)
		payments = append(payments, LeasePayment{
			Date: monthEnd, Amount: money.NewFromInt64(10000), Timing: "postpaid", Type: "fixed",
		})
	}

	capitalized, err := Calculate(LeaseCalculation{
		CommencementDate: start, LeaseEndDate: end,
		LeaseScope: LeaseScopeInScope, DiscountRate: 0.05, Payments: payments,
	})
	if err != nil {
		t.Fatal(err)
	}
	exempt, err := Calculate(LeaseCalculation{
		CommencementDate: start, LeaseEndDate: end,
		LeaseScope: LeaseScopeShortTermExempt, DiscountRate: 0.05, Payments: payments,
	})
	if err != nil {
		t.Fatal(err)
	}
	remeasurement, err := RecalculateFromDate(money.NewFromInt64(100000), money.NewFromInt64(90000), RemeasurementInput{
		EffectiveDate:       time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
		LeaseEndDate:        end,
		RevisedDiscountRate: 0.05,
		RevisedPayments:     payments,
	})
	if err != nil {
		t.Fatal(err)
	}
	carrying, rou, err := GetCarryingAmount(LeaseCalculation{
		CommencementDate: start, LeaseEndDate: end,
		LeaseScope: LeaseScopeInScope, DiscountRate: 0.05, Payments: payments,
	}, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := DeriveRevisedPayments(payments, PaymentRevision{
		Kind:        RevisionPercentage,
		Percentage:  5,
		AppliesFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}, start)
	if err != nil {
		t.Fatal(err)
	}

	payloads := map[string]any{
		"calc": map[string]any{
			"initial_liability": capitalized.InitialLiability,
			"initial_rou":       capitalized.InitialROUAsset,
			"monthly_summary":   capitalized.MonthlySummary,
			"daily_sample": []any{
				capitalized.DailyAmortization[0],
				capitalized.DailyAmortization[len(capitalized.DailyAmortization)-1],
			},
		},
		"straight": map[string]any{
			"measurement_basis": exempt.MeasurementBasis,
			"monthly_summary":   exempt.MonthlySummary,
		},
		"remeasure": map[string]any{
			"new_liability":   remeasurement.NewLiability,
			"liability_delta": remeasurement.LiabilityDelta,
			"rou_adjustment":  remeasurement.ROUAdjustment,
			"pnl_gain":        remeasurement.PnLGain,
			"pnl_loss":        remeasurement.PnLLoss,
			"new_rou":         remeasurement.NewROU,
		},
		"carrying": map[string]any{"liability": carrying, "rou": rou},
		"revision": revision,
	}

	for name, payload := range payloads {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		reference, err := os.ReadFile(filepath.Join("testdata", "wire_before", name+".json"))
		if err != nil {
			t.Fatalf("read reference %s: %v", name, err)
		}
		got, want := normaliseWireJSON(raw), normaliseWireJSON(reference)
		if !bytes.Equal(got, want) {
			t.Errorf("%s: wire JSON differs from pre-migration reference\n got: %s\nwant: %s", name, got, want)
		}
	}
}

// normaliseWireJSON sorts monthly summaries and quantises every number to the
// cent (see the test comment for why both sides need it).
func normaliseWireJSON(raw []byte) []byte {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		panic(err)
	}
	normaliseValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return out
}

func normaliseValue(value any) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			node[key] = normaliseChild(child)
			if rows, ok := node["monthly_summary"].([]any); ok {
				sort.Slice(rows, func(i, j int) bool {
					yearI, _ := rows[i].(map[string]any)["Year"].(float64)
					yearJ, _ := rows[j].(map[string]any)["Year"].(float64)
					monthI, _ := rows[i].(map[string]any)["Month"].(float64)
					monthJ, _ := rows[j].(map[string]any)["Month"].(float64)
					if yearI != yearJ {
						return yearI < yearJ
					}
					return monthI < monthJ
				})
			}
		}
	case []any:
		for i, child := range node {
			node[i] = normaliseChild(child)
		}
	}
}

// normaliseChild returns a node with every descendant number quantised to the
// cent; maps and lists are normalised in place.
func normaliseChild(value any) any {
	switch node := value.(type) {
	case map[string]any:
		normaliseValue(node)
		return node
	case []any:
		normaliseValue(node)
		return node
	case float64:
		return math.Round(node*100) / 100
	}
	return value
}
