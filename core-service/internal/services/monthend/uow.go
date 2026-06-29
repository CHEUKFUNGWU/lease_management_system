package monthend

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

// closeWriter is the set of writes a single close performs. Both the
// transaction-scoped MonthlyClosingRepository and a test fake satisfy it.
type closeWriter interface {
	CreateBatch(ctx context.Context, batch *repository.MonthlyClosingBatch) (*repository.MonthlyClosingBatch, error)
	SaveMeasurementResult(ctx context.Context, mr *repository.MeasurementResult) error
	DeleteDraftEntriesByTypes(ctx context.Context, contractID, period, legalEntityID string, entryTypes []string) error
	CreateJournalEntry(ctx context.Context, entry *repository.JournalEntry) error
	LinkDraftEntriesToBatch(ctx context.Context, contractID, period, batchID, legalEntityID string, entryTypes []string) (int, error)
	UpdateBatchStatus(ctx context.Context, batchID, status string, processed, failed, total, posted int) error
}

// auditSink records the audit result of a close. It is written inside the close
// transaction so the audit entry commits atomically with the batch.
type auditSink interface {
	LogEvent(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, meta audit.Metadata) error
}

// unitOfWork runs the body of a close inside a transaction. The body receives a
// transaction-scoped writer and audit sink; returning a non-nil error rolls the
// whole transaction back, so a partial batch can never be committed.
type unitOfWork interface {
	Do(ctx context.Context, body func(w closeWriter, a auditSink) error) error
}

// pgxUnitOfWork is the production unit of work backed by a pgx pool.
type pgxUnitOfWork struct {
	pool        *pgxpool.Pool
	mcRepo      *repository.MonthlyClosingRepository
	auditLogger *audit.Logger
}

// NewUnitOfWork builds the production unit of work.
func NewUnitOfWork(pool *pgxpool.Pool, mcRepo *repository.MonthlyClosingRepository, auditLogger *audit.Logger) unitOfWork {
	return &pgxUnitOfWork{pool: pool, mcRepo: mcRepo, auditLogger: auditLogger}
}

func (u *pgxUnitOfWork) Do(ctx context.Context, body func(w closeWriter, a auditSink) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin close transaction: %w", err)
	}
	// Rollback is a no-op once the transaction has been committed.
	defer tx.Rollback(ctx)

	if err := body(u.mcRepo.WithTx(tx), u.auditLogger.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit close transaction: %w", err)
	}
	return nil
}
