// Package controlrules contains the stable vocabulary shared by close
// readiness and persisted close exception governance. The rule descriptions
// themselves live in close_control_rules so they can be versioned without a
// code release.
package controlrules

import (
	"context"
	"fmt"
	"time"
)

const (
	RuleMissingPaymentSchedule = "missing_payment_schedule"
	RuleMissingDiscountRate    = "missing_discount_rate"
	RulePendingEvent           = "pending_event_before_period_end"
	RuleFailedCloseBatch       = "failed_close_batch"

	SeverityBlocking = "blocking"
)

var Codes = []string{
	RuleMissingPaymentSchedule,
	RuleMissingDiscountRate,
	RulePendingEvent,
	RuleFailedCloseBatch,
}

// Definition is the effective-dated control policy used by a detector.
// Display text and remediation are data because finance policy owners may
// revise them without changing the detector implementation.
type Definition struct {
	Code           string
	Version        string
	Name           string
	Severity       string
	GateEffect     string
	Title          string
	ReasonTemplate string
	Remediation    string
}

// Source returns the rule effective for a close period.
type Source interface {
	GetActiveRule(ctx context.Context, code string, asOf time.Time) (Definition, bool, error)
}

func LoadActive(ctx context.Context, source Source, asOf time.Time) (map[string]Definition, error) {
	if source == nil {
		return nil, fmt.Errorf("close control rule source is required")
	}
	rules := make(map[string]Definition, len(Codes))
	for _, code := range Codes {
		rule, ok, err := source.GetActiveRule(ctx, code, asOf)
		if err != nil {
			return nil, fmt.Errorf("load close control rule %s: %w", code, err)
		}
		if !ok {
			return nil, fmt.Errorf("no active close control rule for %s", code)
		}
		rules[code] = rule
	}
	return rules, nil
}
