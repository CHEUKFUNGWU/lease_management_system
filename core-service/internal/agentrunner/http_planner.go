package agentrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// HTTPPlanner delegates only planning to the AI Service. It never receives a
// database handle, MinIO credential or authority to execute a Tool; the
// Runner still validates every returned call against the server-owned
// descriptors before execution.
type HTTPPlanner struct {
	BaseURL       string
	Token         string
	Client        *http.Client
	UsageRecorder func(context.Context, string, PlannerUsage) error
}

type plannerRequest struct {
	RunID            string                      `json:"run_id"`
	SessionID        string                      `json:"session_id,omitempty"`
	Message          string                      `json:"message"`
	SkillID          string                      `json:"skill_id,omitempty"`
	SkillVersion     string                      `json:"skill_version,omitempty"`
	Tools            []agenttools.ToolDescriptor `json:"tools"`
	CompletedResults []agenttools.ToolResult     `json:"completed_results,omitempty"`
	SteerInstruction string                      `json:"steer_instruction,omitempty"`
}

type plannerResponse struct {
	ToolCalls []agenttools.ToolCall `json:"tool_calls"`
	Model     string                `json:"model,omitempty"`
	Usage     *PlannerUsage         `json:"usage,omitempty"`
}

func NewHTTPPlanner(baseURL, token string, client *http.Client) *HTTPPlanner {
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPPlanner{BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Token: strings.TrimSpace(token), Client: client}
}

// WithUsageRecorder returns a transport clone so one shared planner can be
// safely rebound to the worker-specific Gateway/lease for each Run.
func (p *HTTPPlanner) WithUsageRecorder(recorder func(context.Context, string, PlannerUsage) error) *HTTPPlanner {
	if p == nil {
		return nil
	}
	clone := *p
	clone.UsageRecorder = recorder
	return &clone
}

func (p *HTTPPlanner) Plan(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
	calls, _, err := p.PlanWithUsage(ctx, request)
	return calls, err
}

func (p *HTTPPlanner) PlanWithUsage(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, *PlannerUsage, error) {
	if p == nil || p.Client == nil || p.BaseURL == "" {
		return nil, nil, fmt.Errorf("planner base URL and client are required")
	}
	payload, err := json.Marshal(plannerRequest{
		RunID: request.RunID, SessionID: request.SessionID, Message: request.Message,
		SkillID: request.SkillID, SkillVersion: request.SkillVersion, Tools: request.Tools,
		CompletedResults: request.CompletedResults, SteerInstruction: request.SteerInstruction,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("encode planner request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/v1/agent/plan", bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("create planner request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(p.Token) != "" {
		httpRequest.Header.Set("Authorization", bearerToken(p.Token))
	}
	response, err := p.Client.Do(httpRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("AI planner request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("read AI planner response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("AI planner returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var result plannerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("decode AI planner response: %w", err)
	}
	if len(result.ToolCalls) == 0 {
		return nil, nil, fmt.Errorf("AI planner returned an empty Tool plan")
	}
	if result.Usage != nil && p.UsageRecorder != nil {
		if err := p.UsageRecorder(ctx, request.RunID, *result.Usage); err != nil {
			return nil, nil, fmt.Errorf("record AI planner usage: %w", err)
		}
	}
	return result.ToolCalls, result.Usage, nil
}

var _ Planner = (*HTTPPlanner)(nil)
