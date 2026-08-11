package agentrunner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

func TestHTTPGatewayUsesDescriptorDiscoveryCapabilityAndToolHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer jwt-token" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/agent/tools":
			if request.URL.Query().Get("run_id") != "run-1" || request.URL.Query().Get("skill_id") != "contract_review" {
				t.Fatalf("query=%v", request.URL.Query())
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"tools": []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1"}}})
		case "/api/v1/agent/capabilities":
			_ = json.NewEncoder(writer).Encode(map[string]string{"capability_token": "cap-token"})
		case "/api/v1/agent/tools/execute":
			if request.Header.Get("X-Agent-Capability") != "cap-token" {
				t.Fatalf("capability=%q", request.Header.Get("X-Agent-Capability"))
			}
			_ = json.NewEncoder(writer).Encode(agenttools.ToolResult{Status: agenttools.StatusCompleted})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	descriptors, err := gateway.Describe(context.Background(), agenttools.ToolFilter{SkillID: "contract_review"}, "run-1")
	if err != nil || len(descriptors) != 1 {
		t.Fatalf("descriptors=%+v err=%v", descriptors, err)
	}
	capability, err := gateway.IssueCapability(context.Background(), CapabilityRequest{RunID: "run-1", SkillID: "contract_review", SkillVersion: "v1", AllowedTools: []string{"lease.contract.get"}})
	if err != nil || capability != "cap-token" {
		t.Fatalf("capability=%q err=%v", capability, err)
	}
	if _, err := gateway.Execute(context.Background(), agenttools.ToolCall{CallID: "call-1", RunID: "run-1", SkillID: "contract_review", SkillVersion: "v1", ToolName: "lease.contract.get", ToolVersion: "v1", Arguments: []byte(`{}`)}, capability); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPGatewayPersistsAndLoadsCheckpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case http.MethodPost + " /api/v1/agent/runs/run-1/checkpoint":
			var body struct {
				Checkpoint Checkpoint `json:"checkpoint"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.Checkpoint.RunID != "run-1" {
				t.Fatalf("checkpoint body=%+v err=%v", body, err)
			}
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"saved":true}`))
		case http.MethodGet + " /api/v1/agent/runs/run-1/checkpoint":
			_ = json.NewEncoder(writer).Encode(map[string]any{"run_id": "run-1", "checkpoint": Checkpoint{RunID: "run-1", NextIndex: 2, Status: "paused"}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	checkpoint := Checkpoint{RunID: "run-1", NextIndex: 1, Status: "running"}
	if err := gateway.Save(context.Background(), checkpoint); err != nil {
		t.Fatal(err)
	}
	loaded, err := gateway.Load(context.Background(), "run-1")
	if err != nil || loaded.NextIndex != 2 || loaded.Status != "paused" {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestHTTPGatewayWorkerLeaseLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case http.MethodPost + " /api/v1/agent/runs/claim":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"run":         map[string]any{"id": "run-1", "session_id": "session-1", "status": "running"},
				"lease_token": "lease-1", "worker_id": "worker-a", "lease_seconds": 60,
			})
		case http.MethodPost + " /api/v1/agent/runs/run-1/lease/heartbeat":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"accepted":true}`))
		case http.MethodPost + " /api/v1/agent/runs/run-1/lease/release":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"accepted":true}`))
		case http.MethodPost + " /api/v1/agent/runs/recover-leases":
			_, _ = writer.Write([]byte(`{"recovered":3}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	lease, err := gateway.ClaimRun(context.Background(), "worker-a", 60)
	if err != nil || lease.Run.ID != "run-1" || lease.LeaseToken != "lease-1" {
		t.Fatalf("lease=%+v err=%v", lease, err)
	}
	if err := gateway.HeartbeatRunLease(context.Background(), "run-1", "worker-a", "lease-1", 60); err != nil {
		t.Fatal(err)
	}
	if err := gateway.ReleaseRunLease(context.Background(), "run-1", "worker-a", "lease-1", true); err != nil {
		t.Fatal(err)
	}
	if recovered, err := gateway.RecoverRunLeases(context.Background()); err != nil || recovered != 3 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
}

func TestHTTPGatewayClaimRunDistinguishesEmptyQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/runs/claim" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	if _, err := gateway.ClaimRun(context.Background(), "worker-a", 60); err != ErrNoQueuedRun {
		t.Fatalf("empty queue error = %v, want %v", err, ErrNoQueuedRun)
	}
}

func TestHTTPGatewayLoadsQueuedRunInstructionFromMessageStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/runs/run-1/events" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"events":[{"event_type":"message_start","payload":{"message":"从队列审阅合同"}}]}`))
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	instruction, err := gateway.LoadRunInstruction(context.Background(), "run-1")
	if err != nil || instruction != "从队列审阅合同" {
		t.Fatalf("instruction=%q err=%v", instruction, err)
	}
}

func TestHTTPGatewayWorkerLeaseHeadersAreAttachedToRunDataPlane(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Agent-Worker-ID") != "worker-a" || request.Header.Get("X-Agent-Lease-Token") != "lease-a" {
			t.Fatalf("worker headers=%q/%q", request.Header.Get("X-Agent-Worker-ID"), request.Header.Get("X-Agent-Lease-Token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"events":[{"event_type":"message_start","payload":{"message":"lease-bound"}}]}`))
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client()).WithWorkerLease("worker-a", "lease-a")
	instruction, err := gateway.LoadRunInstruction(context.Background(), "run-1")
	if err != nil || instruction != "lease-bound" {
		t.Fatalf("instruction=%q err=%v", instruction, err)
	}
}

func TestHTTPGatewaySubscribesToLeaseBoundRunEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/runs/run-1/stream" {
			http.NotFound(writer, request)
			return
		}
		if request.URL.Query().Get("after_sequence") != "1" {
			t.Fatalf("after_sequence=%q", request.URL.Query().Get("after_sequence"))
		}
		if request.Header.Get("X-Agent-Worker-ID") != "worker-a" || request.Header.Get("X-Agent-Lease-Token") != "lease-a" {
			t.Fatalf("worker headers=%q/%q", request.Header.Get("X-Agent-Worker-ID"), request.Header.Get("X-Agent-Lease-Token"))
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Fatal("test server does not support flushing")
		}
		_, _ = writer.Write([]byte("event: run_event\ndata: {\"event\":{\"run_id\":\"run-1\",\"sequence_no\":2,\"event_type\":\"run_steer\",\"payload\":{\"payload\":{\"instruction\":\"use official report\"}}}}\n\n"))
		flusher.Flush()
		_, _ = writer.Write([]byte("event: run_event\ndata: {\"event\":{\"run_id\":\"run-1\",\"sequence_no\":3,\"event_type\":\"run_cancelled\",\"is_terminal\":true,\"payload\":{}}}\n\n"))
		flusher.Flush()
		_, _ = writer.Write([]byte("event: complete\ndata: {\"run_id\":\"run-1\",\"last_sequence\":3}\n\n"))
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client()).WithWorkerLease("worker-a", "lease-a")
	subscription, err := gateway.SubscribeRunEvents(context.Background(), "run-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Close()
	var events []RunEvent
	deadline := time.After(2 * time.Second)
	for len(events) < 2 {
		select {
		case event, ok := <-subscription.Events:
			if !ok {
				t.Fatalf("event stream closed after %d events", len(events))
			}
			events = append(events, event)
		case err, ok := <-subscription.Errors:
			if ok && err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("timed out waiting for SSE events")
		}
	}
	if events[0].EventType != "run_steer" || events[1].EventType != "run_cancelled" || !strings.Contains(string(events[0].Payload), "official report") {
		t.Fatalf("events=%+v", events)
	}
}
