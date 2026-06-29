package eventaccounting

import (
	"fmt"
	"math"
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
	NewValue         *string
	Currency         string
	DiscountRate     float64
	Payments         []ifrs16.LeasePayment
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
	calculation := ifrs16.LeaseCalculation{
		CommencementDate: input.CommencementDate,
		LeaseEndDate:     input.LeaseEndDate,
		DiscountRate:     input.DiscountRate,
		Payments:         input.Payments,
		PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
			CommencementDate: input.CommencementDate,
			Payments:         input.Payments,
		}),
	}
	liabilityBefore, rouBefore, err := ifrs16.GetCarryingAmount(calculation, input.EffectiveDate)
	if err != nil {
		return Result{}, fmt.Errorf("calculate carrying amount: %w", err)
	}

	remeasurement, err := ifrs16.RecalculateFromDate(liabilityBefore, rouBefore, ifrs16.RemeasurementInput{
		EffectiveDate:       input.EffectiveDate,
		LeaseEndDate:        leaseEndDate,
		RevisedDiscountRate: input.DiscountRate,
		RevisedPayments:     paymentsThrough(input.Payments, leaseEndDate),
	})
	if err != nil {
		return Result{}, fmt.Errorf("remeasure lease: %w", err)
	}

	treatment := Classify(input.EventType)
	adjustment := Adjustment{
		EventID: input.EventID, ContractID: input.ContractID, Treatment: treatment,
		EffectiveDate: input.EffectiveDate, LiabilityBefore: liabilityBefore,
		LiabilityAfter: remeasurement.NewLiability, LiabilityAdjustment: remeasurement.LiabilityDelta,
		ROUBefore: rouBefore, ROUAfter: remeasurement.NewROU,
		ROUAdjustment: remeasurement.ROUAdjustment, PnLGain: remeasurement.PnLGain,
		PnLLoss: remeasurement.PnLLoss, RevisedDiscountRate: input.DiscountRate,
	}

	return Result{
		Treatment: treatment, LeaseEndDate: leaseEndDate, Adjustment: adjustment,
		ForwardSchedule: remeasurement.ForwardSchedule,
		JournalEntries:  buildJournalEntries(input, adjustment),
	}, nil
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
	case "area_adjustment", "rent_change", "index_update", "discount_rate_change":
		return "modification"
	case "renewal", "early_termination":
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
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "2801-租赁负债", "1701-使用权资产", math.Abs(adjustment.LiabilityAdjustment)
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
		entry.DebitAccount, entry.CreditAccount, entry.Amount = "6711-资产处置损失", "2801-租赁负债", adjustment.PnLLoss
		entry.Description += " (处置损失)"
		entries = append(entries, entry)
	}
	return entries
}
