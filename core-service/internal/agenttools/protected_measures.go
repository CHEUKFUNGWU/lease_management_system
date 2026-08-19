package agenttools

import "strings"

// ProtectedMeasure is one measurement semantics that may never be produced by
// an exploratory (Tier B) path. The list is finalized by ADR-0025 §2; the
// dated decision-log entry recording the finalization lives in
// docs/CodebaseDesign_AI阶段0产物底座与W1内核抽取_模块深化.md §4 M2.
// Changing this list after release is a breaking change and requires an ADR.
type ProtectedMeasure struct {
	ID string `json:"id"`
	ZH string `json:"zh"`
	EN string `json:"en"`
}

// protectedMeasures is the finalized list of ten protected measurements.
var protectedMeasures = []ProtectedMeasure{
	{ID: "lease_liability", ZH: "租赁负债", EN: "lease liability"},
	{ID: "rou_asset", ZH: "使用权资产", EN: "rou asset"},
	{ID: "discount_rate_applied", ZH: "实际采用的折现率", EN: "discount rate"},
	{ID: "interest_expense", ZH: "利息费用", EN: "interest expense"},
	{ID: "rou_depreciation", ZH: "使用权资产折旧", EN: "rou depreciation"},
	{ID: "amortization_schedule_row", ZH: "摊销表", EN: "amortization schedule"},
	{ID: "journal_amount", ZH: "会计分录金额", EN: "journal entry"},
	{ID: "disclosure_maturity_bucket", ZH: "披露到期分析分档", EN: "maturity bucket"},
	{ID: "weighted_average_discount_rate", ZH: "加权平均折现率", EN: "weighted average discount rate"},
	{ID: "remeasurement_adjustment", ZH: "重计量调整额", EN: "remeasurement"},
}

// lexicalProbes backs the artifact-time lint: when a cell label mentions a
// protected measure but the cell carries no measure_id, the probe still
// catches it. The lint records these as suspected bypass events.
var lexicalProbes = map[string]string{
	// measure id -> probe keywords (already normalized: lowercased, de-spaced)
	"lease_liability":                "租赁负债|leaseliability",
	"rou_asset":                      "使用权资产|rouasset|rou资产|right-of-useasset|rightofuseasset",
	"discount_rate_applied":          "折现率|discountrate",
	"interest_expense":               "利息费用|利息支出|interestexpense",
	"rou_depreciation":               "使用权资产折旧|roudepreciation|rightofusedepreciation",
	"amortization_schedule_row":      "摊销表|amortizationschedule|摊销计划",
	"journal_amount":                 "分录|journalentry",
	"disclosure_maturity_bucket":     "到期分析|披露到期|maturitybucket",
	"weighted_average_discount_rate": "加权平均折现率|weightedaveragediscountrate",
	"remeasurement_adjustment":       "重计量|remeasurement",
}

// ProtectedMeasureIDs returns the finalized list of protected measure IDs.
func ProtectedMeasureIDs() []string {
	out := make([]string, len(protectedMeasures))
	for i, m := range protectedMeasures {
		out[i] = m.ID
	}
	return out
}

// IsProtected reports whether a measure id is on the protected list.
func IsProtected(measureID string) bool {
	for _, m := range protectedMeasures {
		if m.ID == measureID {
			return true
		}
	}
	return false
}

// MatchLexicalProbe normalizes a cell label and reports which protected
// measure it mentions, if any. The probe is the fallback for cells that lack
// a measure_id; it must stay conservative (specific phrases only) to avoid
// flagging ordinary operating labels. When several probes hit, the longest
// probe wins — "加权平均折现率" must resolve to
// weighted_average_discount_rate, not the generic 折现率.
func MatchLexicalProbe(label string) (string, bool) {
	norm := normalizeProbeText(label)
	if norm == "" {
		return "", false
	}
	bestID, bestLen := "", -1
	for id, probes := range lexicalProbes {
		for _, p := range strings.Split(probes, "|") {
			if strings.Contains(norm, p) && len(p) > bestLen {
				bestID, bestLen = id, len(p)
			}
		}
	}
	return bestID, bestID != ""
}

// RouteDecision is the request-time routing outcome. A request needing
// protected measures may only be satisfied by a certified (Tier A) tool; any
// other path is rejected — never downgraded to Tier B.
type RouteDecision struct {
	// Tier is "A" when the request may proceed through certified tools.
	Tier string `json:"tier"`
	// Protected lists the protected measures the request touches.
	Protected []string `json:"protected"`
	// RejectReason explains what is missing when the request is rejected.
	RejectReason string `json:"reject_reason,omitempty"`
}

// RouteMeasures applies the request-time rule from the working-paper design
// §4.2 step 2: if the request touches protected measures and no certified
// tool can satisfy them, reject with a helpful reason; never route to Tier B.
func RouteMeasures(measureIDs []string, certifiedSatisfiable bool) RouteDecision {
	var protected []string
	for _, id := range measureIDs {
		if IsProtected(id) {
			protected = append(protected, id)
		}
	}
	if len(protected) == 0 {
		return RouteDecision{Tier: "A"}
	}
	if certifiedSatisfiable {
		return RouteDecision{Tier: "A", Protected: protected}
	}
	return RouteDecision{
		Tier:         "Reject",
		Protected:    protected,
		RejectReason: "该需求涉及受保护度量（" + strings.Join(protected, "、") + "），只能由确定性引擎计算。请补充引擎所需输入后重试。",
	}
}

// LintCell applies the artifact-time rule for a single cell (design §4.3
// checks 3 and 4). basis is the cell's provenance basis as a string so this
// package stays free of the workingpaper dependency. It returns violation
// codes: "protected_measure_exploratory" or "lexical_probe_exploratory".
func LintCell(measureID, label, basis string) []string {
	var out []string
	exploratory := basis == "Exploratory"
	if IsProtected(measureID) && exploratory {
		out = append(out, "protected_measure_exploratory")
	}
	if measureID == "" {
		if _, hit := MatchLexicalProbe(label); hit && exploratory {
			out = append(out, "lexical_probe_exploratory")
		}
	}
	return out
}

func normalizeProbeText(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		// Full-width ASCII to half-width so "ＲＯＵ" matches "rou".
		if r >= 0xFF01 && r <= 0xFF5E {
			return r - 0xFEE0
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), "")
	return s
}
