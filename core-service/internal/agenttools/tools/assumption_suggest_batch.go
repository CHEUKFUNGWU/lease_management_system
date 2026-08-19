package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
)

// BatchSuggestionArguments is the strict input of the S4-3 batch tool. The
// evidence is explicit — historical series, seasonality and registered
// approved assumptions the caller assembled through its read tools — so the
// engine stays a pure function of declared facts (determinism, D-S2).
type BatchSuggestionArguments struct {
	TargetPeriod string                     `json:"target_period"`
	Historical   map[string][]*float64      `json:"historical"`
	Seasonality  map[string][]*float64      `json:"seasonality"`
	Registered   map[string]json.RawMessage `json:"registered"`
}

// NewAssumptionSuggestionBatchDefinition registers
// fpna.assumptions.suggest_batch: one whole-board draft set organized into
// revenue/expense/working-capital/capex/tax blocks for block-wise
// confirmation, plus the explicit list of keys the engine will not guess
// (无法建议项，不编造). Writes happen through the same draft-only store as
// fpna.assumptions.suggest; dry runs compute without writing.
func NewAssumptionSuggestionBatchDefinition(writer AssumptionDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.assumptions.suggest_batch",
			Version:     "v1",
			DisplayName: "批量假设初稿（按区块）",
			Description: "对空白模型按历史 run-rate + 季节性 + 已登记假设一次性产出整版假设草稿（draft，source=ai_suggestion），按收入/费用/营运资本/CAPEX/税务分块供逐块确认；缺失输入显式列入无法建议项并说明原因，不编造。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "fin_models", Action: "write"}},
			InputSchema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"required":["target_period","historical"],
				"properties":{
					"target_period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"},
					"historical":{"type":"object"},
					"seasonality":{"type":"object"},
					"registered":{"type":"object"}
				}}`),
			OutputSchema: json.RawMessage(`{"type":"object","required":["blocks","unsuggestable"]}`),
			Review: agenttools.ReviewPolicy{
				Required: true, Reasons: []string{"ai_suggestion_draft", "assumptions_unconfirmed"},
				ConfirmAction: "confirm",
			},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             200,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{retailIngestFillSkill},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if writer == nil {
				return agenttools.ToolResult{}, errors.New("assumption batch writer unavailable")
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args BatchSuggestionArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid batch arguments: " + err.Error())
			}
			if strings.TrimSpace(args.TargetPeriod) == "" {
				return agenttools.ToolResult{}, errors.New("target_period is required (YYYY-MM)")
			}
			if strings.TrimSpace(call.CallID) == "" {
				return agenttools.ToolResult{}, errors.New("call_id is required: every draft basis must anchor to its tool call")
			}

			out := suggestion.PlanBatch(suggestion.BatchInput{
				LegalEntityID: execution.Principal.Scope.LegalEntityID,
				ToolCallID:    call.CallID,
				TargetPeriod:  args.TargetPeriod,
				Historical:    args.Historical,
				Seasonality:   args.Seasonality,
				Registered:    args.Registered,
			})

			data := map[string]any{
				"blocks":        out.Blocks,
				"unsuggestable": out.UnSuggestable,
				"side_effects":  false,
				"status":        "draft",
			}
			if call.DryRun {
				data["dry_run"] = true
				return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
					Review: agenttools.ReviewResult{Required: true, Reasons: []string{"ai_suggestion_draft"}}}, nil
			}
			if call.IdempotencyKey == "" {
				return agenttools.ToolResult{}, errors.New("idempotency_key is required for draft writes")
			}
			ids, err := suggestion.SaveBatch(ctx, memStoreAdapter{writer: writer}, suggestion.BatchInput{
				LegalEntityID: execution.Principal.Scope.LegalEntityID,
			}, out, call.IdempotencyKey)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			data["draft_ids"] = ids
			data["draft_count"] = len(ids)
			data["side_effects"] = len(ids) > 0
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"ai_suggestion_draft"}}}, nil
		},
	}
}

// memStoreAdapter bridges the AssumptionDraftWriter closure to the
// suggestion.Store seam so the write path is the single SaveDrafts shape
// already covered by the draft-only tests.
type memStoreAdapter struct {
	writer AssumptionDraftWriter
}

func (a memStoreAdapter) SaveDrafts(ctx context.Context, legalEntityID string, drafts []suggestion.SuggestionDraft, idempotencyKey string) ([]string, error) {
	return a.writer.SaveDrafts(ctx, legalEntityID, drafts, idempotencyKey)
}
