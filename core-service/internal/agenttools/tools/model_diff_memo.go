package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/memo"
	"github.com/lease-management-system/core-service/internal/repository"
)

// ModelDiffMemoArguments is the strict input of the S4-4 tool: two run-line
// sets (usually two model versions, or Actual vs Budget) plus the AI's
// narrative items. The amounts the narrative can talk about are computed by
// the difference bridge inside the handler — AI-002: amounts come from the
// deterministic service, not from the model.
type ModelDiffMemoArguments struct {
	Title                   string               `json:"title"`
	SystemFacts             map[string]any       `json:"system_facts"`
	SourceReferences        []any                `json:"source_references"`
	LeftLines               map[string]*float64  `json:"left_lines"`
	RightLines              map[string]*float64  `json:"right_lines"`
	Narratives              []memo.NarrativeItem `json:"narratives"`
	Basis                   string               `json:"basis"`
	DataVersion             string               `json:"data_version"`
	AssumptionVersion       string               `json:"assumption_version"`
	MetricDefinitionVersion string               `json:"metric_definition_version"`
}

// NewModelDiffMemoDefinition registers fpna.memos.model_diff.draft: the
// four-layer decision memo for a model-version or Actual-vs-Budget
// difference. The handler writes the memo draft; the bridge and the
// fail-closed Compose rules guarantee the residual stays explicit.
func NewModelDiffMemoDefinition(writer DecisionMemoDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.memos.model_diff.draft",
			Version:     "v1",
			DisplayName: "模型差异解释草稿（四层备忘录）",
			Description: "对模型版本间或 Actual vs Budget 差异，用确定性差异桥计算逐键差异与显式残差，把 AI 叙述压在四层备忘录（系统事实/确定性计算/人工输入/AI 叙事）中落为 memo 草稿；叙述只能认领桥算出的金额，解释不了的部分以残差显式保留，不补全原因。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "fpna_memos", Action: "write"}},
			InputSchema: json.RawMessage(`{
				"type":"object","additionalProperties":false,
				"required":["title","left_lines","right_lines"],
				"properties":{
					"title":{"type":"string"},
					"system_facts":{"type":"object"},
					"source_references":{"type":"array"},
					"left_lines":{"type":"object"},
					"right_lines":{"type":"object"},
					"narratives":{"type":"array","items":{"type":"object","required":["key","explanation","amount_covered"],
						"properties":{"key":{"type":"string"},"explanation":{"type":"string"},"amount_covered":{"type":"number"}}}},
					"basis":{"type":"string"},
					"data_version":{"type":"string"},
					"assumption_version":{"type":"string"},
					"metric_definition_version":{"type":"string"}
				}}`),
			OutputSchema: json.RawMessage(`{"type":"object","required":["draft","memo"]}`),
			Review: agenttools.ReviewPolicy{
				Required: true, Reasons: []string{"assist_mode", "decision_memo_review"},
				AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_decision_memo",
			},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             1,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"fpna_copilot", "retail_performance"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args ModelDiffMemoArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid model diff arguments: " + err.Error())
			}
			if strings.TrimSpace(args.Title) == "" {
				return agenttools.ToolResult{}, errors.New("title is required")
			}

			bridge := memo.Bridge(args.LeftLines, args.RightLines)
			marshal := func(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
			facts := json.RawMessage(`{}`)
			if args.SystemFacts != nil {
				facts = marshal(args.SystemFacts)
			}
			refs := json.RawMessage(`[]`)
			if args.SourceReferences != nil {
				refs = marshal(args.SourceReferences)
			}
			composed, err := memo.Compose(facts, refs, bridge, args.Narratives)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			if call.DryRun {
				return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{
					"memo": composed, "side_effects": false, "dry_run": true,
				}}, nil
			}
			if writer == nil {
				return agenttools.ToolResult{}, errors.New("decision memo writer unavailable")
			}
			if call.IdempotencyKey == "" {
				return agenttools.ToolResult{}, errors.New("idempotency_key is required for draft writes")
			}
			basis := args.Basis
			if strings.TrimSpace(basis) == "" {
				basis = "Working"
			}
			legal := execution.Principal.Scope.LegalEntityID
			var entity *string
			if legal != "" {
				entity = &legal
			}
			item, err := writer.CreateMemo(ctx, &repository.FPnADecisionMemo{
				LegalEntityID: entity, MemoType: "model_diff", Title: args.Title,
				Basis: basis, Status: "draft",
				SystemFacts:               composed.SystemFacts,
				DeterministicCalculations: composed.DeterministicCalculations,
				HumanInputs:               composed.HumanInputs,
				AINarrative:               composed.AINarrative,
				SourceReferences:          composed.SourceReferences,
				DataVersion:               args.DataVersion,
				AssumptionVersion:         args.AssumptionVersion,
				MetricDefinitionVersion:   args.MetricDefinitionVersion,
				IdempotencyKey:            call.IdempotencyKey,
				CreatedBy:                 &execution.Principal.UserID,
			})
			if err != nil {
				return agenttools.ToolResult{}, errors.New("failed to create model diff memo draft: " + err.Error())
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{
				"draft": item, "memo": composed, "review_required": true,
				"formal_state": "decision_memo_draft", "side_effects": false,
			}, Review: agenttools.ReviewResult{Required: true, Reasons: []string{"assist_mode", "decision_memo_review"}},
				Sources: []agenttools.ToolSource{{Type: "fpna_decision_memo", ID: item.ID, Title: item.Title, Locator: "assist_draft"}}}, nil
		},
	}
}
