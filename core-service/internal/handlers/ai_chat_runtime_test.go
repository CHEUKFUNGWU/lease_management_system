package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type stubAIChatRuntimeStore struct {
	sessions      map[string]*repository.AIChatSession
	runs          map[string]*repository.AIChatRun
	messages      map[string]*repository.AIChatMessage
	sessionMsgs   map[string][]*repository.AIChatMessage
	artifacts     map[string]*repository.AIChatArtifact
	reviewActions map[string]*repository.AIChatReviewAction
	runEvents     map[string][]*repository.AIChatRunEvent
	nextRunSeq    int
	nextMsgSeq    int
}

func (s *stubAIChatRuntimeStore) CreateSession(context.Context, *repository.AIChatSession) error {
	return fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) GetSessionByID(_ context.Context, sessionID, _ string) (*repository.AIChatSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}
func (s *stubAIChatRuntimeStore) ListSessions(context.Context, repository.AIChatSessionFilter) ([]*repository.AIChatSession, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) CreateRun(_ context.Context, run *repository.AIChatRun) error {
	s.nextRunSeq++
	if run.ID == "" {
		run.ID = fmt.Sprintf("generated-run-%d", s.nextRunSeq)
	}
	s.runs[run.ID] = run
	return nil
}
func (s *stubAIChatRuntimeStore) LinkRunTriggerMessage(_ context.Context, runID, triggerMessageID string) error {
	run, ok := s.runs[runID]
	if !ok {
		return fmt.Errorf("run not found")
	}
	run.TriggerMessageID = &triggerMessageID
	return nil
}
func (s *stubAIChatRuntimeStore) UpdateRunStatus(context.Context, string, string, bool, *string, *string, *time.Time, *time.Time) error {
	return fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) UpdateRunParent(_ context.Context, runID, parentRunID string) error {
	run, ok := s.runs[runID]
	if !ok {
		return fmt.Errorf("run not found")
	}
	run.ParentRunID = &parentRunID
	return nil
}
func (s *stubAIChatRuntimeStore) GetRunByID(_ context.Context, runID, _ string) (*repository.AIChatRun, error) {
	run, ok := s.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run not found")
	}
	return run, nil
}
func (s *stubAIChatRuntimeStore) ListRunsBySession(context.Context, string, int, int) ([]*repository.AIChatRun, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) CreateMessage(_ context.Context, message *repository.AIChatMessage) error {
	s.nextMsgSeq++
	if message.ID == "" {
		message.ID = fmt.Sprintf("generated-message-%d", s.nextMsgSeq)
	}
	s.messages[message.ID] = message
	s.sessionMsgs[message.SessionID] = append([]*repository.AIChatMessage{message}, s.sessionMsgs[message.SessionID]...)
	return nil
}
func (s *stubAIChatRuntimeStore) GetNextMessageSequence(_ context.Context, sessionID string) (int, error) {
	return len(s.sessionMsgs[sessionID]) + 1, nil
}
func (s *stubAIChatRuntimeStore) ListMessagesBySession(_ context.Context, sessionID string, _ int) ([]*repository.AIChatMessage, error) {
	return s.sessionMsgs[sessionID], nil
}
func (s *stubAIChatRuntimeStore) GetMessageByID(_ context.Context, messageID, _ string) (*repository.AIChatMessage, error) {
	message, ok := s.messages[messageID]
	if !ok {
		return nil, fmt.Errorf("message not found")
	}
	return message, nil
}
func (s *stubAIChatRuntimeStore) AppendRunEvent(_ context.Context, event *repository.AIChatRunEvent) error {
	s.runEvents[event.RunID] = append(s.runEvents[event.RunID], event)
	return nil
}
func (s *stubAIChatRuntimeStore) ListRunEvents(context.Context, string, int, int) ([]*repository.AIChatRunEvent, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) GetNextRunEventSequence(_ context.Context, runID string) (int, error) {
	return len(s.runEvents[runID]) + 1, nil
}
func (s *stubAIChatRuntimeStore) CreateArtifact(context.Context, *repository.AIChatArtifact) error {
	return fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) GetArtifactByID(_ context.Context, artifactID, _ string) (*repository.AIChatArtifact, error) {
	artifact, ok := s.artifacts[artifactID]
	if !ok {
		return nil, fmt.Errorf("artifact not found")
	}
	return artifact, nil
}
func (s *stubAIChatRuntimeStore) ListArtifactsBySession(context.Context, string, int) ([]*repository.AIChatArtifact, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) UpdateArtifactStatus(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) RecordReviewAction(context.Context, *repository.AIChatReviewAction) error {
	return fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) GetReviewActionByID(_ context.Context, actionID, _ string) (*repository.AIChatReviewAction, error) {
	action, ok := s.reviewActions[actionID]
	if !ok {
		return nil, fmt.Errorf("action not found")
	}
	return action, nil
}
func (s *stubAIChatRuntimeStore) ListReviewActionsBySession(context.Context, string, int) ([]*repository.AIChatReviewAction, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *stubAIChatRuntimeStore) CreateAttachment(context.Context, *repository.AIChatAttachment) error {
	return nil
}

func newContinuationTestHandler() *AIChatHandler {
	session := &repository.AIChatSession{
		ID:    "session-1",
		Title: "test",
	}
	pageContextRaw, _ := json.Marshal(PageContext{
		Page:       "contract-detail",
		ContractID: "contract-1",
	})
	runSkillID := "payment_schedule"
	run := &repository.AIChatRun{
		ID:               "run-1",
		SessionID:        session.ID,
		SkillID:          &runSkillID,
		PageContext:      pageContextRaw,
		TriggerMessageID: stringPtr("msg-2"),
		Status:           "waiting_review",
	}
	message1 := &repository.AIChatMessage{ID: "msg-1", SessionID: session.ID, Role: "user", Content: "上传租金表"}
	message2 := &repository.AIChatMessage{ID: "msg-2", SessionID: session.ID, RunID: &run.ID, Role: "assistant", Content: "已生成付款计划草稿"}
	message3 := &repository.AIChatMessage{ID: "msg-3", SessionID: session.ID, RunID: &run.ID, Role: "user", Content: "继续"}
	artifactData, _ := json.Marshal(map[string]interface{}{
		"summary": map[string]interface{}{"contract_id": "contract-1"},
	})
	artifact := &repository.AIChatArtifact{
		ID:           "artifact-1",
		SessionID:    session.ID,
		RunID:        run.ID,
		ArtifactType: "payment_schedule_draft",
		Title:        "付款计划草稿",
		Status:       "ready",
		Data:         artifactData,
	}
	actionPayload, _ := json.Marshal(map[string]interface{}{
		"contract_id": "contract-1",
	})
	action := &repository.AIChatReviewAction{
		ID:            "action-1",
		SessionID:     session.ID,
		RunID:         &run.ID,
		ArtifactID:    &artifact.ID,
		ActionType:    "confirm",
		ActionPayload: actionPayload,
	}

	return &AIChatHandler{
		runtimeRepo: &stubAIChatRuntimeStore{
			sessions: map[string]*repository.AIChatSession{
				session.ID: session,
			},
			runs: map[string]*repository.AIChatRun{
				run.ID: run,
			},
			messages: map[string]*repository.AIChatMessage{
				message1.ID: message1,
				message2.ID: message2,
				message3.ID: message3,
			},
			sessionMsgs: map[string][]*repository.AIChatMessage{
				session.ID: {message3, message2, message1},
			},
			artifacts: map[string]*repository.AIChatArtifact{
				artifact.ID: artifact,
			},
			reviewActions: map[string]*repository.AIChatReviewAction{
				action.ID: action,
			},
			runEvents:  map[string][]*repository.AIChatRunEvent{},
			nextRunSeq: 1,
			nextMsgSeq: 3,
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func ginContextForTest() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return ctx
}

func TestReverseMessagesDoesNotMutateOriginal(t *testing.T) {
	original := []*repository.AIChatMessage{
		{ID: "m1"},
		{ID: "m2"},
		{ID: "m3"},
	}

	reversed := reverseMessages(original)

	if got, want := reversed[0].ID, "m3"; got != want {
		t.Fatalf("reversed[0] = %s, want %s", got, want)
	}
	if got, want := original[0].ID, "m1"; got != want {
		t.Fatalf("original slice was mutated: original[0] = %s, want %s", got, want)
	}
}

func TestBuildCompressedHistoryStrategies(t *testing.T) {
	shortMessages := []*repository.AIChatMessage{
		{ID: "m1", Role: "user", Content: "u1"},
		{ID: "m2", Role: "assistant", Content: "a1"},
	}

	history, strategy := buildCompressedHistory(shortMessages, "")
	if strategy != "full_session" {
		t.Fatalf("strategy = %s, want full_session", strategy)
	}
	if len(history) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(history))
	}

	longMessages := make([]*repository.AIChatMessage, 0, 16)
	for i := 1; i <= 16; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		longMessages = append(longMessages, &repository.AIChatMessage{
			ID:      "m" + string(rune('A'+i-1)),
			Role:    role,
			Content: role,
		})
	}

	tailHistory, tailStrategy := buildCompressedHistory(longMessages, "")
	if tailStrategy != "session_tail" {
		t.Fatalf("tail strategy = %s, want session_tail", tailStrategy)
	}
	if len(tailHistory) != 12 {
		t.Fatalf("len(tailHistory) = %d, want 12", len(tailHistory))
	}

	anchoredHistory, anchoredStrategy := buildCompressedHistory(longMessages, "mH")
	if anchoredStrategy != "anchor_window" {
		t.Fatalf("anchored strategy = %s, want anchor_window", anchoredStrategy)
	}
	if len(anchoredHistory) != 12 {
		t.Fatalf("len(anchoredHistory) = %d, want 12", len(anchoredHistory))
	}
	if anchoredHistory[0].Content == "" {
		t.Fatalf("anchored history should preserve message content")
	}
}

func TestBuildContinuationRunbookFallsBackToSourceRunSkill(t *testing.T) {
	handler := &AIChatHandler{}
	skillID := "payment_schedule"
	sourceRun := &repository.AIChatRun{
		SkillID: &skillID,
	}

	runbook := handler.buildContinuationRunbook(AIChatRequest{
		Message: "继续",
	}, "", sourceRun)
	if runbook == nil {
		t.Fatalf("runbook = nil, want payment_schedule fallback")
	}
	if got, want := runbook.SkillID, "payment_schedule"; got != want {
		t.Fatalf("runbook.SkillID = %s, want %s", got, want)
	}
}

func TestBuildContinuationRunbookPrefersCurrentIntentOverSourceRunSkill(t *testing.T) {
	handler := &AIChatHandler{}
	skillID := "payment_schedule"
	sourceRun := &repository.AIChatRun{
		SkillID: &skillID,
	}

	runbook := handler.buildContinuationRunbook(AIChatRequest{
		Message: "请生成审计包，包含披露核对清单",
	}, "", sourceRun)
	if runbook == nil {
		t.Fatalf("runbook = nil, want audit_pack from current intent")
	}
	if got, want := runbook.SkillID, "audit_pack"; got != want {
		t.Fatalf("runbook.SkillID = %s, want %s", got, want)
	}
}

func TestResolveContinuationSeedSupportsAllTargetTypes(t *testing.T) {
	handler := newContinuationTestHandler()
	targets := []struct {
		name           string
		targetType     string
		targetID       string
		wantParentRun  string
		wantContractID string
	}{
		{name: "run", targetType: "run", targetID: "run-1", wantParentRun: "run-1", wantContractID: "contract-1"},
		{name: "message", targetType: "message", targetID: "msg-2", wantParentRun: "run-1", wantContractID: "contract-1"},
		{name: "artifact", targetType: "artifact", targetID: "artifact-1", wantParentRun: "run-1", wantContractID: "contract-1"},
		{name: "action", targetType: "action", targetID: "action-1", wantParentRun: "run-1", wantContractID: "contract-1"},
	}

	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			seed, err := handler.resolveContinuationSeed(context.Background(), &CreateAIChatContinuationRequest{
				Target: AIChatContinuationTarget{
					Type: tc.targetType,
					ID:   tc.targetID,
				},
			}, "user-1")
			if err != nil {
				t.Fatalf("resolveContinuationSeed returned error: %v", err)
			}
			if seed == nil {
				t.Fatalf("seed = nil")
			}
			if got, want := seed.parentRunID, tc.wantParentRun; got != want {
				t.Fatalf("seed.parentRunID = %s, want %s", got, want)
			}
			if got, want := seed.effectiveContractID, tc.wantContractID; got != want {
				t.Fatalf("seed.effectiveContractID = %s, want %s", got, want)
			}
			if seed.instruction == "" {
				t.Fatalf("seed.instruction should not be empty")
			}
			if len(seed.history) == 0 {
				t.Fatalf("seed.history should not be empty")
			}
		})
	}
}

func TestCreateContinuationRuntimeRunBuildsUnifiedContinuation(t *testing.T) {
	handler := newContinuationTestHandler()

	state, runbook, runtimeReq, seed, err := handler.createContinuationRuntimeRun(ginContextForTest(), &CreateAIChatContinuationRequest{
		Target: AIChatContinuationTarget{
			Type: "action",
			ID:   "action-1",
		},
	}, "user-1")
	if err != nil {
		t.Fatalf("createContinuationRuntimeRun returned error: %v", err)
	}
	if state == nil || state.run == nil || state.userMessage == nil {
		t.Fatalf("state/run/userMessage should not be nil")
	}
	if runbook == nil {
		t.Fatalf("runbook should not be nil")
	}
	if got, want := runbook.SkillID, "payment_schedule"; got != want {
		t.Fatalf("runbook.SkillID = %s, want %s", got, want)
	}
	if runtimeReq == nil || runtimeReq.Message == "" {
		t.Fatalf("runtimeReq.Message should not be empty")
	}
	if state.run.ParentRunID == nil || *state.run.ParentRunID != "run-1" {
		t.Fatalf("state.run.ParentRunID = %v, want run-1", state.run.ParentRunID)
	}
	if got, want := seed.targetType, "action"; got != want {
		t.Fatalf("seed.targetType = %s, want %s", got, want)
	}
}
