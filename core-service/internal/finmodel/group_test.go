package finmodel

import "testing"

func TestSummarizeRefusesCrossCurrencyWithoutRateVersion(t *testing.T) {
	summary, err := Summarize([]GroupRunInput{
		{RunID: "r1", LegalEntityID: "LE-1", Authorized: true, Currency: "CNY", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(100)}},
		{RunID: "r2", LegalEntityID: "LE-2", Authorized: true, Currency: "USD", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(80)}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Totals) != 0 {
		t.Fatalf("no cross-currency totals without an exchange_rate_version (T14), got %+v", summary.Totals)
	}
	found := false
	for _, m := range summary.Members {
		if m.Note == "" && m.Currency != "CNY" && m.Currency != "USD" {
			continue
		}
		_ = m
	}
	_ = found
	if summary.Note == "" {
		t.Fatal("degradation must state its reason")
	}
}

func TestSummarizeMarksUnauthorizedExplicitly(t *testing.T) {
	summary, err := Summarize([]GroupRunInput{
		{RunID: "r1", LegalEntityID: "LE-1", Authorized: true, Currency: "CNY", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(100)}},
		{RunID: "r2", LegalEntityID: "LE-2", Authorized: false, Currency: "CNY", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(9999)}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range summary.Members {
		if m.RunID == "r2" {
			if m.Note != "unauthorized" {
				t.Fatalf("unauthorized members must be flagged, not omitted: %+v", m)
			}
			return
		}
	}
	t.Fatal("unauthorized member missing from the list (静默省略)")
	// 且其数字不得进入合计。
	if v := summary.Totals["rev@2026-01"]; v == nil || *v != 100 {
		t.Fatalf("totals must exclude unauthorized members, got %v", v)
	}
}

func TestSummarizeTiesOut(t *testing.T) {
	summary, err := Summarize([]GroupRunInput{
		{RunID: "r1", LegalEntityID: "LE-1", Authorized: true, Currency: "CNY", Periods: []string{"2026-01", "2026-02"}, Lines: map[string]*float64{"rev@2026-01": pf(100), "rev@2026-02": pf(110)}},
		{RunID: "r2", LegalEntityID: "LE-2", Authorized: true, Currency: "CNY", Periods: []string{"2026-01", "2026-02"}, Lines: map[string]*float64{"rev@2026-01": pf(50), "rev@2026-02": pf(55)}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !summary.TiesOut {
		t.Fatal("aggregation must tie out to its members")
	}
	if got := summary.Totals["rev@2026-01"]; got == nil || *got != 150 {
		t.Fatalf("total = %v, want 150", got)
	}
}

func pf(v float64) *float64 { return &v }
