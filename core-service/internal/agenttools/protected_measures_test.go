package agenttools

import "testing"

func TestProtectedMeasureIDsFinalized(t *testing.T) {
	ids := ProtectedMeasureIDs()
	if len(ids) != 10 {
		t.Fatalf("protected measure list must stay at 10 (ADR-0025 §2), got %d: %v", len(ids), ids)
	}
	for _, want := range []string{
		"lease_liability", "rou_asset", "discount_rate_applied",
		"interest_expense", "rou_depreciation", "amortization_schedule_row",
		"journal_amount", "disclosure_maturity_bucket",
		"weighted_average_discount_rate", "remeasurement_adjustment",
	} {
		if !IsProtected(want) {
			t.Fatalf("measure %s must be protected", want)
		}
	}
	if IsProtected("revenue") {
		t.Fatal("ordinary operating measure must not be protected")
	}
}

func TestMatchLexicalProbe(t *testing.T) {
	cases := []struct {
		label string
		want  string
		hit   bool
	}{
		{"租赁负债余额", "lease_liability", true},
		{"Lease Liability at period end", "lease_liability", true},
		{"期末使用权资产", "rou_asset", true},
		{"ＲＯＵ 资产净值", "rou_asset", true}, // full-width letters
		{"Discount Rate applied", "discount_rate_applied", true},
		{"加权平均折现率", "weighted_average_discount_rate", true},
		{"本期利息费用", "interest_expense", true},
		{"摊销表第 12 行", "amortization_schedule_row", true},
		{"Journal Entry amount", "journal_amount", true},
		{"重计量调整", "remeasurement_adjustment", true},
		{"门店销售额", "", false},
		{"客流量", "", false},
		{"Revenue", "", false},
	}
	for _, c := range cases {
		id, hit := MatchLexicalProbe(c.label)
		if hit != c.hit || (hit && id != c.want) {
			t.Fatalf("MatchLexicalProbe(%q) = (%q, %v), want (%q, %v)", c.label, id, hit, c.want, c.hit)
		}
	}
}

func TestRouteMeasures(t *testing.T) {
	// Ordinary measures never reject.
	if d := RouteMeasures([]string{"revenue", "traffic"}, false); d.Tier != "A" {
		t.Fatalf("ordinary measures must route to A, got %s", d.Tier)
	}
	// Protected + certified satisfiable -> A.
	d := RouteMeasures([]string{"lease_liability"}, true)
	if d.Tier != "A" || len(d.Protected) != 1 {
		t.Fatalf("protected with certified path must route to A, got %+v", d)
	}
	// Protected + no certified path -> reject, never B.
	d = RouteMeasures([]string{"lease_liability"}, false)
	if d.Tier != "Reject" {
		t.Fatalf("protected without certified path must reject, got %s", d.Tier)
	}
	if d.RejectReason == "" {
		t.Fatal("rejection must carry a helpful reason")
	}
}

func TestLintCell(t *testing.T) {
	// Protected measure with Exploratory basis -> violation.
	if v := LintCell("lease_liability", "期末租赁负债", "Exploratory"); len(v) != 1 || v[0] != "protected_measure_exploratory" {
		t.Fatalf("protected exploratory must violate, got %v", v)
	}
	// Protected measure with Certified basis -> clean.
	if v := LintCell("lease_liability", "期末租赁负债", "Certified"); len(v) != 0 {
		t.Fatalf("protected certified must pass, got %v", v)
	}
	// No measure_id but label hits the probe + Exploratory -> suspected bypass.
	if v := LintCell("", "使用权资产余额", "Exploratory"); len(v) != 1 || v[0] != "lexical_probe_exploratory" {
		t.Fatalf("lexical probe must catch unlabelled exploratory cells, got %v", v)
	}
	// Unlabelled but harmless label -> clean.
	if v := LintCell("", "门店销售额", "Exploratory"); len(v) != 0 {
		t.Fatalf("ordinary label must pass, got %v", v)
	}
}
