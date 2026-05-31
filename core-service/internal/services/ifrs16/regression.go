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

	addAssert := func(name string, expected, actual float64) {
		assertion := RegressionAssert{
			Name:      name,
			Expected:  expected,
			Actual:    actual,
			Delta:     round(math.Abs(actual - expected)),
			Tolerance: tolerance,
		}
		assertion.Passed = assertion.Delta <= tolerance
		if !assertion.Passed {
			caseRun.Passed = false
		}
		caseRun.Assertions = append(caseRun.Assertions, assertion)
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

	if len(caseRun.Assertions) == 0 {
		caseRun.Passed = false
		caseRun.Error = "regression case has no expected assertions"
	}

	return caseRun
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
		CommencementDate:  commencementDate,
		LeaseEndDate:      leaseEndDate,
		LeaseScope:        input.LeaseScope,
		DiscountRate:      input.DiscountRate,
		InitialDirectCost: input.InitialDirectCost,
		PrepaidRent:       input.PrepaidRent,
		IncentiveReceived: input.IncentiveReceived,
		RestorationCost:   input.RestorationCost,
		Payments:          payments,
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
