package agentrunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

var ErrNoQueuedRun = fmt.Errorf("no queued agent run")

type HTTPStatusError struct {
	Code int
	Body string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("gateway returned HTTP %d: %s", e.Code, strings.TrimSpace(e.Body))
}

type HTTPGateway struct {
	BaseURL    string
	Token      string
	Client     *http.Client
	WorkerID   string
	LeaseToken string
}

type RunRequest struct {
	SessionID    string `json:"session_id"`
	Message      string `json:"message"`
	SkillID      string `json:"skill_id,omitempty"`
	SkillVersion string `json:"skill_version,omitempty"`
	PageContext  any    `json:"page_context,omitempty"`
}

type SessionRequest struct {
	Title           string `json:"title,omitempty"`
	BoundContractID string `json:"bound_contract_id,omitempty"`
	ContextSnapshot any    `json:"context_snapshot,omitempty"`
}

type RemoteSession struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	LegalEntityID string `json:"legal_entity_id,omitempty"`
	Title         string `json:"title"`
	Status        string `json:"status"`
}

type RemoteRun struct {
	ID             string `json:"id"`
	SessionID      string `json:"session_id"`
	Status         string `json:"status"`
	AgentMode      bool   `json:"agent_mode"`
	SkillID        string `json:"skill_id,omitempty"`
	SkillVersion   string `json:"skill_version,omitempty"`
	ReviewRequired bool   `json:"review_required"`
}

type RunLease struct {
	Run          RemoteRun `json:"run"`
	LeaseToken   string    `json:"lease_token"`
	WorkerID     string    `json:"worker_id"`
	LeaseSeconds int       `json:"lease_seconds"`
}

type RunEvent struct {
	ID         string          `json:"id"`
	RunID      string          `json:"run_id"`
	SessionID  string          `json:"session_id"`
	SequenceNo int             `json:"sequence_no"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
	IsTerminal bool            `json:"is_terminal"`
}

type RunEventPage struct {
	Run    RemoteRun  `json:"run"`
	Events []RunEvent `json:"events"`
}

func NewHTTPGateway(baseURL, token string, client *http.Client) *HTTPGateway {
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPGateway{BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Token: strings.TrimSpace(token), Client: client}
}

// WithWorkerLease returns a transport clone carrying the lease proof on
// worker data-plane requests. Claim/release remain available on the base
// gateway; only Run events and checkpoints need this narrower identity.
func (g *HTTPGateway) WithWorkerLease(workerID, leaseToken string) *HTTPGateway {
	if g == nil {
		return nil
	}
	clone := *g
	clone.WorkerID = strings.TrimSpace(workerID)
	clone.LeaseToken = strings.TrimSpace(leaseToken)
	return &clone
}

func (g *HTTPGateway) Describe(ctx context.Context, filter agenttools.ToolFilter, runID string) ([]agenttools.ToolDescriptor, error) {
	query := url.Values{}
	if filter.SkillID != "" {
		query.Set("skill_id", filter.SkillID)
	}
	if filter.IncludeSchema {
		query.Set("include_schema", "true")
	}
	if runID != "" {
		query.Set("run_id", runID)
	}
	var response struct {
		Tools []agenttools.ToolDescriptor `json:"tools"`
	}
	if err := g.doJSON(ctx, http.MethodGet, "/api/v1/agent/tools", query, nil, "", &response); err != nil {
		return nil, err
	}
	return response.Tools, nil
}

func (g *HTTPGateway) IssueCapability(ctx context.Context, request CapabilityRequest) (string, error) {
	var response struct {
		Token string `json:"capability_token"`
	}
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/capabilities", nil, request, "", &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", fmt.Errorf("gateway returned an empty capability token")
	}
	return response.Token, nil
}

func (g *HTTPGateway) Execute(ctx context.Context, call agenttools.ToolCall, capability string) (agenttools.ToolResult, error) {
	var result agenttools.ToolResult
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/tools/execute", nil, call, capability, &result); err != nil {
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusFailed}, err
	}
	return result, nil
}

func (g *HTTPGateway) RevokeCapability(ctx context.Context, runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("run ID is required for capability revocation")
	}
	return g.doJSON(ctx, http.MethodPost, "/api/v1/agent/capabilities/revoke", nil, map[string]string{"run_id": runID}, "", nil)
}

func (g *HTTPGateway) CreateRun(ctx context.Context, request RunRequest) (RemoteRun, error) {
	var response struct {
		Run RemoteRun `json:"run"`
	}
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/runs", nil, request, "", &response); err != nil {
		return RemoteRun{}, err
	}
	if strings.TrimSpace(response.Run.ID) == "" {
		return RemoteRun{}, fmt.Errorf("gateway returned an empty run ID")
	}
	return response.Run, nil
}

func (g *HTTPGateway) ClaimRun(ctx context.Context, workerID string, leaseSeconds int) (RunLease, error) {
	var response RunLease
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/runs/claim", nil, map[string]any{
		"worker_id": workerID, "lease_seconds": leaseSeconds,
	}, "", &response); err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) && statusErr.Code == http.StatusNoContent {
			return RunLease{}, ErrNoQueuedRun
		}
		return RunLease{}, err
	}
	if strings.TrimSpace(response.Run.ID) == "" || strings.TrimSpace(response.LeaseToken) == "" {
		return RunLease{}, fmt.Errorf("gateway returned an incomplete run lease")
	}
	return response, nil
}

func (g *HTTPGateway) HeartbeatRunLease(ctx context.Context, runID, workerID, leaseToken string, leaseSeconds int) error {
	path := "/api/v1/agent/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/lease/heartbeat"
	return g.doJSON(ctx, http.MethodPost, path, nil, map[string]any{
		"worker_id": workerID, "lease_token": leaseToken, "lease_seconds": leaseSeconds,
	}, "", nil)
}

func (g *HTTPGateway) ReleaseRunLease(ctx context.Context, runID, workerID, leaseToken string, requeue bool) error {
	path := "/api/v1/agent/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/lease/release"
	return g.doJSON(ctx, http.MethodPost, path, nil, map[string]any{
		"worker_id": workerID, "lease_token": leaseToken, "requeue": requeue,
	}, "", nil)
}

func (g *HTTPGateway) RecoverRunLeases(ctx context.Context) (int, error) {
	var response struct {
		Recovered int `json:"recovered"`
	}
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/runs/recover-leases", nil, nil, "", &response); err != nil {
		return 0, err
	}
	return response.Recovered, nil
}

func (g *HTTPGateway) CreateSession(ctx context.Context, request SessionRequest) (RemoteSession, error) {
	var response struct {
		Session RemoteSession `json:"session"`
	}
	if err := g.doJSON(ctx, http.MethodPost, "/api/v1/agent/sessions", nil, request, "", &response); err != nil {
		return RemoteSession{}, err
	}
	if strings.TrimSpace(response.Session.ID) == "" {
		return RemoteSession{}, fmt.Errorf("gateway returned an empty session ID")
	}
	return response.Session, nil
}

func (g *HTTPGateway) ListRunEvents(ctx context.Context, runID string, afterSequence, limit int) (RunEventPage, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunEventPage{}, fmt.Errorf("run ID is required")
	}
	query := url.Values{}
	if afterSequence > 0 {
		query.Set("after_sequence", fmt.Sprintf("%d", afterSequence))
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	var response RunEventPage
	if err := g.doJSON(ctx, http.MethodGet, "/api/v1/agent/runs/"+url.PathEscape(runID)+"/events", query, nil, "", &response); err != nil {
		return RunEventPage{}, err
	}
	return response, nil
}

// SubscribeRunEvents connects to Core's SSE stream. Worker clones carry the
// lease proof on this request, so the live stream has the same authorization
// boundary as worker event reads and writes.
func (g *HTTPGateway) SubscribeRunEvents(ctx context.Context, runID string, afterSequence int) (*RunEventSubscription, error) {
	if g == nil || g.Client == nil || g.BaseURL == "" || g.Token == "" {
		return nil, fmt.Errorf("gateway base URL and bearer token are required")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run ID is required")
	}
	query := url.Values{}
	if afterSequence > 0 {
		query.Set("after_sequence", fmt.Sprintf("%d", afterSequence))
	}
	requestURL := g.BaseURL + "/api/v1/agent/runs/" + url.PathEscape(runID) + "/stream"
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Authorization", bearerToken(g.Token))
	if g.WorkerID != "" || g.LeaseToken != "" {
		request.Header.Set("X-Agent-Worker-ID", g.WorkerID)
		request.Header.Set("X-Agent-Lease-Token", g.LeaseToken)
	}
	response, err := g.Client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		_ = response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		return nil, &HTTPStatusError{Code: response.StatusCode, Body: string(raw)}
	}
	events := make(chan RunEvent, 256)
	errorsChannel := make(chan error, 1)
	subscriptionContext, cancel := context.WithCancel(ctx)
	go func() {
		defer close(events)
		defer close(errorsChannel)
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 4096), 2<<20)
		eventName := ""
		dataLines := make([]string, 0, 1)
		flush := func() bool {
			if len(dataLines) == 0 {
				eventName = ""
				return true
			}
			data := strings.Join(dataLines, "\n")
			dataLines = dataLines[:0]
			switch eventName {
			case "run_event":
				var envelope struct {
					Event RunEvent `json:"event"`
				}
				if err := json.Unmarshal([]byte(data), &envelope); err != nil {
					errorsChannel <- fmt.Errorf("decode run event SSE payload: %w", err)
					return false
				}
				select {
				case events <- envelope.Event:
				case <-subscriptionContext.Done():
					return false
				}
			case "error":
				var payload struct {
					Error string `json:"error"`
				}
				if json.Unmarshal([]byte(data), &payload) == nil && strings.TrimSpace(payload.Error) != "" {
					errorsChannel <- errors.New(strings.TrimSpace(payload.Error))
					return false
				}
			}
			eventName = ""
			return true
		}
		for scanner.Scan() {
			select {
			case <-subscriptionContext.Done():
				return
			default:
			}
			line := scanner.Text()
			switch {
			case line == "":
				if !flush() {
					return
				}
			case strings.HasPrefix(line, "event:"):
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			case strings.HasPrefix(line, ":"):
				// SSE comment/heartbeat.
			}
		}
		if len(dataLines) > 0 && !flush() {
			return
		}
		if err := scanner.Err(); err != nil && subscriptionContext.Err() == nil {
			errorsChannel <- fmt.Errorf("read run event SSE stream: %w", err)
		}
	}()
	return &RunEventSubscription{Events: events, Errors: errorsChannel, Close: cancel}, nil
}

// LoadRunInstruction reconstructs the initial instruction from the durable
// message_start event. A queue worker can therefore claim a Run without
// receiving the user's message through an out-of-band channel.
func (g *HTTPGateway) LoadRunInstruction(ctx context.Context, runID string) (string, error) {
	page, err := g.ListRunEvents(ctx, runID, 0, 100)
	if err != nil {
		return "", err
	}
	for _, event := range page.Events {
		if event.EventType != "message_start" {
			continue
		}
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return "", fmt.Errorf("decode run instruction: %w", err)
		}
		if strings.TrimSpace(payload.Message) != "" {
			return strings.TrimSpace(payload.Message), nil
		}
	}
	return "", fmt.Errorf("run %s has no message_start instruction", runID)
}

func (g *HTTPGateway) AppendRunEvent(ctx context.Context, runID string, event Event) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("run ID is required")
	}
	payload := map[string]any{"type": event.Type, "call_id": event.CallID, "payload": event.Payload}
	return g.doJSON(ctx, http.MethodPost, "/api/v1/agent/runs/"+url.PathEscape(runID)+"/events", nil, payload, "", nil)
}

func (g *HTTPGateway) SaveCheckpoint(ctx context.Context, checkpoint Checkpoint) error {
	if strings.TrimSpace(checkpoint.RunID) == "" {
		return fmt.Errorf("checkpoint run ID is required")
	}
	return g.doJSON(ctx, http.MethodPost, "/api/v1/agent/runs/"+url.PathEscape(checkpoint.RunID)+"/checkpoint", nil, map[string]any{"checkpoint": checkpoint}, "", nil)
}

func (g *HTTPGateway) LoadCheckpoint(ctx context.Context, runID string) (Checkpoint, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return Checkpoint{}, fmt.Errorf("run ID is required")
	}
	var response struct {
		Checkpoint Checkpoint `json:"checkpoint"`
	}
	if err := g.doJSON(ctx, http.MethodGet, "/api/v1/agent/runs/"+url.PathEscape(runID)+"/checkpoint", nil, nil, "", &response); err != nil {
		return Checkpoint{}, err
	}
	if response.Checkpoint.RunID != "" && response.Checkpoint.RunID != runID {
		return Checkpoint{}, fmt.Errorf("checkpoint run ID mismatch")
	}
	return response.Checkpoint, nil
}

// Save and Load make HTTPGateway a Runner CheckpointStore without exposing
// transport details to the Runner itself.
func (g *HTTPGateway) Save(ctx context.Context, checkpoint Checkpoint) error {
	return g.SaveCheckpoint(ctx, checkpoint)
}

func (g *HTTPGateway) Load(ctx context.Context, runID string) (Checkpoint, error) {
	return g.LoadCheckpoint(ctx, runID)
}

// Record implements EventRecorder so a Runner can stream its protocol events
// back to Core's durable Run event log.
func (g *HTTPGateway) Record(ctx context.Context, event Event) error {
	return g.AppendRunEvent(ctx, event.RunID, event)
}

func (g *HTTPGateway) CancelRun(ctx context.Context, runID string) error {
	return g.runControl(ctx, runID, "cancel", nil)
}

func (g *HTTPGateway) SteerRun(ctx context.Context, runID, instruction string) error {
	return g.runControl(ctx, runID, "steer", map[string]string{"instruction": instruction})
}

func (g *HTTPGateway) FollowUpRun(ctx context.Context, runID, instruction string) error {
	return g.runControl(ctx, runID, "follow-up", map[string]string{"instruction": instruction})
}

func (g *HTTPGateway) runControl(ctx context.Context, runID, action string, payload any) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("run ID is required")
	}
	path := "/api/v1/agent/runs/" + url.PathEscape(runID) + "/" + action
	return g.doJSON(ctx, http.MethodPost, path, nil, payload, "", nil)
}

func (g *HTTPGateway) doJSON(ctx context.Context, method, path string, query url.Values, payload any, capability string, target any) error {
	if g == nil || g.Client == nil || g.BaseURL == "" || g.Token == "" {
		return fmt.Errorf("gateway base URL and bearer token are required")
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	requestURL := g.BaseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", bearerToken(g.Token))
	if g.WorkerID != "" || g.LeaseToken != "" {
		request.Header.Set("X-Agent-Worker-ID", g.WorkerID)
		request.Header.Set("X-Agent-Lease-Token", g.LeaseToken)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(capability) != "" {
		request.Header.Set("X-Agent-Capability", strings.TrimSpace(capability))
	}
	response, err := g.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNoContent {
		return &HTTPStatusError{Code: response.StatusCode, Body: string(raw)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPStatusError{Code: response.StatusCode, Body: string(raw)}
	}
	if target == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode gateway response: %w", err)
	}
	return nil
}

func bearerToken(token string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(token)), "bearer ") {
		return strings.TrimSpace(token)
	}
	return "Bearer " + strings.TrimSpace(token)
}
