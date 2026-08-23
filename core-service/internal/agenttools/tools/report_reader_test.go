package tools

// B-4 报表工具单元测试：诚实降级、口径词表映射（approved/pending/draft →
// 投影模式）、叠加层只在 draft 口径出现、各 kind 参数校验、折现率缺失的
// 具名降级。跨法人隔离见带库集成测试。

import (
	"context"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

type fakeReportReader struct {
	modes       []reporting.Mode
	requests    []reporting.ProjectionRequest
	plans       []*repository.FPnAPlanVersion
	assumptions []*repository.FPnAAssumptionVersion
	drafts      []*repository.FPnAScenarioDraft
	projectErr  error
}

func (f *fakeReportReader) Project(_ context.Context, mode reporting.Mode, request reporting.ProjectionRequest) (reporting.ProjectionResult, error) {
	f.modes = append(f.modes, mode)
	f.requests = append(f.requests, request)
	if f.projectErr != nil {
		return reporting.ProjectionResult{}, f.projectErr
	}
	return reporting.ProjectionResult{Payload: map[string]any{"rows": []any{}}}, nil
}

func (f *fakeReportReader) ClosePack(context.Context, reporting.Mode, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (f *fakeReportReader) ListPlanVersions(context.Context, access.EntityFilter, string, string, string) ([]*repository.FPnAPlanVersion, error) {
	return f.plans, nil
}

func (f *fakeReportReader) ListAssumptions(context.Context, access.EntityFilter, string) ([]*repository.FPnAAssumptionVersion, error) {
	return f.assumptions, nil
}

func (f *fakeReportReader) ListScenarioDrafts(context.Context, access.EntityFilter, int) ([]*repository.FPnAScenarioDraft, error) {
	return f.drafts, nil
}

func newFakeReportReader() *fakeReportReader { return &fakeReportReader{} }

func reportDefs() map[string]agenttools.ToolDefinition {
	reader := newFakeReportReader()
	defs := []agenttools.ToolDefinition{
		NewReportScheduleDefinition(reader),
		NewReportDisclosurePackageDefinition(reader),
		NewReportContractViewDefinition(reader),
		NewReportUnitPriceDefinition(reader),
		NewReportTagsDefinition(reader),
	}
	out := map[string]agenttools.ToolDefinition{}
	for _, def := range defs {
		out[def.Descriptor.Name] = def
	}
	return out
}

func TestReportToolsWithoutReaderAreUnavailable(t *testing.T) {
	args := `{"kind":"amortization"}`
	for _, def := range []agenttools.ToolDefinition{
		NewReportScheduleDefinition(nil),
		NewReportDisclosurePackageDefinition(nil),
		NewReportContractViewDefinition(nil),
		NewReportUnitPriceDefinition(nil),
		NewReportTagsDefinition(nil),
	} {
		result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, args))
		if err != nil {
			t.Fatalf("%s: %v", def.Descriptor.Name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable || !strings.Contains(result.Error.Message, "unavailable") {
			t.Fatalf("%s: expected honest unavailable, got %+v", def.Descriptor.Name, result.Error)
		}
	}
}

func TestReportToolsRequireLegalEntity(t *testing.T) {
	noEntityCtx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "admin", Permissions: []string{"reports:read"}},
		RunID:     "run-1",
	})
	for name, def := range reportDefs() {
		_ = name
		result, err := def.Handler(noEntityCtx, factsCall(def.Descriptor.Name, `{"kind":"tags"}`))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Error == nil || result.Error.Code != agenttools.ErrorScopeDenied {
			t.Fatalf("%s: missing legal entity must be scope_denied, got %+v", name, result.Error)
		}
	}
}

func TestReportDescriptorsDeclarePermissions(t *testing.T) {
	for _, def := range reportDefs() {
		d := def.Descriptor
		if !strings.HasPrefix(d.Name, "lease.report.") {
			t.Fatalf("%s must live in the lease.* namespace", d.Name)
		}
		if len(d.Permissions) != 1 || d.Permissions[0].Resource != "reports" || d.Permissions[0].Action != "read" {
			t.Fatalf("%s permissions wrong: %+v", d.Name, d.Permissions)
		}
		if d.Level != agenttools.LevelRead || !d.ReadOnly {
			t.Fatalf("%s must be read-only", d.Name)
		}
	}
}

// 口径词表是本批的核心：approved→Official、pending→Pending、draft→Working，
// 缺省 approved。mode 是线格式 legacy 回显，is_official_version 只跟 Official。
func TestReportBasisMapsToProjectionMode(t *testing.T) {
	cases := []struct {
		basis     string
		wantMode  reporting.Mode
		wantBasis string
	}{
		{"", reporting.Official, "approved"},
		{"approved", reporting.Official, "approved"},
		{"pending", reporting.Pending, "pending"},
		{"draft", reporting.Working, "draft"},
	}
	for _, tc := range cases {
		reader := newFakeReportReader()
		def := NewReportScheduleDefinition(reader)
		raw := `{"kind":"liability_rolling"}`
		if tc.basis != "" {
			raw = `{"kind":"liability_rolling","report_basis":"` + tc.basis + `"}`
		}
		result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, raw))
		if err != nil {
			t.Fatal(err)
		}
		data := result.Data.(ReportProjectionToolData)
		if len(reader.modes) != 1 || reader.modes[0] != tc.wantMode {
			t.Fatalf("basis %q mapped to %v, want %v", tc.basis, reader.modes, tc.wantMode)
		}
		if data.ReportBasis != tc.wantBasis || data.Mode != string(tc.wantMode) || data.IsOfficialVersion != (tc.wantMode == reporting.Official) {
			t.Fatalf("envelope wrong for basis %q: %+v", tc.basis, data)
		}
	}

	// 非法口径拒绝。
	reader := newFakeReportReader()
	def := NewReportScheduleDefinition(reader)
	result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"kind":"liability_rolling","report_basis":"official"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("legacy word 'official' must be rejected as basis, got %+v", result.Error)
	}
	if len(reader.modes) != 0 {
		t.Fatal("rejected basis must not reach the reader")
	}
}

// 叠加层纪律：draft 才有 user_overlays；approved/pending 恒无
// （approved-only 读取永不回采 draft）。叠加内容经过筛选：只含 custom 计划
// 版本与 draft 假设。
func TestUserOverlaysOnlyOnDraftBasis(t *testing.T) {
	reader := newFakeReportReader()
	customPlan := &repository.FPnAPlanVersion{ID: "p-custom", ScenarioType: "custom", Status: "draft"}
	budgetPlan := &repository.FPnAPlanVersion{ID: "p-budget", ScenarioType: "budget", Status: "approved"}
	approvedAssumption := &repository.FPnAAssumptionVersion{ID: "a-approved", Status: "approved"}
	draftAssumption := &repository.FPnAAssumptionVersion{ID: "a-draft", Status: "draft"}
	scenarioDraft := &repository.FPnAScenarioDraft{ID: "s-draft", Name:客流下降情景(), Status: "draft"}
	reader.plans = []*repository.FPnAPlanVersion{customPlan, budgetPlan}
	reader.assumptions = []*repository.FPnAAssumptionVersion{approvedAssumption, draftAssumption}
	reader.drafts = []*repository.FPnAScenarioDraft{scenarioDraft}

	def := NewReportUnitPriceDefinition(reader)
	for _, tc := range []struct {
		basis      string
		wantOverlay bool
	}{
		{"approved", false},
		{"pending", false},
		{"draft", true},
	} {
		result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"report_basis":"`+tc.basis+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		data := result.Data.(ReportProjectionToolData)
		if tc.wantOverlay && data.UserOverlays == nil {
			t.Fatalf("basis %q must carry user overlays", tc.basis)
		}
		if !tc.wantOverlay && data.UserOverlays != nil {
			t.Fatalf("basis %q must never read drafts back: %+v", tc.basis, data.UserOverlays)
		}
		if !tc.wantOverlay {
			continue
		}
		plans := data.UserOverlays["custom_plan_versions"].([]*repository.FPnAPlanVersion)
		if len(plans) != 1 || plans[0].ID != "p-custom" {
			t.Fatalf("overlay plan filter wrong: %+v", plans)
		}
		assumptions := data.UserOverlays["draft_assumptions"].([]*repository.FPnAAssumptionVersion)
		if len(assumptions) != 1 || assumptions[0].ID != "a-draft" {
			t.Fatalf("overlay assumption filter wrong: %+v", assumptions)
		}
		drafts := data.UserOverlays["scenario_drafts"].([]*repository.FPnAScenarioDraft)
		if len(drafts) != 1 || drafts[0].ID != "s-draft" {
			t.Fatalf("overlay scenario drafts wrong: %+v", drafts)
		}
	}
}

func 客流下降情景() string { return "客流下降10%情景" }

func TestReportScheduleKindValidation(t *testing.T) {
	reader := newFakeReportReader()
	def := NewReportScheduleDefinition(reader)
	cases := []struct{ name, args string }{
		{"missing kind", `{}`},
		{"bad kind", `{"kind":"magic"}`},
		{"amortization missing dates", `{"kind":"amortization"}`},
		{"amortization bad granularity", `{"kind":"amortization","start_date":"2026-01-01","end_date":"2026-03-01","granularity":"week"}`},
		{"amortization bad view", `{"kind":"amortization","start_date":"2026-01-01","end_date":"2026-03-01","view":"planet"}`},
		{"cashflow bad granularity", `{"kind":"cashflow_forecast","start_date":"2026-01-01","end_date":"2026-03-01","granularity":"day"}`},
		{"cashflow missing dates", `{"kind":"cashflow_forecast"}`},
		{"unknown field", `{"kind":"liability_rolling","surprise":1}`},
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
	// liability_rolling 不需要日期区间。
	result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"kind":"liability_rolling"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("liability_rolling should complete without dates, got %+v", result.Error)
	}
}

func TestReportOtherKindsValidation(t *testing.T) {
	defs := reportDefs()
	closePack := defs["lease.report.disclosure_package.read"]
	contractView := defs["lease.report.contract_view.read"]
	unitPrice := defs["lease.report.unit_price.read"]
	tags := defs["lease.report.tags.read"]

	cases := []struct {
		name string
		def  agenttools.ToolDefinition
		args string
	}{
		{"close_pack missing period", closePack, `{"kind":"close_pack"}`},
		{"close_pack bad period", closePack, `{"kind":"close_pack","period":"2026-6"}`},
		{"disclosure bad period_end", closePack, `{"kind":"disclosure","period_start":"2026-01-01","period_end":"yesterday"}`},
		{"disclosure reversed period", closePack, `{"kind":"disclosure","period_start":"2026-12-31","period_end":"2026-01-01"}`},
		{"disclosure bad kind", closePack, `{"kind":"everything"}`},
		{"standard_comparison missing contract", contractView, `{"kind":"standard_comparison"}`},
		{"contract_view bad kind", contractView, `{"kind":"portfolio"}`},
		{"unit_price bad group_by", unitPrice, `{"group_by":"galaxy"}`},
		{"tags bad kind", tags, `{"kind":"all"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.def.Handler(factsCtx(), factsCall(tc.def.Descriptor.Name, tc.args))
			if err != nil {
				t.Fatal(err)
			}
			if result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
				t.Fatalf("expected invalid arguments, got %+v", result.Error)
			}
		})
	}
}

// 折现率缺失是具名 Gap：快照拒绝以假设利率计量，工具把它映射为 unavailable
// 加 discount_rate_missing 措辞，绝不静默换一个默认利率。
func TestReportDiscountRateMissingIsNamedGap(t *testing.T) {
	reader := &fakeReportReader{projectErr: &reporting.DiscountRateMissingError{ContractNumbers: []string{"HT-001"}}}
	def := NewReportScheduleDefinition(reader)
	result, err := def.Handler(factsCtx(), factsCall(def.Descriptor.Name, `{"kind":"liability_rolling"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil || result.Error.Code != agenttools.ErrorDataUnavailable {
		t.Fatalf("discount rate missing must degrade to unavailable, got %+v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "discount_rate_missing") {
		t.Fatalf("named gap wording lost: %q", result.Error.Message)
	}
}
