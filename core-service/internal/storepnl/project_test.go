package storepnl

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

type memKPI struct{ facts KPIAggregates }

func (m memKPI) Operating(_ context.Context, _ StoreRef) (KPIAggregates, error) {
	return m.facts, nil
}

type memPlan struct {
	values map[string]*float64 // column|source -> value
}

func (m memPlan) StoreValue(_ context.Context, _ StoreRef, column ColumnRef, kpi string) (*float64, error) {
	return m.values[string(column)+"|"+kpi], nil
}

type memLease struct{ lease LeaseMonthValues }

func (m memLease) Monthly(_ context.Context, _ string, _ string) (LeaseMonthValues, error) {
	return m.lease, nil
}

type memPeer struct{}

func (memPeer) Median(_ context.Context, _ StoreRef, _ string) (*float64, string, bool) {
	return nil, "insufficient_peers", false
}

func pf(v float64) *float64 { return &v }

func testTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.DefaultStorePnlTemplate()
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func testFacts() KPIAggregates {
	return KPIAggregates{
		Revenue: pf(1000), GrossProfit: pf(400), LaborCost: pf(200),
		NonLeaseCost: pf(60), OtherControllable: pf(40),
		FixedRent: pf(150), ServiceFee: pf(20), VariableRent: pf(50),
		DecisionReady: true, Classification: "production", Currency: "CNY",
	}
}

func TestSideBySideKeepsBasisBlocksSeparate(t *testing.T) {
	tmpl := testTemplate(t)
	readers := Readers{
		KPI:   memKPI{facts: testFacts()},
		Plan:  memPlan{values: map[string]*float64{}},
		Lease: memLease{lease: LeaseMonthValues{ROUDepreciation: pf(55), LeaseInterest: pf(10), OtherDepreciation: pf(8)}},
		Peer:  memPeer{},
	}
	pnl, err := Project(context.Background(), tmpl, StoreRef{StoreID: "S1", AsOf: "2026-08-18", WindowDays: 7, Classification: "production"}, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisSideBySide, readers)
	if err != nil {
		t.Fatal(err)
	}
	if pnl.Operating == nil || pnl.Ifrs16 == nil {
		t.Fatal("side_by_side must render both basis blocks")
	}
	if pnl.Operating.Basis != "operating_basis" || pnl.Ifrs16.Basis != "ifrs16_basis" {
		t.Fatalf("blocks must carry their basis labels (T15), got %s/%s", pnl.Operating.Basis, pnl.Ifrs16.Basis)
	}
	// T15 structure: occupancy (operating) never appears in the ifrs16 block,
	// ROU depreciation (ifrs16) never in the operating block.
	inBlock := func(block *Block, key string) bool {
		for _, row := range block.Rows {
			if row.Key == key {
				return true
			}
		}
		return false
	}
	if inBlock(pnl.Ifrs16, "occupancy_cost") {
		t.Fatal("occupancy must not leak into the ifrs16 block")
	}
	if inBlock(pnl.Operating, "rou_depreciation") {
		t.Fatal("ROU depreciation must not leak into the operating block")
	}
	// Operating rows resolve from the semantic layer.
	for _, row := range pnl.Operating.Rows {
		if row.Key == "revenue" && (row.Actual == nil || *row.Actual != 1000) {
			t.Fatalf("revenue actual = %v, want 1000", row.Actual)
		}
	}
	// IFRS16 rows resolve from the lease port.
	for _, row := range pnl.Ifrs16.Rows {
		if row.Key == "rou_depreciation" && (row.Actual == nil || *row.Actual != 55) {
			t.Fatalf("rou_depreciation = %v, want 55 (lease port)", row.Actual)
		}
	}
}

func TestMissingNeverShowsAsZero(t *testing.T) {
	tmpl := testTemplate(t)
	facts := testFacts()
	facts.Marketing = nil // 营销 facts 缺列 → 该行缺失
	pnl, err := Project(context.Background(), tmpl, StoreRef{StoreID: "S1", AsOf: "2026-08-18", WindowDays: 7, Classification: "production"}, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: facts}, Plan: memPlan{values: map[string]*float64{}}, Lease: memLease{}, Peer: memPeer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range pnl.Operating.Rows {
		if row.Key == "marketing" && row.Actual != nil {
			t.Fatalf("missing marketing must stay nil, got %v", *row.Actual)
		}
	}
}

func TestVarianceAgainstSecondColumn(t *testing.T) {
	tmpl := testTemplate(t)
	pnl, err := Project(context.Background(), tmpl, StoreRef{StoreID: "S1", AsOf: "2026-08-18", WindowDays: 7, Classification: "production"}, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI:   memKPI{facts: testFacts()},
		Plan:  memPlan{values: map[string]*float64{"budget|fact.revenue": pf(900)}},
		Lease: memLease{}, Peer: memPeer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range pnl.Operating.Rows {
		if row.Key == "revenue" {
			if row.Other == nil || *row.Other != 900 {
				t.Fatalf("budget column = %v, want 900", row.Other)
			}
			if row.Variance == nil || *row.Variance != 100 || row.Pct == nil || *row.Pct-0.1111 > 0.001 {
				t.Fatalf("variance math wrong: var=%v pct=%v", row.Variance, row.Pct)
			}
		}
	}
}

func TestDecisionReadyDowngradeSurfacesReason(t *testing.T) {
	tmpl := testTemplate(t)
	facts := testFacts()
	facts.DecisionReady = false
	facts.DecisionReadyReason = "incomplete_store_day_coverage"
	pnl, err := Project(context.Background(), tmpl, StoreRef{StoreID: "S1", AsOf: "2026-08-18", WindowDays: 7, Classification: "production"}, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: facts}, Plan: memPlan{values: map[string]*float64{}}, Lease: memLease{}, Peer: memPeer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pnl.DecisionReady || pnl.DecisionReadyReason != "incomplete_store_day_coverage" {
		t.Fatalf("downgrade must carry its reason, got %v/%q", pnl.DecisionReady, pnl.DecisionReadyReason)
	}
	found := false
	for _, g := range pnl.Gaps {
		if len(g) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("downgrade reason must appear in gaps")
	}
}

func TestCustomFormulaRowsCarryUngovernedMarker(t *testing.T) {
	tmpl := testTemplate(t)
	ref := StoreRef{StoreID: "S1", AsOf: "2026-08-19", WindowDays: 7}
	// 空登记集 = fail-closed：出厂模板里的公式行必须带「未经指标治理」。
	pnl, err := Project(context.Background(), tmpl, ref, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: testFacts()}, Plan: memPlan{values: map[string]*float64{}}, Governed: map[string]bool{},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range pnl.Operating.Rows {
		if row.Kind == "formula" {
			found = true
			if !row.Ungoverned {
				t.Fatalf("formula row %q must be marked ungoverned with an empty registry", row.Key)
			}
		}
	}
	if !found {
		t.Fatal("default template has no formula row to assert on")
	}

	// 登记后同一条行不再带标识。
	governed := map[string]bool{}
	for _, row := range tmpl.Rows {
		if row.Kind == template.RowFormula {
			governed[row.Key] = true
		}
	}
	pnl2, err := Project(context.Background(), tmpl, ref, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: testFacts()}, Plan: memPlan{values: map[string]*float64{}}, Governed: governed,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range pnl2.Operating.Rows {
		if row.Kind == "formula" && row.Ungoverned {
			t.Fatalf("registered formula row %q must not carry the marker", row.Key)
		}
	}
}

// memPeerOK serves a complete peer probe.
type memPeerOK struct{ value float64 }

func (m memPeerOK) Median(_ context.Context, _ StoreRef, _ string) (*float64, string, bool) {
	v := m.value
	return &v, "complete", true
}

func TestPeerColumnRendersAndDegrades(t *testing.T) {
	tmpl := testTemplate(t)
	ref := StoreRef{StoreID: "S1", AsOf: "2026-08-19", WindowDays: 7}

	// 可用：同群中位数落到每行，头部状态 complete。
	pnl, err := Project(context.Background(), tmpl, ref, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: testFacts()}, Plan: memPlan{values: map[string]*float64{}}, Peer: memPeerOK{value: 900}, Governed: map[string]bool{},
	})
	if err != nil {
		t.Fatal(err)
	}
	peerRows := 0
	for _, row := range pnl.Operating.Rows {
		if row.Peer != nil {
			peerRows++
			if *row.Peer != 900 || row.PeerStatus != "complete" {
				t.Fatalf("peer row = %+v", row)
			}
		}
	}
	if peerRows == 0 || pnl.PeerStatus != "complete" {
		t.Fatalf("peer column must render: rows=%d status=%q", peerRows, pnl.PeerStatus)
	}

	// 样本不足：显式降级，不出数字。
	deg, err := Project(context.Background(), tmpl, ref, Period{From: "2026-08", To: "2026-08"}, [2]ColumnRef{ColActual, ColBudget}, BasisOperating, Readers{
		KPI: memKPI{facts: testFacts()}, Plan: memPlan{values: map[string]*float64{}}, Peer: memPeer{}, Governed: map[string]bool{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deg.PeerStatus != "insufficient_peers" {
		t.Fatalf("degraded peer status = %q, want insufficient_peers", deg.PeerStatus)
	}
	for _, row := range deg.Operating.Rows {
		if row.Peer != nil {
			t.Fatalf("degraded peer must not fabricate a number: %+v", row)
		}
	}
}
