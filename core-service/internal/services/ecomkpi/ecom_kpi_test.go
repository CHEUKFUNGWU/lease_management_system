package ecomkpi

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ecomfact"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func dayF(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func sfFact(currency string, gmv, discount, refund, chargeback, landed, fulfillment, fee float64, orders, newOrders int) ecomfact.StorefrontDayFact {
	return ecomfact.StorefrontDayFact{
		StorefrontRef: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: "SF-1"},
		BusinessDate:  dayF("2026-08-01"), Channel: "direct", SKU: "", Currency: currency,
		GMVAmount: ptrF(gmv), DiscountAmount: ptrF(discount), RefundAmount: ptrF(refund),
		ChargebackLoss: ptrF(chargeback), LandedCostAmount: ptrF(landed),
		FulfillmentAmount: ptrF(fulfillment), PaymentFeeAmount: ptrF(fee),
		OrderCount: ptrI(orders), NewCustomerOrders: ptrI(newOrders),
		SourceEnvelope: ecomfact.Envelope{SourceSystem: "shopify", FactVersion: 1, DataClassification: "production"},
	}
}

func paidAds(currency string, spend float64) []ecomfact.CampaignDayFact {
	return []ecomfact.CampaignDayFact{{
		StorefrontRef: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: "SF-1"},
		CampaignID:    "all", BusinessDate: dayF("2026-08-01"), Basis: ecomfact.AdBasisPaid,
		SpendAmount: spend, Currency: currency,
		SourceEnvelope: ecomfact.Envelope{SourceSystem: "ad_invoice", FactVersion: 1, DataClassification: "production"},
	}}
}

func win() ecomfact.Window {
	return ecomfact.Window{From: dayF("2026-08-01"), To: dayF("2026-08-07")}
}

func TestValidateSurfaceKnownCodesPass(t *testing.T) {
	if err := ValidateSurface(Surface); err != nil {
		t.Fatalf("真实 Surface 必须通过启动校验：%v", err)
	}
}

func TestValidateSurfaceUnknownCodeFails(t *testing.T) {
	bad := append([]SurfaceEntry{}, Surface...)
	bad = append(bad, SurfaceEntry{Code: "gmv_evil", Page: "site-pulse"})
	bad = append(bad, SurfaceEntry{Code: "mer_evil", Page: "site-360"})
	err := ValidateSurface(bad)
	if err == nil {
		t.Fatal("Surface 含未知 code 必须启动失败")
	}
	text := err.Error()
	if !contains(text, "gmv_evil") || !contains(text, "mer_evil") {
		t.Fatalf("未知 code 必须一次全报：%v", err)
	}
}

func TestEvaluateByCurrencyGolden(t *testing.T) {
	// 单日：GMV 10000 − 折扣 1000 = 9000；退款 500；拒付 100 → 净收入 8400
	// 成本 3000/500/300 → CM1 4600；广告实付 2000 → CM2 2600、MER 4.2、ROAS 4.2
	facts := []ecomfact.StorefrontDayFact{
		sfFact("USD", 10000, 1000, 500, 100, 3000, 500, 300, 200, 60),
	}
	codes := []string{"gmv", "net_revenue", "cm1", "cm1_rate", "cm2", "mer", "roas", "aov", "refund_rate", "cac_paid", "cac_blended", "ad_spend_paid", "tax_collected"}
	res, coverage := EvaluateByCurrency(codes, facts, paidAds("USD", 2000), win())
	if len(res) != 1 || res[0].Currency != "USD" {
		t.Fatalf("币种分区错误：%+v", res)
	}
	kpis := res[0].KPIs
	expect := map[string]float64{
		"gmv": 9000, "net_revenue": 8400, "cm1": 4600, "cm2": 2600,
		"mer": 4.2, "roas": 4.2, "aov": 42, "cac_paid": 33.33, "cac_blended": 10,
	}
	for code, want := range expect {
		v, ok := kpis[code]
		if !ok || v.Value == nil {
			t.Fatalf("%s 应可算：%+v", code, v)
		}
		if *v.Value != want {
			t.Fatalf("%s 期望 %.2f 实际 %.2f", code, want, *v.Value)
		}
	}
	// cm1_rate = 4600/8400 ≈ 0.547619 → 0.55
	if v := kpis["cm1_rate"]; v.Value == nil || *v.Value != 0.55 {
		t.Fatalf("cm1_rate 应 0.55：%+v", v)
	}
	// tax_collected 缺字段 → unavailable（不许补 0）
	if v := kpis["tax_collected"]; v.Status != StatusUnavailable || v.Reason != "missing_required_field" {
		t.Fatalf("tax 缺失必须 unavailable：%+v", v)
	}
	// CAC 分子分母标明
	cac := kpis["cac_paid"]
	if cac.Numerator != "ad_spend_paid" || cac.Denominator != "paying_new_customers" {
		t.Fatalf("CAC 分子分母必须标明：%+v", cac)
	}
	// 覆盖：窗口 7 天只有 1 天 → DecisionReady=false
	if !CoverageIncomplete(coverage) {
		t.Fatalf("1/7 天覆盖必须 incomplete：%+v", coverage)
	}
	if res[0].DecisionReady {
		t.Fatalf("覆盖不足必须 DecisionReady=false")
	}
}

func TestEvaluateByCurrencyStrictNull(t *testing.T) {
	// 缺退款字段 → 净收入 nil（never 0-fill）
	f := sfFact("USD", 9000, 0, 0, 0, 3000, 500, 300, 200, 60)
	f.RefundAmount = nil
	f.ChargebackLoss = nil
	res, _ := EvaluateByCurrency([]string{"net_revenue", "refund_rate"}, []ecomfact.StorefrontDayFact{f}, nil, win())
	v := res[0].KPIs["net_revenue"]
	if v.Value != nil || v.Status != StatusUnavailable {
		t.Fatalf("缺退款字段时净收入必须 nil + unavailable：%+v", v)
	}
}

func TestEvaluateByCurrencyZeroDenominator(t *testing.T) {
	f := sfFact("USD", 0, 0, 0, 0, 0, 0, 0, 0, 0) // GMV 0 → aov/mer 零分母
	res, _ := EvaluateByCurrency([]string{"aov", "mer"}, []ecomfact.StorefrontDayFact{f}, paidAds("USD", 0), win())
	if v := res[0].KPIs["aov"]; v.Status != StatusUnavailable || v.Reason != "zero_denominator" {
		t.Fatalf("零分母必须 explicit unavailable：%+v", v)
	}
}

func TestEvaluateByCurrencyPartitionNeverMixes(t *testing.T) {
	usd := sfFact("USD", 1000, 0, 0, 0, 300, 50, 30, 10, 2)
	jpy := sfFact("JPY", 100000, 0, 0, 0, 30000, 5000, 3000, 10, 2)
	jpy.BusinessDate = dayF("2026-08-02")
	jpy.SourceEnvelope.SourceSystem = "shopify"
	res, _ := EvaluateByCurrency([]string{"net_revenue"}, []ecomfact.StorefrontDayFact{usd, jpy}, nil, win())
	if len(res) != 2 {
		t.Fatalf("多币种必须分区（永不跨币种聚合）：%+v", res)
	}
	m := map[string]float64{}
	for _, p := range res {
		m[p.Currency] = *p.KPIs["net_revenue"].Value
	}
	if m["USD"] != 1000 || m["JPY"] != 100000 {
		t.Fatalf("分区金额错误：%+v", m)
	}
}

func TestEvaluateByCurrencyRestatementUsesHighestVersion(t *testing.T) {
	// v1 净收入 1000；v2（退款到达重述）→ 读取必须走 v2 语义：逐度量取最高版本
	v1 := sfFact("USD", 1000, 0, 0, 0, 300, 50, 30, 10, 2)
	v1.SourceEnvelope.FactVersion = 1
	v1.RefundAmount = nil // v1 没退款
	v2 := sfFact("USD", 1000, 0, 100, 0, 300, 50, 30, 10, 2)
	v2.SourceEnvelope.FactVersion = 2
	v2.SourceEnvelope.Restated = true
	res, _ := EvaluateByCurrency([]string{"net_revenue"}, []ecomfact.StorefrontDayFact{v1, v2}, nil, win())
	v := res[0].KPIs["net_revenue"]
	// 同业务键（同 date/channel/sku/source）v2 胜出 → 900，而不是 v1 的 1000 或混拌
	if v.Value == nil || *v.Value != 900 {
		t.Fatalf("Highest Fact Version 解析后净收入应 900：%+v", v)
	}
}

func TestDefinitionsHaveVersionsAndNames(t *testing.T) {
	defs := Definitions()
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Code == "" || d.MetricDefinitionVersion == "" {
			t.Fatalf("定义必须挂 code 与 Metric Definition Version：%+v", d)
		}
		if d.NameZH == "" {
			t.Fatalf("中文名唯一真相源：%s 缺 NameZH", d.Code)
		}
		if seen[d.Code] {
			t.Fatalf("重复定义 %s", d.Code)
		}
		seen[d.Code] = true
	}
	if _, ok := Label("net_revenue"); !ok {
		t.Fatal("Label 必须命中已登记指标")
	}
	if label, ok := Label("nonexistent"); ok || label != "" {
		t.Fatal("未登记指标 Label 必须报缺失")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
