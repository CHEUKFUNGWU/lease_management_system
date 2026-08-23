package retailkpi

import (
	"strings"
	"testing"
	"time"
)

// RH1（R1-1）：Surface / Label / ValidateSurface 三符号的单元测试。
// 自检句：把 ValidateSurface 改成恒返回 nil，TestValidateSurfaceRejectsUndefinedCodes 必红；
// 把 Label 改成恒返回 ("", false) 或恒 true，对应断言必红。

func TestLabelReturnsChineseNameForDefinedCode(t *testing.T) {
	name, ok := Label("sales_per_labor_hour")
	if !ok || name != "销售人效" {
		t.Fatalf("Label(sales_per_labor_hour) = (%q, %v), want (销售人效, true)", name, ok)
	}
}

func TestLabelReportsUnrecognizedCode(t *testing.T) {
	name, ok := Label("no_such_metric_code")
	if ok {
		t.Fatalf("Label on undefined code must report ok=false, got (%q, true)", name)
	}
	if name != "" {
		t.Fatalf("Label on undefined code must not fabricate a name, got %q", name)
	}
}

func TestValidateSurfaceAcceptsClosedList(t *testing.T) {
	if err := ValidateSurface(Surface{Codes: []string{"revenue", "sales_per_labor_hour", "headcount"}}); err != nil {
		t.Fatalf("valid surface must pass, got %v", err)
	}
	if err := ValidateSurface(Surface{Codes: nil}); err != nil {
		t.Fatalf("empty surface must pass, got %v", err)
	}
}

func TestValidateSurfaceRejectsUndefinedCodes(t *testing.T) {
	err := ValidateSurface(Surface{Codes: []string{"revenue", "not_a_metric", "also_fake"}})
	if err == nil {
		t.Fatal("surface with undefined codes must fail validation")
	}
	for _, code := range []string{"not_a_metric", "also_fake"} {
		if !strings.Contains(err.Error(), code) {
			t.Fatalf("error must list every undefined code, missing %q in: %v", code, err)
		}
	}
	if !strings.Contains(err.Error(), "revenue") == false {
		t.Fatalf("error must not flag defined codes: %v", err)
	}
}

// 工单 R1-1 验收：labor_hours 为 nil / 0 / 正数三种输入，
// sales_per_labor_hour 分别为 Partial(nil) / Unavailable(nil, zero_denominator) / Complete(正确值)。
// 不存在从 labor_cost 反推 labor_hours 的路径——反推会造出类型上是事实、
// 语义上是猜测的值，绕过整套覆盖率门槛与 decision_ready 判定。
func TestLaborHoursNilZeroPositive(t *testing.T) {
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	req := Request{DateFrom: base, DateTo: base, RequestedDateFrom: "2026-02-01", RequestedDateTo: "2026-02-01", GroupBy: "total", ExpectedStoreCount: 1}
	mk := func(hours *float64) []DailyFact {
		return []DailyFact{{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(1000), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(100), MappingStatus: "mapped", LaborHours: hours}}
	}

	// nil：缺失 → Partial + missing_required_field，值为 nil
	rows, _, err := AggregateFacts(mk(nil), req)
	if err != nil {
		t.Fatal(err)
	}
	got := rows[0].KPIs["sales_per_labor_hour"]
	if got.Value != nil || got.Status != StatusPartial || got.Reason != "missing_required_field" {
		t.Fatalf("labor_hours=nil => Partial(nil, missing_required_field), got %+v", got)
	}

	// 0：零分母 → Unavailable + zero_denominator，值为 nil；不许出现 0 或估算值
	rows, _, err = AggregateFacts(mk(ptr(0)), req)
	if err != nil {
		t.Fatal(err)
	}
	got = rows[0].KPIs["sales_per_labor_hour"]
	if got.Value != nil || got.Status != StatusUnavailable || got.Reason != "zero_denominator" {
		t.Fatalf("labor_hours=0 => Unavailable(nil, zero_denominator), got %+v", got)
	}

	// 正数：1000 / 25 = 40
	rows, _, err = AggregateFacts(mk(ptr(25)), req)
	if err != nil {
		t.Fatal(err)
	}
	got = rows[0].KPIs["sales_per_labor_hour"]
	if got.Value == nil || *got.Value != 40 || got.Status != StatusComplete {
		t.Fatalf("labor_hours=25 => Complete(40), got %+v", got)
	}
}

// headcount 只做登记暴露（values 已在聚合循环里求和），零新计算：
// 有事实给和值、缺事实 Partial(nil)，与其它 SUM 型指标同语义。
func TestHeadcountRegistrationOnly(t *testing.T) {
	def := findDefinition("headcount")
	if def == nil {
		t.Fatal("headcount must be registered as a Definition")
	}
	if def.Formula != "SUM(headcount)" {
		t.Fatalf("headcount is a registration of the existing sum, formula must stay SUM(headcount), got %q", def.Formula)
	}
	base := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	req := Request{DateFrom: base, DateTo: base, RequestedDateFrom: "2026-02-01", RequestedDateTo: "2026-02-01", GroupBy: "total", ExpectedStoreCount: 1}
	facts := []DailyFact{{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(100), Transactions: ptr(5), Footfall: ptr(10), AreaSqm: ptr(10), MappingStatus: "mapped", Headcount: ptr(4)}}
	rows, _, err := AggregateFacts(facts, req)
	if err != nil {
		t.Fatal(err)
	}
	got := rows[0].KPIs["headcount"]
	if got.Value == nil || *got.Value != 4 || got.Status != StatusComplete {
		t.Fatalf("headcount=4 => Complete(4), got %+v", got)
	}
	missingRows, _, _ := AggregateFacts([]DailyFact{{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(100), Transactions: ptr(5), Footfall: ptr(10), AreaSqm: ptr(10), MappingStatus: "mapped"}}, req)
	got = missingRows[0].KPIs["headcount"]
	if got.Value != nil || got.Status != StatusPartial {
		t.Fatalf("headcount missing => Partial(nil), got %+v", got)
	}
}
