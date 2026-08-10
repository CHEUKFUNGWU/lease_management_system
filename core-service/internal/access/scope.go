package access

import (
	"context"
	"strings"
)

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

// IntersectScopes returns the safe intersection of two scopes. Global means
// "no restriction" for that operand; two non-global scopes must agree on
// legal entity and their populated dimensions must overlap.
func IntersectScopes(left, right Scope) (Scope, bool) {
	if left.Global {
		return cloneScope(right), right.Global || strings.TrimSpace(right.LegalEntityID) != ""
	}
	if right.Global {
		return cloneScope(left), strings.TrimSpace(left.LegalEntityID) != ""
	}
	leftEntity := strings.TrimSpace(left.LegalEntityID)
	rightEntity := strings.TrimSpace(right.LegalEntityID)
	if leftEntity == "" || rightEntity == "" || leftEntity != rightEntity {
		return Scope{}, false
	}
	result := Scope{LegalEntityID: leftEntity}
	var ok bool
	if result.StoreIDs, ok = intersectDimension(left.StoreIDs, right.StoreIDs); !ok {
		return Scope{}, false
	}
	if result.Regions, ok = intersectDimension(left.Regions, right.Regions); !ok {
		return Scope{}, false
	}
	if result.Brands, ok = intersectDimension(left.Brands, right.Brands); !ok {
		return Scope{}, false
	}
	return result, true
}

func cloneScope(scope Scope) Scope {
	scope.StoreIDs = append([]string(nil), scope.StoreIDs...)
	scope.Regions = append([]string(nil), scope.Regions...)
	scope.Brands = append([]string(nil), scope.Brands...)
	return scope
}

func intersectDimension(left, right []string) ([]string, bool) {
	if len(left) == 0 {
		return append([]string(nil), right...), true
	}
	if len(right) == 0 {
		return append([]string(nil), left...), true
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, value := range left {
		if _, exists := rightSet[value]; !exists {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, len(result) > 0
}
