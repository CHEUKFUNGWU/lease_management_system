package agentrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/llm"
)

// PlannerLLM is the in-process planner (W4-3). It replaces the Python
// ai-service agent_plan.py endpoint: the model is asked for a Tool plan
// without being given Tool execution access, and its answer is strictly
// validated against the server-owned descriptor set before any call is
// trusted. The whitelist is a security boundary — a model that names an
// unregistered tool fails the plan, it is never silently executed.
type PlannerLLM struct {
	client   *llm.Client
	recorder func(context.Context, string, PlannerUsage) error
}

// NewLLMPlanner builds a planner backed by the shared in-process LLM client.
func NewLLMPlanner(client *llm.Client) *PlannerLLM {
	return &PlannerLLM{client: client}
}

// WithUsageRecorder returns a clone for a worker-specific Gateway/lease, so
// the shared planner can be rebound per run without mutating the original.
func (p *PlannerLLM) WithUsageRecorder(recorder func(context.Context, string, PlannerUsage) error) *PlannerLLM {
	if p == nil {
		return nil
	}
	clone := *p
	clone.recorder = recorder
	return &clone
}

var _ Planner = (*PlannerLLM)(nil)
var _ PlannerWithUsage = (*PlannerLLM)(nil)

// Plan implements minimal Planner.
func (p *PlannerLLM) Plan(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
	calls, _, err := p.PlanWithUsage(ctx, request)
	return calls, err
}

// PlanWithUsage implements PlannerWithUsage and returns the model's measured
// usage for the Run event/checkpoint accounting (operational metadata only,
// never an accounting amount).
func (p *PlannerLLM) PlanWithUsage(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, *PlannerUsage, error) {
	if p == nil || p.client == nil {
		return nil, nil, fmt.Errorf("planner client is required")
	}
	if len(request.Tools) == 0 {
		return nil, nil, fmt.Errorf("at least one discovered tool is required")
	}
	if strings.TrimSpace(request.Message) == "" {
		return nil, nil, fmt.Errorf("planner message is required")
	}

	allowedIdentifiers := allowedToolIdentifiers(request.Tools)
	result, err := p.client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: plannerSystemPrompt(allowedIdentifiers)},
			{Role: "user", Content: plannerUserPrompt(request, allowedIdentifiers)},
		},
		Temp:           0.0,
		MaxTokens:      1200,
		ResponseFormat: map[string]any{"type": "json_object"},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("planner unavailable: %w", err)
	}
	parsed, err := parseJSONObject(result.Answer)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid structured agent plan: %w", err)
	}
	calls, err := normalizePlan(parsed, request.Tools)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid structured agent plan: %w", err)
	}
	var usage *PlannerUsage
	if result.Usage != nil {
		usage = plannerUsageFromMetadata(*result.Usage)
	}
	if usage != nil && p.recorder != nil {
		if err := p.recorder(ctx, request.RunID, *usage); err != nil {
			return nil, nil, fmt.Errorf("record planner usage: %w", err)
		}
	}
	return calls, usage, nil
}

// plannerSystemPrompt ports agent_plan.py's system prompt verbatim.
func plannerSystemPrompt(allowedIdentifiers []string) string {
	return "You are a constrained lease-management Agent planner. " +
		"Return JSON only in the shape {\"tool_calls\":[{\"tool_name\":\"exact.name\",\"tool_version\":\"v1\",\"arguments\":{}}]}. " +
		"You may select only a Tool in the supplied descriptor list. " +
		"Copy the exact Tool name into tool_name; never put the @version suffix in tool_name, " +
		"and never use @v1 or another placeholder as a Tool name. " +
		"Never invent a Tool, SQL, URL, shell command, identity, permission, tenant or scope. " +
		"Arguments must be a JSON object. For draft/write Tools, include a stable idempotency_key. " +
		"Do not perform execution; the Core Runner will validate and execute each call. " +
		"The exact allowed Tool identifiers for this request are: " + strings.Join(allowedIdentifiers, ", ") + "."
}

// plannerUserPrompt ports agent_plan.py's user payload JSON serialization.
func plannerUserPrompt(request PlanRequest, allowedIdentifiers []string) string {
	payload := map[string]any{
		"run_id":                   request.RunID,
		"session_id":               request.SessionID,
		"skill_id":                 request.SkillID,
		"skill_version":            request.SkillVersion,
		"instruction":              request.Message,
		"steer_instruction":        request.SteerInstruction,
		"allowed_tool_identifiers": allowedIdentifiers,
		"discovered_tools":         descriptorView(request.Tools),
		"completed_results":        request.CompletedResults,
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(out)
}

// descriptorView mirrors agent_plan.py's per-tool view sent to the model.
func descriptorView(tools []agenttools.ToolDescriptor) []map[string]any {
	view := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		view = append(view, map[string]any{
			"name":                 t.Name,
			"version":              t.Version,
			"description":          t.Description,
			"level":                string(t.Level),
			"input_schema":         json.RawMessage(t.InputSchema),
			"supports_idempotency": t.SupportsIdempotency,
		})
	}
	return view
}

func allowedToolIdentifiers(tools []agenttools.ToolDescriptor) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		if strings.TrimSpace(t.Name) != "" && strings.TrimSpace(t.Version) != "" {
			out = append(out, t.Name+"@"+t.Version)
		}
	}
	return out
}

// parseJSONObject accepts strict JSON plus the fenced JSON models commonly
// produce, exactly like agent_plan.py's _json_from_model_output.
func parseJSONObject(content string) (any, error) {
	text := strings.TrimSpace(content)
	text = fenceJSON.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	var value any
	if err := json.Unmarshal([]byte(text), &value); err == nil {
		return value, nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, errors.New("model did not return a JSON object")
	}
	var obj any
	if err := json.Unmarshal([]byte(text[start:end+1]), &obj); err != nil {
		return nil, fmt.Errorf("model JSON decode failed: %w", err)
	}
	return obj, nil
}

// fenceJSON strips ```json fences from model output.
var fenceJSON = regexp.MustCompile("(?is)^\\s*```(?:json)?\\s*|\\s*```\\s*$")

var errEmptyPlan = errors.New("model returned an empty Tool plan")

// normalizePlan is the security gate: every planned call must select a tool
// in the discovered descriptor set, with arguments as a JSON object. A model
// naming an unregistered tool rejects the whole plan (fail-closed).
func normalizePlan(raw any, tools []agenttools.ToolDescriptor) ([]agenttools.ToolCall, error) {
	var rawCalls []any
	switch v := raw.(type) {
	case []any:
		rawCalls = v
	case map[string]any:
		rawCalls, _ = v["tool_calls"].([]any)
	default:
		return nil, errEmptyPlan
	}
	if len(rawCalls) == 0 {
		return nil, errEmptyPlan
	}
	allowed := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		allowed[name+"@"+strings.TrimSpace(t.Version)] = struct{}{}
	}
	calls := make([]agenttools.ToolCall, 0, len(rawCalls))
	for _, rawCall := range rawCalls {
		obj, ok := rawCall.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each planned Tool call must be an object")
		}
		name := strings.TrimSpace(stringify(obj["tool_name"]))
		version := strings.TrimSpace(stringify(obj["tool_version"]))
		if version == "" {
			version = "v1"
		}
		if _, ok := allowed[name+"@"+version]; !ok {
			return nil, fmt.Errorf("model selected a Tool outside the discovered descriptor set: %s@%s", name, version)
		}
		args, ok := obj["arguments"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("arguments for %s must be an object", name)
		}
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("arguments for %s do not marshal: %w", name, err)
		}
		call := agenttools.ToolCall{
			CallID:         stringifyOrNil(obj["call_id"]),
			ToolName:       name,
			ToolVersion:    version,
			Arguments:      argsJSON,
			IdempotencyKey: stringifyOrNil(obj["idempotency_key"]),
			DryRun:         boolify(obj["dry_run"]),
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func stringifyOrNil(v any) string {
	s := strings.TrimSpace(stringify(v))
	if s == "" {
		return ""
	}
	return s
}

func boolify(v any) bool {
	b, _ := v.(bool)
	return b
}

// plannerUsageFromMetadata adapts the llm client's usage metadata into the
// runner's PlannerUsage record (a Run event payload, not accounting data).
func plannerUsageFromMetadata(u llm.UsageMetadata) *PlannerUsage {
	toI64 := func(p *int) *int64 {
		if p == nil {
			return nil
		}
		v := int64(*p)
		return &v
	}
	return &PlannerUsage{
		SchemaVersion:  u.SchemaVersion,
		Provider:       u.Provider,
		Model:          u.Model,
		InputTokens:    toI64(u.InputTokens),
		OutputTokens:   toI64(u.OutputTokens),
		TotalTokens:    toI64(u.TotalTokens),
		CostMicros:     u.CostMicros,
		CostCurrency:   u.CostCurrency,
		CostStatus:     u.CostStatus,
		PricingVersion: u.PricingVersion,
		PricingSource:  u.PricingSource,
	}
}
