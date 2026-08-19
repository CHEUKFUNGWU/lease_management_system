// Package memo is S4-4: the model-difference memo assembled from the four
// governed layers. Layer 2 (deterministic_calculations) is produced here by
// the difference bridge — AI never computes an amount; it only narrates
// amounts the bridge already produced (AI-002). Compose enforces that
// structurally: every narrative item must point at a bridged delta and may
// not cover more than the delta; anything left over is recorded as an
// explicit residual, never papered over.
package memo

import (
	"encoding/json"
	"fmt"
	"math"
)

// DeltaItem is one bridged difference for a row@period key. A nil side is
// a missing value, never a zero (D-S4).
type DeltaItem struct {
	Key         string   `json:"key"`
	Left        *float64 `json:"left,omitempty"`
	Right       *float64 `json:"right,omitempty"`
	Delta       *float64 `json:"delta,omitempty"`        // left − right; nil when a side is missing
	SideMissing bool     `json:"side_missing,omitempty"` // one side absent: delta not computable
}

// Bridge computes the deterministic difference set over two run line maps
// keyed by row_key@period. Keys present on only one side appear with the
// other side nil and SideMissing set — the memo can state the gap without
// inventing the missing number.
func Bridge(left, right map[string]*float64) map[string]DeltaItem {
	out := map[string]DeltaItem{}
	for key, lv := range left {
		rv, ok := right[key]
		totalDelta := DeltaItem{Key: key, Left: lv, Right: rv}
		if !ok {
			totalDelta.Right = nil
			totalDelta.SideMissing = true
			out[key] = totalDelta
			continue
		}
		d := *lv - *rv
		totalDelta.Delta = &d
		out[key] = totalDelta
	}
	for key, rv := range right {
		if _, ok := left[key]; !ok {
			out[key] = DeltaItem{Key: key, Right: rv, SideMissing: true}
		}
	}
	return out
}

// NarrativeItem is one AI explanation tied to a bridged delta.
type NarrativeItem struct {
	Key           string   `json:"key"`
	Explanation   string   `json:"explanation"`
	AmountCovered *float64 `json:"amount_covered"` // signed share of the delta this narrative owns
}

const narrativeTolerance = 1e-9

// Memo is the four-layer value written to fpna_decision_memos.
type Memo struct {
	SystemFacts               json.RawMessage `json:"system_facts"`
	DeterministicCalculations json.RawMessage `json:"deterministic_calculations"`
	HumanInputs               json.RawMessage `json:"human_inputs"`
	AINarrative               json.RawMessage `json:"ai_narrative"`
	SourceReferences          json.RawMessage `json:"source_references"`
}

// Deterministic is the shape of layer 2: the bridged deltas plus the
// per-key residual — delta minus everything the narrative covered.
type Deterministic struct {
	Deltas    map[string]DeltaItem `json:"deltas"`
	Residuals map[string]float64   `json:"residuals"`
}

// Compose assembles the four layers. Fail-closed rules:
//   - a narrative key not present in the bridge is rejected (AI cannot
//     explain an amount the deterministic service did not produce);
//   - |covered| beyond |delta| is rejected (no over-claiming);
//   - the residual per key is always computed — an unexplained portion
//     stays visible (残差显式), never silently zeroed.
func Compose(systemFacts, sourceRefs json.RawMessage, bridge map[string]DeltaItem, narratives []NarrativeItem) (Memo, error) {
	covered := map[string]float64{}
	for _, n := range narratives {
		item, ok := bridge[n.Key]
		if !ok {
			return Memo{}, fmt.Errorf("memo: narrative references %q which is not a bridged delta", n.Key)
		}
		if item.Delta == nil {
			return Memo{}, fmt.Errorf("memo: narrative references %q whose delta is not computable (side missing)", n.Key)
		}
		if n.AmountCovered == nil {
			return Memo{}, fmt.Errorf("memo: narrative for %q must state its amount_covered", n.Key)
		}
		covered[n.Key] += *n.AmountCovered
		if math.Abs(covered[n.Key]) > math.Abs(*item.Delta)+narrativeTolerance {
			return Memo{}, fmt.Errorf("memo: narrative for %q covers %v, beyond its delta %v", n.Key, covered[n.Key], *item.Delta)
		}
	}

	residuals := map[string]float64{}
	for key, item := range bridge {
		if item.Delta == nil {
			continue // 缺一边：残差无从计算，SideMissing 已在 deltas 中显式
		}
		residuals[key] = *item.Delta - covered[key]
	}
	detRaw, err := json.Marshal(Deterministic{Deltas: bridge, Residuals: residuals})
	if err != nil {
		return Memo{}, err
	}
	narrRaw, err := json.Marshal(narratives)
	if err != nil {
		return Memo{}, err
	}
	if len(systemFacts) == 0 {
		systemFacts = json.RawMessage(`{}`)
	}
	if len(sourceRefs) == 0 {
		sourceRefs = json.RawMessage(`[]`)
	}
	return Memo{
		SystemFacts:               systemFacts,
		DeterministicCalculations: detRaw,
		HumanInputs:               json.RawMessage(`{}`),
		AINarrative:               narrRaw,
		SourceReferences:          sourceRefs,
	}, nil
}
