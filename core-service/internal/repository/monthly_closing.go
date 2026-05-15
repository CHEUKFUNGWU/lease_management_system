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

type EventAdjustment struct {
	ID                   string     `json:"id"`
	EventID              string     `json:"event_id"`
	ContractID           string     `json:"contract_id"`
	AdjustmentType       string     `json:"adjustment_type"`
	EffectiveDate        time.Time  `json:"effective_date"`
	LiabilityBefore      float64    `json:"liability_before"`
	LiabilityAfter       float64    `json:"liability_after"`
	LiabilityAdjustment  float64    `json:"liability_adjustment"`
	ROUBefore            float64    `json:"rou_before"`
	ROUAfter             float64    `json:"rou_after"`
	ROUAdjustment        float64    `json:"rou_adjustment"`
	PnLGain              float64    `json:"pnl_gain"`
	PnLLoss              float64    `json:"pnl_loss"`
	RevisedDiscountRate  float64    `json:"revised_discount_rate"`
	DiscountRateSource   *string    `json:"discount_rate_source"`
	CalculationBatchID   *string    `json:"calculation_batch_id"`
	CreatedAt            time.Time  `json:"created_at"`
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

// ApproveJournalEntry approves a single journal entry in draft status
func (r *MonthlyClosingRepository) ApproveJournalEntry(ctx context.Context, entryID string, userID string) error {
	query := `
		UPDATE journal_entries SET
			posting_status = 'approved',
			approved_by = $1,
			approved_at = NOW(),
			updated_at = NOW()
		WHERE id = $2 AND posting_status = 'draft'
	`
	result, err := r.db.Exec(ctx, query, userID, entryID)
	if err != nil {
		return fmt.Errorf("failed to approve journal entry: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("journal entry not found or not in draft status")
	}
	return nil
}

// PostJournalEntry posts an approved journal entry
func (r *MonthlyClosingRepository) PostJournalEntry(ctx context.Context, entryID string, userID string, erpRef string) error {
	query := `
		UPDATE journal_entries SET
			posting_status = 'posted',
			posted_by = $1,
			posted_at = NOW(),
			erp_reference = $2,
			updated_at = NOW()
		WHERE id = $3 AND posting_status = 'approved'
	`
	var erpRefVal interface{}
	if erpRef == "" {
		erpRefVal = nil
	} else {
		erpRefVal = erpRef
	}
	result, err := r.db.Exec(ctx, query, userID, erpRefVal, entryID)
	if err != nil {
		return fmt.Errorf("failed to post journal entry: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("journal entry not found or not in approved status")
	}
	return nil
}

// ApproveBatchEntries approves all draft journal entries in a batch
func (r *MonthlyClosingRepository) ApproveBatchEntries(ctx context.Context, batchID string, userID string) (int, error) {
	query := `
		UPDATE journal_entries SET
			posting_status = 'approved',
			approved_by = $1,
			approved_at = NOW(),
			updated_at = NOW()
		WHERE batch_id = $2 AND posting_status = 'draft'
	`
	result, err := r.db.Exec(ctx, query, userID, batchID)
	if err != nil {
		return 0, fmt.Errorf("failed to approve batch entries: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// PostBatchEntries posts all approved journal entries in a batch
func (r *MonthlyClosingRepository) PostBatchEntries(ctx context.Context, batchID string, userID string) (int, error) {
	query := `
		UPDATE journal_entries SET
			posting_status = 'posted',
			posted_by = $1,
			posted_at = NOW(),
			updated_at = NOW()
		WHERE batch_id = $2 AND posting_status = 'approved'
	`
	result, err := r.db.Exec(ctx, query, userID, batchID)
	if err != nil {
		return 0, fmt.Errorf("failed to post batch entries: %w", err)
	}
	count := int(result.RowsAffected())

	// Update the batch posted_entries count
	if count > 0 {
		updateQuery := `
			UPDATE monthly_closing_batches SET
				posted_entries = posted_entries + $1,
				updated_at = NOW()
			WHERE id = $2
		`
		_, err = r.db.Exec(ctx, updateQuery, count, batchID)
		if err != nil {
			return count, fmt.Errorf("failed to update batch posted entries: %w", err)
		}
	}

	return count, nil
}

// LockPeriod locks an accounting period for the given legal entity
func (r *MonthlyClosingRepository) LockPeriod(ctx context.Context, period, legalEntityID, userID string) error {
	query := `
		INSERT INTO period_locks (accounting_period, legal_entity_id, is_locked, locked_by, locked_at, created_at, updated_at)
		VALUES ($1, $2, true, $3, NOW(), NOW(), NOW())
		ON CONFLICT (accounting_period, legal_entity_id) DO UPDATE SET
			is_locked = true,
			locked_by = $3,
			locked_at = NOW(),
			unlocked_by = NULL,
			unlocked_at = NULL,
			updated_at = NOW()
	`
	var legalEntityIDVal interface{}
	if legalEntityID == "" {
		legalEntityIDVal = nil
	} else {
		legalEntityIDVal = legalEntityID
	}
	_, err := r.db.Exec(ctx, query, period, legalEntityIDVal, userID)
	if err != nil {
		return fmt.Errorf("failed to lock period: %w", err)
	}
	return nil
}

// UnlockPeriod unlocks an accounting period for the given legal entity
func (r *MonthlyClosingRepository) UnlockPeriod(ctx context.Context, period, legalEntityID, userID string) error {
	query := `
		UPDATE period_locks SET
			is_locked = false,
			unlocked_by = $1,
			unlocked_at = NOW(),
			updated_at = NOW()
		WHERE accounting_period = $2 AND (legal_entity_id = $3 OR legal_entity_id IS NULL)
	`
	var legalEntityIDVal interface{}
	if legalEntityID == "" {
		legalEntityIDVal = nil
	} else {
		legalEntityIDVal = legalEntityID
	}
	result, err := r.db.Exec(ctx, query, userID, period, legalEntityIDVal)
	if err != nil {
		return fmt.Errorf("failed to unlock period: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("period lock not found for period %s", period)
	}
	return nil
}

// IsPeriodLocked checks if a period is locked
func (r *MonthlyClosingRepository) IsPeriodLocked(ctx context.Context, period, legalEntityID string) (bool, error) {
	query := `
		SELECT is_locked FROM period_locks
		WHERE accounting_period = $1 AND (legal_entity_id = $2 OR legal_entity_id IS NULL)
	`
	var legalEntityIDVal interface{}
	if legalEntityID == "" {
		legalEntityIDVal = nil
	} else {
		legalEntityIDVal = legalEntityID
	}
	var isLocked bool
	err := r.db.QueryRow(ctx, query, period, legalEntityIDVal).Scan(&isLocked)
	if err != nil {
		// No lock record means not locked
		return false, nil
	}
	return isLocked, nil
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

// CreateEventAdjustment inserts a new event adjustment record.
func (r *MonthlyClosingRepository) CreateEventAdjustment(ctx context.Context, adj *EventAdjustment) (*EventAdjustment, error) {
	adj.ID = uuid.New().String()
	adj.CreatedAt = time.Now()

	query := `
		INSERT INTO event_adjustments (
			id, event_id, contract_id, adjustment_type, effective_date,
			liability_before, liability_after, liability_adjustment,
			rou_before, rou_after, rou_adjustment,
			pnl_gain, pnl_loss, revised_discount_rate, discount_rate_source,
			calculation_batch_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	_, err := r.db.Exec(ctx, query,
		adj.ID, adj.EventID, adj.ContractID, adj.AdjustmentType, adj.EffectiveDate,
		adj.LiabilityBefore, adj.LiabilityAfter, adj.LiabilityAdjustment,
		adj.ROUBefore, adj.ROUAfter, adj.ROUAdjustment,
		adj.PnLGain, adj.PnLLoss, adj.RevisedDiscountRate, adj.DiscountRateSource,
		adj.CalculationBatchID, adj.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create event adjustment: %w", err)
	}
	return adj, nil
}

// GetEventAdjustmentsForContract returns all event adjustments for a contract, ordered by effective date.
func (r *MonthlyClosingRepository) GetEventAdjustmentsForContract(ctx context.Context, contractID string) ([]*EventAdjustment, error) {
	query := `
		SELECT id, event_id, contract_id, adjustment_type, effective_date,
			liability_before, liability_after, liability_adjustment,
			rou_before, rou_after, rou_adjustment,
			pnl_gain, pnl_loss, revised_discount_rate, discount_rate_source,
			calculation_batch_id, created_at
		FROM event_adjustments
		WHERE contract_id = $1
		ORDER BY effective_date ASC
	`

	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list event adjustments: %w", err)
	}
	defer rows.Close()

	var adjustments []*EventAdjustment
	for rows.Next() {
		a := &EventAdjustment{}
		err := rows.Scan(
			&a.ID, &a.EventID, &a.ContractID, &a.AdjustmentType, &a.EffectiveDate,
			&a.LiabilityBefore, &a.LiabilityAfter, &a.LiabilityAdjustment,
			&a.ROUBefore, &a.ROUAfter, &a.ROUAdjustment,
			&a.PnLGain, &a.PnLLoss, &a.RevisedDiscountRate, &a.DiscountRateSource,
			&a.CalculationBatchID, &a.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event adjustment: %w", err)
		}
		adjustments = append(adjustments, a)
	}
	return adjustments, nil
}

// GetEventAdjustmentByEventID returns the event adjustment record for a specific event.
func (r *MonthlyClosingRepository) GetEventAdjustmentByEventID(ctx context.Context, eventID string) (*EventAdjustment, error) {
	query := `
		SELECT id, event_id, contract_id, adjustment_type, effective_date,
			liability_before, liability_after, liability_adjustment,
			rou_before, rou_after, rou_adjustment,
			pnl_gain, pnl_loss, revised_discount_rate, discount_rate_source,
			calculation_batch_id, created_at
		FROM event_adjustments
		WHERE event_id = $1
	`

	a := &EventAdjustment{}
	err := r.db.QueryRow(ctx, query, eventID).Scan(
		&a.ID, &a.EventID, &a.ContractID, &a.AdjustmentType, &a.EffectiveDate,
		&a.LiabilityBefore, &a.LiabilityAfter, &a.LiabilityAdjustment,
		&a.ROUBefore, &a.ROUAfter, &a.ROUAdjustment,
		&a.PnLGain, &a.PnLLoss, &a.RevisedDiscountRate, &a.DiscountRateSource,
		&a.CalculationBatchID, &a.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event adjustment: %w", err)
	}
	return a, nil
}

// LinkAdjustmentToBatch updates an event adjustment's calculation_batch_id.
func (r *MonthlyClosingRepository) LinkAdjustmentToBatch(ctx context.Context, adjustmentID, batchID string) error {
	query := `UPDATE event_adjustments SET calculation_batch_id = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, batchID, adjustmentID)
	return err
}

// UpdateMeasurementResultsFromDate overwrites measurement_results from a given effective date forward.
func (r *MonthlyClosingRepository) UpdateMeasurementResultsFromDate(ctx context.Context, contractID, effectivePeriod string, periods []*MeasurementResult) error {
	for _, mr := range periods {
		// Use the existing SaveMeasurementResult which handles upsert
		if err := r.SaveMeasurementResult(ctx, mr); err != nil {
			return fmt.Errorf("failed to update measurement result for period %s: %w", mr.AccountingPeriod, err)
		}
	}
	return nil
}
