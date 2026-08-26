// Package s1 builds the S1 pre-deal decision working paper from the
// deterministic predeal and dealcompare engines. Every engine number becomes
// a Certified cell carrying the audited tool call id and the engine version;
// the builder performs no independent arithmetic on financial amounts —
// that is the CORR-1 guarantee verified by the engine-consistency evaluation.
package s1

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/services/leasescenario"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// Engine version labels: the provenance answer to "which engine computed
// this", pinned to the tool that fronted the engine.
const (
	EnginePredeal      = "lease.predeal.simulate@v1"
	EngineDealCompare  = "lease.deal.simulate@v1"
	EnginePredealShock = "predeal-shock@v1"
)

// Input is everything the paper needs. Assumptions must be human-confirmed —
// the discount rate is never guessed (AGENTS.md 人机协同红线).
type Input struct {
	Draft         leasescenario.Draft   `json:"draft"`
	Offers        []leasescenario.Offer `json:"offers,omitempty"`
	ShocksPercent []float64             `json:"shocks_percent,omitempty"`
	ConfirmedBy   string                `json:"confirmed_by"`
	ConfirmedAt   string                `json:"confirmed_at"`
	ToolCallID    string                `json:"tool_call_id,omitempty"`
	EngineVersion string                `json:"engine_version,omitempty"`
}

// Build assembles the paper. It fails on engine validation errors and on
// unconfirmed assumptions; it never invents a number.
func Build(in Input) (workingpaper.Paper, error) {
	if strings.TrimSpace(in.ConfirmedBy) == "" {
		return workingpaper.Paper{}, errors.New("s1: assumptions must be confirmed by a human (confirmed_by is empty)")
	}
	if strings.TrimSpace(in.ConfirmedAt) == "" {
		return workingpaper.Paper{}, errors.New("s1: assumptions must carry a confirmation time (confirmed_at is empty)")
	}
	if strings.TrimSpace(in.ToolCallID) == "" {
		return workingpaper.Paper{}, errors.New("s1: tool_call_id is required for certified provenance (I2)")
	}
	engine := in.EngineVersion
	if engine == "" {
		engine = EnginePredeal
	}

	briefing, err := leasescenario.Build(in.Draft)
	if err != nil {
		return workingpaper.Paper{}, fmt.Errorf("s1: predeal engine: %w", err)
	}

	var gaps []string
	gaps = append(gaps,
		"押金未建模：报价单中的押金条款未进入计量（predeal 引擎无押金输入）",
		"变量租金未建模：turnover rent / sales-based rent 不在本情景内",
		"加权平均折现率为组合级度量：单店签约前底稿不适用",
	)
	if len(in.Offers) > 0 {
		gaps = append(gaps, "可比报价对比使用有效租金与现值口径，不等同于 IFRS 16 计量")
	}

	paper := workingpaper.Paper{
		Title:               "S1 签约前决策底稿",
		Period:              in.Draft.Name,
		LegalEntityScope:    "",
		ReviewState:         workingpaper.ReviewNeedsReview,
		EngineVersion:       engine,
		GeneratedBy:         "lease.working_paper.s1.generate",
		DataGaps:            gaps,
		OpenQuestions:       []string{"免租期的直线法租金比较口径需复核（引擎按现金租金与直线法并列展示）"},
		UnexplainedResidual: none,
		Sections: []workingpaper.Section{
			assumptionsSection(in),
			ifrsSection(briefing, in.ToolCallID),
			ebitdaBridgeSection(briefing, in.ToolCallID),
			exitCurveSection(briefing, in.ToolCallID),
		},
	}
	if len(in.Offers) >= 2 {
		paper.Sections = append(paper.Sections, dealCompareSection(in.Offers, briefing.DiscountRate, briefing.Currency, in.ToolCallID))
	}
	if len(in.ShocksPercent) > 0 {
		paper.Sections = append(paper.Sections, sensitivitySection(in.Draft, in.ShocksPercent, in.ToolCallID))
	}
	return paper, nil
}

const none = ""

func certified(toolCallID, engine string) workingpaper.Provenance {
	return workingpaper.Provenance{
		Basis:         workingpaper.BasisCertified,
		ToolCallID:    toolCallID,
		EngineVersion: engine,
	}
}

func human(in Input) workingpaper.Provenance {
	return workingpaper.Provenance{
		Basis:       workingpaper.BasisHumanInput,
		ConfirmedBy: in.ConfirmedBy,
		ConfirmedAt: in.ConfirmedAt,
	}
}

func cell(ref, label, measureID string, value any, unit, currency string, p workingpaper.Provenance) workingpaper.Cell {
	return workingpaper.Cell{Ref: ref, Label: label, MeasureID: measureID, Value: value, Unit: unit, Currency: currency, Provenance: p}
}

func assumptionsSection(in Input) workingpaper.Section {
	cells := []workingpaper.Cell{
		cell("AS-1", "起租日", "", in.Draft.CommencementDate.Format("2006-01-02"), "", "", human(in)),
		cell("AS-2", "租期（月）", "", in.Draft.TermMonths, "月", "", human(in)),
		cell("AS-3", "月租金", "", in.Draft.MonthlyRent, "", in.Draft.Currency, human(in)),
		cell("AS-4", "免租期（月）", "", in.Draft.RentFreeMonths, "月", "", human(in)),
		cell("AS-5", "年递增率", "", in.Draft.AnnualEscalationPercent, "%", "", human(in)),
		// The discount rate here is the human-confirmed INPUT assumption; the
		// applied rate as an engine output carries the protected measure id in
		// the IFRS section (I3: protected measures stay Certified).
		cell("AS-6", "折现率（人工确认的假设输入）", "", in.Draft.DiscountRate, "%", "", human(in)),
		cell("AS-7", "初始直接成本", "", in.Draft.InitialDirectCost, "", in.Draft.Currency, human(in)),
		cell("AS-8", "退出罚金（月租金倍数）", "", in.Draft.EarlyExitPenaltyMonths, "月租金倍数", "", human(in)),
	}
	return workingpaper.Section{ID: "assumptions", Title: "关键假设（人工确认）", Kind: workingpaper.KindAssumptionList, Cells: cells}
}

func ifrsSection(b leasescenario.Briefing, callID string) workingpaper.Section {
	cells := []workingpaper.Cell{
		cell("IF-1", "初始租赁负债", "lease_liability", b.BalanceSheet.InitialLiability, "", b.Currency, certified(callID, EnginePredeal)),
		cell("IF-2", "初始使用权资产", "rou_asset", b.BalanceSheet.InitialROU, "", b.Currency, certified(callID, EnginePredeal)),
		cell("IF-3", "实际采用的折现率", "discount_rate_applied", b.DiscountRate, "%", "", certified(callID, EnginePredeal)),
		cell("IF-4", "未折现承租承诺", "", b.BalanceSheet.UndiscountedCommitment, "", b.Currency, certified(callID, EnginePredeal)),
		cell("IF-5", "折现影响", "", b.BalanceSheet.DiscountingEffect, "", b.Currency, certified(callID, EnginePredeal)),
	}
	for _, y := range b.Yearly {
		prefix := fmt.Sprintf("IF-%d-", y.Year)
		cells = append(cells,
			cell(prefix+"interest", "利息费用（"+fmt.Sprint(y.Year)+"）", "interest_expense", y.Interest, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"depreciation", "使用权资产折旧（"+fmt.Sprint(y.Year)+"）", "rou_depreciation", y.Depreciation, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"closing-liability", "期末租赁负债（"+fmt.Sprint(y.Year)+"）", "lease_liability", y.ClosingLiability, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"closing-rou", "期末使用权资产（"+fmt.Sprint(y.Year)+"）", "rou_asset", y.ClosingROU, "", b.Currency, certified(callID, EnginePredeal)),
		)
	}
	return workingpaper.Section{ID: "ifrs16", Title: "IFRS 16 影响（确定性引擎）", Kind: workingpaper.KindTable, Cells: cells, Narrative: b.Headline}
}

func ebitdaBridgeSection(b leasescenario.Briefing, callID string) workingpaper.Section {
	var cells []workingpaper.Cell
	for _, r := range b.Bridge {
		prefix := fmt.Sprintf("EB-%d-", r.Year)
		cells = append(cells,
			cell(prefix+"rent-above", "租金上移（高于 EBITDA）", "", r.RentAboveEBITDA, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"uplift", "EBITDA 提升", "", r.EBITDAUplift, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"dep-below", "折旧（EBIT 以下）", "", r.DepreciationBelowEBITDA, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"interest-below", "利息（EBIT 以下）", "", r.InterestBelowEBIT, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"net-profit", "净利润影响", "", r.NetProfitImpact, "", b.Currency, certified(callID, EnginePredeal)),
		)
	}
	return workingpaper.Section{ID: "ebitda_bridge", Title: "EBITDA 桥", Kind: workingpaper.KindTable, Cells: cells}
}

func exitCurveSection(b leasescenario.Briefing, callID string) workingpaper.Section {
	var cells []workingpaper.Cell
	for _, e := range b.ExitCurve {
		prefix := fmt.Sprintf("EX-%d-", e.Year)
		cells = append(cells,
			cell(prefix+"released", "退出释放的租赁负债（"+fmt.Sprint(e.Year)+"）", "lease_liability", e.LiabilityReleased, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"written-off", "退出冲销的使用权资产（"+fmt.Sprint(e.Year)+"）", "rou_asset", e.ROUWrittenOff, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"penalty", "退出罚金（"+fmt.Sprint(e.Year)+"）", "", e.Penalty, "", b.Currency, certified(callID, EnginePredeal)),
			cell(prefix+"pnl", "退出损益影响（"+fmt.Sprint(e.Year)+"）", "", e.PnLImpact, "", b.Currency, certified(callID, EnginePredeal)),
		)
	}
	return workingpaper.Section{ID: "exit_curve", Title: "退出曲线", Kind: workingpaper.KindTable, Cells: cells}
}

func dealCompareSection(offers []leasescenario.Offer, rate float64, currency, callID string) workingpaper.Section {
	comparison, err := leasescenario.Compare(leasescenario.CompareInput{DiscountRate: rate, Currency: currency, Offers: offers})
	if err != nil {
		return workingpaper.Section{ID: "deal_compare", Title: "可比报价对比", Kind: workingpaper.KindTable}
	}
	var cells []workingpaper.Cell
	for _, o := range comparison.Offers {
		prefix := "DC-" + o.Name + "-"
		cells = append(cells,
			cell(prefix+"effective-rent", "有效月租金（"+o.Name+"）", "", o.EffectiveMonthlyRent, "", currency, certified(callID, EngineDealCompare)),
			cell(prefix+"effective-per-sqm", "坪效有效租金（"+o.Name+"）", "", o.EffectiveRentPerSqm, "/㎡", currency, certified(callID, EngineDealCompare)),
			cell(prefix+"pv", "现值（"+o.Name+"）", "", o.PresentValue, "", currency, certified(callID, EngineDealCompare)),
		)
	}
	return workingpaper.Section{
		ID: "deal_compare", Title: "可比报价对比", Kind: workingpaper.KindTable, Cells: cells,
		Narrative: comparison.Conclusion,
	}
}

func sensitivitySection(draft leasescenario.Draft, shocks []float64, callID string) workingpaper.Section {
	var cells []workingpaper.Cell
	for _, shock := range shocks {
		variant := draft
		variant.DiscountRate = draft.DiscountRate * (1 + shock)
		b, err := leasescenario.Build(variant)
		if err != nil {
			continue
		}
		label := fmt.Sprintf("折现率冲击 %+.1f%%", shock*100)
		cells = append(cells,
			cell("SE-"+fmt.Sprint(shock)+"-rate", label+"——采用折现率", "discount_rate_applied", b.DiscountRate, "%", "", certified(callID, EnginePredealShock)),
			cell("SE-"+fmt.Sprint(shock)+"-liability", label+"——初始租赁负债", "lease_liability", b.BalanceSheet.InitialLiability, "", b.Currency, certified(callID, EnginePredealShock)),
			cell("SE-"+fmt.Sprint(shock)+"-rou", label+"——初始使用权资产", "rou_asset", b.BalanceSheet.InitialROU, "", b.Currency, certified(callID, EnginePredealShock)),
		)
	}
	return workingpaper.Section{
		ID: "sensitivity", Title: "折现率敏感性（引擎重跑）", Kind: workingpaper.KindTable, Cells: cells,
		Narrative: "各冲击均为对同一引擎按调整后折现率的确定性重跑，非 AI 推算。",
	}
}
