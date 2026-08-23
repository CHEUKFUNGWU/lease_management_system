package tools

// B-1 单元测试：fpna.store_pnl.read 的校验、期间解析、法人必填、诚实降级与
// scope_denied 映射。写读路径的跨法人隔离见 aiagent 的带库集成测试。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

const testStoreID = "11111111-1111-4111-8111-111111111111"

type fakeStorePnlReader struct {
	calls    int
	query    StorePnlQuery
	response *storepnl.StorePnl
	err      error
}

func (f *fakeStorePnlReader) Project(_ context.Context, q StorePnlQuery) (*storepnl.StorePnl, error) {
	f.calls++
	f.query = q
	if f.err != nil {
		return nil, f.err
	}
	if f.response != nil {
		return f.response, nil
	}
	return &storepnl.StorePnl{
		StoreID:        q.Ref.StoreID,
		Classification: q.Ref.Classification,
		DatasetVersion: q.Ref.DatasetVersion,
		Period:         q.Period,
	}, nil
}

func storePnlToolCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "bp-1", Permissions: []string{"reports:read"},
			Scope: access.Scope{LegalEntityID: "entity-a"},
		},
		RunID: "run-1",
	})
}

func storePnlCall(args string) agenttools.ToolCall {
	return agenttools.ToolCall{CallID: "c1", ToolName: "fpna.store_pnl.read", Arguments: json.RawMessage(args)}
}

func TestStorePnlReadDefinitionDescriptor(t *testing.T) {
	def := NewStorePnlReadDefinition(&fakeStorePnlReader{})
	if def.Descriptor.Name != "fpna.store_pnl.read" || def.Descriptor.Level != agenttools.LevelRead || !def.Descriptor.ReadOnly {
		t.Fatalf("descriptor wrong: %+v", def.Descriptor)
	}
	if len(def.Descriptor.Permissions) == 0 {
		t.Fatal("descriptor must declare permissions")
	}
}

func TestStorePnlReadPeriodSpecResolvesWindow(t *testing.T) {
	reader := &fakeStorePnlReader{}
	def := NewStorePnlReadDefinition(reader)
	result, err := def.Handler(storePnlToolCtx(), storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"period":"2026-06",
		"data_classification":"production",
		"basis":"side_by_side",
		"primary":"actual",
		"secondary":"budget"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s: %+v", result.Status, result.Error)
	}
	if reader.calls != 1 {
		t.Fatalf("reader must be invoked once, got %d", reader.calls)
	}
	q := reader.query
	if q.Ref.StoreID != testStoreID || q.Ref.LegalEntityID != "entity-a" {
		t.Fatalf("ref wrong: %+v", q.Ref)
	}
	if q.Ref.DateFrom != "2026-06-01" || q.Ref.DateTo != "2026-06-30" || q.Ref.PeriodLabel != "2026-06" {
		t.Fatalf("calendar month window wrong: %+v", q.Ref)
	}
	if q.Period.From != "2026-06-01" || q.Period.To != "2026-06-30" {
		t.Fatalf("period wrong: %+v", q.Period)
	}
	if q.Basis != storepnl.BasisSideBySide || q.Pair != [2]storepnl.ColumnRef{storepnl.ColActual, storepnl.ColBudget} {
		t.Fatalf("basis/pair wrong: %s %v", q.Basis, q.Pair)
	}
	if _, ok := result.Data.(StorePnlToolData); !ok {
		t.Fatalf("data must be StorePnlToolData, got %T", result.Data)
	}
	if len(result.Sources) != 1 || result.Sources[0].Type != "store_pnl" {
		t.Fatalf("sources wrong: %+v", result.Sources)
	}
}

func TestStorePnlReadAsOfWindowDaysFallback(t *testing.T) {
	reader := &fakeStorePnlReader{}
	def := NewStorePnlReadDefinition(reader)
	result, err := def.Handler(storePnlToolCtx(), storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"as_of":"2026-06-14",
		"window_days":7,
		"data_classification":"simulated",
		"dataset_version":"planA-v1"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	q := reader.query
	if q.Ref.DateFrom != "" || q.Ref.DateTo != "" {
		t.Fatalf("legacy anchor must not resolve calendar window: %+v", q.Ref)
	}
	if q.Ref.AsOf != "2026-06-14" || q.Ref.WindowDays != 7 || q.Ref.Classification != "simulated" || q.Ref.DatasetVersion != "planA-v1" {
		t.Fatalf("ref wrong: %+v", q.Ref)
	}
	if q.Period.From != "2026-06-08" || q.Period.To != "2026-06-14" {
		t.Fatalf("period wrong: %+v", q.Period)
	}
}

func TestStorePnlReadWithoutReaderIsUnavailable(t *testing.T) {
	def := NewStorePnlReadDefinition(nil)
	result, err := def.Handler(storePnlToolCtx(), storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"period":"2026-06",
		"data_classification":"production"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable {
		t.Fatalf("expected unavailable, got %+v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "unavailable") {
		t.Fatalf("unavailable message must be honest: %q", result.Error.Message)
	}
}

func TestStorePnlReadRequiresLegalEntity(t *testing.T) {
	def := NewStorePnlReadDefinition(&fakeStorePnlReader{})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "admin", Permissions: []string{"*:*"}, Scope: access.Scope{Global: true}},
		RunID:     "run-1",
	})
	result, err := def.Handler(ctx, storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"period":"2026-06",
		"data_classification":"production"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
		t.Fatalf("missing legal entity must be scope_denied, got %+v", result.Error)
	}
}

func TestStorePnlReadValidation(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"missing store_id", `{"period":"2026-06","data_classification":"production"}`},
		{"bad store_id", `{"store_id":"not-a-uuid","period":"2026-06","data_classification":"production"}`},
		{"missing classification", `{"store_id":"` + testStoreID + `","period":"2026-06"}`},
		{"bad classification", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"live"}`},
		{"simulated without dataset", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"simulated"}`},
		{"production with dataset", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"production","dataset_version":"v1"}`},
		{"neither period nor as_of", `{"store_id":"` + testStoreID + `","data_classification":"production"}`},
		{"bad as_of", `{"store_id":"` + testStoreID + `","as_of":"2026/06/14","window_days":7,"data_classification":"production"}`},
		{"bad window_days", `{"store_id":"` + testStoreID + `","as_of":"2026-06-14","window_days":5,"data_classification":"production"}`},
		{"same columns", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"production","primary":"actual","secondary":"actual"}`},
		{"bad basis", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"production","basis":"cash"}`},
		{"bad period spec", `{"store_id":"` + testStoreID + `","period":"next-week","data_classification":"production"}`},
		{"unknown field", `{"store_id":"` + testStoreID + `","period":"2026-06","data_classification":"production","surprise":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := NewStorePnlReadDefinition(&fakeStorePnlReader{})
			result, err := def.Handler(storePnlToolCtx(), storePnlCall(tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
				t.Fatalf("expected invalid arguments, got %+v", result.Error)
			}
		})
	}
}

func TestStorePnlReadScopeDeniedKeepsReason(t *testing.T) {
	reader := &fakeStorePnlReader{err: ErrStoreScopeDenied()}
	def := NewStorePnlReadDefinition(reader)
	result, err := def.Handler(storePnlToolCtx(), storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"period":"2026-06",
		"data_classification":"production"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
		t.Fatalf("expected scope_denied, got %+v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "scope_denied") {
		t.Fatalf("scope_denied reason must be preserved, got %q", result.Error.Message)
	}
}

func TestStorePnlReadNotFoundReadsAsScopeDenied(t *testing.T) {
	reader := &fakeStorePnlReader{err: ErrStoreNotFound()}
	def := NewStorePnlReadDefinition(reader)
	result, err := def.Handler(storePnlToolCtx(), storePnlCall(`{
		"store_id":"`+testStoreID+`",
		"period":"2026-06",
		"data_classification":"production"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	// 异法人视同不存在：对 Agent 只呈现 scope_denied 措辞，不泄漏存在性。
	if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
		t.Fatalf("expected scope_denied for unknown/foreign store, got %+v", result.Error)
	}
	if strings.Contains(result.Error.Message, "store not found") {
		t.Fatalf("must not leak existence: %q", result.Error.Message)
	}
}
