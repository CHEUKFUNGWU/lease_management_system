package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakePerformanceReader struct{}

func (fakePerformanceReader) Overview(context.Context, access.EntityFilter, string) (*repository.PerformanceOverview, error) {
	return &repository.PerformanceOverview{Period: "2026-07", OpenActionCount: 2}, nil
}
func (fakePerformanceReader) ListStores(context.Context, access.EntityFilter, string, string) ([]*repository.StoreOperatingFact, error) {
	return []*repository.StoreOperatingFact{{ID: "fact-1", StoreID: "store-1", StoreCode: "S-1", Period: "2026-07", Currency: "CNY", Revenue: 10}}, nil
}
func (fakePerformanceReader) ListEquipmentFacts(context.Context, access.EntityFilter, string, string, string) ([]*repository.EquipmentOperatingFact, error) {
	return []*repository.EquipmentOperatingFact{{ID: "fact-2", EquipmentID: "equipment-1", EquipmentCode: "EQ-1", Period: "2026-07", Currency: "CNY"}}, nil
}
func (fakePerformanceReader) ListActions(context.Context, access.EntityFilter, string, string, string) ([]*repository.FPnAActionItem, error) {
	return []*repository.FPnAActionItem{{ID: "action-1", Title: "缺少对账", SourceTable: "store_operating_facts", SourceRecordID: "fact-1"}}, nil
}

type fakeActionWriter struct{}

func (fakeActionWriter) CreateAction(_ context.Context, item *repository.FPnAActionItem) (*repository.FPnAActionItem, error) {
	item.ID = "draft-1"
	return item, nil
}

type fakeScenarioWriter struct{}

func (fakeScenarioWriter) CreateScenarioDraft(_ context.Context, item *repository.FPnAScenarioDraft) (*repository.FPnAScenarioDraft, error) {
	item.ID = "scenario-draft-1"
	return item, nil
}

func TestPerformanceDefinitionsAreReadOnlyAndReturnSources(t *testing.T) {
	reader := fakePerformanceReader{}
	definitions := []agenttools.ToolDefinition{NewPortfolioSummaryDefinition(reader), NewStorePerformanceDefinition(reader), NewEquipmentPerformanceDefinition(reader), NewActionListDefinition(reader)}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u1", Permissions: []string{"reports:read"}, Scope: access.Scope{Global: true}}, RunID: "run-1"})
	for _, definition := range definitions {
		if definition.Descriptor.Level != agenttools.LevelRead || !definition.Descriptor.ReadOnly {
			t.Fatalf("definition %s is not read-only", definition.Descriptor.Name)
		}
		args := map[string]any{}
		if definition.Descriptor.Name != "lease.portfolio.summary" {
			args["period"] = "2026-07"
		}
		encoded, _ := json.Marshal(args)
		result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "call-1", RunID: "run-1", Arguments: encoded})
		if err != nil || result.Status != agenttools.StatusCompleted || len(result.Sources) == 0 {
			t.Fatalf("tool %s result=%+v err=%v", definition.Descriptor.Name, result, err)
		}
	}
}

func TestPerformanceToolRejectsUnknownFields(t *testing.T) {
	definition := NewStorePerformanceDefinition(fakePerformanceReader{})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u1", Permissions: []string{"reports:read"}, Scope: access.Scope{Global: true}}, RunID: "run-1"})
	result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "call-1", RunID: "run-1", Arguments: json.RawMessage(`{"period":"2026-07","user_id":"forged"}`)})
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestActionDraftIsReviewableAndCarriesAssistEvidence(t *testing.T) {
	definition := NewActionDraftDefinition(fakeActionWriter{})
	if err := definition.Descriptor.Validate(); err != nil {
		t.Fatal(err)
	}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u1", Permissions: []string{"fpna_actions:write"}, Scope: access.Scope{LegalEntityID: "le-1"}}, RunID: "run-1"})
	result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "call-1", RunID: "run-1", IdempotencyKey: "idem-1", Arguments: json.RawMessage(`{"category":"rent_to_sales","title":"租售比超阈值","rule_code":"rent_to_sales_high","source_table":"store_operating_facts","source_record_id":"fact-1"}`)})
	if err != nil || result.Status != agenttools.StatusCompleted || len(result.Sources) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	payload, payloadOK := result.Data.(map[string]any)
	draft, ok := payload["draft"].(*repository.FPnAActionItem)
	if !payloadOK || !ok || draft.IdempotencyKey != "idem-1" {
		t.Fatalf("idempotency key was not preserved: %#v", result.Data)
	}
}

type fakeBudgetReader struct{}

func (fakeBudgetReader) ReadVariance(context.Context, access.EntityFilter, string, string) (any, error) {
	return map[string]any{"result": "ok"}, nil
}

type fakeCashflowReader struct{}

func (fakeCashflowReader) ReadScenario(context.Context, string, CashflowScenarioArguments) (any, error) {
	return map[string]any{"side_effects": false}, nil
}

type fakeRenewalReader struct{}

func (fakeRenewalReader) ReadDecisions(context.Context, access.EntityFilter, string) (any, error) {
	return map[string]any{"total": 1}, nil
}

func TestControlReadDefinitionsAreStrictAndReadOnly(t *testing.T) {
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u1", Permissions: []string{"reports:read"}, Scope: access.Scope{Global: true}}})
	definitions := []agenttools.ToolDefinition{
		NewBudgetVarianceDefinition(fakeBudgetReader{}),
		NewCashflowScenarioDefinition(fakeCashflowReader{}),
		NewRenewalDecisionDefinition(fakeRenewalReader{}),
	}
	args := []string{
		`{"version_id":"v1","period":"2026-07"}`,
		`{"as_of":"2026-07-01","horizon_months":12,"scenarios":[{"name":"base"}]}`,
		`{"contract_id":"00000000-0000-0000-0000-000000000001"}`,
	}
	for index, definition := range definitions {
		if definition.Descriptor.Level != agenttools.LevelRead || !definition.Descriptor.ReadOnly {
			t.Fatalf("%s must be read-only", definition.Descriptor.Name)
		}
		result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "call-control", Arguments: json.RawMessage(args[index])})
		if err != nil || result.Status != agenttools.StatusCompleted || len(result.Sources) == 0 {
			t.Fatalf("%s result=%+v err=%v", definition.Descriptor.Name, result, err)
		}
	}
	result, err := definitions[0].Handler(ctx, agenttools.ToolCall{CallID: "call-control-bad", Arguments: json.RawMessage(`{"version_id":"v1","period":"2026-07","role":"admin"}`)})
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("unknown fields must be rejected: result=%+v err=%v", result, err)
	}
}

func TestScenarioDraftRequiresReviewAndIdempotencyEvidence(t *testing.T) {
	definition := NewScenarioDraftDefinition(fakeScenarioWriter{})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u1", Permissions: []string{"fpna_actions:write"}, Scope: access.Scope{LegalEntityID: "le-1"}}, RunID: "run-1"})
	result, err := definition.Handler(ctx, agenttools.ToolCall{CallID: "scenario-call", IdempotencyKey: "scenario-idem", Arguments: json.RawMessage(`{"scenario_type":"store_decision","name":"renew-vs-close","assumptions":{"discount_rate":0.12}}`)})
	if err != nil || result.Status != agenttools.StatusCompleted || len(result.Sources) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
