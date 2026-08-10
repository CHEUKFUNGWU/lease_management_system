package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRunCompletesHTTPGatewayPlannerVerticalSlice(t *testing.T) {
	var mu sync.Mutex
	var eventTypes []string
	var usageEvent bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/plan" && request.Header.Get("Authorization") != "Bearer jwt-token" {
			t.Fatalf("authorization=%q path=%s", request.Header.Get("Authorization"), request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/sessions":
			_, _ = writer.Write([]byte(`{"session":{"id":"session-1"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/runs":
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1","session_id":"session-1","status":"queued"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/agent/tools":
			_, _ = writer.Write([]byte(`{"tools":[{"name":"lease.contract.get","version":"v1","description":"read contract","level":"read","read_only":true,"permissions":[{"resource":"contracts","action":"read"}]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/capabilities":
			_, _ = writer.Write([]byte(`{"capability_token":"cap-1"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/plan":
			_, _ = writer.Write([]byte(`{"tool_calls":[{"call_id":"call-1","tool_name":"lease.contract.get","tool_version":"v1","arguments":{"contract_id":"contract-1"}}],"model":"deepseek/test","usage":{"schema_version":"llm-usage.v1","provider":"deepseek","model":"deepseek/test","input_tokens":10,"output_tokens":5,"total_tokens":15,"cost_micros":3,"cost_currency":"USD","cost_status":"calculated","pricing_version":"test-v1"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/tools/execute":
			_, _ = writer.Write([]byte(`{"call_id":"call-1","status":"completed","review":{"required":false},"data":{"contract_id":"contract-1"}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/stream"):
			writer.Header().Set("Content-Type", "text/event-stream")
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/capabilities/revoke"):
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/events"):
			var body struct {
				EventType string          `json:"type"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			mu.Lock()
			eventTypes = append(eventTypes, body.EventType)
			if body.EventType == "planner_usage" {
				var usage map[string]any
				if json.Unmarshal(body.Payload, &usage) == nil && usage["pricing_version"] == "test-v1" {
					usageEvent = true
				}
			}
			mu.Unlock()
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/checkpoint"):
			writer.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	err := run([]string{
		"--base-url", server.URL,
		"--planner-url", server.URL,
		"--planner-token", "planner-token",
		"--token", "jwt-token",
		"--message", "读取合同",
		"--skill", "contract_review",
		"--skill-version", "v1",
		"--deadline", "2s",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !usageEvent {
		t.Fatalf("planner usage was not persisted; events=%v", eventTypes)
	}
	if len(eventTypes) == 0 {
		t.Fatal("expected durable run events")
	}
}
