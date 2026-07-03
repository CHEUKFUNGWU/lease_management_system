package access

import "context"

type scopeContextKey struct{}

// Scope is the maximum legal-entity access plus optional narrowing dimensions.
type Scope struct {
	Global        bool
	LegalEntityID string
	StoreIDs      []string
	Regions       []string
	Brands        []string
}

type ContractAttributes struct {
	LegalEntityID string
	StoreID       string
	Region        string
	Brand         string
}

type ApprovalParticipants struct {
	CreatorID  string
	ReviewerID string
}

func (s Scope) AllowsContract(contract ContractAttributes) bool {
	if s.Global {
		return true
	}
	if s.LegalEntityID == "" || contract.LegalEntityID != s.LegalEntityID {
		return false
	}
	return matchesDimension(s.StoreIDs, contract.StoreID) &&
		matchesDimension(s.Regions, contract.Region) &&
		matchesDimension(s.Brands, contract.Brand)
}

func matchesDimension(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == actual {
			return true
		}
	}
	return false
}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	return scope, ok
}
