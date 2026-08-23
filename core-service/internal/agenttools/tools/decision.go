package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/dealcompare"
	"github.com/lease-management-system/core-service/internal/services/predeal"
	"github.com/lease-management-system/core-service/internal/services/renewaldecision"
)

func NewDealSimulationDefinition() agenttools.ToolDefinition {
	return simulationDefinition("lease.deal.simulate", "模拟签约报价", "比较候选租赁报价的有效租金、现值和现金成本", `{"type":"object","additionalProperties":false,"required":["discount_rate","offers"],"properties":{"discount_rate":{"type":"number","exclusiveMinimum":0},"currency":{"type":"string"},"offers":{"type":"array","minItems":2,"maxItems":5}}}`, dealSimulationHandler)
}

func NewPreDealSimulationDefinition() agenttools.ToolDefinition {
	return simulationDefinition("lease.predeal.simulate", "模拟签约前经济性", "计算签约前租赁的 IFRS 16、EBITDA、现金和退出曲线", `{"type":"object","additionalProperties":false,"required":["draft"],"properties":{"draft":{"type":"object"}}}`, preDealSimulationHandler)
}

func NewRenewalSimulationDefinition() agenttools.ToolDefinition {
	return simulationDefinition("lease.renewal.simulate", "模拟续租方案", "比较续租、议价和退出的现金、损益及 IFRS 16 影响", `{"type":"object","additionalProperties":false,"required":["decision_date","currency","discount_rate","scenarios"],"properties":{"decision_date":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},"currency":{"type":"string"},"discount_rate":{"type":"number","exclusiveMinimum":0},"scenarios":{"type":"array","minItems":2}}}`, renewalSimulationHandler)
}

type DecisionSummaryArguments struct {
	Title            string         `json:"title"`
	Facts            map[string]any `json:"facts"`
	Calculations     map[string]any `json:"calculations"`
	Assumptions      map[string]any `json:"assumptions"`
	DataGaps         []string       `json:"data_gaps"`
	Counterarguments []string       `json:"counterarguments"`
	Questions        []string       `json:"questions"`
}

func NewDecisionSummaryDefinition() agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "fpna.decision.summary", Version: "v1", DisplayName: "生成一页决策摘要", Description: "把已提供的系统事实、确定性计算、假设、数据缺口、反方论点和待问问题组织为一页摘要；不写入正式记录", Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","facts","calculations"],"properties":{"title":{"type":"string"},"facts":{"type":"object"},"calculations":{"type":"object"},"assumptions":{"type":"object"},"data_gaps":{"type":"array","items":{"type":"string"}},"counterarguments":{"type":"array","items":{"type":"string"}},"questions":{"type":"array","items":{"type":"string"}}}}`), SupportsDryRun: true, MaxRows: 1, TimeoutSeconds: 15}, SkillIDs: []string{"fpna_copilot", "retail_performance", "manufacturing_performance"}, Handler: decisionSummaryHandler}
}

func decisionSummaryHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	if _, err := agenttools.RequireExecutionContext(ctx); err != nil {
		return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
	}
	args, err := decodeStrict[DecisionSummaryArguments](call.Arguments)
	if err != nil || strings.TrimSpace(args.Title) == "" || args.Facts == nil || args.Calculations == nil {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "title, facts and calculations are required"), nil
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"title": args.Title, "facts": args.Facts, "calculations": args.Calculations, "assumptions": args.Assumptions, "data_gaps": args.DataGaps, "counterarguments": args.Counterarguments, "questions": args.Questions, "review_required": true, "side_effects": false}, Sources: []agenttools.ToolSource{{Type: "decision_summary_input", ID: call.CallID, Title: args.Title, Locator: "agent_input"}}}, nil
}

func simulationDefinition(name, display, description, schema string, handler agenttools.ToolHandler) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: name, Version: "v1", DisplayName: display, Description: description,
			Level: agenttools.LevelRead, ReadOnly: true,
			Permissions:    []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema:    json.RawMessage(schema),
			OutputSchema:   json.RawMessage(`{"type":"object","required":["basis","side_effects"]}`),
			SupportsDryRun: true, MaxRows: 20, TimeoutSeconds: 30,
		},
		SkillIDs: []string{"fpna_copilot", "retail_performance", "manufacturing_performance"},
		Handler:  handler,
	}
}

func dealSimulationHandler(_ context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	var input dealcompare.Input
	if err := decodeDecisionStrict(call.Arguments, &input); err != nil {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid deal simulation arguments"), nil
	}
	result, err := dealcompare.Compare(input)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, err.Error()), nil
	}
	return scenarioResult(call, "deal_terms", result), nil
}

type preDealArguments struct {
	Draft predeal.Draft `json:"draft"`
}

func preDealSimulationHandler(_ context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	var input preDealArguments
	if err := decodeDecisionStrict(call.Arguments, &input); err != nil {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid pre-deal simulation arguments"), nil
	}
	result, err := predeal.Build(input.Draft)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, err.Error()), nil
	}
	return scenarioResult(call, "pre_deal", result), nil
}

type renewalSimulationArguments struct {
	DecisionDate        string                     `json:"decision_date"`
	Currency            string                     `json:"currency"`
	DiscountRate        float64                    `json:"discount_rate"`
	CurrentMonthlyRent  float64                    `json:"current_monthly_rent"`
	RemainingCommitment float64                    `json:"remaining_commitment"`
	CurrentLiability    float64                    `json:"current_liability"`
	CurrentROU          float64                    `json:"current_rou"`
	RemainingTermMonths int                        `json:"remaining_term_months"`
	Scenarios           []renewaldecision.Scenario `json:"scenarios"`
}

func renewalSimulationHandler(_ context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	var input renewalSimulationArguments
	if err := decodeDecisionStrict(call.Arguments, &input); err != nil {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid renewal simulation arguments"), nil
	}
	date, err := time.Parse("2006-01-02", input.DecisionDate)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "decision_date must be YYYY-MM-DD"), nil
	}
	result, err := renewaldecision.Evaluate(renewaldecision.Input{
		DecisionDate: date, Currency: input.Currency, DiscountRate: input.DiscountRate,
		CurrentMonthlyRent: input.CurrentMonthlyRent, RemainingCommitment: input.RemainingCommitment,
		CurrentLiability: input.CurrentLiability, CurrentROU: input.CurrentROU,
		RemainingTermMonths: input.RemainingTermMonths, Scenarios: input.Scenarios,
	})
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, err.Error()), nil
	}
	return scenarioResult(call, "renewal", result), nil
}

func scenarioResult(call agenttools.ToolCall, kind string, result any) agenttools.ToolResult {
	return agenttools.ToolResult{
		CallID: call.CallID, Status: agenttools.StatusCompleted,
		Data:    map[string]any{"basis": "Scenario", "scenario_type": kind, "result": result, "side_effects": false, "review_required": true},
		Sources: []agenttools.ToolSource{{Type: "scenario_assumptions", ID: call.CallID, Title: "用户提供的情景假设", Locator: "agent_input"}},
	}
}

type ActionExplanationDraftWriter interface {
	CreateAction(context.Context, *repository.FPnAActionItem) (*repository.FPnAActionItem, error)
}

func NewExplanationDraftDefinition(writer ActionExplanationDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "fpna.explanations.draft.create", Version: "v1", DisplayName: "创建差异解释草稿",
			Description: "保存确定性差异桥、人工解释和 AI 建议的分层草稿，不形成正式控制结论",
			Level:       agenttools.LevelDraft, ReadOnly: false,
			Permissions:    []agenttools.Permission{{Resource: "fpna_actions", Action: "write"}},
			InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period","title","rule_code","source_table","source_record_id","deterministic_attribution"],"properties":{"period":{"type":"string"},"title":{"type":"string"},"rule_code":{"type":"string"},"source_table":{"type":"string"},"source_record_id":{"type":"string"},"data_version":{"type":"string"},"deterministic_attribution":{"type":"object"},"human_explanation":{"type":"string"},"ai_suggestion":{"type":"string"},"impact_amount":{"type":"number"},"currency":{"type":"string"}}}`),
			SupportsDryRun: true, SupportsIdempotency: true,
			Review: agenttools.ReviewPolicy{Required: true, Reasons: []string{"human_explanation_confirmation"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_explanation_draft"},
			Retry:  agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler:  explanationDraftHandler(writer),
	}
}

type explanationDraftArguments struct {
	Period                   string         `json:"period"`
	Title                    string         `json:"title"`
	RuleCode                 string         `json:"rule_code"`
	SourceTable              string         `json:"source_table"`
	SourceRecordID           string         `json:"source_record_id"`
	DataVersion              string         `json:"data_version"`
	DeterministicAttribution map[string]any `json:"deterministic_attribution"`
	HumanExplanation         string         `json:"human_explanation"`
	AISuggestion             string         `json:"ai_suggestion"`
	ImpactAmount             *float64       `json:"impact_amount"`
	Currency                 string         `json:"currency"`
}

func explanationDraftHandler(writer ActionExplanationDraftWriter) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeExplanationArguments(call.Arguments)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, err.Error()), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		evidence, _ := json.Marshal(map[string]any{"deterministic_attribution": args.DeterministicAttribution, "source": "agent_input"})
		item, err := writer.CreateAction(ctx, &repository.FPnAActionItem{
			LegalEntityID: optionalEntity(execution.Principal.Scope.LegalEntityID), Period: args.Period,
			Category: "variance_explanation", Status: "open", Severity: "medium", Title: args.Title,
			RuleCode: args.RuleCode, SourceTable: args.SourceTable, SourceRecordID: args.SourceRecordID,
			DataVersion: args.DataVersion, ImpactAmount: args.ImpactAmount, Currency: args.Currency,
			HumanRootCause: args.HumanExplanation, AISuggestion: args.AISuggestion, Evidence: evidence,
			CreatedBy: optionalString(execution.Principal.UserID), UpdatedBy: optionalString(execution.Principal.UserID),
		})
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to create explanation draft"), err
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"draft": item, "formal_state": "agent_signal_draft", "review_required": true}, Sources: []agenttools.ToolSource{{Type: "fpna_action_item", ID: item.ID, Title: item.Title, Locator: "assist_draft"}}}, nil
	}
}

func decodeExplanationArguments(raw json.RawMessage) (explanationDraftArguments, error) {
	var args explanationDraftArguments
	if err := decodeDecisionStrict(raw, &args); err != nil {
		return args, fmt.Errorf("invalid explanation draft arguments")
	}
	if strings.TrimSpace(args.Period) == "" || strings.TrimSpace(args.Title) == "" || strings.TrimSpace(args.RuleCode) == "" || strings.TrimSpace(args.SourceTable) == "" || strings.TrimSpace(args.SourceRecordID) == "" {
		return args, fmt.Errorf("period, title, rule_code, source_table and source_record_id are required")
	}
	return args, nil
}

func decodeDecisionStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
func optionalEntity(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
