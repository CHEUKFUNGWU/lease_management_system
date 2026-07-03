package aiintake

import (
	"os"
	"strings"
	"testing"
)

func TestDecodePaymentScheduleV1Contract(t *testing.T) {
	fixture, err := os.Open("../../../contracts/ai-intake.v1/payment-schedule.json")
	if err != nil {
		t.Fatalf("open contract fixture: %v", err)
	}
	defer fixture.Close()

	draft, err := DecodePaymentSchedule(fixture)
	if err != nil {
		t.Fatalf("DecodePaymentSchedule(): %v", err)
	}
	if draft.SchemaVersion != SchemaVersion || draft.IntakeID != "intake-fixture-001" {
		t.Fatalf("version metadata = %#v", draft)
	}
	if len(draft.Schedules) != 1 || draft.Schedules[0].Amount != 1000 || draft.Schedules[0].PaymentTiming != "postpaid" {
		t.Fatalf("payment schedules = %#v", draft.Schedules)
	}
	if !draft.Evidence.Complete || len(draft.Evidence.Locators) != 1 || draft.Evidence.Locators[0].Source != "Sheet1!D2" {
		t.Fatalf("evidence = %#v", draft.Evidence)
	}
	if !draft.ReviewGate.Required || len(draft.ReviewGate.Reasons) == 0 {
		t.Fatalf("review gate = %#v", draft.ReviewGate)
	}
}

func TestDecodePaymentScheduleRejectsUnknownContractVersion(t *testing.T) {
	payload := `{
		"schema_version":"ai-intake.v2",
		"intake_id":"intake-1",
		"file_id":"file-1",
		"mode":"assist",
		"draft_type":"payment_schedule_draft",
		"review_gate":{"required":true,"reasons":["assist_mode"],"confidence_threshold":0.8},
		"evidence":{"source_file_id":"file-1","complete":false,"missing_reason":"not_available"},
		"schedules":[]
	}`

	_, err := DecodePaymentSchedule(strings.NewReader(payload))
	if err == nil || !strings.Contains(err.Error(), "unsupported AI intake schema version") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodePaymentScheduleRejectsUnsafePolicyMetadata(t *testing.T) {
	fixture, err := os.ReadFile("../../../contracts/ai-intake.v1/payment-schedule.json")
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}

	tests := []struct {
		name        string
		payload     string
		errorSubstr string
	}{
		{
			name:        "review gate disabled",
			payload:     strings.Replace(string(fixture), `"required": true`, `"required": false`, 1),
			errorSubstr: "must require human review",
		},
		{
			name:        "assist reason absent",
			payload:     strings.Replace(string(fixture), `"assist_mode"`, `"manual_policy"`, 1),
			errorSubstr: "must include assist_mode",
		},
		{
			name:        "overall confidence absent",
			payload:     strings.Replace(string(fixture), `"overall": 0.92,`, ``, 1),
			errorSubstr: "must include overall",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodePaymentSchedule(strings.NewReader(test.payload))
			if err == nil || !strings.Contains(err.Error(), test.errorSubstr) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
