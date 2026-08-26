package settlement

import (
	"testing"
	"time"
)

// golden 用例固定值：2026-08 月结。
var d = func(day int) time.Time { return time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC) }
var day7 = d(7)

func mustCategory(t *testing.T, results []MatchResult, want Category) float64 {
	t.Helper()
	for _, r := range results {
		if r.Status == "difference" && r.Category == want {
			return r.Amount
		}
	}
	t.Fatalf("期望差异类别 %q，实际分类：%+v", want, categoriesOf(results))
	return 0
}

func categoriesOf(results []MatchResult) []Category {
	out := []Category{}
	for _, r := range results {
		if r.Status == "difference" {
			out = append(out, r.Category)
		}
	}
	return out
}

func missing(t *testing.T, results []MatchResult, want Category) {
	t.Helper()
	for _, r := range results {
		if r.Status == "difference" && r.Category == want {
			t.Fatalf("不该出现类别 %q：%+v", want, results)
		}
	}
}

func countMatched(results []MatchResult) int {
	n := 0
	for _, r := range results {
		if r.Status == "matched" {
			n++
		}
	}
	return n
}

func TestMatchAllClean(t *testing.T) {
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-001", PayoutDate: day7, Currency: "USD",
		GrossAmount: 1000, FeeAmount: 40, NetAmount: 960,
	}}
	receivables := []ReceivableLine{{PayoutID: "P-001", Currency: "USD", Amount: 1000}}
	banks := []BankLine{{BankRef: "BNK-001", ValueDate: day7, Currency: "USD", Amount: 960}}

	results := Match(payouts, receivables, banks, MatchPolicy{})
	if countMatched(results) != 1 {
		t.Fatalf("干净样本应 1 条 matched：%+v", results)
	}
	for _, want := range Categories {
		missing(t, results, want)
	}
	if verdict := ApprovalGate("2026-08", results); verdict.Verdict != "allow" {
		t.Fatalf("干净样本门禁应 allow：%+v", verdict)
	}
}

func TestMatchFeeCategory(t *testing.T) {
	// 应收应结 1000，实结 gross 960（手续费 40 已扣）→ 差额 40 归 fee
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-001", PayoutDate: day7, Currency: "USD",
		GrossAmount: 960, FeeAmount: 40, NetAmount: 960,
	}}
	receivables := []ReceivableLine{{PayoutID: "P-001", Currency: "USD", Amount: 1000}}
	banks := []BankLine{{BankRef: "BNK-001", ValueDate: day7, Currency: "USD", Amount: 960}}

	results := Match(payouts, receivables, banks, MatchPolicy{})
	mustCategory(t, results, CategoryFee)
}

func TestMatchFxCategory(t *testing.T) {
	payouts := []PayoutLine{{
		Provider: "paypal", PayoutID: "P-002", PayoutDate: day7, Currency: "USD",
		GrossAmount: 1012, FXAmount: 12, NetAmount: 1012,
	}}
	receivables := []ReceivableLine{{PayoutID: "P-002", Currency: "USD", Amount: 1000}}
	banks := []BankLine{{BankRef: "BNK-002", ValueDate: day7, Currency: "USD", Amount: 1012}}
	results := Match(payouts, receivables, banks, MatchPolicy{})
	mustCategory(t, results, CategoryFX)
}

func TestMatchChargebackCategory(t *testing.T) {
	// 应收侧 1000，payout 实结 880：差 120 = 拒付
	payouts := []PayoutLine{{
		Provider: "stripe", PayoutID: "P-003", PayoutDate: day7, Currency: "USD",
		GrossAmount: 880, ChargebackAmount: 120, NetAmount: 880,
	}}
	receivables := []ReceivableLine{{PayoutID: "P-003", Currency: "USD", Amount: 1000}}
	banks := []BankLine{{BankRef: "BNK-003", ValueDate: day7, Currency: "USD", Amount: 880}}
	results := Match(payouts, receivables, banks, MatchPolicy{})
	mustCategory(t, results, CategoryChargeback)
}

func TestMatchInTransitCategory(t *testing.T) {
	// payout 无等额银行到账 → in_transit
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-004", PayoutDate: day7, Currency: "USD",
		GrossAmount: 500, NetAmount: 480,
	}}
	results := Match(payouts, nil, nil, MatchPolicy{})
	mustCategory(t, results, CategoryInTransit)

	// 银行到账无 payout 认领 → in_transit
	banks := []BankLine{{BankRef: "BNK-004", ValueDate: d(8), Currency: "USD", Amount: 320}}
	results = Match(nil, nil, banks, MatchPolicy{})
	mustCategory(t, results, CategoryInTransit)
}

func TestMatchAdjustmentCategory(t *testing.T) {
	// 平台账单调整：应收 1000 实结 950，无组成字段可归 → adjustment
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-005", PayoutDate: day7, Currency: "USD",
		GrossAmount: 950, NetAmount: 950,
	}}
	receivables := []ReceivableLine{{PayoutID: "P-005", Currency: "USD", Amount: 1000}}
	banks := []BankLine{{BankRef: "BNK-005", ValueDate: day7, Currency: "USD", Amount: 950}}
	results := Match(payouts, receivables, banks, MatchPolicy{})
	mustCategory(t, results, CategoryAdjustment)
}

func TestMatchReserveCategory(t *testing.T) {
	// 准备金占用单独产出一条 reserve
	payouts := []PayoutLine{{
		Provider: "paypal", PayoutID: "P-006", PayoutDate: day7, Currency: "USD",
		GrossAmount: 2000, ReserveHoldAmount: 100, NetAmount: 1900,
	}}
	results := Match(payouts, nil, nil, MatchPolicy{})
	mustCategory(t, results, CategoryReserve)
}

func TestMatchCurrencyPartitionNeverCrossMatches(t *testing.T) {
	// 跨币种永不匹配：USD payout 不能被 EUR 银行行认领
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-007", PayoutDate: day7, Currency: "USD",
		GrossAmount: 100, NetAmount: 96,
	}}
	banks := []BankLine{{BankRef: "BNK-007", ValueDate: day7, Currency: "EUR", Amount: 96}}
	results := Match(payouts, nil, banks, MatchPolicy{})
	mustCategory(t, results, CategoryInTransit)
	missing(t, results, CategoryAdjustment)
}

func TestMatchTransitWindowRule(t *testing.T) {
	// 超出在途窗口的银行行不算匹配（默认 7 天）
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-008", PayoutDate: d(1), Currency: "USD",
		GrossAmount: 100, NetAmount: 96,
	}}
	banks := []BankLine{{BankRef: "BNK-008", ValueDate: d(20), Currency: "USD", Amount: 96}}
	results := Match(payouts, nil, banks, MatchPolicy{})
	mustCategory(t, results, CategoryInTransit)
}

func TestApprovalGateDenyOnAnyDifference(t *testing.T) {
	payouts := []PayoutLine{{
		Provider: "shopify_payments", PayoutID: "P-009", PayoutDate: day7, Currency: "USD",
		GrossAmount: 100, NetAmount: 96,
	}}
	results := Match(payouts, nil, nil, MatchPolicy{})
	verdict := ApprovalGate("2026-08", results)
	if verdict.Verdict != "deny" {
		t.Fatalf("存在在途差异时门禁必须 deny：%+v", verdict)
	}
	if verdict.DifferenceCount != 1 || verdict.ByCategory["in_transit"] != 1 {
		t.Fatalf("差异统计错误：%+v", verdict)
	}
	// allow 的门禁必须零差异
	clean := ApprovalGate("2026-08", []MatchResult{{Status: "matched"}})
	if clean.Verdict != "allow" {
		t.Fatalf("零差异门禁应 allow：%+v", clean)
	}
}

func TestCategoryClassClosedEnum(t *testing.T) {
	if len(Categories) != 6 {
		t.Fatalf("六类封闭枚举应有 6 项：%v", Categories)
	}
	seen := map[Category]bool{}
	for _, c := range Categories {
		if !c.Valid() {
			t.Fatalf("枚举项 %q 自身不合法", c)
		}
		if seen[c] {
			t.Fatalf("重复类别 %q", c)
		}
		seen[c] = true
	}
	for _, c := range []Category{"other", "misc", "", "charge_back"} {
		if c.Valid() {
			t.Fatalf("非封闭值 %q 不应通过 Valid()", c)
		}
	}
}

func TestReservePositionsStateMachine(t *testing.T) {
	events := []ReserveEvent{
		{EventType: "hold", EventDate: d(1), Currency: "USD", Amount: 500, PayoutID: "P-100"},
		{EventType: "hold", EventDate: d(2), Currency: "USD", Amount: 300, PayoutID: "P-101"},
		{EventType: "release", EventDate: d(15), Currency: "USD", Amount: 500, PayoutID: "P-100"},
	}
	positions := ReservePositions(events)
	if len(positions) != 1 {
		t.Fatalf("应只有一个币种分区：%+v", positions)
	}
	pos := positions[0]
	if pos.HeldOpen != 300 || pos.Released != 500 || pos.NetFrozen != 300 {
		t.Fatalf("释放后头寸错误（P-101 的 300 仍在占用中）：%+v", pos)
	}
	if len(pos.Issues) != 0 {
		t.Fatalf("正常释放不应有 issue：%+v", pos.Issues)
	}

	// release 无对应 open hold → issue
	bad := ReservePositions([]ReserveEvent{{EventType: "release", EventDate: d(3), Currency: "USD", Amount: 100, PayoutID: "P-NONE"}})
	if len(bad) != 1 || len(bad[0].Issues) == 0 {
		t.Fatalf("无主 release 必须报 issue：%+v", bad)
	}
	if bad[0].Released != 100 || bad[0].HeldOpen != -100 {
		t.Fatalf("无主 release 的头寸必须显式可见：%+v", bad[0])
	}
}
