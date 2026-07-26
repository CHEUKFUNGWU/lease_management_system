// Package predeal answers the questions finance is asked before a lease is
// signed, rather than the one it is asked after: not "how is this booked" but
// "what does this decision do to us for the next five years".
//
// Every figure here comes from the measurement engine that already exists. What
// was missing was the framing — three views in particular, each of which
// answers a question a spreadsheet answers badly:
//
//   - The expense curve. IFRS 16 front-loads cost: interest is highest when the
//     liability is largest, so the accounting charge starts above the cash rent
//     and ends below it. A budget built on "rent is 100k a month" is wrong in
//     year one in a direction nobody expects.
//   - The EBITDA bridge. Capitalising a lease moves rent below the EBITDA line.
//     EBITDA rises without anything about the business improving, and somebody
//     has to explain that to a board. Stating the lift explicitly is the whole
//     point of the view.
//   - The exit cost curve. "What does it cost to walk away in year N" is the
//     question asked when a strategy changes, and it is the one nobody has
//     ready. The termination treatment already knows.
package predeal

import (
	"fmt"
	"math"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// Draft is a set of terms under consideration. It is not a contract and is
// never stored: the point is to ask before committing.
type Draft struct {
	Name             string    `json:"name"`
	CommencementDate time.Time `json:"commencement_date"`
	TermMonths       int       `json:"term_months"`
	MonthlyRent      float64   `json:"monthly_rent"`
	RentFreeMonths   int       `json:"rent_free_months"`
	// AnnualEscalationPercent raises the rent on each anniversary.
	AnnualEscalationPercent float64 `json:"annual_escalation_percent"`
	DiscountRate            float64 `json:"discount_rate"`
	Currency                string  `json:"currency"`

	// InitialDirectCost is capitalised into the asset, so it lifts depreciation
	// without touching the liability.
	InitialDirectCost float64 `json:"initial_direct_cost"`

	// EarlyExitPenaltyMonths is the rent payable as a penalty on walking away,
	// expressed in months of the then-current rent. Break clauses are almost
	// always written that way.
	EarlyExitPenaltyMonths float64 `json:"early_exit_penalty_months"`
}

// YearlyImpact is one financial year of the decision.
type YearlyImpact struct {
	Year int `json:"year"`

	// CashRent is what leaves the bank.
	CashRent float64 `json:"cash_rent"`
	// StraightLineRent is what the charge would have been without IFRS 16 —
	// the comparison a budget was probably built on.
	StraightLineRent float64 `json:"straight_line_rent"`

	Interest     float64 `json:"interest"`
	Depreciation float64 `json:"depreciation"`
	// IFRS16Expense is interest plus depreciation: the actual charge.
	IFRS16Expense float64 `json:"ifrs16_expense"`
	// ExpenseVsStraightLine is positive in the years the standard costs more
	// than a straight-line rent would have.
	ExpenseVsStraightLine float64 `json:"expense_vs_straight_line"`

	ClosingLiability float64 `json:"closing_liability"`
	ClosingROU       float64 `json:"closing_rou"`
}

// EBITDABridge shows where the charge lands in the income statement.
type EBITDABridge struct {
	Year int `json:"year"`

	// RentAboveEBITDA is what an uncapitalised lease would have charged as
	// operating expense, above the EBITDA line.
	RentAboveEBITDA float64 `json:"rent_above_ebitda"`
	// EBITDAUplift is how much EBITDA rises purely because that rent moved
	// below the line. Nothing about the business changed.
	EBITDAUplift float64 `json:"ebitda_uplift"`
	// DepreciationBelowEBITDA and InterestBelowEBIT are where it went.
	DepreciationBelowEBITDA float64 `json:"depreciation_below_ebitda"`
	InterestBelowEBIT       float64 `json:"interest_below_ebit"`
	// NetProfitImpact is the difference the standard makes at the bottom line:
	// the charge under IFRS 16 less the rent that would otherwise have been
	// expensed. It nets to zero over the whole term and is only ever a timing
	// difference — which is exactly what a board needs told.
	NetProfitImpact float64 `json:"net_profit_impact"`
}

// ExitPoint is what it costs to walk away at the end of a given year.
type ExitPoint struct {
	Year int `json:"year"`

	// RemainingCommitment is the undiscounted rent still owed.
	RemainingCommitment float64 `json:"remaining_commitment"`
	// LiabilityReleased is the carrying liability that would be derecognised.
	LiabilityReleased float64 `json:"liability_released"`
	// ROUWrittenOff is the asset that would go with it.
	ROUWrittenOff float64 `json:"rou_written_off"`
	// Penalty is the break fee, in money.
	Penalty float64 `json:"penalty"`
	// PnLImpact is the gain or loss on derecognition plus the penalty.
	// Positive is a cost.
	PnLImpact float64 `json:"pnl_impact"`
	// TotalCashToExit is the penalty — the remaining rent is avoided, which is
	// the point of exiting, so it is reported separately rather than added.
	TotalCashToExit float64 `json:"total_cash_to_exit"`
}

// BalanceSheetImpact is the "what lands on the balance sheet" summary.
type BalanceSheetImpact struct {
	InitialLiability float64 `json:"initial_liability"`
	InitialROU       float64 `json:"initial_rou"`
	// UndiscountedCommitment is the headline number a reader compares the
	// liability against, and the gap between them is the discounting.
	UndiscountedCommitment float64 `json:"undiscounted_commitment"`
	DiscountingEffect      float64 `json:"discounting_effect"`
}

// Briefing is the one page the workbench produces.
type Briefing struct {
	Name         string  `json:"name"`
	Currency     string  `json:"currency"`
	DiscountRate float64 `json:"discount_rate"`
	TermMonths   int     `json:"term_months"`

	BalanceSheet BalanceSheetImpact `json:"balance_sheet"`
	Yearly       []YearlyImpact     `json:"yearly"`
	Bridge       []EBITDABridge     `json:"ebitda_bridge"`
	ExitCurve    []ExitPoint        `json:"exit_curve"`

	// FrontLoadedYears is how many years the standard charges more than a
	// straight-line rent would have. It is the sentence a budget holder needs.
	FrontLoadedYears int    `json:"front_loaded_years"`
	Headline         string `json:"headline"`
}

// Build produces the briefing for a draft.
func Build(draft Draft) (Briefing, error) {
	if draft.TermMonths <= 0 {
		return Briefing{}, fmt.Errorf("租期月数必须大于零")
	}
	if draft.MonthlyRent < 0 {
		return Briefing{}, fmt.Errorf("月租金不能为负数")
	}
	if draft.RentFreeMonths < 0 || draft.RentFreeMonths > draft.TermMonths {
		return Briefing{}, fmt.Errorf("免租期须介于零与租期之间")
	}
	if draft.DiscountRate <= 0 {
		// The whole briefing moves with the rate, so inventing one would be
		// answering a question nobody asked.
		return Briefing{}, fmt.Errorf("请提供折现率：入表金额与费用曲线都取决于它")
	}
	if draft.CommencementDate.IsZero() {
		return Briefing{}, fmt.Errorf("请提供起租日")
	}

	payments := buildPayments(draft)

	// The schedule boundary is exclusive, so the term runs to the day after the
	// final payment: a sixty-month lease from 1 January covers through
	// 31 December, and stopping on that date would drop the last month's rent.
	leaseEndDate := payments[len(payments)-1].Date.AddDate(0, 0, 1)

	calculation := ifrs16.LeaseCalculation{
		CommencementDate:  draft.CommencementDate,
		LeaseEndDate:      leaseEndDate,
		DiscountRate:      draft.DiscountRate,
		LeaseScope:        ifrs16.LeaseScopeInScope,
		InitialDirectCost: draft.InitialDirectCost,
		Payments:          payments,
	}

	measured, err := ifrs16.Calculate(calculation)
	if err != nil {
		return Briefing{}, fmt.Errorf("测算失败：%w", err)
	}

	briefing := Briefing{
		Name: draft.Name, Currency: draft.Currency,
		DiscountRate: draft.DiscountRate, TermMonths: draft.TermMonths,
	}

	var undiscounted float64
	for _, payment := range payments {
		undiscounted += payment.Amount
	}
	briefing.BalanceSheet = BalanceSheetImpact{
		InitialLiability:       round2(measured.InitialLiability),
		InitialROU:             round2(measured.InitialROUAsset),
		UndiscountedCommitment: round2(undiscounted),
		DiscountingEffect:      round2(undiscounted - measured.InitialLiability),
	}

	// A straight-line rent spreads the whole commitment evenly, which is the
	// charge the business would have carried had the lease stayed off balance
	// sheet. It is the benchmark every one of these views compares against.
	straightLineMonthly := undiscounted / float64(draft.TermMonths)

	briefing.Yearly = summariseByYear(measured.MonthlySummary, draft, straightLineMonthly)
	briefing.Bridge = buildBridge(briefing.Yearly)
	briefing.ExitCurve = buildExitCurve(draft, calculation, payments)

	for _, year := range briefing.Yearly {
		if year.ExpenseVsStraightLine > 0 {
			briefing.FrontLoadedYears++
		}
	}
	briefing.Headline = headline(briefing)
	return briefing, nil
}

// buildPayments turns the draft terms into a monthly schedule. Payments fall at
// month end, which is how commercial rent is normally settled.
func buildPayments(draft Draft) []ifrs16.LeasePayment {
	payments := make([]ifrs16.LeasePayment, 0, draft.TermMonths)
	firstOfMonth := time.Date(draft.CommencementDate.Year(), draft.CommencementDate.Month(), 1,
		0, 0, 0, 0, draft.CommencementDate.Location())

	for month := 0; month < draft.TermMonths; month++ {
		amount := 0.0
		if month >= draft.RentFreeMonths {
			amount = draft.MonthlyRent * math.Pow(1+draft.AnnualEscalationPercent/100, float64(month/12))
		}
		monthEnd := firstOfMonth.AddDate(0, month+1, 0).AddDate(0, 0, -1)
		payments = append(payments, ifrs16.LeasePayment{
			Date: monthEnd, Amount: round2(amount), Timing: "postpaid", Type: "fixed",
		})
	}
	return payments
}

func summariseByYear(monthly []ifrs16.MonthlyEntry, draft Draft, straightLineMonthly float64) []YearlyImpact {
	// Years are counted from the start of the lease, not from January: "year
	// one of this lease" is the unit the decision is discussed in.
	startYear, startMonth := draft.CommencementDate.Year(), int(draft.CommencementDate.Month())
	byYear := map[int]*YearlyImpact{}
	monthsInYear := map[int]int{}

	for _, entry := range monthly {
		elapsed := (entry.Year-startYear)*12 + (entry.Month - startMonth)
		if elapsed < 0 {
			continue
		}
		year := elapsed/12 + 1
		impact := byYear[year]
		if impact == nil {
			impact = &YearlyImpact{Year: year}
			byYear[year] = impact
		}
		impact.CashRent += entry.TotalPayments
		impact.Interest += entry.InterestExpense
		impact.Depreciation += entry.Depreciation
		impact.ClosingLiability = entry.ClosingLiability
		impact.ClosingROU = entry.ClosingROUAsset
		monthsInYear[year]++
	}

	years := make([]YearlyImpact, 0, len(byYear))
	for year := 1; year <= (draft.TermMonths+11)/12; year++ {
		impact := byYear[year]
		if impact == nil {
			continue
		}
		impact.StraightLineRent = round2(straightLineMonthly * float64(monthsInYear[year]))
		impact.IFRS16Expense = round2(impact.Interest + impact.Depreciation)
		impact.ExpenseVsStraightLine = round2(impact.IFRS16Expense - impact.StraightLineRent)
		impact.CashRent = round2(impact.CashRent)
		impact.Interest = round2(impact.Interest)
		impact.Depreciation = round2(impact.Depreciation)
		impact.ClosingLiability = round2(impact.ClosingLiability)
		impact.ClosingROU = round2(impact.ClosingROU)
		years = append(years, *impact)
	}
	return years
}

func buildBridge(years []YearlyImpact) []EBITDABridge {
	bridge := make([]EBITDABridge, 0, len(years))
	for _, year := range years {
		// Off balance sheet, the straight-line rent would have been an
		// operating expense sitting above EBITDA. Capitalising the lease takes
		// it out of there and puts depreciation and interest below.
		bridge = append(bridge, EBITDABridge{
			Year:                    year.Year,
			RentAboveEBITDA:         year.StraightLineRent,
			EBITDAUplift:            year.StraightLineRent,
			DepreciationBelowEBITDA: year.Depreciation,
			InterestBelowEBIT:       year.Interest,
			NetProfitImpact:         year.ExpenseVsStraightLine,
		})
	}
	return bridge
}

// buildExitCurve prices walking away at the end of each year.
func buildExitCurve(draft Draft, calculation ifrs16.LeaseCalculation, payments []ifrs16.LeasePayment) []ExitPoint {
	totalYears := (draft.TermMonths + 11) / 12
	curve := make([]ExitPoint, 0, totalYears)

	for year := 1; year < totalYears; year++ {
		exitDate := draft.CommencementDate.AddDate(0, year*12, 0)
		liability, rou, err := ifrs16.GetCarryingAmount(calculation, exitDate)
		if err != nil {
			continue
		}

		var remaining float64
		for _, payment := range payments {
			if payment.Date.After(exitDate) {
				remaining += payment.Amount
			}
		}

		// The break fee is quoted in months of the rent in force at the time,
		// not of the opening rent.
		rentAtExit := draft.MonthlyRent * math.Pow(1+draft.AnnualEscalationPercent/100, float64(year))
		penalty := round2(draft.EarlyExitPenaltyMonths * rentAtExit)

		// Full derecognition: the liability goes, the asset goes, and the
		// difference is a gain or a loss. Positive PnLImpact is a cost, so a
		// liability larger than the asset reduces it.
		derecognition := rou - liability

		curve = append(curve, ExitPoint{
			Year:                year,
			RemainingCommitment: round2(remaining),
			LiabilityReleased:   round2(liability),
			ROUWrittenOff:       round2(rou),
			Penalty:             penalty,
			PnLImpact:           round2(derecognition + penalty),
			TotalCashToExit:     penalty,
		})
	}
	return curve
}

func headline(briefing Briefing) string {
	if len(briefing.Yearly) == 0 {
		return ""
	}
	first := briefing.Yearly[0]
	uplift := 0.0
	if len(briefing.Bridge) > 0 {
		uplift = briefing.Bridge[0].EBITDAUplift
	}

	return fmt.Sprintf(
		"入表负债 %.2f、使用权资产 %.2f。首年 IFRS 16 费用 %.2f，较直线租金高 %.2f；"+
			"前 %d 年费用高于直线租金，之后低于——这是时间性差异，全期合计为零。"+
			"首年 EBITDA 因租金移至线下被动抬升 %.2f，与经营改善无关，需向管理层说明。",
		briefing.BalanceSheet.InitialLiability, briefing.BalanceSheet.InitialROU,
		first.IFRS16Expense, first.ExpenseVsStraightLine, briefing.FrontLoadedYears, uplift)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
