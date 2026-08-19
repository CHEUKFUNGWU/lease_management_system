package finmodel

// SM8 的克制形态（D-S8）：不建服务包——集团视图是授权法人集合上若干 run
// 的按期间汇总。三条纪律全部类型化：跨币种只在显式汇率版本折算后才可加总；
// 无权限法人显式呈现为 unauthorized（不泄露、不静默省略）；汇总与逐法人
// 明细勾稽（ties_out），差异容差 0.05（S5-2 验收）。

import "math"

// GroupRunInput is one member run's summarized lines.
type GroupRunInput struct {
	RunID         string
	LegalEntityID string
	Authorized    bool
	Currency      string
	Periods       []string
	// Lines: row_key@period → value in the run's own currency (missing =
	// absent entry).
	Lines map[string]*float64
	// Translated-view state (S5-2): when an exchange_rate_version is
	// applied the caller fills TranslatedLines with per-cell
	// reporting-currency values and TranslatedCurrency with the target.
	// Summarize refuses to produce cross-currency totals that are not
	// backed by translated values on every authorized member.
	ExchangeRateVersion string
	TranslatedCurrency  string
	TranslatedLines     map[string]*float64
	// Note is a member-level caveat (e.g. missing_exchange_rate); "" = none.
	Note string
}

// GroupSummary is the aggregate view. The default view is the original-
// currency partition (Totals only when every authorized member shares one
// currency); the translated second view additionally carries the rate
// version banner.
type GroupSummary struct {
	Periods             []string            `json:"periods"`
	CurrencyPartitions  map[string][]string `json:"currency_partitions,omitempty"` // currency → run ids (authorized members)
	Members             []GroupMember       `json:"members"`
	Totals              map[string]*float64 `json:"totals,omitempty"` // row@period → sum (reporting currency)
	TotalsCurrency      string              `json:"totals_currency,omitempty"`
	ExchangeRateVersion string              `json:"exchange_rate_version,omitempty"`
	ExchangeRateType    string              `json:"exchange_rate_type,omitempty"`
	TiesOut             bool                `json:"ties_out"`
	Note                string              `json:"note,omitempty"`
}

// GroupMember is one input as seen by the viewer.
type GroupMember struct {
	RunID         string `json:"run_id"`
	LegalEntityID string `json:"legal_entity_id"`
	Authorized    bool   `json:"authorized"`
	Currency      string `json:"currency"`
	Note          string `json:"note,omitempty"` // "unauthorized" | "missing_exchange_rate" | "" — never silent
}

const (
	tieOutTolerance = 0.05
	groupNote       = "管理口径汇总，未抵销内部交易"
)

// Summarize aggregates members that the caller is authorized to see.
//   - No totals promise, no totals: with mixed currencies and no
//     exchange_rate_version every cross-currency total is withheld (T14).
//   - With an exchange_rate_version, totals are computed from per-member
//     translated values; a member without translated values degrades the
//     whole translated view (fail-closed, S5-2).
//   - ties_out verifies the displayed contract: the rounded group total
//     must equal the sum of the rounded member contributions within
//     tieOutTolerance; a rounding drift beyond it fails the summary.
func Summarize(members []GroupRunInput, exchangeRateVersion string) (GroupSummary, error) {
	summary := GroupSummary{
		Members:             []GroupMember{},
		CurrencyPartitions:  map[string][]string{},
		Totals:              map[string]*float64{},
		ExchangeRateVersion: exchangeRateVersion,
	}
	authorized := make([]GroupRunInput, 0, len(members))
	currencies := map[string]bool{}
	var everyPeriods []string

	for _, m := range members {
		if !m.Authorized {
			summary.Members = append(summary.Members, GroupMember{
				RunID: m.RunID, LegalEntityID: m.LegalEntityID,
				Authorized: false, Currency: m.Currency, Note: "unauthorized",
			})
			continue
		}
		cur, ok := normalizeCurrency(m.Currency)
		if !ok {
			cur = m.Currency
		}
		currencies[cur] = true
		summary.CurrencyPartitions[cur] = append(summary.CurrencyPartitions[cur], m.RunID)
		authorized = append(authorized, m)
		if len(m.Periods) > len(everyPeriods) {
			everyPeriods = append([]string(nil), m.Periods...)
		}
		summary.Members = append(summary.Members, GroupMember{
			RunID: m.RunID, LegalEntityID: m.LegalEntityID,
			Authorized: true, Currency: cur, Note: m.Note,
		})
	}
	summary.Periods = everyPeriods
	if len(authorized) == 0 {
		summary.TiesOut = true
		return summary, nil
	}

	singleCurrency := len(currencies) == 1
	reporting := ""
	for cur := range currencies {
		reporting = cur
	}

	if !singleCurrency && exchangeRateVersion == "" {
		summary.Note = "跨币种汇总缺 exchange_rate_version：不产生任何跨币种合计数字（T14）"
		summary.TiesOut = true
		return summary, nil
	}

	translated := !singleCurrency
	if !translated {
		// 单一币种：折算为恒等，直接使用原币值。
		summary.TotalsCurrency = reporting
	}
	if translated {
		missing := []string{}
		for _, m := range authorized {
			if m.TranslatedLines == nil || m.TranslatedCurrency == "" {
				missing = append(missing, m.RunID)
			}
		}
		if len(missing) > 0 {
			summary.Note = "折算缺失：以下成员在汇率版本 " + exchangeRateVersion + " 下无折算值 —— " + joinRunIDs(missing)
			summary.TiesOut = false // 请求的折算视图未能构建
			return summary, nil
		}
		summary.TotalsCurrency = authorized[0].TranslatedCurrency
	}

	// Totals: sum per row@period over authorized members; a missing member
	// value drops the total (缺失不填 0, D-S4).
	union := map[string]bool{}
	values := make(map[string]map[string]*float64, len(authorized))
	for _, m := range authorized {
		source := m.Lines
		if translated {
			source = m.TranslatedLines
		}
		values[m.RunID] = source
		for key := range source {
			union[key] = true
		}
	}
	tiesOut := true
	for key := range union {
		raw := 0.0
		complete := true
		shown := 0.0
		for _, m := range authorized {
			v, ok := values[m.RunID][key]
			if !ok {
				complete = false
				break
			}
			raw += *v
			shown += round2(*v)
		}
		if !complete {
			continue // 合计行缺失，显式留空
		}
		total := round2(raw)
		summary.Totals[key] = &total
		// 展示口径勾稽：显示的总计必须等于各成员显示值之和（容差 0.05）。
		if math.Abs(shown-total) > tieOutTolerance {
			tiesOut = false
		}
	}
	summary.TiesOut = tiesOut

	summary.Note = groupNote
	if translated {
		summary.Note += "；折算视图基于 exchange_rate_version=" + exchangeRateVersion
	}
	return summary, nil
}

// normalizeCurrency trims the currency code and uppercases it; ok=false
// means the code is empty, in which case callers fall back to the raw value.
func normalizeCurrency(currency string) (string, bool) {
	cur := ""
	for _, r := range currency {
		if r == ' ' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			cur += string(r - 'a' + 'A')
			continue
		}
		cur += string(r)
	}
	return cur, cur != ""
}

func joinRunIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
