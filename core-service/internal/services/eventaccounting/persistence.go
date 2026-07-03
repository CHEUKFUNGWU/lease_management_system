package eventaccounting

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type persistenceStore interface {
	GetEventAdjustmentByEventID(ctx context.Context, eventID string) (*repository.EventAdjustment, error)
	CreateEventAdjustment(ctx context.Context, adjustment *repository.EventAdjustment) (*repository.EventAdjustment, error)
	SaveMeasurementResult(ctx context.Context, result *repository.MeasurementResult) error
	CreateJournalEntry(ctx context.Context, entry *repository.JournalEntry) error
	LinkRecalculationBatch(ctx context.Context, eventID, adjustmentID string) error
}

type auditSink interface {
	LogEvent(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, meta audit.Metadata) error
}

type persistenceUnitOfWork interface {
	Do(ctx context.Context, eventID string, body func(persistenceStore, auditSink) error) error
}

// PersistenceService atomically commits a calculated event accounting plan.
// Its interface accepts the same Result returned to preview callers, ensuring
// committed accounting cannot diverge from the preview calculation path.
type PersistenceService struct {
	uow persistenceUnitOfWork
}

func NewPersistenceService(
	pool *pgxpool.Pool,
	mcRepo *repository.MonthlyClosingRepository,
	eventRepo *repository.EventRepository,
	auditLogger *audit.Logger,
) *PersistenceService {
	return &PersistenceService{uow: newPGXPersistenceUnitOfWork(pool, mcRepo, eventRepo, auditLogger)}
}

func (s *PersistenceService) Commit(ctx context.Context, result Result, actor audit.Metadata) (*repository.EventAdjustment, error) {
	var committed *repository.EventAdjustment
	err := s.uow.Do(ctx, result.Adjustment.EventID, func(store persistenceStore, logger auditSink) error {
		existing, err := store.GetEventAdjustmentByEventID(ctx, result.Adjustment.EventID)
		if err != nil {
			return fmt.Errorf("check existing event adjustment: %w", err)
		}
		if existing != nil {
			committed = existing
			return nil
		}

		adjustment := repositoryAdjustment(result.Adjustment)
		committed, err = store.CreateEventAdjustment(ctx, adjustment)
		if err != nil {
			return fmt.Errorf("create event adjustment: %w", err)
		}

		calculatedAt := time.Now()
		for _, measurement := range monthlyMeasurements(result.ForwardSchedule) {
			measurement.ContractID = result.Adjustment.ContractID
			measurement.DiscountRate = result.Adjustment.RevisedDiscountRate
			measurement.IsCalculated = true
			measurement.CalculatedAt = &calculatedAt
			if err := store.SaveMeasurementResult(ctx, measurement); err != nil {
				return fmt.Errorf("save measurement result for %s: %w", measurement.AccountingPeriod, err)
			}
		}

		for _, planned := range result.JournalEntries {
			description := planned.Description
			entry := &repository.JournalEntry{
				ContractID: result.Adjustment.ContractID, AccountingPeriod: planned.AccountingPeriod,
				EntryDate: planned.EntryDate, EntryType: planned.EntryType,
				DebitAccount: planned.DebitAccount, CreditAccount: planned.CreditAccount,
				Amount: planned.Amount, Currency: planned.Currency,
				Description: &description, PostingStatus: "draft",
			}
			if err := store.CreateJournalEntry(ctx, entry); err != nil {
				return fmt.Errorf("create event journal entry: %w", err)
			}
		}

		if err := store.LinkRecalculationBatch(ctx, result.Adjustment.EventID, committed.ID); err != nil {
			return fmt.Errorf("link event recalculation: %w", err)
		}
		if err := logger.LogEvent(ctx, "lease_events", result.Adjustment.EventID, "recalculate", nil, committed, actor); err != nil {
			return fmt.Errorf("audit event recalculation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return committed, nil
}

func repositoryAdjustment(adjustment Adjustment) *repository.EventAdjustment {
	return &repository.EventAdjustment{
		EventID: adjustment.EventID, ContractID: adjustment.ContractID,
		AdjustmentType: adjustment.Treatment, EffectiveDate: adjustment.EffectiveDate,
		LiabilityBefore: adjustment.LiabilityBefore, LiabilityAfter: adjustment.LiabilityAfter,
		LiabilityAdjustment: adjustment.LiabilityAdjustment,
		ROUBefore:           adjustment.ROUBefore, ROUAfter: adjustment.ROUAfter,
		ROUAdjustment: adjustment.ROUAdjustment, PnLGain: adjustment.PnLGain,
		PnLLoss: adjustment.PnLLoss, RevisedDiscountRate: adjustment.RevisedDiscountRate,
	}
}

func monthlyMeasurements(schedule []ifrs16.DailyEntry) []*repository.MeasurementResult {
	measurements := make([]*repository.MeasurementResult, 0)
	var current *repository.MeasurementResult
	for _, daily := range schedule {
		period := daily.Date.Format("2006-01")
		if current == nil || current.AccountingPeriod != period {
			periodStart := time.Date(daily.Date.Year(), daily.Date.Month(), 1, 0, 0, 0, 0, daily.Date.Location())
			current = &repository.MeasurementResult{
				AccountingPeriod: period, PeriodStartDate: periodStart,
				PeriodEndDate:    periodStart.AddDate(0, 1, -1),
				OpeningLiability: daily.OpeningLiability, OpeningROUAsset: daily.OpeningROUAsset,
			}
			measurements = append(measurements, current)
		}
		current.InterestExpense += daily.InterestExpense
		current.TotalPayment += daily.Payment
		current.Depreciation += daily.Depreciation
		current.VariableRentExpense += daily.VariableRentExpense
		current.NonLeaseExpense += daily.NonLeaseExpense
		current.ClosingLiability = daily.ClosingLiability
		current.ClosingROUAsset = daily.ClosingROUAsset
	}
	for _, measurement := range measurements {
		measurement.PrincipalRepayment = measurement.TotalPayment - measurement.InterestExpense
	}
	return measurements
}
