package reporting

import (
	"fmt"
	"strings"

	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
)

// DiscountRateMissingError reports the contracts whose lease liability cannot
// be measured because no discount rate is confirmed (no request override, no
// contract-confirmed rate, no global policy). It wraps
// contractsvc.ErrDiscountRateRequired so existing errors.Is checks keep
// working, while carrying the human-facing contract numbers so the HTTP
// adapter can tell the user exactly which contracts to fix. The engine
// behaviour is unchanged: it still refuses to guess a rate (AGENTS.md,
// Discount Rate 人机协同).
type DiscountRateMissingError struct {
	ContractNumbers []string
}

func (e *DiscountRateMissingError) Error() string {
	return fmt.Sprintf(
		"discount rate requires policy matching or human confirmation (contracts: %s)",
		strings.Join(e.ContractNumbers, ", "),
	)
}

func (e *DiscountRateMissingError) Unwrap() error {
	return contractsvc.ErrDiscountRateRequired
}
