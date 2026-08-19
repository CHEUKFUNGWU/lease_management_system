package memo

import (
	"encoding/json"
	"strings"
	"testing"
)

func pf(v float64) *float64 { return &v }

func TestBridgeComputesDeltasAndMarksMissingSides(t *testing.T) {
	bridge := Bridge(
		map[string]*float64{"rev@2026-01": pf(120), "rev@2026-02": pf(130), "left_only@2026-01": pf(5)},
		map[string]*float64{"rev@2026-01": pf(100), "rev@2026-02": pf(135), "right_only@2026-01": pf(7)},
	)
	if d := bridge["rev@2026-01"]; d.Delta == nil || *d.Delta != 20 {
		t.Fatalf("rev@2026-01 delta = %v, want 20", d.Delta)
	}
	if d := bridge["rev@2026-02"]; d.Delta == nil || *d.Delta != -5 {
		t.Fatalf("rev@2026-02 delta = %v, want -5", d.Delta)
	}
	if !bridge["left_only@2026-01"].SideMissing || bridge["left_only@2026-01"].Delta != nil {
		t.Fatalf("left-only key must carry side_missing and no delta: %+v", bridge["left_only@2026-01"])
	}
	if !bridge["right_only@2026-01"].SideMissing || bridge["right_only@2026-01"].Left != nil {
		t.Fatalf("right-only key must carry side_missing and nil left: %+v", bridge["right_only@2026-01"])
	}
}

func TestComposeFourLayersAndExplicitResidual(t *testing.T) {
	bridge := Bridge(
		map[string]*float64{"rev@2026-01": pf(120), "cost@2026-01": pf(60)},
		map[string]*float64{"rev@2026-01": pf(100), "cost@2026-01": pf(50)},
	)
	value, err := Compose(
		json.RawMessage(`{"period":"2026-01","data_version":"d7"}`),
		json.RawMessage(`[{"tool_call_id":"tcall-1"}]`),
		bridge,
		[]NarrativeItem{
			{Key: "rev@2026-01", Explanation: "客流回升带动收入", AmountCovered: pf(20)},
			{Key: "cost@2026-01", Explanation: "人工成本部分被效率优化抵消", AmountCovered: pf(6)},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	// 四层齐全。
	var sys map[string]any
	if err := json.Unmarshal(value.SystemFacts, &sys); err != nil || sys["data_version"] != "d7" {
		t.Fatalf("layer 1 mangled: %s", value.SystemFacts)
	}
	var det Deterministic
	if err := json.Unmarshal(value.DeterministicCalculations, &det); err != nil {
		t.Fatal(err)
	}
	if len(det.Deltas) != 2 || det.Residuals["cost@2026-01"] != 4 {
		t.Fatalf("deterministic layer must keep deltas and residuals: %+v", det)
	}
	// cost delta = 10；叙述只认领 6 → 残差 4 必须显式保留（不抹平）。
	if det.Residuals["cost@2026-01"] != 4 {
		t.Fatalf("residual = %v, want 4", det.Residuals["cost@2026-01"])
	}
	if det.Residuals["rev@2026-01"] != 0 {
		t.Fatalf("fully covered key must have residual 0, got %v", det.Residuals["rev@2026-01"])
	}
	if string(value.HumanInputs) != `{}` {
		t.Fatalf("layer 3 must start as empty human input placeholder: %s", value.HumanInputs)
	}
	var narratives []NarrativeItem
	if err := json.Unmarshal(value.AINarrative, &narratives); err != nil || len(narratives) != 2 {
		t.Fatalf("layer 4 mangled: %s", value.AINarrative)
	}
}

func TestComposeRejectsUnbridgedNarrative(t *testing.T) {
	bridge := Bridge(map[string]*float64{"rev@2026-01": pf(120)}, map[string]*float64{"rev@2026-01": pf(100)})
	_, err := Compose(json.RawMessage(`{}`), json.RawMessage(`[]`), bridge, []NarrativeItem{
		{Key: "made_up@2026-01", Explanation: "AI 无法解释确定性服务没算出的数字", AmountCovered: pf(1)},
	})
	if err == nil || !strings.Contains(err.Error(), "not a bridged delta") {
		t.Fatalf("unbridged narrative must be rejected, got %v", err)
	}
}

func TestComposeRejectsOverClaiming(t *testing.T) {
	bridge := Bridge(map[string]*float64{"rev@2026-01": pf(120)}, map[string]*float64{"rev@2026-01": pf(100)})
	_, err := Compose(json.RawMessage(`{}`), json.RawMessage(`[]`), bridge, []NarrativeItem{
		{Key: "rev@2026-01", Explanation: "把整条差异和背后的猜测都归因", AmountCovered: pf(30)},
	})
	if err == nil || !strings.Contains(err.Error(), "beyond its delta") {
		t.Fatalf("over-claiming narrative must be rejected, got %v", err)
	}
}

func TestComposeRejectsMissingSideNarrative(t *testing.T) {
	bridge := Bridge(map[string]*float64{"rev@2026-01": pf(120)}, map[string]*float64{})
	_, err := Compose(json.RawMessage(`{}`), json.RawMessage(`[]`), bridge, []NarrativeItem{
		{Key: "rev@2026-01", Explanation: "缺一边的数字不能解释", AmountCovered: pf(0)},
	})
	if err == nil || !strings.Contains(err.Error(), "not computable") {
		t.Fatalf("narrative on a side-missing delta must be rejected, got %v", err)
	}
}

func TestComposeNarrativeRequiresAmount(t *testing.T) {
	bridge := Bridge(map[string]*float64{"rev@2026-01": pf(120)}, map[string]*float64{"rev@2026-01": pf(100)})
	_, err := Compose(json.RawMessage(`{}`), json.RawMessage(`[]`), bridge, []NarrativeItem{
		{Key: "rev@2026-01", Explanation: "没有认领金额的解释不是归因"},
	})
	if err == nil || !strings.Contains(err.Error(), "amount_covered") {
		t.Fatalf("amount-less narrative must be rejected, got %v", err)
	}
}
