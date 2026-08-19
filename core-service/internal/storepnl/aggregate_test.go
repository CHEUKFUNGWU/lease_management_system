package storepnl

import (
	"strings"
	"testing"
)

func memberPnl(storeID, region, brand, entity, currency string, revenue, other float64, ready bool) *StorePnl {
	pnlRevenue := revenue
	pnlOther := other
	return &StorePnl{
		StoreID: storeID, Currency: currency,
		DecisionReady:       ready,
		DecisionReadyReason: map[bool]string{true: "", false: "覆盖不足"}[ready],
		BasisMode:           BasisSideBySide,
		Columns:             []ColumnRef{ColActual, ColBudget},
		Operating: &Block{Basis: "operating_basis", Rows: []RowValue{
			{Key: "revenue", Label: "营业收入", Kind: "link", Basis: "shared", Actual: &pnlRevenue, Other: &pnlOther},
			{Key: "labor_cost", Label: "人工", Kind: "link", Basis: "shared", Actual: pf(10), Other: pf(8)},
			{Key: "occupancy_cost", Label: "占用成本", Kind: "subtotal", Basis: "operating_basis",
				Children: []string{"fixed_rent"}, Components: []Component{{Label: "固定租金", Value: pf(5)}}},
		}},
	}
}

func TestAggregateGroupsByRegionAndSums(t *testing.T) {
	result, err := Aggregate(GroupByRegion, Period{From: "2026-08-06", To: "2026-08-19"}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Region: "华东", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
		{StoreID: "S2", LegalEntityID: "LE-1", Region: "华东", Pnl: memberPnl("S2", "", "", "", "CNY", 120, 90, true)},
		{StoreID: "S3", LegalEntityID: "LE-1", Region: "华南", Pnl: memberPnl("S3", "", "", "", "CNY", 50, 40, true)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %d, want 2 (华东/华南)", len(result.Groups))
	}
	east := result.Groups[0]
	if east.Key != "华东" || east.StoreCount != 2 || east.MixedCurrency {
		t.Fatalf("east group = %+v", east)
	}
	if len(east.Partitions) != 1 || east.Partitions[0].Currency != "CNY" {
		t.Fatalf("single-currency group must have one partition: %+v", east.Partitions)
	}
	revenue := findRow(east.Partitions[0].Operating, "revenue")
	if revenue.Actual == nil || *revenue.Actual != 220 || revenue.Other == nil || *revenue.Other != 170 {
		t.Fatalf("aggregated revenue = %+v, want actual 220 other 170", revenue)
	}
	// 差异由聚合后的两列重算；比率以聚合基数为分母。
	if revenue.Variance == nil || *revenue.Variance != 50 || revenue.Pct == nil || *revenue.Pct != 50.0/170.0 {
		t.Fatalf("aggregated variance/pct = %v/%v", revenue.Variance, revenue.Pct)
	}
	// 构成下钻同样聚合。
	occ := findRow(east.Partitions[0].Operating, "occupancy_cost")
	if len(occ.Components) != 1 || occ.Components[0].Value == nil || *occ.Components[0].Value != 10 {
		t.Fatalf("aggregated components = %+v", occ.Components)
	}
	if east.Partitions[0].Operating.Rows[0].Peer != nil {
		t.Fatal("aggregate mode must not fabricate a peer median")
	}
}

func TestAggregateCurrencyPartitionNeverSums(t *testing.T) {
	result, err := Aggregate(GroupByBrand, Period{}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Brand: "B1", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
		{StoreID: "S2", LegalEntityID: "LE-1", Brand: "B1", Pnl: memberPnl("S2", "", "", "", "USD", 60, 50, true)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	group := result.Groups[0]
	if !group.MixedCurrency || len(group.Partitions) != 2 {
		t.Fatalf("mixed-currency group must partition, got %+v", group.Partitions)
	}
	if group.Partitions[0].Currency == group.Partitions[1].Currency {
		t.Fatal("partitions must be per currency")
	}
	if !strings.Contains(group.Note, "Currency Partition") || !strings.Contains(result.Note, "跨币种合计") {
		t.Fatalf("T14 note missing: group=%q result=%q", group.Note, result.Note)
	}
	// 每个分区独立成表，任何位置不存在跨币种合计（结构上根本没有 Totals）。
	for _, partition := range group.Partitions {
		if partition.Operating == nil {
			t.Fatalf("partition %q must carry its own operating block", partition.Currency)
		}
	}
}

func TestAggregateMissingMemberValueDegradesRow(t *testing.T) {
	withNil := memberPnl("S2", "", "", "", "CNY", 120, 90, true)
	withNil.Operating.Rows[0].Actual = nil // S2 收入缺失
	result, err := Aggregate(GroupByRegion, Period{}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Region: "华东", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
		{StoreID: "S2", LegalEntityID: "LE-1", Region: "华东", Pnl: withNil},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	partition := result.Groups[0].Partitions[0]
	revenue := findRow(partition.Operating, "revenue")
	if revenue.Actual != nil {
		t.Fatalf("partial sum must stay missing (不填 0), got %v", *revenue.Actual)
	}
	found := false
	for _, gap := range partition.Gaps {
		if strings.Contains(gap, "S2") && strings.Contains(gap, "revenue") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing member must be named in gaps: %+v", partition.Gaps)
	}
}

func TestAggregateDecisionReadyPropagates(t *testing.T) {
	result, err := Aggregate(GroupByRegion, Period{}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Region: "华东", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
		{StoreID: "S2", LegalEntityID: "LE-1", Region: "华东", Pnl: memberPnl("S2", "", "", "", "CNY", 120, 90, false)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	partition := result.Groups[0].Partitions[0]
	if partition.DecisionReady {
		t.Fatal("one unready member must degrade the partition")
	}
	if !strings.Contains(partition.DecisionReadyReason+" "+strings.Join(partition.Gaps, " "), "覆盖不足") {
		t.Fatalf("degradation reason must be preserved: %+v", partition)
	}
}

func TestAggregateRejectsUnknownGroupBy(t *testing.T) {
	if _, err := Aggregate(GroupBy("city"), Period{}, [2]ColumnRef{ColActual, ColBudget}, nil, nil); err == nil {
		t.Fatal("unknown dimension must be rejected")
	}
}

func TestAggregateGroupsByEntityAndUnsetDimension(t *testing.T) {
	result, err := Aggregate(GroupByLegalEntity, Period{}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
		{StoreID: "S2", LegalEntityID: "LE-2", Pnl: memberPnl("S2", "", "", "", "CNY", 120, 90, true)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 2 || result.Groups[0].Key == result.Groups[1].Key {
		t.Fatalf("entity grouping must split members into their entities: %+v", result.Groups)
	}
	// 维度值缺失的门店归入空键组，绝不静默省略。
	result, err = Aggregate(GroupByBrand, Period{}, [2]ColumnRef{ColActual, ColBudget}, []AggregateMember{
		{StoreID: "S1", LegalEntityID: "LE-1", Brand: "", Pnl: memberPnl("S1", "", "", "", "CNY", 100, 80, true)},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 1 || result.Groups[0].Key != "" || result.Groups[0].StoreCount != 1 {
		t.Fatalf("unset dimension value must group under the empty key: %+v", result.Groups)
	}
}
