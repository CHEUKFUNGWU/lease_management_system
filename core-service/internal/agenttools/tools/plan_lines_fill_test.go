package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
)

type fakePlanReader struct{ data []byte }

func (f fakePlanReader) ReadObject(_ context.Context, _ string) ([]byte, error) { return f.data, nil }

const planCSV = `store_code,period,currency,revenue,gross_profit,labor_cost
s-001,2026-08,CNY,120000,48000,15000
s-002,2026-09,CNY,120000,48000,15000
s-001,2026-13,CNY,120000,48000,15000
s-003,2026-10,CNY,-5,48000,15000
`

func planExecContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "u1", Role: "editor", Permissions: []string{"*:*"}},
		RunID:     "run-plan",
		SkillID:   "fpna_copilot",
	})
}

func planFillCall(def agenttools.ToolDefinition, callID string, args json.RawMessage) (agenttools.ToolResult, error) {
	return def.Handler(planExecContext(), agenttools.ToolCall{
		CallID: callID, RunID: "run-plan", ToolName: def.Descriptor.Name, ToolVersion: "v1",
		Arguments: args, IdempotencyKey: callID,
	})
}

func validPlanArgs() json.RawMessage {
	return json.RawMessage(`{"file_id":"f1","object_name":"budget.csv","content_type":"text/csv","name":"BUDGET 2026H2","version_type":"budget","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12","currency":"cny"}`)
}

// TestPlanLinesFillProducesPageFill locks the P0-B contract: valid CSV in, a
// Fill out whose payload carries ONLY the human envelope fields and whose
// suggestions hold the parsed lines — machine values never reach Payload.
func TestPlanLinesFillProducesPageFill(t *testing.T) {
	def := NewPlanLinesFillDefinition(fakePlanReader{data: []byte(planCSV)})
	result, err := planFillCall(def, "call-1", validPlanArgs())
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if result.Status != agenttools.StatusCompleted || !result.Review.Required {
		t.Fatalf("status=%s review=%+v", result.Status, result.Review)
	}
	fill, ok := result.Data.(map[string]any)["page_fill"].(*pagefill.Fill)
	if !ok {
		t.Fatalf("page_fill type = %T", result.Data.(map[string]any)["page_fill"])
	}
	if fill.TargetPage != "retail-data-import" || fill.TargetAPI != "POST /api/v1/fpna/plan-versions/import" {
		t.Fatalf("target = %s / %s", fill.TargetPage, fill.TargetAPI)
	}
	if fill.DeepLink != "/retail-data-import?plan_fill=call-1&section=plan" {
		t.Fatalf("deep link = %s", fill.DeepLink)
	}
	payload := fill.FillPayload()
	for _, key := range []string{"name", "version_type", "as_of_period", "from_period", "to_period"} {
		if _, present := payload[key]; !present {
			t.Fatalf("payload missing %s: %+v", key, payload)
		}
	}
	if payload["currency"] != "CNY" {
		t.Fatalf("currency must be normalised to upper: %v", payload["currency"])
	}
	if _, present := payload["is_official"]; present {
		t.Fatalf("is_official must only enter payload when explicitly requested: %+v", payload)
	}
	if _, present := fill.FillPayload()["first_row"]; present {
		t.Fatalf("machine-extracted rows must never enter the payload: %+v", payload)
	}
	// 有效 = 2（s-001/08、s-002/09）；s-001/2026-13 期间形状非法跳过，
	// s-003 负数跳过。
	summary, ok := fill.Suggestions["plan_summary"].Value.(map[string]any)
	if !ok {
		t.Fatalf("plan_summary type = %T", fill.Suggestions["plan_summary"].Value)
	}
	if summary["valid_rows"] != 2 || summary["skipped_rows"] != 2 {
		t.Fatalf("valid=%v skipped=%v", summary["valid_rows"], summary["skipped_rows"])
	}
	if summary["total_revenue"] != 240000.0 {
		t.Fatalf("total revenue = %v", summary["total_revenue"])
	}
	if summary["store_count"] != 2 {
		t.Fatalf("store count = %v", summary["store_count"])
	}
	first, ok := fill.Suggestions["first_row"].Value.(map[string]any)
	if !ok || first["store_code"] != "s-001" || first["period"] != "2026-08" {
		t.Fatalf("first_row = %+v", fill.Suggestions["first_row"].Value)
	}
}

func TestPlanLinesFillExplicitOfficialEntersPayload(t *testing.T) {
	def := NewPlanLinesFillDefinition(fakePlanReader{data: []byte(planCSV)})
	args := json.RawMessage(`{"file_id":"f2","object_name":"budget.csv","content_type":"text/csv","name":"B1","version_type":"forecast","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12","is_official":true}`)
	result, err := planFillCall(def, "call-2", args)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	fill := result.Data.(map[string]any)["page_fill"].(*pagefill.Fill)
	if fill.FillPayload()["is_official"] != "true" {
		t.Fatalf("explicit is_official must reach payload: %+v", fill.FillPayload())
	}
}

func TestPlanLinesFillRejectsBadInput(t *testing.T) {
	goodReader := NewPlanLinesFillDefinition(fakePlanReader{data: []byte(planCSV)})
	// 版本范围倒置是具名 Gap
	_, err := planFillCall(goodReader, "call-3", json.RawMessage(`{"file_id":"f3","object_name":"o.csv","content_type":"text/csv","name":"B","version_type":"budget","as_of_period":"2026-08","from_period":"2026-12","to_period":"2026-08"}`))
	if err == nil || !strings.Contains(err.Error(), "period_range_invalid") {
		t.Fatalf("inverted range must name the gap, got %v", err)
	}
	// version_type 枚举外拒绝
	_, err = planFillCall(goodReader, "call-4", json.RawMessage(`{"file_id":"f4","object_name":"o.csv","content_type":"text/csv","name":"B","version_type":"wishful","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12"}`))
	if err == nil || !strings.Contains(err.Error(), "version_type") {
		t.Fatalf("bad version_type must be refused, got %v", err)
	}
	// 未知字段拒绝（严格解码）
	_, err = planFillCall(goodReader, "call-5", json.RawMessage(`{"file_id":"f5","object_name":"o.csv","content_type":"text/csv","name":"B","version_type":"budget","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12","hacker":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "invalid plan lines fill arguments") {
		t.Fatalf("unknown field must be refused, got %v", err)
	}
	// 缺必需列拒绝并列出表头
	badReader := NewPlanLinesFillDefinition(fakePlanReader{data: []byte("foo,bar\nx,y\n")})
	_, err = planFillCall(badReader, "call-6", json.RawMessage(`{"file_id":"f6","object_name":"o.csv","content_type":"text/csv","name":"B","version_type":"budget","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12"}`))
	if err == nil || !strings.Contains(err.Error(), "store_code") {
		t.Fatalf("missing-column refusal must name the required columns, got %v", err)
	}
	// 全坏行拒绝而不是编造空预填
	garbage := NewPlanLinesFillDefinition(fakePlanReader{data: []byte("store_code,period\ns-1,2026-13\ns-2,2026-14\n")})
	_, err = planFillCall(garbage, "call-7", json.RawMessage(`{"file_id":"f7","object_name":"o.csv","content_type":"text/csv","name":"B","version_type":"budget","as_of_period":"2026-08","from_period":"2026-08","to_period":"2026-12"}`))
	if err == nil || !strings.Contains(err.Error(), "no valid plan lines") {
		t.Fatalf("all-invalid file must refuse honestly, got %v", err)
	}
}

func TestPlanLinesFillRequiresReader(t *testing.T) {
	def := NewPlanLinesFillDefinition(nil)
	_, err := planFillCall(def, "call-8", validPlanArgs())
	if err == nil || !strings.Contains(err.Error(), "reader is not wired") {
		t.Fatalf("unwired reader must refuse honestly, got %v", err)
	}
}
