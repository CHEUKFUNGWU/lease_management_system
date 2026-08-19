// Package closereadiness evaluates the deterministic, read-only checks that a
// Finance Analyst needs before starting a month-end close.
//
// The output is deliberately a diagnostic view, not a formal Close Exception
// or Control Conclusion. Persisted exception governance belongs behind the
// Projection Version and Close Snapshot boundaries described by the domain
// ADRs.
package closereadiness

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/controlrules"
)

const (
	StatusNotRun       = "not_run"
	StatusBlocked      = "blocked"
	StatusReady        = "ready"
	StatusScopeLimited = "scope_limited"

	RuleMissingPaymentSchedule = controlrules.RuleMissingPaymentSchedule
	RuleMissingDiscountRate    = controlrules.RuleMissingDiscountRate
	RulePendingEvent           = controlrules.RulePendingEvent
	RuleFailedCloseBatch       = controlrules.RuleFailedCloseBatch
)

// ContractFact is the minimum read-side evidence required for the V1 rules.
// It intentionally contains facts, not conclusions.
type ContractFact struct {
	ContractID             string
	ContractNumber         string
	ContractName           string
	LeaseScope             string
	DiscountRateValue      *float64
	HasApprovedPaymentPlan bool
	HasPendingEvent        bool
}

type FailedBatchFact struct {
	BatchID         string
	BatchNumber     string
	Status          string
	FailedContracts int
}

type Facts struct {
	Contracts     []ContractFact
	FailedBatches []FailedBatchFact
}

type FactsSource interface {
	LoadFacts(ctx context.Context, period, legalEntityID string) (Facts, error)
}

type RateSource interface {
	GetFloat64(ctx context.Context, key string, fallback float64) float64
}

type Service struct {
	source FactsSource
	rates  RateSource
	rules  controlrules.Source
	now    func() time.Time
}

func NewService(source FactsSource, rates RateSource, ruleSources ...controlrules.Source) *Service {
	var rules controlrules.Source
	if len(ruleSources) > 0 {
		rules = ruleSources[0]
	}
	return &Service{source: source, rates: rates, rules: rules, now: func() time.Time { return time.Now().UTC() }}
}

type Command struct {
	AccountingPeriod string
	LegalEntityID    string
	ScopeComplete    bool
}

func (s *Service) Evaluate(ctx context.Context, cmd Command) (*Result, error) {
	if s == nil || s.source == nil {
		return nil, errors.New("close readiness facts source is required")
	}
	if _, err := time.Parse("2006-01", strings.TrimSpace(cmd.AccountingPeriod)); err != nil {
		return nil, errors.New("accounting period must use YYYY-MM format")
	}

	facts, err := s.source.LoadFacts(ctx, strings.TrimSpace(cmd.AccountingPeriod), cmd.LegalEntityID)
	if err != nil {
		return nil, err
	}
	periodEnd, _ := time.Parse("2006-01", strings.TrimSpace(cmd.AccountingPeriod))
	periodEnd = periodEnd.AddDate(0, 1, -1)
	rules, err := controlrules.LoadActive(ctx, s.rules, periodEnd)
	if err != nil {
		return nil, err
	}

	globalRate := 0.0
	if s.rates != nil {
		globalRate = s.rates.GetFloat64(ctx, "global_discount_rate", 0)
	}

	result := EvaluateWithRules(Input{
		AccountingPeriod:   strings.TrimSpace(cmd.AccountingPeriod),
		ScopeComplete:      cmd.ScopeComplete,
		GlobalDiscountRate: globalRate,
		EvaluatedAt:        s.now(),
	}, facts, rules)
	return &result, nil
}

type Input struct {
	AccountingPeriod   string
	ScopeComplete      bool
	GlobalDiscountRate float64
	EvaluatedAt        time.Time
}

type Result struct {
	AccountingPeriod string    `json:"accounting_period"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	ScopeComplete    bool      `json:"scope_complete"`
	PopulationCount  int       `json:"population_count"`
	Status           string    `json:"status"`
	BlockingCount    int       `json:"blocking_count"`
	FindingCount     int       `json:"finding_count"`
	Findings         []Finding `json:"findings"`
}

type Finding struct {
	RuleCode       string `json:"rule_code"`
	RuleVersion    string `json:"rule_version,omitempty"`
	Severity       string `json:"severity"`
	GateEffect     string `json:"gate_effect"`
	ContractID     string `json:"contract_id,omitempty"`
	ContractNumber string `json:"contract_number,omitempty"`
	ContractName   string `json:"contract_name,omitempty"`
	Title          string `json:"title"`
	Reason         string `json:"reason"`
	Remediation    string `json:"remediation"`
	SourceKind     string `json:"source_kind"`
	SourceID       string `json:"source_id"`
	TargetPath     string `json:"target_path"`
}

// EvaluateWithRules applies the effective control rules to a stable facts set.
// It is deterministic so the same facts and rule version produce the same
// ordering and status.
func EvaluateWithRules(input Input, facts Facts, rules map[string]controlrules.Definition) Result {
	result := Result{
		AccountingPeriod: input.AccountingPeriod,
		EvaluatedAt:      input.EvaluatedAt,
		ScopeComplete:    input.ScopeComplete,
		Findings:         []Finding{},
	}

	evaluatedContracts := 0
	for _, contract := range facts.Contracts {
		if contract.LeaseScope != "not_a_lease" {
			evaluatedContracts++
		}
	}
	result.PopulationCount = evaluatedContracts
	if evaluatedContracts == 0 {
		result.Status = StatusNotRun
		return result
	}

	for _, contract := range facts.Contracts {
		scope := contract.LeaseScope
		if scope == "" {
			scope = "in_scope"
		}
		if scope == "not_a_lease" {
			continue
		}

		if !contract.HasApprovedPaymentPlan {
			rule := rules[RuleMissingPaymentSchedule]
			result.Findings = append(result.Findings, Finding{
				RuleCode: RuleMissingPaymentSchedule, RuleVersion: rule.Version, Severity: rule.Severity, GateEffect: rule.GateEffect,
				ContractID: contract.ContractID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
				Title: rule.Title, Reason: renderReason(rule.ReasonTemplate),
				Remediation: rule.Remediation,
				SourceKind:  "contract", SourceID: contract.ContractID, TargetPath: "/contracts/" + contract.ContractID,
			})
		}

		if scope == "in_scope" && !hasDiscountRate(contract.DiscountRateValue, input.GlobalDiscountRate) {
			rule := rules[RuleMissingDiscountRate]
			result.Findings = append(result.Findings, Finding{
				RuleCode: RuleMissingDiscountRate, RuleVersion: rule.Version, Severity: rule.Severity, GateEffect: rule.GateEffect,
				ContractID: contract.ContractID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
				Title: rule.Title, Reason: renderReason(rule.ReasonTemplate),
				Remediation: rule.Remediation,
				SourceKind:  "contract", SourceID: contract.ContractID, TargetPath: "/contracts/" + contract.ContractID,
			})
		}

		if scope == "in_scope" && contract.HasPendingEvent {
			rule := rules[RulePendingEvent]
			result.Findings = append(result.Findings, Finding{
				RuleCode: RulePendingEvent, RuleVersion: rule.Version, Severity: rule.Severity, GateEffect: rule.GateEffect,
				ContractID: contract.ContractID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
				Title: rule.Title, Reason: renderReason(rule.ReasonTemplate),
				Remediation: rule.Remediation,
				SourceKind:  "contract", SourceID: contract.ContractID, TargetPath: "/contracts/" + contract.ContractID,
			})
		}
	}

	for _, batch := range facts.FailedBatches {
		rule := rules[RuleFailedCloseBatch]
		result.Findings = append(result.Findings, Finding{
			RuleCode: RuleFailedCloseBatch, RuleVersion: rule.Version, Severity: rule.Severity, GateEffect: rule.GateEffect,
			Title:       rule.Title,
			Reason:      renderReason(rule.ReasonTemplate, batch.BatchNumber, batch.Status, batch.FailedContracts),
			Remediation: rule.Remediation,
			SourceKind:  "monthly_closing_batch", SourceID: batch.BatchID, TargetPath: "/monthly-closing",
		})
	}

	sort.SliceStable(result.Findings, func(i, j int) bool {
		left := result.Findings[i]
		right := result.Findings[j]
		if left.Severity != right.Severity {
			return left.Severity == controlrules.SeverityBlocking
		}
		if left.RuleCode != right.RuleCode {
			return left.RuleCode < right.RuleCode
		}
		if left.ContractNumber != right.ContractNumber {
			return left.ContractNumber < right.ContractNumber
		}
		return left.SourceID < right.SourceID
	})

	result.FindingCount = len(result.Findings)
	for _, finding := range result.Findings {
		if finding.Severity == controlrules.SeverityBlocking {
			result.BlockingCount++
		}
	}
	if result.BlockingCount > 0 {
		result.Status = StatusBlocked
	} else if !input.ScopeComplete {
		result.Status = StatusScopeLimited
	} else {
		result.Status = StatusReady
	}
	return result
}

func renderReason(template string, args ...any) string {
	if template == "" {
		return ""
	}
	// Interpolate %s/%v verbs without fmt.Sprintf: templates are rule data,
	// not code, so a variable format string must not be passed to a printf
	// family call (go vet, Go 1.24+).
	out := template
	for _, arg := range args {
		repl := fmt.Sprint(arg)
		if idx := strings.Index(out, "%s"); idx >= 0 {
			out = out[:idx] + repl + out[idx+2:]
			continue
		}
		if idx := strings.Index(out, "%v"); idx >= 0 {
			out = out[:idx] + repl + out[idx+2:]
			continue
		}
		break
	}
	return out
}

func hasDiscountRate(contractRate *float64, globalRate float64) bool {
	if contractRate != nil && validRate(*contractRate) {
		return true
	}
	return validRate(globalRate)
}

func validRate(rate float64) bool {
	if rate > 1 {
		rate /= 100
	}
	return rate > 0 && rate <= 1
}
