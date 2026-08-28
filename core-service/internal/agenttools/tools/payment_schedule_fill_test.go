package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
)

type fakeScheduleReader struct{ data []byte }

func (f fakeScheduleReader) ReadObject(_ context.Context, _ string) ([]byte, error) { return f.data, nil }

const scheduleCSV = `period_start,period_end,due_date,amount,payment_timing
2026-01-01,2026-01-31,2026-01-01,50000.00,prepaid
2026-02-01,2026-02-28,2026-02-01,50000,postpaid
2026-03-01,2026-03-31,2026-03-01,not-a-number,postpaid
,2026-04-30,2026-04-01,50000,postpaid
`

func scheduleExecContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "u1", Role: "editor", Permissions: []string{"*:*"}},
		RunID:     "run-sched",
		SkillID:   "payment_schedule",
	})
}

func scheduleFillCall(def agenttools.ToolDefinition, callID string, args json.RawMessage) (agenttools.ToolResult, error) {
	return def.Handler(scheduleExecContext(), agenttools.ToolCall{
		CallID: callID, RunID: "run-sched", ToolName: def.Descriptor.Name, ToolVersion: "v1",
		Arguments: args, IdempotencyKey: callID,
	})
}

// TestPaymentScheduleFillProducesPageFill locks the P0-B contract: valid CSV
// in, a Fill out whose payload carries ONLY the human envelope fields and
// whose suggestions hold the parsed rows — machine values never reach Payload.
func TestPaymentScheduleFillProducesPageFill(t *testing.T) {
	def := NewPaymentScheduleFillDefinition(fakeScheduleReader{data: []byte(scheduleCSV)})
	args, _ := json.Marshal(map[string]any{
		"file_id": "f1", "object_name": "rent.csv", "content_type": "text/csv",
		"contract_id": "c-77", "currency": "cny",
	})
	result, err := scheduleFillCall(def, "call-1", args)
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
	if fill.TargetPage != "contract-workspace" {
		t.Fatalf("target page = %s", fill.TargetPage)
	}
	if fill.DeepLink != "/contracts/c-77?schedule_fill=call-1" {
		t.Fatalf("deep link = %s", fill.DeepLink)
	}
	payload := fill.FillPayload()
	if payload["contract_id"] != "c-77" {
		t.Fatalf("payload contract_id = %v", payload["contract_id"])
	}
	if payload["currency"] != "CNY" {
		t.Fatalf("currency must be normalised to upper: %v", payload["currency"])
	}
	if _, present := payload["first_row"]; present {
		t.Fatalf("machine-extracted rows must never enter the payload: %+v", payload)
	}
	refs := fill.ExploratoryRefs()
	if len(refs) == 0 {
		t.Fatalf("first_row/schedule_summary suggestions missing")
	}
	summary, ok := fill.Suggestions["schedule_summary"].Value.(map[string]any)
	if !ok {
		t.Fatalf("schedule_summary type = %T", fill.Suggestions["schedule_summary"].Value)
	}
	// 4 data rows: 1 skipped for unparseable amount, 1 skipped for missing
	// period_start is NOT a skip (optional column) — so valid = 3? No: the
	// blank period_start row stays valid, the not-a-number row is skipped.
	if summary["valid_rows"] != 3 || summary["skipped_rows"] != 1 {
		t.Fatalf("valid=%v skipped=%v", summary["valid_rows"], summary["skipped_rows"])
	}
	if summary["total_amount"] != 150000.0 {
		t.Fatalf("total = %v", summary["total_amount"])
	}
	first, ok := fill.Suggestions["first_row"].Value.(scheduleFillRow)
	if !ok || first.DueDate != "2026-01-01" || first.PaymentTiming != "prepaid" {
		t.Fatalf("first_row = %+v (%T)", fill.Suggestions["first_row"].Value, fill.Suggestions["first_row"].Value)
	}
}

func TestPaymentScheduleFillRejectsBadInput(t *testing.T) {
	goodReader := NewPaymentScheduleFillDefinition(fakeScheduleReader{data: []byte(scheduleCSV)})
	// missing contract binding is the named contract_unbound gap
	_, err := scheduleFillCall(goodReader, "call-2", json.RawMessage(`{"file_id":"f2","object_name":"o.csv","content_type":"text/csv"}`))
	if err == nil || !strings.Contains(err.Error(), "contract_unbound") {
		t.Fatalf("missing contract must name the gap, got %v", err)
	}
	// unknown fields rejected (strict decoding)
	_, err = scheduleFillCall(goodReader, "call-3", json.RawMessage(`{"file_id":"f3","object_name":"o.csv","content_type":"text/csv","contract_id":"c1","hacker":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "invalid payment schedule fill arguments") {
		t.Fatalf("unknown field must be refused, got %v", err)
	}
	// a file lacking the required columns must be refused with the column list
	badReader := NewPaymentScheduleFillDefinition(fakeScheduleReader{data: []byte("foo,bar\nx,y\n")})
	_, err = scheduleFillCall(badReader, "call-4", json.RawMessage(`{"file_id":"f4","object_name":"o.csv","content_type":"text/csv","contract_id":"c1"}`))
	if err == nil || !strings.Contains(err.Error(), "due_date") {
		t.Fatalf("missing-column refusal must name the required columns, got %v", err)
	}
	// an all-invalid file refuses instead of fabricating an empty prefill
	garbage := NewPaymentScheduleFillDefinition(fakeScheduleReader{data: []byte("due_date,amount\n2026-01-01,-5\n2026-02-01,0\n")})
	_, err = scheduleFillCall(garbage, "call-5", json.RawMessage(`{"file_id":"f5","object_name":"o.csv","content_type":"text/csv","contract_id":"c1"}`))
	if err == nil || !strings.Contains(err.Error(), "no valid payment schedule rows") {
		t.Fatalf("all-invalid file must refuse honestly, got %v", err)
	}
	// a malformed date skips its row instead of flowing into the suggestions
	mixed := NewPaymentScheduleFillDefinition(fakeScheduleReader{data: []byte("due_date,amount\n2026/01/01,100\n2026-02-01,200\n")})
	result, err := scheduleFillCall(mixed, "call-7", json.RawMessage(`{"file_id":"f7","object_name":"o.csv","content_type":"text/csv","contract_id":"c1"}`))
	if err != nil {
		t.Fatalf("mixed-date handler: %v", err)
	}
	fill := result.Data.(map[string]any)["page_fill"].(*pagefill.Fill)
	summary := fill.Suggestions["schedule_summary"].Value.(map[string]any)
	if summary["valid_rows"] != 1 || summary["skipped_rows"] != 1 {
		t.Fatalf("malformed date must skip the row: %+v", summary)
	}
}

func TestPaymentScheduleFillRequiresReader(t *testing.T) {
	def := NewPaymentScheduleFillDefinition(nil)
	_, err := scheduleFillCall(def, "call-6", json.RawMessage(`{"file_id":"f6","object_name":"o.csv","content_type":"text/csv","contract_id":"c1"}`))
	if err == nil || !strings.Contains(err.Error(), "reader is not wired") {
		t.Fatalf("unwired reader must refuse honestly, got %v", err)
	}
}
