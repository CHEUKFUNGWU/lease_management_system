package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeasurementResult struct {
	ID                   string     `json:"id"`
	ContractID           string     `json:"contract_id"`
	AccountingPeriod     string     `json:"accounting_period"`
	PeriodStartDate      time.Time  `json:"period_start_date"`
	PeriodEndDate        time.Time  `json:"period_end_date"`
	OpeningLiability     float64    `json:"opening_liability"`
	InterestExpense      float64    `json:"interest_expense"`
	PrincipalRepayment   float64    `json:"principal_repayment"`
	TotalPayment         float64    `json:"total_payment"`
	ClosingLiability     float64    `json:"closing_liability"`
	OpeningROUAsset      float64    `json:"opening_rou_asset"`
	Depreciation         float64    `json:"depreciation"`
	ClosingROUAsset      float64    `json:"closing_rou_asset"`
	VariableRentExpense  float64    `json:"variable_rent_expense"`
	NonLeaseExpense      float64    `json:"non_lease_expense"`
	DiscountRate         float64    `json:"discount_rate"`
	IsCalculated         bool       `json:"is_calculated"`
	CalculationBatchID   *string    `json:"calculation_batch_id"`
	CalculatedAt         *time.Time `json:"calculated_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type JournalEntry struct {
	ID                string     `json:"id"`
	ContractID        string     `json:"contract_id"`
	MeasurementResultID *string  `json:"measurement_result_id"`
	AccountingPeriod  string     `json:"accounting_period"`
	EntryDate         time.Time  `json:"entry_date"`
	EntryType         string     `json:"entry_type"`
	DebitAccount      string     `json:"debit_account"`
	CreditAccount     string     `json:"credit_account"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	Description       *string    `json:"description"`
	VoucherNumber     *string    `json:"voucher_number"`
	PostingStatus     string     `json:"posting_status"`
	PostedAt          *time.Time `json:"posted_at"`
	PostedBy          *string    `json:"posted_by"`
	ApprovedBy        *string    `json:"approved_by"`
	ApprovedAt        *time.Time `json:"approved_at"`
	ERPReference      *string    `json:"erp_reference"`
	BatchID           *string    `json:"batch_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type MonthlyClosingBatch struct {
	ID                 string     `json:"id"`
	BatchNumber        string     `json:"batch_number"`
	AccountingPeriod   string     `json:"accounting_period"`
	LegalEntityID      *string    `json:"legal_entity_id"`
	Region             *string    `json:"region"`
	Brand              *string    `json:"brand"`
	Status             string     `json:"status"`
	TotalContracts     int        `json:"total_contracts"`
	ProcessedContracts int        `json:"processed_contracts"`
	FailedContracts    int        `json:"failed_contracts"`
	TotalEntries       int        `json:"total_entries"`
	PostedEntries      int        `json:"posted_entries"`
	StartedAt          *time.Time `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	CreatedBy          *string    `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type MonthlyClosingRepository struct {
	db *pgxpool.Pool
}

func NewMonthlyClosingRepository(db *pgxpool.Pool) *MonthlyClosingRepository {
	return &MonthlyClosingRepository{db: db}
}

func (r *MonthlyClosingRepository) CreateBatch(ctx context.Context, batch *MonthlyClosingBatch) (*MonthlyClosingBatch, error) {
	batch.ID = uuid.New().String()
	batch.Status = "draft"
	batch.CreatedAt = time.Now()
	batch.UpdatedAt = time.Now()
	
	query := `
		INSERT INTO monthly_closing_batches (
			id, batch_number, accounting_period, legal_entity_id, region, brand,
			status, total_contracts, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	
	_, err := r.db.Exec(ctx, query,
		batch.ID, batch.BatchNumber, batch.AccountingPeriod,
		batch.LegalEntityID, batch.Region, batch.Brand,
		batch.Status, batch.TotalContracts, batch.CreatedBy,
		batch.CreatedAt, batch.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch: %w", err)
	}
	return batch, nil
}

func (r *MonthlyClosingRepository) GetBatches(ctx context.Context, period, legalEntityID string) ([]*MonthlyClosingBatch, error) {
	query := `
		SELECT id, batch_number, accounting_period, legal_entity_id, region, brand,
			status, total_contracts, processed_contracts, failed_contracts,
			total_entries, posted_entries, started_at, completed_at,
			created_by, created_at, updated_at
		FROM monthly_closing_batches
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if period != "" {
		query += fmt.Sprintf(" AND accounting_period = $%d", argIdx)
		args = append(args, period)
		argIdx++
	}
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list batches: %w", err)
	}
	defer rows.Close()

	var batches []*MonthlyClosingBatch
	for rows.Next() {
		b := &MonthlyClosingBatch{}
		err := rows.Scan(
			&b.ID, &b.BatchNumber, &b.AccountingPeriod, &b.LegalEntityID, &b.Region, &b.Brand,
			&b.Status, &b.TotalContracts, &b.ProcessedContracts, &b.FailedContracts,
			&b.TotalEntries, &b.PostedEntries, &b.StartedAt, &b.CompletedAt,
			&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, b)
	}
	return batches, nil
}

func (r *MonthlyClosingRepository) SaveMeasurementResult(ctx context.Context, mr *MeasurementResult) error {
	mr.ID = uuid.New().String()
	mr.CreatedAt = time.Now()
	mr.UpdatedAt = time.Now()
	
	query := `
		INSERT INTO measurement_results (
			id, contract_id, accounting_period, period_start_date, period_end_date,
			opening_liability, interest_expense, principal_repayment, total_payment,
			closing_liability, opening_rou_asset, depreciation, closing_rou_asset,
			variable_rent_expense, non_lease_expense, discount_rate,
			is_calculated, calculation_batch_id, calculated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		ON CONFLICT (contract_id, accounting_period) DO UPDATE SET
			opening_liability = EXCLUDED.opening_liability,
			interest_expense = EXCLUDED.interest_expense,
			principal_repayment = EXCLUDED.principal_repayment,
			total_payment = EXCLUDED.total_payment,
			closing_liability = EXCLUDED.closing_liability,
			opening_rou_asset = EXCLUDED.opening_rou_asset,
			depreciation = EXCLUDED.depreciation,
			closing_rou_asset = EXCLUDED.closing_rou_asset,
			variable_rent_expense = EXCLUDED.variable_rent_expense,
			non_lease_expense = EXCLUDED.non_lease_expense,
			discount_rate = EXCLUDED.discount_rate,
			is_calculated = EXCLUDED.is_calculated,
			calculation_batch_id = EXCLUDED.calculation_batch_id,
			calculated_at = EXCLUDED.calculated_at,
			updated_at = NOW()
	`
	
	_, err := r.db.Exec(ctx, query,
		mr.ID, mr.ContractID, mr.AccountingPeriod, mr.PeriodStartDate, mr.PeriodEndDate,
		mr.OpeningLiability, mr.InterestExpense, mr.PrincipalRepayment, mr.TotalPayment,
		mr.ClosingLiability, mr.OpeningROUAsset, mr.Depreciation, mr.ClosingROUAsset,
		mr.VariableRentExpense, mr.NonLeaseExpense, mr.DiscountRate,
		mr.IsCalculated, mr.CalculationBatchID, mr.CalculatedAt,
		mr.CreatedAt, mr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save measurement result: %w", err)
	}
	return nil
}

func (r *MonthlyClosingRepository) GetMeasurementResults(ctx context.Context, contractID, period string) ([]*MeasurementResult, error) {
	query := `
		SELECT id, contract_id, accounting_period, period_start_date, period_end_date,
			opening_liability, interest_expense, principal_repayment, total_payment,
			closing_liability, opening_rou_asset, depreciation, closing_rou_asset,
			variable_rent_expense, non_lease_expense, discount_rate,
			is_calculated, calculation_batch_id, calculated_at, created_at, updated_at
		FROM measurement_results WHERE contract_id = $1
	`
	args := []interface{}{contractID}
	if period != "" {
		query += " AND accounting_period = $2"
		args = append(args, period)
	}
	query += " ORDER BY accounting_period ASC"
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list results: %w", err)
	}
	defer rows.Close()
	
	var results []*MeasurementResult
	for rows.Next() {
		mr := &MeasurementResult{}
		err := rows.Scan(
			&mr.ID, &mr.ContractID, &mr.AccountingPeriod, &mr.PeriodStartDate, &mr.PeriodEndDate,
			&mr.OpeningLiability, &mr.InterestExpense, &mr.PrincipalRepayment, &mr.TotalPayment,
			&mr.ClosingLiability, &mr.OpeningROUAsset, &mr.Depreciation, &mr.ClosingROUAsset,
			&mr.VariableRentExpense, &mr.NonLeaseExpense, &mr.DiscountRate,
			&mr.IsCalculated, &mr.CalculationBatchID, &mr.CalculatedAt, &mr.CreatedAt, &mr.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, mr)
	}
	return results, nil
}

func (r *MonthlyClosingRepository) CreateJournalEntry(ctx context.Context, entry *JournalEntry) error {
	entry.ID = uuid.New().String()
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	
	query := `
		INSERT INTO journal_entries (
			id, contract_id, measurement_result_id, accounting_period, entry_date,
			entry_type, debit_account, credit_account, amount, currency,
			description, voucher_number, posting_status, batch_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	
	_, err := r.db.Exec(ctx, query,
		entry.ID, entry.ContractID, entry.MeasurementResultID, entry.AccountingPeriod, entry.EntryDate,
		entry.EntryType, entry.DebitAccount, entry.CreditAccount, entry.Amount, entry.Currency,
		entry.Description, entry.VoucherNumber, entry.PostingStatus, entry.BatchID,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create journal entry: %w", err)
	}
	return nil
}

func (r *MonthlyClosingRepository) GetJournalEntries(ctx context.Context, contractID, period, status string) ([]*JournalEntry, error) {
	query := `
		SELECT id, contract_id, measurement_result_id, accounting_period, entry_date,
			entry_type, debit_account, credit_account, amount, currency,
			description, voucher_number, posting_status, posted_at, posted_by,
			approved_by, approved_at, erp_reference, batch_id, created_at, updated_at
		FROM journal_entries WHERE 1=1
	`
	var args []interface{}
	argIdx := 1
	
	if contractID != "" {
		query += fmt.Sprintf(" AND contract_id = $%d", argIdx)
		args = append(args, contractID)
		argIdx++
	}
	if period != "" {
		query += fmt.Sprintf(" AND accounting_period = $%d", argIdx)
		args = append(args, period)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND posting_status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query += " ORDER BY entry_date ASC, entry_type ASC"
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}
	defer rows.Close()
	
	var entries []*JournalEntry
	for rows.Next() {
		e := &JournalEntry{}
		err := rows.Scan(
			&e.ID, &e.ContractID, &e.MeasurementResultID, &e.AccountingPeriod, &e.EntryDate,
			&e.EntryType, &e.DebitAccount, &e.CreditAccount, &e.Amount, &e.Currency,
			&e.Description, &e.VoucherNumber, &e.PostingStatus, &e.PostedAt, &e.PostedBy,
			&e.ApprovedBy, &e.ApprovedAt, &e.ERPReference, &e.BatchID, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *MonthlyClosingRepository) UpdateBatchStatus(ctx context.Context, batchID, status string, processed, failed, total, posted int) error {
	now := time.Now()
	var completedAt interface{}
	if status == "completed" || status == "failed" || status == "cancelled" {
		completedAt = now
	}
	
	query := `
		UPDATE monthly_closing_batches SET
			status = $1,
			processed_contracts = $2,
			failed_contracts = $3,
			total_entries = $4,
			posted_entries = $5,
			completed_at = $6,
			updated_at = $7
		WHERE id = $8
	`
	_, err := r.db.Exec(ctx, query, status, processed, failed, total, posted, completedAt, now, batchID)
	if err != nil {
		return fmt.Errorf("failed to update batch: %w", err)
	}
	return nil
}
