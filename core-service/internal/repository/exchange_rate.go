package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Rate types recognised by IAS 21 translation.
const (
	RateTypeClosing = "closing"
	RateTypeAverage = "average"
)

// ExchangeRate is one published rate: units of ToCurrency per unit of
// FromCurrency on RateDate.
type ExchangeRate struct {
	ID           string    `json:"id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	RateDate     time.Time `json:"rate_date"`
	RateType     string    `json:"rate_type"`
	Rate         float64   `json:"rate"`
	Source       *string   `json:"source"`
	CreatedBy    *string   `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ExchangeRateRepository struct {
	db DBTX
}

func NewExchangeRateRepository(db DBTX) *ExchangeRateRepository {
	return &ExchangeRateRepository{db: db}
}

func (r *ExchangeRateRepository) WithTx(tx DBTX) *ExchangeRateRepository {
	return &ExchangeRateRepository{db: tx}
}

// Upsert records a rate, replacing any existing rate for the same currency
// pair, date and type. Re-publishing a corrected rate is normal; keeping two
// rates for the same day is not.
func (r *ExchangeRateRepository) Upsert(ctx context.Context, rate *ExchangeRate) (*ExchangeRate, error) {
	if rate.RateType == "" {
		rate.RateType = RateTypeClosing
	}
	if rate.Rate <= 0 {
		return nil, fmt.Errorf("exchange rate must be greater than zero")
	}
	if rate.FromCurrency == rate.ToCurrency {
		return nil, fmt.Errorf("exchange rate requires two different currencies")
	}

	rate.ID = uuid.New().String()
	err := r.db.QueryRow(ctx, `
		INSERT INTO exchange_rates (
			id, from_currency, to_currency, rate_date, rate_type, rate, source, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (from_currency, to_currency, rate_date, rate_type) DO UPDATE
		SET rate = EXCLUDED.rate, source = EXCLUDED.source, updated_at = NOW()
		RETURNING id, created_at, updated_at
	`, rate.ID, rate.FromCurrency, rate.ToCurrency, rate.RateDate, rate.RateType,
		rate.Rate, rate.Source, rate.CreatedBy,
	).Scan(&rate.ID, &rate.CreatedAt, &rate.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save exchange rate: %w", err)
	}
	return rate, nil
}

// List returns rates for a currency pair, newest first. Empty filters list all.
func (r *ExchangeRateRepository) List(ctx context.Context, fromCurrency, toCurrency string) ([]*ExchangeRate, error) {
	query := `
		SELECT id, from_currency, to_currency, rate_date, rate_type, rate, source,
			created_by, created_at, updated_at
		FROM exchange_rates
		WHERE ($1 = '' OR from_currency = $1) AND ($2 = '' OR to_currency = $2)
		ORDER BY rate_date DESC, from_currency, to_currency, rate_type
		LIMIT 500
	`
	rows, err := r.db.Query(ctx, query, fromCurrency, toCurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to list exchange rates: %w", err)
	}
	defer rows.Close()

	rates := make([]*ExchangeRate, 0)
	for rows.Next() {
		rate := &ExchangeRate{}
		if err := rows.Scan(&rate.ID, &rate.FromCurrency, &rate.ToCurrency, &rate.RateDate,
			&rate.RateType, &rate.Rate, &rate.Source, &rate.CreatedBy,
			&rate.CreatedAt, &rate.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan exchange rate: %w", err)
		}
		rates = append(rates, rate)
	}
	return rates, nil
}

// ErrRateNotFound reports that no published rate covers the requested date.
// Callers must surface it rather than substituting a guess.
var ErrRateNotFound = fmt.Errorf("no published exchange rate for the requested date")

// GetRate returns the most recent rate of the given type on or before asOf.
// Using the latest prior rate matches how closing rates are published: a rate
// stands until the next one is issued. It never looks forward, so a close can
// not accidentally use a rate published after the period it reports.
func (r *ExchangeRateRepository) GetRate(ctx context.Context, fromCurrency, toCurrency, rateType string, asOf time.Time) (float64, error) {
	if fromCurrency == toCurrency {
		return 1, nil
	}
	if rateType == "" {
		rateType = RateTypeClosing
	}
	var rate float64
	err := r.db.QueryRow(ctx, `
		SELECT rate FROM exchange_rates
		WHERE from_currency = $1 AND to_currency = $2 AND rate_type = $3 AND rate_date <= $4
		ORDER BY rate_date DESC
		LIMIT 1
	`, fromCurrency, toCurrency, rateType, asOf).Scan(&rate)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("%w: %s/%s %s as of %s",
			ErrRateNotFound, fromCurrency, toCurrency, rateType, asOf.Format("2006-01-02"))
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read exchange rate: %w", err)
	}
	return rate, nil
}

// ExchangeRateVersion is a controlled rate version for translation.
type ExchangeRateVersion struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	VersionType   string     `json:"version_type"`
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	CreatedBy     *string    `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (r *ExchangeRateRepository) ListVersions(ctx context.Context) ([]*ExchangeRateVersion, error) {
	query := `
		SELECT id, name, version_type, effective_from, effective_to, source, status,
			created_by, created_at, updated_at
		FROM exchange_rate_versions
		ORDER BY effective_from DESC, name ASC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list exchange rate versions: %w", err)
	}
	defer rows.Close()

	versions := make([]*ExchangeRateVersion, 0)
	for rows.Next() {
		v := &ExchangeRateVersion{}
		if err := rows.Scan(&v.ID, &v.Name, &v.VersionType, &v.EffectiveFrom, &v.EffectiveTo,
			&v.Source, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan exchange rate version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (r *ExchangeRateRepository) GetVersion(ctx context.Context, versionRef string) (*ExchangeRateVersion, error) {
	query := `
		SELECT id, name, version_type, effective_from, effective_to, source, status,
			created_by, created_at, updated_at
		FROM exchange_rate_versions
		WHERE id::text = $1 OR name = $1
		LIMIT 1
	`
	v := &ExchangeRateVersion{}
	err := r.db.QueryRow(ctx, query, versionRef).Scan(
		&v.ID, &v.Name, &v.VersionType, &v.EffectiveFrom, &v.EffectiveTo,
		&v.Source, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get exchange rate version: %w", err)
	}
	return v, nil
}

func (r *ExchangeRateRepository) CreateVersion(ctx context.Context, v *ExchangeRateVersion) (*ExchangeRateVersion, error) {
	if v.Name == "" {
		return nil, fmt.Errorf("version name is required")
	}
	if v.VersionType == "" {
		v.VersionType = "budget"
	}
	if v.Status == "" {
		v.Status = "draft"
	}
	v.ID = uuid.New().String()
	err := r.db.QueryRow(ctx, `
		INSERT INTO exchange_rate_versions (
			id, name, version_type, effective_from, effective_to, source, status, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`, v.ID, v.Name, v.VersionType, v.EffectiveFrom, v.EffectiveTo, v.Source, v.Status, v.CreatedBy).
		Scan(&v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create exchange rate version: %w", err)
	}
	return v, nil
}

func (r *ExchangeRateRepository) GetRatesForVersion(ctx context.Context, versionID, rateType string) ([]ExchangeRate, error) {
	query := `
		SELECT id, from_currency, to_currency, rate_date, rate_type, rate, source,
			created_by, created_at, updated_at
		FROM exchange_rates
		WHERE (version_id::text = $1 OR ($1 = '' AND version_id IS NULL))
		  AND ($2 = '' OR rate_type = $2)
		ORDER BY rate_date DESC
	`
	rows, err := r.db.Query(ctx, query, versionID, rateType)
	if err != nil {
		return nil, fmt.Errorf("get rates for version: %w", err)
	}
	defer rows.Close()

	var rates []ExchangeRate
	for rows.Next() {
		var er ExchangeRate
		if err := rows.Scan(&er.ID, &er.FromCurrency, &er.ToCurrency, &er.RateDate,
			&er.RateType, &er.Rate, &er.Source, &er.CreatedBy,
			&er.CreatedAt, &er.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rate: %w", err)
		}
		rates = append(rates, er)
	}
	return rates, nil
}
