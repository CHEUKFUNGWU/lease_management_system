package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
)

func TestAppendContractScopePredicateAddsAllAssignedDimensions(t *testing.T) {
	ctx := access.WithScope(context.Background(), access.Scope{
		LegalEntityID: "le-001",
		StoreIDs:      []string{"store-1"},
		Regions:       []string{"east"},
		Brands:        []string{"brand-a"},
	})

	query, args, next := appendContractScopePredicate(ctx, "SELECT 1 FROM lease_contracts c WHERE c.id = $1", []any{"contract-1"}, 2, "c")
	for _, fragment := range []string{
		"c.legal_entity_id::text = $2",
		"c.store_id::text = ANY($3)",
		"access_store.region = ANY($4)",
		"access_store.brand = ANY($5)",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query %q does not contain %q", query, fragment)
		}
	}
	if len(args) != 5 || next != 6 {
		t.Fatalf("scope args/next = %d/%d, want 5/6", len(args), next)
	}
}

func TestAppendContractScopePredicateDeniesMissingTenant(t *testing.T) {
	ctx := access.WithScope(context.Background(), access.Scope{})
	query, args, next := appendContractScopePredicate(ctx, "SELECT 1 WHERE true", nil, 1, "c")
	if !strings.Contains(query, "AND false") || len(args) != 0 || next != 1 {
		t.Fatalf("missing tenant scope = query %q args=%v next=%d", query, args, next)
	}
}

func TestAppendContractScopePredicateLeavesUnscopedInternalQueriesUntouched(t *testing.T) {
	query, args, next := appendContractScopePredicate(context.Background(), "SELECT 1", []any{"arg"}, 2, "c")
	if query != "SELECT 1" || len(args) != 1 || next != 2 {
		t.Fatalf("unscoped query = %q args=%v next=%d", query, args, next)
	}
}
