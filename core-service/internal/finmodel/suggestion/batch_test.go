package suggestion

import (
	"encoding/json"
	"testing"
)

func fp(v float64) *float64 { return &v }

func TestPlanBatchBlocksAndUnsuggestable(t *testing.T) {
	// 收入 12 个月全区间历史（含季节指数）；费用只有 4 个月历史；
	// 营运资本/CAPEX/税务仅靠登记；tax_rate 未登记 → 无法建议。
	season := make([]*float64, 12)
	for i := range season {
		season[i] = fp(1.0)
	}
	season[5] = fp(1.2) // 目标 2026-06：季节指数 1.2

	out := PlanBatch(BatchInput{
		LegalEntityID: "LE-1", ToolCallID: "tcall-b1", TargetPeriod: "2026-06",
		Historical: map[string][]*float64{
			"revenue":    {fp(90), fp(92), fp(91), fp(95), fp(93), fp(96), fp(94), fp(97), fp(98), fp(99), fp(100), fp(101)},
			"labor_cost": {fp(30), fp(31), fp(32), fp(33)},
			"fixed_rent": {fp(20), nil, fp(20), fp(20)}, // 3 个非空月，刚好过门槛
		},
		Seasonality: map[string][]*float64{"revenue": season},
		Registered: map[string]json.RawMessage{
			"capex":              json.RawMessage(`20000`),
			"wc_days_receivable": json.RawMessage(`45`),
		},
	})

	draftsByArea := map[Area][]SuggestionDraft{}
	for _, block := range out.Blocks {
		draftsByArea[block.Area] = block.Drafts
	}
	unsug := map[string]UnSuggestableItem{}
	for _, item := range out.UnSuggestable {
		unsug[item.AssumptionKey] = item
	}

	// 收入：run-rate = 最近三个月均值 (99+100+101)/3 = 100 → ×1.2 = 120。
	if drafts := draftsByArea[AreaRevenue]; len(drafts) < 1 || drafts[0].AssumptionKey != "revenue" {
		t.Fatalf("revenue draft missing: %+v", drafts)
	} else {
		var v float64
		if err := json.Unmarshal(drafts[0].Value, &v); err != nil || v != 120 {
			t.Fatalf("revenue value = %s (want 120), err=%v", drafts[0].Value, err)
		}
		// 覆盖 12/12 + 季节可用：0.5 + 0.4×1 = 0.9。
		if drafts[0].Confidence != 0.9 {
			t.Fatalf("revenue confidence = %v, want 0.9", drafts[0].Confidence)
		}
	}

	// 费用：4 个月历史 → 置信度 0.5+0.4×(4/12)。
	expense := draftsByArea[AreaExpense]
	if len(expense) != 2 {
		t.Fatalf("expense drafts = %+v, want labor_cost + fixed_rent", expense)
	}
	foundLabor := false
	for _, d := range expense {
		if d.AssumptionKey == "labor_cost" {
			foundLabor = true
			want := 0.5 + 0.4*(4.0/12.0)
			if d.Confidence != round3(want) {
				t.Fatalf("labor confidence = %v, want %v", d.Confidence, round3(want))
			}
		}
	}
	if !foundLabor {
		t.Fatal("labor_cost draft missing")
	}

	// 登记透传：capex / wc_days_receivable 出现；tax_rate 进无建议清单且理由明确。
	if _, ok := draftsByArea[AreaCapex]; !ok {
		t.Fatalf("capex block missing: %+v", out.Blocks)
	}
	if _, ok := draftsByArea[AreaWorkingCapital]; !ok {
		t.Fatalf("working capital block missing")
	}
	item, ok := unsug["tax_rate"]
	if !ok || item.Reason != ReasonNoRegisteredBasis {
		t.Fatalf("tax_rate must be unsuggestable with no_registered_basis, got %+v", item)
	}
	// 无历史的新店收入键：不足覆盖 → 无法建议，绝不编造。
	item, ok = unsug["gross_profit"]
	if !ok || item.Reason != ReasonInsufficientCoverage {
		t.Fatalf("gross_profit without history must be flagged insufficient_coverage, got %+v", item)
	}
	// variable_rent / non_lease_cost / other_opex：完全无历史 → 全部进无建议清单。
	for _, key := range []string{"variable_rent", "non_lease_cost", "other_opex", "wc_days_payable", "wc_days_inventory"} {
		if _, ok := unsug[key]; !ok {
			t.Fatalf("%s must appear in unsuggestable", key)
		}
	}
}

func TestPlanBatchDeterministicAndBasisValid(t *testing.T) {
	in := BatchInput{
		LegalEntityID: "LE-1", ToolCallID: "tcall-b2", TargetPeriod: "2026-01",
		Historical: map[string][]*float64{"revenue": {fp(10), fp(12), fp(11)}},
		Registered: map[string]json.RawMessage{"tax_rate": json.RawMessage(`0.25`)},
	}
	first := PlanBatch(in)
	second := PlanBatch(in)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("PlanBatch must be deterministic:\n%s\n%s", a, b)
	}
	// 每条草稿必须自校验通过（含 basis 非空），否则写入路径会拒绝。
	for _, block := range first.Blocks {
		for _, d := range block.Drafts {
			if d.SourceTag != "ai_suggestion" {
				t.Fatalf("source tag = %q", d.SourceTag)
			}
			if err := d.Validate(); err != nil {
				t.Fatalf("draft %s fails its own contract: %v", d.AssumptionKey, err)
			}
		}
	}
}

func TestPlanBatchPartialSeasonalityDowngradesConfidence(t *testing.T) {
	partial := make([]*float64, 12)
	for i := range partial {
		partial[i] = fp(1.0)
	}
	partial[3] = nil // 缺两个月（<11 非空）→ 整序列不使用
	partial[9] = nil
	out := PlanBatch(BatchInput{
		LegalEntityID: "LE-1", ToolCallID: "tcall-b3", TargetPeriod: "2026-06",
		Historical:  map[string][]*float64{"revenue": {fp(90), fp(92), fp(91), fp(95), fp(93), fp(96), fp(94), fp(97), fp(98), fp(99), fp(100), fp(101)}},
		Seasonality: map[string][]*float64{"revenue": partial},
	})
	for _, block := range out.Blocks {
		if block.Area != AreaRevenue {
			continue
		}
		var v float64
		_ = json.Unmarshal(block.Drafts[0].Value, &v)
		if v != 100 {
			t.Fatalf("partial seasonality must not be applied, value = %v", v)
		}
		if block.Drafts[0].Confidence != 0.9-confidenceSeasonalGap {
			t.Fatalf("partial seasonality must downgrade confidence, got %v", block.Drafts[0].Confidence)
		}
		return
	}
	t.Fatal("revenue block missing")
}

func TestSaveBatchEmptyIsNoop(t *testing.T) {
	store := NewMemoryStore("LE-1")
	out := PlanBatch(BatchInput{LegalEntityID: "LE-1", ToolCallID: "x", TargetPeriod: "2026-01",
		Historical: map[string][]*float64{}, Registered: map[string]json.RawMessage{}})
	ids, err := SaveBatch(t.Context(), store, BatchInput{LegalEntityID: "LE-1"}, out, "k")
	if err != nil || len(ids) != 0 {
		t.Fatalf("all-unsuggestable batch must save nothing, got %v/%v", ids, err)
	}
}
