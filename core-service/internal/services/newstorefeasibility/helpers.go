package newstorefeasibility

import (
	"math"
	"sort"
	"strings"
	"time"
)

func inputsValid(in Input) bool {
	b := in.Business
	return in.Horizon > 0 &&
		b.DailyAreaFootfall >= 0 && b.OperatingDays > 0 &&
		b.EntryRate >= 0 && b.EntryRate <= 1 &&
		b.ConversionRate >= 0 && b.ConversionRate <= 1 &&
		b.AvgTicket >= 0 && b.GrossMarginRate >= 0 && b.GrossMarginRate <= 1 &&
		in.Investment.FitoutAndEquipment >= 0 && in.Investment.InitialInventory >= 0 &&
		in.Investment.RampMonths >= 0 &&
		strings.Count(in.StartMonth, "-") == 1 && len(in.StartMonth) == 7
}

// monthlyRevenue 推导链（D-R4）：商圈日均客流 × 营业天数 × 进店率 × 转化率 × 客单价。
func monthlyRevenue(b BusinessDrivers) float64 {
	return b.DailyAreaFootfall * float64(b.OperatingDays) * b.EntryRate * b.ConversionRate * b.AvgTicket
}

func initialInvestment(p InvestmentPlan) float64 {
	return p.FitoutAndEquipment + p.InitialInventory
}

func rampFactor(p InvestmentPlan, index int) float64 {
	if p.RampMonths <= 0 || index >= p.RampMonths || index >= len(p.RampFactors) {
		return 1
	}
	f := p.RampFactors[index]
	if f <= 0 {
		return 1
	}
	return f
}

func monthLabels(startMonth string, months int) []string {
	t, err := time.Parse("2006-01", startMonth)
	if err != nil {
		return nil
	}
	out := make([]string, 0, months)
	for i := 0; i < months; i++ {
		out = append(out, t.AddDate(0, i, 0).Format("2006-01"))
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func roundPtr(v float64) *float64 { r := round2(v); return &r }

// irrMonthly 二分求月度 IRR：NPV(r) = Σ ncf_m/(1+r)^m − initial = 0。
// 区间 [-0.99, 1]，200 次二分（远超 float64 精度），确定性可 golden。
// 无解（区间端点同号）返回 NaN，由调用方决定是否留空。
func irrMonthly(monthlyNCF []float64, initial float64) float64 {
	npvAt := func(r float64) float64 {
		var s float64
		for i, cf := range monthlyNCF {
			s += cf / math.Pow(1+r, float64(i+1))
		}
		return s - initial
	}
	const lo, hi = -0.99, 1.0
	flo, fhi := npvAt(lo), npvAt(hi)
	if flo*fhi > 0 {
		return math.NaN()
	}
	low, high := lo, hi
	flow := flo
	mid := low
	for i := 0; i < 200; i++ {
		mid = (low + high) / 2
		fm := npvAt(mid)
		if flow*fm <= 0 {
			high = mid
			fhi = fm
		} else {
			low = mid
			flow = fm
		}
	}
	return mid
}
