package repository

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/access"
)

// appendStoreScopePredicate narrows a query that selects from stores directly.
// It is the same slice as the contract predicate applies, read off the store's
// own columns rather than through a contract — which matters for store revenue,
// where there may be no contract in the picture at all.
func appendStoreScopePredicate(ctx context.Context, query string, args []interface{}, argIdx int, storeAlias string) (string, []interface{}, int) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return query, args, argIdx
	}
	if scope.LegalEntityID == "" {
		return query + " AND false", args, argIdx
	}

	query += fmt.Sprintf(" AND %s.legal_entity_id::text = $%d", storeAlias, argIdx)
	args = append(args, scope.LegalEntityID)
	argIdx++
	if len(scope.StoreIDs) > 0 {
		query += fmt.Sprintf(" AND %s.id::text = ANY($%d)", storeAlias, argIdx)
		args = append(args, scope.StoreIDs)
		argIdx++
	}
	if len(scope.Regions) > 0 {
		query += fmt.Sprintf(" AND %s.region = ANY($%d)", storeAlias, argIdx)
		args = append(args, scope.Regions)
		argIdx++
	}
	if len(scope.Brands) > 0 {
		query += fmt.Sprintf(" AND %s.brand = ANY($%d)", storeAlias, argIdx)
		args = append(args, scope.Brands)
		argIdx++
	}
	return query, args, argIdx
}

func appendContractScopePredicate(ctx context.Context, query string, args []interface{}, argIdx int, contractAlias string) (string, []interface{}, int) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return query, args, argIdx
	}
	if scope.LegalEntityID == "" {
		return query + " AND false", args, argIdx
	}

	query += fmt.Sprintf(" AND %s.legal_entity_id::text = $%d", contractAlias, argIdx)
	args = append(args, scope.LegalEntityID)
	argIdx++
	if len(scope.StoreIDs) > 0 {
		query += fmt.Sprintf(" AND %s.store_id::text = ANY($%d)", contractAlias, argIdx)
		args = append(args, scope.StoreIDs)
		argIdx++
	}
	if len(scope.Regions) > 0 {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM stores access_store WHERE access_store.id = %s.store_id AND access_store.region = ANY($%d))", contractAlias, argIdx)
		args = append(args, scope.Regions)
		argIdx++
	}
	if len(scope.Brands) > 0 {
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM stores access_store WHERE access_store.id = %s.store_id AND access_store.brand = ANY($%d))", contractAlias, argIdx)
		args = append(args, scope.Brands)
		argIdx++
	}
	return query, args, argIdx
}
