// Package renewaldecision turns an expiry reminder into a comparable business
// decision. It deliberately returns a scenario result and never mutates a
// contract or creates an accounting event.
package renewaldecision

import (
	"fmt"
	"math"
	"time"

	"github.com/lease-management-system/core-service/internal/services/leasescenario"
)

type Scenario struct {
	Name                    string  `json:"name"`
	Decision                string  `json:"decision"`
	TermMonths              int     `json:"term_months"`
	MonthlyRent             float64 `json:"monthly_rent"`
	RentFreeMonths          int     `json:"rent_free_months"`
	AnnualEscalationPercent float64 `json:"annual_escalation_percent"`
	OtherMonthlyCost        float64 `json:"other_monthly_cost"`
	EarlyExitPenaltyMonths  float64 `json:"early_exit_penalty_months"`
}

type Input struct {
	DecisionDate        time.Time
	Currency            string
	DiscountRate        float64
	CurrentMonthlyRent  float64
	RemainingCommitment float64
	CurrentLiability    float64
	CurrentROU          float64
	RemainingTermMonths int
	Scenarios           []Scenario
}

type YearlyImpact struct {
	Year             int     `json:"year"`
	CashOutflow      float64 `json:"cash_outflow"`
	IFRS16Expense    float64 `json:"ifrs16_expense"`
	EBITDAImpact     float64 `json:"ebitda_impact"`
	EBITImpact       float64 `json:"ebit_impact"`
	NetProfitImpact  float64 `json:"net_profit_impact"`
	ClosingLiability float64 `json:"closing_liability"`
	ClosingROU       float64 `json:"closing_rou"`
}

type ExitImpact struct {
	Year                int     `json:"year,omitempty"`
	RemainingCommitment float64 `json:"remaining_commitment"`
	LiabilityReleased   float64 `json:"liability_released"`
	ROUWrittenOff       float64 `json:"rou_written_off"`
	Penalty             float64 `json:"penalty"`
	PnLImpact           float64 `json:"pnl_impact"`
	TotalCashToExit     float64 `json:"total_cash_to_exit"`
}

type ScenarioResult struct {
	Name                    string         `json:"name"`
	Decision                string         `json:"decision"`
	AssumptionSource        string         `json:"assumption_source"`
	TermMonths              int            `json:"term_months"`
	MonthlyRent             float64        `json:"monthly_rent"`
	RentFreeMonths          int            `json:"rent_free_months"`
	AnnualEscalationPercent float64        `json:"annual_escalation_percent"`
	OtherMonthlyCost        float64        `json:"other_monthly_cost"`
	EarlyExitPenaltyMonths  float64        `json:"early_exit_penalty_months"`
	TotalCashOutflow        float64        `json:"total_cash_outflow"`
	TotalIFRS16Expense      float64        `json:"total_ifrs16_expense"`
	Yearly                  []YearlyImpact `json:"yearly"`
	Exit                    *ExitImpact    `json:"exit,omitempty"`
	ExitCurve               []ExitImpact   `json:"exit_curve,omitempty"`
}

type Result struct {
	DecisionDate time.Time        `json:"decision_date"`
	Currency     string           `json:"currency"`
	DiscountRate float64          `json:"discount_rate"`
	Scenarios    []ScenarioResult `json:"scenarios"`
}

func Evaluate(input Input) (Result, error) {
	if input.DecisionDate.IsZero() {
		return Result{}, fmt.Errorf("请提供决策日期")
	}
	if input.DiscountRate <= 0 {
		return Result{}, fmt.Errorf("请提供已确认折现率：续租现值、使用权资产和负债影响不能猜测")
	}
	if len(input.Scenarios) < 2 {
		return Result{}, fmt.Errorf("至少需要两种决策情景")
	}
	result := Result{DecisionDate: input.DecisionDate, Currency: input.Currency, DiscountRate: input.DiscountRate}
	for _, scenario := range input.Scenarios {
		if scenario.Decision == "" {
			scenario.Decision = "renew"
		}
		if scenario.Decision != "renew" && scenario.Decision != "renegotiate" && scenario.Decision != "terminate" {
			return Result{}, fmt.Errorf("情景 %q 的 decision 只能是 renew、renegotiate 或 terminate", scenario.Name)
		}
		if scenario.MonthlyRent < 0 || scenario.OtherMonthlyCost < 0 {
			return Result{}, fmt.Errorf("情景 %q 的租金和其他成本不能为负数", scenario.Name)
		}
		if scenario.Decision == "terminate" {
			result.Scenarios = append(result.Scenarios, evaluateTermination(input, scenario))
			continue
		}
		if scenario.TermMonths <= 0 {
			return Result{}, fmt.Errorf("情景 %q 的租期月数必须大于零", scenario.Name)
		}
		briefing, err := leasescenario.Build(leasescenario.Draft{
			Name: scenario.Name, CommencementDate: input.DecisionDate,
			TermMonths: scenario.TermMonths, MonthlyRent: scenario.MonthlyRent,
			RentFreeMonths:          scenario.RentFreeMonths,
			AnnualEscalationPercent: scenario.AnnualEscalationPercent,
			DiscountRate:            input.DiscountRate, Currency: input.Currency,
		})
		if err != nil {
			return Result{}, fmt.Errorf("情景 %q 测算失败：%w", scenario.Name, err)
		}
		item := ScenarioResult{
			Name: scenario.Name, Decision: scenario.Decision, AssumptionSource: "scenario_assumption",
			TermMonths: scenario.TermMonths, MonthlyRent: round2(scenario.MonthlyRent),
			RentFreeMonths: scenario.RentFreeMonths, AnnualEscalationPercent: scenario.AnnualEscalationPercent,
			OtherMonthlyCost: round2(scenario.OtherMonthlyCost), EarlyExitPenaltyMonths: scenario.EarlyExitPenaltyMonths,
			Yearly: make([]YearlyImpact, 0, len(briefing.Yearly)),
		}
		for index, yearly := range briefing.Yearly {
			otherCost := scenario.OtherMonthlyCost * float64(monthsInYear(scenario.TermMonths, index))
			bridge := briefing.Bridge[index]
			item.Yearly = append(item.Yearly, YearlyImpact{
				Year:             yearly.Year,
				CashOutflow:      round2(yearly.CashRent + otherCost),
				IFRS16Expense:    yearly.IFRS16Expense,
				EBITDAImpact:     round2(bridge.EBITDAUplift),
				EBITImpact:       round2(bridge.RentAboveEBITDA - bridge.DepreciationBelowEBITDA),
				NetProfitImpact:  round2(-yearly.ExpenseVsStraightLine),
				ClosingLiability: yearly.ClosingLiability,
				ClosingROU:       yearly.ClosingROU,
			})
			item.TotalCashOutflow += yearly.CashRent + otherCost
			item.TotalIFRS16Expense += yearly.IFRS16Expense
		}
		item.TotalCashOutflow = round2(item.TotalCashOutflow)
		item.TotalIFRS16Expense = round2(item.TotalIFRS16Expense)
		result.Scenarios = append(result.Scenarios, item)
	}
	return result, nil
}

func evaluateTermination(input Input, scenario Scenario) ScenarioResult {
	rentAtExit := input.CurrentMonthlyRent
	penalty := scenario.EarlyExitPenaltyMonths * rentAtExit
	derecognition := input.CurrentROU - input.CurrentLiability
	exit := &ExitImpact{
		RemainingCommitment: round2(input.RemainingCommitment),
		LiabilityReleased:   round2(input.CurrentLiability),
		ROUWrittenOff:       round2(input.CurrentROU),
		Penalty:             round2(penalty),
		PnLImpact:           round2(derecognition + penalty),
		TotalCashToExit:     round2(penalty),
	}
	return ScenarioResult{
		Name: scenario.Name, Decision: "terminate", AssumptionSource: "scenario_assumption",
		TermMonths: 0, MonthlyRent: round2(input.CurrentMonthlyRent), EarlyExitPenaltyMonths: scenario.EarlyExitPenaltyMonths,
		TotalCashOutflow:   exit.TotalCashToExit,
		TotalIFRS16Expense: exit.PnLImpact,
		Yearly:             []YearlyImpact{{Year: 0, CashOutflow: exit.TotalCashToExit, NetProfitImpact: round2(-exit.PnLImpact)}},
		Exit:               exit,
		ExitCurve:          buildExitCurve(input, scenario),
	}
}

func buildExitCurve(input Input, scenario Scenario) []ExitImpact {
	termMonths := input.RemainingTermMonths
	if termMonths <= 0 {
		return []ExitImpact{{Year: 0, RemainingCommitment: round2(input.RemainingCommitment), LiabilityReleased: round2(input.CurrentLiability), ROUWrittenOff: round2(input.CurrentROU), Penalty: round2(scenario.EarlyExitPenaltyMonths * input.CurrentMonthlyRent), PnLImpact: round2(input.CurrentROU - input.CurrentLiability + scenario.EarlyExitPenaltyMonths*input.CurrentMonthlyRent), TotalCashToExit: round2(scenario.EarlyExitPenaltyMonths * input.CurrentMonthlyRent)}}
	}
	years := int(math.Ceil(float64(termMonths) / 12))
	curve := make([]ExitImpact, 0, years)
	for year := 1; year <= years; year++ {
		elapsed := year * 12
		if elapsed > termMonths {
			elapsed = termMonths
		}
		remainingFraction := float64(termMonths-elapsed) / float64(termMonths)
		if remainingFraction < 0 {
			remainingFraction = 0
		}
		remainingCommitment := input.RemainingCommitment * remainingFraction
		liabilityReleased := input.CurrentLiability * remainingFraction
		rouWrittenOff := input.CurrentROU * remainingFraction
		penalty := scenario.EarlyExitPenaltyMonths * input.CurrentMonthlyRent
		pnlImpact := rouWrittenOff - liabilityReleased + penalty
		curve = append(curve, ExitImpact{
			Year: year, RemainingCommitment: round2(remainingCommitment), LiabilityReleased: round2(liabilityReleased),
			ROUWrittenOff: round2(rouWrittenOff), Penalty: round2(penalty), PnLImpact: round2(pnlImpact),
			TotalCashToExit: round2(penalty),
		})
	}
	return curve
}

func monthsInYear(termMonths, yearIndex int) int {
	remaining := termMonths - yearIndex*12
	if remaining <= 0 {
		return 0
	}
	if remaining > 12 {
		return 12
	}
	return remaining
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
