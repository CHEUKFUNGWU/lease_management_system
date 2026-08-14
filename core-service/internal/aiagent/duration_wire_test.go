package aiagent

import (
	"encoding/json"
	"strings"
	"testing"
)

// FIX-001 F2: a call that never ran has no duration_ms on the wire at all —
// absent is the only honest representation, never a fabricated zero.
func TestAgentToolCallDurationAbsentWhenNotMeasured(t *testing.T) {
	planned := AgentToolCall{Tool: "lease.contract_batch_parser", Status: "pending", InputSummary: "等待上传文件", OutputSummary: "生成合同草稿"}
	raw, err := json.Marshal(planned)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "duration_ms") {
		t.Fatalf("unexecuted call must not carry duration_ms, got %s", raw)
	}

	// The same struct with a measured duration emits the number for ToolChip.
	ms := int64(320)
	executed := AgentToolCall{Tool: "retail.operating_pulse.read", Status: "completed", DurationMs: &ms}
	raw, err = json.Marshal(executed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"duration_ms":320`) {
		t.Fatalf("executed call must emit duration_ms as a number, got %s", raw)
	}
}
