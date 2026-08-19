package finmodel

import (
	"strings"
	"testing"
)

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
	if !strings.Contains(summary.Note, "exchange_rate_version") {
		t.Fatalf("degradation must state its reason, note=%q", summary.Note)
	}
	// 默认视图 = 原币分区：两个成员各在其币种分区下，合计不存在。
	if got := summary.CurrencyPartitions["CNY"]; len(got) != 1 || got[0] != "r1" {
		t.Fatalf("CNY partition = %v", got)
	}
	if got := summary.CurrencyPartitions["USD"]; len(got) != 1 || got[0] != "r2" {
		t.Fatalf("USD partition = %v", got)
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
	flagged := false
	for _, m := range summary.Members {
		if m.RunID == "r2" {
			if m.Note != "unauthorized" {
				t.Fatalf("unauthorized members must be flagged, not omitted: %+v", m)
			}
			flagged = true
		}
	}
	if !flagged {
		t.Fatal("unauthorized member missing from the list (静默省略)")
	}
	// 且其数字不得进入合计。
	if v := summary.Totals["rev@2026-01"]; v == nil || *v != 100 {
		t.Fatalf("totals must exclude unauthorized members, got %v", v)
	}
}

func TestSummarizeTiesOutSingleCurrency(t *testing.T) {
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
	if !strings.Contains(summary.Note, "未抵销") {
		t.Fatalf("group note must carry the no-intercompany-offset statement, got %q", summary.Note)
	}
}

func TestSummarizeTranslatedSecondView(t *testing.T) {
	// 两币种成员，汇率版本 7.0 CNY/USD，目标币人民币。
	trans := map[string]*float64{"rev@2026-01": pf(560)}
	summary, err := Summarize([]GroupRunInput{
		{RunID: "r1", LegalEntityID: "LE-1", Authorized: true, Currency: "CNY", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(100)}, ExchangeRateVersion: "fx-2026Q2", TranslatedCurrency: "CNY", TranslatedLines: map[string]*float64{"rev@2026-01": pf(100)}},
		{RunID: "r2", LegalEntityID: "LE-2", Authorized: true, Currency: "USD", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(80)}, ExchangeRateVersion: "fx-2026Q2", TranslatedCurrency: "CNY", TranslatedLines: trans},
	}, "fx-2026Q2")
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalsCurrency != "CNY" {
		t.Fatalf("totals currency = %q, want CNY", summary.TotalsCurrency)
	}
	if got := summary.Totals["rev@2026-01"]; got == nil || *got != 660 {
		t.Fatalf("translated total = %v, want 660", got)
	}
	if !summary.TiesOut {
		t.Fatal("translated totals must tie out to member contributions")
	}
	if !strings.Contains(summary.Note, "未抵销") || !strings.Contains(summary.Note, "exchange_rate_version=fx-2026Q2") {
		t.Fatalf("translated view must carry the offset statement and the version banner, got %q", summary.Note)
	}
}

func TestSummarizeTranslatedViewDegradesWhenMemberMissing(t *testing.T) {
	summary, err := Summarize([]GroupRunInput{
		{RunID: "r1", LegalEntityID: "LE-1", Authorized: true, Currency: "CNY", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(100)}, ExchangeRateVersion: "fx-2026Q2", TranslatedCurrency: "CNY", TranslatedLines: map[string]*float64{"rev@2026-01": pf(100)}},
		// 该成员缺 USD→CNY 汇率：不得混用原币值凑合计。
		{RunID: "r2", LegalEntityID: "LE-2", Authorized: true, Currency: "USD", Periods: []string{"2026-01"}, Lines: map[string]*float64{"rev@2026-01": pf(80)}, ExchangeRateVersion: "fx-2026Q2", Note: "missing_exchange_rate"},
	}, "fx-2026Q2")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Totals) != 0 {
		t.Fatalf("a member without translated values must degrade the whole translated view, got %+v", summary.Totals)
	}
	if summary.TiesOut {
		t.Fatal("an unconstructible translated view must not claim ties_out")
	}
	if !strings.Contains(summary.Note, "r2") {
		t.Fatalf("degradation must name the member, got %q", summary.Note)
	}
}

func pf(v float64) *float64 { return &v }

func TestSummarizeTiesOutFailsOnRoundingDrift(t *testing.T) {
	// 30 个成员各 0.014999：成员显示值四舍五入后合计 0.30，总计四舍五入
	// 0.45 —— 展示口径差额 0.15 > 容差 0.05，勾稽必须失败（反向测试：断言
	// 不是恒真）。
	members := make([]GroupRunInput, 0, 30)
	for i := 0; i < 30; i++ {
		members = append(members, GroupRunInput{
			RunID: "r" + string(rune('a'+i)), LegalEntityID: "LE", Authorized: true,
			Currency: "CNY", Periods: []string{"2026-01"},
			Lines: map[string]*float64{"rev@2026-01": pf(0.014999)},
		})
	}
	summary, err := Summarize(members, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.TiesOut {
		t.Fatalf("rounding drift of %v must fail ties_out; totals=%v", 0.15, summary.Totals)
	}
	if summary.Totals["rev@2026-01"] == nil {
		t.Fatal("total present but ties_out claimed by construction")
	}
}
