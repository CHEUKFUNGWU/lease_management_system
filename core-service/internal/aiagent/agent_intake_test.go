package aiagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePaymentScheduleUsesVersionedIntakeContract(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "payment-schedule.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared AI intake fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/parse/payment-schedule" {
			t.Errorf("unexpected AI service path %q", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	agent := &Agent{}
	result, err := agent.parsePaymentSchedule(
		context.Background(),
		"Bearer test-token",
		"file-001",
		"rent-schedule.xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"contract-001",
	)
	if err != nil {
		t.Fatalf("parse payment schedule: %v", err)
	}

	if len(result.Schedules) != 1 || result.Schedules[0].Amount != 1000 {
		t.Fatalf("unexpected typed schedules: %+v", result.Schedules)
	}
	if result.Summary.SchemaVersion != "ai-intake.v1" {
		t.Fatalf("schema version = %q", result.Summary.SchemaVersion)
	}
	if result.Summary.IntakeID != "intake-fixture-001" {
		t.Fatalf("intake id = %q", result.Summary.IntakeID)
	}
	if !result.Summary.EvidenceComplete || !result.Summary.RequiresHumanConfirm {
		t.Fatalf("expected complete evidence and mandatory review: %+v", result.Summary)
	}
}

func TestParsePaymentScheduleRejectsMismatchedSourceIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"schema_version":"ai-intake.v1",
			"intake_id":"intake-mismatch",
			"task_id":"task_ps_other-file",
			"file_id":"other-file",
			"mode":"assist",
			"draft_type":"payment_schedule_draft",
			"status":"draft_generated",
			"schedules":[],
			"confidence_scores":{"overall":0},
			"missing_fields":["all"],
			"warnings":[],
			"requires_human_confirmation":true,
			"evidence":{"source_file_id":"other-file","object_name":"other.xlsx","content_type":"application/json","locators":[],"complete":false,"missing_reason":"not_available"},
			"review_gate":{"required":true,"reasons":["assist_mode"],"confidence_threshold":0.8}
		}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	agent := &Agent{}
	_, err := agent.parsePaymentSchedule(
		context.Background(),
		"Bearer test-token",
		"file-001",
		"rent-schedule.xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"contract-001",
	)
	if err == nil {
		t.Fatal("expected mismatched source identity to be rejected")
	}
}

func TestParseFileUsesVersionedContractIntake(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "contract.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared contract intake fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/parse/contract" {
			t.Errorf("unexpected AI service path %q", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	agent := &Agent{}
	draft, err := agent.parseFile(
		context.Background(),
		"Bearer test-token",
		"file-contract-001",
		"lease-contract.pdf",
		"application/pdf",
	)
	if err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if draft.ExtractedData.ContractNumber != "LEASE-001" {
		t.Fatalf("contract draft = %#v", draft.ExtractedData)
	}
	if draft.Evidence.Complete || !draft.ReviewGate.Required {
		t.Fatalf("unsafe intake metadata: %#v", draft.IntakeMetadata)
	}
}

func TestParseContractBatchUsesVersionedIntakeContract(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "contract-batch.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared contract batch fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/parse/contract-batch" {
			t.Errorf("unexpected AI service path %q", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	agent := &Agent{}
	result, err := agent.parseContractBatch(
		context.Background(),
		"Bearer test-token",
		"file-batch-001",
		"lease-ledger.xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	)
	if err != nil {
		t.Fatalf("parse contract batch: %v", err)
	}
	if len(result.Contracts) != 1 || result.Contracts[0].ContractNumber != "LEASE-BATCH-001" {
		t.Fatalf("contracts = %#v", result.Contracts)
	}
	if result.Summary.SchemaVersion != "ai-intake.v1" || result.Summary.IntakeID != "intake-batch-fixture-001" {
		t.Fatalf("intake trace = %#v", result.Summary)
	}
	if !result.Summary.EvidenceComplete || !result.Summary.RequiresHumanConfirm {
		t.Fatalf("unsafe batch metadata = %#v", result.Summary)
	}
}
