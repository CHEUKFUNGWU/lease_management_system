package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

func TestBearerTokenDoesNotDuplicatePrefix(t *testing.T) {
	if got := bearerToken("Bearer abc"); got != "Bearer abc" {
		t.Fatalf("got %q", got)
	}
	if got := bearerToken("abc"); got != "Bearer abc" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONRequestCarriesCapabilityOutsideToolCallPayload(t *testing.T) {
	request, err := newJSONRequest(http.MethodPost, "http://localhost/agent/tools/execute", "jwt-token", "cap-token", map[string]string{"tool_name": "lease.contract.get"})
	if err != nil {
		t.Fatal(err)
	}
	if request.Header.Get("Authorization") != "Bearer jwt-token" || request.Header.Get("X-Agent-Capability") != "cap-token" {
		t.Fatalf("headers=%v", request.Header)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "cap-token") {
		t.Fatalf("capability leaked into JSON payload: %s", body)
	}
}

func TestReadArgumentsAcceptsOnlyJSONObjectsForExecution(t *testing.T) {
	raw, err := readArguments(`{"contract_id":"contract-1"}`)
	if err != nil || !isJSONObject(raw) {
		t.Fatalf("expected JSON object, raw=%s err=%v", raw, err)
	}
	if isJSONObject([]byte(`[1,2,3]`)) || isJSONObject([]byte(`null`)) {
		t.Fatal("arrays and null must not be accepted as Tool arguments")
	}
}

func TestToolCallPayloadDoesNotContainIdentityFields(t *testing.T) {
	call := map[string]any{
		"call_id": "call-1", "run_id": "run-1", "tool_name": "lease.contract.get",
		"tool_version": "v1", "arguments": map[string]string{"contract_id": "contract-1"},
	}
	raw, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "user_id") || strings.Contains(string(raw), "permissions") || strings.Contains(string(raw), "legal_entity_id") {
		t.Fatalf("CLI payload leaked identity fields: %s", raw)
	}
}

func TestFriendlyContractGetMapsToVersionedToolCall(t *testing.T) {
	var received agenttools.ToolCall
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/tools/execute" || request.Header.Get("Authorization") != "Bearer jwt-token" {
			t.Fatalf("request path=%s auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatalf("decode call: %v", err)
		}
		_ = json.NewEncoder(writer).Encode(agenttools.ToolResult{CallID: received.CallID, Status: agenttools.StatusCompleted})
	}))
	defer server.Close()

	status := runBusiness([]string{"contract", "get", "--base-url", server.URL, "--token", "jwt-token", "--id", "contract-1", "--run-id", "run-1"})
	if status != exitOK {
		t.Fatalf("friendly command exit=%d", status)
	}
	if received.ToolName != "lease.contract.get" || received.ToolVersion != "v1" || received.RunID != "run-1" {
		t.Fatalf("received call=%#v", received)
	}
	var arguments map[string]string
	if err := json.Unmarshal(received.Arguments, &arguments); err != nil {
		t.Fatal(err)
	}
	if arguments["contract_id"] != "contract-1" {
		t.Fatalf("arguments=%v", arguments)
	}
}

func TestSkillsCommandUsesGatewayDiscoveryEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/agent/skills" || request.Header.Get("Authorization") != "Bearer jwt-token" {
			t.Fatalf("request path=%s auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"skills":[{"id":"payment_schedule","version":"v1"}]}`))
	}))
	defer server.Close()

	if status := runSkills([]string{"--base-url", server.URL, "--token", "jwt-token"}); status != exitOK {
		t.Fatalf("skills command exit=%d", status)
	}
}

func TestAuthRefreshCommandUsesUnauthenticatedRefreshEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/auth/refresh" {
			t.Fatalf("request=%s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatalf("refresh endpoint must not receive an access Authorization header")
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["refresh_token"] != "refresh-1" {
			t.Fatalf("payload=%v err=%v", payload, err)
		}
		_, _ = writer.Write([]byte(`{"token":"access-2","refresh_token":"refresh-2"}`))
	}))
	defer server.Close()

	if status := runAuth([]string{"refresh", "--base-url", server.URL, "--refresh-token", "refresh-1"}); status != exitOK {
		t.Fatalf("auth refresh exit=%d", status)
	}
}

func TestRunCommandsUseCoreRunGateway(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/v1/agent/runs" {
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1","status":"queued"}}`))
			return
		}
		if strings.HasSuffix(request.URL.Path, "/events") {
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1"},"events":[]}`))
			return
		}
		if strings.HasSuffix(request.URL.Path, "/trace") {
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1"},"events":[],"artifacts":[],"review_actions":[],"tool_audits":[]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()

	if status := runAgentRun([]string{"create", "--base-url", server.URL, "--token", "jwt-token", "--session-id", "session-1", "--message", "inspect"}); status != exitOK {
		t.Fatalf("run create exit=%d", status)
	}
	if status := runAgentRun([]string{"events", "--base-url", server.URL, "--token", "jwt-token", "--run-id", "run-1"}); status != exitOK {
		t.Fatalf("run events exit=%d", status)
	}
	if status := runAgentRun([]string{"trace", "--base-url", server.URL, "--token", "jwt-token", "--run-id", "run-1"}); status != exitOK {
		t.Fatalf("run trace exit=%d", status)
	}
	if status := runAgentRun([]string{"claim", "--base-url", server.URL, "--token", "jwt-token", "--worker-id", "worker-a"}); status != exitOK {
		t.Fatalf("run claim exit=%d", status)
	}
	if status := runAgentRun([]string{"heartbeat", "--base-url", server.URL, "--token", "jwt-token", "--worker-id", "worker-a", "--run-id", "run-1", "--lease-token", "lease-1"}); status != exitOK {
		t.Fatalf("run heartbeat exit=%d", status)
	}
	if status := runAgentRun([]string{"release", "--base-url", server.URL, "--token", "jwt-token", "--worker-id", "worker-a", "--run-id", "run-1", "--lease-token", "lease-1"}); status != exitOK {
		t.Fatalf("run release exit=%d", status)
	}
	if status := runAgentRun([]string{"steer", "--base-url", server.URL, "--token", "jwt-token", "--run-id", "run-1", "--instruction", "focus"}); status != exitOK {
		t.Fatalf("run steer exit=%d", status)
	}
	if status := runAgentRun([]string{"branch", "--base-url", server.URL, "--token", "jwt-token", "--run-id", "run-1", "--instruction", "branch from checkpoint"}); status != exitOK {
		t.Fatalf("run branch exit=%d", status)
	}
	if status := runAgentRun([]string{"cancel", "--base-url", server.URL, "--token", "jwt-token", "--run-id", "run-1"}); status != exitOK {
		t.Fatalf("run cancel exit=%d", status)
	}
	if len(paths) != 9 || paths[0] != "POST /api/v1/agent/runs" || paths[1] != "GET /api/v1/agent/runs/run-1/events" || paths[2] != "GET /api/v1/agent/runs/run-1/trace" || paths[3] != "POST /api/v1/agent/runs/claim" || paths[4] != "POST /api/v1/agent/runs/run-1/lease/heartbeat" || paths[5] != "POST /api/v1/agent/runs/run-1/lease/release" || paths[6] != "POST /api/v1/agent/runs/run-1/steer" || paths[7] != "POST /api/v1/agent/runs/run-1/branch" || paths[8] != "POST /api/v1/agent/runs/run-1/cancel" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestSessionCreateUsesAgentGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/agent/sessions" || request.Header.Get("Authorization") != "Bearer jwt-token" {
			t.Fatalf("request=%s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		_, _ = writer.Write([]byte(`{"session":{"id":"session-1"}}`))
	}))
	defer server.Close()
	if status := runAgentSession([]string{"create", "--base-url", server.URL, "--token", "jwt-token", "--title", "review"}); status != exitOK {
		t.Fatalf("session create exit=%d", status)
	}
}
