package ifrs16

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// RegressionSuite is the file format for IFRS 16 calculation regression cases.
type RegressionSuite struct {
	Version   string           `json:"version"`
	Currency  string           `json:"currency"`
	Tolerance float64          `json:"tolerance"`
	Cases     []RegressionCase `json:"cases"`
}

type RegressionCase struct {
	ID              string             `json:"id"`
	Title           string             `json:"title"`
	Scenario        string             `json:"scenario"`
	IFRS16Reference string             `json:"ifrs16_reference"`
	ReviewStatus    string             `json:"review_status"`
	Input           RegressionInput    `json:"input"`
	Expected        RegressionExpected `json:"expected"`
}

type RegressionInput struct {
	CommencementDate  string              `json:"commencement_date"`
	LeaseEndDate      string              `json:"lease_end_date"`
	LeaseScope        string              `json:"lease_scope"`
	DiscountRate      float64             `json:"discount_rate"`
	InitialDirectCost float64             `json:"initial_direct_cost"`
	PrepaidRent       float64             `json:"prepaid_rent"`
	IncentiveReceived float64             `json:"incentive_received"`
	RestorationCost   float64             `json:"restoration_cost"`
	Payments          []RegressionPayment `json:"payments"`

	// ForeignCurrency is set only by multi-currency cases. When present the
	// runner also checks the IAS 21 remeasurement of the lease liability.
	ForeignCurrency *RegressionForeignCurrency `json:"foreign_currency,omitempty"`

	// Remeasurement is set only by cases that test a mid-term change. When
	// present the runner derives the revised payments from the stated clause
	// and remeasures, so the case covers the whole chain a rent review actually
	// travels: clause as written → revised schedule → remeasured amounts.
	Remeasurement *RegressionRemeasurement `json:"remeasurement,omitempty"`
}

// RegressionRemeasurement describes a change to a running lease.
type RegressionRemeasurement struct {
	EffectiveDate string `json:"effective_date"`
	// RevisedLeaseEndDate shortens or extends the term. Empty leaves it alone.
	RevisedLeaseEndDate string `json:"revised_lease_end_date,omitempty"`
	// RevisedDiscountRate is used when the change is a rate reassessment. Zero
	// keeps the original rate.
	RevisedDiscountRate float64 `json:"revised_discount_rate,omitempty"`
	// Clause is the rent term as the landlord's notice states it.
	Clause *PaymentRevision `json:"clause,omitempty"`
}

// RegressionForeignCurrency describes a lease denominated in a currency other
// than the reporting entity's functional currency, and the rates that apply to
// the period under test.
type RegressionForeignCurrency struct {
	ContractCurrency   string  `json:"contract_currency"`
	FunctionalCurrency string  `json:"functional_currency"`
	Period             string  `json:"period"`
	OpeningRate        float64 `json:"opening_rate"`
	ClosingRate        float64 `json:"closing_rate"`
	AverageRate        float64 `json:"average_rate"`
}

type RegressionPayment struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Timing string  `json:"timing"`
	Type   string  `json:"type"`
}

type RegressionExpected struct {
	InitialLiability *float64                    `json:"initial_liability,omitempty"`
	InitialROUAsset  *float64                    `json:"initial_rou_asset,omitempty"`
	Monthly          []RegressionMonthlyExpected `json:"monthly,omitempty"`

	// ExchangeDifference is the expected IAS 21 gain or loss on the lease
	// liability for the period named in Input.ForeignCurrency; positive is a
	// loss. That the right-of-use asset is *not* retranslated is asserted the
	// ordinary way, through Monthly.ClosingROUAsset in the contract currency.
	ExchangeDifference *float64 `json:"exchange_difference,omitempty"`

	// Remeasured holds what the change in Input.Remeasurement should produce.
	Remeasured *RegressionRemeasured `json:"remeasured,omitempty"`
}

// RegressionRemeasured is the expected outcome of a mid-term change.
type RegressionRemeasured struct {
	// RevisedPaymentTotal is the sum of the payments still outstanding after
	// the clause is applied. It pins the derivation itself, so a case that
	// fails tells you whether the schedule or the measurement was wrong.
	RevisedPaymentTotal *float64 `json:"revised_payment_total,omitempty"`
	ChangedPaymentCount *int     `json:"changed_payment_count,omitempty"`
	AppliedFactor       *float64 `json:"applied_factor,omitempty"`

	LiabilityBefore *float64 `json:"liability_before,omitempty"`
	LiabilityAfter  *float64 `json:"liability_after,omitempty"`
	ROUBefore       *float64 `json:"rou_before,omitempty"`
	ROUAfter        *float64 `json:"rou_after,omitempty"`
	PnLGain         *float64 `json:"pnl_gain,omitempty"`
	PnLLoss         *float64 `json:"pnl_loss,omitempty"`
}

type RegressionMonthlyExpected struct {
	Period              string   `json:"period"`
	OpeningLiability    *float64 `json:"opening_liability,omitempty"`
	InterestExpense     *float64 `json:"interest_expense,omitempty"`
	TotalPayments       *float64 `json:"total_payments,omitempty"`
	PrepaidPayment      *float64 `json:"prepaid_payment,omitempty"`
	ClosingLiability    *float64 `json:"closing_liability,omitempty"`
	OpeningROUAsset     *float64 `json:"opening_rou_asset,omitempty"`
	Depreciation        *float64 `json:"depreciation,omitempty"`
	ClosingROUAsset     *float64 `json:"closing_rou_asset,omitempty"`
	ExemptLeaseExpense  *float64 `json:"exempt_lease_expense,omitempty"`
	VariableRentExpense *float64 `json:"variable_rent_expense,omitempty"`
	NonLeaseExpense     *float64 `json:"non_lease_expense,omitempty"`
}

type RegressionRun struct {
	Suite       RegressionSuite     `json:"suite"`
	CaseRuns    []RegressionCaseRun `json:"case_runs"`
	Passed      int                 `json:"passed"`
	Failed      int                 `json:"failed"`
	Assertions  int                 `json:"assertions"`
	GeneratedAt time.Time           `json:"generated_at"`
}

type RegressionCaseRun struct {
	Case       RegressionCase     `json:"case"`
	Result     *CalculationResult `json:"-"`
	Assertions []RegressionAssert `json:"assertions"`
	Error      string             `json:"error,omitempty"`
	Passed     bool               `json:"passed"`
}

type RegressionAssert struct {
	Name      string  `json:"name"`
	Expected  float64 `json:"expected"`
	Actual    float64 `json:"actual"`
	Delta     float64 `json:"delta"`
	Tolerance float64 `json:"tolerance"`
	Passed    bool    `json:"passed"`
}

func LoadRegressionSuite(path string) (RegressionSuite, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RegressionSuite{}, err
	}
	var suite RegressionSuite
	if err := json.Unmarshal(raw, &suite); err != nil {
		return RegressionSuite{}, err
	}
	if suite.Tolerance <= 0 {
		suite.Tolerance = 1
	}
	return suite, nil
}

func RunRegressionSuite(suite RegressionSuite) RegressionRun {
	run := RegressionRun{
		Suite:       suite,
		GeneratedAt: time.Now(),
	}

	for _, tc := range suite.Cases {
		caseRun := runRegressionCase(tc, suite.Tolerance)
		run.Assertions += len(caseRun.Assertions)
		if caseRun.Passed {
			run.Passed++
		} else {
			run.Failed++
		}
		run.CaseRuns = append(run.CaseRuns, caseRun)
	}

	return run
}

func runRegressionCase(tc RegressionCase, tolerance float64) RegressionCaseRun {
	caseRun := RegressionCaseRun{Case: tc, Passed: true}

	input, err := tc.Input.toCalculation()
	if err != nil {
		caseRun.Error = err.Error()
		caseRun.Passed = false
		return caseRun
	}

	result, err := Calculate(input)
	if err != nil {
		caseRun.Error = err.Error()
		caseRun.Passed = false
		return caseRun
	}
	caseRun.Result = result

	// The suite tolerance is a money tolerance — one unit of currency. A
	// dimensionless quantity such as an index factor needs its own, or a
	// difference that matters would sit far inside the allowance and the
	// assertion could never fail.
	addAssertWithin := func(name string, expected, actual, allowed float64) {
		assertion := RegressionAssert{
			Name:      name,
			Expected:  expected,
			Actual:    actual,
			Delta:     math.Abs(actual - expected),
			Tolerance: allowed,
		}
		assertion.Passed = assertion.Delta <= allowed
		if !assertion.Passed {
			caseRun.Passed = false
		}
		caseRun.Assertions = append(caseRun.Assertions, assertion)
	}
	addAssert := func(name string, expected, actual float64) {
		addAssertWithin(name, expected, round(actual), tolerance)
	}

	if tc.Expected.InitialLiability != nil {
		addAssert("initial_liability", *tc.Expected.InitialLiability, result.InitialLiability)
	}
	if tc.Expected.InitialROUAsset != nil {
		addAssert("initial_rou_asset", *tc.Expected.InitialROUAsset, result.InitialROUAsset)
	}

	monthlyByPeriod := map[string]MonthlyEntry{}
	for _, entry := range result.MonthlySummary {
		monthlyByPeriod[fmt.Sprintf("%04d-%02d", entry.Year, entry.Month)] = entry
	}

	for _, expectedMonth := range tc.Expected.Monthly {
		actualMonth, ok := monthlyByPeriod[expectedMonth.Period]
		if !ok {
			caseRun.Passed = false
			caseRun.Assertions = append(caseRun.Assertions, RegressionAssert{
				Name:      expectedMonth.Period + ".exists",
				Expected:  1,
				Actual:    0,
				Delta:     1,
				Tolerance: tolerance,
				Passed:    false,
			})
			continue
		}
		prefix := expectedMonth.Period + "."
		if expectedMonth.OpeningLiability != nil {
			addAssert(prefix+"opening_liability", *expectedMonth.OpeningLiability, actualMonth.OpeningLiability)
		}
		if expectedMonth.InterestExpense != nil {
			addAssert(prefix+"interest_expense", *expectedMonth.InterestExpense, round(actualMonth.InterestExpense))
		}
		if expectedMonth.TotalPayments != nil {
			addAssert(prefix+"total_payments", *expectedMonth.TotalPayments, round(actualMonth.TotalPayments))
		}
		if expectedMonth.PrepaidPayment != nil {
			addAssert(prefix+"prepaid_payment", *expectedMonth.PrepaidPayment, round(actualMonth.PrepaidPayment))
		}
		if expectedMonth.ClosingLiability != nil {
			addAssert(prefix+"closing_liability", *expectedMonth.ClosingLiability, actualMonth.ClosingLiability)
		}
		if expectedMonth.OpeningROUAsset != nil {
			addAssert(prefix+"opening_rou_asset", *expectedMonth.OpeningROUAsset, actualMonth.OpeningROUAsset)
		}
		if expectedMonth.Depreciation != nil {
			addAssert(prefix+"depreciation", *expectedMonth.Depreciation, round(actualMonth.Depreciation))
		}
		if expectedMonth.ClosingROUAsset != nil {
			addAssert(prefix+"closing_rou_asset", *expectedMonth.ClosingROUAsset, actualMonth.ClosingROUAsset)
		}
		if expectedMonth.ExemptLeaseExpense != nil {
			addAssert(prefix+"exempt_lease_expense", *expectedMonth.ExemptLeaseExpense, round(actualMonth.ExemptLeaseExpense))
		}
		if expectedMonth.VariableRentExpense != nil {
			addAssert(prefix+"variable_rent_expense", *expectedMonth.VariableRentExpense, round(actualMonth.VariableRentExpense))
		}
		if expectedMonth.NonLeaseExpense != nil {
			addAssert(prefix+"non_lease_expense", *expectedMonth.NonLeaseExpense, round(actualMonth.NonLeaseExpense))
		}
	}

	// Multi-currency cases additionally assert the IAS 21 translation of the
	// lease liability, and that the right-of-use asset is left alone.
	if fx := tc.Input.ForeignCurrency; fx != nil && tc.Expected.ExchangeDifference != nil {
		month, ok := monthlyByPeriod[fx.Period]
		if !ok {
			caseRun.Passed = false
			caseRun.Assertions = append(caseRun.Assertions, RegressionAssert{
				Name: fx.Period + ".exists", Expected: 1, Actual: 0,
				Delta: 1, Tolerance: tolerance, Passed: false,
			})
		} else {
			difference, err := RemeasureForeignCurrencyLiability(FXRemeasurementInput{
				ContractCurrency:   fx.ContractCurrency,
				FunctionalCurrency: fx.FunctionalCurrency,
				OpeningLiability:   month.OpeningLiability,
				Interest:           month.InterestExpense,
				Payments:           month.TotalPayments,
				ClosingLiability:   month.ClosingLiability,
				OpeningRate:        fx.OpeningRate,
				ClosingRate:        fx.ClosingRate,
				AverageRate:        fx.AverageRate,
			})
			if err != nil {
				caseRun.Passed = false
				caseRun.Error = err.Error()
			} else {
				addAssert(fx.Period+".exchange_difference", *tc.Expected.ExchangeDifference, difference)
			}
		}
	}

	// Mid-term change cases walk the whole chain: the clause as written, the
	// schedule it derives, and the amounts that schedule remeasures to.
	if change := tc.Input.Remeasurement; change != nil && tc.Expected.Remeasured != nil {
		if err := assertRemeasurement(&caseRun, input, *change, *tc.Expected.Remeasured, addAssert, addAssertWithin); err != nil {
			caseRun.Passed = false
			caseRun.Error = err.Error()
		}
	}

	if len(caseRun.Assertions) == 0 {
		caseRun.Passed = false
		caseRun.Error = "regression case has no expected assertions"
	}

	return caseRun
}

// assertRemeasurement applies a mid-term change to the case's lease and checks
// what it produced.
func assertRemeasurement(
	caseRun *RegressionCaseRun,
	original LeaseCalculation,
	change RegressionRemeasurement,
	expected RegressionRemeasured,
	addAssert func(name string, expected, actual float64),
	addAssertWithin func(name string, expected, actual, allowed float64),
) error {
	effectiveDate, err := parseRegressionDate(change.EffectiveDate)
	if err != nil {
		return fmt.Errorf("invalid remeasurement effective_date: %w", err)
	}

	leaseEndDate := original.LeaseEndDate
	if change.RevisedLeaseEndDate != "" {
		leaseEndDate, err = parseRegressionDate(change.RevisedLeaseEndDate)
		if err != nil {
			return fmt.Errorf("invalid revised_lease_end_date: %w", err)
		}
	}
	discountRate := original.DiscountRate
	if change.RevisedDiscountRate > 0 {
		discountRate = change.RevisedDiscountRate
	}

	liabilityBefore, rouBefore, err := GetCarryingAmount(original, effectiveDate)
	if err != nil {
		return fmt.Errorf("carrying amount at the effective date: %w", err)
	}
	// Only stated expectations are asserted. Comparing an actual against itself
	// always passes and would report confidence the case has not earned.
	if expected.LiabilityBefore != nil {
		addAssert("remeasured.liability_before", *expected.LiabilityBefore, liabilityBefore)
	}
	if expected.ROUBefore != nil {
		addAssert("remeasured.rou_before", *expected.ROUBefore, rouBefore)
	}

	revisedPayments := original.Payments
	if change.Clause != nil {
		draft, err := DeriveRevisedPayments(original.Payments, *change.Clause, effectiveDate)
		if err != nil {
			return fmt.Errorf("derive revised payments: %w", err)
		}
		revisedPayments = draft.Payments()

		if expected.AppliedFactor != nil {
			// A factor is a ratio, not an amount: the fourth decimal place is
			// the difference between a capped and an uncapped review.
			addAssertWithin("remeasured.applied_factor", *expected.AppliedFactor, draft.AppliedFactor, 0.00005)
		}
		if expected.ChangedPaymentCount != nil {
			// A count is exact. Under the money tolerance of one unit, "12
			// payments changed" and "11 payments changed" would both pass.
			addAssertWithin("remeasured.changed_payment_count",
				float64(*expected.ChangedPaymentCount), float64(draft.ChangedCount), 0)
		}
		if expected.RevisedPaymentTotal != nil {
			addAssert("remeasured.revised_payment_total", *expected.RevisedPaymentTotal, draft.RevisedTotal)
		}
	}

	// A shortened term is a scope decrease, which is the only path that can
	// produce a gain or a loss.
	var scopeDecrease float64
	if leaseEndDate.Before(original.LeaseEndDate) {
		remaining := original.LeaseEndDate.Sub(effectiveDate).Hours()
		if remaining > 0 {
			revisedRemaining := math.Max(leaseEndDate.Sub(effectiveDate).Hours(), 0)
			scopeDecrease = (remaining - revisedRemaining) / remaining
		}
	}

	output, err := RecalculateFromDate(liabilityBefore, rouBefore, RemeasurementInput{
		EffectiveDate:           effectiveDate,
		LeaseEndDate:            leaseEndDate,
		RevisedDiscountRate:     discountRate,
		RevisedPayments:         revisedPayments,
		ScopeDecreaseProportion: scopeDecrease,
	})
	if err != nil {
		return fmt.Errorf("remeasure: %w", err)
	}

	if expected.LiabilityAfter != nil {
		addAssert("remeasured.liability_after", *expected.LiabilityAfter, output.NewLiability)
	}
	if expected.ROUAfter != nil {
		addAssert("remeasured.rou_after", *expected.ROUAfter, output.NewROU)
	}
	if expected.PnLGain != nil {
		addAssert("remeasured.pnl_gain", *expected.PnLGain, output.PnLGain)
	}
	if expected.PnLLoss != nil {
		addAssert("remeasured.pnl_loss", *expected.PnLLoss, output.PnLLoss)
	}
	return nil
}

func (input RegressionInput) toCalculation() (LeaseCalculation, error) {
	commencementDate, err := parseRegressionDate(input.CommencementDate)
	if err != nil {
		return LeaseCalculation{}, fmt.Errorf("invalid commencement_date: %w", err)
	}
	leaseEndDate, err := parseRegressionDate(input.LeaseEndDate)
	if err != nil {
		return LeaseCalculation{}, fmt.Errorf("invalid lease_end_date: %w", err)
	}

	payments := make([]LeasePayment, 0, len(input.Payments))
	for _, p := range input.Payments {
		paymentDate, err := parseRegressionDate(p.Date)
		if err != nil {
			return LeaseCalculation{}, fmt.Errorf("invalid payment date %q: %w", p.Date, err)
		}
		payments = append(payments, LeasePayment{
			Date:   paymentDate,
			Amount: p.Amount,
			Timing: p.Timing,
			Type:   p.Type,
		})
	}

	calcInput := LeaseCalculation{
		CommencementDate: commencementDate,
		LeaseEndDate:     leaseEndDate,
		// The original regression fixture predates the scope-gate field. Keep
		// those historical cases comparable while production calculation input
		// still rejects an omitted scope.
		LeaseScope:        input.LeaseScope,
		DiscountRate:      input.DiscountRate,
		InitialDirectCost: input.InitialDirectCost,
		PrepaidRent:       input.PrepaidRent,
		IncentiveReceived: input.IncentiveReceived,
		RestorationCost:   input.RestorationCost,
		Payments:          payments,
	}
	if calcInput.LeaseScope == "" {
		calcInput.LeaseScope = LeaseScopeInScope
	}
	if calcInput.PrepaidRent == 0 {
		calcInput.PrepaidRent = CalculatePrepaidRent(calcInput)
	}

	return calcInput, nil
}

func parseRegressionDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func RenderRegressionMarkdown(run RegressionRun) string {
	var b strings.Builder
	status := "PASS"
	if run.Failed > 0 {
		status = "FAIL"
	}

	fmt.Fprintf(&b, "# IFRS 16 计量回归对数报告\n\n")
	fmt.Fprintf(&b, "- 版本：%s\n", run.Suite.Version)
	fmt.Fprintf(&b, "- 生成时间：%s\n", run.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- 币种：%s\n", run.Suite.Currency)
	fmt.Fprintf(&b, "- 容忍差异：%.2f\n", run.Suite.Tolerance)
	fmt.Fprintf(&b, "- 总状态：%s\n", status)
	fmt.Fprintf(&b, "- 用例：%d 通过 / %d 失败\n", run.Passed, run.Failed)
	fmt.Fprintf(&b, "- 断言数：%d\n\n", run.Assertions)

	for _, caseRun := range run.CaseRuns {
		caseStatus := "PASS"
		if !caseRun.Passed {
			caseStatus = "FAIL"
		}
		fmt.Fprintf(&b, "## %s %s — %s\n\n", caseStatus, caseRun.Case.ID, caseRun.Case.Title)
		fmt.Fprintf(&b, "- 场景：%s\n", caseRun.Case.Scenario)
		fmt.Fprintf(&b, "- IFRS 16 依据：%s\n", caseRun.Case.IFRS16Reference)
		fmt.Fprintf(&b, "- 审核状态：%s\n", caseRun.Case.ReviewStatus)
		if caseRun.Error != "" {
			fmt.Fprintf(&b, "- 错误：%s\n", caseRun.Error)
		}
		if caseRun.Result != nil {
			fmt.Fprintf(&b, "- 实际初始负债：%.2f\n", caseRun.Result.InitialLiability)
			fmt.Fprintf(&b, "- 实际初始 ROU：%.2f\n", caseRun.Result.InitialROUAsset)
			fmt.Fprintf(&b, "- 计量范围：%s\n", caseRun.Result.LeaseScope)
			fmt.Fprintf(&b, "- 计量路径：%s\n", caseRun.Result.MeasurementBasis)
			fmt.Fprintf(&b, "\n实际月度结果：\n\n")
			fmt.Fprintf(&b, "| 期间 | 期初负债 | 利息 | 付款 | 先付款 | 期末负债 | 期初 ROU | 折旧 | 期末 ROU | 豁免费用 | 变量租金 | 非租赁费用 |\n")
			fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
			monthly := append([]MonthlyEntry(nil), caseRun.Result.MonthlySummary...)
			sort.Slice(monthly, func(i, j int) bool {
				left := fmt.Sprintf("%04d-%02d", monthly[i].Year, monthly[i].Month)
				right := fmt.Sprintf("%04d-%02d", monthly[j].Year, monthly[j].Month)
				return left < right
			})
			for _, entry := range monthly {
				fmt.Fprintf(
					&b,
					"| %04d-%02d | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
					entry.Year,
					entry.Month,
					entry.OpeningLiability,
					round(entry.InterestExpense),
					round(entry.TotalPayments),
					round(entry.PrepaidPayment),
					entry.ClosingLiability,
					entry.OpeningROUAsset,
					round(entry.Depreciation),
					entry.ClosingROUAsset,
					round(entry.ExemptLeaseExpense),
					round(entry.VariableRentExpense),
					round(entry.NonLeaseExpense),
				)
			}
		}
		fmt.Fprintf(&b, "\n校验明细：\n\n")
		fmt.Fprintf(&b, "| 校验项 | 期望 | 实际 | 差异 | 结果 |\n")
		fmt.Fprintf(&b, "|---|---:|---:|---:|---|\n")

		assertions := append([]RegressionAssert(nil), caseRun.Assertions...)
		sort.Slice(assertions, func(i, j int) bool {
			return assertions[i].Name < assertions[j].Name
		})
		for _, assertion := range assertions {
			assertStatus := "PASS"
			if !assertion.Passed {
				assertStatus = "FAIL"
			}
			fmt.Fprintf(
				&b,
				"| %s | %.2f | %.2f | %.2f | %s |\n",
				assertion.Name,
				assertion.Expected,
				assertion.Actual,
				assertion.Delta,
				assertStatus,
			)
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}
