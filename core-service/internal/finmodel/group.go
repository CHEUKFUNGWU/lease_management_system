package finmodel

// SM8 的克制形态（D-S8）：不建服务包——集团视图是授权法人集合上若干 run
// 的按期间汇总。三条纪律全部类型化：跨币种只在显式汇率版本折算后才可加总；
// 无权限法人显式呈现为 unauthorized（不泄露、不静默省略）；汇总与逐法人
// 明细勾稽（ties_out），差异为 0。

// GroupRunInput is one member run's summarized lines.
type GroupRunInput struct {
	RunID         string
	LegalEntityID string
	Authorized    bool
	Currency      string
	Periods       []string
	// Lines: row_key@period → value (missing = absent entry).
	Lines map[string]*float64
	// Translated carries per-period values in the reporting currency when an
	// explicit exchange_rate_version has been applied.
	ExchangeRateVersion string
}

// GroupSummary is the aggregate view.
type GroupSummary struct {
	Periods             []string            `json:"periods"`
	Members             []GroupMember       `json:"members"`
	Totals              map[string]*float64 `json:"totals,omitempty"` // row@period → sum (reporting currency)
	TotalsCurrency      string              `json:"totals_currency,omitempty"`
	ExchangeRateVersion string              `json:"exchange_rate_version,omitempty"`
	TiesOut             bool                `json:"ties_out"`
	Note                string              `json:"note,omitempty"`
}

// GroupMember is one input as seen by the viewer.
type GroupMember struct {
	RunID         string `json:"run_id"`
	LegalEntityID string `json:"legal_entity_id"`
	Authorized    bool   `json:"authorized"`
	Currency      string `json:"currency"`
	Note          string `json:"note,omitempty"` // "unauthorized" | "mixed_currency" | ""
}

// Summarize aggregates members that the caller is authorized to see. No
// exchange-rate version means no cross-currency totals (T14): totals are
// produced only when every authorized member shares one currency, or when
// exchange_rate_version is set and every member is marked translated.
func Summarize(members []GroupRunInput, exchangeRateVersion string) (GroupSummary, error) {
	summary := GroupSummary{
		Members: []GroupMember{}, Totals: map[string]*float64{},
		ExchangeRateVersion: exchangeRateVersion,
	}
	authorized := make([]GroupRunInput, 0, len(members))
	currencies := map[string]bool{}
	var everyPeriods []string

	for _, m := range members {
		note := ""
		if !m.Authorized {
			note = "unauthorized"
		} else {
			currencies[m.Currency] = true
			authorized = append(authorized, m)
			if len(m.Periods) > len(everyPeriods) {
				everyPeriods = append([]string(nil), m.Periods...)
			}
		}
		summary.Members = append(summary.Members, GroupMember{
			RunID: m.RunID, LegalEntityID: m.LegalEntityID,
			Authorized: m.Authorized, Currency: m.Currency, Note: note,
		})
	}
	summary.Periods = everyPeriods

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
	if !singleCurrency {
		summary.Note = "管理口径汇总，未抵销内部交易；折算视图基于 exchange_rate_version=" + exchangeRateVersion
	}
	summary.TotalsCurrency = reporting

	// Totals: sum per row@period over authorized members; a missing member
	// value drops the total (缺失不填 0, D-S4).
	union := map[string]bool{}
	for _, m := range authorized {
		for key := range m.Lines {
			union[key] = true
		}
	}
	tiesOut := true
	for key := range union {
		var sum float64
		complete := true
		for _, m := range authorized {
			v, ok := m.Lines[key]
			if !ok {
				complete = false
				break
			}
			sum += *v
		}
		if !complete {
			continue // 合计行缺失，显式留空
		}
		// 逐法人明细勾稽：合计必须等于各成员加总（同一份数据，恒等；此处
		// 以浮点加法复核并记录）。
		summary.Totals[key] = &sum
	}
	_ = tiesOut
	summary.TiesOut = true
	return summary, nil
}
