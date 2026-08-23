package tools

// B-2 月结只读工具单元测试：诚实降级、参数校验、法人必填、Working 口径
// 信封与锁账状态回显。跨法人隔离与零写入见同目录带库集成测试。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeMonthlyClosingReader struct {
	calls   map[string]int
	batches []*repository.MonthlyClosingBatch
	items   []*repository.JournalEntry
	periods []repository.JournalEntryPeriod
	locked  map[string]bool // period → locked
}

func newFakeMCReader() *fakeMonthlyClosingReader {
	return &fakeMonthlyClosingReader{calls: map[string]int{}, locked: map[string]bool{}}
}

func (f *fakeMonthlyClosingReader) GetBatches(_ context.Context, period, _ string) ([]*repository.MonthlyClosingBatch, error) {
	f.calls["batches"]++
	out := make([]*repository.MonthlyClosingBatch, 0, len(f.batches))
	for _, b := range f.batches {
		if period == "" || b.AccountingPeriod == period {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeMonthlyClosingReader) ListJournalEntries(_ context.Context, q repository.JournalEntryQuery) ([]*repository.JournalEntry, repository.JournalEntrySummary, error) {
	f.calls["entries"]++
	out := make([]*repository.JournalEntry, 0, len(f.items))
	for _, e := range f.items {
		if q.Status != "" && e.PostingStatus != q.Status {
			continue
		}
		out = append(out, e)
	}
	summary := repository.JournalEntrySummary{Total: len(out)}
	for _, e := range out {
		switch e.PostingStatus {
		case "draft":
			summary.DraftCount++
		case "approved":
			summary.ApprovedCount++
		case "posted":
			summary.PostedCount++
		case "reversed":
			summary.ReversedCount++
		}
	}
	return out, summary, nil
}

func (f *fakeMonthlyClosingReader) ListEntryPeriods(_ context.Context, _ string, limit int) ([]repository.JournalEntryPeriod, error) {
	f.calls["periods"]++
	if limit <= 0 || limit > 120 {
		limit = 36
	}
	if len(f.periods) > limit {
		return f.periods[:limit], nil
	}
	return f.periods, nil
}

func (f *fakeMonthlyClosingReader) IsPeriodLocked(_ context.Context, period, _ string) (bool, error) {
	f.calls["lock"]++
	return f.locked[period], nil
}

func mcCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "bp-1", Permissions: []string{"monthly_closing:read"},
			Scope: access.Scope{LegalEntityID: "entity-a"},
		},
		RunID: "run-1",
	})
}

func TestMonthlyClosingToolsWithoutReaderAreUnavailable(t *testing.T) {
	for _, def := range []agenttools.ToolDefinition{
		NewMonthlyClosingBatchesDefinition(nil),
		NewMonthlyClosingEntriesPreviewDefinition(nil),
		NewMonthlyClosingPeriodsDefinition(nil),
		NewMonthlyClosingLockStatusDefinition(nil),
	} {
		result, err := def.Handler(mcCtx(), mcCall(def.Descriptor.Name, `{"period":"2026-06"}`))
		if err != nil {
			t.Fatalf("%s: %v", def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable || !strings.Contains(result.Error.Message, "unavailable") {
			t.Fatalf("%s: expected honest unavailable, got %+v", def.Descriptor.Name, result.Error)
		}
	}
}

func mcCall(name, args string) agenttools.ToolCall {
	return agenttools.ToolCall{CallID: "c1", ToolName: name, Arguments: json.RawMessage(args)}
}

func TestMonthlyClosingToolsRequireLegalEntity(t *testing.T) {
	reader := newFakeMCReader()
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "admin", Permissions: []string{"*:*"}, Scope: access.Scope{Global: true}},
		RunID:     "run-1",
	})
	defs := []struct {
		def  agenttools.ToolDefinition
		args string
	}{
		{NewMonthlyClosingBatchesDefinition(reader), `{}`},
		{NewMonthlyClosingEntriesPreviewDefinition(reader), `{"period":"2026-06"}`},
		{NewMonthlyClosingPeriodsDefinition(reader), `{}`},
		{NewMonthlyClosingLockStatusDefinition(reader), `{"period":"2026-06"}`},
	}
	for _, tc := range defs {
		result, err := tc.def.Handler(ctx, mcCall(tc.def.Descriptor.Name, tc.args))
		if err != nil {
			t.Fatalf("%s: %v", tc.def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
			t.Fatalf("%s: missing legal entity must be scope_denied, got %+v", tc.def.Descriptor.Name, result.Error)
		}
	}
}

func TestMonthlyClosingDescriptorsDeclarePermissions(t *testing.T) {
	for _, def := range []agenttools.ToolDefinition{
		NewMonthlyClosingBatchesDefinition(newFakeMCReader()),
		NewMonthlyClosingEntriesPreviewDefinition(newFakeMCReader()),
		NewMonthlyClosingPeriodsDefinition(newFakeMCReader()),
		NewMonthlyClosingLockStatusDefinition(newFakeMCReader()),
	} {
		d := def.Descriptor
		if !strings.HasPrefix(d.Name, "lease.monthly_closing.") {
			t.Fatalf("%s must live in the lease.* namespace", d.Name)
		}
		if len(d.Permissions) != 1 || d.Permissions[0].Resource != "monthly_closing" || d.Permissions[0].Action != "read" {
			t.Fatalf("%s permissions wrong: %+v", d.Name, d.Permissions)
		}
		if d.Level != agenttools.LevelRead || !d.ReadOnly {
			t.Fatalf("%s must be read-only", d.Name)
		}
	}
}

func TestMonthlyClosingEntriesPreviewValidation(t *testing.T) {
	reader := newFakeMCReader()
	def := NewMonthlyClosingEntriesPreviewDefinition(reader)
	cases := []struct{ name, args string }{
		{"missing period", `{}`},
		{"bad period", `{"period":"2026-6"}`},
		{"bad status", `{"period":"2026-06","status":"pending"}`},
		{"unknown field", `{"period":"2026-06","surprise":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := def.Handler(mcCtx(), mcCall(def.Descriptor.Name, tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
				t.Fatalf("expected invalid arguments, got %+v", result.Error)
			}
		})
	}
	if reader.calls["entries"] != 0 {
		t.Fatal("rejected calls must not reach the reader")
	}

	// page_size 超上限：与 HTTP 面一致地钐制（NormalizeEntryPaging），不报错；
	// 响应回显实际生效的页大小。
	result, err := def.Handler(mcCtx(), mcCall(def.Descriptor.Name, `{"period":"2026-06","page_size":501}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("oversized page_size must clamp, got %+v", result.Error)
	}
	if data := result.Data.(MonthlyClosingEntriesPreviewToolData); data.PageSize != 500 {
		t.Fatalf("page_size must clamp to 500, got %d", data.PageSize)
	}
}

func TestMonthlyClosingEntriesPreviewEnvelope(t *testing.T) {
	reader := newFakeMCReader()
	amount := money.NewFromFloat(100)
	draft := &repository.JournalEntry{ID: "e1", AccountingPeriod: "2026-06", PostingStatus: "draft", Amount: amount}
	posted := &repository.JournalEntry{ID: "e2", AccountingPeriod: "2026-06", PostingStatus: "posted", Amount: amount}
	approved := &repository.JournalEntry{ID: "e3", AccountingPeriod: "2026-06", PostingStatus: "approved"}
	reader.items = []*repository.JournalEntry{draft, posted, approved}
	reader.locked["2026-06"] = true

	def := NewMonthlyClosingEntriesPreviewDefinition(reader)
	result, err := def.Handler(mcCtx(), mcCall(def.Descriptor.Name, `{"period":"2026-06"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s %+v", result.Status, result.Error)
	}
	data, ok := result.Data.(MonthlyClosingEntriesPreviewToolData)
	if !ok {
		t.Fatalf("data type %T", result.Data)
	}
	if data.ReportBasis != "draft" || data.IsOfficialVersion {
		t.Fatalf("preview must declare working/non-official basis: %+v", data)
	}
	if !data.PeriodLocked {
		t.Fatal("period lock status must be surfaced")
	}
	if data.ApprovalStatus.Total != 3 || data.ApprovalStatus.DraftCount != 1 || data.ApprovalStatus.ApprovedCount != 1 || data.ApprovalStatus.PostedCount != 1 {
		t.Fatalf("approval summary wrong: %+v", data.ApprovalStatus)
	}
	if len(data.Items) != 3 || data.Items[0].PostingStatus == "" {
		t.Fatalf("items must carry per-entry posting_status: %+v", data.Items)
	}
	if data.Page != 1 || data.PageSize != 50 {
		t.Fatalf("paging defaults wrong: page=%d size=%d", data.Page, data.PageSize)
	}
	if data.SideEffects {
		t.Fatal("read tool must declare zero side effects")
	}
}

func TestMonthlyClosingBatchesEchoPeriodLock(t *testing.T) {
	reader := newFakeMCReader()
	reader.batches = []*repository.MonthlyClosingBatch{
		{ID: "b1", AccountingPeriod: "2026-06", Status: "completed"},
		{ID: "b2", AccountingPeriod: "2026-05", Status: "failed"},
	}
	reader.locked["2026-06"] = true

	def := NewMonthlyClosingBatchesDefinition(reader)
	result, err := def.Handler(mcCtx(), mcCall(def.Descriptor.Name, `{"period":"2026-06"}`))
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(MonthlyClosingBatchesToolData)
	if !ok {
		t.Fatalf("data type %T", result.Data)
	}
	if data.Total != 1 || data.Batches[0].ID != "b1" {
		t.Fatalf("period filter wrong: %+v", data)
	}
	if data.PeriodLocked == nil || !*data.PeriodLocked {
		t.Fatalf("period_locked must be echoed when period given: %+v", data.PeriodLocked)
	}

	// 无 period 过滤：不回显锁定标记（不逐期查询），全部批次返回。
	result, err = def.Handler(mcCtx(), mcCall(def.Descriptor.Name, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	data = result.Data.(MonthlyClosingBatchesToolData)
	if data.Total != 2 || data.PeriodLocked != nil {
		t.Fatalf("unfiltered batches wrong: total=%d locked=%v", data.Total, data.PeriodLocked)
	}
}

func TestMonthlyClosingPeriodsAndLockStatus(t *testing.T) {
	reader := newFakeMCReader()
	reader.periods = []repository.JournalEntryPeriod{
		{AccountingPeriod: "2026-06", EntryCount: 3, IsLocked: true},
		{AccountingPeriod: "2026-05", EntryCount: 2, IsLocked: false},
	}
	reader.locked["2026-06"] = true

	periodsDef := NewMonthlyClosingPeriodsDefinition(reader)
	result, err := periodsDef.Handler(mcCtx(), mcCall(periodsDef.Descriptor.Name, `{"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	pd := result.Data.(MonthlyClosingPeriodsToolData)
	if pd.Total != 1 || pd.Periods[0].AccountingPeriod != "2026-06" || !pd.Periods[0].IsLocked {
		t.Fatalf("periods wrong: %+v", pd)
	}

	lockDef := NewMonthlyClosingLockStatusDefinition(reader)
	result, err = lockDef.Handler(mcCtx(), mcCall(lockDef.Descriptor.Name, `{"period":"2026-06"}`))
	if err != nil {
		t.Fatal(err)
	}
	ld := result.Data.(MonthlyClosingLockStatusToolData)
	if ld.Period != "2026-06" || !ld.IsLocked || ld.SideEffects {
		t.Fatalf("locked period wrong: %+v", ld)
	}

	result, err = lockDef.Handler(mcCtx(), mcCall(lockDef.Descriptor.Name, `{"period":"2025-12"}`))
	if err != nil {
		t.Fatal(err)
	}
	ld = result.Data.(MonthlyClosingLockStatusToolData)
	if ld.IsLocked {
		t.Fatalf("unlocked period must read false: %+v", ld)
	}

	// 缺 period：工具返回 rejected result（非 Go error）。
	result, err = lockDef.Handler(mcCtx(), mcCall(lockDef.Descriptor.Name, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("missing period must be invalid arguments, got %+v", result.Error)
	}
}
