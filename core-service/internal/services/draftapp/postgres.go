package draftapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
)

// PostgresUnitOfWork provides the production transaction boundary. Each
// command uses one transaction; a batch intentionally invokes the service one
// item at a time so successful rows can be retained and failed rows can be
// retried independently.
type PostgresUnitOfWork struct {
	pool         *pgxpool.Pool
	contractRepo *repository.ContractRepository
	paymentRepo  *repository.PaymentScheduleRepository
	eventRepo    *repository.EventRepository
}

func NewPostgresUnitOfWork(
	pool *pgxpool.Pool,
	contractRepo *repository.ContractRepository,
	paymentRepo *repository.PaymentScheduleRepository,
	eventRepos ...*repository.EventRepository,
) *PostgresUnitOfWork {
	var eventRepo *repository.EventRepository
	if len(eventRepos) > 0 {
		eventRepo = eventRepos[0]
	}
	return &PostgresUnitOfWork{pool: pool, contractRepo: contractRepo, paymentRepo: paymentRepo, eventRepo: eventRepo}
}

func NewPostgresService(
	pool *pgxpool.Pool,
	contractRepo *repository.ContractRepository,
	paymentRepo *repository.PaymentScheduleRepository,
	eventRepos ...*repository.EventRepository,
) *Service {
	return NewService(
		NewPostgresUnitOfWork(pool, contractRepo, paymentRepo, eventRepos...),
		PostgresContractReader{repo: contractRepo},
	)
}

func (u *PostgresUnitOfWork) Execute(ctx context.Context, fn func(DraftStore) error) error {
	if u == nil || u.pool == nil || u.contractRepo == nil || u.paymentRepo == nil {
		return ErrDependenciesRequired
	}
	tx, err := u.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin draft transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	store := &postgresStore{
		db:             tx,
		contractWriter: u.contractRepo.WithTx(tx),
		paymentWriter:  u.paymentRepo.WithTx(tx),
		eventWriter:    u.eventRepo,
	}
	if u.eventRepo != nil {
		store.eventWriter = u.eventRepo.WithTx(tx)
	}
	if err := fn(store); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit draft transaction: %w", err)
	}
	return nil
}

func (u *PostgresUnitOfWork) ExecuteTransactional(ctx context.Context, fn func(DraftStore, repository.DBTX) error) error {
	if u == nil || u.pool == nil || u.contractRepo == nil || u.paymentRepo == nil {
		return ErrDependenciesRequired
	}
	tx, err := u.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin review transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	store := &postgresStore{
		db: tx, contractWriter: u.contractRepo.WithTx(tx), paymentWriter: u.paymentRepo.WithTx(tx), eventWriter: u.eventRepo,
	}
	if u.eventRepo != nil {
		store.eventWriter = u.eventRepo.WithTx(tx)
	}
	if err := fn(store, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit review transaction: %w", err)
	}
	return nil
}

type postgresStore struct {
	db             repository.DBTX
	contractWriter *repository.ContractRepository
	paymentWriter  *repository.PaymentScheduleRepository
	eventWriter    *repository.EventRepository
}

func (s *postgresStore) LookupIdempotency(ctx context.Context, operation, key string) (*ItemResult, bool, error) {
	// Lock by operation/key before checking for a row. This closes the race
	// between two first attempts with the same idempotency key: the waiter sees
	// the committed result and replays it instead of creating a duplicate draft.
	if _, err := s.db.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, operation+"\x00"+key,
	); err != nil {
		return nil, false, fmt.Errorf("lock draft idempotency key: %w", err)
	}

	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT result
		FROM agent_draft_idempotency
		WHERE operation = $1 AND idempotency_key = $2
	`, operation, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read draft idempotency: %w", err)
	}
	var result ItemResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false, fmt.Errorf("decode draft idempotency result: %w", err)
	}
	return &result, true, nil
}

func (s *postgresStore) CreateContractDraft(ctx context.Context, contract *repository.Contract) (*repository.Contract, error) {
	return s.contractWriter.Create(ctx, contract)
}

func (s *postgresStore) CreatePaymentScheduleDraft(ctx context.Context, schedule *repository.PaymentSchedule) (*repository.PaymentSchedule, error) {
	return s.paymentWriter.Create(ctx, schedule)
}

func (s *postgresStore) CreateEventDraft(ctx context.Context, event *repository.LeaseEvent) (*repository.LeaseEvent, error) {
	if s.eventWriter == nil {
		return nil, ErrDependenciesRequired
	}
	return s.eventWriter.Create(ctx, event)
}

func (s *postgresStore) SaveIdempotency(ctx context.Context, operation, key string, result ItemResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode draft idempotency result: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO agent_draft_idempotency (operation, idempotency_key, result)
		VALUES ($1, $2, $3::jsonb)
	`, operation, key, raw)
	if err != nil {
		return fmt.Errorf("insert draft idempotency: %w", err)
	}
	return nil
}

func (s *postgresStore) SaveDraftBatch(ctx context.Context, batch DraftBatch) error {
	items, err := json.Marshal(batch.Items)
	if err != nil {
		return fmt.Errorf("encode draft batch items: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO agent_draft_batches (
			batch_id, operation, status, items, created_by, run_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4::jsonb, NULLIF($5, ''), NULLIF($6, ''), NOW(), NOW())
		ON CONFLICT (batch_id) DO UPDATE SET
			status = EXCLUDED.status, items = EXCLUDED.items,
			created_by = COALESCE(agent_draft_batches.created_by, EXCLUDED.created_by),
			run_id = COALESCE(agent_draft_batches.run_id, EXCLUDED.run_id),
			updated_at = NOW()
	`, batch.BatchID, batch.Operation, batch.Status, items, batch.CreatedBy, batch.RunID)
	if err != nil {
		return fmt.Errorf("save draft batch: %w", err)
	}
	return nil
}

func (s *postgresStore) GetDraftBatch(ctx context.Context, batchID, actorID string) (*DraftBatch, error) {
	var batch DraftBatch
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT batch_id, operation, status, items,
		       COALESCE(created_by, ''), COALESCE(run_id, '')
		FROM agent_draft_batches
		WHERE batch_id = $1 AND created_by = $2
	`, batchID, actorID).Scan(
		&batch.BatchID, &batch.Operation, &batch.Status, &raw, &batch.CreatedBy, &batch.RunID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDraftBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read draft batch: %w", err)
	}
	if err := json.Unmarshal(raw, &batch.Items); err != nil {
		return nil, fmt.Errorf("decode draft batch items: %w", err)
	}
	return &batch, nil
}

// PostgresContractReader is a scope-aware adapter for payment-schedule
// validation. ContractRepository applies access.ScopeFromContext to the read.
type PostgresContractReader struct {
	repo *repository.ContractRepository
}

func (r PostgresContractReader) GetContract(ctx context.Context, contractID string) (*repository.Contract, error) {
	if r.repo == nil || strings.TrimSpace(contractID) == "" {
		return nil, ErrContractNotFound
	}
	return r.repo.GetByID(ctx, contractID, "")
}
