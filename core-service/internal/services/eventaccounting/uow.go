package eventaccounting

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

type pgxPersistenceUnitOfWork struct {
	pool        *pgxpool.Pool
	mcRepo      *repository.MonthlyClosingRepository
	eventRepo   *repository.EventRepository
	auditLogger *audit.Logger
}

func newPGXPersistenceUnitOfWork(
	pool *pgxpool.Pool,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	auditLogger *audit.Logger,
) persistenceUnitOfWork {
	return &pgxPersistenceUnitOfWork{pool: pool, mcRepo: mcRepo, eventRepo: eventRepo, auditLogger: auditLogger}
}

func (u *pgxPersistenceUnitOfWork) Do(ctx context.Context, eventID string, body func(persistenceStore, auditSink) error) error {
	// Read committed is intentional: a concurrent waiter must see the adjustment
	// committed by the lock holder after acquiring the per-event advisory lock.
	tx, err := u.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin event recalculation transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "event-recalculation:"+eventID); err != nil {
		return fmt.Errorf("serialize event recalculation: %w", err)
	}
	store := &txPersistenceStore{
		MonthlyClosingRepository: u.mcRepo.WithTx(tx),
		events:                   u.eventRepo.WithTx(tx),
	}
	if err := body(store, u.auditLogger.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit event recalculation transaction: %w", err)
	}
	return nil
}

type txPersistenceStore struct {
	*repository.MonthlyClosingRepository
	events *repository.EventRepository
}

func (s *txPersistenceStore) LinkRecalculationBatch(ctx context.Context, eventID, adjustmentID string) error {
	return s.events.LinkRecalculationBatch(ctx, eventID, adjustmentID)
}
