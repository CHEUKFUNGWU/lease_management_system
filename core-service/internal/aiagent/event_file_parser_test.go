package aiagent

import (
	"context"
	"testing"
)

// W5-3: the event parse runs through the in-process producer (Assist Mode only
// — a reviewable draft with a wajib human gate, never an approved event).

func TestParseEventUsesInProcessProducer(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "event-modification")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"event/2026/08/event-001.pdf": []byte(text)})

	result, err := agent.parseEvent(context.Background(), "Bearer test-token", "event-001", "event/2026/08/event-001.pdf", ct, "contract-001")
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if result.Event.EventType != "area_adjustment" {
		t.Fatalf("event = %+v", result.Event)
	}
	if result.Confidence < 0 || len(result.MissingFields) != 0 {
		t.Fatalf("review metadata = %+v", result)
	}
	// The event draft must carry the unconditional Assist-Mode review gate.
	if len(result.ReviewPrompts) == 0 {
		t.Fatal("event draft must carry review prompts")
	}
}
