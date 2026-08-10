package aiintake

import (
	"fmt"
	"io"
	"time"
)

// DecodeEvent validates a document-derived event draft. It intentionally
// allows missing business fields because the result is a human-review draft;
// it never upgrades an incomplete document into a writable business event.
func DecodeEvent(reader io.Reader) (*EventDraft, error) {
	var draft EventDraft
	if err := decodeJSON(reader, &draft); err != nil {
		return nil, fmt.Errorf("decode AI intake event: %w", err)
	}
	if err := validateMetadata(&draft.IntakeMetadata, "event_draft"); err != nil {
		return nil, err
	}
	if draft.Event.EffectiveDate != "" {
		if _, err := time.Parse("2006-01-02", draft.Event.EffectiveDate); err != nil {
			return nil, fmt.Errorf("event draft has invalid effective_date: %w", err)
		}
	}
	for field, value := range map[string]string{
		"event_type":     draft.Event.EventType,
		"change_reason":  draft.Event.ChangeReason,
		"judgment_basis": draft.Event.JudgmentBasis,
	} {
		if value == "" {
			continue
		}
		if len([]rune(value)) > 5000 {
			return nil, fmt.Errorf("event draft %s is too long", field)
		}
	}
	for field, confidence := range draft.Event.FieldConfidence {
		if confidence < 0 || confidence > 1 {
			return nil, fmt.Errorf("event draft confidence %q is outside [0,1]", field)
		}
	}
	return &draft, nil
}
