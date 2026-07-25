package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BudgetVersion is a frozen measurement snapshot used as the plan to compare
// later actuals against.
type BudgetVersion struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	LegalEntityID *string   `json:"legal_entity_id"`
	AsOfPeriod    string    `json:"as_of_period"`
	FromPeriod    string    `json:"from_period"`
	ToPeriod      string    `json:"to_period"`
	ContractCount int       `json:"contract_count"`
	CreatedBy     *string   `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// BudgetLine is one contract's planned cost for one period.
type BudgetLine struct {
	ContractID       string  `json:"contract_id"`
	AccountingPeriod string  `json:"accounting_period"`
	Currency         string  `json:"currency"`
	InterestExpense  float64 `json:"interest_expense"`
	Depreciation     float64 `json:"depreciation"`
	TotalPayment     float64 `json:"total_payment"`
	ClosingLiability float64 `json:"closing_liability"`
}

type BudgetRepository struct {
	db DBTX
}

func NewBudgetRepository(db DBTX) *BudgetRepository {
	return &BudgetRepository{db: db}
}

func (r *BudgetRepository) WithTx(tx DBTX) *BudgetRepository {
	return &BudgetRepository{db: tx}
}

// CreateVersion stores a budget version and its frozen lines.
func (r *BudgetRepository) CreateVersion(ctx context.Context, version *BudgetVersion, lines []BudgetLine) (*BudgetVersion, error) {
	version.ID = uuid.New().String()
	contracts := map[string]bool{}
	for _, line := range lines {
		contracts[line.ContractID] = true
	}
	version.ContractCount = len(contracts)

	err := r.db.QueryRow(ctx, `
		INSERT INTO budget_versions (id, name, legal_entity_id, as_of_period, from_period, to_period, contract_count, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`, version.ID, version.Name, version.LegalEntityID, version.AsOfPeriod,
		version.FromPeriod, version.ToPeriod, version.ContractCount, version.CreatedBy,
	).Scan(&version.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create budget version: %w", err)
	}

	for _, line := range lines {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO budget_lines (
				budget_version_id, contract_id, accounting_period, currency,
				interest_expense, depreciation, total_payment, closing_liability
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (budget_version_id, contract_id, accounting_period) DO NOTHING
		`, version.ID, line.ContractID, line.AccountingPeriod, line.Currency,
			line.InterestExpense, line.Depreciation, line.TotalPayment, line.ClosingLiability); err != nil {
			return nil, fmt.Errorf("failed to store budget line: %w", err)
		}
	}
	return version, nil
}

// ListVersions returns budget versions, newest first.
func (r *BudgetRepository) ListVersions(ctx context.Context, legalEntityID string) ([]*BudgetVersion, error) {
	query := `
		SELECT id, name, legal_entity_id, as_of_period, from_period, to_period,
			contract_count, created_by, created_at
		FROM budget_versions
		WHERE ($1 = '' OR legal_entity_id::text = $1)
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, query, legalEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to list budget versions: %w", err)
	}
	defer rows.Close()

	versions := make([]*BudgetVersion, 0)
	for rows.Next() {
		v := &BudgetVersion{}
		if err := rows.Scan(&v.ID, &v.Name, &v.LegalEntityID, &v.AsOfPeriod, &v.FromPeriod,
			&v.ToPeriod, &v.ContractCount, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan budget version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// BudgetContractPeriod carries a budget line together with the contract's
// identity, which the variance report needs to name the row.
type BudgetContractPeriod struct {
	ContractID     string
	ContractNumber string
	ContractName   string
	Currency       string
	LeaseCost      float64
}

// LinesForPeriod returns the planned lease cost per contract for one period.
func (r *BudgetRepository) LinesForPeriod(ctx context.Context, versionID, period string) ([]BudgetContractPeriod, error) {
	rows, err := r.db.Query(ctx, `
		SELECT bl.contract_id, COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''),
			bl.currency, bl.interest_expense + bl.depreciation
		FROM budget_lines bl
		JOIN lease_contracts c ON c.id = bl.contract_id
		WHERE bl.budget_version_id = $1 AND bl.accounting_period = $2
	`, versionID, period)
	if err != nil {
		return nil, fmt.Errorf("failed to load budget lines: %w", err)
	}
	defer rows.Close()

	items := make([]BudgetContractPeriod, 0)
	for rows.Next() {
		var item BudgetContractPeriod
		if err := rows.Scan(&item.ContractID, &item.ContractNumber, &item.ContractName,
			&item.Currency, &item.LeaseCost); err != nil {
			return nil, fmt.Errorf("failed to scan budget line: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// ActualsForPeriod returns the measured lease cost per contract for one period,
// which is what the close actually produced.
func (r *BudgetRepository) ActualsForPeriod(ctx context.Context, legalEntityID, period string) ([]BudgetContractPeriod, error) {
	query := `
		SELECT mr.contract_id, COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''),
			COALESCE(c.currency, 'CNY'), mr.interest_expense + mr.depreciation
		FROM measurement_results mr
		JOIN lease_contracts c ON c.id = mr.contract_id
		WHERE mr.accounting_period = $1
	`
	args := []interface{}{period}
	argIdx := 2
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "c")

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load actuals: %w", err)
	}
	defer rows.Close()

	items := make([]BudgetContractPeriod, 0)
	for rows.Next() {
		var item BudgetContractPeriod
		if err := rows.Scan(&item.ContractID, &item.ContractNumber, &item.ContractName,
			&item.Currency, &item.LeaseCost); err != nil {
			return nil, fmt.Errorf("failed to scan actual: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// EventTypesByContract returns the lease event types effective in a period, which
// is what lets the variance be explained rather than merely reported.
func (r *BudgetRepository) EventTypesByContract(ctx context.Context, period string) (map[string][]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT contract_id, event_type
		FROM lease_events
		WHERE to_char(effective_date, 'YYYY-MM') = $1
		  AND approval_status = 'approved'
	`, period)
	if err != nil {
		return nil, fmt.Errorf("failed to load period events: %w", err)
	}
	defer rows.Close()

	byContract := map[string][]string{}
	for rows.Next() {
		var contractID, eventType string
		if err := rows.Scan(&contractID, &eventType); err != nil {
			return nil, fmt.Errorf("failed to scan period event: %w", err)
		}
		byContract[contractID] = append(byContract[contractID], eventType)
	}
	return byContract, nil
}

// FXByContract returns the exchange difference recognised per contract in a
// period, taken from the entries the close produced.
func (r *BudgetRepository) FXByContract(ctx context.Context, period string) (map[string]float64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT contract_id, SUM(CASE WHEN debit_account LIKE '6603%' THEN amount ELSE -amount END)
		FROM journal_entries
		WHERE accounting_period = $1 AND entry_type = 'fx_remeasurement'
		  AND posting_status <> 'reversed'
		GROUP BY contract_id
	`, period)
	if err != nil {
		return nil, fmt.Errorf("failed to load exchange differences: %w", err)
	}
	defer rows.Close()

	byContract := map[string]float64{}
	for rows.Next() {
		var contractID string
		var amount float64
		if err := rows.Scan(&contractID, &amount); err != nil {
			return nil, fmt.Errorf("failed to scan exchange difference: %w", err)
		}
		byContract[contractID] = amount
	}
	return byContract, nil
}
