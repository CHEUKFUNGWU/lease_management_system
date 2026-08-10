package aiagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseEventUsesVersionedAssistIntake(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "event.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read event fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/parse/event" {
			t.Errorf("unexpected AI service path %q", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	result, err := (&Agent{}).parseEvent(context.Background(), "Bearer test-token", "event-001", "event/2026/08/event-001.pdf", "application/pdf", "contract-001")
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if result.Event.EventType != "modification" || result.Event.ContractID != "contract-001" {
		t.Fatalf("event = %+v", result.Event)
	}
	if result.Confidence != 0.62 || len(result.MissingFields) == 0 || len(result.EvidenceRefs) != 1 {
		t.Fatalf("review metadata = %+v", result)
	}
}
