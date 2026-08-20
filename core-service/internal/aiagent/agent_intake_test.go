package aiagent

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/aiintake"
)

// W5-3: the parse endpoints now run the in-process producer. Each test drives
// the new path with a CORR-2-recorded input (file bytes + LLM response) and
// asserts the decoded typed draft — GUARD-001 evidence that the replacement
// actually works, not just that the Python hop is gone.

func TestParseFileUsesInProcessProducer(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "contract-full")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"lease-contract.pdf": []byte(text)})

	draft, err := agent.parseFile(context.Background(), "Bearer test-token", "file-contract-001", "lease-contract.pdf", ct)
	if err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if draft.ExtractedData.ContractNumber != "LEASE-CORR2-001" || draft.ExtractedData.Currency != "CNY" {
		t.Fatalf("in-process producer draft = %#v", draft.ExtractedData)
	}
	if draft.Evidence.Complete || !draft.ReviewGate.Required {
		t.Fatalf("unsafe intake metadata: %#v", draft.IntakeMetadata)
	}
	if len(draft.ReviewGate.Reasons) == 0 {
		t.Fatal("review gate must carry reasons")
	}
}

func TestParseFileNoDiscountRateIsIndependentlyFlagged(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "contract-no-discount-rate")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"no-rate.pdf": []byte(text)})

	draft, err := agent.parseFile(context.Background(), "Bearer t", "file-002", "no-rate.pdf", ct)
	if err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	found := false
	for _, m := range draft.MissingFields {
		if m == "discount_rate" {
			found = true
		}
	}
	if !found {
		t.Fatalf("discount_rate must be flagged as missing: %v", draft.MissingFields)
	}
	if draft.ReviewGate.Reasons == nil {
		t.Fatalf("review gate must be present")
	}
	if !draft.RequiresHumanConfirmation {
		t.Fatal("Assist Mode draft must require human confirmation")
	}
}

func TestParsePaymentScheduleUsesInProcessProducer(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "payment-full")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"rent-schedule.pdf": []byte(text)})

	result, err := agent.parsePaymentSchedule(context.Background(), "Bearer test-token", "file-001", "rent-schedule.pdf", ct, "contract-001")
	if err != nil {
		t.Fatalf("parse payment schedule: %v", err)
	}
	if len(result.Schedules) != 2 || result.Schedules[0].Amount != 50000 {
		t.Fatalf("unexpected typed schedules: %+v", result.Schedules)
	}
	if result.Summary.SchemaVersion != "ai-intake.v1" {
		t.Fatalf("schema version = %q", result.Summary.SchemaVersion)
	}
	if !result.Summary.RequiresHumanConfirm {
		t.Fatalf("payment draft must require human confirmation: %+v", result.Summary)
	}
}

func TestParseContractBatchUsesInProcessProducer(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "batch-full")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"lease-ledger.pdf": []byte(text)})

	result, err := agent.parseContractBatch(context.Background(), "Bearer test-token", "file-batch-001", "lease-ledger.pdf", ct)
	if err != nil {
		t.Fatalf("parse contract batch: %v", err)
	}
	if len(result.Contracts) != 3 || result.Contracts[0].ContractNumber != "L-B1" {
		t.Fatalf("contracts = %#v", result.Contracts)
	}
	if result.Summary.SchemaVersion != "ai-intake.v1" || result.Summary.IntakeID == "" {
		t.Fatalf("intake trace = %#v", result.Summary)
	}
	if !result.Summary.RequiresHumanConfirm {
		t.Fatalf("batch draft must require human confirmation: %#v", result.Summary)
	}
}

// The source-identity guard still holds: a draft envelope claiming a different
// source must be rejected before any draft is surfaced (底1/底3).
func TestParseRejectsMismatchedSourceIdentity(t *testing.T) {
	meta := aiintake.IntakeMetadata{
		FileID:        "file-other",
		SchemaVersion: aiintake.SchemaVersion,
		Evidence: aiintake.Evidence{
			SourceFileID: "file-other", ObjectName: "other.pdf", ContentType: "application/pdf",
			Locators: []aiintake.EvidenceLocator{}, Complete: false, MissingReason: "x",
		},
	}
	if err := validateAIIntakeSource(meta, "file-001", "mine.pdf", "application/pdf"); err == nil {
		t.Fatal("mismatched source identity must be rejected")
	}
	if err := validateAIIntakeSource(meta, "file-other", "other.pdf", "application/pdf"); err != nil {
		t.Fatalf("matching identity must pass: %v", err)
	}
}
