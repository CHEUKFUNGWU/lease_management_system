package aiintake

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// DecodePaymentSchedule is the core side of the versioned AI intake seam.
// It rejects incompatible or unsafe payloads before handlers can build import
// actions from them.
func DecodePaymentSchedule(reader io.Reader) (*PaymentScheduleDraft, error) {
	var draft PaymentScheduleDraft
	if err := json.NewDecoder(reader).Decode(&draft); err != nil {
		return nil, fmt.Errorf("decode AI intake payment schedule: %w", err)
	}
	if err := validatePaymentScheduleDraft(&draft); err != nil {
		return nil, err
	}
	return &draft, nil
}

func validatePaymentScheduleDraft(draft *PaymentScheduleDraft) error {
	if draft.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported AI intake schema version %q", draft.SchemaVersion)
	}
	if draft.IntakeID == "" || draft.TaskID == "" || draft.FileID == "" {
		return fmt.Errorf("AI intake identity is incomplete")
	}
	if draft.Mode != "assist" {
		return fmt.Errorf("AI intake mode %q is not allowed", draft.Mode)
	}
	if draft.DraftType != "payment_schedule_draft" {
		return fmt.Errorf("unexpected AI intake draft type %q", draft.DraftType)
	}
	if draft.Status != "draft_generated" {
		return fmt.Errorf("unexpected AI intake status %q", draft.Status)
	}
	if draft.Evidence.SourceFileID != draft.FileID {
		return fmt.Errorf("AI intake evidence does not match source file")
	}
	if draft.Evidence.ObjectName == "" || draft.Evidence.ContentType == "" {
		return fmt.Errorf("AI intake source evidence is incomplete")
	}
	if draft.Evidence.Complete && len(draft.Evidence.Locators) == 0 {
		return fmt.Errorf("AI intake evidence is marked complete without locators")
	}
	if !draft.Evidence.Complete && draft.Evidence.MissingReason == "" {
		return fmt.Errorf("incomplete AI intake evidence requires a reason")
	}
	if !draft.ReviewGate.Required {
		return fmt.Errorf("Assist Mode AI intake must require human review")
	}
	if !contains(draft.ReviewGate.Reasons, "assist_mode") {
		return fmt.Errorf("Assist Mode AI intake review reasons must include assist_mode")
	}
	if draft.ReviewGate.ConfidenceThreshold <= 0 || draft.ReviewGate.ConfidenceThreshold > 1 {
		return fmt.Errorf("invalid AI intake confidence threshold %v", draft.ReviewGate.ConfidenceThreshold)
	}
	if _, ok := draft.ConfidenceScores["overall"]; !ok {
		return fmt.Errorf("AI intake confidence scores must include overall")
	}
	for name, confidence := range draft.ConfidenceScores {
		if confidence < 0 || confidence > 1 {
			return fmt.Errorf("confidence score %q is outside [0,1]", name)
		}
	}
	for index := range draft.Schedules {
		if err := validatePaymentSchedule(index, &draft.Schedules[index]); err != nil {
			return err
		}
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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
