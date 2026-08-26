package unitecon

import (
	"math"
	"testing"
)

// R-E3-3 三组用例：正常值、零值（cm1_rate=0）、负值（cm1_rate<0）。
// 负值组断言 unachievable 而非任何数字——「不可达成」是有效结论不是失败（D-E7）。

func TestBreakEvenNormalValues(t *testing.T) {
	// (固定 100000 + 目标 20000) ÷ 0.25 = 480000；ROAS = 1/0.25 = 4
	res := BreakEven(0.25, 100000, 20000)
	if res.Status != StatusAchieved {
		t.Fatalf("正常值应 achieved：%+v", res)
	}
	if res.BreakEvenMER == nil || *res.BreakEvenMER != 480000 {
		t.Fatalf("break-even MER 应为 480000：%+v", res.BreakEvenMER)
	}
	if res.BreakEvenROAS == nil || *res.BreakEvenROAS != 4 {
		t.Fatalf("break-even ROAS 应为 4：%+v", res.BreakEvenROAS)
	}
	if res.RequiredRevenue == nil || *res.RequiredRevenue != 480000 {
		t.Fatalf("required revenue 应等于 (固定+目标)/CM1率：%+v", res.RequiredRevenue)
	}
}

func TestBreakEvenZeroRateUnachievable(t *testing.T) {
	res := BreakEven(0, 100000, 0)
	if res.Status != StatusUnachievable {
		t.Fatalf("CM1 率为零必须 unachievable：%+v", res)
	}
	if res.Reason != ReasonUnachievableZero {
		t.Fatalf("原因应为 cm1_rate_is_zero：%q", res.Reason)
	}
	if res.BreakEvenMER != nil || res.BreakEvenROAS != nil {
		t.Fatalf("unachievable 不得携带任何数值：%+v", res)
	}
}

func TestBreakEvenNegativeRateUnachievable(t *testing.T) {
	res := BreakEven(-0.05, 100000, 0)
	if res.Status != StatusUnachievable {
		t.Fatalf("CM1 率为负必须 unachievable：%+v", res)
	}
	if res.Reason != ReasonUnachievableNegative {
		t.Fatalf("原因应为 cm1_rate_is_negative：%q", res.Reason)
	}
	if res.BreakEvenMER != nil || res.BreakEvenROAS != nil || res.RequiredRevenue != nil {
		t.Fatalf("unachievable 不得携带任何数值：%+v", res)
	}
}

func TestBreakEvenInvalidInputs(t *testing.T) {
	for _, v := range []float64{NaN(), Inf()} {
		res := BreakEven(v, 100000, 0)
		if res.Status != StatusUnachievable || res.Reason != "invalid_input" {
			t.Fatalf("NaN/Inf 输入必须 unachievable invalid_input：%+v", res)
		}
	}
}

func TestCACViewPaidAndBlended(t *testing.T) {
	spend := 60000.0
	newCustomers := 300
	orders := 1500
	report := CACView(CACInput{AdSpendPaid: &spend, PayingNewCustomers: &newCustomers, TotalOrders: &orders})
	if report.Paid.Value == nil || *report.Paid.Value != 200 {
		t.Fatalf("付费新客 CAC 应为 60000/300=200：%+v", report.Paid)
	}
	if report.Paid.Numerator != "ad_spend_paid" || report.Paid.Denominator != "paying_new_customers" {
		t.Fatalf("付费 CAC 分子分母必须标明：%+v", report.Paid)
	}
	if report.Blended.Value == nil || *report.Blended.Value != 40 {
		t.Fatalf("混合 CAC 应为 60000/1500=40：%+v", report.Blended)
	}
	if report.Blended.Denominator != "order_count" {
		t.Fatalf("混合 CAC 分母必须是 order_count：%+v", report.Blended)
	}
}

func TestCACViewZeroDenominatorUnavailable(t *testing.T) {
	spend := 60000.0
	zero := 0
	report := CACView(CACInput{AdSpendPaid: &spend, PayingNewCustomers: &zero})
	if report.Paid.Value != nil || report.Paid.Reason != "zero_denominator" {
		t.Fatalf("零分母 CAC 必须 unavailable 且说明原因：%+v", report.Paid)
	}
	if report.Paid.NumValue == nil {
		t.Fatalf("分子值要展示出来（CAC 分子分母在响应中标明）：%+v", report.Paid)
	}
}

func TestCACViewMissingInput(t *testing.T) {
	report := CACView(CACInput{})
	if report.Paid.Status != "unavailable" || report.Paid.Reason != "missing_required_field" {
		t.Fatalf("缺花费时 CAC 必须 unavailable：%+v", report.Paid)
	}
}

func NaN() float64 { return math.NaN() }
func Inf() float64 { return math.Inf(1) }
