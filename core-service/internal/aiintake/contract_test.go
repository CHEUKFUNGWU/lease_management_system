package aiintake

import (
	"os"
	"testing"
)

func TestDecodeContractV1Contract(t *testing.T) {
	fixture, err := os.Open("../../../contracts/ai-intake.v1/contract.json")
	if err != nil {
		t.Fatalf("open contract fixture: %v", err)
	}
	defer fixture.Close()

	draft, err := DecodeContract(fixture)
	if err != nil {
		t.Fatalf("DecodeContract(): %v", err)
	}
	if draft.SchemaVersion != SchemaVersion || draft.DraftType != "contract_draft" {
		t.Fatalf("version metadata = %#v", draft.IntakeMetadata)
	}
	if draft.ExtractedData.ContractNumber != "LEASE-001" || draft.ExtractedData.LeaseScope != "in_scope" {
		t.Fatalf("contract draft = %#v", draft.ExtractedData)
	}
	if draft.Evidence.Complete || draft.Evidence.MissingReason == "" {
		t.Fatalf("evidence = %#v", draft.Evidence)
	}
	if !draft.ReviewGate.Required || !contains(draft.ReviewGate.Reasons, "missing_fields") {
		t.Fatalf("review gate = %#v", draft.ReviewGate)
	}
}
