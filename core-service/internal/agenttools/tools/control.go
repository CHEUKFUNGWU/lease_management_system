package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/cashflow"
)

// ControlReaders are application adapters for existing deterministic FP&A
// services. The Agent sees only these read seams; it never receives a handler,
// database pool or arbitrary HTTP capability.
type ControlReaders struct {
	Budget   BudgetVarianceReader
	Cashflow CashflowScenarioReader
	Renewal  RenewalDecisionReader
}

type BudgetVarianceReader interface {
	ReadVariance(context.Context, access.EntityFilter, string, string) (any, error)
}

type CashflowScenarioReader interface {
	ReadScenario(context.Context, string, CashflowScenarioArguments) (any, error)
}

type RenewalDecisionReader interface {
	ReadDecisions(context.Context, access.EntityFilter, string) (any, error)
}

type BudgetVarianceArguments struct {
	VersionID string `json:"version_id"`
	Period    string `json:"period"`
}

type CashflowScenarioArguments struct {
	AsOf          string              `json:"as_of"`
	HorizonMonths int                 `json:"horizon_months"`
	Scenarios     []cashflow.Scenario `json:"scenarios"`
}

type RenewalDecisionArguments struct {
	ContractID string `json:"contract_id"`
}

func NewBudgetVarianceDefinition(reader BudgetVarianceReader) agenttools.ToolDefinition {
	return controlReadDefinition(
		"fpna.budget.variance.read", "读取预算差异桥", "读取 Actual 与 Budget/Forecast 版本的确定性差异桥；未解释金额保留为 residual，不由 AI 分摊。",
		json.RawMessage(`{"type":"object","additionalProperties":false,"required":["version_id","period"],"properties":{"version_id":{"type":"string"},"period":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}$"}}}`),
		[]string{"fpna_copilot"},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, args BudgetVarianceArguments) (any, []agenttools.ToolSource, error) {
			if reader == nil {
				return nil, nil, errReaderUnavailable
			}
			entity, ok := filterFromExecution(execution)
			if !ok {
				return nil, nil, fmt.Errorf("legal entity scope is required")
			}
			data, err := reader.ReadVariance(ctx, entity, strings.TrimSpace(args.VersionID), strings.TrimSpace(args.Period))
			return data, []agenttools.ToolSource{{Type: "budget_variance", ID: args.VersionID + ":" + args.Period, Title: "预算差异桥", Locator: "version=" + args.VersionID + ";period=" + args.Period}}, err
		},
	)
}

func NewCashflowScenarioDefinition(reader CashflowScenarioReader) agenttools.ToolDefinition {
	return controlReadDefinition(
		"fpna.cashflow.scenario", "模拟租赁现金流方案", "调用现有现金流确定性服务比较续租/关店比例和租金假设；只返回 Scenario，不改变 Forecast 或合同。",
		json.RawMessage(`{"type":"object","additionalProperties":false,"required":["as_of","horizon_months","scenarios"],"properties":{"as_of":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},"horizon_months":{"type":"integer","minimum":1},"scenarios":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string"},"renewal_rate":{"type":"number","minimum":0,"maximum":1},"renewal_term_months":{"type":"integer","minimum":0},"renewal_uplift_percent":{"type":"number"},"closure_rate":{"type":"number","minimum":0,"maximum":1},"closure_cost_months":{"type":"number","minimum":0}}}}}}`),
		[]string{"fpna_copilot"},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, args CashflowScenarioArguments) (any, []agenttools.ToolSource, error) {
			if reader == nil {
				return nil, nil, errReaderUnavailable
			}
			data, err := reader.ReadScenario(ctx, execution.Principal.Scope.LegalEntityID, args)
			return data, []agenttools.ToolSource{{Type: "cashflow_scenario", ID: call.CallID, Title: "租赁现金流情景", Locator: "agent_input"}}, err
		},
	)
}

func NewRenewalDecisionDefinition(reader RenewalDecisionReader) agenttools.ToolDefinition {
	return controlReadDefinition(
		"lease.renewal.decisions", "读取续租决策证据", "读取合同范围内已经保存的续租情景快照；不会创建或审批续租事件。",
		json.RawMessage(`{"type":"object","additionalProperties":false,"required":["contract_id"],"properties":{"contract_id":{"type":"string","format":"uuid"}}}`),
		[]string{"fpna_copilot"},
		func(ctx context.Context, call agenttools.ToolCall, execution agenttools.ExecutionContext, args RenewalDecisionArguments) (any, []agenttools.ToolSource, error) {
			if reader == nil {
				return nil, nil, errReaderUnavailable
			}
			entity, ok := filterFromExecution(execution)
			if !ok {
				return nil, nil, fmt.Errorf("legal entity scope is required")
			}
			data, err := reader.ReadDecisions(ctx, entity, strings.TrimSpace(args.ContractID))
			return data, []agenttools.ToolSource{{Type: "renewal_decision_snapshot", ID: args.ContractID, Title: "续租决策快照", Locator: "contract=" + args.ContractID}}, err
		},
	)
}

var errReaderUnavailable = &readerError{message: "control reader unavailable"}

type readerError struct{ message string }

func (e *readerError) Error() string { return e.message }

func controlReadDefinition[T any](name, display, description string, schema json.RawMessage, skills []string, read func(context.Context, agenttools.ToolCall, agenttools.ExecutionContext, T) (any, []agenttools.ToolSource, error)) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{Name: name, Version: "v1", DisplayName: display, Description: description, Level: agenttools.LevelRead, ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}}, InputSchema: schema, SupportsDryRun: true, MaxRows: 2000, TimeoutSeconds: 20},
		SkillIDs:   skills,
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			var args T
			if args, err = decodeStrict[T](call.Arguments); err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, "arguments contain unsupported or invalid fields"), nil
			}
			data, sources, readErr := read(ctx, call, execution, args)
			if readErr != nil {
				return rejected(call.CallID, agenttools.ErrorBusinessFailure, readErr.Error()), nil
			}
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data, Sources: sources}, nil
		},
	}
}
