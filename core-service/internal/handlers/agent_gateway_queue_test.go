package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

type memoryAgentRunQueue struct {
	run       *repository.AIChatRun
	token     string
	worker    string
	released  bool
	requeued  bool
	heartbeat bool
}

func (q *memoryAgentRunQueue) ClaimQueuedRun(context.Context, string, time.Duration) (*repository.AIChatRun, string, error) {
	if q.run == nil {
		return nil, "", repository.ErrNoQueuedAgentRun
	}
	return q.run, q.token, nil
}
func (q *memoryAgentRunQueue) HeartbeatRunLease(context.Context, string, string, string, time.Duration) error {
	q.heartbeat = true
	return nil
}
func (q *memoryAgentRunQueue) ReleaseRunLease(_ context.Context, _ string, _ string, _ string, requeue bool) error {
	q.released, q.requeued = true, requeue
	return nil
}
func (q *memoryAgentRunQueue) RecoverExpiredRunLeases(context.Context) (int, error) { return 2, nil }

func newWorkerGatewayTestRouter(queue AgentRunQueueStore, permissions []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "worker-user")
		c.Set("role", "admin")
		c.Set("permissions", permissions)
		scope := access.Scope{LegalEntityID: "le-001"}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(nil).WithQueueStore(queue)
	router.POST("/agent/runs/claim", handler.ClaimRun)
	router.POST("/agent/runs/recover-leases", handler.RecoverRunLeases)
	router.POST("/agent/runs/:id/lease/heartbeat", handler.HeartbeatRunLease)
	router.POST("/agent/runs/:id/lease/release", handler.ReleaseRunLease)
	return router
}

func TestAgentGatewayWorkerLeaseLifecycleRequiresWorkerPermission(t *testing.T) {
	queue := &memoryAgentRunQueue{
		run:   &repository.AIChatRun{ID: "run-1", SessionID: "session-1", Status: "running"},
		token: "lease-1",
	}
	router := newWorkerGatewayTestRouter(queue, []string{"agent_runtime:worker"})

	claim := httptest.NewRecorder()
	claimRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/claim", strings.NewReader(`{"worker_id":"worker-a","lease_seconds":90}`))
	claimRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(claim, claimRequest)
	if claim.Code != http.StatusOK || !strings.Contains(claim.Body.String(), "lease-1") {
		t.Fatalf("claim status=%d body=%s", claim.Code, claim.Body.String())
	}

	heartbeat := httptest.NewRecorder()
	heartbeatRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/run-1/lease/heartbeat", strings.NewReader(`{"worker_id":"worker-a","lease_token":"lease-1"}`))
	heartbeatRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(heartbeat, heartbeatRequest)
	if heartbeat.Code != http.StatusAccepted || !queue.heartbeat {
		t.Fatalf("heartbeat status=%d body=%s queue=%+v", heartbeat.Code, heartbeat.Body.String(), queue)
	}

	release := httptest.NewRecorder()
	releaseRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/run-1/lease/release", strings.NewReader(`{"worker_id":"worker-a","lease_token":"lease-1","requeue":true}`))
	releaseRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(release, releaseRequest)
	if release.Code != http.StatusAccepted || !queue.released || !queue.requeued {
		t.Fatalf("release status=%d body=%s queue=%+v", release.Code, release.Body.String(), queue)
	}

	recover := httptest.NewRecorder()
	router.ServeHTTP(recover, httptest.NewRequest(http.MethodPost, "/agent/runs/recover-leases", nil))
	if recover.Code != http.StatusOK || !strings.Contains(recover.Body.String(), `"recovered":2`) {
		t.Fatalf("recover status=%d body=%s", recover.Code, recover.Body.String())
	}
}

func TestAgentGatewayWorkerLeaseRejectsMissingPermission(t *testing.T) {
	router := newWorkerGatewayTestRouter(&memoryAgentRunQueue{}, []string{"ai_chat:use"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/agent/runs/claim", strings.NewReader(`{"worker_id":"worker-a"}`)))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
