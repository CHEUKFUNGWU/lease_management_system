package aiintake

import (
	"os"
	"testing"
)

func TestDecodeContractBatchV1Contract(t *testing.T) {
	fixture, err := os.Open("../../../contracts/ai-intake.v1/contract-batch.json")
	if err != nil {
		t.Fatalf("open contract batch fixture: %v", err)
	}
	defer fixture.Close()

	draft, err := DecodeContractBatch(fixture)
	if err != nil {
		t.Fatalf("DecodeContractBatch(): %v", err)
	}
	if draft.SchemaVersion != SchemaVersion || draft.DraftType != "contract_batch_draft" {
		t.Fatalf("version metadata = %#v", draft.IntakeMetadata)
	}
	if draft.TotalCount != 1 || len(draft.Contracts) != 1 {
		t.Fatalf("batch counts = total %d, contracts %d", draft.TotalCount, len(draft.Contracts))
	}
	if draft.Contracts[0].ContractNumber != "LEASE-BATCH-001" {
		t.Fatalf("contract = %#v", draft.Contracts[0])
	}
	if !draft.Evidence.Complete || draft.Evidence.Locators[0].Source != "Leases!A2:Z2" {
		t.Fatalf("evidence = %#v", draft.Evidence)
	}
}
