package eventaccounting

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

const materialityThreshold = 0.01

type Input struct {
	EventID          string
	ContractID       string
	EventType        string
	EffectiveDate    time.Time
	CommencementDate time.Time
	LeaseEndDate     time.Time
	// NewValue is event-specific: revised end date for renewal/termination,
	// recurring fixed rent for rent_change, annual rate for
	// discount_rate_change, and post-impairment ROU carrying value for impairment.
	NewValue *string
	// Revision states the rent clause in structured form. When present it
	// derives the revised payment schedule, which is the only way clauses like
	// CPI indexation or a stepped ladder can be expressed. When absent the
	// engine falls back to NewValue, so events recorded before clauses existed
	// keep calculating exactly as they did.
	Revision     *PaymentRevision
	Currency     string
	DiscountRate float64
	Payments     []ifrs16.LeasePayment
}

type Adjustment struct {
	EventID             string
	ContractID          string
	Treatment           string
	EffectiveDate       time.Time
	LiabilityBefore     float64
	LiabilityAfter      float64
	LiabilityAdjustment float64
	ROUBefore           float64
	ROUAfter            float64
	ROUAdjustment       float64
	PnLGain             float64
	PnLLoss             float64
	RevisedDiscountRate float64
}

type JournalEntry struct {
	AccountingPeriod string
	EntryDate        time.Time
	EntryType        string
	DebitAccount     string
	CreditAccount    string
	Amount           float64
	Currency         string
	Description      string
}

type Result struct {
	Treatment       string
	LeaseEndDate    time.Time
	Adjustment      Adjustment
	ForwardSchedule []ifrs16.DailyEntry
	JournalEntries  []JournalEntry
}

// Calculate returns the complete accounting plan for an event. Preview callers
// serialize this result; commit callers persist the same result.
func Calculate(input Input) (Result, error) {
	leaseEndDate := revisedLeaseEndDate(input.EventType, input.NewValue, input.LeaseEndDate)
	revisedRate, err := revisedDiscountRate(input)
	if err != nil {
		return Result{}, err
	}
	originalPayments := paymentsThrough(input.Payments, input.LeaseEndDate)
	calculation := ifrs16.LeaseCalculation{
		CommencementDate: input.CommencementDate,
		LeaseEndDate:     input.LeaseEndDate,
		DiscountRate:     input.DiscountRate,
		Payments:         originalPayments,
		PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
			CommencementDate: input.CommencementDate,
			Payments:         originalPayments,
		}),
	}
	liabilityBefore, rouBefore, err := ifrs16.GetCarryingAmount(calculation, input.EffectiveDate)
	if err != nil {
		return Result{}, fmt.Errorf("calculate carrying amount: %w", err)
	}
	treatment := Classify(input.EventType)
	if treatment == "impairment" {
		return calculateImpairment(input, liabilityBefore, rouBefore)
	}
	revisedPayments, err := paymentsForEvent(input, leaseEndDate)
	if err != nil {
		return Result{}, err
	}

	remeasurement, err := ifrs16.RecalculateFromDate(liabilityBefore, rouBefore, ifrs16.RemeasurementInput{
		EffectiveDate:       input.EffectiveDate,
		LeaseEndDate:        leaseEndDate,
		RevisedDiscountRate: revisedRate,
		RevisedPayments:     revisedPayments,
		ScopeDecreaseProportion: scopeDecreaseProportion(
			input.EffectiveDate, input.LeaseEndDate, leaseEndDate),
	})
	if err != nil {
		return Result{}, fmt.Errorf("remeasure lease: %w", err)
	}

	adjustment := Adjustment{
		EventID: input.EventID, ContractID: input.ContractID, Treatment: treatment,
		EffectiveDate: input.EffectiveDate, LiabilityBefore: liabilityBefore,
		LiabilityAfter: remeasurement.NewLiability, LiabilityAdjustment: remeasurement.LiabilityDelta,
		ROUBefore: rouBefore, ROUAfter: remeasurement.NewROU,
		ROUAdjustment: remeasurement.ROUAdjustment, PnLGain: remeasurement.PnLGain,
		PnLLoss: remeasurement.PnLLoss, RevisedDiscountRate: revisedRate,
	}

	return Result{
		Treatment: treatment, LeaseEndDate: leaseEndDate, Adjustment: adjustment,
		ForwardSchedule: remeasurement.ForwardSchedule,
		JournalEntries:  buildJournalEntries(input, adjustment),
	}, nil
}

// scopeDecreaseProportion measures how much of the remaining lease is being
// given up, as a share of the term that was left on the effective date. A
// shortened term is the scope decrease this engine can see; an area reduction
// would be another, but the contract's area is not part of this input, so it is
// deliberately not guessed at here.
//
// Returning zero means "not a scope decrease", which keeps every other
// remeasurement on the treatment it has always had.
func scopeDecreaseProportion(effectiveDate, originalEnd, revisedEnd time.Time) float64 {
	if !revisedEnd.Before(originalEnd) {
		return 0
	}
	originalRemaining := originalEnd.Sub(effectiveDate).Hours()
	if originalRemaining <= 0 {
		return 0
	}
	revisedRemaining := revisedEnd.Sub(effectiveDate).Hours()
	if revisedRemaining < 0 {
		// The lease ends on or before the effective date: it is given up whole.
		revisedRemaining = 0
	}
	return (originalRemaining - revisedRemaining) / originalRemaining
}

func paymentsForEvent(input Input, leaseEndDate time.Time) ([]ifrs16.LeasePayment, error) {
	payments := paymentsThrough(input.Payments, leaseEndDate)

	// A stated clause always wins: it is the more precise description of what
	// the landlord's notice said, and it applies to any event type rather than
	// only to rent_change.
	if input.Revision != nil {
		schedule, err := DeriveRevisedPayments(payments, *input.Revision, input.EffectiveDate)
		if err != nil {
			return nil, err
		}
		return schedule.Payments(), nil
	}

	if input.EventType != "rent_change" {
		return payments, nil
	}
	if input.NewValue == nil {
		return nil, fmt.Errorf("revised rent is required")
	}
	revisedRent, err := strconv.ParseFloat(*input.NewValue, 64)
	if err != nil || revisedRent < 0 {
		return nil, fmt.Errorf("invalid revised rent %q", *input.NewValue)
	}
	for index := range payments {
		payment := &payments[index]
		if !payment.Date.Before(input.EffectiveDate) && payment.Type != "variable" && payment.Type != "non_lease" {
			payment.Amount = revisedRent
		}
	}
	return payments, nil
}

func calculateImpairment(input Input, liabilityBefore, rouBefore float64) (Result, error) {
	if input.NewValue == nil {
		return Result{}, fmt.Errorf("post-impairment ROU value is required")
	}
	rouAfter, err := strconv.ParseFloat(*input.NewValue, 64)
	if err != nil || rouAfter < 0 || rouAfter >= rouBefore {
		return Result{}, fmt.Errorf("invalid post-impairment ROU value %q", *input.NewValue)
	}
	loss := rouBefore - rouAfter
	adjustment := Adjustment{
		EventID: input.EventID, ContractID: input.ContractID, Treatment: "impairment",
		EffectiveDate: input.EffectiveDate, LiabilityBefore: liabilityBefore,
		LiabilityAfter: liabilityBefore, LiabilityAdjustment: 0,
		ROUBefore: rouBefore, ROUAfter: rouAfter, ROUAdjustment: -loss,
		PnLLoss: loss, RevisedDiscountRate: input.DiscountRate,
	}
	return Result{
		Treatment: "impairment", LeaseEndDate: input.LeaseEndDate, Adjustment: adjustment,
		ForwardSchedule: ifrs16.GenerateForwardSchedule(
			input.EffectiveDate, input.LeaseEndDate, liabilityBefore, rouAfter,
			input.DiscountRate, paymentsThrough(input.Payments, input.LeaseEndDate), input.EffectiveDate,
		),
		JournalEntries: buildJournalEntries(input, adjustment),
	}, nil
}

func revisedDiscountRate(input Input) (float64, error) {
	if input.EventType != "discount_rate_change" || input.NewValue == nil {
		return input.DiscountRate, nil
	}
	rate, err := strconv.ParseFloat(*input.NewValue, 64)
	if err != nil || rate <= 0 {
		return 0, fmt.Errorf("invalid revised discount rate %q", *input.NewValue)
	}
	if rate > 1 {
		rate /= 100
	}
	return rate, nil
}

func paymentsThrough(payments []ifrs16.LeasePayment, leaseEndDate time.Time) []ifrs16.LeasePayment {
	result := make([]ifrs16.LeasePayment, 0, len(payments))
	for _, payment := range payments {
		if !payment.Date.After(leaseEndDate) {
			result = append(result, payment)
		}
	}
	return result
}

func Classify(eventType string) string {
	switch eventType {
	case "area_adjustment", "rent_change":
		return "modification"
	case "renewal", "early_termination", "index_update", "discount_rate_change":
		return "reassessment"
	case "impairment":
		return "impairment"
	default:
		return "modification"
	}
}

func revisedLeaseEndDate(eventType string, newValue *string, current time.Time) time.Time {
	if (eventType == "early_termination" || eventType == "renewal") && newValue != nil {
		if parsed, err := time.Parse("2006-01-02", *newValue); err == nil {
			return parsed
		}
	}
	return current
}

func buildJournalEntries(input Input, adjustment Adjustment) []JournalEntry {
	period := input.EffectiveDate.Format("2006-01")
	entryDate := input.EffectiveDate.AddDate(0, 1, -1)
	base := JournalEntry{AccountingPeriod: period, EntryDate: entryDate, EntryType: adjustment.Treatment, Currency: input.Currency}
	shortID := input.EventID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	if adjustment.Treatment == "impairment" {
		amount := adjustment.ROUBefore - adjustment.ROUAfter
		if amount <= materialityThreshold {
			return nil
		}
		base.DebitAccount, base.CreditAccount = "6701-资产减值损失", "1702-使用权资产减值准备"
		base.Amount, base.Description = amount, fmt.Sprintf("使用权资产减值 - 事件 %s", shortID)
		return []JournalEntry{base}
	}

	description := "租赁修改调整"
	if adjustment.Treatment == "reassessment" {
		description = "租赁重新评估"
	}
	base.Description = fmt.Sprintf("%s - 事件 %s", description, shortID)
	entries := make([]JournalEntry, 0, 3)
	if adjustment.LiabilityAdjustment > materialityThreshold {
		entry := base
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "1701-使用权资产", "2801-租赁负债", adjustment.LiabilityAdjustment
		entries = append(entries, entry)
	} else if adjustment.LiabilityAdjustment < -materialityThreshold {
		entry := base
		rouReduction := math.Abs(adjustment.ROUAdjustment)
		if rouReduction > math.Abs(adjustment.LiabilityAdjustment) {
			rouReduction = math.Abs(adjustment.LiabilityAdjustment)
		}
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "2801-租赁负债", "1701-使用权资产", rouReduction
		entry.Description += " (负债减少)"
		entries = append(entries, entry)
	}
	if adjustment.PnLGain > materialityThreshold {
		entry := base
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "2801-租赁负债", "6301-资产处置收益", adjustment.PnLGain
		entry.Description += " (处置收益)"
		entries = append(entries, entry)
	}
	if adjustment.PnLLoss > materialityThreshold {
		entry := base
		// A loss on a scope decrease is the asset written off beyond what the
		// liability release absorbed, so it is credited against the right-of-use
		// asset. Crediting the liability — which the entry above has already
		// released in full — would leave the ledger unbalanced by the loss.
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "6711-资产处置损失", "1701-使用权资产", adjustment.PnLLoss
		entry.Description += " (处置损失)"
		entries = append(entries, entry)
	}
	return entries
}
