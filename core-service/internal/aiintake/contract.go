package aiintake

import (
	"fmt"
	"io"
	"slices"
	"time"
)

// DecodeContract validates a single contract draft at the versioned AI intake seam.
func DecodeContract(reader io.Reader) (*ContractDraft, error) {
	var draft ContractDraft
	if err := decodeJSON(reader, &draft); err != nil {
		return nil, fmt.Errorf("decode AI intake contract: %w", err)
	}
	if err := validateMetadata(&draft.IntakeMetadata, "contract_draft"); err != nil {
		return nil, err
	}
	if draft.Evidence.Complete {
		if err := validateRecordEvidence(&draft.Evidence, "extracted_data", 1); err != nil {
			return nil, err
		}
	}
	if err := validateContractDraftData(&draft.ExtractedData); err != nil {
		return nil, err
	}
	return &draft, nil
}

func validateContractDraftData(contract *ContractDraftData) error {
	for field, value := range map[string]string{
		"commencement_date": contract.CommencementDate,
		"lease_start_date":  contract.LeaseStartDate,
		"lease_end_date":    contract.LeaseEndDate,
	} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("contract draft has invalid %s: %w", field, err)
		}
	}
	if contract.PaymentTiming != "" && contract.PaymentTiming != "prepaid" && contract.PaymentTiming != "postpaid" {
		return fmt.Errorf("contract draft has invalid payment timing %q", contract.PaymentTiming)
	}
	if contract.LeaseScope != "" && !slices.Contains([]string{"in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease"}, contract.LeaseScope) {
		return fmt.Errorf("contract draft has invalid lease scope %q", contract.LeaseScope)
	}
	if contract.SuggestedScope != "" && !slices.Contains([]string{"in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease"}, contract.SuggestedScope) {
		return fmt.Errorf("contract draft has invalid suggested scope %q", contract.SuggestedScope)
	}
	if contract.FixedRentAmount < 0 || contract.CAMAmount < 0 || contract.ServiceFee < 0 || contract.DiscountRate < 0 {
		return fmt.Errorf("contract draft has a negative financial value")
	}
	if contract.ScopeConfidence < 0 || contract.ScopeConfidence > 1 || contract.Confidence < 0 || contract.Confidence > 1 {
		return fmt.Errorf("contract draft confidence is outside [0,1]")
	}
	return nil
}
