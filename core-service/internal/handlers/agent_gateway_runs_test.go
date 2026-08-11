package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentskill"
	"github.com/lease-management-system/core-service/internal/repository"
)

type memoryAgentRunStore struct {
	mu          sync.Mutex
	sessions    map[string]*repository.AIChatSession
	runs        map[string]*repository.AIChatRun
	events      map[string][]*repository.AIChatRunEvent
	checkpoints map[string]json.RawMessage
	nextRun     int
}

func newMemoryAgentRunStore() *memoryAgentRunStore {
	return &memoryAgentRunStore{
		sessions: map[string]*repository.AIChatSession{
			"session-1": {ID: "session-1", UserID: "user-1", Status: "active"},
		},
		runs:        make(map[string]*repository.AIChatRun),
		events:      make(map[string][]*repository.AIChatRunEvent),
		checkpoints: make(map[string]json.RawMessage),
	}
}

func (s *memoryAgentRunStore) GetSessionByID(_ context.Context, sessionID, userID string) (*repository.AIChatSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[sessionID]
	if session == nil || session.UserID != userID {
		return nil, context.Canceled
	}
	copy := *session
	return &copy, nil
}

func (s *memoryAgentRunStore) CreateSession(_ context.Context, session *repository.AIChatSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session.ID == "" {
		session.ID = "session-created"
	}
	s.sessions[session.ID] = session
	return nil
}

func (s *memoryAgentRunStore) GetContractAttributes(_ context.Context, contractID string) (access.ContractAttributes, bool, error) {
	if contractID == "contract-allowed" {
		return access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-allowed"}, true, nil
	}
	if contractID == "contract-foreign" {
		return access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-foreign"}, true, nil
	}
	return access.ContractAttributes{}, false, nil
}

func (s *memoryAgentRunStore) CreateRun(_ context.Context, run *repository.AIChatRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRun++
	if run.ID == "" {
		run.ID = "run-" + string(rune('0'+s.nextRun))
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	copy := *run
	s.runs[run.ID] = &copy
	if len(run.Checkpoint) > 0 {
		s.checkpoints[run.ID] = append(json.RawMessage(nil), run.Checkpoint...)
	}
	return nil
}

func (s *memoryAgentRunStore) GetRunByID(_ context.Context, runID, userID string) (*repository.AIChatRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run == nil || s.sessions[run.SessionID] == nil || s.sessions[run.SessionID].UserID != userID {
		return nil, context.Canceled
	}
	copy := *run
	return &copy, nil
}

func (s *memoryAgentRunStore) UpdateRunStatus(_ context.Context, runID, status string, reviewRequired bool, summaryText, errorMessage *string, startedAt, completedAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run == nil {
		return context.Canceled
	}
	run.Status, run.ReviewRequired = status, reviewRequired
	run.SummaryText, run.ErrorMessage = summaryText, errorMessage
	run.StartedAt, run.CompletedAt = startedAt, completedAt
	return nil
}

func (s *memoryAgentRunStore) AppendRunEvent(_ context.Context, event *repository.AIChatRunEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *event
	copy.CreatedAt = time.Now().UTC()
	if copy.ID == "" {
		copy.ID = "event-" + string(rune('0'+copy.SequenceNo))
	}
	s.events[event.RunID] = append(s.events[event.RunID], &copy)
	return nil
}

func (s *memoryAgentRunStore) GetNextRunEventSequence(_ context.Context, runID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events[runID]) + 1, nil
}

func (s *memoryAgentRunStore) ListRunEvents(_ context.Context, runID string, afterSequence, limit int) ([]*repository.AIChatRunEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*repository.AIChatRunEvent, 0)
	for _, event := range s.events[runID] {
		if event.SequenceNo <= afterSequence {
			continue
		}
		copy := *event
		result = append(result, &copy)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *memoryAgentRunStore) SaveRunCheckpoint(_ context.Context, runID, userID string, checkpoint json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run == nil || s.sessions[run.SessionID] == nil || s.sessions[run.SessionID].UserID != userID {
		return context.Canceled
	}
	s.checkpoints[runID] = append(json.RawMessage(nil), checkpoint...)
	return nil
}

func (s *memoryAgentRunStore) GetRunCheckpoint(_ context.Context, runID, userID string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run == nil || s.sessions[run.SessionID] == nil || s.sessions[run.SessionID].UserID != userID {
		return nil, context.Canceled
	}
	return append(json.RawMessage(nil), s.checkpoints[runID]...), nil
}

func newRunGatewayTestRouter(store *memoryAgentRunStore) *gin.Engine {
	return newRunGatewayTestRouterWithPermissions(store, []string{"ai_chat:use"})
}

func newRunGatewayTestRouterWithPermissions(store *memoryAgentRunStore, permissions []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("role", "editor")
		c.Set("permissions", permissions)
		scope := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-allowed"}}
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := NewAgentGatewayHandler(nil).WithSkillRegistry(agentskill.ProductionRegistry()).WithSessionStore(store).WithContractScopeReader(store).WithRunStore(store).WithCheckpointStore(store).WithWorkerRunStore(store)
	router.POST("/agent/runs", handler.CreateRun)
	router.POST("/agent/sessions", handler.CreateSession)
	router.GET("/agent/runs/:id/events", handler.ListRunEvents)
	router.GET("/agent/runs/:id/checkpoint", handler.GetRunCheckpoint)
	router.POST("/agent/runs/:id/events", handler.AppendRunEvent)
	router.POST("/agent/runs/:id/checkpoint", handler.SaveRunCheckpoint)
	router.POST("/agent/runs/:id/cancel", handler.CancelRun)
	router.POST("/agent/runs/:id/steer", handler.SteerRun)
	router.POST("/agent/runs/:id/follow-up", handler.FollowUpRun)
	router.POST("/agent/runs/:id/branch", handler.BranchRun)
	return router
}

func (s *memoryAgentRunStore) GetClaimedRun(_ context.Context, runID, workerID, leaseToken string) (*repository.AIChatRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[runID]
	if run == nil || run.WorkerID == nil || *run.WorkerID != workerID || run.LeaseToken != leaseToken || run.LeasedUntil == nil || !run.LeasedUntil.After(time.Now()) {
		return nil, repository.ErrAgentRunLeaseLost
	}
	copy := *run
	return &copy, nil
}

func (s *memoryAgentRunStore) ListClaimedRunEvents(_ context.Context, runID, workerID, leaseToken string, afterSequence, limit int) ([]*repository.AIChatRunEvent, error) {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return nil, err
	}
	return s.ListRunEvents(context.Background(), runID, afterSequence, limit)
}

func (s *memoryAgentRunStore) AppendClaimedRunEvent(_ context.Context, runID, workerID, leaseToken string, event *repository.AIChatRunEvent) error {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return err
	}
	sequence, err := s.GetNextRunEventSequence(context.Background(), runID)
	if err != nil {
		return err
	}
	event.SequenceNo = sequence
	return s.AppendRunEvent(context.Background(), event)
}

func (s *memoryAgentRunStore) UpdateClaimedRunStatus(_ context.Context, runID, workerID, leaseToken, status string, reviewRequired bool, summaryText, errorMessage *string, startedAt, completedAt *time.Time) error {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return err
	}
	return s.UpdateRunStatus(context.Background(), runID, status, reviewRequired, summaryText, errorMessage, startedAt, completedAt)
}

func (s *memoryAgentRunStore) SaveClaimedRunCheckpoint(_ context.Context, runID, workerID, leaseToken string, checkpoint json.RawMessage) error {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[runID] = append(json.RawMessage(nil), checkpoint...)
	return nil
}

func (s *memoryAgentRunStore) GetClaimedRunCheckpoint(_ context.Context, runID, workerID, leaseToken string) (json.RawMessage, error) {
	if _, err := s.GetClaimedRun(context.Background(), runID, workerID, leaseToken); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append(json.RawMessage(nil), s.checkpoints[runID]...), nil
}

func TestAgentGatewayCreatesSessionFromAuthenticatedScope(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/sessions", stringsReader(`{"title":"CLI review","context_snapshot":{"source":"cli"}}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	created := store.sessions["session-created"]
	if created == nil || created.UserID != "user-1" || created.LegalEntityID == nil || *created.LegalEntityID != "le-001" {
		t.Fatalf("session=%+v", created)
	}
}

func TestAgentGatewayRejectsOutOfScopeBoundContract(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/sessions", stringsReader(`{"bound_contract_id":"contract-foreign"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAgentGatewayRunLifecyclePersistsEventsAndControls(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)

	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/runs", stringsReader(`{"session_id":"session-1","message":"审阅合同","skill_id":"contract_review","page_context":{"page":"contract-detail","contract_id":"contract-allowed"}}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Run repository.AIChatRun `json:"run"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Run.ID == "" || created.Run.SkillID == nil || *created.Run.SkillID != "contract_review" {
		t.Fatalf("created run=%+v", created.Run)
	}

	events := httptest.NewRecorder()
	router.ServeHTTP(events, httptest.NewRequest(http.MethodGet, "/agent/runs/"+created.Run.ID+"/events", nil))
	if events.Code != http.StatusOK || !containsRunEvent(events.Body.Bytes(), "message_start") {
		t.Fatalf("events status=%d body=%s", events.Code, events.Body.String())
	}

	steer := httptest.NewRecorder()
	steerRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/steer", stringsReader(`{"instruction":"只关注租赁期限"}`))
	steerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(steer, steerRequest)
	if steer.Code != http.StatusAccepted {
		t.Fatalf("steer status=%d body=%s", steer.Code, steer.Body.String())
	}

	cancel := httptest.NewRecorder()
	router.ServeHTTP(cancel, httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/cancel", nil))
	if cancel.Code != http.StatusAccepted {
		t.Fatalf("cancel status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	run, err := store.GetRunByID(context.Background(), created.Run.ID, "user-1")
	if err != nil || run.Status != "cancelled" {
		t.Fatalf("run=%+v err=%v", run, err)
	}

	followUp := httptest.NewRecorder()
	followUpRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/follow-up", stringsReader(`{"instruction":"补充关键日期"}`))
	followUpRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(followUp, followUpRequest)
	if followUp.Code != http.StatusCreated {
		t.Fatalf("follow-up status=%d body=%s", followUp.Code, followUp.Body.String())
	}
	var followUpBody struct {
		Run         repository.AIChatRun `json:"run"`
		ParentRunID string               `json:"parent_run_id"`
	}
	if err := json.Unmarshal(followUp.Body.Bytes(), &followUpBody); err != nil {
		t.Fatal(err)
	}
	child := store.runs[followUpBody.Run.ID]
	if child == nil || child.Status != "queued" || child.ParentRunID == nil || *child.ParentRunID != created.Run.ID || followUpBody.ParentRunID != created.Run.ID {
		t.Fatalf("follow-up child=%+v parent=%q", child, followUpBody.ParentRunID)
	}
	if string(child.PageContext) != `{"page":"contract-detail","contract_id":"contract-allowed"}` {
		t.Fatalf("follow-up page context=%s", child.PageContext)
	}
	if child.SkillID == nil || *child.SkillID != "contract_review" || child.SkillVersion == nil || *child.SkillVersion != "v1" {
		t.Fatalf("follow-up skill=%v version=%v", child.SkillID, child.SkillVersion)
	}
	if len(store.events[created.Run.ID]) != 4 {
		t.Fatalf("events=%d, want message_start + steer + follow-up + cancel", len(store.events[created.Run.ID]))
	}
}

func containsRunEvent(raw []byte, eventType string) bool {
	var response struct {
		Events []repository.AIChatRunEvent `json:"events"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return false
	}
	for _, event := range response.Events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func TestAgentGatewayRunEventPaginationRejectsInvalidInteger(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)
	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/runs", stringsReader(`{"session_id":"session-1","message":"inspect"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	var created struct {
		Run repository.AIChatRun `json:"run"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &created)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent/runs/"+created.Run.ID+"/events?after_sequence=bad", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAgentGatewayPersistsOwnedRunCheckpoint(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)
	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/runs", stringsReader(`{"session_id":"session-1","message":"inspect"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	var created struct {
		Run repository.AIChatRun `json:"run"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	save := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/checkpoint", stringsReader(`{"checkpoint":{"run_id":"`+created.Run.ID+`","next_index":1,"status":"paused"}}`))
	saveRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(save, saveRequest)
	if save.Code != http.StatusAccepted {
		t.Fatalf("save status=%d body=%s", save.Code, save.Body.String())
	}
	load := httptest.NewRecorder()
	router.ServeHTTP(load, httptest.NewRequest(http.MethodGet, "/agent/runs/"+created.Run.ID+"/checkpoint", nil))
	if load.Code != http.StatusOK || !strings.Contains(load.Body.String(), `"next_index":1`) {
		t.Fatalf("load status=%d body=%s", load.Code, load.Body.String())
	}
}

func TestAgentGatewayWorkerCanUseOnlyItsClaimedRunDataPlane(t *testing.T) {
	store := newMemoryAgentRunStore()
	workerID, leaseToken := "worker-a", "lease-a"
	leasedUntil := time.Now().Add(time.Minute)
	worker := &repository.AIChatRun{ID: "run-claimed", SessionID: "session-1", Status: "running", WorkerID: &workerID, LeaseToken: leaseToken, LeasedUntil: &leasedUntil}
	store.runs[worker.ID] = worker
	store.events[worker.ID] = []*repository.AIChatRunEvent{{ID: "event-1", RunID: worker.ID, SessionID: worker.SessionID, SequenceNo: 1, EventType: "message_start", Payload: json.RawMessage(`{"message":"queued"}`)}}
	router := newRunGatewayTestRouterWithPermissions(store, []string{"agent_runtime:worker"})
	withLease := func(request *http.Request) {
		request.Header.Set("X-Agent-Worker-ID", workerID)
		request.Header.Set("X-Agent-Lease-Token", leaseToken)
	}

	events := httptest.NewRecorder()
	eventsRequest := httptest.NewRequest(http.MethodGet, "/agent/runs/"+worker.ID+"/events", nil)
	withLease(eventsRequest)
	router.ServeHTTP(events, eventsRequest)
	if events.Code != http.StatusOK || !containsRunEvent(events.Body.Bytes(), "message_start") {
		t.Fatalf("worker events status=%d body=%s", events.Code, events.Body.String())
	}

	appendResponse := httptest.NewRecorder()
	appendRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+worker.ID+"/events", stringsReader(`{"type":"run_started","payload":{"status":"running"}}`))
	appendRequest.Header.Set("Content-Type", "application/json")
	withLease(appendRequest)
	router.ServeHTTP(appendResponse, appendRequest)
	if appendResponse.Code != http.StatusAccepted || len(store.events[worker.ID]) != 2 {
		t.Fatalf("worker append status=%d body=%s events=%d", appendResponse.Code, appendResponse.Body.String(), len(store.events[worker.ID]))
	}

	save := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+worker.ID+"/checkpoint", stringsReader(`{"checkpoint":{"run_id":"run-claimed","next_index":2}}`))
	saveRequest.Header.Set("Content-Type", "application/json")
	withLease(saveRequest)
	router.ServeHTTP(save, saveRequest)
	if save.Code != http.StatusAccepted {
		t.Fatalf("worker checkpoint save status=%d body=%s", save.Code, save.Body.String())
	}

	control := httptest.NewRecorder()
	controlRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+worker.ID+"/steer", stringsReader(`{"instruction":"no"}`))
	controlRequest.Header.Set("Content-Type", "application/json")
	withLease(controlRequest)
	router.ServeHTTP(control, controlRequest)
	if control.Code != http.StatusForbidden {
		t.Fatalf("worker control status=%d body=%s", control.Code, control.Body.String())
	}

	otherLease := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodGet, "/agent/runs/"+worker.ID+"/events", nil)
	otherRequest.Header.Set("X-Agent-Worker-ID", "worker-b")
	otherRequest.Header.Set("X-Agent-Lease-Token", "lease-b")
	router.ServeHTTP(otherLease, otherRequest)
	if otherLease.Code != http.StatusNotFound {
		t.Fatalf("other worker status=%d body=%s", otherLease.Code, otherLease.Body.String())
	}
}

func TestAgentGatewayBranchesFromOwnedCheckpoint(t *testing.T) {
	store := newMemoryAgentRunStore()
	router := newRunGatewayTestRouter(store)
	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent/runs", stringsReader(`{"session_id":"session-1","message":"inspect"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	var created struct {
		Run repository.AIChatRun `json:"run"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	save := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/checkpoint", stringsReader(`{"checkpoint":{"run_id":"`+created.Run.ID+`","next_index":3,"facts":["verified"]}}`))
	saveRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(save, saveRequest)
	if save.Code != http.StatusAccepted {
		t.Fatalf("save status=%d body=%s", save.Code, save.Body.String())
	}
	branch := httptest.NewRecorder()
	branchRequest := httptest.NewRequest(http.MethodPost, "/agent/runs/"+created.Run.ID+"/branch", stringsReader(`{"message":"从当前证据分支分析付款时点"}`))
	branchRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(branch, branchRequest)
	if branch.Code != http.StatusCreated {
		t.Fatalf("branch status=%d body=%s", branch.Code, branch.Body.String())
	}
	var response struct {
		Run repository.AIChatRun `json:"run"`
	}
	if err := json.Unmarshal(branch.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Run.ID == "" || response.Run.ParentRunID == nil || *response.Run.ParentRunID != created.Run.ID {
		t.Fatalf("branch run=%+v", response.Run)
	}
	if got := string(store.checkpoints[response.Run.ID]); !strings.Contains(got, `"next_index":3`) {
		t.Fatalf("branch checkpoint=%s", got)
	}
	events := store.events[created.Run.ID]
	if len(events) == 0 || events[len(events)-1].EventType != "run_branch_created" {
		t.Fatalf("parent events=%+v", events)
	}
}
