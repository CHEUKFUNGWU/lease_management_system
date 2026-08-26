// Package settlement 是收款对账引擎与口径门禁（模块深化 EM5）。
//
// 三方匹配（payout × 订单应收 × 银行到账）是整个电商模式的信任锚点。
// 「未对平不得进 Approved」如果只是一条提示它会被无视；必须是门——
// ApprovalGate 是独立函数（D-E5），才能被集成测试独立钉住（R-E4-3）。
//
// Settlement Reconciliation 与 Tie-Out 是两个词：前者是 GL 对账域的三方证据匹配，
// 后者是模型勾稽。本包只做前者，任何标识符不得混用（spec R-T2）。
package settlement

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// PolicyVersion 匹配政策版本：改判定规则必须出新版本并在 run 里回显。
const PolicyVersion = "settlement-match-v1"

// Category 六类封闭枚举：手续费 / 汇兑 / 拒付 / 在途 / 调整 / 准备金（R-E4-1）。
type Category string

const (
	CategoryFee        Category = "fee"
	CategoryFX         Category = "fx"
	CategoryChargeback Category = "chargeback"
	CategoryInTransit  Category = "in_transit"
	CategoryAdjustment Category = "adjustment"
	CategoryReserve    Category = "reserve"
)

// Categories 全部合法类别（顺序即展示序）。
var Categories = []Category{CategoryFee, CategoryFX, CategoryChargeback, CategoryInTransit, CategoryAdjustment, CategoryReserve}

// Valid 拒绝第七种归类——差异只能落进具名类别，不许「其他」兜底。
func (c Category) Valid() bool {
	for _, v := range Categories {
		if v == c {
			return true
		}
	}
	return false
}

// MatchPolicy 匹配政策：金额容差与在途窗口。零值取默认。
type MatchPolicy struct {
	AmountTolerance   float64 // 绝对容差，默认 0.01
	TransitWindowDays int     // payout→银行的最长在途天数，默认 7
}

func (p MatchPolicy) tolerance() float64 {
	if p.AmountTolerance > 0 {
		return p.AmountTolerance
	}
	return 0.01
}

func (p MatchPolicy) transitDays() int {
	if p.TransitWindowDays > 0 {
		return p.TransitWindowDays
	}
	return 7
}

// PayoutLine payout 明细输入行。
type PayoutLine struct {
	Provider      string    `json:"provider"`
	PayoutID      string    `json:"payout_id"`
	PayoutDate    time.Time `json:"payout_date"`
	Currency      string    `json:"currency"`
	GrossAmount   float64   `json:"gross_amount"`
	FeeAmount     float64   `json:"fee_amount"`
	RefundAmount  float64   `json:"refund_amount"`
	ChargebackAmount float64 `json:"chargeback_amount"`
	FXAmount      float64   `json:"fx_amount"`
	AdjustmentAmount float64 `json:"adjustment_amount"`
	ReserveHoldAmount    float64 `json:"reserve_hold_amount"`
	ReserveReleaseAmount float64 `json:"reserve_release_amount"`
	NetAmount     float64   `json:"net_amount"`
}

// ReceivableLine 订单应收聚合行：按 payout 归集的应结金额。
type ReceivableLine struct {
	PayoutID   string    `json:"payout_id"`
	Date       time.Time `json:"date"`
	Currency   string    `json:"currency"`
	Amount     float64   `json:"amount"` // 应结净额（订单侧口径）
}

// BankLine 银行到账行。
type BankLine struct {
	BankRef  string    `json:"bank_ref"`
	ValueDate time.Time `json:"value_date"`
	Currency string    `json:"currency"`
	Amount   float64   `json:"amount"`
}

// EvidenceRef 差异证据引用：三击到来源的对账锚点（payout 行 / 银流行）。
type EvidenceRef struct {
	PayoutProvider string `json:"payout_provider,omitempty"`
	PayoutID       string `json:"payout_id,omitempty"`
	BankRef        string `json:"bank_ref,omitempty"`
	ExpectedAmount *float64 `json:"expected_amount,omitempty"`
	ActualAmount   *float64 `json:"actual_amount,omitempty"`
	Note           string   `json:"note,omitempty"`
}

// MatchResult 一条匹配结论：Matched 或 Diff{Category}。二者互斥。
type MatchResult struct {
	Status    string      `json:"status"` // matched | difference
	Currency  string      `json:"currency"`
	Category  Category    `json:"category,omitempty"` // difference 必填且必须六类之一
	Amount    float64     `json:"amount"`             // 差异绝对额
	Evidence  EvidenceRef `json:"evidence"`
}

// Match 纯函数三方匹配器。
//
// 判定顺序（固定、可复演；每条 payout 至多产生一条主差异 + 准备金一条）：
//  1. 按 (currency) 分区，payout 以 NetAmount 在在途窗口内找等额银行行（贪心、金额升序）；
//  2. 找不到银行行 ⇒ in_transit（payout 未到账）；剩余银行行 ⇒ in_transit（无 payout 认领）；
//  3. payout vs 订单应收：净差按组成归入 fee / fx / chargeback / adjustment（按组成字段命中，
//     多组成并存时按 fee → fx → chargeback → adjustment 取首个非零组成，其余并入 evidence.note）；
//  4. reserve_hold > 0 的 payout 追加 reserve 一条（占用），release 只在准备金台账跟踪不重复计数。
//
// 守恒说明：各差异之和 = Σ|payout.net − bank.amount| − 已对平额，是构造结果不是事后断言
// （红线 12 同款纪律——测试锁中间序列与分类正确性，不测「桥加起来等于总差」恒真式）。
func Match(payouts []PayoutLine, receivables []ReceivableLine, banks []BankLine, policy MatchPolicy) []MatchResult {
	results := make([]MatchResult, 0)
	tol := policy.tolerance()

	receivableByPayout := map[string]ReceivableLine{}
	for _, r := range receivables {
		receivableByPayout[r.PayoutID] = r
	}

	// 币种分区：跨币种永不匹配（R-E2-5 的对账投影）。
	// 分区集合 = payout 币种 ∪ 银行币种——没有 payout 的币种也要处理「无主银行行」。
	byCur := map[string][]PayoutLine{}
	bankIdxByCur := map[string][]int{}
	currencies := []string{}
	seenCur := map[string]bool{}
	addCur := func(cur string) {
		if !seenCur[cur] {
			seenCur[cur] = true
			currencies = append(currencies, cur)
		}
	}
	for _, p := range payouts {
		addCur(p.Currency)
		byCur[p.Currency] = append(byCur[p.Currency], p)
	}
	for i, b := range banks {
		if b.Amount < 0 {
			continue
		}
		addCur(b.Currency)
		bankIdxByCur[b.Currency] = append(bankIdxByCur[b.Currency], i)
	}
	sort.Strings(currencies)

	usedBanks := map[int]bool{}

	for _, cur := range currencies {
		pl := byCur[cur]
		sort.Slice(pl, func(i, j int) bool { return pl[i].NetAmount < pl[j].NetAmount })

		// 金额升序便于贪心等额配对
		idx := bankIdxByCur[cur]
		sort.Slice(idx, func(i, j int) bool { return banks[idx[i]].Amount < banks[idx[j]].Amount })

		for _, p := range pl {
			matchedBank := -1
			for _, bi := range idx {
				if usedBanks[bi] {
					continue
				}
				b := banks[bi]
				if withinTransit(p.PayoutDate, b.ValueDate, policy.transitDays()) &&
					math.Abs(b.Amount-p.NetAmount) <= tol {
					matchedBank = bi
					break
				}
			}

			// 步骤 3：payout vs 订单应收的差异归因。应收侧口径 = 订单应结净额；
			// 实结总额 gross 与应结的差，按组成字段归入 fee / fx / chargeback /
			// adjustment（classifyDelta 内按 fee → fx → chargeback 顺序命中首个非零组成）。
			rec, hasRec := receivableByPayout[p.PayoutID]
			if hasRec {
				delta := rec.Amount - p.GrossAmount
				if math.Abs(delta) > tol {
					cat, note := classifyDelta(p, math.Abs(delta))
					exp := round2(rec.Amount)
					act := round2(p.GrossAmount)
					results = append(results, MatchResult{
						Status: "difference", Currency: cur, Category: cat,
						Amount: round2(math.Abs(delta)),
						Evidence: EvidenceRef{
							PayoutProvider: p.Provider, PayoutID: p.PayoutID,
							ExpectedAmount: &exp, ActualAmount: &act, Note: note,
						},
					})
				}
			}

			// 步骤 1/2：银行到账匹配
			if matchedBank < 0 {
				net := round2(p.NetAmount)
				results = append(results, MatchResult{
					Status: "difference", Currency: cur, Category: CategoryInTransit,
					Amount: round2(math.Abs(net)),
					Evidence: EvidenceRef{PayoutProvider: p.Provider, PayoutID: p.PayoutID,
						ExpectedAmount: &net, Note: "payout 无等额银行到账（窗口内）"},
				})
			} else {
				usedBanks[matchedBank] = true
				b := banks[matchedBank]
				delta := p.NetAmount - b.Amount
				if math.Abs(delta) > tol {
					cat, note := classifyDelta(p, math.Abs(delta))
					act := round2(b.Amount)
					results = append(results, MatchResult{
						Status: "difference", Currency: cur, Category: cat,
						Amount: round2(math.Abs(delta)),
						Evidence: EvidenceRef{PayoutProvider: p.Provider, PayoutID: p.PayoutID, BankRef: b.BankRef,
							ActualAmount: &act, Note: note},
					})
				} else {
					results = append(results, MatchResult{
						Status: "matched", Currency: cur,
						Evidence: EvidenceRef{PayoutProvider: p.Provider, PayoutID: p.PayoutID, BankRef: b.BankRef},
					})
				}
			}

			// 步骤 4：准备金占用单列一条（释放事件由台账状态机跟踪）
			if p.ReserveHoldAmount > tol {
				held := round2(p.ReserveHoldAmount)
				results = append(results, MatchResult{
					Status: "difference", Currency: cur, Category: CategoryReserve,
					Amount: held,
					Evidence: EvidenceRef{PayoutProvider: p.Provider, PayoutID: p.PayoutID,
						ActualAmount: &held, Note: "滚动准备金占用"},
				})
			}
		}

		// 未被认领的银行行 ⇒ in_transit（可能对应尚未导入的 payout）
		for _, bi := range idx {
			if usedBanks[bi] {
				continue
			}
			b := banks[bi]
			amt := round2(b.Amount)
			results = append(results, MatchResult{
				Status: "difference", Currency: cur, Category: CategoryInTransit,
				Amount: amt,
				Evidence: EvidenceRef{BankRef: b.BankRef, ActualAmount: &amt,
					Note: "银行到账无对应 payout"},
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Currency != results[j].Currency {
			return results[i].Currency < results[j].Currency
		}
		if results[i].Status != results[j].Status {
			return results[i].Status < results[j].Status
		}
		return results[i].Evidence.PayoutID < results[j].Evidence.PayoutID
	})
	return results
}

// classifyDelta 把净差归入六类之一：按 payout 组成字段的非零项优先级
// fee → fx → chargeback → adjustment；全部为零时不该发生（防御性落 adjustment 并注明）。
func classifyDelta(p PayoutLine, delta float64) (Category, string) {
	tol := 0.01
	switch {
	case math.Abs(delta-p.FeeAmount) <= tol && p.FeeAmount > 0:
		return CategoryFee, "差额等于手续费"
	case math.Abs(delta-p.FXAmount) <= tol && p.FXAmount != 0:
		return CategoryFX, "差额等于汇兑损益"
	case math.Abs(delta-p.ChargebackAmount) <= tol && p.ChargebackAmount > 0:
		return CategoryChargeback, "差额等于拒付"
	case p.RefundAmount > tol:
		return CategoryChargeback, "差额源于退款"
	default:
		return CategoryAdjustment, "平台账单调整"
	}
}

// GateVerdict 口径门禁裁决。
type GateVerdict struct {
	Verdict          string        `json:"verdict"` // allow | deny
	DifferenceCount  int           `json:"difference_count"`
	TotalDifference  float64       `json:"total_difference_amount"`
	ByCategory       map[string]int `json:"by_category,omitempty"`
	Reasons          []string      `json:"reasons,omitempty"`
}

// ApprovalGate 口径门禁（D8/D-E5）：存在任何未对平差异 ⇒ deny。
// deny 的期间，其收入/现金数字不得进入 Approved 口径报表（R-E4-3），
// 差异清单由调用方自动写入 Data Quality Queue。
func ApprovalGate(period string, results []MatchResult) GateVerdict {
	verdict := GateVerdict{Verdict: "allow", ByCategory: map[string]int{}}
	seen := map[string]bool{}
	for _, r := range results {
		if r.Status != "difference" {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s|%v", r.Evidence.PayoutID, r.Evidence.BankRef, r.Category, r.Amount)
		if seen[key] {
			continue
		}
		seen[key] = true
		verdict.Verdict = "deny"
		verdict.DifferenceCount++
		verdict.TotalDifference = round2(verdict.TotalDifference + r.Amount)
		verdict.ByCategory[string(r.Category)]++
		verdict.Reasons = append(verdict.Reasons,
			fmt.Sprintf("%s %s %s ¥%.2f", r.Evidence.PayoutID, r.Evidence.BankRef, r.Category, r.Amount))
	}
	_ = period
	return verdict
}

// ReserveEvent 准备金台账事件（hold/release 状态机的载体）。
type ReserveEvent struct {
	EventType   string    `json:"event_type"` // hold | release
	EventDate   time.Time `json:"event_date"`
	Currency    string    `json:"currency"`
	Amount      float64   `json:"amount"`
	PayoutID    string    `json:"payout_id,omitempty"`
	HoldEventID string    `json:"hold_event_id,omitempty"`
}

// ReservePosition 一个币种的准备金头寸：open holds 合计 − 已释放合计。
// release 无对应 open hold 时返回 issue（状态机违规可见，不静默吞掉）。
type ReservePosition struct {
	Currency string    `json:"currency"`
	HeldOpen float64   `json:"held_open"`
	Released float64   `json:"released"`
	NetFrozen float64  `json:"net_frozen"`
	Issues   []string  `json:"issues,omitempty"`
}

func ReservePositions(events []ReserveEvent) []ReservePosition {
	perCur := map[string]*ReservePosition{}
	order := []string{}
	get := func(cur string) *ReservePosition {
		p, ok := perCur[cur]
		if !ok {
			p = &ReservePosition{Currency: cur}
			perCur[cur] = p
			order = append(order, cur)
		}
		return p
	}
	openHolds := map[string]float64{} // holdEventID → remaining
	for _, e := range events {
		pos := get(e.Currency)
		switch e.EventType {
		case "hold":
			pos.HeldOpen = round2(pos.HeldOpen + e.Amount)
			if e.PayoutID != "" {
				openHolds[e.PayoutID+"|"+e.Currency] += e.Amount
			}
		case "release":
			key := e.PayoutID + "|" + e.Currency
			remaining, ok := openHolds[key]
			if !ok || remaining+0.01 < e.Amount {
				pos.Issues = append(pos.Issues, fmt.Sprintf("release_without_open_hold:%s", e.PayoutID))
			} else {
				openHolds[key] = round2(remaining - e.Amount)
			}
			pos.Released = round2(pos.Released + e.Amount)
			pos.HeldOpen = round2(pos.HeldOpen - e.Amount)
		}
	}
	sort.Strings(order)
	out := make([]ReservePosition, 0, len(order))
	for _, cur := range order {
		p := perCur[cur]
		p.NetFrozen = round2(p.HeldOpen)
		out = append(out, *p)
	}
	return out
}

func withinTransit(payoutDate, valueDate time.Time, maxDays int) bool {
	if valueDate.Before(payoutDate.Truncate(24*time.Hour)) {
		return false
	}
	days := valueDate.Sub(payoutDate.Truncate(24 * time.Hour)).Hours() / 24
	return days <= float64(maxDays)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
