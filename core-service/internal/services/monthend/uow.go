package monthend

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

// closeWriter is the set of writes a single close performs.
type closeWriter interface {
	IsPeriodLocked(ctx context.Context, period, legalEntityID string) (bool, error)
	HasFinalizedEntries(ctx context.Context, contractIDs []string, period, legalEntityID string, entryTypes []string) (bool, error)
	HasFinalizedBatchEntries(ctx context.Context, batchID string, entryTypes []string) (bool, error)
	GetReusableBatch(ctx context.Context, period, legalEntityID, contractID string) (*repository.MonthlyClosingBatch, error)
	ResetBatch(ctx context.Context, batchID string, totalContracts int) error
	DeleteDraftEntriesFromBatch(ctx context.Context, batchID string, entryTypes []string) error
	DetachDraftEntriesFromBatch(ctx context.Context, batchID string, entryTypes []string) error
	CreateBatch(ctx context.Context, batch *repository.MonthlyClosingBatch) (*repository.MonthlyClosingBatch, error)
	SaveMeasurementResult(ctx context.Context, mr *repository.MeasurementResult) error
	DeleteDraftEntriesByTypes(ctx context.Context, contractID, period, legalEntityID string, entryTypes []string) error
	CreateJournalEntry(ctx context.Context, entry *repository.JournalEntry) error
	LinkDraftEntriesToBatch(ctx context.Context, contractID, period, batchID, legalEntityID string, entryTypes []string) (int, error)
	UpdateBatchStatus(ctx context.Context, batchID, status string, processed, failed, total, posted int) error
}

// closeStore is the complete transaction-scoped view used by a close. Keeping
// reads and writes on this one internal seam guarantees a consistent accounting
// input snapshot without enlarging Service.Close's public interface.
type closeStore interface {
	closeWriter
	contractSource
	scheduleSource
	rateSource
}

// auditSink records the audit result of a close. It is written inside the close
// transaction so the audit entry commits atomically with the batch.
type auditSink interface {
	LogEvent(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, meta audit.Metadata) error
}

// unitOfWork runs the body of a close inside a serialized transaction. The body
// receives a transaction-scoped store and audit sink; returning a non-nil error
// rolls the whole transaction back, so a partial batch can never be committed.
type unitOfWork interface {
	Do(ctx context.Context, lockKey string, body func(store closeStore, a auditSink) error) error
}

// pgxUnitOfWork is the production unit of work backed by a pgx pool.
type pgxUnitOfWork struct {
	pool        *pgxpool.Pool
	mcRepo      *repository.MonthlyClosingRepository
	contracts   *repository.ContractRepository
	schedules   *repository.PaymentScheduleRepository
	rates       *repository.SystemSettingRepository
	auditLogger *audit.Logger
}

// NewUnitOfWork builds the production unit of work.
func NewUnitOfWork(
	pool *pgxpool.Pool,
	mcRepo *repository.MonthlyClosingRepository,
	contracts *repository.ContractRepository,
	schedules *repository.PaymentScheduleRepository,
	rates *repository.SystemSettingRepository,
	auditLogger *audit.Logger,
) unitOfWork {
	return &pgxUnitOfWork{
		pool: pool, mcRepo: mcRepo, contracts: contracts,
		schedules: schedules, rates: rates, auditLogger: auditLogger,
	}
}

func (u *pgxUnitOfWork) Do(ctx context.Context, lockKey string, body func(store closeStore, a auditSink) error) error {
	tx, err := u.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin close transaction: %w", err)
	}
	// Rollback is a no-op once the transaction has been committed.
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return fmt.Errorf("failed to serialize close: %w", err)
	}

	store := &txCloseStore{
		MonthlyClosingRepository: u.mcRepo.WithTx(tx),
		contracts:                u.contracts.WithTx(tx),
		schedules:                u.schedules.WithTx(tx),
		rates:                    u.rates.WithTx(tx),
	}
	if err := body(store, u.auditLogger.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit close transaction: %w", err)
	}
	return nil
}

type txCloseStore struct {
	*repository.MonthlyClosingRepository
	contracts *repository.ContractRepository
	schedules *repository.PaymentScheduleRepository
	rates     *repository.SystemSettingRepository
}

func (s *txCloseStore) GetByID(ctx context.Context, id, legalEntityID string) (*repository.Contract, error) {
	return s.contracts.GetByID(ctx, id, legalEntityID)
}

func (s *txCloseStore) GetByStatuses(ctx context.Context, statuses []string, legalEntityID string) ([]*repository.Contract, error) {
	return s.contracts.GetByStatuses(ctx, statuses, legalEntityID)
}

func (s *txCloseStore) GetByContractID(ctx context.Context, contractID string) ([]*repository.PaymentSchedule, error) {
	return s.schedules.GetByContractID(ctx, contractID)
}

func (s *txCloseStore) GetFloat64(ctx context.Context, key string, fallback float64) float64 {
	return s.rates.GetFloat64(ctx, key, fallback)
}
