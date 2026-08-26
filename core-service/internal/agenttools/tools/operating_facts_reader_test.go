package tools

// B-3 经营事实工具单元测试：诚实降级、skill 限定、参数校验、法人必填、
// 来源信封逐字段回显、严格 null 语义与 decision_ready 降级。
// 跨法人隔离与零写入见同目录带库集成测试（make test-integration 实跑）。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakeOperatingFactsReader struct {
	stores     []*repository.StoreOperatingFact
	pageResult *repository.RetailStoreDayFactsPage
	calls      map[string]int
}

func newFakeFactsReader() *fakeOperatingFactsReader {
	return &fakeOperatingFactsReader{calls: map[string]int{}}
}

func (f *fakeOperatingFactsReader) ListStores(context.Context, access.EntityFilter, string, string) ([]*repository.StoreOperatingFact, error) {
	f.calls["stores"]++
	return f.stores, nil
}

func (f *fakeOperatingFactsReader) ListRetailStoreDayFactsPage(context.Context, access.EntityFilter, string, string, []string, string, int, int) (*repository.RetailStoreDayFactsPage, error) {
	f.calls["store_days"]++
	return f.pageResult, nil
}

type fakeKpiFactReader struct {
	query func(call int, stores []string) (*repository.RetailKPIFactSet, error)
	calls int
}

func (f *fakeKpiFactReader) QueryFacts(ctx context.Context, legal, from, to, class, dataset, source string, stores []string) (*repository.RetailKPIFactSet, error) {
	f.calls++
	if f.query != nil {
		return f.query(f.calls, stores)
	}
	return &repository.RetailKPIFactSet{}, nil
}

func factsCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "bp-1", Permissions: []string{"reports:read"},
			Scope: access.Scope{LegalEntityID: "entity-a"},
		},
		RunID: "run-1", SkillID: "retail_operations", SkillVersion: "v1",
	})
}

func TestOperatingFactsToolsWithoutReaderAreUnavailable(t *testing.T) {
	for _, def := range []agenttools.ToolDefinition{
		NewOperatingStoresDefinition(nil),
		NewOperatingStoreDaysDefinition(nil),
		NewKpiStoreDaysDefinition(nil),
	} {
		result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"production"}`))
		if err != nil {
			t.Fatalf("%s: %v", def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable || !strings.Contains(result.Error.Message, "unavailable") {
			t.Fatalf("%s: expected honest unavailable, got %+v", def.Descriptor.Name, result.Error)
		}
	}
}

func factsCall(name, args string) agenttools.ToolCall {
	return agenttools.ToolCall{CallID: "c1", ToolName: name, Arguments: json.RawMessage(args)}
}

func TestOperatingFactsToolsRestrictedToRetailSkill(t *testing.T) {
	wrongSkillCtx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-1", Permissions: []string{"reports:read"}, Scope: access.Scope{LegalEntityID: "entity-a"}},
		RunID:     "run-1", SkillID: "fpna_copilot",
	})
	for _, def := range []agenttools.ToolDefinition{
		NewOperatingStoresDefinition(newFakeFactsReader()),
		NewOperatingStoreDaysDefinition(newFakeFactsReader()),
		NewKpiStoreDaysDefinition(&fakeKpiFactReader{}),
	} {
		result, err := def.Handler(wrongSkillCtx, factsCall(def.Descriptor.Name, `{"date_from":"2026-06-01","date_to":"2026-06-02","data_classification":"production"}`))
		if err != nil {
			t.Fatalf("%s: %v", def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorPermissionDenied {
			t.Fatalf("%s: non-retail skill must be denied, got %+v", def.Descriptor.Name, result.Error)
		}
	}
}

func TestOperatingFactsToolsRequireLegalEntity(t *testing.T) {
	noEntityCtx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "admin", Permissions: []string{"*:*"}},
		RunID:     "run-1", SkillID: "retail_operations",
	})
	for _, tc := range []struct {
		def  agenttools.ToolDefinition
		args string
	}{
		{NewOperatingStoresDefinition(newFakeFactsReader()), `{}`},
		{NewOperatingStoreDaysDefinition(newFakeFactsReader()), `{"date_from":"2026-06-01","date_to":"2026-06-02","data_classification":"production"}`},
		{NewKpiStoreDaysDefinition(&fakeKpiFactReader{}), `{"date_from":"2026-06-01","date_to":"2026-06-02","data_classification":"production"}`},
	} {
		result, err := tc.def.Handler(noEntityCtx, factsCall(tc.def.Descriptor.Name, tc.args))
		if err != nil {
			t.Fatalf("%s: %v", tc.def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
			t.Fatalf("%s: missing legal entity must be scope_denied, got %+v", tc.def.Descriptor.Name, result.Error)
		}
	}
}

func TestOperatingFactsDescriptorsDeclarePermissions(t *testing.T) {
	for _, def := range []agenttools.ToolDefinition{
		NewOperatingStoresDefinition(newFakeFactsReader()),
		NewOperatingStoreDaysDefinition(newFakeFactsReader()),
		NewKpiStoreDaysDefinition(&fakeKpiFactReader{}),
	} {
		d := def.Descriptor
		if !strings.HasPrefix(d.Name, "retail.") {
			t.Fatalf("%s must live in the retail.* namespace", d.Name)
		}
		if len(d.Permissions) != 1 || d.Permissions[0].Resource != "reports" || d.Permissions[0].Action != "read" {
			t.Fatalf("%s permissions wrong: %+v", d.Name, d.Permissions)
		}
		if d.Level != agenttools.LevelRead || !d.ReadOnly {
			t.Fatalf("%s must be read-only", d.Name)
		}
	}
}

func TestOperatingStoreDaysValidation(t *testing.T) {
	reader := newFakeFactsReader()
	def := NewOperatingStoreDaysDefinition(reader)
	cases := []struct{ name, args string }{
		{"missing dates", `{}`},
		{"bad date format", `{"date_from":"2026/06/01","date_to":"2026-06-07"}`},
		{"reversed range", `{"date_from":"2026-06-07","date_to":"2026-06-01"}`},
		{"range too long", `{"date_from":"2025-01-01","date_to":"2026-06-01"}`},
		{"bad classification", `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"live"}`},
		{"bad store id", `{"date_from":"2026-06-01","date_to":"2026-06-07","store_ids":["nope"]}`},
		{"page_size over max", `{"date_from":"2026-06-01","date_to":"2026-06-07","page_size":5001}`},
		{"unknown field", `{"date_from":"2026-06-01","date_to":"2026-06-07","surprise":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
				t.Fatalf("expected invalid arguments, got %+v", result.Error)
			}
		})
	}
	if reader.calls["store_days"] != 0 {
		t.Fatal("rejected calls must not reach the reader")
	}
}

// 来源信封逐字段断言：data_classification / source_system / import_batch_id /
// as_of_at / version 五个字段一个都不能丢（底线 3）。
func TestOperatingStoreDaysEchoEnvelopeFieldByField(t *testing.T) {
	reader := newFakeFactsReader()
	batchID := "batch-xyz-001"
	asOf := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	reader.pageResult = &repository.RetailStoreDayFactsPage{
		Data: []*repository.RetailStoreDayFact{
			{
				ID: "row-1", StoreID: "11111111-1111-4111-8111-111111111111", StoreCode: "S1",
				BusinessDate: "2026-06-05", Currency: "CNY", Revenue: 100,
				GrossProfit:  nil, // 缺失是 nil，不填 0
				SourceSystem: "agent-b3-fixture", ImportBatchID: &batchID,
				AsOfAt: asOf, Version: 2, DataClassification: "production",
			},
		},
		Total: 1,
	}
	def := NewOperatingStoreDaysDefinition(reader)
	result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"date_from":"2026-06-01","date_to":"2026-06-07"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s %+v", result.Status, result.Error)
	}
	data, ok := result.Data.(OperatingStoreDaysToolData)
	if !ok {
		t.Fatalf("data type %T", result.Data)
	}
	if len(data.Data) != 1 {
		t.Fatalf("rows wrong: %d", len(data.Data))
	}
	row := data.Data[0]
	// 逐字段断言，不是「信封非空」。
	if row.DataClassification != "production" {
		t.Fatalf("envelope data_classification lost: %q", row.DataClassification)
	}
	if row.SourceSystem != "agent-b3-fixture" {
		t.Fatalf("envelope source_system lost: %q", row.SourceSystem)
	}
	if row.ImportBatchID == nil || *row.ImportBatchID != "batch-xyz-001" {
		t.Fatalf("envelope import_batch_id lost: %v", row.ImportBatchID)
	}
	if !row.AsOfAt.Equal(asOf) {
		t.Fatalf("envelope as_of_at lost: %v", row.AsOfAt)
	}
	if row.Version != 2 {
		t.Fatalf("envelope version lost: %d", row.Version)
	}
	// 严格 null：缺失指标保持 nil。
	if row.GrossProfit != nil {
		t.Fatalf("missing metric must stay nil, got %v", *row.GrossProfit)
	}
	// 汇总信封同样回显来源维度。
	if data.Envelope.FactVersionMin != 2 || data.Envelope.FactVersionMax != 2 {
		t.Fatalf("aggregate fact version range wrong: %+v", data.Envelope)
	}
	if len(data.Envelope.SourceSystems) != 1 || data.Envelope.SourceSystems[0] != "agent-b3-fixture" {
		t.Fatalf("aggregate source_systems wrong: %+v", data.Envelope.SourceSystems)
	}
	if data.Basis != "Working" || data.SideEffects {
		t.Fatalf("basis/side-effects wrong: %+v", data)
	}
}

func TestKpiStoreDaysValidationAndNullSemantics(t *testing.T) {
	reader := &fakeKpiFactReader{}
	def := NewKpiStoreDaysDefinition(reader)
	cases := []struct{ name, args string }{
		{"missing classification", `{"date_from":"2026-06-01","date_to":"2026-06-07"}`},
		{"simulated without dataset", `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"simulated"}`},
		{"production with dataset", `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"production","dataset_version":"v1"}`},
		{"bad group_by", `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"production","group_by":"planet"}`},
		{"range too long", `{"date_from":"2024-06-01","date_to":"2026-06-07","data_classification":"production"}`},
		{"unknown field", `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"production","surprise":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
				t.Fatalf("expected invalid arguments, got %+v", result.Error)
			}
		})
	}

	// 覆盖不足 → decision_ready=false 且带原因；缺失 KPI 的 value 是 null 不是 0。
	reader.query = func(call int, stores []string) (*repository.RetailKPIFactSet, error) {
		value := 100.0
		storeID := "11111111-1111-4111-8111-111111111111"
		day, _ := time.Parse("2006-01-02", "2026-06-03")
		return &repository.RetailKPIFactSet{
			ExpectedStoreCount: 2, // 种群 2 家，实际只有 1 家有事实 → 覆盖不足
			ExpectedStores:     []retailkpi.StorePopulation{{StoreID: storeID, StoreCode: "S1"}},
			Facts: []retailkpi.DailyFact{{
				StoreID: storeID, BusinessDate: day, Currency: "CNY",
				SourceSystem: "agent-b3-fixture", DataClassification: "production",
				Version: 1, Revenue: &value, LaborCost: nil, // labor_cost 缺失
				MappingStatus: "mapped", DataQualityStatus: "valid",
			}},
		}, nil
	}
	result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"date_from":"2026-06-01","date_to":"2026-06-07","data_classification":"production"}`))
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(KpiStoreDaysToolData)
	if !ok {
		t.Fatalf("data type %T", result.Data)
	}
	if data.DecisionReady {
		t.Fatal("insufficient coverage must degrade to decision_ready=false")
	}
	if data.DecisionReadyReason == "" {
		t.Fatal("degraded decision_ready must carry a reason")
	}
	if len(data.Data) != 1 {
		t.Fatalf("aggregates wrong: %d", len(data.Data))
	}
	laborCost, exists := data.Data[0].KPIs["labor_cost"]
	if !exists {
		t.Fatal("labor_cost KPI entry must exist even when unavailable")
	}
	if laborCost.Value != nil {
		t.Fatalf("unavailable labor_cost must have nil value, got %v", *laborCost.Value)
	}
	if data.SideEffects {
		t.Fatal("read tool must declare zero side effects")
	}
}
