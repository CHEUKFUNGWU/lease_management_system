package finmodel

import (
	"testing"
)

func monthsList(from string, count int) []string {
	out := []string{}
	year, month := 0, 0
	for i := 0; i < 4; i++ {
		year = year*10 + int(from[i]-'0')
	}
	for i := 5; i < 7; i++ {
		month = month*10 + int(from[i]-'0')
	}
	for i := 0; i < count; i++ {
		out = append(out, itoa(year)+"-"+pad2(month))
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return out
}

func pad2(value int) string {
	if value < 10 {
		return "0" + itoa(value)
	}
	return itoa(value)
}

func TestFoldBucketsQuarterAndPartial(t *testing.T) {
	quarters := FoldBuckets(monthsList("2026-01", 12), FoldQuarter)
	if len(quarters) != 4 {
		t.Fatalf("12 months fold into 4 quarters, got %d", len(quarters))
	}
	if quarters[0].Label != "2026-Q1" || len(quarters[0].Periods) != 3 {
		t.Fatalf("bucket 0 = %+v", quarters[0])
	}
	if quarters[3].Label != "2026-Q4" {
		t.Fatalf("bucket 3 label = %q", quarters[3].Label)
	}

	// 2 个月的部分季度：标签声明覆盖，绝不冒充完整季度。
	partial := FoldBuckets([]string{"2026-07", "2026-08"}, FoldQuarter)
	if len(partial) != 1 || partial[0].Label != "2026-Q3(2/3)" || len(partial[0].Periods) != 2 {
		t.Fatalf("partial quarter = %+v", partial)
	}

	// 跨年份的季度不混合：2025-12 与 2026-01 分属两个桶。
	split := FoldBuckets([]string{"2025-12", "2026-01", "2026-02"}, FoldQuarter)
	if len(split) != 2 || split[0].Label != "2025-Q4(1/3)" || split[1].Label != "2026-Q1(2/3)" {
		t.Fatalf("year boundary = %+v", split)
	}
}

func TestFoldBucketsYear(t *testing.T) {
	years := FoldBuckets(monthsList("2025-01", 24), FoldYear)
	if len(years) != 2 || years[0].Label != "2025" || years[1].Label != "2026" {
		t.Fatalf("two full years = %+v", years)
	}
	partial := FoldBuckets(monthsList("2026-01", 9), FoldYear)
	if len(partial) != 1 || partial[0].Label != "2026(9/12)" {
		t.Fatalf("current-year partial = %+v", partial)
	}
}

func TestFoldMonthValuesFlowSumsAndMissingStaysMissing(t *testing.T) {
	values := map[string]map[string]*float64{
		"rev":  {"2026-01": pf(10), "2026-02": pf(20), "2026-03": pf(30)},
		"cost": {"2026-01": pf(5), "2026-02": pf(6)}, // 2026-03 缺失 → 整桶缺失

	}
	folded := FoldMonthValues(values, FoldBuckets(monthsList("2026-01", 3), FoldQuarter))
	if got := folded["rev"]["2026-Q1"]; got == nil || *got != 60 {
		t.Fatalf("flow fold = %v, want 60", got)
	}
	if folded["cost"]["2026-Q1"] != nil {
		t.Fatalf("missing month must make the bucket missing (不填 0), got %v", *folded["cost"]["2026-Q1"])
	}
}

func TestFoldMonthValuesStockTakesLatest(t *testing.T) {
	values := map[string]map[string]*float64{
		"cash": {"2026-01": pf(100), "2026-02": pf(90), "2026-03": pf(110)},
		"ar":   {"2026-01": pf(40), "2026-03": pf(44)}, // 中间缺失：取最新非空
	}
	folded := FoldMonthValues(values, FoldBuckets(monthsList("2026-01", 3), FoldQuarter))
	if got := folded["cash"]["2026-Q1"]; got == nil || *got != 110 {
		t.Fatalf("stock fold must take the period end, got %v", got)
	}
	if got := folded["ar"]["2026-Q1"]; got == nil || *got != 44 {
		t.Fatalf("stock fold with a gap must take the latest non-nil month, got %v", got)
	}
	// 全空桶 → 缺失。
	empty := FoldMonthValues(map[string]map[string]*float64{"cash": {}}, FoldBuckets([]string{"2026-01", "2026-02", "2026-03"}, FoldQuarter))
	if empty["cash"]["2026-Q1"] != nil {
		t.Fatal("an all-empty bucket must stay missing")
	}

	// 未注册的自定义行按流量处理（文档化默认）。
	flowLike := FoldMonthValues(map[string]map[string]*float64{"custom": {"2026-01": pf(1), "2026-02": pf(2), "2026-03": pf(3)}}, FoldBuckets(monthsList("2026-01", 3), FoldQuarter))
	if got := flowLike["custom"]["2026-Q1"]; got == nil || *got != 6 {
		t.Fatalf("unregistered rows default to flow semantics: %v", got)
	}
	if !ValidFoldKind("quarter") || !ValidFoldKind("year") || !ValidFoldKind("month") || ValidFoldKind("week") || ValidFoldKind("") {
		t.Fatal("fold-kind validation broken")
	}
}
