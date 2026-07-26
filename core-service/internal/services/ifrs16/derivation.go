package ifrs16

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// A lease event used to say only what the new rent was, as a single free-text
// value, and someone had to edit the payment schedule by hand before recording
// it. The two steps were never checked against each other: change the payments
// and forget the event, or record the event and forget the payments, and the
// remeasurement is wrong either way.
//
// This file closes that gap. An event carries the terms as the landlord's
// notice states them — "+5% from 1 July", "CPI from 102.4 to 105.1, capped at
// 4%", "step to 12,000 next year and 13,000 the year after" — and the system
// derives the revised payment schedule from them. The derivation is a pure
// function of the original schedule and the stated terms, so the draft can be
// shown for confirmation before anything is written.

// materialityRounding is the cent at which a restated payment counts as having
// moved. Below it the difference is rounding, not a change in the rent.
const materialityRounding = 0.01

// Revision kinds. Each names how future payments are restated.
const (
	// RevisionSetAmount replaces each future fixed payment with one amount.
	RevisionSetAmount = "set_amount"
	// RevisionPercentage moves each future fixed payment by a percentage, which
	// is how a fixed escalation clause is written.
	RevisionPercentage = "percentage"
	// RevisionIndex restates payments by the ratio of two index readings, which
	// is how a CPI clause is written.
	RevisionIndex = "index"
	// RevisionStepped applies a dated ladder of amounts, which is how a lease
	// with pre-agreed increases is written.
	RevisionStepped = "stepped"
)

// StepChange is one rung of a stepped escalation: from FromDate onward the
// fixed payments become Amount.
type StepChange struct {
	FromDate time.Time `json:"from_date"`
	Amount   float64   `json:"amount"`
}

// PaymentRevision states, in the landlord's terms, how the rent changes. Only
// the fields belonging to Kind are read; the others are ignored.
type PaymentRevision struct {
	Kind string `json:"kind"`

	// AppliesFrom is the first payment date the revision reaches. When zero the
	// event's effective date is used, which is the common case.
	AppliesFrom time.Time `json:"applies_from"`
	// AppliesTo bounds a temporary revision, such as a rent-free or
	// reduced-rent concession. Zero means "to the end of the lease".
	AppliesTo time.Time `json:"applies_to"`

	// Amount is the new payment for RevisionSetAmount.
	Amount float64 `json:"amount"`

	// Percentage is the movement for RevisionPercentage, expressed as a
	// percentage: 5 means +5%, -10 means a 10% reduction.
	Percentage float64 `json:"percentage"`

	// BaseIndex and NewIndex are the two readings for RevisionIndex. Payments
	// are restated by NewIndex/BaseIndex.
	BaseIndex float64 `json:"base_index"`
	NewIndex  float64 `json:"new_index"`
	// CapPercentage and FloorPercentage bound an indexed movement, which retail
	// leases almost always do. Zero means unbounded on that side.
	CapPercentage   float64 `json:"cap_percentage"`
	FloorPercentage float64 `json:"floor_percentage"`

	// Steps is the ladder for RevisionStepped, in any order.
	Steps []StepChange `json:"steps"`
}

// PaymentChange is one line of the derived draft, kept alongside its original
// so the person confirming it can see what moved rather than only the result.
type PaymentChange struct {
	Date           time.Time `json:"date"`
	OriginalAmount float64   `json:"original_amount"`
	RevisedAmount  float64   `json:"revised_amount"`
	Delta          float64   `json:"delta"`
	Timing         string    `json:"timing"`
	Type           string    `json:"type"`
	// Changed is false for payments the revision left alone. They are still
	// returned, because a draft that hides the untouched lines invites the
	// reader to assume they were deleted.
	Changed bool `json:"changed"`
}

// RevisedSchedule is the draft a user confirms before it is committed.
type RevisedSchedule struct {
	Changes []PaymentChange `json:"changes"`
	// OriginalTotal and RevisedTotal cover only the payments on or after the
	// revision start, since that is the span the event can affect.
	OriginalTotal float64 `json:"original_total"`
	RevisedTotal  float64 `json:"revised_total"`
	Delta         float64 `json:"delta"`
	ChangedCount  int     `json:"changed_count"`
	// AppliedFactor is the multiplier a percentage or index revision worked out
	// to, after any cap or floor. It is reported because "CPI gave 2.6% but the
	// 2% cap applied" is exactly the sentence an auditor asks for.
	AppliedFactor float64 `json:"applied_factor"`
	// CapApplied and FloorApplied say whether a bound bit.
	CapApplied   bool `json:"cap_applied"`
	FloorApplied bool `json:"floor_applied"`
}

// Payments returns the revised schedule as the measurement engine consumes it.
func (s RevisedSchedule) Payments() []LeasePayment {
	payments := make([]LeasePayment, 0, len(s.Changes))
	for _, change := range s.Changes {
		payments = append(payments, LeasePayment{
			Date:   change.Date,
			Amount: change.RevisedAmount,
			Timing: change.Timing,
			Type:   change.Type,
		})
	}
	return payments
}

// revisable reports whether a payment is one the rent terms reach. Variable and
// non-lease components are excluded: an escalation clause restates the rent, not
// the turnover rent or the service charge, and quietly inflating those would
// overstate the liability.
func revisable(payment LeasePayment) bool {
	return payment.Type != "variable" && payment.Type != "non_lease"
}

// DeriveRevisedPayments produces the revised payment draft from the original
// schedule and the stated terms.
func DeriveRevisedPayments(original []LeasePayment, revision PaymentRevision, effectiveDate time.Time) (RevisedSchedule, error) {
	from := revision.AppliesFrom
	if from.IsZero() {
		from = effectiveDate
	}
	if !revision.AppliesTo.IsZero() && revision.AppliesTo.Before(from) {
		return RevisedSchedule{}, fmt.Errorf("revision ends %s, before it starts %s",
			revision.AppliesTo.Format("2006-01-02"), from.Format("2006-01-02"))
	}

	factor, capped, floored, err := revisionFactor(revision)
	if err != nil {
		return RevisedSchedule{}, err
	}
	steps, err := sortedSteps(revision)
	if err != nil {
		return RevisedSchedule{}, err
	}

	schedule := RevisedSchedule{
		Changes:       make([]PaymentChange, 0, len(original)),
		AppliedFactor: factor,
		CapApplied:    capped,
		FloorApplied:  floored,
	}

	for _, payment := range original {
		change := PaymentChange{
			Date:           payment.Date,
			OriginalAmount: payment.Amount,
			RevisedAmount:  payment.Amount,
			Timing:         payment.Timing,
			Type:           payment.Type,
		}

		inWindow := !payment.Date.Before(from) &&
			(revision.AppliesTo.IsZero() || !payment.Date.After(revision.AppliesTo))
		if inWindow && revisable(payment) {
			revised, err := reviseAmount(payment, revision, factor, steps)
			if err != nil {
				return RevisedSchedule{}, err
			}
			change.RevisedAmount = round(revised)
			change.Delta = round(change.RevisedAmount - change.OriginalAmount)
			change.Changed = math.Abs(change.Delta) > materialityRounding
		}

		if !payment.Date.Before(from) {
			schedule.OriginalTotal += change.OriginalAmount
			schedule.RevisedTotal += change.RevisedAmount
		}
		if change.Changed {
			schedule.ChangedCount++
		}
		schedule.Changes = append(schedule.Changes, change)
	}

	schedule.OriginalTotal = round(schedule.OriginalTotal)
	schedule.RevisedTotal = round(schedule.RevisedTotal)
	schedule.Delta = round(schedule.RevisedTotal - schedule.OriginalTotal)
	return schedule, nil
}

// reviseAmount restates one payment under the stated terms.
func reviseAmount(payment LeasePayment, revision PaymentRevision, factor float64, steps []StepChange) (float64, error) {
	switch revision.Kind {
	case RevisionSetAmount:
		return revision.Amount, nil
	case RevisionPercentage, RevisionIndex:
		return payment.Amount * factor, nil
	case RevisionStepped:
		// The rung in force is the last one that has already started. A payment
		// before the first rung keeps its original amount.
		amount := payment.Amount
		for _, step := range steps {
			if !payment.Date.Before(step.FromDate) {
				amount = step.Amount
			}
		}
		return amount, nil
	default:
		return 0, fmt.Errorf("unknown revision kind %q", revision.Kind)
	}
}

// revisionFactor works out the multiplier for the proportional kinds, and
// reports whether a cap or floor changed it.
func revisionFactor(revision PaymentRevision) (factor float64, capped, floored bool, err error) {
	switch revision.Kind {
	case RevisionSetAmount:
		if revision.Amount < 0 {
			return 0, false, false, fmt.Errorf("revised rent cannot be negative")
		}
		return 1, false, false, nil

	case RevisionPercentage:
		if revision.Percentage <= -100 {
			return 0, false, false, fmt.Errorf("a reduction of %.2f%% would take the rent to zero or below", revision.Percentage)
		}
		return 1 + revision.Percentage/100, false, false, nil

	case RevisionIndex:
		if revision.BaseIndex <= 0 || revision.NewIndex <= 0 {
			return 0, false, false, fmt.Errorf("both index readings are required and must be positive")
		}
		factor = revision.NewIndex / revision.BaseIndex
		// A cap or floor is stated as a percentage movement, so it is compared
		// against the movement rather than against the factor.
		movement := (factor - 1) * 100
		if revision.CapPercentage != 0 && movement > revision.CapPercentage {
			factor = 1 + revision.CapPercentage/100
			capped = true
		}
		if revision.FloorPercentage != 0 && movement < revision.FloorPercentage {
			factor = 1 + revision.FloorPercentage/100
			floored = true
		}
		if factor <= 0 {
			return 0, false, false, fmt.Errorf("the stated index movement would take the rent to zero or below")
		}
		return factor, capped, floored, nil

	case RevisionStepped:
		return 1, false, false, nil

	default:
		return 0, false, false, fmt.Errorf("unknown revision kind %q", revision.Kind)
	}
}

func sortedSteps(revision PaymentRevision) ([]StepChange, error) {
	if revision.Kind != RevisionStepped {
		return nil, nil
	}
	if len(revision.Steps) == 0 {
		return nil, fmt.Errorf("a stepped revision needs at least one step")
	}
	steps := make([]StepChange, len(revision.Steps))
	copy(steps, revision.Steps)
	for _, step := range steps {
		if step.Amount < 0 {
			return nil, fmt.Errorf("step amount cannot be negative")
		}
		if step.FromDate.IsZero() {
			return nil, fmt.Errorf("every step needs a start date")
		}
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].FromDate.Before(steps[j].FromDate) })
	return steps, nil
}
