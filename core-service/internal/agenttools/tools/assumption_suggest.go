package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
)

// AssumptionSuggestionArguments is the strict input of the suggest tool.
type AssumptionSuggestionArguments struct {
	Suggestions []suggestion.SuggestionDraft `json:"suggestions"`
}

// AssumptionDraftWriter is the persistence seam; the production adapter
// writes draft rows only (status=draft, source=ai_suggestion). The tool
// never exposes an approved path — 人工确认接口在外，不在工具里.
type AssumptionDraftWriter interface {
	SaveDrafts(ctx context.Context, legalEntityID string, drafts []suggestion.SuggestionDraft, idempotencyKey string) ([]string, error)
}

// NewAssumptionSuggestionDefinition registers fpna.assumptions.suggest
// (LevelDraft + Review Gate + idempotency). The structural rule travels
// with the payload: every draft must carry non-empty basis evidence and a
// derived confidence; the writer re-validates, so an evidence-less request
// dies at the gate.
func NewAssumptionSuggestionDefinition(writer AssumptionDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.assumptions.suggest",
			Version:     "v1",
			DisplayName: "假设建议（草稿）",
			Description: "产出假设草稿（source=ai_suggestion，draft 状态），每条附依据引用与推导置信度；人确认后才转 approved，AI 无 approved 路径。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "fin_models", Action: "write"}},
			InputSchema: json.RawMessage(`{
				"type":"object","required":["suggestions"],
				"properties":{"suggestions":{"type":"array","minItems":1,"items":{"type":"object","required":["assumption_key","category","value","basis","confidence"],"properties":{
					"assumption_key":{"type":"string"},"category":{"type":"string"},
					"value":{},"unit":{"type":"string"},
					"basis":{"type":"array","minItems":1,"items":{"type":"object","required":["tool_call_id","scope"],"properties":{"tool_call_id":{"type":"string"},"scope":{"type":"string"},"period":{"type":"string"}}}},
					"confidence":{"type":"number"},"source_tag":{"type":"string"}
				}}}}}
			}`),
			OutputSchema:        json.RawMessage(`{"type":"object","required":["draft_ids","side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"ai_suggestion_draft", "assumptions_unconfirmed"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             100,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{retailIngestFillSkill}, // placeholder; the fin skill family registers at the workbench stage
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if writer == nil {
				return agenttools.ToolResult{}, errors.New("assumption draft writer unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args AssumptionSuggestionArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil || len(args.Suggestions) == 0 {
				return agenttools.ToolResult{}, errors.New("at least one suggestion is required")
			}
			// 结构性前置：无依据的建议在进写路径前就被拒；source 恒为
			// ai_suggestion（写路径不存在 approved）。
			if call.IdempotencyKey == "" {
				return agenttools.ToolResult{}, errors.New("idempotency_key is required for draft writes")
			}
			drafts := make([]suggestion.SuggestionDraft, 0, len(args.Suggestions))
			for _, draft := range args.Suggestions {
				if draft.SourceTag == "" {
					draft.SourceTag = "ai_suggestion"
				}
				if err := draft.Validate(); err != nil {
					return agenttools.ToolResult{}, err
				}
				drafts = append(drafts, draft)
			}
			ids, err := writer.SaveDrafts(ctx, execution.Principal.Scope.LegalEntityID, drafts, call.IdempotencyKey)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			return agenttools.ToolResult{
				CallID: call.CallID, Status: agenttools.StatusCompleted,
				Data:   map[string]any{"draft_ids": ids, "draft_count": len(ids), "side_effects": true, "status": "draft"},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"ai_suggestion_draft"}},
			}, nil
		},
	}
}
