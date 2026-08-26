package s1

import (
	"fmt"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/leasescenario"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// sampleInput is one realistic pre-deal assumption set (simulated quote in
// the style of public tender listings).
func sampleInput() Input {
	commence := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	return Input{
		Draft: leasescenario.Draft{
			Name:                    "方案A：某购物中心一层门店",
			CommencementDate:        commence,
			TermMonths:              36,
			MonthlyRent:             50000,
			RentFreeMonths:          2,
			AnnualEscalationPercent: 3,
			DiscountRate:            0.0485,
			Currency:                "CNY",
			InitialDirectCost:       80000,
			EarlyExitPenaltyMonths:  3,
		},
		Offers: []leasescenario.Offer{
			{Name: "店A", TermMonths: 36, BaseMonthlyRent: 50000, RentFreeMonths: 2, AnnualEscalationPercent: 3, AreaSqm: 120},
			{Name: "店B", TermMonths: 36, BaseMonthlyRent: 48000, RentFreeMonths: 0, AnnualEscalationPercent: 5, AreaSqm: 110},
		},
		ShocksPercent: []float64{-0.01, -0.005, 0.005, 0.01},
		ConfirmedBy:   "bp-zhang",
		ConfirmedAt:   "2026-08-19T10:00:00Z",
		ToolCallID:    "call-s1-1",
	}
}

// CORR-1 deterministic half: every engine-derived cell must equal the engine
// direct output — the builder performs no arithmetic of its own.
func TestBuildEngineConsistency(t *testing.T) {
	in := sampleInput()
	briefing, err := leasescenario.Build(in.Draft)
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := leasescenario.Compare(leasescenario.CompareInput{DiscountRate: in.Draft.DiscountRate, Currency: in.Draft.Currency, Offers: in.Offers})
	if err != nil {
		t.Fatal(err)
	}
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}

	byRef := map[string]workingpaper.Cell{}
	for _, c := range paper.AllCells() {
		byRef[c.Ref] = c
	}
	check := func(ref string, label string, want any) {
		t.Helper()
		c, ok := byRef[ref]
		if !ok {
			t.Fatalf("missing cell %s (%s)", ref, label)
		}
		if c.Value != want {
			t.Fatalf("cell %s = %v, engine says %v", ref, c.Value, want)
		}
		if c.Provenance.Basis != workingpaper.BasisCertified {
			t.Fatalf("cell %s must be Certified, got %s", ref, c.Provenance.Basis)
		}
	}
	check("IF-1", "初始租赁负债", briefing.BalanceSheet.InitialLiability)
	check("IF-2", "初始使用权资产", briefing.BalanceSheet.InitialROU)
	check("IF-3", "实际采用的折现率", briefing.DiscountRate)
	for _, y := range briefing.Yearly {
		check("IF-"+fmt.Sprint(y.Year)+"-interest", "利息", y.Interest)
		check("IF-"+fmt.Sprint(y.Year)+"-depreciation", "折旧", y.Depreciation)
	}
	for _, r := range briefing.Bridge {
		check("EB-"+fmt.Sprint(r.Year)+"-uplift", "EBITDA 提升", r.EBITDAUplift)
	}
	for _, o := range comparison.Offers {
		check("DC-"+o.Name+"-effective-rent", "有效月租金", o.EffectiveMonthlyRent)
		check("DC-"+o.Name+"-pv", "现值", o.PresentValue)
	}
	// Sensitivity cells equal a direct engine re-run with the shocked rate.
	for _, shock := range in.ShocksPercent {
		variant := in.Draft
		variant.DiscountRate = in.Draft.DiscountRate * (1 + shock)
		shocked, err := leasescenario.Build(variant)
		if err != nil {
			t.Fatal(err)
		}
		check("SE-"+fmt.Sprint(shock)+"-liability", "冲击后初始负债", shocked.BalanceSheet.InitialLiability)
	}
}

// I1/I2/I3/I6: the assembled paper must pass the fail-closed lint with the
// audited call known completed, and carry zero exploratory cells.
func TestBuildPaperPassesLint(t *testing.T) {
	in := sampleInput()
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	paper = workingpaper.Build(paper, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(paper, auditSet{"call-s1-1": true})
	if !rep.OK {
		t.Fatalf("S1 paper must pass lint, got %+v", rep.Violations)
	}
	if refs := paper.ExploratoryRefs(); len(refs) != 0 {
		t.Fatalf("S1 is pure Tier A: no exploratory cells allowed, got %v", refs)
	}
}

// The discount rate is never guessed: unconfirmed assumptions fail the build.
func TestBuildRequiresHumanConfirmation(t *testing.T) {
	in := sampleInput()
	in.ConfirmedBy = ""
	if _, err := Build(in); err == nil {
		t.Fatal("empty confirmed_by must fail")
	}
	in = sampleInput()
	in.ConfirmedAt = ""
	if _, err := Build(in); err == nil {
		t.Fatal("empty confirmed_at must fail")
	}
	in = sampleInput()
	in.ToolCallID = ""
	if _, err := Build(in); err == nil {
		t.Fatal("empty tool_call_id must fail (I2 anchor)")
	}
}

type auditSet map[string]bool

func (a auditSet) CompletedToolCall(callID string) bool { return a[callID] }
