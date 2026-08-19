package main

import (
	"strings"
	"testing"
)

const eventsFixture = `{"run":{"id":"r1"},"events":[
	{"sequence_no":1,"event_type":"message_start","is_terminal":false,"payload":{"message":{"role":"user","content":"出一份经营底稿"}}},
	{"sequence_no":2,"event_type":"tool_end","is_terminal":false,"payload":[{"tool":"retail.working_paper.store.generate","status":"needs_review"}]},
	{"sequence_no":3,"event_type":"run_end","is_terminal":true,"payload":{"status":"waiting_review"}}
]}`

func TestFormatEventsNDJSON(t *testing.T) {
	out, err := formatEventsNDJSON([]byte(eventsFixture))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 NDJSON lines, got %d: %q", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], `{"sequence_no":1`) || !strings.Contains(lines[1], "tool_end") {
		t.Fatalf("unexpected NDJSON content: %q", out)
	}
}

func TestFormatEventsTable(t *testing.T) {
	out, err := formatEventsTable([]byte(eventsFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"NO", "EVENT", "TERMINAL", "message_start", "tool_end", "run_end", "✗"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table output must contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatEventsEmpty(t *testing.T) {
	out, err := formatEventsTable([]byte(`{"events":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "(no events)") {
		t.Fatalf("empty list must say so: %q", out)
	}
}
