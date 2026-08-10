package agentrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

func TestHTTPPlannerSendsDescriptorsAndDecodesStructuredPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/plan" || request.Header.Get("Authorization") != "Bearer planner-token" {
			t.Fatalf("path=%s authorization=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload struct {
			Message string                      `json:"message"`
			Tools   []agenttools.ToolDescriptor `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Message != "inspect the lease" || len(payload.Tools) != 1 {
			t.Fatalf("payload=%+v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tool_calls":[{"call_id":"call-1","tool_name":"lease.contract.get","tool_version":"v1","arguments":{"contract_id":"c-1"}}],"model":"deepseek/test"}`))
	}))
	defer server.Close()

	planner := NewHTTPPlanner(server.URL, "planner-token", server.Client())
	calls, err := planner.Plan(context.Background(), PlanRequest{
		RunID: "run-1", Message: "inspect the lease",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err != nil || len(calls) != 1 || calls[0].ToolName != "lease.contract.get" {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
}

func TestHTTPPlannerRejectsEmptyPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"tool_calls":[]}`))
	}))
	defer server.Close()
	_, err := NewHTTPPlanner(server.URL, "", server.Client()).Plan(context.Background(), PlanRequest{RunID: "run-1", Message: "inspect"})
	if err == nil {
		t.Fatal("empty planner plan must fail closed")
	}
}

func TestHTTPPlannerPersistsVersionedUsageThroughRecorder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tool_calls":[{"tool_name":"lease.contract.get","tool_version":"v1","arguments":{}}],"model":"deepseek/test","usage":{"schema_version":"llm-usage.v1","provider":"deepseek","model":"deepseek/test","input_tokens":100,"output_tokens":25,"total_tokens":125,"cost_micros":7,"cost_currency":"USD","cost_status":"calculated","pricing_version":"2026-08"}}`))
	}))
	defer server.Close()

	var recorded PlannerUsage
	var recordedRun string
	planner := NewHTTPPlanner(server.URL, "", server.Client()).WithUsageRecorder(func(_ context.Context, runID string, usage PlannerUsage) error {
		recordedRun = runID
		recorded = usage
		return nil
	})
	calls, err := planner.Plan(context.Background(), PlanRequest{RunID: "run-usage", Message: "inspect", Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1"}}})
	if err != nil || len(calls) != 1 {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	if recordedRun != "run-usage" || recorded.Provider != "deepseek" || recorded.CostMicros == nil || *recorded.CostMicros != 7 || recorded.PricingVersion != "2026-08" {
		t.Fatalf("recorded usage=%+v run=%q", recorded, recordedRun)
	}
}
