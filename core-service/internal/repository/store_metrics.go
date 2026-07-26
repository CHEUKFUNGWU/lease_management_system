package repository

import (
	"context"
	"fmt"
	"time"
)

// StoreMetric is one store's trading figures for one period, as the customer
// reported them. The system consumes this data and never owns it.
type StoreMetric struct {
	ID          string    `json:"id"`
	StoreID     string    `json:"store_id"`
	StoreCode   string    `json:"store_code,omitempty"`
	StoreName   string    `json:"store_name,omitempty"`
	Period      string    `json:"period"`
	PeriodBasis string    `json:"period_basis"`
	Revenue     float64   `json:"revenue"`
	GrossProfit *float64  `json:"gross_profit"`
	Currency    string    `json:"currency"`
	Version     int       `json:"version"`
	Source      string    `json:"source"`
	Note        *string   `json:"note"`
	CreatedBy   *string   `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RentToSalesRow pairs a store's rent for a period with the sales it made.
type RentToSalesRow struct {
	StoreID   string `json:"store_id"`
	StoreCode string `json:"store_code"`
	StoreName string `json:"store_name"`
	Brand     string `json:"brand"`
	Region    string `json:"region"`
	Period    string `json:"period"`

	// CashRent is the rent actually payable in the period, not the IFRS 16
	// interest and depreciation. Rent-to-sales is a trading measure and the
	// number a business partner means by "rent" is the one that leaves the bank.
	CashRent     float64 `json:"cash_rent"`
	RentCurrency string  `json:"rent_currency"`

	// Revenue is nil when the store has reported nothing for the period. That
	// is different from zero sales, and the report keeps the two apart.
	Revenue         *float64 `json:"revenue"`
	RevenueCurrency string   `json:"revenue_currency"`
	RevenueVersion  *int     `json:"revenue_version"`
	RevenueSource   string   `json:"revenue_source"`

	AreaSqm *float64 `json:"area_sqm"`
}

type StoreMetricsRepository struct {
	db DBTX
}

func NewStoreMetricsRepository(db DBTX) *StoreMetricsRepository {
	return &StoreMetricsRepository{db: db}
}

func (r *StoreMetricsRepository) WithTx(tx DBTX) *StoreMetricsRepository {
	return &StoreMetricsRepository{db: tx}
}

// Upsert records a store's figures for a period. Re-sending the same version is
// idempotent, which is what lets a scheduled push retry safely; sending a new
// version keeps the earlier one, because a restatement should not erase what a
// report was previously based on.
func (r *StoreMetricsRepository) Upsert(ctx context.Context, metric *StoreMetric) (*StoreMetric, error) {
	if metric.Version <= 0 {
		metric.Version = 1
	}
	if metric.PeriodBasis == "" {
		metric.PeriodBasis = "calendar_month"
	}
	if metric.Currency == "" {
		metric.Currency = "CNY"
	}
	if metric.Source == "" {
		metric.Source = "manual"
	}

	query := `
		INSERT INTO store_metrics (
			store_id, period, period_basis, revenue, gross_profit,
			currency, version, source, note, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (store_id, period, version) DO UPDATE SET
			period_basis = EXCLUDED.period_basis,
			revenue = EXCLUDED.revenue,
			gross_profit = EXCLUDED.gross_profit,
			currency = EXCLUDED.currency,
			source = EXCLUDED.source,
			note = EXCLUDED.note,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query,
		metric.StoreID, metric.Period, metric.PeriodBasis, metric.Revenue, metric.GrossProfit,
		metric.Currency, metric.Version, metric.Source, metric.Note, metric.CreatedBy,
	).Scan(&metric.ID, &metric.CreatedAt, &metric.UpdatedAt); err != nil {
		return nil, fmt.Errorf("failed to record store metrics: %w", err)
	}
	return metric, nil
}

// List returns the reported figures, newest period first.
func (r *StoreMetricsRepository) List(ctx context.Context, legalEntityID, period, storeID string) ([]*StoreMetric, error) {
	query := `
		SELECT sm.id, sm.store_id, s.code, s.name, sm.period, sm.period_basis,
			sm.revenue, sm.gross_profit, sm.currency, sm.version, sm.source,
			sm.note, sm.created_by, sm.created_at, sm.updated_at
		FROM store_metrics sm
		JOIN stores s ON s.id = sm.store_id
		WHERE 1=1
	`
	args := make([]interface{}, 0, 4)
	argIdx := 1
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND s.legal_entity_id::text = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	if period != "" {
		query += fmt.Sprintf(" AND sm.period = $%d", argIdx)
		args = append(args, period)
		argIdx++
	}
	if storeID != "" {
		query += fmt.Sprintf(" AND sm.store_id::text = $%d", argIdx)
		args = append(args, storeID)
		argIdx++
	}
	// Revenue is commercially sensitive, so the caller's brand and region slice
	// applies to it exactly as it does to contracts.
	query, args, _ = appendStoreScopePredicate(ctx, query, args, argIdx, "s")
	query += " ORDER BY sm.period DESC, s.code ASC, sm.version DESC LIMIT 500"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list store metrics: %w", err)
	}
	defer rows.Close()

	metrics := make([]*StoreMetric, 0)
	for rows.Next() {
		metric := &StoreMetric{}
		if err := rows.Scan(&metric.ID, &metric.StoreID, &metric.StoreCode, &metric.StoreName,
			&metric.Period, &metric.PeriodBasis, &metric.Revenue, &metric.GrossProfit,
			&metric.Currency, &metric.Version, &metric.Source, &metric.Note,
			&metric.CreatedBy, &metric.CreatedAt, &metric.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan store metric: %w", err)
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

// RentToSales pairs each store's cash rent for a period with the sales reported
// for it. Stores with no reported sales are still returned, because "we do not
// know" is an answer the reader needs to see rather than a row to hide.
func (r *StoreMetricsRepository) RentToSales(ctx context.Context, legalEntityID, period string) ([]RentToSalesRow, error) {
	// The rent is the fixed lease rent falling due in the period. Variable rent
	// is excluded because turnover rent is a function of the sales this ratio
	// divides by, and including it would make the measure chase itself.
	query := `
		WITH period_rent AS (
			SELECT c.store_id,
				SUM(ps.amount) AS cash_rent,
				MIN(ps.currency) AS rent_currency
			FROM lease_payment_schedules ps
			JOIN lease_contracts c ON c.id = ps.contract_id
			WHERE to_char(ps.due_date, 'YYYY-MM') = $1
			  AND ps.is_variable = false
			  AND ps.is_non_lease_component = false
			  -- approval_status is what marks a schedule line as counting; the
			  -- is_official_version flag is maintained for events, not for
			  -- payment schedules, where every row sits at false. Filtering on
			  -- it would report every store as paying no rent at all.
			  AND ps.approval_status = 'approved'
			  AND c.store_id IS NOT NULL
			GROUP BY c.store_id
		),
		latest_metrics AS (
			SELECT DISTINCT ON (store_id) store_id, revenue, currency, version, source
			FROM store_metrics
			WHERE period = $1
			ORDER BY store_id, version DESC
		),
		store_area AS (
			SELECT store_id, SUM(area_sqm) AS area_sqm
			FROM lease_contracts
			WHERE store_id IS NOT NULL AND area_sqm IS NOT NULL
			GROUP BY store_id
		)
		SELECT s.id, s.code, s.name, COALESCE(s.brand, ''), COALESCE(s.region, ''),
			COALESCE(pr.cash_rent, 0), COALESCE(pr.rent_currency, ''),
			lm.revenue, COALESCE(lm.currency, ''), lm.version, COALESCE(lm.source, ''),
			sa.area_sqm
		FROM stores s
		LEFT JOIN period_rent pr ON pr.store_id = s.id
		LEFT JOIN latest_metrics lm ON lm.store_id = s.id
		LEFT JOIN store_area sa ON sa.store_id = s.id
		WHERE (pr.cash_rent IS NOT NULL OR lm.revenue IS NOT NULL)
	`
	args := []interface{}{period}
	argIdx := 2
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND s.legal_entity_id::text = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendStoreScopePredicate(ctx, query, args, argIdx, "s")
	query += " ORDER BY s.code ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load rent-to-sales rows: %w", err)
	}
	defer rows.Close()

	result := make([]RentToSalesRow, 0)
	for rows.Next() {
		var row RentToSalesRow
		row.Period = period
		if err := rows.Scan(&row.StoreID, &row.StoreCode, &row.StoreName, &row.Brand, &row.Region,
			&row.CashRent, &row.RentCurrency, &row.Revenue, &row.RevenueCurrency,
			&row.RevenueVersion, &row.RevenueSource, &row.AreaSqm); err != nil {
			return nil, fmt.Errorf("failed to scan rent-to-sales row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
