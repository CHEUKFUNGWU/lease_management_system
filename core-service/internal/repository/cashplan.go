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

	query := `
		SELECT
			f.store_id::text,
			COALESCE(s.code, '') as store_code,
			COALESCE(s.name, '') as store_name,
			TO_CHAR(f.fact_date, 'YYYY-MM') as period,
			COALESCE(f.currency, 'CNY') as currency,
			COALESCE(SUM(f.revenue), 0) as revenue,
			COALESCE(SUM(f.gross_profit), 0) as gross_profit,
			COALESCE(SUM(f.labor_cost), 0) as labor_cost,
			COALESCE(SUM(f.fixed_rent), 0) as fixed_rent,
			COALESCE(SUM(f.variable_rent), 0) as variable_rent,
			COALESCE(SUM(f.non_lease_cost), 0) as non_lease_cost,
			COALESCE(SUM(f.other_cost), 0) as other_cost,
			COUNT(f.id) as days_count
		FROM retail_store_day_facts f
		LEFT JOIN stores s ON f.store_id = s.id
		WHERE f.legal_entity_id = $1
		  AND f.fact_date >= $2 AND f.fact_date <= $3
		  AND f.data_classification = $4
		  AND ($5 = '' OR f.dataset_version = $5)
	`
	args := []interface{}{legalEntityID, startDate, endDate, classification, datasetVersion}
	if len(storeIDs) > 0 {
		placeholders := make([]string, len(storeIDs))
		for i, sid := range storeIDs {
			args = append(args, sid)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		query += fmt.Sprintf(" AND f.store_id IN (%s)", strings.Join(placeholders, ","))
	}
	query += `
		GROUP BY f.store_id, s.code, s.name, TO_CHAR(f.fact_date, 'YYYY-MM'), f.currency
		ORDER BY period ASC, store_code ASC
	`

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
			COALESCE(SUM(CASE WHEN ps.payment_type = 'fixed' THEN ps.amount ELSE 0 END), 0) as fixed_rent,
			COALESCE(SUM(CASE WHEN ps.payment_type = 'variable' THEN ps.amount ELSE 0 END), 0) as variable_rent,
			COALESCE(SUM(CASE WHEN ps.payment_type = 'non_lease' THEN ps.amount ELSE 0 END), 0) as non_lease,
			COALESCE(SUM(CASE WHEN ps.payment_type = 'tax' THEN ps.amount ELSE 0 END), 0) as tax,
			COALESCE(SUM(ps.amount), 0) as total_outflow
		FROM payment_schedules ps
		JOIN contracts c ON ps.contract_id = c.id
		WHERE c.legal_entity_id = $1
		  AND ps.due_date >= $2 AND ps.due_date <= $3
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
