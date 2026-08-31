package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/repository"
)

type testResponse struct {
	Answer string
	Model  string
}

func TestOpenSessionPersistsUserAndTenantContext(t *testing.T) {
	store := newMemoryStore()
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) { return testResponse{}, nil }),
		func(testResponse) Result { return Result{} },
		Options{},
	)

	session, err := runtime.OpenSession(context.Background(), SessionCommand{
		UserID: "user-1", LegalEntityID: "entity-1", Title: "Review lease",
		BoundContractID: "contract-1", ContextSnapshot: map[string]any{"page": "contract-detail"},
	})
	if err != nil {
		t.Fatalf("OpenSession returned error: %v", err)
	}
	if session.UserID != "user-1" || session.LegalEntityID == nil || *session.LegalEntityID != "entity-1" {
		t.Fatalf("session identity = %#v", session)
	}
	if session.BoundContractID == nil || *session.BoundContractID != "contract-1" {
		t.Fatalf("bound contract = %v", session.BoundContractID)
	}
}

func TestStartPersistsOneInspectableSuccessfulRun(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan {
			return Plan{AgentMode: true, SkillID: "contract_review"}
		}),
		ExecutorFunc[testResponse](func(_ context.Context, execution Execution) (testResponse, error) {
			if err := execution.Emit(context.Background(), "tool_start", map[string]any{"tool": "contract.lookup"}); err != nil {
				return testResponse{}, err
			}
			return testResponse{Answer: "review complete", Model: "test-model"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	started, err := runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{
		SessionID: "session-1",
		Message:   "review this contract",
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	inspection, err := runtime.Inspect(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), "user-1", started.Run.ID, 0)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if got, want := inspection.Run.Status, "completed"; got != want {
		t.Fatalf("run status = %q, want %q", got, want)
	}
	if got, want := eventTypes(inspection.Events), []string{"message_start", "run_status", "tool_start", "message_end", "run_end"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
	if !inspection.Events[len(inspection.Events)-1].IsTerminal {
		t.Fatalf("last event should be terminal")
	}
	if started.TriggerMessage.Content != "review this contract" {
		t.Fatalf("trigger message = %q", started.TriggerMessage.Content)
	}
}

func TestStartPreservesAccessScopeForDetachedAgentRun(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("le-001")}
	var observed access.Scope
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(ctx context.Context, _ Execution) (testResponse, error) {
			var ok bool
			observed, ok = access.ScopeFromContext(ctx)
			if !ok {
				return testResponse{}, fmt.Errorf("access scope was lost before detached execution")
			}
			return testResponse{Answer: "ok", Model: "test"}, nil
		}),
		func(testResponse) Result { return Result{} },
		Options{Dispatch: func(task func()) { task() }},
	)

	scope := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}}
	if _, err := runtime.Start(access.WithScope(context.Background(), scope), Input{
		SessionID: "session-1", Message: "review", UserID: "user-1",
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if observed.LegalEntityID != scope.LegalEntityID || len(observed.StoreIDs) != 1 || observed.StoreIDs[0] != "store-1" {
		t.Fatalf("observed scope = %+v, want %+v", observed, scope)
	}
}

func TestStartTerminatesFailedExecution(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) {
			return testResponse{Answer: "AI provider unavailable", Model: "fallback"}, fmt.Errorf("provider timeout")
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	started, err := runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{SessionID: "session-1", Message: "answer", UserID: "user-1"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	inspection, err := runtime.Inspect(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), "user-1", started.Run.ID, 0)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if got, want := inspection.Run.Status, "failed"; got != want {
		t.Fatalf("run status = %q, want %q", got, want)
	}
	if inspection.Run.ErrorMessage == nil || *inspection.Run.ErrorMessage != "provider timeout" {
		t.Fatalf("run error = %v, want provider timeout", inspection.Run.ErrorMessage)
	}
	last := inspection.Events[len(inspection.Events)-1]
	if last.EventType != "run_error" || !last.IsTerminal {
		t.Fatalf("last event = %#v, want terminal run_error", last)
	}
}

func TestContinueResolvesRunTargetInsideRuntime(t *testing.T) {
	store := newMemoryStore()
	pageContext, _ := json.Marshal(PageContext{Page: "contract-detail", ContractID: "contract-1"})
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	skillID := "payment_schedule"
	store.runs["source-run"] = &repository.AIChatRun{
		ID: "source-run", SessionID: "session-1", SkillID: &skillID,
		PageContext: pageContext, Status: "waiting_review",
	}
	store.messages["session-1"] = []*repository.AIChatMessage{
		{ID: "message-1", SessionID: "session-1", Role: "user", Content: "upload rent table"},
		{ID: "message-2", SessionID: "session-1", RunID: stringPointer("source-run"), Role: "assistant", Content: "draft ready"},
	}
	store.runs["source-run"].TriggerMessageID = stringPointer("message-1")
	var plannedFrom string
	runtime := newRuntime(
		store,
		PlannerFunc(func(_ Input, source *repository.AIChatRun) Plan {
			if source != nil {
				plannedFrom = source.ID
			}
			return Plan{AgentMode: true, SkillID: skillID}
		}),
		ExecutorFunc[testResponse](func(_ context.Context, execution Execution) (testResponse, error) {
			if execution.Input.ContractID != "contract-1" {
				return testResponse{}, fmt.Errorf("contract ID was not resolved")
			}
			return testResponse{Answer: "continued", Model: "test-model"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	started, err := runtime.Continue(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), ContinueCommand{
		Target: Target{Type: "run", ID: "source-run"}, UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("Continue returned error: %v", err)
	}
	if started.Run.ParentRunID == nil || *started.Run.ParentRunID != "source-run" {
		t.Fatalf("parent run = %v, want source-run", started.Run.ParentRunID)
	}
	if plannedFrom != "source-run" {
		t.Fatalf("planner source = %q, want source-run", plannedFrom)
	}
	if started.Continuation == nil || started.Continuation.EffectiveContractID != "contract-1" {
		t.Fatalf("continuation = %#v", started.Continuation)
	}
	if started.Continuation.HistoryMessageCount != 2 {
		t.Fatalf("history count = %d, want 2", started.Continuation.HistoryMessageCount)
	}
}

func TestContinuePreservesAccessScopeForDetachedAgentRun(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("le-001")}
	store.runs["source-run"] = &repository.AIChatRun{ID: "source-run", SessionID: "session-1", Status: "waiting_review"}
	store.messages["session-1"] = []*repository.AIChatMessage{
		{ID: "message-1", SessionID: "session-1", RunID: stringPointer("source-run"), Role: "user", Content: "review"},
	}
	store.runs["source-run"].TriggerMessageID = stringPointer("message-1")
	var observed access.Scope
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(ctx context.Context, _ Execution) (testResponse, error) {
			var ok bool
			observed, ok = access.ScopeFromContext(ctx)
			if !ok {
				return testResponse{}, fmt.Errorf("continuation lost access scope")
			}
			return testResponse{Answer: "continued", Model: "test"}, nil
		}),
		func(testResponse) Result { return Result{} },
		Options{Dispatch: func(task func()) { task() }},
	)
	scope := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}}
	if _, err := runtime.Continue(access.WithScope(context.Background(), scope), ContinueCommand{
		Target: Target{Type: "run", ID: "source-run"}, UserID: "user-1",
	}); err != nil {
		t.Fatalf("Continue returned error: %v", err)
	}
	if observed.LegalEntityID != scope.LegalEntityID || len(observed.StoreIDs) != 1 || observed.StoreIDs[0] != "store-1" {
		t.Fatalf("observed scope = %+v, want %+v", observed, scope)
	}
}

func TestContinueSupportsMessageArtifactAndActionTargets(t *testing.T) {
	targets := []Target{
		{Type: "message", ID: "message-2"},
		{Type: "artifact", ID: "artifact-1"},
		{Type: "action", ID: "action-1"},
	}
	for _, target := range targets {
		t.Run(target.Type, func(t *testing.T) {
			store := newMemoryStore()
			pageContext, _ := json.Marshal(PageContext{Page: "contract-detail", ContractID: "contract-1"})
			store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
			store.runs["source-run"] = &repository.AIChatRun{ID: "source-run", SessionID: "session-1", PageContext: pageContext, Status: "waiting_review"}
			store.messages["session-1"] = []*repository.AIChatMessage{
				{ID: "message-1", SessionID: "session-1", Role: "user", Content: "upload"},
				{ID: "message-2", SessionID: "session-1", RunID: stringPointer("source-run"), Role: "assistant", Content: "draft ready"},
			}
			artifactData, _ := json.Marshal(map[string]any{"contract_id": "contract-1"})
			store.artifacts["artifact-1"] = &repository.AIChatArtifact{ID: "artifact-1", SessionID: "session-1", RunID: "source-run", Data: artifactData}
			store.actions["action-1"] = &repository.AIChatReviewAction{ID: "action-1", SessionID: "session-1", RunID: stringPointer("source-run"), ArtifactID: stringPointer("artifact-1")}
			runtime := newRuntime(
				store,
				PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
				ExecutorFunc[testResponse](func(_ context.Context, execution Execution) (testResponse, error) {
					if execution.Input.ContractID != "contract-1" {
						return testResponse{}, fmt.Errorf("contract ID was not resolved")
					}
					return testResponse{Answer: "continued", Model: "test-model"}, nil
				}),
				func(response testResponse) Result {
					return Result{Answer: response.Answer, Model: response.Model}
				},
				Options{Dispatch: func(task func()) { task() }},
			)

			started, err := runtime.Continue(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), ContinueCommand{Target: target, UserID: "user-1"})
			if err != nil {
				t.Fatalf("Continue returned error: %v", err)
			}
			if started.Run.ParentRunID == nil || *started.Run.ParentRunID != "source-run" {
				t.Fatalf("parent run = %v, want source-run", started.Run.ParentRunID)
			}
			if started.Continuation == nil || started.Continuation.Target != target {
				t.Fatalf("continuation = %#v", started.Continuation)
			}
		})
	}
}

func TestReviewRecordsActionAndProjectsArtifactStatus(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	store.runs["run-1"] = &repository.AIChatRun{ID: "run-1", SessionID: "session-1", Status: "waiting_review"}
	store.artifacts["artifact-1"] = &repository.AIChatArtifact{
		ID: "artifact-1", SessionID: "session-1", RunID: "run-1",
		ArtifactType: "contract_draft", Status: "ready",
	}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) { return testResponse{}, nil }),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	result, err := runtime.Review(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), ReviewCommand{
		ArtifactID: "artifact-1", ActionType: "confirm", UserID: "user-1",
		ActionPayload: map[string]any{"selected_count": 1},
	})
	if err != nil {
		t.Fatalf("Review returned error: %v", err)
	}
	if result.Action.ID == "" || result.Action.ActionType != "confirm" {
		t.Fatalf("action = %#v", result.Action)
	}
	if got, want := result.ArtifactStatus, "confirmed"; got != want {
		t.Fatalf("artifact status = %q, want %q", got, want)
	}
	if store.artifacts["artifact-1"].Status != "confirmed" {
		t.Fatalf("persisted artifact status = %q", store.artifacts["artifact-1"].Status)
	}
}

func TestReviewUsesAtomicRuntimeCommitWhenAvailable(t *testing.T) {
	base := newMemoryStore()
	base.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	base.runs["run-1"] = &repository.AIChatRun{ID: "run-1", SessionID: "session-1", Status: "waiting_review"}
	base.artifacts["artifact-1"] = &repository.AIChatArtifact{
		ID: "artifact-1", SessionID: "session-1", RunID: "run-1", ArtifactType: "event_draft", Status: "ready",
	}
	store := &atomicMemoryStore{memoryStore: base}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) { return testResponse{}, nil }),
		func(response testResponse) Result { return Result{Answer: response.Answer, Model: response.Model} },
		Options{Dispatch: func(task func()) { task() }},
	)
	if _, err := runtime.Review(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), ReviewCommand{ArtifactID: "artifact-1", ActionType: "confirm", UserID: "user-1"}); err != nil {
		t.Fatalf("Review returned error: %v", err)
	}
	if !store.atomic || base.artifacts["artifact-1"].Status != "confirmed" || len(base.actions) != 1 {
		t.Fatalf("atomic commit state = atomic:%v artifact:%+v actions:%d", store.atomic, base.artifacts["artifact-1"], len(base.actions))
	}
}

func TestReviewUsesApplicationTransactionCommitResult(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	store.runs["run-1"] = &repository.AIChatRun{ID: "run-1", SessionID: "session-1", Status: "waiting_review"}
	store.artifacts["artifact-1"] = &repository.AIChatArtifact{
		ID: "artifact-1", SessionID: "session-1", RunID: "run-1", ArtifactType: "contract_draft", Status: "ready",
	}
	committed := false
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) { return testResponse{}, nil }),
		func(response testResponse) Result { return Result{} },
		Options{
			Dispatch: func(task func()) { task() },
			ReviewCommit: func(_ context.Context, artifact *repository.AIChatArtifact, action *repository.AIChatReviewAction, status string, _ ReviewCommand) (any, error) {
				if artifact.ID != "artifact-1" || action.ActionType != "confirm" || status != "confirmed" {
					t.Fatalf("unexpected review transaction input: artifact=%+v action=%+v status=%s", artifact, action, status)
				}
				committed = true
				return map[string]any{"batch_id": "batch-1"}, nil
			},
		},
	)
	result, err := runtime.Review(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), ReviewCommand{ArtifactID: "artifact-1", ActionType: "confirm", UserID: "user-1"})
	if err != nil || !committed {
		t.Fatalf("Review err=%v committed=%v", err, committed)
	}
	if result.CommitResult == nil {
		t.Fatal("application transaction result was not returned")
	}
	if len(store.actions) != 0 || store.artifacts["artifact-1"].Status != "ready" {
		t.Fatal("runtime fallback store should not be mutated when application transaction owns the commit")
	}
}

func TestStartPersistsReviewArtifactBeforeWaitingForReview(t *testing.T) {
	store := newMemoryStore()
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan {
			return Plan{AgentMode: true, SkillID: "contract_review", ReviewRequired: true}
		}),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) {
			return testResponse{Answer: "draft ready", Model: "test-model"}, nil
		}),
		func(response testResponse) Result {
			return Result{
				Answer: response.Answer, Model: response.Model, ReviewRequired: true,
				Artifacts: []ArtifactDraft{{
					Type: "contract_draft", Title: "Contract draft",
					Data:           map[string]any{"contracts": []any{map[string]any{"contract_number": "LEASE-1"}}},
					ReviewRequired: true, EvidenceComplete: true,
					EvidenceRefs: []agentartifact.EvidenceReference{{
						SourceFileID: "file-1", Complete: true,
						Locators: []agentartifact.EvidenceLocator{{Field: "contracts[0].contract_number", Source: "Sheet1!A2"}},
					}},
					ReviewReasons: []string{"assist_mode"}, ModelVersion: "test-model", RuleVersion: "test-rules.v1",
				}},
			}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	started, err := runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{SessionID: "session-1", Message: "review", UserID: "user-1"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	inspection, err := runtime.Inspect(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), "user-1", started.Run.ID, 0)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if got, want := inspection.Run.Status, "waiting_review"; got != want {
		t.Fatalf("run status = %q, want %q", got, want)
	}
	if len(store.artifacts) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(store.artifacts))
	}
	for _, artifact := range store.artifacts {
		if artifact.SchemaVersion != agentartifact.SchemaVersion || !artifact.EvidenceComplete || artifact.ModelVersion == nil || artifact.RuleVersion == nil {
			t.Fatalf("artifact protocol fields = %+v", artifact)
		}
		if len(artifact.EvidenceRefs) == 0 || len(artifact.ReviewReasons) == 0 {
			t.Fatalf("artifact evidence/review fields = %+v", artifact)
		}
	}
	if got := eventTypes(inspection.Events); fmt.Sprint(got[len(got)-2:]) != fmt.Sprint([]string{"artifact_ready", "run_end"}) {
		t.Fatalf("terminal events = %v", got)
	}
}

func TestBindPageFillArtifactID(t *testing.T) {
	bound, err := bindPageFillArtifactID(json.RawMessage(`{
		"target_page":"retail-data-import",
		"deep_link":"/retail-data-import?section=plan&plan_fill=fpna.plan_lines.fill.draft-run-1"
	}`), "2d8e1136-9c80-4d23-95b4-104ead531fa6")
	if err != nil {
		t.Fatalf("bindPageFillArtifactID returned error: %v", err)
	}
	var fill map[string]any
	if err := json.Unmarshal(bound, &fill); err != nil {
		t.Fatalf("unmarshal bound fill: %v", err)
	}
	if got, want := fill["deep_link"], "/retail-data-import?plan_fill=2d8e1136-9c80-4d23-95b4-104ead531fa6&section=plan"; got != want {
		t.Fatalf("deep_link = %q, want %q", got, want)
	}
}

func TestStartTurnsCompletionPersistenceFailureIntoTerminalFailure(t *testing.T) {
	store := newMemoryStore()
	store.failAssistantMessage = true
	store.sessions["session-1"] = &repository.AIChatSession{ID: "session-1", UserID: "user-1", LegalEntityID: stringPointer("entity-1")}
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) {
			return testResponse{Answer: "done", Model: "test-model"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	started, err := runtime.Start(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), Input{SessionID: "session-1", Message: "run", UserID: "user-1"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	inspection, err := runtime.Inspect(access.WithScope(context.Background(), access.Scope{LegalEntityID: "entity-1"}), "user-1", started.Run.ID, 0)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if inspection.Run.Status != "failed" {
		t.Fatalf("run status = %q, want failed", inspection.Run.Status)
	}
	last := inspection.Events[len(inspection.Events)-1]
	if last.EventType != "run_error" || !last.IsTerminal {
		t.Fatalf("last event = %#v, want terminal run_error", last)
	}
}

func stringPointer(value string) *string { return &value }

func eventTypes(events []*repository.AIChatRunEvent) []string {
	result := make([]string, 0, len(events))
	for _, event := range events {
		result = append(result, event.EventType)
	}
	return result
}

type memoryStore struct {
	sessions             map[string]*repository.AIChatSession
	runs                 map[string]*repository.AIChatRun
	messages             map[string][]*repository.AIChatMessage
	events               map[string][]*repository.AIChatRunEvent
	artifacts            map[string]*repository.AIChatArtifact
	actions              map[string]*repository.AIChatReviewAction
	nextID               int
	failAssistantMessage bool
}

type atomicMemoryStore struct {
	*memoryStore
	atomic bool
}

func (s *atomicMemoryStore) CommitReviewAction(ctx context.Context, action *repository.AIChatReviewAction, status string) error {
	s.atomic = true
	if err := s.RecordReviewAction(ctx, action); err != nil {
		return err
	}
	return s.UpdateArtifactStatus(ctx, *action.ArtifactID, status)
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions:  map[string]*repository.AIChatSession{},
		runs:      map[string]*repository.AIChatRun{},
		messages:  map[string][]*repository.AIChatMessage{},
		events:    map[string][]*repository.AIChatRunEvent{},
		artifacts: map[string]*repository.AIChatArtifact{},
		actions:   map[string]*repository.AIChatReviewAction{},
	}
}

func (s *memoryStore) id(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%d", prefix, s.nextID)
}
func (s *memoryStore) CreateSession(_ context.Context, session *repository.AIChatSession) error {
	if session.ID == "" {
		session.ID = s.id("session")
	}
	s.sessions[session.ID] = session
	return nil
}
func (s *memoryStore) GetSessionByID(_ context.Context, id, userID string, entity access.EntityFilter) (*repository.AIChatSession, error) {
	session := s.sessions[id]
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.UserID != userID {
		return nil, fmt.Errorf("scope_denied: session belongs to another user")
	}
	if !entity.IsGlobal() {
		want, err := entity.LegalEntityID()
		if err != nil {
			return nil, err
		}
		if session.LegalEntityID == nil || *session.LegalEntityID != want {
			return nil, fmt.Errorf("scope_denied: session belongs to another legal entity")
		}
	}
	return session, nil
}
func (s *memoryStore) CreateRun(_ context.Context, run *repository.AIChatRun) error {
	if run.ID == "" {
		run.ID = s.id("run")
	}
	s.runs[run.ID] = run
	return nil
}
func (s *memoryStore) LinkRunTriggerMessage(_ context.Context, runID, messageID string) error {
	s.runs[runID].TriggerMessageID = &messageID
	return nil
}
func (s *memoryStore) UpdateRunStatus(_ context.Context, runID, status string, review bool, summary, failure *string, started, completed *time.Time) error {
	run := s.runs[runID]
	run.Status, run.ReviewRequired, run.SummaryText, run.ErrorMessage = status, review, summary, failure
	if started != nil {
		run.StartedAt = started
	}
	if completed != nil {
		run.CompletedAt = completed
	}
	return nil
}
func (s *memoryStore) GetRunByID(_ context.Context, id, userID string, entity access.EntityFilter) (*repository.AIChatRun, error) {
	run := s.runs[id]
	if run == nil || !s.sessionOwned(run.SessionID, userID, entity) {
		return nil, fmt.Errorf("run not found")
	}
	return run, nil
}

// sessionOwned mirrors the repository's two-axis ownership check for the
// in-memory fake: user always, entity per filter.
func (s *memoryStore) sessionOwned(sessionID, userID string, entity access.EntityFilter) bool {
	session := s.sessions[sessionID]
	if session == nil || session.UserID != userID {
		return false
	}
	if entity.IsGlobal() {
		return true
	}
	want, err := entity.LegalEntityID()
	if err != nil {
		return false
	}
	return session.LegalEntityID != nil && *session.LegalEntityID == want
}
func (s *memoryStore) CreateMessage(_ context.Context, message *repository.AIChatMessage) error {
	if message.Role == "assistant" && s.failAssistantMessage {
		return fmt.Errorf("assistant message write failed")
	}
	if message.ID == "" {
		message.ID = s.id("message")
	}
	s.messages[message.SessionID] = append(s.messages[message.SessionID], message)
	return nil
}
func (s *memoryStore) GetNextMessageSequence(_ context.Context, sessionID string) (int, error) {
	return len(s.messages[sessionID]) + 1, nil
}
func (s *memoryStore) ListMessagesBySession(_ context.Context, sessionID, userID string, entity access.EntityFilter, _ int) ([]*repository.AIChatMessage, error) {
	if !s.sessionOwned(sessionID, userID, entity) {
		return nil, fmt.Errorf("scope_denied: session belongs to another user or legal entity")
	}
	messages := append([]*repository.AIChatMessage(nil), s.messages[sessionID]...)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}
func (s *memoryStore) GetMessageByID(_ context.Context, id, userID string, entity access.EntityFilter) (*repository.AIChatMessage, error) {
	for sessionID, messages := range s.messages {
		if !s.sessionOwned(sessionID, userID, entity) {
			continue
		}
		for _, message := range messages {
			if message.ID == id {
				return message, nil
			}
		}
	}
	return nil, fmt.Errorf("message not found")
}
func (s *memoryStore) AppendRunEvent(_ context.Context, event *repository.AIChatRunEvent) error {
	if event.ID == "" {
		event.ID = s.id("event")
	}
	s.events[event.RunID] = append(s.events[event.RunID], event)
	return nil
}
func (s *memoryStore) ListRunEvents(_ context.Context, runID string, after, limit int, entity access.EntityFilter, userID string) ([]*repository.AIChatRunEvent, error) {
	run := s.runs[runID]
	if run != nil && !s.sessionOwned(run.SessionID, userID, entity) {
		return nil, fmt.Errorf("scope_denied: run belongs to another user or legal entity")
	}
	var result []*repository.AIChatRunEvent
	for _, event := range s.events[runID] {
		if event.SequenceNo > after {
			result = append(result, event)
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}
func (s *memoryStore) GetNextRunEventSequence(_ context.Context, runID string) (int, error) {
	return len(s.events[runID]) + 1, nil
}
func (s *memoryStore) CreateArtifact(_ context.Context, artifact *repository.AIChatArtifact) error {
	if artifact.ID == "" {
		artifact.ID = s.id("artifact")
	}
	s.artifacts[artifact.ID] = artifact
	return nil
}
func (s *memoryStore) GetArtifactByID(_ context.Context, id, userID string, entity access.EntityFilter) (*repository.AIChatArtifact, error) {
	artifact := s.artifacts[id]
	if artifact == nil || !s.sessionOwned(artifact.SessionID, userID, entity) {
		return nil, fmt.Errorf("artifact not found")
	}
	return artifact, nil
}
func (s *memoryStore) UpdateArtifactStatus(_ context.Context, id, status string) error {
	s.artifacts[id].Status = status
	return nil
}
func (s *memoryStore) RecordReviewAction(_ context.Context, action *repository.AIChatReviewAction) error {
	if action.ID == "" {
		action.ID = s.id("action")
	}
	s.actions[action.ID] = action
	return nil
}
func (s *memoryStore) GetReviewActionByID(_ context.Context, id, userID string, entity access.EntityFilter) (*repository.AIChatReviewAction, error) {
	action := s.actions[id]
	if action == nil || !s.sessionOwned(action.SessionID, userID, entity) {
		return nil, fmt.Errorf("action not found")
	}
	return action, nil
}
func (s *memoryStore) CreateAttachment(context.Context, *repository.AIChatAttachment) error {
	return nil
}

var _ store = (*memoryStore)(nil)
var _ = json.RawMessage{}
