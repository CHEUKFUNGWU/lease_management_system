package agenttools

import (
	"context"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
)

// ContractAccessReader is the only dependency needed by the scope guard. A
// repository or application service can satisfy it; Agent code never needs a
// database handle or a second copy of tenant rules.
type ContractAccessReader interface {
	GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error)
}

// RequireContractAccess must run before any contract-linked query. It returns
// only access attributes; callers then load related data through already scoped
// repository methods.
func RequireContractAccess(ctx context.Context, contractID string, reader ContractAccessReader) (access.ContractAttributes, error) {
	if strings.TrimSpace(contractID) == "" {
		return access.ContractAttributes{}, ErrContractIDRequired
	}
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped {
		return access.ContractAttributes{}, ErrScopeUnavailable
	}
	attributes, found, err := reader.GetContractAttributes(ctx, contractID)
	if err != nil {
		return access.ContractAttributes{}, fmt.Errorf("verify contract access: %w", err)
	}
	if !found {
		return access.ContractAttributes{}, ErrContractNotFound
	}
	if !scope.AllowsContract(attributes) {
		return access.ContractAttributes{}, ErrContractOutOfScope
	}
	return attributes, nil
}
