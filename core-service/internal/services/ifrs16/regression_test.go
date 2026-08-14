package ifrs16

import (
	"path/filepath"
	"testing"

	"github.com/lease-management-system/core-service/internal/money"
)

func TestRegressionFixtures(t *testing.T) {
	suite, err := LoadRegressionSuite(filepath.Join("testdata", "ifrs16_regression_cases.json"))
	if err != nil {
		t.Fatalf("load regression suite: %v", err)
	}

	run := RunRegressionSuite(suite)
	if run.Assertions == 0 {
		t.Fatal("regression suite did not execute any assertions")
	}
	for _, caseRun := range run.CaseRuns {
		if caseRun.Passed {
			continue
		}
		t.Run(caseRun.Case.ID, func(t *testing.T) {
			if caseRun.Error != "" {
				t.Fatalf("%s", caseRun.Error)
			}
			for _, assertion := range caseRun.Assertions {
				if assertion.Passed {
					continue
				}
				t.Errorf(
					"%s: expected %.2f, actual %.2f, delta %.2f, tolerance %.2f",
					assertion.Name,
					assertion.Expected.Float64(),
					assertion.Actual.Float64(),
					assertion.Delta.Float64(),
					assertion.Tolerance,
				)
			}
		})
	}
}

// moneyRound reproduces the historical cent rounding for test comparisons.
func moneyRound(value money.Amount) float64 { return value.Round("CNY").Float64() }
