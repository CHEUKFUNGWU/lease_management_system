package aiintake

import (
	"fmt"
	"io"
	"time"
)

// DecodePaymentSchedule is the core side of the versioned AI intake seam.
// It rejects incompatible or unsafe payloads before handlers can build import
// actions from them.
func DecodePaymentSchedule(reader io.Reader) (*PaymentScheduleDraft, error) {
	var draft PaymentScheduleDraft
	if err := decodeJSON(reader, &draft); err != nil {
		return nil, fmt.Errorf("decode AI intake payment schedule: %w", err)
	}
	if err := validatePaymentScheduleDraft(&draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

func validatePaymentScheduleDraft(draft *PaymentScheduleDraft) error {
	if err := validateMetadata(&draft.IntakeMetadata, "payment_schedule_draft"); err != nil {
		return err
	}
	if draft.Evidence.Complete {
		if err := validateRecordEvidence(&draft.Evidence, "schedules", len(draft.Schedules)); err != nil {
			return err
		}
	}
	for index := range draft.Schedules {
		if err := validatePaymentSchedule(index, &draft.Schedules[index]); err != nil {
			return err
		}
	}
	return nil
}

func validatePaymentSchedule(index int, schedule *PaymentSchedule) error {
	if schedule.Amount <= 0 {
		return fmt.Errorf("payment schedule %d has non-positive amount", index)
	}
	if schedule.PaymentTiming != "prepaid" && schedule.PaymentTiming != "postpaid" {
		return fmt.Errorf("payment schedule %d has invalid payment timing %q", index, schedule.PaymentTiming)
	}
	for field, value := range map[string]string{
		"period_start": schedule.PeriodStart,
		"period_end":   schedule.PeriodEnd,
		"due_date":     schedule.DueDate,
	} {
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("payment schedule %d has invalid %s: %w", index, field, err)
		}
	}
	if schedule.Confidence < 0 || schedule.Confidence > 1 {
		return fmt.Errorf("payment schedule %d confidence is outside [0,1]", index)
	}
	return nil
}
