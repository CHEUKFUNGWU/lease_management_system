package agentrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/llm"
)

func testLLMPlanner(t *testing.T, body string) *PlannerLLM {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("planner must use the in-process client's chat-completions endpoint, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewLLMPlanner(client)
}

// W4-3: the planner now runs in-process through internal/llm. The old
// HTTPPlanner round-trip semantics (descriptors + structured plan decoding)
// is preserved — the mandated substitution must be proven to work, not just
// the old hop be gone.
func TestLLMPlannerSendsDescriptorsAndDecodesStructuredPlan(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"call_id\":\"call-1\",\"tool_name\":\"lease.contract.get\",\"tool_version\":\"v1\",\"arguments\":{\"contract_id\":\"c-1\"}}]}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	calls, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect the lease",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(calls) != 1 || calls[0].ToolName != "lease.contract.get" || calls[0].ToolVersion != "v1" {
		t.Fatalf("calls=%+v", calls)
	}
	if string(calls[0].Arguments) != `{"contract_id":"c-1"}` {
		t.Fatalf("arguments=%s", calls[0].Arguments)
	}
}

// Old http_planner_test.go:43 semantics preserved: an empty plan must fail
// closed.
func TestLLMPlannerRejectsEmptyPlan(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[]}"}}]}`)
	_, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err == nil {
		t.Fatal("empty planner plan must fail closed")
	}
}

// W4-3 reverse test: the model naming an unregistered tool must be rejected —
// the whitelist is a security boundary, not a formatting nicety.
func TestLLMPlannerRejectsUnregisteredTool(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"tool_name\":\"lease.contract.inject\",\"tool_version\":\"v1\",\"arguments\":{}}]}"}}]}`)
	_, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err == nil {
		t.Fatal("unregistered tool name must reject the plan")
	}
	if !strings.Contains(err.Error(), "outside the discovered descriptor set") {
		t.Fatalf("error must name the whitelist violation: %v", err)
	}
}

func TestLLMPlannerRejectsWrongVersionSuffix(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"tool_name\":\"lease.contract.get@v1\",\"tool_version\":\"v1\",\"arguments\":{}}]}"}}]}`)
	_, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err == nil {
		t.Fatal("model placing the @version suffix in tool_name must be rejected")
	}
}

func TestLLMPlannerRejectsNonObjectArguments(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"tool_name\":\"lease.contract.get\",\"tool_version\":\"v1\",\"arguments\":\"SELECT 1\"}]}"}}]}`)
	_, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err == nil {
		t.Fatal("non-object arguments must be rejected (no raw command pass-through)")
	}
}

func TestLLMPlannerPersistsVersionedUsageThroughRecorder(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"tool_name\":\"lease.contract.get\",\"tool_version\":\"v1\",\"arguments\":{}}]}"}}],"usage":{"schema_version":"llm-usage.v1","provider":"deepseek","model":"deepseek-v4-flash","input_tokens":100,"output_tokens":25,"total_tokens":125,"cost_usd":0.000007,"cost_currency":"USD","cost_status":"measured"}}`)
	calls, usage, err := planner.PlanWithUsage(context.Background(), PlanRequest{
		RunID: "run-usage", Message: "inspect",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	if usage == nil || usage.Provider != "deepseek" || usage.Model != "deepseek-v4-flash" || usage.TotalTokens == nil || *usage.TotalTokens != 125 {
		t.Fatalf("usage=%+v", usage)
	}
	if usage.CostMicros == nil || *usage.CostMicros != 7 || usage.CostStatus != "measured" || usage.PricingVersion != "unconfigured" {
		t.Fatalf("cost usage=%+v", usage)
	}
}

func TestParseJSONObjectAcceptsFencedOutput(t *testing.T) {
	for _, input := range []string{
		"```json\n{\"tool_calls\":[{\"tool_name\":\"a\"}]}\n```",
		"```\n{\"tool_calls\":[{\"tool_name\":\"a\"}]}\n```",
		"  {\"tool_calls\":[{\"tool_name\":\"a\"}]}  ",
		"prefix text {\"tool_calls\":[{\"tool_name\":\"a\"}]} suffix",
	} {
		val, err := parseJSONObject(input)
		if err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
		obj, ok := val.(map[string]any)
		if !ok {
			t.Fatalf("input %q: parsed %#v", input, val)
		}
		if _, ok := obj["tool_calls"]; !ok {
			t.Fatalf("input %q: missing tool_calls in %#v", input, obj)
		}
	}
	// JSON output is mandatory — non-JSON must fail.
	if _, err := parseJSONObject("I cannot do that"); err == nil {
		t.Fatal("non-JSON output must fail closed")
	}
}

func TestLLMPlannerEmptyToolsFailsClosed(t *testing.T) {
	planner := testLLMPlanner(t, `{"choices":[{"index":0,"message":{"role":"assistant","content":"{}"}}]}`)
	if _, err := planner.Plan(context.Background(), PlanRequest{RunID: "r", Message: "hi"}); err == nil {
		t.Fatal("planning without discovered tools must fail closed")
	}
}

func TestPlannerUsageFromMetadata(t *testing.T) {
	tokens := 42
	u := llm.UsageMetadata{
		SchemaVersion: "llm-usage.v1", Provider: "deepseek", Model: "deepseek-v4-flash",
		InputTokens: &tokens, OutputTokens: &tokens, TotalTokens: &tokens,
		PricingVersion: "unconfigured", PricingSource: "configured_settings",
		CostCurrency: "USD", CostStatus: "unavailable",
	}
	got := plannerUsageFromMetadata(u)
	if got.SchemaVersion != "llm-usage.v1" || got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("mapping dropped fields: %+v", got)
	}
	if got.InputTokens == nil || *got.InputTokens != 42 || got.PricingVersion != "unconfigured" || got.CostStatus != "unavailable" {
		t.Fatalf("mapping dropped token/cost state: %+v", got)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "cost_micros") {
		t.Fatalf("unavailable cost must stay absent from the JSON record: %s", b)
	}
}