package aiintake

import (
	"fmt"
	"io"
)

// DecodeContractBatch validates a batch contract draft at the versioned AI intake seam.
func DecodeContractBatch(reader io.Reader) (*ContractBatchDraft, error) {
	var draft ContractBatchDraft
	if err := decodeJSON(reader, &draft); err != nil {
		return nil, fmt.Errorf("decode AI intake contract batch: %w", err)
	}
	if err := validateMetadata(&draft.IntakeMetadata, "contract_batch_draft"); err != nil {
		return nil, err
	}
	if draft.TotalCount != len(draft.Contracts) {
		return nil, fmt.Errorf("contract batch total_count %d does not match %d contracts", draft.TotalCount, len(draft.Contracts))
	}
	if draft.Evidence.Complete {
		if err := validateRecordEvidence(&draft.Evidence, "contracts", len(draft.Contracts)); err != nil {
			return nil, err
		}
	}
	for index := range draft.Contracts {
		contract := &draft.Contracts[index]
		if contract.ContractNumber == "" || contract.Lessee == "" || contract.Lessor == "" {
			return nil, fmt.Errorf("contract batch item %d is missing its contract number, lessee, or lessor", index)
		}
		if err := validateContractDraftData(contract); err != nil {
			return nil, fmt.Errorf("contract batch item %d: %w", index, err)
		}
	}
	return &draft, nil
}
