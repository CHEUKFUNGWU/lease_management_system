package repository

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/closereadiness"
)

type CloseReadinessRepository struct {
	db DBTX
}

func NewCloseReadinessRepository(db DBTX) *CloseReadinessRepository {
	return &CloseReadinessRepository{db: db}
}

// LoadFacts reads the preflight's read-side facts through one repository
// boundary. The queries apply the same contract data-scope predicate as the
// work queue; the service, not SQL, owns the rule conclusions and overall
// status.
func (r *CloseReadinessRepository) LoadFacts(ctx context.Context, period, legalEntityID string) (closereadiness.Facts, error) {
	periodStart := period + "-01"
	periodEnd := periodStart
	args := []interface{}{periodStart, periodEnd}

	query := `
		SELECT c.id::text, COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''),
			COALESCE(c.lease_scope, 'in_scope'), c.discount_rate_value,
			EXISTS (
				SELECT 1 FROM lease_payment_schedules ps
				WHERE ps.contract_id = c.id
				  AND ps.approval_status = 'approved'
				  AND ps.is_official_version = true
			) AS has_approved_payment_plan,
			EXISTS (
				SELECT 1 FROM lease_events e
				WHERE e.contract_id = c.id
				  AND e.approval_status IN ('submitted', 'reviewed', 'pending_approval')
				  AND e.effective_date <= ($2::date + INTERVAL '1 month' - INTERVAL '1 day')::date
			) AS has_pending_event
		FROM lease_contracts c
		WHERE c.approval_status = 'approved'
		  AND c.commencement_date <= ($2::date + INTERVAL '1 month' - INTERVAL '1 day')::date
		  AND c.lease_end_date >= $1::date
		  AND COALESCE(c.lease_scope, 'in_scope') <> 'not_a_lease'
	`
	argIndex := 3
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIndex)
		args = append(args, legalEntityID)
		argIndex++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIndex, "c")
	query += " ORDER BY c.contract_number, c.id"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return closereadiness.Facts{}, fmt.Errorf("failed to load close readiness contracts: %w", err)
	}
	defer rows.Close()

	facts := closereadiness.Facts{Contracts: []closereadiness.ContractFact{}, FailedBatches: []closereadiness.FailedBatchFact{}}
	for rows.Next() {
		var fact closereadiness.ContractFact
		if err := rows.Scan(
			&fact.ContractID, &fact.ContractNumber, &fact.ContractName, &fact.LeaseScope,
			&fact.DiscountRateValue, &fact.HasApprovedPaymentPlan, &fact.HasPendingEvent,
		); err != nil {
			return closereadiness.Facts{}, fmt.Errorf("failed to scan close readiness contract: %w", err)
		}
		facts.Contracts = append(facts.Contracts, fact)
	}
	if err := rows.Err(); err != nil {
		return closereadiness.Facts{}, fmt.Errorf("failed to iterate close readiness contracts: %w", err)
	}

	batchQuery := `
		SELECT b.id::text, COALESCE(b.batch_number, ''), b.status, b.failed_contracts
		FROM monthly_closing_batches b
		LEFT JOIN lease_contracts scoped_contract ON scoped_contract.id = b.scope_contract_id
		WHERE b.accounting_period = $1
		  AND b.status IN ('failed', 'completed_with_errors')
		  AND b.failed_contracts > 0
	`
	batchArgs := []interface{}{period}
	batchArgIndex := 2
	if legalEntityID != "" {
		batchQuery += fmt.Sprintf(" AND b.legal_entity_id = $%d", batchArgIndex)
		batchArgs = append(batchArgs, legalEntityID)
		batchArgIndex++
	}
	if scope, scoped := access.ScopeFromContext(ctx); !scoped || scope.Global ||
		(len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0) {
		// A legal-entity-wide user can see a tenant-wide batch whose
		// scope_contract_id is NULL. A narrowed user must only see batches
		// explicitly scoped to a contract they can access.
	} else {
		batchQuery, batchArgs, _ = appendContractScopePredicate(ctx, batchQuery, batchArgs, batchArgIndex, "scoped_contract")
	}
	batchQuery += " ORDER BY b.created_at DESC"

	batchRows, err := r.db.Query(ctx, batchQuery, batchArgs...)
	if err != nil {
		return closereadiness.Facts{}, fmt.Errorf("failed to load failed close batches: %w", err)
	}
	defer batchRows.Close()
	for batchRows.Next() {
		var batch closereadiness.FailedBatchFact
		if err := batchRows.Scan(&batch.BatchID, &batch.BatchNumber, &batch.Status, &batch.FailedContracts); err != nil {
			return closereadiness.Facts{}, fmt.Errorf("failed to scan failed close batch: %w", err)
		}
		facts.FailedBatches = append(facts.FailedBatches, batch)
	}
	if err := batchRows.Err(); err != nil {
		return closereadiness.Facts{}, fmt.Errorf("failed to iterate failed close batches: %w", err)
	}

	return facts, nil
}
