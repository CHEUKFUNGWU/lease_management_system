package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type traceRuntimeStore struct {
	run         *repository.AIChatRun
	events      []*repository.AIChatRunEvent
	artifacts   []*repository.AIChatArtifact
	actions     []*repository.AIChatReviewAction
	links       []*repository.AgentRunAuditLink
	checkpoints []*repository.AgentRunCheckpointAudit
}

func (s *traceRuntimeStore) GetSessionByID(context.Context, string, string) (*repository.AIChatSession, error) {
	return &repository.AIChatSession{ID: "session-1", UserID: "user-1"}, nil
}
func (s *traceRuntimeStore) ListSessions(context.Context, repository.AIChatSessionFilter) ([]*repository.AIChatSession, error) {
	return nil, nil
}
func (s *traceRuntimeStore) GetRunByID(_ context.Context, runID, userID string) (*repository.AIChatRun, error) {
	if s.run == nil || s.run.ID != runID || userID != "user-1" {
		return nil, context.Canceled
	}
	return s.run, nil
}
func (s *traceRuntimeStore) ListRunsBySession(context.Context, string, int, int) ([]*repository.AIChatRun, error) {
	return nil, nil
}
func (s *traceRuntimeStore) ListMessagesBySession(context.Context, string, int) ([]*repository.AIChatMessage, error) {
	return nil, nil
}
func (s *traceRuntimeStore) ListRunEvents(_ context.Context, runID string, _, _ int) ([]*repository.AIChatRunEvent, error) {
	result := make([]*repository.AIChatRunEvent, 0)
	for _, event := range s.events {
		if event != nil && event.RunID == runID {
			result = append(result, event)
		}
	}
	return result, nil
}
func (s *traceRuntimeStore) ListArtifactsBySession(context.Context, string, int) ([]*repository.AIChatArtifact, error) {
	return s.artifacts, nil
}
func (s *traceRuntimeStore) GetArtifactByID(context.Context, string, string) (*repository.AIChatArtifact, error) {
	return nil, nil
}
func (s *traceRuntimeStore) UpdateArtifactStatus(context.Context, string, string) error { return nil }
func (s *traceRuntimeStore) ListReviewActionsBySession(context.Context, string, int) ([]*repository.AIChatReviewAction, error) {
	return s.actions, nil
}
func (s *traceRuntimeStore) ListRunAuditLinks(context.Context, string, string) ([]*repository.AgentRunAuditLink, error) {
	return s.links, nil
}
func (s *traceRuntimeStore) ListRunCheckpointAudits(context.Context, string, string) ([]*repository.AgentRunCheckpointAudit, error) {
	return s.checkpoints, nil
}

type traceAuditReader struct {
	rows []*repository.AuditLog
}

func (r *traceAuditReader) List(_ context.Context, filter repository.AuditLogFilter) ([]*repository.AuditLog, int, error) {
	if filter.RunID == "run-1" {
		return r.rows, len(r.rows), nil
	}
	return nil, 0, nil
}

func TestGetAgentRunTraceFiltersSessionDataToOwnedRun(t *testing.T) {
	store := &traceRuntimeStore{
		run:    &repository.AIChatRun{ID: "run-1", SessionID: "session-1", Status: "waiting_review", CreatedAt: time.Now()},
		events: []*repository.AIChatRunEvent{{RunID: "run-1", EventType: "run_end"}},
		artifacts: []*repository.AIChatArtifact{
			{ID: "artifact-1", RunID: "run-1", ArtifactType: "contract_draft"},
			{ID: "artifact-2", RunID: "run-2", ArtifactType: "report_explanation"},
		},
		actions: []*repository.AIChatReviewAction{
			{ID: "action-1", RunID: stringPtrForTrace("run-1")},
			{ID: "action-2", RunID: stringPtrForTrace("run-2")},
		},
		links:       []*repository.AgentRunAuditLink{{ID: "link-1", RunID: "run-1", BusinessTable: "lease_contracts", BusinessRecordID: "contract-1", Relation: "review_draft_created"}},
		checkpoints: []*repository.AgentRunCheckpointAudit{{ID: "checkpoint-1", RunID: "run-1", CheckpointHash: "hash-1", CheckpointStatus: "running"}},
	}
	handler := (&AIChatHandler{runtimeRepo: store}).WithAuditRepository(&traceAuditReader{
		rows: []*repository.AuditLog{{ID: "audit-1", TableName: "agent_tool_executions", RecordID: "audit-1"}},
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "user-1"); c.Next() })
	router.GET("/agent/runs/:id/trace", handler.GetAgentRunTrace)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/runs/run-1/trace", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Artifacts   []repository.AIChatArtifact          `json:"artifacts"`
		Actions     []repository.AIChatReviewAction      `json:"review_actions"`
		Audits      []repository.AuditLog                `json:"tool_audits"`
		Links       []repository.AgentRunAuditLink       `json:"audit_links"`
		Checkpoints []repository.AgentRunCheckpointAudit `json:"checkpoint_audits"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Artifacts) != 1 || payload.Artifacts[0].ID != "artifact-1" {
		t.Fatalf("artifacts=%+v", payload.Artifacts)
	}
	if len(payload.Actions) != 1 || payload.Actions[0].ID != "action-1" {
		t.Fatalf("actions=%+v", payload.Actions)
	}
	if len(payload.Audits) != 1 || payload.Audits[0].ID != "audit-1" {
		t.Fatalf("audits=%+v", payload.Audits)
	}
	if len(payload.Links) != 1 || payload.Links[0].BusinessRecordID != "contract-1" || len(payload.Checkpoints) != 1 {
		t.Fatalf("audit links/checkpoints=%+v/%+v", payload.Links, payload.Checkpoints)
	}
}

func TestGetAgentRunTraceDoesNotRevealOtherOwnerRun(t *testing.T) {
	store := &traceRuntimeStore{run: &repository.AIChatRun{ID: "run-1", SessionID: "session-1"}}
	handler := &AIChatHandler{runtimeRepo: store}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "user-2"); c.Next() })
	router.GET("/agent/runs/:id/trace", handler.GetAgentRunTrace)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/runs/run-1/trace", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func stringPtrForTrace(value string) *string { return &value }
