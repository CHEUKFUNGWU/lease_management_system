package repository

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/access"
)

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
