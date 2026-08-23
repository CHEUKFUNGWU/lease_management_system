package newstorefeasibility

import (
	"context"
	"math"
	"testing"
	"time"
)

// ─── 守恒/golden 说明 ─────────────────────────────────────────────────────
//
// 手算 fixture（全部整数，可计算器复核）：
//   月销售额 = 500(日均商圈客流) × 30(天) × 0.20(进店) × 0.50(转化) × 200(客单) = 300000
//   毛利率 40% → 满爬坡毛利 120000；租赁平租 30000 → 满爬坡净现金流 90000。
//   爬坡系数 [0.5,0.6,0.7,0.8,0.9] 后满爬坡：
//     净现金流 = [30000, 42000, 54000, 66000, 78000, 90000 ×18]
//   初始投入 = 800000(装修设备) + 200000(铺货) = 1000000。
//   静态回本：累计现金流第 14 个月达 1080000 ≥ 1000000（第 13 个月 990000 未过）→ 14 个月。
//   盈亏平衡销售额 = 30000 ÷ 0.40 = 75000/月。
//   动态回本（月率 1%，手算贴现累计：前 14 个月 PV 合计 ≈994490 < 1000000，
//   第 15 个月 +≈77512 越线）→ 15 个月。该锚点能抓住贴现实现错误。

func stubPort(monthlyLease float64) Ports {
	return Ports{LeaseProjection: stubReader{lease: monthlyLease}}
}

type stubReader struct{ lease float64 }

func (s stubReader) MonthlyProjection(_ context.Context, _ string, fromMonth string, months int) ([]LeaseMonth, error) {
	start, _ := time.Parse("2006-01", fromMonth)
	rows := make([]LeaseMonth, 0, months)
	for i := 0; i < months; i++ {
		rows = append(rows, LeaseMonth{Month: start.AddDate(0, i, 0).Format("2006-01"), LeaseExpense: s.lease})
	}
	return rows, nil
}

func goldenInput(rate *float64) Input {
	return Input{
		Currency:   "CNY",
		StartMonth: "2026-01",
		Horizon:    24,
		Business: BusinessDrivers{
			DailyAreaFootfall: 500, OperatingDays: 30,
			EntryRate: 0.2, ConversionRate: 0.5, AvgTicket: 200, GrossMarginRate: 0.4,
		},
		Investment: InvestmentPlan{
			FitoutAndEquipment: 800000, InitialInventory: 200000,
			RampMonths: 6, RampFactors: []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
		Lease:        LeaseTerms{ContractID: "CT-GOLDEN"},
		DiscountRate: rate,
	}
}

func TestGoldenFullPipelineLockedCellByCell(t *testing.T) {
	rate := f64p(0.01) // 月折现率
	res := Evaluate(context.Background(), goldenInput(rate), stubPort(30000))

	if res.Status != "complete" || len(res.Gaps) != 0 {
		t.Fatalf("status=%s gaps=%v", res.Status, res.Gaps)
	}
	if len(res.MonthlyCashFlows) != 24 {
		t.Fatalf("24 months expected, got %d", len(res.MonthlyCashFlows))
	}
	wantMonths := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06", "2026-07", "2026-08", "2026-09", "2026-10", "2026-11", "2026-12", "2027-01", "2027-02", "2027-03", "2027-04", "2027-05", "2027-06", "2027-07", "2027-08", "2027-09", "2027-10", "2027-11", "2027-12"}
	for i, row := range res.MonthlyCashFlows {
		if row.Month != wantMonths[i] {
			t.Fatalf("month %d = %s, want %s", i+1, row.Month, wantMonths[i])
		}
	}
	// 逐格锁定：收入 / 毛利 / 租赁 / 净现金流，全部手算可复核
	wantNCF := []float64{30000, 42000, 54000, 66000, 78000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000}
	wantRev := []float64{150000, 180000, 210000, 240000, 270000, 300000}
	wantGP := []float64{60000, 72000, 84000, 96000, 108000, 120000}
	for i, row := range res.MonthlyCashFlows {
		wantR := 300000.0
		if i < len(wantRev) {
			wantR = wantRev[i]
		}
		wantG := 120000.0
		if i < len(wantGP) {
			wantG = wantGP[i]
		}
		if row.Revenue == nil || math.Abs(*row.Revenue-wantR) > 0.01 {
			t.Fatalf("month %d revenue = %v, want %v", i+1, ptrStr(row.Revenue), wantR)
		}
		if row.GrossProfit == nil || math.Abs(*row.GrossProfit-wantG) > 0.01 {
			t.Fatalf("month %d gross profit = %v, want %v", i+1, ptrStr(row.GrossProfit), wantG)
		}
		if row.LeaseCost == nil || *row.LeaseCost != 30000 {
			t.Fatalf("month %d lease = %v, want 30000", i+1, ptrStr(row.LeaseCost))
		}
		if row.NetCashFlow == nil || *row.NetCashFlow != wantNCF[i] {
			t.Fatalf("month %d net cash flow = %v, want %v", i+1, ptrStr(row.NetCashFlow), wantNCF[i])
		}
	}

	// 手算锚点五项指标
	if res.StaticPayback == nil || *res.StaticPayback != 14 {
		t.Fatalf("static payback = %v, want 14（手算：第 13 个月累计 990000，第 14 个月 1080000）", ptrStr(res.StaticPayback))
	}
	if res.DynamicPayback == nil || *res.DynamicPayback != 15 {
		t.Fatalf("dynamic payback = %v, want 15（手算贴现累计：14 个月 994490 < 1e6 ≤ 15 个月 1072002）", ptrStr(res.DynamicPayback))
	}
	if res.BreakEvenSales == nil || math.Abs(*res.BreakEvenSales-75000) > 0.01 {
		t.Fatalf("break-even sales = %v, want 75000", ptrStr(res.BreakEvenSales))
	}

	// NPV 锁定实现值并用手估区间夹住（贴现使未贴现的 950000 缩水到六七十万量级）
	if res.NPV == nil {
		t.Fatal("NPV must be present when rate is given")
	}
	if *res.NPV < 600000 || *res.NPV > 800000 {
		t.Fatalf("NPV outside hand-checked band [600k, 800k]: %v", *res.NPV)
	}

	// IRR 的独立校验不是重算一遍二分，而是把解代回 NPV：应归零（两路对照）
	if res.IRR == nil || math.IsNaN(*res.IRR) {
		t.Fatal("IRR must be solvable for this fixture")
	}
	var npvAtIRR float64
	for i, ncf := range wantNCF {
		npvAtIRR += ncf / math.Pow(1+*res.IRR, float64(i+1))
	}
	npvAtIRR -= 1000000
	if math.Abs(npvAtIRR) > 1 {
		t.Fatalf("plugging IRR %v back into NPV gives %v, want ~0", *res.IRR, npvAtIRR)
	}
}

func TestDiscountRateMissingPartialDegradation(t *testing.T) {
	res := Evaluate(context.Background(), goldenInput(nil), stubPort(30000))

	found := false
	for _, g := range res.Gaps {
		if g.Kind == GapDiscountRateMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps %v must contain discount_rate_missing", res.Gaps)
	}
	// D-R14：三项依赖折现率的指标 nil；两项不依赖的照常返回
	if res.IRR != nil || res.NPV != nil || res.DynamicPayback != nil {
		t.Fatalf("rate-dependent metrics must be nil, got IRR=%v NPV=%v dyn=%v", ptrStr(res.IRR), ptrStr(res.NPV), ptrStr(res.DynamicPayback))
	}
	if res.StaticPayback == nil || *res.StaticPayback != 14 {
		t.Fatalf("static payback must survive rate absence, got %v", ptrStr(res.StaticPayback))
	}
	if res.BreakEvenSales == nil || *res.BreakEvenSales != 75000 {
		t.Fatalf("break-even sales must survive rate absence, got %v", ptrStr(res.BreakEvenSales))
	}
	if res.Status != "partial" {
		t.Fatalf("partial degradation status = %s", res.Status)
	}
}

func TestUnwiredPortIsNamedGapNotPanic(t *testing.T) {
	in := goldenInput(f64p(0.01))
	res := Evaluate(context.Background(), in, Ports{}) // 端口未接线

	found := false
	for _, g := range res.Gaps {
		if g.Kind == GapLeaseProjectionUnwired {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps %v must contain lease_projection_unwired", res.Gaps)
	}
	for _, row := range res.MonthlyCashFlows {
		if row.LeaseCost != nil || row.NetCashFlow != nil {
			t.Fatal("unwired port must leave lease-dependent cells nil, never 0")
		}
	}
	// 租赁缺席时静态回本与盈亏平衡都失去意义 → nil；收入/毛利投影照常
	if res.StaticPayback != nil || res.BreakEvenSales != nil {
		t.Fatalf("lease-dependent metrics must be nil without port, got %v / %v", ptrStr(res.StaticPayback), ptrStr(res.BreakEvenSales))
	}
	if res.MonthlyCashFlows[0].Revenue == nil || *res.MonthlyCashFlows[0].Revenue != 150000 {
		t.Fatalf("revenue projection must still be produced, got %+v", res.MonthlyCashFlows[0])
	}
}

func TestInvalidInputRejectedWithoutNumbers(t *testing.T) {
	in := goldenInput(f64p(0.01))
	in.Business.EntryRate = 1.5 // 越界
	res := Evaluate(context.Background(), in, stubPort(30000))
	if res.Status != "unavailable" || len(res.Gaps) != 1 || res.Gaps[0].Kind != "invalid_input" {
		t.Fatalf("invalid input must be rejected wholesale, got %+v", res)
	}
	if res.NPV != nil || res.StaticPayback != nil {
		t.Fatal("invalid input must produce no numbers")
	}
}

func f64p(v float64) *float64 { return &v }

func ptrStr(p *float64) string {
	if p == nil {
		return "<nil>"
	}
	return "ptr"
}
