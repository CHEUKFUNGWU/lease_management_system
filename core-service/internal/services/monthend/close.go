// Package monthend owns the IFRS 16 month-end close as a single deep behavior.
//
// One command, Service.Close, takes a period (optionally a single contract) and
// produces a complete, atomic close: it validates the period lock, decides which
// contracts are eligible, runs the IFRS 16 measurement for each, produces the
// matching journal entries, links any pending event-adjustment entries, records
// the batch result, and writes the audit entry — all inside one database
// transaction. If any write fails, the whole transaction rolls back, so partial
// batches cannot exist. A re-run before the period is locked replaces its own
// draft entries instead of duplicating them.
//
// The HTTP layer is left with only request parsing and response shaping; all
// close rules, transactionality, and the audit result live behind this seam.
package monthend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// ErrPeriodLocked is returned by Close when the target period has been locked
// and may not be regenerated.
var ErrPeriodLocked = errors.New("accounting period is locked")

// systemEntryTypes are the journal entry types the close itself produces. They
// are cleared (when still in draft) before regeneration so a re-run is
// idempotent.
var systemEntryTypes = []string{
	"interest", "depreciation", "payment",
	"variable_rent", "non_lease", "lease_expense",
}

// eventEntryTypes are entries created by lease events (modification,
// reassessment, impairment) outside the close. The close links the pending
// drafts to its batch rather than re-creating them.
var eventEntryTypes = []string{"modification", "reassessment", "impairment"}

const fallbackDiscountRate = 0.05

// Read-side dependencies. Each is satisfied by the corresponding concrete
// repository and by test fakes.
type (
	periodLockReader interface {
		IsPeriodLocked(ctx context.Context, period, legalEntityID string) (bool, error)
	}
	contractSource interface {
		GetByID(ctx context.Context, id, legalEntityID string) (*repository.Contract, error)
		GetByStatuses(ctx context.Context, statuses []string, legalEntityID string) ([]*repository.Contract, error)
	}
	scheduleSource interface {
		GetByContractID(ctx context.Context, contractID string) ([]*repository.PaymentSchedule, error)
	}
	rateSource interface {
		GetFloat64(ctx context.Context, key string, fallback float64) float64
	}
)

// Service performs the month-end close. Reads happen up front through the read
// dependencies; all writes happen through the unit of work, which owns the
// transaction.
type Service struct {
	locks     periodLockReader
	contracts contractSource
	schedules scheduleSource
	rates     rateSource
	uow       unitOfWork
}

// NewService wires the close service from concrete repositories.
func NewService(
	pool *pgxpool.Pool,
	mcRepo *repository.MonthlyClosingRepository,
	contractRepo contractSource,
	psRepo scheduleSource,
	systemSettingRepo rateSource,
	auditLogger *audit.Logger,
) *Service {
	return &Service{
		locks:     mcRepo,
		contracts: contractRepo,
		schedules: psRepo,
		rates:     systemSettingRepo,
		uow:       NewUnitOfWork(pool, mcRepo, auditLogger),
	}
}

// Command is the complete input to a month-end close.
type Command struct {
	AccountingPeriod     string  // "2006-01"
	ContractID           string  // optional: close a single contract
	LegalEntityID        string  // tenant scope
	DiscountRateOverride float64 // optional per-run discount rate

	// Actor identifies who triggered the close, for the audit result.
	Actor audit.Metadata
}

// Result summarizes a completed close batch.
type Result struct {
	BatchID            string `json:"batch_id"`
	BatchNumber        string `json:"batch_number"`
	AccountingPeriod   string `json:"accounting_period"`
	Status             string `json:"status"`
	TotalContracts     int    `json:"total_contracts"`
	ProcessedContracts int    `json:"processed_contracts"`
	FailedContracts    int    `json:"failed_contracts"`
	TotalEntries       int    `json:"total_entries"`
}

// Close runs the full month-end close for the command's period as one atomic
// transaction. It returns ErrPeriodLocked if the period is locked. A per-contract
// measurement failure is counted and skipped without aborting the batch; any
// database write failure rolls the whole batch back and returns the error.
func (s *Service) Close(ctx context.Context, cmd Command) (*Result, error) {
	locked, err := s.locks.IsPeriodLocked(ctx, cmd.AccountingPeriod, cmd.LegalEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to check period lock: %w", err)
	}
	if locked {
		return nil, ErrPeriodLocked
	}

	contracts, err := s.eligibleContracts(ctx, cmd)
	if err != nil {
		return nil, err
	}

	periodStart, err := time.Parse("2006-01", cmd.AccountingPeriod)
	if err != nil {
		return nil, fmt.Errorf("invalid accounting period %q: %w", cmd.AccountingPeriod, err)
	}
	periodEnd := periodStart.AddDate(0, 1, -1)

	// Resolve the global discount rate fallback once per close (it is the same
	// for every contract in the batch).
	globalRate := s.resolveGlobalDiscountRate(ctx)

	var result *Result
	err = s.uow.Do(ctx, func(w closeWriter, a auditSink) error {
		var legalEntityID *string
		if cmd.LegalEntityID != "" {
			legalEntityID = &cmd.LegalEntityID
		}
		batch, err := w.CreateBatch(ctx, &repository.MonthlyClosingBatch{
			BatchNumber:      fmt.Sprintf("BATCH-%s-%s", cmd.AccountingPeriod, time.Now().Format("20060102-150405")),
			AccountingPeriod: cmd.AccountingPeriod,
			LegalEntityID:    legalEntityID,
			TotalContracts:   len(contracts),
		})
		if err != nil {
			return fmt.Errorf("failed to create batch: %w", err)
		}

		processed := 0
		failed := 0
		totalEntries := 0

		for _, contract := range contracts {
			// Skip contracts whose term does not overlap the period at all.
			if contract.LeaseEndDate.Before(periodStart) || contract.CommencementDate.After(periodEnd) {
				continue
			}

			monthly, basis, discountRate, soft := s.measureContract(ctx, contract, cmd, globalRate)
			if soft {
				failed++
				continue
			}
			if monthly == nil {
				// "skipped" basis or period outside the contract term: nothing to post.
				if basis == "skipped" {
					processed++
				}
				continue
			}

			now := time.Now()
			mr := measurementResult(contract.ID, cmd.AccountingPeriod, periodStart, periodEnd, monthly, discountRate, batch.ID, now)
			if err := w.SaveMeasurementResult(ctx, mr); err != nil {
				return fmt.Errorf("contract %s: %w", contract.ID, err)
			}

			// Idempotency: clear our own previously generated draft entries before
			// producing fresh ones, then write the new set.
			if err := w.DeleteDraftEntriesByTypes(ctx, contract.ID, cmd.AccountingPeriod, cmd.LegalEntityID, systemEntryTypes); err != nil {
				return fmt.Errorf("contract %s: %w", contract.ID, err)
			}
			entries := buildJournalEntries(contract.ID, cmd.AccountingPeriod, periodEnd, monthly, batch.ID, basis)
			for _, entry := range entries {
				if err := w.CreateJournalEntry(ctx, entry); err != nil {
					return fmt.Errorf("contract %s: %w", contract.ID, err)
				}
				totalEntries++
			}

			processed++
		}

		// Link pending event-adjustment drafts (modification/reassessment/impairment)
		// to this batch. Unlike the previous copy approach this update is idempotent,
		// so re-running the close cannot duplicate event entries.
		for _, contract := range contracts {
			linked, err := w.LinkDraftEntriesToBatch(ctx, contract.ID, cmd.AccountingPeriod, batch.ID, cmd.LegalEntityID, eventEntryTypes)
			if err != nil {
				return fmt.Errorf("contract %s: %w", contract.ID, err)
			}
			totalEntries += linked
		}

		status := closeStatus(processed, failed)
		if err := w.UpdateBatchStatus(ctx, batch.ID, status, processed, failed, totalEntries, 0); err != nil {
			return fmt.Errorf("failed to update batch status: %w", err)
		}

		// Audit the close as part of the same transaction, so the audit result
		// commits atomically with the batch it describes.
		if a != nil {
			if err := a.LogEvent(ctx, "monthly_closing_batches", batch.ID, "generate", nil, map[string]interface{}{
				"batch_id":          batch.ID,
				"accounting_period": cmd.AccountingPeriod,
				"status":            status,
				"processed":         processed,
				"total_entries":     totalEntries,
			}, cmd.Actor); err != nil {
				return fmt.Errorf("failed to write audit log: %w", err)
			}
		}

		result = &Result{
			BatchID:            batch.ID,
			BatchNumber:        batch.BatchNumber,
			AccountingPeriod:   cmd.AccountingPeriod,
			Status:             status,
			TotalContracts:     len(contracts),
			ProcessedContracts: processed,
			FailedContracts:    failed,
			TotalEntries:       totalEntries,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// eligibleContracts returns the contracts the close should consider: a single
// contract when requested, otherwise every approved contract for the tenant.
func (s *Service) eligibleContracts(ctx context.Context, cmd Command) ([]*repository.Contract, error) {
	if cmd.ContractID != "" {
		contract, err := s.contracts.GetByID(ctx, cmd.ContractID, cmd.LegalEntityID)
		if err != nil {
			return nil, err
		}
		if contract == nil {
			return nil, nil
		}
		return []*repository.Contract{contract}, nil
	}
	return s.contracts.GetByStatuses(ctx, []string{"approved"}, cmd.LegalEntityID)
}

// measureContract runs the IFRS 16 calculation for one contract and returns the
// monthly entry for the close period. The soft flag is true when the contract
// should be counted as failed and skipped (a recoverable, per-contract problem)
// rather than aborting the whole batch.
func (s *Service) measureContract(ctx context.Context, contract *repository.Contract, cmd Command, globalRate float64) (monthly *ifrs16svc.MonthlyEntry, basis string, discountRate float64, soft bool) {
	schedules, err := s.schedules.GetByContractID(ctx, contract.ID)
	if err != nil {
		return nil, "", 0, true
	}

	var payments []ifrs16svc.LeasePayment
	if len(schedules) == 0 {
		payments = mockPayments(contract)
	} else {
		payments = repository.ToIFRS16Payments(schedules)
	}

	discountRate = resolveDiscountRate(cmd.DiscountRateOverride, globalRate, contract.DiscountRateValue)

	calculation := ifrs16svc.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     contract.LeaseEndDate,
		LeaseScope:       contract.LeaseScope,
		DiscountRate:     discountRate,
		Payments:         payments,
		PrepaidRent: ifrs16svc.CalculatePrepaidRent(ifrs16svc.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			Payments:         payments,
		}),
	}

	result, err := ifrs16svc.Calculate(calculation)
	if err != nil {
		return nil, "", 0, true
	}
	if result.MeasurementBasis == "skipped" {
		return nil, "skipped", discountRate, false
	}

	for i := range result.MonthlySummary {
		m := result.MonthlySummary[i]
		if fmt.Sprintf("%04d-%02d", m.Year, m.Month) == cmd.AccountingPeriod {
			return &m, result.MeasurementBasis, discountRate, false
		}
	}
	// Period is not part of this contract's term.
	return nil, result.MeasurementBasis, discountRate, false
}

func (s *Service) resolveGlobalDiscountRate(ctx context.Context) float64 {
	if s.rates == nil {
		return 0.0
	}
	return s.rates.GetFloat64(ctx, "global_discount_rate", 0.0)
}

// resolveDiscountRate applies the rate priority: per-run override, then the
// global system setting, then the contract's own rate, then a 5% fallback.
func resolveDiscountRate(override, globalRate float64, contractRate *float64) float64 {
	rate := override
	if rate <= 0 {
		rate = globalRate
	}
	if rate <= 0 && contractRate != nil && *contractRate > 0 {
		rate = *contractRate
	}
	if rate <= 0 {
		rate = fallbackDiscountRate
	}
	return rate
}

// closeStatus derives the batch status from per-contract outcomes.
func closeStatus(processed, failed int) string {
	switch {
	case failed > 0 && processed == 0:
		return "failed"
	case failed > 0:
		return "completed_with_errors"
	default:
		return "completed"
	}
}
