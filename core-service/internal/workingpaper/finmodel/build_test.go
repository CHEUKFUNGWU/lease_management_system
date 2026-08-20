package finmodel

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func pf(v float64) *float64 { return &v }

func sampleInput() Input {
	return Input{
		Title: "三表模型运行底稿", LegalEntityID: "LE-1", Currency: "CNY",
		DataClassification:     "production",
		ModelDefinitionVersion: "v3", TemplateVersion: "v1",
		DataVersion: "ds-1", AssumptionVersion: "as-1", ExchangeRateVersion: "fx-1",
		MetricDefinitionVersion: "md-1",
		Periods:                 []string{"2026-01", "2026-02"},
		Lines: []LineValue{
			{RowKey: "rev", Label: "营业收入", Period: "2026-01", Value: pf(1000), SourceType: "fact_aggregate", Classification: "production"},
			{RowKey: "net_income", Label: "净利润", Period: "2026-01", Value: pf(7.5), SourceType: "formula", Classification: "production"},
			// Missing stays missing: nil never becomes a zero cell.
			{RowKey: "labor", Label: "人工成本", Period: "2026-01", Value: nil, SourceType: "fact_aggregate", Classification: "production"},
			{RowKey: "rou_asset", Label: "使用权资产", Period: "2026-01", Value: pf(1150), SourceType: "ifrs16_engine", Classification: "production"},
		},
		TieOuts: []TieOutValue{
			{CheckCode: "T1", Period: "2026-01", Expected: pf(2160), Actual: pf(2160), Diff: pf(0), Status: "passed"},
			{CheckCode: "T7", Period: "2026-01", Expected: pf(910), Actual: pf(910), Diff: pf(0), Status: "passed"},
		},
		GapDetails:  []string{"预测期收入以 SSSG 驱动"},
		ToolCallID:  "call-finmodel-1",
		GeneratedBy: "fpna.working_paper.finmodel.generate",
	}
}

// 保值断言：单元格值与输入 LineValue 一一对应，构建器不重算。
func TestBuildPreservesRunValuesOneToOne(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	byRef := map[string]workingpaper.Cell{}
	for _, c := range paper.AllCells() {
		byRef[c.Ref] = c
	}
	if got := byRef["rev@2026-01"].Value.(float64); got != 1000 {
		t.Fatalf("rev cell = %v, run says 1000", got)
	}
	if got := byRef["net_income@2026-01"].Value.(float64); got != 7.5 {
		t.Fatalf("net_income cell = %v, run says 7.5", got)
	}
	if _, exists := byRef["labor@2026-01"]; exists {
		t.Fatal("nil lines must be skipped, never zero-filled")
	}
	// basis mapping: facts → SystemFact, engine/formula → Certified.
	if byRef["rev@2026-01"].Provenance.Basis != workingpaper.BasisSystemFact {
		t.Fatalf("fact cells must be SystemFact, got %s", byRef["rev@2026-01"].Provenance.Basis)
	}
	if byRef["net_income@2026-01"].Provenance.Basis != workingpaper.BasisCertified {
		t.Fatalf("formula cells must be Certified, got %s", byRef["net_income@2026-01"].Provenance.Basis)
	}
}

func TestBuildPaperPassesLint(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	built := workingpaper.Build(paper, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(built, auditSet{"call-finmodel-1": true})
	if !rep.OK {
		t.Fatalf("finmodel paper must pass lint, got %+v", rep.Violations)
	}
	if refs := built.ExploratoryRefs(); len(refs) != 0 {
		t.Fatalf("model papers carry no exploratory cells (金额全部来自确定性引擎), got %v", refs)
	}
}

func TestBuildFlagsFailedTieOuts(t *testing.T) {
	in := sampleInput()
	in.TieOuts = append(in.TieOuts, TieOutValue{CheckCode: "T1", Period: "2026-02", Diff: pf(7), Status: "failed"})
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sec := range paper.Sections {
		if sec.ID == "tie_outs" && sec.Narrative != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("failed tie-outs must be flagged in the check section narrative")
	}
}

func TestBuildSimulationFlagEntersGaps(t *testing.T) {
	in := sampleInput()
	in.DataClassification = "simulated"
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, g := range paper.DataGaps {
		joined += g + "|"
	}
	if len(joined) == 0 || !contains(joined, "SIMULATED") {
		t.Fatalf("simulation flag must enter the gaps, got %v", paper.DataGaps)
	}
}

type auditSet map[string]bool

func (a auditSet) CompletedToolCall(id string) bool { return a[id] }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCoverCarriesAllFiveVersionLines(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if paper.DataVersion != "ds-1" || paper.AssumptionVersion != "as-1" ||
		paper.ExchangeRateVersion != "fx-1" || paper.MetricDefinitionVersion != "md-1" ||
		paper.TemplateVersion != "v1" || paper.EngineVersion != "finmodel@v3" {
		t.Fatalf("cover must carry all five version lines + engine version, got %+v", paper)
	}
}

// P1-3 (D-S5 第二处 fail-closed): 勾稽失败的 run 构建的底稿必须被 lint 拒绝
// — 底稿可看（诊断），但永远不能导出为 artifact。
func TestLintRejectsFailedTieOutRun(t *testing.T) {
	in := sampleInput()
	in.TieOuts = append(in.TieOuts, TieOutValue{CheckCode: "T13", Period: "2026-02", Expected: pf(200), Actual: pf(233), Diff: pf(33), Status: "failed"})
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	built := workingpaper.Build(paper, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(built, auditSet{"call-finmodel-1": true})
	if rep.OK {
		t.Fatal("a failed-tie-out run's paper must be refused by lint (no artifact export)")
	}
	found := false
	for _, v := range rep.Violations {
		if v.Code == "tie_out_unpassed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("lint must name the tie_out_unpassed violation, got %+v", rep.Violations)
	}

	// 对照：全绿的同一底稿通过 lint。
	paperOK, _ := Build(sampleInput())
	repOK := workingpaper.Lint(workingpaper.Build(paperOK, time.Time{}), auditSet{"call-finmodel-1": true})
	if !repOK.OK {
		t.Fatalf("an all-green run's paper must pass lint, got %+v", repOK.Violations)
	}
}

// P1-4: 勾稽区同金额行的 nil 跳格纪律——Diff 缺失（not_applicable / 未判）
// 不留数值单元格，绝不用 0.0 填。
func TestBuildSkipsNilTieOutDiffs(t *testing.T) {
	in := sampleInput()
	in.TieOuts = append(in.TieOuts,
		TieOutValue{CheckCode: "T9", Period: "2026-02", Status: "not_applicable", Diff: nil},
		TieOutValue{CheckCode: "T14", Period: "2026-02", Status: "not_applicable", Diff: nil},
	)
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range paper.AllCells() {
		if c.Ref == "T9@2026-02" || c.Ref == "T14@2026-02" {
			t.Fatalf("nil Diff must not produce a numeric cell (no 0.0), got %+v", c)
		}
	}
	// 有 Diff 的勾稽行仍照常入底稿。
	found := false
	for _, c := range paper.AllCells() {
		if c.Ref == "T1@2026-01" {
			found = true
		}
	}
	if !found {
		t.Fatal("tie-outs with a Diff must still render as cells")
	}
}
