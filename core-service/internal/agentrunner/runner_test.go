package agentrunner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

type fakeGateway struct {
	descriptors []agenttools.ToolDescriptor
	calls       []agenttools.ToolCall
	issued      CapabilityRequest
	revoked     []string
	results     []agenttools.ToolResult
	heartbeats  int
	releases    int
}

type remoteControlGateway struct {
	*fakeGateway
	controlType string
}

type streamControlGateway struct {
	*fakeGateway
	events chan RunEvent
}

type usagePlanner struct {
	usage PlannerUsage
}

func (p usagePlanner) Plan(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
	calls, _, err := p.PlanWithUsage(ctx, request)
	return calls, err
}

func (p usagePlanner) PlanWithUsage(context.Context, PlanRequest) ([]agenttools.ToolCall, *PlannerUsage, error) {
	return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{}`)}}, &p.usage, nil
}

func (g *streamControlGateway) SubscribeRunEvents(context.Context, string, int) (*RunEventSubscription, error) {
	if g.events == nil {
		g.events = make(chan RunEvent, 1)
	}
	errorsChannel := make(chan error)
	close(errorsChannel)
	return &RunEventSubscription{Events: g.events, Errors: errorsChannel}, nil
}

func (g *remoteControlGateway) ListRunEvents(_ context.Context, _ string, afterSequence, _ int) (RunEventPage, error) {
	if len(g.calls) == 0 || afterSequence > 0 {
		return RunEventPage{}, nil
	}
	payload := json.RawMessage(`{"payload":{"instruction":"show the second contract"}}`)
	if g.controlType == "cancel" {
		payload = json.RawMessage(`{"payload":{"reason":"operator cancelled"}}`)
		return RunEventPage{Events: []RunEvent{{SequenceNo: 1, EventType: "run_cancelled", Payload: payload}}}, nil
	}
	return RunEventPage{Events: []RunEvent{{SequenceNo: 1, EventType: "run_steer", Payload: payload}}}, nil
}

func (g *fakeGateway) Describe(_ context.Context, _ agenttools.ToolFilter, _ string) ([]agenttools.ToolDescriptor, error) {
	return g.descriptors, nil
}

func (g *fakeGateway) IssueCapability(_ context.Context, request CapabilityRequest) (string, error) {
	g.issued = request
	return "capability", nil
}

func (g *fakeGateway) Execute(_ context.Context, call agenttools.ToolCall, _ string) (agenttools.ToolResult, error) {
	g.calls = append(g.calls, call)
	if len(g.results) == 0 {
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"ok": true}}, nil
	}
	result := g.results[0]
	g.results = g.results[1:]
	return result, nil
}

func (g *streamControlGateway) Execute(ctx context.Context, call agenttools.ToolCall, capability string) (agenttools.ToolResult, error) {
	result, err := g.fakeGateway.Execute(ctx, call, capability)
	if err == nil && len(g.calls) == 1 {
		g.events <- RunEvent{SequenceNo: 1, EventType: "run_steer", Payload: json.RawMessage(`{"payload":{"instruction":"show the second contract"}}`)}
	}
	return result, err
}

func (g *fakeGateway) RevokeCapability(_ context.Context, runID string) error {
	g.revoked = append(g.revoked, runID)
	return nil
}

func (g *fakeGateway) HeartbeatRunLease(context.Context, string, string, string, int) error {
	g.heartbeats++
	return nil
}

func (g *fakeGateway) ReleaseRunLease(context.Context, string, string, string, bool) error {
	g.releases++
	return nil
}

func descriptor(name string, level agenttools.ToolLevel) agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{Name: name, Version: "v1", Description: "test", Level: level, ReadOnly: level == agenttools.LevelRead, Permissions: []agenttools.Permission{{Resource: "contracts", Action: "read"}}, SupportsIdempotency: level != agenttools.LevelRead, Review: agenttools.ReviewPolicy{Required: level != agenttools.LevelRead}}
}

func TestRunnerUsesDiscoveredToolsAndBindsEveryCallToRun(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-1"}`)}}, nil
	})}
	result, err := runner.Run(context.Background(), Request{RunID: "run-1", SkillID: "contract_review", SkillVersion: "v1"})
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(gateway.calls) != 1 || gateway.calls[0].RunID != "run-1" || gateway.issued.RunID != "run-1" {
		t.Fatalf("calls=%+v issued=%+v", gateway.calls, gateway.issued)
	}
	if gateway.issued.SkillID != "contract_review" || gateway.issued.SkillVersion != "v1" || gateway.calls[0].SkillID != "contract_review" || len(gateway.revoked) != 1 || gateway.revoked[0] != "run-1" {
		t.Fatalf("skill/lifecycle metadata not propagated: issued=%+v call=%+v revoked=%v", gateway.issued, gateway.calls[0], gateway.revoked)
	}
}

func TestRunnerStopsWhenPlannerUsageExceedsModelTokenBudget(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	total := int64(11)
	runner := &Runner{Gateway: gateway, Planner: usagePlanner{usage: PlannerUsage{TotalTokens: &total}}}
	result, err := runner.Run(context.Background(), Request{RunID: "run-token-budget", SkillID: "contract_review", Limits: Limits{MaxModelTokens: 10}})
	if !errors.Is(err, ErrBudgetExceeded) || result.ModelTokens != 11 || len(gateway.calls) != 0 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, len(gateway.calls))
	}
}

func TestRunnerEmitsTerminalFailureWhenToolIsRejected(t *testing.T) {
	callID := "rejected-call"
	gateway := &fakeGateway{
		descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)},
		results: []agenttools.ToolResult{{
			CallID: callID, Status: agenttools.StatusRejected,
			Error: &agenttools.ToolError{Code: agenttools.ErrorInvalidArguments, Message: "invalid arguments"},
		}},
	}
	var events []Event
	runner := &Runner{
		Gateway: gateway,
		Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
			return []agenttools.ToolCall{{CallID: callID, ToolName: "lease.contract.get", Arguments: []byte(`{}`)}}, nil
		}),
		Emit: func(event Event) { events = append(events, event) },
	}
	result, err := runner.Run(context.Background(), Request{RunID: "run-tool-rejected", SkillID: "contract_review", SkillVersion: "v1"})
	if err != nil || result.Status != "failed" || result.Error != "invalid arguments" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(events) == 0 || events[len(events)-1].Type != "run_failed" || events[len(events)-1].Payload != result.Error {
		t.Fatalf("events=%+v", events)
	}
}

func TestRunnerReleasesOwnedWorkerLeaseAfterTerminalRun(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{}`)}}, nil
	})}
	result, err := runner.Run(context.Background(), Request{
		RunID: "run-leased", WorkerID: "worker-a", LeaseToken: "lease-a", LeaseSeconds: 60,
		SkillID: "contract_review", SkillVersion: "v1",
	})
	if err != nil || result.Status != "completed" || gateway.releases != 1 {
		t.Fatalf("result=%+v err=%v releases=%d", result, err, gateway.releases)
	}
}

func TestRunnerRejectsPlannerToolOutsideSkillAndRequiresWriteIdempotency(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		return []agenttools.ToolCall{{ToolName: "lease.event.list", Arguments: []byte(`{}`)}}, nil
	})}
	if _, err := runner.Run(context.Background(), Request{RunID: "run-1", SkillID: "contract_review"}); !errors.Is(err, ErrPlanNotAllowed) {
		t.Fatalf("outside skill error=%v", err)
	}

	gateway = &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.draft.create", agenttools.LevelDraft)}}
	runner.Gateway = gateway
	runner.Planner = PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		return []agenttools.ToolCall{{ToolName: "lease.contract.draft.create", Arguments: []byte(`{}`)}}, nil
	})
	if _, err := runner.Run(context.Background(), Request{RunID: "run-1", SkillID: "excel_ledger"}); !errors.Is(err, ErrPlanNotAllowed) {
		t.Fatalf("missing idempotency error=%v", err)
	}
}

func TestRunnerStopsAtToolCallBudget(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		return []agenttools.ToolCall{
			{ToolName: "lease.contract.get", Arguments: []byte(`{}`)},
			{ToolName: "lease.contract.get", Arguments: []byte(`{}`)},
		}, nil
	})}
	if _, err := runner.Run(context.Background(), Request{RunID: "run-1", SkillID: "contract_review", Limits: Limits{MaxToolCalls: 1}}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("budget error=%v", err)
	}
}

func TestRunnerResumesFromCheckpointWithoutRepeatingCompletedTool(t *testing.T) {
	store := NewMemoryCheckpointStore()
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	plannerCalls := 0
	runner := &Runner{Gateway: gateway, Checkpoints: store, Planner: PlannerFunc(func(context.Context, PlanRequest) ([]agenttools.ToolCall, error) {
		plannerCalls++
		return []agenttools.ToolCall{
			{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-1"}`)},
			{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-2"}`)},
		}, nil
	})}
	if _, err := runner.Run(context.Background(), Request{RunID: "run-resume", SkillID: "contract_review", SkillVersion: "v1", Limits: Limits{MaxToolCalls: 1}}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("first run error=%v", err)
	}
	result, err := runner.Run(context.Background(), Request{RunID: "run-resume", SkillID: "contract_review", SkillVersion: "v1", Resume: true})
	if err != nil || result.Status != "completed" {
		t.Fatalf("resume result=%+v err=%v", result, err)
	}
	if plannerCalls != 1 || len(gateway.calls) != 2 {
		t.Fatalf("planner_calls=%d gateway_calls=%d calls=%+v", plannerCalls, len(gateway.calls), gateway.calls)
	}
}

func TestRunnerSteerReplansWithCompletedResults(t *testing.T) {
	gateway := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	plannerCalls := 0
	var runner *Runner
	steered := false
	runner = &Runner{
		Gateway: gateway,
		Planner: PlannerFunc(func(_ context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
			plannerCalls++
			if request.SteerInstruction == "show the second contract" {
				if len(request.CompletedResults) != 1 {
					t.Fatalf("completed results=%d, want 1", len(request.CompletedResults))
				}
				return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-2"}`)}}, nil
			}
			return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-1"}`)}}, nil
		}),
	}
	runner.Emit = func(event Event) {
		if event.Type == "tool_completed" && !steered {
			steered = true
			if err := runner.Steer("run-steer", "show the second contract"); err != nil {
				t.Fatalf("steer=%v", err)
			}
		}
	}
	result, err := runner.Run(context.Background(), Request{RunID: "run-steer", SkillID: "contract_review", SkillVersion: "v1"})
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if plannerCalls != 2 || len(gateway.calls) != 2 || string(gateway.calls[1].Arguments) != `{"contract_id":"c-2"}` {
		t.Fatalf("planner_calls=%d calls=%+v", plannerCalls, gateway.calls)
	}
}

func TestRunnerConsumesCoreRunSteerEvent(t *testing.T) {
	base := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	gateway := &remoteControlGateway{fakeGateway: base}
	plannerCalls := 0
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(_ context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
		plannerCalls++
		if request.SteerInstruction != "show the second contract" {
			return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-1"}`)}}, nil
		}
		if len(request.CompletedResults) != 1 {
			t.Fatalf("completed results=%d, want 1", len(request.CompletedResults))
		}
		return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-2"}`)}}, nil
	})}
	result, err := runner.Run(context.Background(), Request{RunID: "run-remote-steer", SkillID: "contract_review", SkillVersion: "v1"})
	if err != nil || result.Status != "completed" || plannerCalls != 2 || len(base.calls) != 2 {
		t.Fatalf("result=%+v err=%v planner_calls=%d calls=%+v", result, err, plannerCalls, base.calls)
	}
	if string(base.calls[1].Arguments) != `{"contract_id":"c-2"}` {
		t.Fatalf("calls=%+v", base.calls)
	}
}

func TestRunnerConsumesLiveRunEventSubscription(t *testing.T) {
	base := &fakeGateway{descriptors: []agenttools.ToolDescriptor{descriptor("lease.contract.get", agenttools.LevelRead)}}
	gateway := &streamControlGateway{fakeGateway: base}
	plannerCalls := 0
	runner := &Runner{Gateway: gateway, Planner: PlannerFunc(func(_ context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
		plannerCalls++
		if request.SteerInstruction != "show the second contract" {
			return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-1"}`)}}, nil
		}
		if len(request.CompletedResults) != 1 {
			t.Fatalf("completed results=%d, want 1", len(request.CompletedResults))
		}
		return []agenttools.ToolCall{{ToolName: "lease.contract.get", Arguments: []byte(`{"contract_id":"c-2"}`)}}, nil
	})}
	result, err := runner.Run(context.Background(), Request{RunID: "run-live-steer", SkillID: "contract_review", SkillVersion: "v1"})
	if err != nil || result.Status != "completed" || plannerCalls != 2 || len(base.calls) != 2 {
		t.Fatalf("result=%+v err=%v planner_calls=%d calls=%+v", result, err, plannerCalls, base.calls)
	}
	if string(base.calls[1].Arguments) != `{"contract_id":"c-2"}` {
		t.Fatalf("calls=%+v", base.calls)
	}
}

func TestHTTPGatewayRunLifecycleAndEventRecorder(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/runs":
			_, _ = writer.Write([]byte(`{"run":{"id":"run-http","session_id":"session-1","status":"queued","agent_mode":true}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/events"):
			_, _ = writer.Write([]byte(`{"run":{"id":"run-http","status":"running"},"events":[]}`))
		default:
			_, _ = writer.Write([]byte(`{"accepted":true}`))
		}
	}))
	defer server.Close()

	gateway := NewHTTPGateway(server.URL, "jwt-token", server.Client())
	run, err := gateway.CreateRun(context.Background(), RunRequest{SessionID: "session-1", Message: "inspect"})
	if err != nil || run.ID != "run-http" {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if err := gateway.Record(context.Background(), Event{Type: "run_started", RunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.ListRunEvents(context.Background(), run.ID, 0, 20); err != nil {
		t.Fatal(err)
	}
	if err := gateway.SteerRun(context.Background(), run.ID, "focus on dates"); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 || paths[0] != "POST /api/v1/agent/runs" || paths[1] != "POST /api/v1/agent/runs/run-http/events" || paths[2] != "GET /api/v1/agent/runs/run-http/events" || paths[3] != "POST /api/v1/agent/runs/run-http/steer" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestCompactContextDoesNotPromoteUnsupportedInference(t *testing.T) {
	compacted := CompactContext([]ContextItem{
		{Text: "system fact", Verified: true, EvidenceRefs: []string{"evidence-1"}},
		{Text: "model guess", Verified: true},
	})
	if len(compacted.VerifiedFacts) != 1 || len(compacted.Inferences) != 1 || compacted.Inferences[0].Verified {
		t.Fatalf("compacted=%+v", compacted)
	}
}

func TestJSONFileCheckpointStoreRoundTripsAtomically(t *testing.T) {
	directory := t.TempDir()
	store, err := NewJSONFileCheckpointStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	want := Checkpoint{
		RunID: "run/with/path", SkillID: "contract_review", SkillVersion: "v1",
		Plan:      []agenttools.ToolCall{{CallID: "call-1", RunID: "run/with/path", ToolName: "lease.contract.get", ToolVersion: "v1", Arguments: []byte(`{}`)}},
		NextIndex: 1, Status: "running",
	}
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(context.Background(), want.RunID)
	if err != nil || got.RunID != want.RunID || got.NextIndex != 1 || len(got.Plan) != 1 {
		t.Fatalf("checkpoint=%+v err=%v", got, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].IsDir() {
		t.Fatalf("unexpected checkpoint files: entries=%v err=%v", entries, err)
	}
}
