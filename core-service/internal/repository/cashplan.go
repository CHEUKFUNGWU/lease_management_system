package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/cashplan"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type CashPlanRepository struct {
	db DBTX
}

func NewCashPlanRepository(db DBTX) *CashPlanRepository {
	return &CashPlanRepository{db: db}
}

// ReadOperating aggregates daily store facts into monthly operating cash facts.
func (r *CashPlanRepository) ReadOperating(ctx context.Context, legalEntityID, fromPeriod, toPeriod, classification, datasetVersion string, storeIDs []string) ([]cashplan.OperatingFact, error) {
	if legalEntityID == "" {
		return nil, fmt.Errorf("legalEntityID is required")
	}
	if classification == "" {
		classification = "production"
	}

	startDate := fromPeriod + "-01"
	// Calculate end date for toPeriod
	tEnd, err := time.Parse("2006-01", toPeriod)
	if err != nil {
		return nil, fmt.Errorf("invalid toPeriod %q: %w", toPeriod, err)
	}
	endDate := tEnd.AddDate(0, 1, -1).Format("2006-01-02")

	whereClause := `
		WHERE s.legal_entity_id = $1
		  AND f.business_date >= $2 AND f.business_date <= $3
		  AND f.data_classification = $4
		  AND ($5 = '' OR f.simulation_dataset_version = $5)
	`
	args := []interface{}{legalEntityID, startDate, endDate, classification, datasetVersion}
	if len(storeIDs) > 0 {
		placeholders := make([]string, len(storeIDs))
		for i, sid := range storeIDs {
			args = append(args, sid)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		whereClause += fmt.Sprintf(" AND f.store_id IN (%s)", strings.Join(placeholders, ","))
	}

	query := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				f.id,
				f.store_id,
				s.code as store_code,
				s.name as store_name,
				f.business_date,
				f.currency,
				f.revenue,
				f.gross_profit,
				f.labor_cost,
				f.fixed_rent,
				f.variable_rent,
				f.non_lease_cost,
				f.other_controllable_cost,
				ROW_NUMBER() OVER (
					PARTITION BY f.store_id, f.business_date
					ORDER BY f.version DESC, f.as_of_at DESC, f.id DESC
				) AS rn
			FROM retail_store_day_facts f
			-- The fact table carries no legal_entity_id; tenancy is reached through
			-- the store, which is why this join is INNER rather than LEFT. A LEFT join
			-- would let a fact whose store is missing escape the tenant filter.
			JOIN stores s ON f.store_id = s.id
			%s
		)
		SELECT
			store_id::text,
			COALESCE(store_code, '') as store_code,
			COALESCE(store_name, '') as store_name,
			TO_CHAR(business_date, 'YYYY-MM') as period,
			COALESCE(currency, 'CNY') as currency,
			COALESCE(SUM(revenue), 0) as revenue,
			COALESCE(SUM(gross_profit), 0) as gross_profit,
			COALESCE(SUM(labor_cost), 0) as labor_cost,
			COALESCE(SUM(fixed_rent), 0) as fixed_rent,
			COALESCE(SUM(variable_rent), 0) as variable_rent,
			COALESCE(SUM(non_lease_cost), 0) as non_lease_cost,
			COALESCE(SUM(other_controllable_cost), 0) as other_cost,
			COUNT(id) as days_count
		FROM ranked
		WHERE rn = 1
		GROUP BY store_id, store_code, store_name, TO_CHAR(business_date, 'YYYY-MM'), currency
		ORDER BY period ASC, store_code ASC
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read operating facts: %w", err)
	}
	defer rows.Close()

	var results []cashplan.OperatingFact
	for rows.Next() {
		var f cashplan.OperatingFact
		var daysCount int
		if err := rows.Scan(
			&f.StoreID, &f.StoreCode, &f.StoreName, &f.Period, &f.Currency,
			&f.Revenue, &f.GrossProfit, &f.LaborCost, &f.FixedRent, &f.VariableRent,
			&f.NonLeaseCost, &f.OtherCost, &daysCount,
		); err != nil {
			return nil, fmt.Errorf("scan operating fact: %w", err)
		}
		// Operating cash = Revenue - labor - non_lease - other - (fixed_rent + variable_rent)
		f.OperatingCash = f.Revenue - f.LaborCost - f.NonLeaseCost - f.OtherCost - (f.FixedRent + f.VariableRent)
		coverageRate := 100.0
		if daysCount < 28 { // rough threshold
			coverageRate = float64(daysCount) / 30.0 * 100.0
		}
		f.Coverage = retailkpi.Coverage{CoverageRate: &coverageRate}
		results = append(results, f)
	}
	return results, nil
}

// ReadLeasePayments queries payment schedules for lease payment outflows.
func (r *CashPlanRepository) ReadLeasePayments(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]cashplan.LeasePaymentFact, error) {
	if legalEntityID == "" {
		return nil, fmt.Errorf("legalEntityID is required")
	}

	startDate := fromPeriod + "-01"
	tEnd, err := time.Parse("2006-01", toPeriod)
	if err != nil {
		return nil, fmt.Errorf("invalid toPeriod %q: %w", toPeriod, err)
	}
	endDate := tEnd.AddDate(0, 1, -1).Format("2006-01-02")

	query := `
		SELECT
			COALESCE(c.id::text, '') as contract_id,
			COALESCE(c.store_id::text, '') as store_id,
			TO_CHAR(ps.due_date, 'YYYY-MM') as period,
			COALESCE(c.currency, 'CNY') as currency,
			-- The schedule has no payment_type column; the classification lives in
			-- boolean flags. These three predicates partition ps.amount exactly, and
			-- match the split already used by RentToSales in store_metrics.go —
			-- "fixed rent" means the lease component that is not turnover-driven.
			COALESCE(SUM(CASE WHEN ps.is_variable = false AND ps.is_non_lease_component = false THEN ps.amount ELSE 0 END), 0) as fixed_rent,
			COALESCE(SUM(CASE WHEN ps.is_variable = true AND ps.is_non_lease_component = false THEN ps.amount ELSE 0 END), 0) as variable_rent,
			COALESCE(SUM(CASE WHEN ps.is_non_lease_component = true THEN ps.amount ELSE 0 END), 0) as non_lease,
			-- Tax is its own column rather than a category of amount, so it adds to
			-- the outflow instead of being carved out of it.
			COALESCE(SUM(ps.tax_amount), 0) as tax,
			COALESCE(SUM(ps.amount), 0) + COALESCE(SUM(ps.tax_amount), 0) as total_outflow
		FROM lease_payment_schedules ps
		JOIN lease_contracts c ON ps.contract_id = c.id
		WHERE c.legal_entity_id = $1
		  AND ps.due_date >= $2 AND ps.due_date <= $3
		  -- approval_status is what marks a schedule line as counting; the
		  -- is_official_version flag sits at false for every payment schedule row,
		  -- so filtering on it would report zero cash outflow for every store.
		  AND ps.approval_status = 'approved'
	`
	args := []interface{}{legalEntityID, startDate, endDate}
	if len(storeIDs) > 0 {
		placeholders := make([]string, len(storeIDs))
		for i, sid := range storeIDs {
			args = append(args, sid)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		query += fmt.Sprintf(" AND c.store_id IN (%s)", strings.Join(placeholders, ","))
	}
	query += `
		GROUP BY c.id, c.store_id, TO_CHAR(ps.due_date, 'YYYY-MM'), c.currency
		ORDER BY period ASC
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read lease payments: %w", err)
	}
	defer rows.Close()

	var results []cashplan.LeasePaymentFact
	for rows.Next() {
		var f cashplan.LeasePaymentFact
		if err := rows.Scan(
			&f.ContractID, &f.StoreID, &f.Period, &f.Currency,
			&f.FixedRent, &f.VariableRent, &f.NonLease, &f.Tax, &f.TotalOutflow,
		); err != nil {
			return nil, fmt.Errorf("scan lease payment fact: %w", err)
		}
		results = append(results, f)
	}
	return results, nil
}

// ReadCapex queries planned capex from fpna_plan_lines.
func (r *CashPlanRepository) ReadCapex(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]cashplan.CapexFact, error) {
	if legalEntityID == "" {
		return nil, fmt.Errorf("legalEntityID is required")
	}

	query := `
		SELECT
			COALESCE(l.store_id::text, '') as store_id,
			l.period,
			l.currency,
			COALESCE(l.capex_category, 'general') as category,
			COALESCE(SUM(l.capex), 0) as amount
		FROM fpna_plan_lines l
		WHERE l.legal_entity_id = $1
		  AND l.period >= $2 AND l.period <= $3
		  AND l.capex IS NOT NULL AND l.capex > 0
	`
	args := []interface{}{legalEntityID, fromPeriod, toPeriod}
	if len(storeIDs) > 0 {
		placeholders := make([]string, len(storeIDs))
		for i, sid := range storeIDs {
			args = append(args, sid)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		query += fmt.Sprintf(" AND l.store_id IN (%s)", strings.Join(placeholders, ","))
	}
	query += `
		GROUP BY l.store_id, l.period, l.currency, l.capex_category
		ORDER BY l.period ASC
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read capex facts: %w", err)
	}
	defer rows.Close()

	var results []cashplan.CapexFact
	for rows.Next() {
		var f cashplan.CapexFact
		if err := rows.Scan(
			&f.StoreID, &f.Period, &f.Currency, &f.Category, &f.Amount,
		); err != nil {
			return nil, fmt.Errorf("scan capex fact: %w", err)
		}
		results = append(results, f)
	}
	return results, nil
}
