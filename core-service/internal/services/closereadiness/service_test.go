package closereadiness

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/controlrules"
)

func testRules() map[string]controlrules.Definition {
	result := make(map[string]controlrules.Definition, len(controlrules.Codes))
	for _, code := range controlrules.Codes {
		result[code] = controlrules.Definition{Code: code, Version: "test", Severity: controlrules.SeverityBlocking, GateEffect: "formal_calculation"}
	}
	return result
}

func TestEvaluateBlocksMissingInputsAndOrdersFindings(t *testing.T) {
	result := EvaluateWithRules(Input{
		AccountingPeriod: "2026-08",
		ScopeComplete:    true,
		EvaluatedAt:      time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
	}, Facts{
		Contracts: []ContractFact{
			{ContractID: "2", ContractNumber: "B-002", ContractName: "B", LeaseScope: "in_scope", HasApprovedPaymentPlan: true, HasPendingEvent: true},
			{ContractID: "1", ContractNumber: "A-001", ContractName: "A", LeaseScope: "in_scope"},
		},
	}, testRules())

	if result.Status != StatusBlocked {
		t.Fatalf("status = %q, want blocked", result.Status)
	}
	if result.PopulationCount != 2 || result.BlockingCount != 4 || result.FindingCount != 4 {
		t.Fatalf("counts = population %d blocking %d findings %d", result.PopulationCount, result.BlockingCount, result.FindingCount)
	}
	if got := result.Findings[0].RuleCode; got != RuleMissingDiscountRate {
		t.Fatalf("first finding = %q, want missing discount rate", got)
	}
	if got := result.Findings[len(result.Findings)-1].ContractNumber; got != "B-002" {
		t.Fatalf("last contract = %q, want B-002", got)
	}
}

func TestEvaluateUsesPolicyRateAndSkipsNonLeases(t *testing.T) {
	result := EvaluateWithRules(Input{
		AccountingPeriod:   "2026-08",
		ScopeComplete:      true,
		GlobalDiscountRate: 0.05,
	}, Facts{Contracts: []ContractFact{
		{ContractID: "1", ContractNumber: "LEASE-1", LeaseScope: "in_scope", HasApprovedPaymentPlan: true},
		{ContractID: "2", ContractNumber: "SERVICE-1", LeaseScope: "not_a_lease"},
	}}, testRules())

	if result.Status != StatusReady {
		t.Fatalf("status = %q, want ready", result.Status)
	}
	if result.PopulationCount != 1 || result.FindingCount != 0 {
		t.Fatalf("population/findings = %d/%d, want 1/0", result.PopulationCount, result.FindingCount)
	}
}

func TestEvaluateDistinguishesPartialScopeAndEmptyPopulation(t *testing.T) {
	partial := EvaluateWithRules(Input{AccountingPeriod: "2026-08", ScopeComplete: false}, Facts{
		Contracts: []ContractFact{{ContractID: "1", LeaseScope: "short_term_exempt", HasApprovedPaymentPlan: true}},
	}, testRules())
	if partial.Status != StatusScopeLimited {
		t.Fatalf("partial status = %q, want scope_limited", partial.Status)
	}

	empty := EvaluateWithRules(Input{AccountingPeriod: "2026-08", ScopeComplete: true}, Facts{}, testRules())
	if empty.Status != StatusNotRun {
		t.Fatalf("empty status = %q, want not_run", empty.Status)
	}
}

func TestEvaluateAddsFailedBatchFinding(t *testing.T) {
	result := EvaluateWithRules(Input{AccountingPeriod: "2026-08", ScopeComplete: true}, Facts{
		Contracts:     []ContractFact{{ContractID: "1", LeaseScope: "short_term_exempt", HasApprovedPaymentPlan: true}},
		FailedBatches: []FailedBatchFact{{BatchID: "batch-1", BatchNumber: "BATCH-2026-08", Status: "completed_with_errors", FailedContracts: 3}},
	}, testRules())

	if result.BlockingCount != 1 || result.Findings[0].RuleCode != RuleFailedCloseBatch {
		t.Fatalf("unexpected failed batch result: %+v", result)
	}
}
