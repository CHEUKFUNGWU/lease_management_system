package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/lease-management-system/core-service/internal/money"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lease-management-system/core-service/internal/access"
)

var ErrUnresolvedBlockingExceptions = errors.New("unresolved blocking close exceptions")

// DBTX is the subset of database operations shared by *pgxpool.Pool and pgx.Tx.
// Depending on this interface lets a repository run its queries either directly
// against the pool or inside a transaction supplied via WithTx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type MeasurementResult struct {
	ID                  string       `json:"id"`
	ContractID          string       `json:"contract_id"`
	AccountingPeriod    string       `json:"accounting_period"`
	PeriodStartDate     time.Time    `json:"period_start_date"`
	PeriodEndDate       time.Time    `json:"period_end_date"`
	OpeningLiability    money.Amount `json:"opening_liability"`
	InterestExpense     money.Amount `json:"interest_expense"`
	PrincipalRepayment  money.Amount `json:"principal_repayment"`
	TotalPayment        money.Amount `json:"total_payment"`
	ClosingLiability    money.Amount `json:"closing_liability"`
	OpeningROUAsset     money.Amount `json:"opening_rou_asset"`
	Depreciation        money.Amount `json:"depreciation"`
	ClosingROUAsset     money.Amount `json:"closing_rou_asset"`
	VariableRentExpense money.Amount `json:"variable_rent_expense"`
	NonLeaseExpense     money.Amount `json:"non_lease_expense"`
	DiscountRate        float64      `json:"discount_rate"`
	IsCalculated        bool         `json:"is_calculated"`
	CalculationBatchID  *string      `json:"calculation_batch_id"`
	CalculatedAt        *time.Time   `json:"calculated_at"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type JournalEntry struct {
	ID                  string       `json:"id"`
	ContractID          string       `json:"contract_id"`
	MeasurementResultID *string      `json:"measurement_result_id"`
	AccountingPeriod    string       `json:"accounting_period"`
	EntryDate           time.Time    `json:"entry_date"`
	EntryType           string       `json:"entry_type"`
	DebitAccount        string       `json:"debit_account"`
	CreditAccount       string       `json:"credit_account"`
	Amount              money.Amount `json:"amount"`
	Currency            string       `json:"currency"`
	Description         *string      `json:"description"`
	VoucherNumber       *string      `json:"voucher_number"`
	PostingStatus       string       `json:"posting_status"`
	PostedAt            *time.Time   `json:"posted_at"`
	PostedBy            *string      `json:"posted_by"`
	ApprovedBy          *string      `json:"approved_by"`
	ApprovedAt          *time.Time   `json:"approved_at"`
	ERPReference        *string      `json:"erp_reference"`
	BatchID             *string      `json:"batch_id"`
	// ReversalOfEntryID and ReversalReason are set on a reversing entry and
	// identify the posted entry it cancels. ReversedAt/ReversedBy are set on the
	// original entry when it is reversed.
	ReversalOfEntryID *string    `json:"reversal_of_entry_id"`
	ReversalReason    *string    `json:"reversal_reason"`
	ReversedAt        *time.Time `json:"reversed_at"`
	ReversedBy        *string    `json:"reversed_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type MonthlyClosingBatch struct {
	ID                 string     `json:"id"`
	BatchNumber        string     `json:"batch_number"`
	AccountingPeriod   string     `json:"accounting_period"`
	LegalEntityID      *string    `json:"legal_entity_id"`
	ScopeContractID    *string    `json:"scope_contract_id"`
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
	ID                  string    `json:"id"`
	EventID             string    `json:"event_id"`
	ContractID          string    `json:"contract_id"`
	AdjustmentType      string    `json:"adjustment_type"`
	EffectiveDate       time.Time `json:"effective_date"`
	LiabilityBefore     float64   `json:"liability_before"`
	LiabilityAfter      float64   `json:"liability_after"`
	LiabilityAdjustment float64   `json:"liability_adjustment"`
	ROUBefore           float64   `json:"rou_before"`
	ROUAfter            float64   `json:"rou_after"`
	ROUAdjustment       float64   `json:"rou_adjustment"`
	PnLGain             float64   `json:"pnl_gain"`
	PnLLoss             float64   `json:"pnl_loss"`
	RevisedDiscountRate float64   `json:"revised_discount_rate"`
	DiscountRateSource  *string   `json:"discount_rate_source"`
	CalculationBatchID  *string   `json:"calculation_batch_id"`
	CreatedAt           time.Time `json:"created_at"`
}

type MonthlyClosingRepository struct {
	db DBTX
}

func NewMonthlyClosingRepository(db DBTX) *MonthlyClosingRepository {
	return &MonthlyClosingRepository{db: db}
}

// WithTx returns a copy of the repository whose queries run on the given
// transaction. Callers begin a transaction on the pool, pass the resulting
// pgx.Tx here, and every write made through the returned repository commits or
// rolls back atomically with that transaction.
func (r *MonthlyClosingRepository) WithTx(tx DBTX) *MonthlyClosingRepository {
	return &MonthlyClosingRepository{db: tx}
}

func (r *MonthlyClosingRepository) journalEntryAllowed(ctx context.Context, entryID string) (bool, error) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return true, nil
	}
	var attributes access.ContractAttributes
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(c.legal_entity_id::text, ''), COALESCE(c.store_id::text, ''),
		       COALESCE(s.region, ''), COALESCE(s.brand, '')
		FROM journal_entries e
		JOIN lease_contracts c ON c.id = e.contract_id
		LEFT JOIN stores s ON s.id = c.store_id
		WHERE e.id = $1
	`, entryID).Scan(&attributes.LegalEntityID, &attributes.StoreID, &attributes.Region, &attributes.Brand)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return scope.AllowsContract(attributes), nil
}

func (r *MonthlyClosingRepository) batchAllowed(ctx context.Context, batchID string) (bool, error) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global {
		return true, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT COALESCE(c.legal_entity_id::text, ''), COALESCE(c.store_id::text, ''),
		       COALESCE(s.region, ''), COALESCE(s.brand, '')
		FROM journal_entries e
		JOIN lease_contracts c ON c.id = e.contract_id
		LEFT JOIN stores s ON s.id = c.store_id
		WHERE e.batch_id = $1
	`, batchID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var attributes access.ContractAttributes
		if err := rows.Scan(&attributes.LegalEntityID, &attributes.StoreID, &attributes.Region, &attributes.Brand); err != nil {
			return false, err
		}
		if !scope.AllowsContract(attributes) {
			return false, nil
		}
	}
	return found, rows.Err()
}

func (r *MonthlyClosingRepository) CreateBatch(ctx context.Context, batch *MonthlyClosingBatch) (*MonthlyClosingBatch, error) {
	batch.ID = uuid.New().String()
	batch.Status = "draft"
	batch.CreatedAt = time.Now()
	batch.UpdatedAt = time.Now()

	query := `
		INSERT INTO monthly_closing_batches (
			id, batch_number, accounting_period, legal_entity_id, scope_contract_id,
			region, brand, status, total_contracts, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Exec(ctx, query,
		batch.ID, batch.BatchNumber, batch.AccountingPeriod,
		batch.LegalEntityID, batch.ScopeContractID, batch.Region, batch.Brand,
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
		SELECT id, batch_number, accounting_period, legal_entity_id, scope_contract_id, region, brand,
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
			&b.ID, &b.BatchNumber, &b.AccountingPeriod, &b.LegalEntityID, &b.ScopeContractID, &b.Region, &b.Brand,
			&b.Status, &b.TotalContracts, &b.ProcessedContracts, &b.FailedContracts,
			&b.TotalEntries, &b.PostedEntries, &b.StartedAt, &b.CompletedAt,
			&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan batch: %w", err)
		}
		batches = append(batches, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if _, scoped := access.ScopeFromContext(ctx); !scoped {
		return batches, nil
	}
	filtered := make([]*MonthlyClosingBatch, 0, len(batches))
	for _, batch := range batches {
		allowed, err := r.batchAllowed(ctx, batch.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate batch access: %w", err)
		}
		if allowed {
			filtered = append(filtered, batch)
		}
	}
	return filtered, nil
}

// GetReusableBatch returns the latest unfinalized batch for the exact close
// scope. A nil scope contract identifies a tenant-wide period close; a non-nil
// scope identifies a single-contract close.
func (r *MonthlyClosingRepository) GetReusableBatch(ctx context.Context, period, legalEntityID, contractID string) (*MonthlyClosingBatch, error) {
	var legalEntityIDVal, contractIDVal interface{}
	if legalEntityID != "" {
		legalEntityIDVal = legalEntityID
	}
	if contractID != "" {
		contractIDVal = contractID
	}
	query := `
		SELECT id, batch_number, accounting_period, legal_entity_id, scope_contract_id,
			region, brand, status, total_contracts, processed_contracts, failed_contracts,
			total_entries, posted_entries, started_at, completed_at,
			created_by, created_at, updated_at
		FROM monthly_closing_batches
		WHERE accounting_period = $1
		  AND legal_entity_id IS NOT DISTINCT FROM $2::uuid
		  AND scope_contract_id IS NOT DISTINCT FROM $3::uuid
		  AND status IN ('draft', 'completed', 'completed_with_errors', 'failed')
		ORDER BY created_at DESC
		LIMIT 1
	`
	b := &MonthlyClosingBatch{}
	err := r.db.QueryRow(ctx, query, period, legalEntityIDVal, contractIDVal).Scan(
		&b.ID, &b.BatchNumber, &b.AccountingPeriod, &b.LegalEntityID, &b.ScopeContractID,
		&b.Region, &b.Brand, &b.Status, &b.TotalContracts, &b.ProcessedContracts,
		&b.FailedContracts, &b.TotalEntries, &b.PostedEntries, &b.StartedAt,
		&b.CompletedAt, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reusable batch: %w", err)
	}
	if _, scoped := access.ScopeFromContext(ctx); scoped {
		allowed, err := r.batchAllowed(ctx, b.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate reusable batch access: %w", err)
		}
		if !allowed {
			return nil, nil
		}
	}
	return b, nil
}

func (r *MonthlyClosingRepository) ResetBatch(ctx context.Context, batchID string, totalContracts int) error {
	query := `
		UPDATE monthly_closing_batches SET
			status = 'draft', total_contracts = $2,
			processed_contracts = 0, failed_contracts = 0,
			total_entries = 0, posted_entries = 0,
			started_at = NOW(), completed_at = NULL, updated_at = NOW()
		WHERE id = $1
	`
	result, err := r.db.Exec(ctx, query, batchID, totalContracts)
	if err != nil {
		return fmt.Errorf("failed to reset reusable batch: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("reusable batch %s not found", batchID)
	}
	return nil
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

// ListMeasurementResultsByEntityPeriod reads the official engine outputs of
// every contract in one legal entity for one accounting period — the source
// the finmodel LeaseRollforwardReader folds into entity-month values (the
// model never touches the measurement tables itself, bottom line 5).
func (r *MonthlyClosingRepository) ListMeasurementResultsByEntityPeriod(ctx context.Context, legalEntityID, period string) ([]*MeasurementResult, error) {
	rows, err := r.db.Query(ctx, `SELECT mr.id, mr.contract_id, mr.accounting_period, mr.period_start_date, mr.period_end_date,
			mr.opening_liability, mr.interest_expense, mr.principal_repayment, mr.total_payment,
			mr.closing_liability, mr.opening_rou_asset, mr.depreciation, mr.closing_rou_asset,
			mr.variable_rent_expense, mr.non_lease_expense, mr.discount_rate,
			mr.is_calculated, mr.calculation_batch_id, mr.calculated_at, mr.created_at, mr.updated_at
		FROM measurement_results mr
		JOIN lease_contracts c ON c.id = mr.contract_id
		WHERE c.legal_entity_id=$1 AND mr.accounting_period=$2
		ORDER BY mr.contract_id`, legalEntityID, period)
	if err != nil {
		return nil, fmt.Errorf("list measurement results by entity period: %w", err)
	}
	defer rows.Close()
	var out []*MeasurementResult
	for rows.Next() {
		item := &MeasurementResult{}
		if err := rows.Scan(&item.ID, &item.ContractID, &item.AccountingPeriod, &item.PeriodStartDate, &item.PeriodEndDate,
			&item.OpeningLiability, &item.InterestExpense, &item.PrincipalRepayment, &item.TotalPayment,
			&item.ClosingLiability, &item.OpeningROUAsset, &item.Depreciation, &item.ClosingROUAsset,
			&item.VariableRentExpense, &item.NonLeaseExpense, &item.DiscountRate,
			&item.IsCalculated, &item.CalculationBatchID, &item.CalculatedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan measurement result: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *MonthlyClosingRepository) GetMeasurementResults(ctx context.Context, contractID, period string) ([]*MeasurementResult, error) {
	query := `
		SELECT mr.id, mr.contract_id, mr.accounting_period, mr.period_start_date, mr.period_end_date,
			mr.opening_liability, mr.interest_expense, mr.principal_repayment, mr.total_payment,
			mr.closing_liability, mr.opening_rou_asset, mr.depreciation, mr.closing_rou_asset,
			mr.variable_rent_expense, mr.non_lease_expense, mr.discount_rate,
			mr.is_calculated, mr.calculation_batch_id, mr.calculated_at, mr.created_at, mr.updated_at
		FROM measurement_results mr
		JOIN lease_contracts lc ON lc.id = mr.contract_id
		WHERE mr.contract_id = $1
	`
	args := []interface{}{contractID}
	argIdx := 2
	if period != "" {
		query += " AND mr.accounting_period = $2"
		args = append(args, period)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "lc")
	query += " ORDER BY mr.accounting_period ASC"

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
			description, voucher_number, posting_status, batch_id,
			posted_at, posted_by, reversal_of_entry_id, reversal_reason,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`

	_, err := r.db.Exec(ctx, query,
		entry.ID, entry.ContractID, entry.MeasurementResultID, entry.AccountingPeriod, entry.EntryDate,
		entry.EntryType, entry.DebitAccount, entry.CreditAccount, entry.Amount, entry.Currency,
		entry.Description, entry.VoucherNumber, entry.PostingStatus, entry.BatchID,
		entry.PostedAt, entry.PostedBy, entry.ReversalOfEntryID, entry.ReversalReason,
		entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create journal entry: %w", err)
	}
	return nil
}

// DeleteDraftEntriesByTypes removes draft journal entries of the given types for
// a contract and period. It is used to make a re-run of the month-end close
// idempotent: previously generated draft entries are cleared before fresh ones
// are produced. Entries that have already been approved or posted are never
// touched, so committed accounting is preserved. When legalEntityID is given the
// delete is additionally guarded by tenant ownership of the contract, so a
// foreign contract id can never delete another tenant's entries.
func (r *MonthlyClosingRepository) DeleteDraftEntriesByTypes(ctx context.Context, contractID, period, legalEntityID string, entryTypes []string) error {
	if len(entryTypes) == 0 {
		return nil
	}
	query := `
		DELETE FROM journal_entries
		WHERE contract_id = $1 AND accounting_period = $2
		  AND posting_status = 'draft'
		  AND entry_type = ANY($3)
	` + tenantOwnsContractClause(legalEntityID, "$4")
	args := []interface{}{contractID, period, entryTypes}
	if legalEntityID != "" {
		args = append(args, legalEntityID)
	}
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to delete draft entries: %w", err)
	}
	return nil
}

// HasFinalizedEntries reports whether regeneration would overlap entries that
// have already entered approval or posting workflow. Such closes must be
// reversed explicitly rather than silently generating a second draft set.
func (r *MonthlyClosingRepository) HasFinalizedEntries(ctx context.Context, contractIDs []string, period, legalEntityID string, entryTypes []string) (bool, error) {
	if len(contractIDs) == 0 || len(entryTypes) == 0 {
		return false, nil
	}
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM journal_entries
			WHERE contract_id::text = ANY($1)
			  AND accounting_period = $2
			  AND posting_status <> 'draft'
			  AND entry_type = ANY($3)
	` + tenantOwnsContractClause(legalEntityID, "$4") + `
		)
	`
	args := []interface{}{contractIDs, period, entryTypes}
	if legalEntityID != "" {
		args = append(args, legalEntityID)
	}
	var exists bool
	if err := r.db.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check finalized entries: %w", err)
	}
	return exists, nil
}

func (r *MonthlyClosingRepository) HasFinalizedBatchEntries(ctx context.Context, batchID string, entryTypes []string) (bool, error) {
	if len(entryTypes) == 0 {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM journal_entries
			WHERE batch_id = $1 AND posting_status <> 'draft' AND entry_type = ANY($2)
		)
	`, batchID, entryTypes).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check finalized batch entries: %w", err)
	}
	return exists, nil
}

func (r *MonthlyClosingRepository) DeleteDraftEntriesFromBatch(ctx context.Context, batchID string, entryTypes []string) error {
	if len(entryTypes) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		DELETE FROM journal_entries
		WHERE batch_id = $1 AND posting_status = 'draft' AND entry_type = ANY($2)
	`, batchID, entryTypes)
	if err != nil {
		return fmt.Errorf("failed to clear reusable batch entries: %w", err)
	}
	return nil
}

func (r *MonthlyClosingRepository) DetachDraftEntriesFromBatch(ctx context.Context, batchID string, entryTypes []string) error {
	if len(entryTypes) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE journal_entries SET batch_id = NULL, updated_at = NOW()
		WHERE batch_id = $1 AND posting_status = 'draft' AND entry_type = ANY($2)
	`, batchID, entryTypes)
	if err != nil {
		return fmt.Errorf("failed to detach reusable batch event entries: %w", err)
	}
	return nil
}

// LinkDraftEntriesToBatch attaches draft event entries to the reusable close
// batch. Re-running moves any entry left on a historical batch back onto the
// current scope batch instead of copying it. It returns the number of entries
// linked. When legalEntityID is given the update is guarded by tenant ownership.
func (r *MonthlyClosingRepository) LinkDraftEntriesToBatch(ctx context.Context, contractID, period, batchID, legalEntityID string, entryTypes []string) (int, error) {
	if len(entryTypes) == 0 {
		return 0, nil
	}
	query := `
		UPDATE journal_entries SET
			batch_id = $1,
			updated_at = NOW()
			WHERE contract_id = $2 AND accounting_period = $3
			  AND posting_status = 'draft'
			  AND entry_type = ANY($4)
	` + tenantOwnsContractClause(legalEntityID, "$5")
	args := []interface{}{batchID, contractID, period, entryTypes}
	if legalEntityID != "" {
		args = append(args, legalEntityID)
	}
	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to link draft entries to batch: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// tenantOwnsContractClause returns a SQL predicate restricting journal_entries to
// contracts owned by the given legal entity, or an empty string when no tenant
// scope is supplied. journal_entries has no legal_entity_id column, so ownership
// is checked through lease_contracts.
func tenantOwnsContractClause(legalEntityID, placeholder string) string {
	if legalEntityID == "" {
		return ""
	}
	return fmt.Sprintf(" AND contract_id IN (SELECT id FROM lease_contracts WHERE legal_entity_id = %s)", placeholder)
}

func (r *MonthlyClosingRepository) GetJournalEntries(ctx context.Context, contractID, period, status string) ([]*JournalEntry, error) {
	query := `
		SELECT je.id, je.contract_id, je.measurement_result_id, je.accounting_period, je.entry_date,
			je.entry_type, je.debit_account, je.credit_account, je.amount, je.currency,
			je.description, je.voucher_number, je.posting_status, je.posted_at, je.posted_by,
			je.approved_by, je.approved_at, je.erp_reference, je.batch_id,
			je.reversal_of_entry_id, je.reversal_reason, je.reversed_at, je.reversed_by,
			je.created_at, je.updated_at
		FROM journal_entries je
		JOIN lease_contracts lc ON lc.id = je.contract_id
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if contractID != "" {
		query += fmt.Sprintf(" AND je.contract_id = $%d", argIdx)
		args = append(args, contractID)
		argIdx++
	}
	if period != "" {
		query += fmt.Sprintf(" AND je.accounting_period = $%d", argIdx)
		args = append(args, period)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND je.posting_status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query, args, argIdx = appendContractScopePredicate(ctx, query, args, argIdx, "lc")
	_ = argIdx
	query += " ORDER BY je.entry_date ASC, je.entry_type ASC"

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
			&e.ApprovedBy, &e.ApprovedAt, &e.ERPReference, &e.BatchID,
			&e.ReversalOfEntryID, &e.ReversalReason, &e.ReversedAt, &e.ReversedBy,
			&e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *MonthlyClosingRepository) GetJournalEntriesForExport(ctx context.Context, legalEntityID, period, status string) ([]*JournalEntry, error) {
	query := `
		SELECT je.id, je.contract_id, je.measurement_result_id, je.accounting_period, je.entry_date,
			je.entry_type, je.debit_account, je.credit_account, je.amount, je.currency,
			je.description, je.voucher_number, je.posting_status, je.posted_at, je.posted_by,
			je.approved_by, je.approved_at, je.erp_reference, je.batch_id,
			je.reversal_of_entry_id, je.reversal_reason, je.reversed_at, je.reversed_by,
			je.created_at, je.updated_at
		FROM journal_entries je
		JOIN lease_contracts lc ON lc.id = je.contract_id
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if _, scoped := access.ScopeFromContext(ctx); !scoped && legalEntityID != "" {
		query += fmt.Sprintf(" AND lc.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, argIdx = appendContractScopePredicate(ctx, query, args, argIdx, "lc")
	if period != "" {
		query += fmt.Sprintf(" AND je.accounting_period = $%d", argIdx)
		args = append(args, period)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND je.posting_status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query += " ORDER BY je.entry_date ASC, je.entry_type ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list export entries: %w", err)
	}
	defer rows.Close()

	var entries []*JournalEntry
	for rows.Next() {
		e := &JournalEntry{}
		err := rows.Scan(
			&e.ID, &e.ContractID, &e.MeasurementResultID, &e.AccountingPeriod, &e.EntryDate,
			&e.EntryType, &e.DebitAccount, &e.CreditAccount, &e.Amount, &e.Currency,
			&e.Description, &e.VoucherNumber, &e.PostingStatus, &e.PostedAt, &e.PostedBy,
			&e.ApprovedBy, &e.ApprovedAt, &e.ERPReference, &e.BatchID,
			&e.ReversalOfEntryID, &e.ReversalReason, &e.ReversedAt, &e.ReversedBy,
			&e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan export entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ApproveJournalEntry approves a single journal entry in draft status
// GetJournalEntryByID returns a single journal entry, or nil when it does not
// exist or falls outside the caller's data scope.
func (r *MonthlyClosingRepository) GetJournalEntryByID(ctx context.Context, entryID string) (*JournalEntry, error) {
	allowed, err := r.journalEntryAllowed(ctx, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate journal entry access: %w", err)
	}
	if !allowed {
		return nil, nil
	}

	e := &JournalEntry{}
	err = r.db.QueryRow(ctx, `
		SELECT id, contract_id, measurement_result_id, accounting_period, entry_date,
			entry_type, debit_account, credit_account, amount, currency,
			description, voucher_number, posting_status, posted_at, posted_by,
			approved_by, approved_at, erp_reference, batch_id,
			reversal_of_entry_id, reversal_reason, reversed_at, reversed_by,
			created_at, updated_at
		FROM journal_entries WHERE id = $1
	`, entryID).Scan(
		&e.ID, &e.ContractID, &e.MeasurementResultID, &e.AccountingPeriod, &e.EntryDate,
		&e.EntryType, &e.DebitAccount, &e.CreditAccount, &e.Amount, &e.Currency,
		&e.Description, &e.VoucherNumber, &e.PostingStatus, &e.PostedAt, &e.PostedBy,
		&e.ApprovedBy, &e.ApprovedAt, &e.ERPReference, &e.BatchID,
		&e.ReversalOfEntryID, &e.ReversalReason, &e.ReversedAt, &e.ReversedBy,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load journal entry: %w", err)
	}
	return e, nil
}

// MarkJournalEntryReversed flags a posted entry as reversed. The status guard is
// part of the statement, so two concurrent reversals cannot both succeed: the
// second updates no row and is reported as a conflict.
func (r *MonthlyClosingRepository) MarkJournalEntryReversed(ctx context.Context, entryID, userID string) error {
	var userIDVal interface{}
	if userID != "" {
		userIDVal = userID
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE journal_entries SET
			posting_status = 'reversed',
			reversed_by = $1,
			reversed_at = NOW(),
			updated_at = NOW()
		WHERE id = $2 AND posting_status = 'posted'
	`, userIDVal, entryID)
	if err != nil {
		return fmt.Errorf("failed to mark journal entry reversed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("journal entry is no longer in posted status")
	}
	return nil
}

func (r *MonthlyClosingRepository) ApproveJournalEntry(ctx context.Context, entryID string, userID string) error {
	allowed, err := r.journalEntryAllowed(ctx, entryID)
	if err != nil {
		return fmt.Errorf("failed to validate journal entry access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("journal entry not found or not in draft status")
	}
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

// RejectJournalEntry sends an approved entry back to draft and clears its
// approval marks, so the approval must be earned again rather than lingering on
// a rejected entry.
//
// Rejection is only available before posting; a posted entry has an accounting
// effect and must be reversed instead. The rejection reason lives in the audit
// log rather than on the row, because a rejected entry returns to draft and is
// normally regenerated by the next close, which would leave any stored reason
// stale.
func (r *MonthlyClosingRepository) RejectJournalEntry(ctx context.Context, entryID string) error {
	allowed, err := r.journalEntryAllowed(ctx, entryID)
	if err != nil {
		return fmt.Errorf("failed to validate journal entry access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("journal entry not found or not in approved status")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE journal_entries SET
			posting_status = 'draft',
			approved_by = NULL,
			approved_at = NULL,
			updated_at = NOW()
		WHERE id = $1 AND posting_status = 'approved'
	`, entryID)
	if err != nil {
		return fmt.Errorf("failed to reject journal entry: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("journal entry not found or not in approved status")
	}
	return nil
}

// PostJournalEntry posts an approved journal entry
func (r *MonthlyClosingRepository) PostJournalEntry(ctx context.Context, entryID string, userID string, erpRef string) error {
	allowed, err := r.journalEntryAllowed(ctx, entryID)
	if err != nil {
		return fmt.Errorf("failed to validate journal entry access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("journal entry not found or not in approved status")
	}
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

func (r *MonthlyClosingRepository) ApplyERPWriteback(ctx context.Context, entryID, userID, erpReference, voucherNumber string) error {
	allowed, err := r.journalEntryAllowed(ctx, entryID)
	if err != nil {
		return fmt.Errorf("failed to validate journal entry access: %w", err)
	}
	if !allowed {
		return fmt.Errorf("journal entry not found or not approved")
	}
	query := `
		UPDATE journal_entries SET
			posting_status = 'posted',
			posted_by = $1,
			posted_at = NOW(),
			erp_reference = $2,
			voucher_number = $3,
			updated_at = NOW()
		WHERE id = $4 AND posting_status IN ('approved', 'posted')
	`
	var erpRefVal interface{}
	if erpReference == "" {
		erpRefVal = nil
	} else {
		erpRefVal = erpReference
	}
	var voucherVal interface{}
	if voucherNumber == "" {
		voucherVal = nil
	} else {
		voucherVal = voucherNumber
	}
	result, err := r.db.Exec(ctx, query, userID, erpRefVal, voucherVal, entryID)
	if err != nil {
		return fmt.Errorf("failed to apply ERP writeback: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("journal entry not found or not approved")
	}
	return nil
}

// ApproveBatchEntries approves all draft journal entries in a batch
func (r *MonthlyClosingRepository) ApproveBatchEntries(ctx context.Context, batchID string, userID string) (int, error) {
	allowed, err := r.batchAllowed(ctx, batchID)
	if err != nil {
		return 0, fmt.Errorf("failed to validate batch access: %w", err)
	}
	if !allowed {
		return 0, fmt.Errorf("batch not found")
	}
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
	allowed, err := r.batchAllowed(ctx, batchID)
	if err != nil {
		return 0, fmt.Errorf("failed to validate batch access: %w", err)
	}
	if !allowed {
		return 0, fmt.Errorf("batch not found")
	}
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
		WITH guard AS (
			SELECT pg_advisory_xact_lock(hashtextextended(
				'monthend:' || COALESCE($2::text, '*') || ':' || $1,
				0
			))
		), blocking AS (
			SELECT EXISTS (
				SELECT 1
				FROM close_exceptions
				WHERE accounting_period = $1
				  AND ($2::uuid IS NULL OR legal_entity_id = $2::uuid)
				  AND severity = 'blocking'
				  AND NOT (
					 exception_state = 'closed'
					 AND closing_disposition IN ('verified_resolution', 'accounting_conclusion', 'period_waiver', 'standing_waiver')
				  )
			) AS found
		)
		INSERT INTO period_locks (accounting_period, legal_entity_id, is_locked, locked_by, locked_at, created_at, updated_at)
		SELECT $1, $2::uuid, true, $3, NOW(), NOW(), NOW()
		FROM guard, blocking
		WHERE NOT blocking.found
		ON CONFLICT (accounting_period, legal_entity_id) DO UPDATE SET
			is_locked = true,
			locked_by = $3,
			locked_at = NOW(),
			unlocked_by = NULL,
			unlocked_at = NULL,
			updated_at = NOW()
		RETURNING id::text
	`
	var legalEntityIDVal interface{}
	if legalEntityID == "" {
		legalEntityIDVal = nil
	} else {
		legalEntityIDVal = legalEntityID
	}
	var lockID string
	err := r.db.QueryRow(ctx, query, period, legalEntityIDVal, userID).Scan(&lockID)
	if err == pgx.ErrNoRows {
		return ErrUnresolvedBlockingExceptions
	}
	if err != nil {
		return fmt.Errorf("failed to lock period: %w", err)
	}
	return nil
}

// UnlockPeriod unlocks an accounting period for the given legal entity
func (r *MonthlyClosingRepository) UnlockPeriod(ctx context.Context, period, legalEntityID, userID string) error {
	query := `
		WITH guard AS (
			SELECT pg_advisory_xact_lock(hashtextextended(
				'monthend:' || COALESCE($3::text, '*') || ':' || $2,
				0
			))
		), updated AS (
			UPDATE period_locks SET
				is_locked = false,
				unlocked_by = $1,
				unlocked_at = NOW(),
				updated_at = NOW()
			FROM guard
			WHERE accounting_period = $2 AND (legal_entity_id = $3::uuid OR legal_entity_id IS NULL)
			RETURNING 1
		)
		SELECT COUNT(*) FROM updated
	`
	var legalEntityIDVal interface{}
	if legalEntityID == "" {
		legalEntityIDVal = nil
	} else {
		legalEntityIDVal = legalEntityID
	}
	var count int
	err := r.db.QueryRow(ctx, query, userID, period, legalEntityIDVal).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to unlock period: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("period lock not found for period %s", period)
	}
	return nil
}

// IsPeriodLocked checks if a period is locked
func (r *MonthlyClosingRepository) IsPeriodLocked(ctx context.Context, period, legalEntityID string) (bool, error) {
	query := `
		SELECT COALESCE(bool_or(is_locked), false) FROM period_locks
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
		return false, fmt.Errorf("failed to check period lock: %w", err)
	}
	return isLocked, nil
}

func (r *MonthlyClosingRepository) UpdateBatchStatus(ctx context.Context, batchID, status string, processed, failed, total, posted int) error {
	now := time.Now()
	var completedAt interface{}
	if isTerminalBatchStatus(status) {
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

func isTerminalBatchStatus(status string) bool {
	switch status {
	case "completed", "completed_with_errors", "failed", "cancelled":
		return true
	default:
		return false
	}
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

// GetEventAdjustmentsForContracts loads event adjustments for a report snapshot in one query.
func (r *MonthlyClosingRepository) GetEventAdjustmentsForContracts(ctx context.Context, contractIDs []string) (map[string][]*EventAdjustment, error) {
	result := make(map[string][]*EventAdjustment, len(contractIDs))
	if len(contractIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, event_id, contract_id, adjustment_type, effective_date,
			liability_before, liability_after, liability_adjustment,
			rou_before, rou_after, rou_adjustment,
			pnl_gain, pnl_loss, revised_discount_rate, discount_rate_source,
			calculation_batch_id, created_at
		FROM event_adjustments
		WHERE contract_id::text = ANY($1)
		ORDER BY contract_id, effective_date ASC
	`, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch list event adjustments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		adjustment := &EventAdjustment{}
		if err := rows.Scan(
			&adjustment.ID, &adjustment.EventID, &adjustment.ContractID, &adjustment.AdjustmentType, &adjustment.EffectiveDate,
			&adjustment.LiabilityBefore, &adjustment.LiabilityAfter, &adjustment.LiabilityAdjustment,
			&adjustment.ROUBefore, &adjustment.ROUAfter, &adjustment.ROUAdjustment,
			&adjustment.PnLGain, &adjustment.PnLLoss, &adjustment.RevisedDiscountRate, &adjustment.DiscountRateSource,
			&adjustment.CalculationBatchID, &adjustment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event adjustment: %w", err)
		}
		result[adjustment.ContractID] = append(result[adjustment.ContractID], adjustment)
	}
	return result, rows.Err()
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
