// Package opening is SM4: the three opening-balance gates (self-balancing,
// cross-period merge stability, lease balances against the IFRS 16 engine).
// It is a pure value validator with two real callers — the import wizard and
// the model engine's run path (D-S7): failures are values, not errors, and
// the judge lives in exactly one place.
package opening

import "fmt"

// Standard lines a period balance is mapped onto (merge rules versioned).
const (
	LineCash               = "cash"
	LineReceivables        = "ar"
	LineInventory          = "inventory"
	LineOtherCurrentAssets = "other_current_assets"
	LinePPE                = "ppe"
	LineROUAsset           = "rou_asset"
	LinePayables           = "ap"
	LineLeaseLiability     = "lease_liability"
	LineOtherCurrentLiabs  = "other_current_liabilities"
	LineBorrowings         = "borrowings"
	LineShareCapital       = "share_capital"
	LineRetainedEarnings   = "retained_earnings"
)

var assetLines = []string{LineCash, LineReceivables, LineInventory, LineOtherCurrentAssets, LinePPE, LineROUAsset}
var liabilityLines = []string{LinePayables, LineLeaseLiability, LineOtherCurrentLiabs, LineBorrowings}
var equityLines = []string{LineShareCapital, LineRetainedEarnings}

// LineSign is the storage convention of a standard line: assets store
// debit-positive (=+1), liabilities and equity store credit-positive
// (=−1 relative to debit-minus-credit). Importers use it to fold raw
// debit/credit rows into gate-1-consistent standard lines.
func LineSign(line string) float64 {
	for _, candidate := range liabilityLines {
		if candidate == line {
			return -1
		}
	}
	for _, candidate := range equityLines {
		if candidate == line {
			return -1
		}
	}
	return 1
}

// PeriodBalance is one historical period's standardized opening balance with
// its merge mapping snapshot (the mapping applied to external accounts).
type PeriodBalance struct {
	Period  string             `json:"period"`
	Lines   map[string]float64 `json:"lines"`
	Mapping map[string]string  `json:"mapping"` // external account key → standard line
}

// OpeningBalance is the imported opening set, periods ascending.
type OpeningBalance struct {
	LegalEntityID string          `json:"legal_entity_id"`
	Currency      string          `json:"currency"`
	Periods       []PeriodBalance `json:"periods"`
}

// ContractBalance is one contract's lease position, either imported (gate 3
// left side) or read from the engine (gate 3 right side).
type ContractBalance struct {
	ContractID     string  `json:"contract_id"`
	LeaseLiability float64 `json:"lease_liability"`
	ROUAsset       float64 `json:"rou_asset"`
}

// MergePolicy carries the versioned merge rule set the import declared.
type MergePolicy struct {
	Version string `json:"version"`
}

// ValidateInput carries everything the three gates need.
type ValidateInput struct {
	Balance  OpeningBalance
	LeaseRef []ContractBalance // imported per-contract opening balances
	Engine   []ContractBalance // engine per-contract balances, same period
	Policy   MergePolicy
}

// GateFailure is one named failure with a diff amount.
type GateFailure struct {
	Gate   string  `json:"gate"` // "1" | "2" | "3"
	Period string  `json:"period,omitempty"`
	Detail string  `json:"detail"`
	Diff   float64 `json:"diff"`
}

// Validate runs the three gates. An empty result means all passed — the
// caller (import wizard or engine) decides how to render failures.
func Validate(in ValidateInput) []GateFailure {
	var out []GateFailure
	out = append(out, gateSelfBalance(in.Balance)...)
	out = append(out, gateMergeStability(in.Balance, in.Policy)...)
	out = append(out, gateLeaseReconciliation(in.LeaseRef, in.Engine, in.Balance)...)
	return out
}

// Gate 1: every historical period must balance itself (assets = liabilities
// + equity, tolerance ±0.01).
func gateSelfBalance(b OpeningBalance) []GateFailure {
	var out []GateFailure
	for _, p := range b.Periods {
		var assets, liabilities, equity float64
		for _, line := range assetLines {
			assets += p.Lines[line]
		}
		for _, line := range liabilityLines {
			liabilities += p.Lines[line]
		}
		for _, line := range equityLines {
			equity += p.Lines[line]
		}
		diff := assets - (liabilities + equity)
		if diff > 0.01 || diff < -0.01 {
			out = append(out, GateFailure{
				Gate: "1", Period: p.Period, Diff: diff,
				Detail: fmt.Sprintf("期初表自身不平衡：资产 %.2f ≠ 负债+权益 %.2f（差额 %.2f，容差 ±0.01）", assets, liabilities+equity, diff),
			})
		}
	}
	return out
}

// Gate 2: the merge mapping must be identical across periods — the same
// external account may not map to different standard lines in different
// periods.
func gateMergeStability(b OpeningBalance, policy MergePolicy) []GateFailure {
	var out []GateFailure
	if len(b.Periods) < 2 || policy.Version == "" {
		if policy.Version == "" {
			out = append(out, GateFailure{Gate: "2", Detail: "归并规则必须声明版本（MergePolicy.Version 为空）"})
		}
		return out
	}
	base := b.Periods[0].Mapping
	for _, p := range b.Periods[1:] {
		for account, line := range base {
			if got := p.Mapping[account]; got != line {
				out = append(out, GateFailure{
					Gate: "2", Period: p.Period, Diff: 0,
					Detail: fmt.Sprintf("科目 %q 在 %s 归并到 %q，与基准期归并 %q 不一致", account, p.Period, got, line),
				})
			}
		}
		for account, line := range p.Mapping {
			if _, ok := base[account]; !ok {
				out = append(out, GateFailure{
					Gate: "2", Period: p.Period,
					Detail: fmt.Sprintf("科目 %q 只在 %s 出现（归并规则跨期不一致：新科目归并到 %q）", account, p.Period, line),
				})
			}
		}
	}
	return out
}

// Gate 3: imported per-contract lease liability and ROU must equal the
// engine's per-contract balances exactly (the engine is the single
// authority — D-S3).
func gateLeaseReconciliation(ref, engine []ContractBalance, b OpeningBalance) []GateFailure {
	var out []GateFailure
	engineByID := map[string]ContractBalance{}
	for _, e := range engine {
		engineByID[e.ContractID] = e
	}
	period := ""
	if len(b.Periods) > 0 {
		period = b.Periods[0].Period
	}
	for _, r := range ref {
		e, ok := engineByID[r.ContractID]
		switch {
		case !ok:
			out = append(out, GateFailure{Gate: "3", Period: period, Detail: fmt.Sprintf("合同 %s 在计量引擎中无同期余额，导入的期初租赁余额无法勾稽", r.ContractID)})
		case e.LeaseLiability != r.LeaseLiability:
			out = append(out, GateFailure{Gate: "3", Period: period, Diff: r.LeaseLiability - e.LeaseLiability, Detail: fmt.Sprintf("合同 %s 期初租赁负债 %.2f ≠ 引擎 %.2f", r.ContractID, r.LeaseLiability, e.LeaseLiability)})
		case e.ROUAsset != r.ROUAsset:
			out = append(out, GateFailure{Gate: "3", Period: period, Diff: r.ROUAsset - e.ROUAsset, Detail: fmt.Sprintf("合同 %s 期初 ROU %.2f ≠ 引擎 %.2f", r.ContractID, r.ROUAsset, e.ROUAsset)})
		}
	}
	return out
}
