package sitepnl

import (
	"context"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ecomfact"
)

// golden 输入：固定构造的站点日事实。期望值逐格锁定。

func ptrF(v float64) *float64   { return &v }
func ptrI(v int) *int           { return &v }

func day(month string, d int) time.Time {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		panic(err)
	}
	return t.AddDate(0, 0, d-1)
}

func fact(storefrontID, date string, gmv, discount, refund, chargeback, landed, fulfillment, fee float64) ecomfact.StorefrontDayFact {
	return ecomfact.StorefrontDayFact{
		StorefrontRef: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		BusinessDate:  mustDate(date), Channel: "direct", SKU: "", Currency: "USD",
		GMVAmount: ptrF(gmv), DiscountAmount: ptrF(discount), RefundAmount: ptrF(refund),
		ChargebackLoss: ptrF(chargeback), LandedCostAmount: ptrF(landed),
		FulfillmentAmount: ptrF(fulfillment), PaymentFeeAmount: ptrF(fee),
		SourceEnvelope: ecomfact.Envelope{SourceSystem: "shopify", FactVersion: 1},
	}
}

func mustDate(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

type fixedFactsReader struct {
	storefrontDays map[string][]ecomfact.StorefrontDayFact
	campaignDays   map[string][]ecomfact.CampaignDayFact
}

func (f *fixedFactsReader) StorefrontDays(_ context.Context, filter ecomfact.StorefrontFilter, w ecomfact.Window) ([]ecomfact.StorefrontDayFact, error) {
	out := []ecomfact.StorefrontDayFact{}
	for _, facts := range f.storefrontDays {
		for _, fact := range facts {
			if w.Contains(fact.BusinessDate) && len(filter.StorefrontIDs) == 0 || contains(filter.StorefrontIDs, fact.StorefrontID) {
				if len(filter.StorefrontIDs) == 0 || contains(filter.StorefrontIDs, fact.StorefrontID) {
					out = append(out, fact)
				}
			}
		}
	}
	return out, nil
}

func (f *fixedFactsReader) CampaignDays(_ context.Context, filter ecomfact.StorefrontFilter, w ecomfact.Window, basis ecomfact.AdBasis) ([]ecomfact.CampaignDayFact, error) {
	out := []ecomfact.CampaignDayFact{}
	for _, facts := range f.campaignDays {
		for _, fact := range facts {
			if fact.Basis != basis {
				continue
			}
			if w.Contains(fact.BusinessDate) && (len(filter.StorefrontIDs) == 0 || contains(filter.StorefrontIDs, fact.StorefrontID)) {
				out = append(out, fact)
			}
		}
	}
	return out, nil
}

func (f *fixedFactsReader) OrderLines(context.Context, ecomfact.EvidenceRef) ([]ecomfact.OrderLine, error) { return nil, nil }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

type glReader struct{ amount *float64 }

func (g glReader) GLRevenue(context.Context, ecomfact.StorefrontRef, Period) (*GLRevenue, error) {
	if g.amount == nil {
		return nil, nil
	}
	return &GLRevenue{Amount: g.amount, Currency: "USD", SourceSystem: "gl", FactVersion: 1}, nil
}

type fixedReader struct{ amount *float64 }

func (f fixedReader) FixedCost(context.Context, ecomfact.StorefrontRef, Period) (*FixedCost, error) {
	if f.amount == nil {
		return nil, nil
	}
	return &FixedCost{Amount: f.amount, Currency: "USD", SourceSystem: "overhead"}, nil
}

func findRow(t *testing.T, block *CurrencyBlock, key string) *Row {
	t.Helper()
	for i := range block.Rows {
		if block.Rows[i].Key == key {
			return &block.Rows[i]
		}
	}
	t.Fatalf("行 %q 不存在：%+v", key, block.Rows)
	return nil
}

func TestProjectStatementGolden(t *testing.T) {
	storefrontID := "SF-US"
	// 单日：GMV 10000 − 折扣 1000 = 9000；退款 500；拒付 100 → 净收入 8400
	// 落地 3000；履约 500；支付费 300 → CM1 = 4600；广告实付 2000 → CM2 = 2600；固定费 1500 → 经营利润 1100
	facts := []ecomfact.StorefrontDayFact{
		fact(storefrontID, "2026-08-01", 10000, 1000, 500, 100, 3000, 500, 300),
	}
	ads := []ecomfact.CampaignDayFact{{
		StorefrontRef: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		CampaignID: "all", BusinessDate: mustDate("2026-08-01"), Basis: ecomfact.AdBasisPaid,
		SpendAmount: 2000, Currency: "USD",
		SourceEnvelope: ecomfact.Envelope{SourceSystem: "ad_invoice", FactVersion: 1},
	}}
	reader := &fixedFactsReader{
		storefrontDays: map[string][]ecomfact.StorefrontDayFact{storefrontID: facts},
		campaignDays:   map[string][]ecomfact.CampaignDayFact{storefrontID: ads},
	}
	stmt, err := Project(context.Background(), SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		Currency:   "USD",
		Period:     Period{Kind: PeriodMonthly, Month: "2026-08"},
		Breakdown:  BreakdownNone,
	}, Readers{Facts: reader, GL: glReader{amount: ptrF(8600)}, Fixed: fixedReader{amount: ptrF(1500)}})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if len(stmt.Blocks) != 1 {
		t.Fatalf("应有 1 个币种块：%+v", stmt.Blocks)
	}
	block := &stmt.Blocks[0]

	checks := map[string]float64{
		RowGMV:             9000, // 商品金额 10000 − 折扣 1000
		RowNetRevenue:      8400, // 9000 − 退款 500 − 拒付 100
		RowLandedCost:      3000,
		RowFulfillment:     500,
		RowPaymentFee:      300,
		RowCM1:             4600,
		RowAdSpendPaid:     2000,
		RowCM2:             2600,
		RowFixedCost:       1500,
		RowOperatingProfit: 1100,
	}
	for key, want := range checks {
		row := findRow(t, block, key)
		if row.Value == nil || *row.Value != want {
			t.Fatalf("%s 期望 %.2f，实际 %v", key, want, row.Value)
		}
	}
	// 子合计声明推导来源（构造性质：子合计由子行推导）
	if row := findRow(t, block, RowCM1); len(row.Children) != 4 {
		t.Fatalf("CM1 子合计必须声明 4 个来源行：%+v", row.Children)
	}
	if row := findRow(t, block, RowOperatingProfit); len(row.Children) != 2 {
		t.Fatalf("经营利润子合计必须声明 2 个来源行：%+v", row.Children)
	}
	// 会计口径块只读来自 GL，basis=gl
	if block.Accounting.Basis != BasisGL || block.Accounting.Revenue == nil || *block.Accounting.Revenue != 8600 {
		t.Fatalf("GL 会计块错误：%+v", block.Accounting)
	}
	if block.Accounting.SourceSystem != "gl" {
		t.Fatalf("会计收入来源必须是 GL 导入：%+v", block.Accounting)
	}
	// 保本：CM1 率 = 4600/8400 ≈ 0.5476；MER* = (1500+0)/0.5476 ≈ 2739.13；ROAS* ≈ 1.8261
	if block.BreakEven.Status != "achieved" {
		t.Fatalf("保本应可计算：%+v", block.BreakEven)
	}
	if block.BreakEven.BreakEvenROAS == nil || *block.BreakEven.BreakEvenROAS < 1.82 || *block.BreakEven.BreakEvenROAS > 1.83 {
		t.Fatalf("保本 ROAS ≈ 1.8261：%v", block.BreakEven.BreakEvenROAS)
	}
}

func TestProjectMissingRowsStayNil(t *testing.T) {
	storefrontID := "SF-US"
	// 缺落地成本/履约/支付费 → CM1 及下游全部 nil，绝不补 0
	facts := []ecomfact.StorefrontDayFact{
		fact(storefrontID, "2026-08-01", 10000, 1000, 500, 100, 0, 0, 0),
	}
	// 清掉成本字段（fact() 给 0 会算成 0；这里改为 nil）
	facts[0].LandedCostAmount = nil
	facts[0].FulfillmentAmount = nil
	facts[0].PaymentFeeAmount = nil

	stmt, err := Project(context.Background(), SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		Currency:   "USD",
		Period:     Period{Kind: PeriodMonthly, Month: "2026-08"},
	}, Readers{Facts: &fixedFactsReader{storefrontDays: map[string][]ecomfact.StorefrontDayFact{storefrontID: facts}},
		GL: glReader{amount: ptrF(8600)}, Fixed: fixedReader{amount: ptrF(1500)}})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	block := &stmt.Blocks[0]
	if row := findRow(t, block, RowCM1); row.Value != nil {
		t.Fatalf("缺成本时 CM1 必须 nil：%v", row.Value)
	}
	if row := findRow(t, block, RowOperatingProfit); row.Value != nil {
		t.Fatalf("缺成本时经营利润必须 nil：%v", row.Value)
	}
	if row := findRow(t, block, RowNetRevenue); row.Value == nil || *row.Value != 8400 {
		t.Fatalf("净收入不依赖成本字段，应照常 8400：%v", row.Value)
	}
}

func TestProjectGLUnavailableDegradesNotFails(t *testing.T) {
	storefrontID := "SF-US"
	facts := []ecomfact.StorefrontDayFact{fact(storefrontID, "2026-08-01", 10000, 1000, 500, 100, 3000, 500, 300)}
	stmt, err := Project(context.Background(), SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		Period:     Period{Kind: PeriodMonthly, Month: "2026-08"},
	}, Readers{Facts: &fixedFactsReader{storefrontDays: map[string][]ecomfact.StorefrontDayFact{storefrontID: facts}},
		GL: glReader{amount: nil}, Fixed: fixedReader{amount: ptrF(1500)}})
	if err != nil {
		t.Fatalf("GL 未导入是降级不是错误：%v", err)
	}
	block := &stmt.Blocks[0]
	if block.Accounting.Revenue != nil || block.Accounting.Gap != "gl_unavailable" {
		t.Fatalf("GL 缺失必须具名降级：%+v", block.Accounting)
	}
	found := false
	for _, g := range stmt.Gaps {
		if g == "gl_unavailable：总账收入未导入该期间" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Gaps 必须列出 GL 降级：%+v", stmt.Gaps)
	}
	// 经营块不服输：净收入仍在
	if row := findRow(t, block, RowNetRevenue); row.Value == nil {
		t.Fatalf("GL 缺失不影响经营口径块")
	}
}

func TestProjectFixedCostMissingBreakEvenUnachievable(t *testing.T) {
	storefrontID := "SF-US"
	facts := []ecomfact.StorefrontDayFact{fact(storefrontID, "2026-08-01", 10000, 1000, 500, 100, 3000, 500, 300)}
	stmt, err := Project(context.Background(), SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		Period:     Period{Kind: PeriodMonthly, Month: "2026-08"},
	}, Readers{Facts: &fixedFactsReader{storefrontDays: map[string][]ecomfact.StorefrontDayFact{storefrontID: facts}},
		GL: glReader{amount: ptrF(8600)}, Fixed: fixedReader{amount: nil}})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	block := &stmt.Blocks[0]
	if block.BreakEven.Status != uniteconStatusUnachievable {
		t.Fatalf("固定费缺失时保本必须 unachievable：%+v", block.BreakEven)
	}
	if row := findRow(t, block, RowFixedCost); row.Value != nil {
		t.Fatalf("固定费行必须 nil：%v", row.Value)
	}
	if row := findRow(t, block, RowOperatingProfit); row.Value != nil {
		t.Fatalf("经营利润行必须 nil：%v", row.Value)
	}
}

const uniteconStatusUnachievable = "unachievable"

func TestProjectWeeklyWindow(t *testing.T) {
	storefrontID := "SF-US"
	facts := []ecomfact.StorefrontDayFact{
		fact(storefrontID, "2026-08-03", 1000, 0, 0, 0, 300, 50, 30),
		fact(storefrontID, "2026-08-04", 2000, 100, 0, 0, 600, 100, 60),
	}
	stmt, err := Project(context.Background(), SitePnlRequest{
		Storefront: ecomfact.StorefrontRef{LegalEntityID: "LE-1", StorefrontID: storefrontID},
		Period:     Period{Kind: PeriodWeekly, From: mustDate("2026-08-03"), To: mustDate("2026-08-09")},
	}, Readers{Facts: &fixedFactsReader{storefrontDays: map[string][]ecomfact.StorefrontDayFact{storefrontID: facts}},
		GL: glReader{amount: nil}, Fixed: fixedReader{amount: ptrF(0)}})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	block := &stmt.Blocks[0]
	// GMV = 1000 + (2000−100) = 2900；净收入 = 2900；CM1 = 2900−900−150−90 = 1760
	if row := findRow(t, block, RowGMV); row.Value == nil || *row.Value != 2900 {
		t.Fatalf("周度 GMV 应 2900：%v", row.Value)
	}
	if row := findRow(t, block, RowCM1); row.Value == nil || *row.Value != 1760 {
		t.Fatalf("周度 CM1 应 1760：%v", row.Value)
	}
	// 周度无 GL 口径：诚实降级
	if block.Accounting.Gap != "gl_unavailable" {
		t.Fatalf("周度 GL 应降级 gl_unavailable：%+v", block.Accounting)
	}
}
