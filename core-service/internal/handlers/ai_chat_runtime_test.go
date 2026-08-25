package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestReverseMessagesDoesNotMutateRepositoryOrder(t *testing.T) {
	original := []*repository.AIChatMessage{{ID: "m1"}, {ID: "m2"}, {ID: "m3"}}

	reversed := reverseMessages(original)

	if got, want := reversed[0].ID, "m3"; got != want {
		t.Fatalf("reversed[0] = %s, want %s", got, want)
	}
	if got, want := original[0].ID, "m1"; got != want {
		t.Fatalf("repository order mutated: original[0] = %s, want %s", got, want)
	}
}


// CHAT-001: the user-facing session list defaults to user-initiated
// sessions; ?include_system=true lifts the filter. Pins the handler
// contract with a stub that records the filter, no database required.
type sessionListerStub struct {
	gotFilter repository.AIChatSessionFilter
}

func (s *sessionListerStub) GetSessionByID(context.Context, string, string, access.EntityFilter) (*repository.AIChatSession, error) {
	return nil, nil
}
func (s *sessionListerStub) ListSessions(ctx context.Context, filter repository.AIChatSessionFilter) ([]*repository.AIChatSession, error) {
	s.gotFilter = filter
	return nil, nil
}
func (s *sessionListerStub) GetRunByID(context.Context, string, string, access.EntityFilter) (*repository.AIChatRun, error) {
	return nil, nil
}
func (s *sessionListerStub) ListRunsBySession(context.Context, string, string, access.EntityFilter, int, int) ([]*repository.AIChatRun, error) {
	return nil, nil
}
func (s *sessionListerStub) ListMessagesBySession(context.Context, string, string, access.EntityFilter, int) ([]*repository.AIChatMessage, error) {
	return nil, nil
}
func (s *sessionListerStub) ListRunEvents(context.Context, string, int, int, access.EntityFilter, string) ([]*repository.AIChatRunEvent, error) {
	return nil, nil
}
func (s *sessionListerStub) ListArtifactsBySession(context.Context, string, string, access.EntityFilter, int) ([]*repository.AIChatArtifact, error) {
	return nil, nil
}
func (s *sessionListerStub) GetArtifactByID(context.Context, string, string, access.EntityFilter) (*repository.AIChatArtifact, error) {
	return nil, nil
}
func (s *sessionListerStub) UpdateArtifactStatus(context.Context, string, string) error {
	return nil
}
func (s *sessionListerStub) ListReviewActionsBySession(context.Context, string, string, access.EntityFilter, int) ([]*repository.AIChatReviewAction, error) {
	return nil, nil
}

func newSessionListRouter(stub *sessionListerStub, scope access.Scope) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := &AIChatHandler{runtimeRepo: stub}
	router.GET("/sessions", handler.ListSessions)
	return router
}

func TestListSessionsDefaultsToExcludingSystem(t *testing.T) {
	scope := access.Scope{LegalEntityID: "le-1"}
	stub := &sessionListerStub{}
	router := newSessionListRouter(stub, scope)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body.String())
	}
	if stub.gotFilter.ExcludeInitiator != "system" {
		t.Fatalf("default ExcludeInitiator = %q; want %q", stub.gotFilter.ExcludeInitiator, "system")
	}
}

func TestListSessionsIncludeSystemLiftsFilter(t *testing.T) {
	scope := access.Scope{LegalEntityID: "le-1"}
	stub := &sessionListerStub{}
	router := newSessionListRouter(stub, scope)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sessions?include_system=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body.String())
	}
	if stub.gotFilter.ExcludeInitiator != "" {
		t.Fatalf("include_system=true: ExcludeInitiator = %q; want empty", stub.gotFilter.ExcludeInitiator)
	}
}
