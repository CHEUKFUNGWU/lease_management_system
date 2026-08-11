package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type workerStreamStore struct {
	run    *repository.AIChatRun
	events []*repository.AIChatRunEvent
}

func (s *workerStreamStore) GetClaimedRun(_ context.Context, runID, workerID, leaseToken string) (*repository.AIChatRun, error) {
	if s.run == nil || s.run.ID != runID || s.run.WorkerID == nil || *s.run.WorkerID != workerID || s.run.LeaseToken != leaseToken {
		return nil, context.Canceled
	}
	copy := *s.run
	return &copy, nil
}

func (s *workerStreamStore) ListClaimedRunEvents(_ context.Context, runID, workerID, leaseToken string, afterSequence, limit int) ([]*repository.AIChatRunEvent, error) {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return nil, err
	}
	result := make([]*repository.AIChatRunEvent, 0)
	for _, event := range s.events {
		if event != nil && event.SequenceNo > afterSequence {
			copy := *event
			result = append(result, &copy)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *workerStreamStore) AppendClaimedRunEvent(context.Context, string, string, string, *repository.AIChatRunEvent) error {
	return nil
}

func (s *workerStreamStore) UpdateClaimedRunStatus(context.Context, string, string, string, string, bool, *string, *string, *time.Time, *time.Time) error {
	return nil
}

func (s *workerStreamStore) SaveClaimedRunCheckpoint(context.Context, string, string, string, json.RawMessage) error {
	return nil
}

func (s *workerStreamStore) GetClaimedRunCheckpoint(context.Context, string, string, string) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func TestWorkerRunStreamUsesLeaseProtectedEventDataPlane(t *testing.T) {
	workerID, leaseToken := "worker-a", "lease-a"
	store := &workerStreamStore{
		run: &repository.AIChatRun{ID: "run-worker", SessionID: "session-1", Status: "running", WorkerID: &workerID, LeaseToken: leaseToken},
		events: []*repository.AIChatRunEvent{{
			RunID: "run-worker", SessionID: "session-1", SequenceNo: 1, EventType: "run_cancelled", IsTerminal: true,
			Payload: json.RawMessage(`{"reason":"operator cancelled"}`),
		}},
	}
	handler := (&AIChatHandler{workerRuns: store})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("permissions", []string{"agent_runtime:worker"})
		c.Next()
	})
	router.GET("/agent/runs/:id/stream", handler.StreamRunEvents)

	request := httptest.NewRequest(http.MethodGet, "/agent/runs/run-worker/stream", nil)
	request.Header.Set("X-Agent-Worker-ID", workerID)
	request.Header.Set("X-Agent-Lease-Token", leaseToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "event: run_meta") || !strings.Contains(body, "event: run_event") || !strings.Contains(body, "event: complete") {
		t.Fatalf("unexpected SSE body=%s", body)
	}
}
