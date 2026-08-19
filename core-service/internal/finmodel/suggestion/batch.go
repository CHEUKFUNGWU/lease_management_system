package suggestion

// S4-3 批量初稿填数：对空白模型一次性给出按块组织的整版假设草稿。引擎是
// 纯函数——给定历史序列、季节指数与已登记假设，产出分块草稿与「无建议项」
// 清单；没有历史或没有登记依据的键绝不编造，只会带着原因进入
// UnSuggestable。「历史 run-rate + 季节性 + 已登记假设」的优先级与置信度
// 公式在这里固定下来，任何调用方（工具、测试、未来 UI）共用同一套规则。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Area is one confirmable block of the batch (PRD S4-3: 收入/费用/营运
// 资本/CAPEX/税务).
type Area string

const (
	AreaRevenue        Area = "revenue"
	AreaExpense        Area = "expense"
	AreaWorkingCapital Area = "working_capital"
	AreaCapex          Area = "capex"
	AreaTax            Area = "tax"
)

var areaOrder = []Area{AreaRevenue, AreaExpense, AreaWorkingCapital, AreaCapex, AreaTax}

var areaLabels = map[Area]string{
	AreaRevenue:        "收入",
	AreaExpense:        "费用",
	AreaWorkingCapital: "营运资本",
	AreaCapex:          "CAPEX",
	AreaTax:            "税务",
}

// metricSpec fixes how one standard assumption key can be suggested.
type metricSpec struct {
	Area Area
	Unit string
	// runRate: value = average of the last three non-nil historical months,
	// optionally seasonally adjusted for the target month.
	RunRate bool
	// registeredOnly passes through an approved assumption value; without
	// one the key is unsuggestable (no other basis exists).
	RegisteredOnly bool
}

// registry is the standard key set. Run-rate keys must not be fabricated on
// sparse history (minimum 3 non-nil months), and lumpy or policy keys
// (capex, working-capital days, tax rate) only ever pass through registered
// approved values.
var registry = map[string]metricSpec{
	"revenue":            {Area: AreaRevenue, Unit: "per_month", RunRate: true},
	"gross_profit":       {Area: AreaRevenue, Unit: "per_month", RunRate: true},
	"labor_cost":         {Area: AreaExpense, Unit: "per_month", RunRate: true},
	"fixed_rent":         {Area: AreaExpense, Unit: "per_month", RunRate: true},
	"variable_rent":      {Area: AreaExpense, Unit: "per_month", RunRate: true},
	"non_lease_cost":     {Area: AreaExpense, Unit: "per_month", RunRate: true},
	"other_opex":         {Area: AreaExpense, Unit: "per_month", RunRate: true},
	"wc_days_receivable": {Area: AreaWorkingCapital, Unit: "days", RegisteredOnly: true},
	"wc_days_payable":    {Area: AreaWorkingCapital, Unit: "days", RegisteredOnly: true},
	"wc_days_inventory":  {Area: AreaWorkingCapital, Unit: "days", RegisteredOnly: true},
	"capex":              {Area: AreaCapex, Unit: "per_month", RegisteredOnly: true},
	"tax_rate":           {Area: AreaTax, Unit: "rate", RegisteredOnly: true},
}

// UnSuggestable reasons are enum-like: every missing input is named, not
// glossed (不编造).
const (
	ReasonNoHistory            = "no_history"
	ReasonInsufficientCoverage = "insufficient_coverage"
	ReasonNoRegisteredBasis    = "no_registered_basis"
)

// BatchInput is the evidence set the engine works from. Historical and
// Seasonality series are ordered oldest→newest with nil for absent months.
type BatchInput struct {
	LegalEntityID string
	// ToolCallID anchors every basis ref to the generating tool call (I2).
	ToolCallID string
	// TargetPeriod is the first forecast month (YYYY-MM); seasonality is
	// applied with this month's index.
	TargetPeriod string
	Historical   map[string][]*float64
	// Seasonality: 12 non-nil monthly indices per metric (full year only;
	// partial series is ignored, never guessed).
	Seasonality map[string][]*float64
	// Registered echoes approved assumption values (key → value) read by
	// the caller through its read tools.
	Registered map[string]json.RawMessage
}

// BatchOutput is the grouped result.
type BatchOutput struct {
	Blocks        []BatchBlock        `json:"blocks"`
	UnSuggestable []UnSuggestableItem `json:"unsuggestable"`
}

// BatchBlock is one confirmable area with its drafts.
type BatchBlock struct {
	Area   Area              `json:"area"`
	Label  string            `json:"label"`
	Drafts []SuggestionDraft `json:"drafts"`
}

// UnSuggestableItem names one key the engine refuses to guess.
type UnSuggestableItem struct {
	AssumptionKey string `json:"assumption_key"`
	Category      string `json:"category"`
	Reason        string `json:"reason"`
	Detail        string `json:"detail,omitempty"`
}

const (
	minHistoryMonths = 3
	runRateWindow    = 3
	// confidence base (run-rate) and registered-passthrough strength.
	confidenceRunRateBase = 0.5
	confidenceRunRateGain = 0.4
	confidenceRegistered  = 0.95
	confidenceSeasonalGap = 0.15
)

// PlanBatch computes the grouped draft set. Deterministic: same input ⇒
// same output.
func PlanBatch(in BatchInput) BatchOutput {
	out := BatchOutput{Blocks: []BatchBlock{}, UnSuggestable: []UnSuggestableItem{}}
	byArea := map[Area]*BatchBlock{}
	for _, area := range areaOrder {
		byArea[area] = &BatchBlock{Area: area, Label: areaLabels[area], Drafts: []SuggestionDraft{}}
	}
	keys := make([]string, 0, len(registry))
	for key := range registry {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		spec := registry[key]
		block := byArea[spec.Area]
		switch {
		case spec.RunRate:
			draft, unsug := runRateDraft(key, spec, in)
			if unsug != nil {
				out.UnSuggestable = append(out.UnSuggestable, *unsug)
				continue
			}
			block.Drafts = append(block.Drafts, *draft)
		case spec.RegisteredOnly:
			raw, ok := in.Registered[key]
			if !ok || len(raw) == 0 {
				out.UnSuggestable = append(out.UnSuggestable, UnSuggestableItem{
					AssumptionKey: key, Category: string(spec.Area), Reason: ReasonNoRegisteredBasis,
					Detail: "无已登记（approved）假设值；该键不允许从历史推断",
				})
				continue
			}
			block.Drafts = append(block.Drafts, SuggestionDraft{
				AssumptionKey: key,
				Category:      string(spec.Area),
				Value:         raw,
				Unit:          spec.Unit,
				Basis: []EvidenceRef{{
					ToolCallID: in.ToolCallID,
					Scope:      "fpna_assumption_versions:approved",
					Period:     in.TargetPeriod,
				}},
				Confidence: confidenceRegistered,
				SourceTag:  "ai_suggestion",
			})
		}
	}

	for _, area := range areaOrder {
		if len(byArea[area].Drafts) > 0 {
			out.Blocks = append(out.Blocks, *byArea[area])
		}
	}
	return out
}

// runRateDraft suggests one historical-run-rate value with a
// coverage-derived confidence; seasonality is applied only from a full
// 12-month series. A nil draft and a non-nil reason cannot coexist.
func runRateDraft(key string, spec metricSpec, in BatchInput) (*SuggestionDraft, *UnSuggestableItem) {
	series := in.Historical[key]
	nonNil := 0
	for _, v := range series {
		if v != nil {
			nonNil++
		}
	}
	if nonNil < minHistoryMonths {
		detail := "完全无历史数据"
		if nonNil > 0 {
			detail = fmt.Sprintf("历史覆盖仅 %d 个月（不足 %d）", nonNil, minHistoryMonths)
		}
		return nil, &UnSuggestableItem{
			AssumptionKey: key, Category: string(spec.Area), Reason: ReasonInsufficientCoverage, Detail: detail,
		}
	}
	// run-rate = 最近 min(3, 非空月数) 个非空月的均值（series 越新越靠后）。
	sum := 0.0
	taken := 0
	for i := len(series) - 1; i >= 0 && taken < runRateWindow; i-- {
		if series[i] == nil {
			continue
		}
		sum += *series[i]
		taken++
	}
	value := sum / float64(taken)

	confidence := confidenceRunRateBase + confidenceRunRateGain*math.Min(1, float64(nonNil)/12.0)
	if month, ok := targetMonthIndex(in.TargetPeriod); ok {
		if idx, ok := seasonalIndex(in.Seasonality[key], month); ok {
			value *= idx
		} else if in.Seasonality[key] != nil {
			// 有季节数据但残缺：不使用也不捏造，置信度诚实降级。
			confidence = math.Max(0, confidence-confidenceSeasonalGap)
		}
	}
	return &SuggestionDraft{
		AssumptionKey: key,
		Category:      string(spec.Area),
		Value:         json.RawMessage(fmt.Sprintf("%.4f", value)),
		Unit:          spec.Unit,
		Basis: []EvidenceRef{{
			ToolCallID: in.ToolCallID,
			Scope:      "historical_store_facts",
			Period:     in.TargetPeriod,
		}},
		Confidence: round3(confidence),
		SourceTag:  "ai_suggestion",
	}, nil
}

// targetMonthIndex parses YYYY-MM into a 1-based month; ok=false keeps the
// caller on the unseasonalized run-rate.
func targetMonthIndex(period string) (int, bool) {
	if len(period) != 7 || period[4] != '-' {
		return 0, false
	}
	month := 0
	for _, r := range period[5:] {
		if r < '0' || r > '9' {
			return 0, false
		}
		month = month*10 + int(r-'0')
	}
	if month < 1 || month > 12 {
		return 0, false
	}
	return month, true
}

// seasonalIndex returns the 0-based month index from a full 12-month
// series (>=11 non-nil); ok=false means the series must not be used.
func seasonalIndex(series []*float64, month int) (float64, bool) {
	if len(series) != 12 {
		return 0, false
	}
	nonNil := 0
	for _, v := range series {
		if v != nil {
			nonNil++
		}
	}
	if nonNil < 11 || series[month-1] == nil || *series[month-1] <= 0 {
		return 0, false
	}
	return *series[month-1], true
}

// SaveBatch persists a whole block-set through the store seam in one
// idempotent write; every draft is re-validated by the store itself.
func SaveBatch(ctx context.Context, store Store, in BatchInput, out BatchOutput, idempotencyKey string) ([]string, error) {
	drafts := make([]SuggestionDraft, 0)
	for _, block := range out.Blocks {
		drafts = append(drafts, block.Drafts...)
	}
	if len(drafts) == 0 {
		return nil, nil
	}
	return store.SaveDrafts(ctx, in.LegalEntityID, drafts, idempotencyKey)
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
