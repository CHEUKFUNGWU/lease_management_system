package retailperiod

import (
	"testing"
	"time"
)

func date(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestParseDefaultsToRollingFourteen(t *testing.T) {
	window, err := Parse("", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if window.Period.Kind != KindRolling || window.Period.Days != DefaultRollingDays || DefaultRollingDays != 14 {
		t.Fatalf("period=%+v", window.Period)
	}
	if window.From.Format("2006-01-02") != "2026-08-03" || window.To.Format("2006-01-02") != "2026-08-16" {
		t.Fatalf("current window=%v..%v", window.From, window.To)
	}
	// Equal-length trailing comparison: the 14 days before the current 14.
	if window.CompareFrom.Format("2006-01-02") != "2026-07-20" || window.CompareTo.Format("2006-01-02") != "2026-08-02" {
		t.Fatalf("comparison window=%v..%v", window.CompareFrom, window.CompareTo)
	}
	if window.Label != "近 14 天" {
		t.Fatalf("label=%q", window.Label)
	}
}

func TestParseRollingAcceptsCustomRangeDays(t *testing.T) {
	// window_days=8 is a legal legacy URL value; the range stays open.
	window, err := Parse("8", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if window.Period.Days != 8 || window.From.Format("2006-01-02") != "2026-08-09" {
		t.Fatalf("window=%+v", window)
	}
	for _, invalid := range []string{"0", "366", "-1", "foo"} {
		if _, err := Parse(invalid, date("2026-08-16")); err == nil {
			t.Fatalf("rolling %q accepted", invalid)
		}
	}
}

func TestParseCalendarMonthComparesWithPreviousCalendarMonth(t *testing.T) {
	window, err := Parse("2026-07", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if window.From.Format("2006-01-02") != "2026-07-01" || window.To.Format("2006-01-02") != "2026-07-31" {
		t.Fatalf("july=%v..%v", window.From, window.To)
	}
	// Feb compares with the full previous calendar month despite the length
	// difference — retail review convention; coverage normalizes per window.
	if window.CompareFrom.Format("2006-01-02") != "2026-06-01" || window.CompareTo.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("comparison=%v..%v", window.CompareFrom, window.CompareTo)
	}
	if window.Label != "2026-07" {
		t.Fatalf("label=%q", window.Label)
	}
	february, err := Parse("2026-02", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if february.CompareFrom.Format("2006-01-02") != "2026-01-01" || february.CompareTo.Format("2006-01-02") != "2026-01-31" {
		t.Fatalf("february comparison=%v..%v", february.CompareFrom, february.CompareTo)
	}
}

func TestParseCalendarQuarter(t *testing.T) {
	window, err := Parse("2026-Q3", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if window.From.Format("2006-01-02") != "2026-07-01" || window.To.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("q3=%v..%v", window.From, window.To)
	}
	if window.CompareFrom.Format("2006-01-02") != "2026-04-01" || window.CompareTo.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("comparison=%v..%v", window.CompareFrom, window.CompareTo)
	}
	if window.Label != "2026-Q3" {
		t.Fatalf("label=%q", window.Label)
	}
	// Q1 compares with the previous year's Q4.
	q1, _ := Parse("2026-Q1", date("2026-08-16"))
	if q1.CompareFrom.Format("2006-01-02") != "2025-10-01" || q1.CompareTo.Format("2006-01-02") != "2025-12-31" {
		t.Fatalf("q1 comparison=%v..%v", q1.CompareFrom, q1.CompareTo)
	}
}

func TestParseNaturalLanguageShorthands(t *testing.T) {
	lastMonth, err := Parse("last-month", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	if lastMonth.Label != "2026-07" || lastMonth.From.Format("2006-01-02") != "2026-07-01" {
		t.Fatalf("last-month=%+v", lastMonth)
	}
	// January anchors to the previous year.
	december, err := Parse("last-month", date("2026-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	if december.Label != "2025-12" {
		t.Fatalf("december=%q", december.Label)
	}
	thisQuarter, err := Parse("this-quarter", date("2026-08-16"))
	if err != nil {
		t.Fatal(err)
	}
	// Quarter-to-date: truncated to asOf, equal-length comparison before Q3
	// (47 days: 07-01..08-16 → 05-15..06-30).
	if thisQuarter.To.Format("2006-01-02") != "2026-08-16" || thisQuarter.From.Format("2006-01-02") != "2026-07-01" {
		t.Fatalf("qtd=%v..%v", thisQuarter.From, thisQuarter.To)
	}
	if thisQuarter.CompareFrom.Format("2006-01-02") != "2026-05-15" || thisQuarter.CompareTo.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("qtd comparison=%v..%v", thisQuarter.CompareFrom, thisQuarter.CompareTo)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, invalid := range []string{"2026-13", "2026-Q5", "2026-7", "abc", "last_year", "-7"} {
		if _, err := Parse(invalid, date("2026-08-16")); err == nil {
			t.Fatalf("period %q accepted", invalid)
		}
	}
}

func TestNormalizeAndDefault(t *testing.T) {
	if _, err := Normalize(Period{Kind: KindRolling, Days: 14, Month: 7}); err == nil {
		t.Fatal("rolling with calendar fields accepted")
	}
	if _, err := Normalize(Period{Kind: KindCalendar, Year: 2026, Month: 7, Quarter: 3}); err == nil {
		t.Fatal("month+quarter accepted")
	}
	if _, err := Normalize(Default(KindRolling)); err != nil {
		t.Fatal(err)
	}
	if got := Default(KindRolling).Days; got != 14 {
		t.Fatalf("default rolling days=%d", got)
	}
	if _, err := ParseRollingDays(21); err != nil {
		t.Fatalf("custom 21-day window rejected: %v", err)
	}
	if _, err := ParseRollingDays(30); err != nil {
		t.Fatalf("custom 30-day window rejected: %v", err)
	}
	if _, err := ParseRollingDays(366); err == nil {
		t.Fatal("366-day rolling accepted")
	}
}

func TestParseCalendarWeekISO(t *testing.T) {
	// 2026-W01 的周一是 2025-12-29（ISO 周 1 含 1 月 4 日）。
	w, err := Parse("2026-W01", date("2026-02-10"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Period.Week != 1 || w.From.Format("2006-01-02") != "2025-12-29" || w.To.Format("2006-01-02") != "2026-01-04" {
		t.Fatalf("week window = %+v", w)
	}
	if w.CompareTo.Format("2006-01-02") != "2025-12-28" || w.CompareFrom.Format("2006-01-02") != "2025-12-22" {
		t.Fatalf("week comparison = %s..%s", w.CompareFrom.Format("2006-01-02"), w.CompareTo.Format("2006-01-02"))
	}
	if w.Label != "2026-W01" {
		t.Fatalf("label = %q", w.Label)
	}
	// 非法周号拒绝，无静默回退。
	if _, err := Parse("2026-W54", date("2026-02-10")); err == nil {
		t.Fatal("week 54 must be rejected")
	}
	if _, err := Parse("2025-W53", date("2026-02-10")); err != nil {
		t.Fatalf("2025 has an ISO week 53: %v", err)
	}
}

func TestParseCalendarYear(t *testing.T) {
	w, err := Parse("2026", date("2026-02-10"))
	if err != nil {
		t.Fatal(err)
	}
	if w.From.Format("2006-01-02") != "2026-01-01" || w.To.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("year window = %s..%s", w.From.Format("2006-01-02"), w.To.Format("2006-01-02"))
	}
	if w.CompareFrom.Format("2006-01-01") != "2025-01-01" || w.CompareTo.Format("2006-01-02") != "2025-12-31" {
		t.Fatalf("year comparison = %s..%s", w.CompareFrom.Format("2006-01-02"), w.CompareTo.Format("2006-01-02"))
	}
	if w.Period.Kind != KindCalendar || w.Period.Year != 2026 || w.Period.Month != 0 {
		t.Fatalf("period = %+v", w.Period)
	}
}

func TestNormalizeWeekExclusivity(t *testing.T) {
	if _, err := Normalize(Period{Kind: KindCalendar, Year: 2026, Month: 3, Week: 5}); err == nil {
		t.Fatal("month+week together must be rejected")
	}
	if _, err := Normalize(Period{Kind: KindCalendar, Year: 2026, Week: 32}); err != nil {
		t.Fatalf("year-week must normalize: %v", err)
	}
	if _, err := Normalize(Period{Kind: KindRolling, Days: 7, Week: 3}); err == nil {
		t.Fatal("rolling with week field must be rejected")
	}
}
