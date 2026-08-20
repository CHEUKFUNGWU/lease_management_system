package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

type stmtModelReader struct{ run, def json.RawMessage }

func (s stmtModelReader) ReadRun(_ context.Context, _ string) (json.RawMessage, error) {
	return s.run, nil
}
func (s stmtModelReader) ReadDefinition(_ context.Context, _ string) (json.RawMessage, error) {
	return s.def, nil
}

type stmtPorts struct{}

func (stmtPorts) Build(_ context.Context, _ agenttools.Principal, _ json.RawMessage) (finmodel.ModelDef, finmodel.ModelInputs, error) {
	tmpl, _ := template.DefaultStatementTemplate()
	return finmodel.ModelDef{
			Name: "tiny", LegalEntityID: "LE-1", Currency: "CNY", Template: tmpl,
			PeriodStart: "2026-01", HistoricalMonths: 1, ForecastMonths: 1,
			ActualCutoffPeriod: "2026-01",
			Policy:             finmodel.ModelPolicy{Version: "p1", InterestCashFlowPresentation: "financing"},
		}, finmodel.ModelInputs{
			Facts: stmtFacts{}, Lease: stmtLease{}, Schedules: stmtSched{},
			Assumptions: stmtAssumptions{}, Versions: finmodel.VersionSet{Data: "ds-1", Assumption: "as-1", ModelDefinition: "v1"},
			DataClassification: "production",
		}, nil
}

type stmtFacts struct{}

func (stmtFacts) Operating(_ context.Context, _, period string) (finmodel.OperatingFacts, error) {
	v := 1000.0
	return finmodel.OperatingFacts{Revenue: &v, GrossProfit: &v, LaborCost: &v, DecisionReady: true, DataClassification: "production"}, nil
}

type stmtLease struct{}

func (stmtLease) Monthly(_ context.Context, _, _ string) (finmodel.LeaseMonth, error) {
	z := 0.0
	return finmodel.LeaseMonth{ROUAsset: &z, LeaseLiability: &z, Interest: &z, Depreciation: &z, Payments: &z, Principal: &z, Additions: &z, Remeasurements: &z, Terminations: &z}, nil
}

type stmtSched struct{}

func (stmtSched) Monthly(_ context.Context, _, _ string) (finmodel.ScheduleFanout, error) {
	return finmodel.ScheduleFanout{}, nil
}

type stmtAssumptions struct{}

func (stmtAssumptions) Value(_ context.Context, _, key, _ string) (json.RawMessage, error) {
	values := map[string]float64{"gross_margin_rate": 0.4, "borrow_interest_rate": 0.05, "tax_rate": 0.25, "dso": 10, "dio": 20, "dpo": 15, "days": 30, "dividend_payout_rate": 0}
	v, ok := values[key]
	if !ok {
		return nil, nil
	}
	return json.Marshal(v)
}

func stmtCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "fpna-u1", Permissions: []string{"*:*"}, Scope: access.Scope{LegalEntityID: "LE-1"}},
		RunID:     "run-fm-1",
	})
}

func TestStatementModelReadTool(t *testing.T) {
	def := NewStatementModelReadDefinition(stmtModelReader{run: json.RawMessage(`{"tie_out_status":"passed"}`), def: json.RawMessage(`{"name":"m"}`)})
	result, err := def.Handler(stmtCtx(), agenttools.ToolCall{Arguments: json.RawMessage(`{"run_id":"r1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	if data["run"] == nil {
		t.Fatalf("run payload missing: %+v", data)
	}
}

func TestStatementModelEvaluateNoSideEffects(t *testing.T) {
	def := NewStatementModelEvaluateDefinition(stmtPorts{})
	result, err := def.Handler(stmtCtx(), agenttools.ToolCall{Arguments: json.RawMessage(`{"model":{"name":"tiny"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	if data["side_effects"] != false {
		t.Fatal("evaluate must be side-effect free")
	}
	runResult, ok := data["result"].(*finmodel.RunResult)
	if !ok || len(runResult.Periods) != 2 {
		t.Fatalf("run result wrong: %+v", runResult)
	}
}

// P1-3 契约：stmtPorts 的极简输入无法让出厂三表模板全绿（T2/T5/T10 等
// 失败），工具必须拒绝产出 artifact——失败的 run 永远不生成底稿。
func TestFinModelPaperToolRefusesFailedTieOutRun(t *testing.T) {
	def := NewFinModelPaperDefinition(stmtPorts{})
	_, err := def.Handler(stmtCtx(), agenttools.ToolCall{
		CallID:    "call-fm-1",
		Arguments: json.RawMessage(`{"model":{"name":"tiny"},"title":"月度三表底稿"}`),
	})
	if err == nil {
		t.Fatal("a run whose tie-outs fail must be refused (no artifact)")
	}
	if !strings.Contains(err.Error(), "tie_out_unpassed") {
		t.Fatalf("the refusal must name the tie_out_unpassed gate, got %v", err)
	}
}
