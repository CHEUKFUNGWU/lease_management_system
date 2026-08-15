package reporting

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// The disclosure and projection wire output must be semantically identical to
// what the float64 rows emitted before the money migration (MONEY-004 D3).
// The references in testdata/wire_before were captured from the pre-migration
// engine on the same fixture. Both sides are quantised to the cent before
// comparison: float64 accumulation noise (e.g. 483295.49909999996) is exactly
// what the migration removes, so raw byte equality is neither possible nor
// wanted — any difference that survives quantisation is a real value change
// and fails this test.
func TestProjectionWireQuantisedIdenticalToPreMigration(t *testing.T) {
	periodStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	snapshot := d3WireSnapshot()

	requests := map[string]ProjectionRequest{
		"disclosure":   {Kind: KindDisclosure, StartDate: periodStart, EndDate: periodEnd},
		"cashflow":     {Kind: KindCashflow, View: ViewSummary, Granularity: GranularityMonth, StartDate: periodStart, EndDate: periodEnd},
		"amortization": {Kind: KindAmortization, View: ViewSummary, Granularity: GranularityMonth, StartDate: periodStart, EndDate: periodEnd, ReportCurrency: "USD", ExchangeRate: 1.23},
		"sensitivity":  {Kind: KindSensitivity, ContractID: "contract-1", Shocks: []float64{0, -0.01, 0.01}},
		"portfolio":    {Kind: KindPortfolio},
		"standard":     {Kind: KindStandardComparison, ContractID: "contract-1"},
	}

	for name, request := range requests {
		result, err := Project(snapshot, request)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		raw, err := json.Marshal(result.Payload)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		reference, err := os.ReadFile(filepath.Join("testdata", "wire_before", name+".json"))
		if err != nil {
			t.Fatalf("read reference %s: %v", name, err)
		}
		if !quantisedWireEqual(raw, reference) {
			t.Errorf("%s: wire JSON differs from pre-migration reference after quantising to the cent\n got: %s\nwant: %s", name, raw, reference)
		}
	}
}

// d3WireSnapshot is the fixture the wire references were captured with: one
// capitalized lease with event adjustments, one exempt lease, and one
// capitalized lease with a non-2dp discount rate and tax amounts, so every
// disclosure path is exercised.
func d3WireSnapshot() *Snapshot {
	commencement := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	tax := 500.0
	return &Snapshot{
		ID: "snapshot-d3", PolicyVersion: policyVersion, Mode: Working,
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Contracts: []ContractFact{
			{
				Contract: &repository.Contract{
					ID: "contract-1", ContractNumber: "LC-001", ContractName: "旗舰店租约",
					StoreName: "南京东路旗舰店", AssetType: "real_estate", Currency: "CNY",
					CommencementDate: commencement,
					LeaseEndDate:     time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
					LeaseScope:       ifrs16.LeaseScopeInScope,
				},
				PaymentSchedules: monthlyRentSchedules(commencement, 36, 10000, "postpaid"),
				DiscountRate:     0.05,
				EventAdjustments: []*repository.EventAdjustment{
					{EffectiveDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), AdjustmentType: "rent_change", LiabilityAdjustment: 5000, ROUAdjustment: 5000, PnLGain: 0, PnLLoss: 0},
					{EffectiveDate: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), AdjustmentType: "impairment", ROUAdjustment: -2500, PnLLoss: 2500},
				},
			},
			{
				Contract: &repository.Contract{
					ID: "contract-2", ContractNumber: "LC-002", ContractName: "短期仓库",
					AssetType: "real_estate", Currency: "CNY",
					CommencementDate: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
					LeaseEndDate:     time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
					LeaseScope:       ifrs16.LeaseScopeShortTermExempt,
				},
				PaymentSchedules: monthlyRentSchedules(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), 6, 3000, "postpaid"),
				DiscountRate:     0.05,
			},
			{
				Contract: &repository.Contract{
					ID: "contract-3", ContractNumber: "LC-003", ContractName: "车辆租约",
					StoreName: "配送中心", AssetType: "vehicle", Currency: "CNY",
					CommencementDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LeaseEndDate:     time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
					LeaseScope:       ifrs16.LeaseScopeInScope,
				},
				PaymentSchedules: func() []*repository.PaymentSchedule {
					var schedules []*repository.PaymentSchedule
					for month := 1; month <= 12; month++ {
						schedules = append(schedules, &repository.PaymentSchedule{
							DueDate: time.Date(2025, time.Month(month), 15, 0, 0, 0, 0, time.UTC),
							Amount:  5000, PaymentTiming: "postpaid", IsFixed: true, TaxAmount: &tax,
						})
					}
					return schedules
				}(),
				DiscountRate: 0.048,
			},
		},
	}
}

// quantisedWireEqual reports whether two JSON documents are equal after every
// number is quantised to the cent (the currency scale the disclosures emit at).
func quantisedWireEqual(got, want []byte) bool {
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		return false
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		return false
	}
	gotBytes, _ := json.Marshal(quantiseToCent(gotValue))
	wantBytes, _ := json.Marshal(quantiseToCent(wantValue))
	return string(gotBytes) == string(wantBytes)
}

func quantiseToCent(value any) any {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			node[key] = quantiseToCent(child)
		}
	case []any:
		for i, child := range node {
			node[i] = quantiseToCent(child)
		}
	case float64:
		return math.Round(node*100) / 100
	}
	return value
}
