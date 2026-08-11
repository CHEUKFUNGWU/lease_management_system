package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/operating"
)

type PerformanceReader interface {
	Overview(context.Context, string, string) (*repository.PerformanceOverview, error)
	ListStores(context.Context, string, string, string) ([]*repository.StoreOperatingFact, error)
	ListEquipmentFacts(context.Context, string, string, string, string) ([]*repository.EquipmentOperatingFact, error)
	ListActions(context.Context, string, string, string, string) ([]*repository.FPnAActionItem, error)
}

type ActionDraftWriter interface {
	CreateAction(context.Context, *repository.FPnAActionItem) (*repository.FPnAActionItem, error)
}

type ScenarioDraftWriter interface {
	CreateScenarioDraft(context.Context, *repository.FPnAScenarioDraft) (*repository.FPnAScenarioDraft, error)
}

type DecisionMemoDraftWriter interface {
	CreateMemo(context.Context, *repository.FPnADecisionMemo) (*repository.FPnADecisionMemo, error)
}

type CloseReadinessReader interface {
	Evaluate(context.Context, closereadiness.Command) (*closereadiness.Result, error)
}

type PerformanceArguments struct {
	Period   string `json:"period"`
	StoreID  string `json:"store_id,omitempty"`
	Plant    string `json:"plant,omitempty"`
	Line     string `json:"line,omitempty"`
	Status   string `json:"status,omitempty"`
	Category string `json:"category,omitempty"`
}

func NewPortfolioSummaryDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.portfolio.summary", "读取经营组合摘要", "读取权限范围内租赁经营事实、设备事实、数据质量和待办影响摘要", json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"period":{"type":"string"}}}`), reader, portfolioSummaryHandler)
}

func NewManagementPreReadDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.management.pre_read", "生成管理层会前材料", "基于权限范围内经营事实和行动生成会前摘要、问题清单和数据缺口", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string"}}}`), reader, managementPreReadHandler)
}

func NewStorePerformanceDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.store.performance", "读取门店四墙表现", "读取权限范围内门店四墙损益、租售比、坪效及数据缺口", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string"},"store_id":{"type":"string"}}}`), reader, storePerformanceHandler)
}

func NewRentToSalesDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.rent_to_sales", "读取租售比", "读取权限范围内门店固定租金、变量租金与销售额的租售比及缺口", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string"},"store_id":{"type":"string"}}}`), reader, rentToSalesHandler)
}

func NewEquipmentPerformanceDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.equipment.performance", "读取设备经营表现", "读取权限范围内产线设备经营事实和制造成本桥", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string"},"plant":{"type":"string"},"line":{"type":"string"}}}`), reader, equipmentPerformanceHandler)
}

func NewActionListDefinition(reader PerformanceReader) agenttools.ToolDefinition {
	return readPerformanceDefinition("lease.fpna.actions", "读取经营行动与异常", "读取权限范围内待处理经营异常、数据问题和行动兑现状态", json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"period":{"type":"string"},"status":{"type":"string"},"category":{"type":"string"}}}`), reader, actionListHandler)
}

type StoreScenarioArguments struct {
	Scenarios []operating.StoreDecisionScenario `json:"scenarios"`
}
type EquipmentScenarioArguments struct {
	Scenarios []operating.EquipmentDecisionScenario `json:"scenarios"`
}

func NewStoreScenarioDefinition() agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.store.scenario.simulate", Version: "v1", DisplayName: "模拟门店经营方案", Description: "对续租、议价、缩店、搬迁和关店方案进行无副作用确定性测算", Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["scenarios"],"properties":{"scenarios":{"type":"array","minItems":2}}}`), SupportsDryRun: true, MaxRows: 20, TimeoutSeconds: 15}, SkillIDs: []string{"fpna_copilot", "retail_performance"}, Handler: storeScenarioHandler}
}

func NewEquipmentScenarioDefinition() agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.equipment.scenario.simulate", Version: "v1", DisplayName: "模拟设备经济方案", Description: "对 Buy、Lease、Renew、Replace 和 Outsource 方案进行无副作用确定性测算", Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["scenarios"],"properties":{"scenarios":{"type":"array","minItems":2}}}`), SupportsDryRun: true, MaxRows: 20, TimeoutSeconds: 15}, SkillIDs: []string{"fpna_copilot", "manufacturing_performance"}, Handler: equipmentScenarioHandler}
}

type ActionDraftArguments struct {
	Period          string   `json:"period"`
	Category        string   `json:"category"`
	Severity        string   `json:"severity"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	RuleCode        string   `json:"rule_code"`
	SourceTable     string   `json:"source_table"`
	SourceRecordID  string   `json:"source_record_id"`
	DataVersion     string   `json:"data_version"`
	ImpactAmount    *float64 `json:"impact_amount"`
	Currency        string   `json:"currency"`
	ExpectedBenefit *float64 `json:"expected_benefit"`
	PlannedAction   string   `json:"planned_action"`
}

func NewActionDraftDefinition(writer ActionDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.fpna.action.draft.create", Version: "v1", DisplayName: "创建经营行动草稿", Description: "创建待人工确认的经营异常解释/行动草稿，不会直接改变 Forecast、合同或会计记录", Level: agenttools.LevelDraft, ReadOnly: false, Permissions: []agenttools.Permission{{Resource: "fpna_actions", Action: "write"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["category","title","rule_code","source_table","source_record_id"],"properties":{"period":{"type":"string"},"category":{"type":"string"},"severity":{"type":"string"},"title":{"type":"string"},"description":{"type":"string"},"rule_code":{"type":"string"},"source_table":{"type":"string"},"source_record_id":{"type":"string"},"data_version":{"type":"string"},"impact_amount":{"type":"number"},"currency":{"type":"string"},"expected_benefit":{"type":"number"},"planned_action":{"type":"string"}}}`), SupportsDryRun: true, SupportsIdempotency: true, MaxRows: 1, TimeoutSeconds: 15, Review: agenttools.ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "human_action_confirmation"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_action_draft"}, Retry: agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2}}, SkillIDs: []string{"fpna_copilot"}, Handler: actionDraftHandler(writer)}
}

func NewMeetingActionDraftDefinition(writer ActionDraftWriter) agenttools.ToolDefinition {
	definition := NewActionDraftDefinition(writer)
	definition.Descriptor.Name = "lease.meeting.action.draft.create"
	definition.Descriptor.DisplayName = "创建会议行动草稿"
	definition.Descriptor.Description = "将会议纪要中的承诺、负责人、截止日期保存为待确认行动草稿"
	definition.Descriptor.Review.ConfirmAction = "confirm_meeting_action"
	return definition
}

type ScenarioDraftArguments struct {
	ScenarioType string         `json:"scenario_type"`
	Name         string         `json:"name"`
	Assumptions  map[string]any `json:"assumptions"`
	Result       map[string]any `json:"result"`
	DataVersion  string         `json:"data_version"`
}

func NewScenarioDraftDefinition(writer ScenarioDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.fpna.scenario.draft.create", Version: "v1", DisplayName: "保存经营情景草稿", Description: "保存经确定性服务计算后的 Scenario 草稿；不会覆盖 Budget/Forecast 或产生正式会计记录", Level: agenttools.LevelDraft, ReadOnly: false, Permissions: []agenttools.Permission{{Resource: "fpna_actions", Action: "write"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["scenario_type","name","assumptions"],"properties":{"scenario_type":{"type":"string"},"name":{"type":"string"},"assumptions":{"type":"object"},"result":{"type":"object"},"data_version":{"type":"string"}}}`), SupportsDryRun: true, SupportsIdempotency: true, MaxRows: 1, TimeoutSeconds: 15, Review: agenttools.ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "scenario_confirmation"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_scenario_draft"}, Retry: agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2}}, SkillIDs: []string{"fpna_copilot"}, Handler: scenarioDraftHandler(writer)}
}

type DecisionMemoArguments struct {
	MemoType                  string         `json:"memo_type"`
	Title                     string         `json:"title"`
	Basis                     string         `json:"basis"`
	ScenarioDraftID           *string        `json:"scenario_draft_id"`
	SystemFacts               map[string]any `json:"system_facts"`
	DeterministicCalculations map[string]any `json:"deterministic_calculations"`
	HumanInputs               map[string]any `json:"human_inputs"`
	AINarrative               map[string]any `json:"ai_narrative"`
	SourceReferences          []any          `json:"source_references"`
	DataVersion               string         `json:"data_version"`
	AssumptionVersion         string         `json:"assumption_version"`
	MetricDefinitionVersion   string         `json:"metric_definition_version"`
}

func NewDecisionMemoDraftDefinition(writer DecisionMemoDraftWriter) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.decision.memo.draft.create", Version: "v1", DisplayName: "创建决策备忘录草稿", Description: "将系统事实、确定性计算、人类输入和 AI 叙事分层保存为需复核的决策备忘录草稿", Level: agenttools.LevelDraft, ReadOnly: false, Permissions: []agenttools.Permission{{Resource: "fpna_memos", Action: "write"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["memo_type","title","system_facts","deterministic_calculations"],"properties":{"memo_type":{"type":"string"},"title":{"type":"string"},"basis":{"type":"string"},"scenario_draft_id":{"type":"string"},"system_facts":{"type":"object"},"deterministic_calculations":{"type":"object"},"human_inputs":{"type":"object"},"ai_narrative":{"type":"object"},"source_references":{"type":"array"},"data_version":{"type":"string"},"assumption_version":{"type":"string"},"metric_definition_version":{"type":"string"}}}`), SupportsDryRun: true, SupportsIdempotency: true, MaxRows: 1, TimeoutSeconds: 15, Review: agenttools.ReviewPolicy{Required: true, Reasons: []string{"assist_mode", "decision_memo_review"}, AllowedRoles: []string{"reviewer", "approver"}, ConfirmAction: "confirm_decision_memo"}, Retry: agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2}}, SkillIDs: []string{"fpna_copilot", "retail_performance", "manufacturing_performance"}, Handler: decisionMemoDraftHandler(writer)}
}

func decisionMemoDraftHandler(writer DecisionMemoDraftWriter) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if writer == nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "decision memo writer unavailable"), nil
		}
		args, err := decodeStrict[DecisionMemoArguments](call.Arguments)
		if err != nil || strings.TrimSpace(args.MemoType) == "" || strings.TrimSpace(args.Title) == "" || args.SystemFacts == nil || args.DeterministicCalculations == nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "memo_type, title, system_facts and deterministic_calculations are required"), nil
		}
		marshal := func(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
		legal := execution.Principal.Scope.LegalEntityID
		var entity *string
		if legal != "" {
			entity = &legal
		}
		item, err := writer.CreateMemo(ctx, &repository.FPnADecisionMemo{LegalEntityID: entity, MemoType: args.MemoType, Title: args.Title, Basis: args.Basis, ScenarioDraftID: args.ScenarioDraftID, SystemFacts: marshal(args.SystemFacts), DeterministicCalculations: marshal(args.DeterministicCalculations), HumanInputs: marshal(args.HumanInputs), AINarrative: marshal(args.AINarrative), SourceReferences: marshal(args.SourceReferences), DataVersion: args.DataVersion, AssumptionVersion: args.AssumptionVersion, MetricDefinitionVersion: args.MetricDefinitionVersion, IdempotencyKey: call.IdempotencyKey, Status: "draft", CreatedBy: &execution.Principal.UserID})
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to create decision memo draft"), nil
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"draft": item, "review_required": true, "formal_state": "decision_memo_draft", "side_effects": false}, Sources: []agenttools.ToolSource{{Type: "fpna_decision_memo", ID: item.ID, Title: item.Title, Locator: "assist_draft"}}}, nil
	}
}

func scenarioDraftHandler(writer ScenarioDraftWriter) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if writer == nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "scenario draft writer unavailable"), nil
		}
		args, err := decodeStrict[ScenarioDraftArguments](call.Arguments)
		if err != nil || strings.TrimSpace(args.ScenarioType) == "" || strings.TrimSpace(args.Name) == "" || args.Assumptions == nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "scenario_type, name and assumptions are required"), nil
		}
		assumptions, _ := json.Marshal(args.Assumptions)
		var result json.RawMessage
		if args.Result != nil {
			result, _ = json.Marshal(args.Result)
		}
		legal := execution.Principal.Scope.LegalEntityID
		var entity *string
		if legal != "" {
			entity = &legal
		}
		item, err := writer.CreateScenarioDraft(ctx, &repository.FPnAScenarioDraft{LegalEntityID: entity, ScenarioType: args.ScenarioType, Name: args.Name, Assumptions: assumptions, Result: result, DataVersion: args.DataVersion, Status: "draft", SourceRunID: execution.RunID, IdempotencyKey: call.IdempotencyKey, CreatedBy: &execution.Principal.UserID})
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to create scenario draft"), nil
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"draft": item, "review_required": true, "formal_state": "scenario_draft", "side_effects": false}, Sources: []agenttools.ToolSource{{Type: "fpna_scenario_draft", ID: item.ID, Title: item.Name, Locator: "assist_draft"}}}, nil
	}
}

func NewCloseReadinessDefinition(reader CloseReadinessReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: "lease.close.readiness", Version: "v1", DisplayName: "读取关账准备度", Description: "读取权限范围内期间关账准备度、阻塞项和证据缺口；不创建正式 Close Exception", Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"type":"string"}}}`), SupportsDryRun: true, MaxRows: 500, TimeoutSeconds: 15}, SkillIDs: []string{"fpna_copilot"}, Handler: closeReadinessHandler(reader)}
}

func closeReadinessHandler(reader CloseReadinessReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		args, err := decodeStrict[PerformanceArguments](call.Arguments)
		if err != nil || strings.TrimSpace(args.Period) == "" {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "period is required"), nil
		}
		if reader == nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "close readiness reader unavailable"), nil
		}
		result, err := reader.Evaluate(ctx, closereadiness.Command{AccountingPeriod: args.Period, LegalEntityID: execution.Principal.Scope.LegalEntityID, ScopeComplete: true})
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to evaluate close readiness"), nil
		}
		sources := make([]agenttools.ToolSource, 0, len(result.Findings))
		for _, finding := range result.Findings {
			sources = append(sources, agenttools.ToolSource{Type: "close_readiness_finding", ID: finding.SourceID, Title: finding.Title, Locator: finding.TargetPath})
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: result, Sources: sources}, nil
	}
}

func actionDraftHandler(writer ActionDraftWriter) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if writer == nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "action draft writer unavailable"), nil
		}
		args, err := decodeStrict[ActionDraftArguments](call.Arguments)
		if err != nil || strings.TrimSpace(args.Category) == "" || strings.TrimSpace(args.Title) == "" || strings.TrimSpace(args.RuleCode) == "" || strings.TrimSpace(args.SourceTable) == "" || strings.TrimSpace(args.SourceRecordID) == "" {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "category, title, rule_code, source_table and source_record_id are required"), nil
		}
		legal := execution.Principal.Scope.LegalEntityID
		var entity *string
		if legal != "" {
			entity = &legal
		}
		evidence, _ := json.Marshal(map[string]any{"origin": "agent", "run_id": execution.RunID, "idempotency_key": call.IdempotencyKey, "mode": "assist"})
		item, err := writer.CreateAction(ctx, &repository.FPnAActionItem{LegalEntityID: entity, Period: args.Period, Category: args.Category, Severity: performanceDefault(args.Severity, "medium"), Status: "open", Title: args.Title, Description: args.Description, RuleCode: args.RuleCode, SourceTable: args.SourceTable, SourceRecordID: args.SourceRecordID, DataVersion: args.DataVersion, IdempotencyKey: call.IdempotencyKey, ImpactAmount: args.ImpactAmount, Currency: args.Currency, ExpectedBenefit: args.ExpectedBenefit, PlannedAction: args.PlannedAction, Evidence: evidence, CreatedBy: &execution.Principal.UserID, UpdatedBy: &execution.Principal.UserID})
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to create action draft"), nil
		}
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"draft": item, "review_required": true, "formal_state": "draft"}, Sources: []agenttools.ToolSource{{Type: "fpna_action_item", ID: item.ID, Title: item.Title, Locator: "assist_draft"}}}, nil
	}
}

func storeScenarioHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	if _, err := agenttools.RequireExecutionContext(ctx); err != nil {
		return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
	}
	args, err := decodeStrict[StoreScenarioArguments](call.Arguments)
	if err != nil || len(args.Scenarios) < 2 {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "at least two structured scenarios are required"), nil
	}
	result, err := operating.EvaluateStoreScenarios(args.Scenarios)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, err.Error()), nil
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"basis": "Scenario", "results": result, "review_required": true, "side_effects": false}, Sources: []agenttools.ToolSource{{Type: "scenario_assumptions", ID: call.CallID, Title: "门店情景假设", Locator: "agent_input"}}}, nil
}
func equipmentScenarioHandler(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
	if _, err := agenttools.RequireExecutionContext(ctx); err != nil {
		return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
	}
	args, err := decodeStrict[EquipmentScenarioArguments](call.Arguments)
	if err != nil || len(args.Scenarios) < 2 {
		return rejected(call.CallID, agenttools.ErrorInvalidArguments, "at least two structured scenarios are required"), nil
	}
	result, err := operating.EvaluateEquipmentScenarios(args.Scenarios)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, err.Error()), nil
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"basis": "Scenario", "results": result, "review_required": true, "side_effects": false}, Sources: []agenttools.ToolSource{{Type: "scenario_assumptions", ID: call.CallID, Title: "设备情景假设", Locator: "agent_input"}}}, nil
}

type performanceHandler func(context.Context, agenttools.ToolCall, PerformanceReader, PerformanceArguments) agenttools.ToolResult

func readPerformanceDefinition(name, display, description string, schema json.RawMessage, reader PerformanceReader, handler performanceHandler) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{Descriptor: agenttools.ToolDescriptor{Name: name, Version: "v1", DisplayName: display, Description: description, Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: schema, SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 15}, SkillIDs: []string{"fpna_copilot", "retail_performance", "manufacturing_performance", "audit_pack"}, Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeStrict[PerformanceArguments](call.Arguments)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments must be a JSON object with supported fields"), nil
		}
		if reader == nil {
			return rejected(call.CallID, agenttools.ErrorBusinessFailure, "performance reader unavailable"), nil
		}
		if _, err := agenttools.RequireExecutionContext(ctx); err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		return handler(ctx, call, reader, args), nil
	}}
}

func portfolioSummaryHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	data, err := reader.Overview(ctx, execution.Principal.Scope.LegalEntityID, strings.TrimSpace(args.Period))
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load portfolio summary")
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data, Sources: []agenttools.ToolSource{{Type: "performance_overview", ID: args.Period, Title: "经营组合摘要", Locator: "period=" + args.Period}}}
}

func managementPreReadHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	overview, err := reader.Overview(ctx, execution.Principal.Scope.LegalEntityID, args.Period)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load management pre-read")
	}
	actions, err := reader.ListActions(ctx, execution.Principal.Scope.LegalEntityID, args.Period, "", "")
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load pre-read actions")
	}
	questions := make([]string, 0, len(actions))
	sources := make([]agenttools.ToolSource, 0, len(actions)+1)
	sources = append(sources, agenttools.ToolSource{Type: "performance_overview", ID: args.Period, Title: "管理层会前经营摘要", Locator: "period=" + args.Period})
	for _, action := range actions {
		questions = append(questions, "请确认："+action.Title+" 的根因、负责人和预计兑现期间？")
		sources = append(sources, agenttools.ToolSource{Type: "fpna_action_item", ID: action.ID, Title: action.Title, Locator: action.SourceTable + ":" + action.SourceRecordID})
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"period": args.Period, "basis": "Working", "overview": overview, "priority_actions": actions, "questions": questions, "data_gaps": []string{}, "review_required": true}, Sources: sources}
}

func storePerformanceHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	rows, err := reader.ListStores(ctx, execution.Principal.Scope.LegalEntityID, args.Period, args.StoreID)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load store performance")
	}
	data := make([]operating.FourWall, 0, len(rows))
	sources := make([]agenttools.ToolSource, 0, len(rows))
	for _, row := range rows {
		data = append(data, operating.CalculateFourWall(*row))
		sources = append(sources, agenttools.ToolSource{Type: "store_operating_fact", ID: row.ID, Title: row.StoreCode + " " + row.Period, Locator: "store=" + row.StoreID + ";version=" + itoa(row.Version)})
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"period": args.Period, "items": data, "total": len(data)}, Sources: sources}
}

func rentToSalesHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	rows, err := reader.ListStores(ctx, execution.Principal.Scope.LegalEntityID, args.Period, args.StoreID)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load rent-to-sales facts")
	}
	items := make([]map[string]any, 0, len(rows))
	sources := make([]agenttools.ToolSource, 0, len(rows))
	for _, row := range rows {
		metric := operating.CalculateFourWall(*row)
		items = append(items, map[string]any{"store_id": row.StoreID, "store_code": row.StoreCode, "period": row.Period, "currency": row.Currency, "rent_to_sales": metric.RentToSales, "occupancy_cost_ratio": metric.OccupancyCostRatio, "data_gaps": metric.DataGaps})
		sources = append(sources, agenttools.ToolSource{Type: "store_operating_fact", ID: row.ID, Title: row.StoreCode + " rent-to-sales", Locator: "version=" + itoa(row.Version)})
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"period": args.Period, "items": items, "total": len(items)}, Sources: sources}
}

func equipmentPerformanceHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	rows, err := reader.ListEquipmentFacts(ctx, execution.Principal.Scope.LegalEntityID, args.Period, args.Plant, args.Line)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load equipment performance")
	}
	items := make([]map[string]any, 0, len(rows))
	sources := make([]agenttools.ToolSource, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{"fact": row}
		if bridge, bridgeErr := operating.CalculateCostBridge(*row); bridgeErr == nil {
			item["bridge"] = bridge
		} else {
			item["data_gaps"] = []string{"standard_cost", "actual_cost"}
		}
		items = append(items, item)
		sources = append(sources, agenttools.ToolSource{Type: "equipment_operating_fact", ID: row.ID, Title: row.EquipmentCode + " " + row.Period})
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"period": args.Period, "items": items, "total": len(items)}, Sources: sources}
}

func actionListHandler(ctx context.Context, call agenttools.ToolCall, reader PerformanceReader, args PerformanceArguments) agenttools.ToolResult {
	execution, _ := agenttools.RequireExecutionContext(ctx)
	rows, err := reader.ListActions(ctx, execution.Principal.Scope.LegalEntityID, args.Period, args.Status, args.Category)
	if err != nil {
		return rejected(call.CallID, agenttools.ErrorBusinessFailure, "failed to load FP&A actions")
	}
	sources := make([]agenttools.ToolSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, agenttools.ToolSource{Type: "fpna_action_item", ID: row.ID, Title: row.Title, Locator: row.SourceTable + ":" + row.SourceRecordID})
	}
	return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: map[string]any{"items": rows, "total": len(rows)}, Sources: sources}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}

func performanceDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
