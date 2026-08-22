package varianceattribution

import (
	"math"
	"reflect"
	"testing"
)

// ─── 守恒断言的恒真陷阱：本文件选模块设计 §7 的方案 (a) ───────────────────
//
// 选 (a)（降级为构造断言，逐格锁定连环替代的中间值序列），不选 (b)
// （残差独立定义两路对照）。理由：
//   (b) 的「高阶交叉项解析式」在七因子下是 2^7−8 个交互项的组合爆炸，
//       写错没有任何人能看出来——等于用一个更难验的东西去验一个难验的东西；
//   (a) 的每一步中间利润都可以用计算器手算复核，fixture 逐格钉死后，
//       改任何一步替换逻辑（比如把累计替代改成对基期隔离替代）立即红。
//
// 同时如实声明一个数学事实：精确连环替代的各步贡献之和按望远镜和恒等于
// 总差异，残差按构造 ≈ 0（只承载浮点噪声）。因此「Σ因子+残差=总差异」
// 在本实现里不是检查对象而是构造性质；真正的检查对象是中间值序列本身
// 与顺序敏感性。残差字段的材料性判定单独测 IsResidualMaterial。

func f64(v float64) *float64 { return &v }

// 手算 fixture：
// 基期：客流 1000、交易 100、销售 20000 → 转化率 0.10、客单价 200；
//       毛利 6000 → 毛利率 0.30；人工 1000、占用 800、其他 200。利润 = 4000。
// 当期：客流 1100、交易 99、销售 24750 → 转化率 0.09、客单价 250；
//       毛利 4950 → 毛利率 0.20；人工 1200、占用 700、其他 150。利润 = 2900。
// 总差异 = −1100。
func fixture() (PeriodFacts, PeriodFacts) {
	base := PeriodFacts{
		Footfall: f64(1000), Transactions: f64(100), Revenue: f64(20000),
		GrossProfit: f64(6000), LaborCost: f64(1000), OccupancyCost: f64(800),
		OtherControllableCost: f64(200),
	}
	current := PeriodFacts{
		Footfall: f64(1100), Transactions: f64(99), Revenue: f64(24750),
		GrossProfit: f64(4950), LaborCost: f64(1200), OccupancyCost: f64(700),
		OtherControllableCost: f64(150),
	}
	return base, current
}

func TestChainReplacementIntermediateSequenceLockedCellByCell(t *testing.T) {
	base, current := fixture()
	res, err := Attribute(base, current, "CNY", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "complete" {
		t.Fatalf("status = %s (%v)", res.Status, res.MissingFacts)
	}
	if res.BaseProfit != 4000 || res.CurrentProfit != 2900 || res.TotalVariance != -1100 {
		t.Fatalf("endpoints wrong: base=%v current=%v total=%v", res.BaseProfit, res.CurrentProfit, res.TotalVariance)
	}

	// 构造断言的核心：逐步替换后的中间利润序列与手算逐格一致。
	// 手算（默认顺序 客流→转化→客单→毛利率→人工→占用→其他）：
	//   S1 客流→1100:      1100×0.10×200×0.30 − 2000 = 4600   C1=+600
	//   S2 转化率→0.09:    1100×0.09×200×0.30 − 2000 = 3940   C2=−660
	//   S3 客单价→250:     1100×0.09×250×0.30 − 2000 = 5425   C3=+1485
	//   S4 毛利率→0.20:    1100×0.09×250×0.20 − 2000 = 2950   C4=−2475
	//   S5 人工→1200:      2950 − 200                = 2750   C5=−200
	//   S6 占用→700:       2750 + 100                = 2850   C6=+100
	//   S7 其他→150:       2850 + 50                 = 2900   C7=+50
	wantIntermediate := []float64{4600, 3940, 5425, 2950, 2750, 2850, 2900}
	wantEffects := []float64{600, -660, 1485, -2475, -200, 100, 50}
	wantOrder := []string{"footfall", "conversion_rate", "average_transaction_value", "gross_margin_rate", "labor_cost", "occupancy_cost", "other_controllable_cost"}

	if !reflect.DeepEqual(res.DecompositionOrder, wantOrder) {
		t.Fatalf("decomposition order must be echoed verbatim, got %v", res.DecompositionOrder)
	}
	for i, fc := range res.Factors {
		if fc.IntermediateProfit != wantIntermediate[i] {
			t.Fatalf("step %d intermediate profit = %v, want %v（手算锚点被改动）", i+1, fc.IntermediateProfit, wantIntermediate[i])
		}
		if fc.Effect != wantEffects[i] {
			t.Fatalf("step %d effect = %v, want %v", i+1, fc.Effect, wantEffects[i])
		}
	}

	// 守恒在这里是构造性质：望远镜和。残差只允许浮点噪声级别的偏差。
	var sum float64
	for _, fc := range res.Factors {
		sum += fc.Effect
	}
	if math.Abs(sum+res.Residual-(-1100)) > 0.01 {
		t.Fatalf("telescope broken: sum=%v residual=%v total=-1100", sum, res.Residual)
	}
	if res.ResidualMaterial {
		t.Fatalf("residual %v must not be material under exact chain replacement", res.Residual)
	}
}

// 顺序敏感性：换序后每个因子的贡献值必须变化（交互作用被吸收进后位因子），
// 且 DecompositionOrder 随之回显；总差异不变（终点相同）。
func TestOrderChangesEffectsAndIsEchoed(t *testing.T) {
	base, current := fixture()

	def, err := Attribute(base, current, "CNY", nil)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []Factor{FactorOther, FactorOccupancy, FactorLabor, FactorMarginRate, FactorAvgTicket, FactorConversion, FactorFootfall}
	rev, err := Attribute(base, current, "CNY", reversed)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(rev.DecompositionOrder, []string{"other_controllable_cost", "occupancy_cost", "labor_cost", "gross_margin_rate", "average_transaction_value", "conversion_rate", "footfall"}) {
		t.Fatalf("reversed order not echoed: %v", rev.DecompositionOrder)
	}
	if def.TotalVariance != rev.TotalVariance {
		t.Fatalf("total variance must be order-independent: %v vs %v", def.TotalVariance, rev.TotalVariance)
	}
	effectOf := func(res Result, factor string) float64 {
		for _, fc := range res.Factors {
			if fc.Factor == factor {
				return fc.Effect
			}
		}
		t.Fatalf("factor %s missing", factor)
		return 0
	}
	// 手算（逆序下毛利率第三步替换）：−2000，而默认序下是 −2475
	if effectOf(def, "gross_margin_rate") == effectOf(rev, "gross_margin_rate") {
		t.Fatalf("margin effect must change with replacement order: %v both", effectOf(def, "gross_margin_rate"))
	}
	if effectOf(rev, "gross_margin_rate") != -2000 {
		t.Fatalf("reversed-order margin effect = %v, want −2000（手算锚点）", effectOf(rev, "gross_margin_rate"))
	}
}

func TestInvalidOrderRejected(t *testing.T) {
	base, current := fixture()
	if _, err := Attribute(base, current, "", []Factor{FactorFootfall}); err == nil {
		t.Fatal("incomplete order must be rejected")
	}
	if _, err := Attribute(base, current, "", []Factor{FactorFootfall, FactorFootfall, FactorConversion, FactorAvgTicket, FactorMarginRate, FactorLabor, FactorOccupancy}); err == nil {
		t.Fatal("duplicate factor must be rejected")
	}
	if _, err := Attribute(base, current, "", []Factor{"dupond"}); err == nil {
		t.Fatal("unknown factor must be rejected")
	}
}

// 缺一即整体不可用：不做部分归因。MissingFacts 列出期间限定字段名。
func TestMissingAnyFactMakesWholeAttributionUnavailable(t *testing.T) {
	base, current := fixture()

	cases := []struct {
		name   string
		mutate func(*PeriodFacts, *PeriodFacts)
		want   []string
	}{
		{"base gross profit nil", func(b, c *PeriodFacts) { b.GrossProfit = nil }, []string{"base.gross_profit"}},
		{"current occupancy nil", func(b, c *PeriodFacts) { c.OccupancyCost = nil }, []string{"current.occupancy_cost"}},
		{"two missing listed together", func(b, c *PeriodFacts) { b.LaborCost = nil; c.Revenue = nil }, []string{"base.labor_cost", "current.revenue"}},
	}
	for _, tc := range cases {
		b, c := base, current
		tc.mutate(&b, &c)
		res, err := Attribute(b, c, "CNY", nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if res.Status != "unavailable" {
			t.Fatalf("%s: status = %s, want unavailable（不做部分归因）", tc.name, res.Status)
		}
		if len(res.Factors) != 0 {
			t.Fatalf("%s: unavailable result must carry no factor numbers", tc.name)
		}
		if !equalStrings(res.MissingFacts, tc.want) {
			t.Fatalf("%s: missing facts = %v, want %v", tc.name, res.MissingFacts, tc.want)
		}
	}
}

// 零分母比率无定义：不许拿 0 顶替转化率继续分解。
func TestZeroDenominatorRatesAreUnavailable(t *testing.T) {
	base, current := fixture()
	base.Transactions = f64(0)
	res, err := Attribute(base, current, "CNY", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "unavailable" {
		t.Fatalf("zero transactions must make attribution unavailable, got %s", res.Status)
	}
	if !equalStrings(res.MissingFacts, []string{"transactions(=0)"}) {
		t.Fatalf("missing facts = %v", res.MissingFacts)
	}
}

func TestIsResidualMaterial(t *testing.T) {
	threshold := ResidualMaterialThreshold
	if threshold != 0.05 {
		t.Fatalf("threshold contract changed unexpectedly: %v", threshold)
	}
	if IsResidualMaterial(0, -1100) {
		t.Fatal("exact-zero residual is never material")
	}
	if !IsResidualMaterial(60, -1100) {
		t.Fatal("residual above 5%% of total must be material")
	}
	if IsResidualMaterial(50, -1100) {
		t.Fatal("residual at or below 5%% of total must not be material")
	}
	if !IsResidualMaterial(0.01, 0) {
		t.Fatal("nonzero residual against a zero total is material by definition")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]int{}
	for _, s := range a {
		set[s]++
	}
	for _, s := range b {
		set[s]--
		if set[s] < 0 {
			return false
		}
	}
	return true
}
