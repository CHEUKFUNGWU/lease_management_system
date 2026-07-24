package monthend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

// --- fakes -------------------------------------------------------------------

type fakeLocks struct {
	locked bool
	err    error
}

func (f fakeLocks) IsPeriodLocked(context.Context, string, string) (bool, error) {
	return f.locked, f.err
}

type fakeContracts struct {
	list []*repository.Contract
	err  error
}

func (f fakeContracts) GetByID(_ context.Context, id, _ string) (*repository.Contract, error) {
	for _, c := range f.list {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, f.err
}

func (f fakeContracts) GetByStatuses(context.Context, []string, string) ([]*repository.Contract, error) {
	return f.list, f.err
}

type fakeSchedules struct {
	// failFor returns an error for the given contract id, exercising the
	// soft-failure path.
	failFor map[string]bool
}

func (f fakeSchedules) GetByContractID(_ context.Context, contractID string) ([]*repository.PaymentSchedule, error) {
	if f.failFor[contractID] {
		return nil, errors.New("schedule read failed")
	}
	return []*repository.PaymentSchedule{{
		ContractID:            contractID,
		EffectiveStartDate:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EffectiveEndDate:      time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		CoverageStartDate:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CoverageEndDate:       time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		DueDate:               time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Amount:                100000,
		PaymentTiming:         "postpaid",
		AmountType:            "fixed",
		IsFixed:               true,
		IsLeaseComponent:      true,
		IncludedInLiabilityPV: true,
	}}, nil
}

type fakeRates struct{ value float64 }

func (f fakeRates) GetFloat64(context.Context, string, float64) float64 { return f.value }

// fakeWriter records every write in order and can be told to fail a named method.
type fakeWriter struct {
	calls         []string
	failOn        string
	batchSeq      int
	locked        bool
	lockErr       error
	finalized     bool
	reusableBatch *repository.MonthlyClosingBatch
}

func (w *fakeWriter) record(name string) error {
	w.calls = append(w.calls, name)
	if w.failOn == name {
		return errors.New("write failed: " + name)
	}
	return nil
}

func (w *fakeWriter) IsPeriodLocked(context.Context, string, string) (bool, error) {
	return w.locked, w.lockErr
}

func (w *fakeWriter) HasFinalizedEntries(context.Context, []string, string, string, []string) (bool, error) {
	return w.finalized, nil
}

func (w *fakeWriter) HasFinalizedBatchEntries(context.Context, string, []string) (bool, error) {
	return w.finalized, nil
}

func (w *fakeWriter) GetReusableBatch(context.Context, string, string, string) (*repository.MonthlyClosingBatch, error) {
	return w.reusableBatch, nil
}

func (w *fakeWriter) ResetBatch(context.Context, string, int) error {
	return w.record("ResetBatch")
}

func (w *fakeWriter) DeleteDraftEntriesFromBatch(context.Context, string, []string) error {
	return w.record("DeleteDraftEntriesFromBatch")
}

func (w *fakeWriter) DetachDraftEntriesFromBatch(context.Context, string, []string) error {
	return w.record("DetachDraftEntriesFromBatch")
}

func (w *fakeWriter) CreateBatch(_ context.Context, b *repository.MonthlyClosingBatch) (*repository.MonthlyClosingBatch, error) {
	if err := w.record("CreateBatch"); err != nil {
		return nil, err
	}
	w.batchSeq++
	b.ID = "batch-1"
	return b, nil
}

func (w *fakeWriter) SaveMeasurementResult(context.Context, *repository.MeasurementResult) error {
	return w.record("SaveMeasurementResult")
}

func (w *fakeWriter) DeleteDraftEntriesByTypes(context.Context, string, string, string, []string) error {
	return w.record("DeleteDraftEntriesByTypes")
}

func (w *fakeWriter) CreateJournalEntry(context.Context, *repository.JournalEntry) error {
	return w.record("CreateJournalEntry")
}

func (w *fakeWriter) LinkDraftEntriesToBatch(context.Context, string, string, string, string, []string) (int, error) {
	return 0, w.record("LinkDraftEntriesToBatch")
}

func (w *fakeWriter) UpdateBatchStatus(context.Context, string, string, int, int, int, int) error {
	return w.record("UpdateBatchStatus")
}

type fakeAudit struct{ logged bool }

func (a *fakeAudit) LogEvent(context.Context, string, string, string, interface{}, interface{}, audit.Metadata) error {
	a.logged = true
	return nil
}

// fakeUOW mirrors the real transaction semantics: it commits only when the body
// returns nil, otherwise it rolls back.
type fakeUOW struct {
	writer    *fakeWriter
	audit     *fakeAudit
	contracts fakeContracts
	schedules fakeSchedules
	rates     fakeRates
	committed bool
	ran       bool
}

type fakeCloseStore struct {
	*fakeWriter
	contracts fakeContracts
	schedules fakeSchedules
	rates     fakeRates
}

func (s *fakeCloseStore) GetByID(ctx context.Context, id, legalEntityID string) (*repository.Contract, error) {
	return s.contracts.GetByID(ctx, id, legalEntityID)
}

func (s *fakeCloseStore) GetByStatuses(ctx context.Context, statuses []string, legalEntityID string) ([]*repository.Contract, error) {
	return s.contracts.GetByStatuses(ctx, statuses, legalEntityID)
}

func (s *fakeCloseStore) GetByContractID(ctx context.Context, contractID string) ([]*repository.PaymentSchedule, error) {
	return s.schedules.GetByContractID(ctx, contractID)
}

func (s *fakeCloseStore) GetFloat64(ctx context.Context, key string, fallback float64) float64 {
	return s.rates.GetFloat64(ctx, key, fallback)
}

func (u *fakeUOW) Do(ctx context.Context, _ string, body func(store closeStore, a auditSink) error) error {
	u.ran = true
	err := body(&fakeCloseStore{
		fakeWriter: u.writer,
		contracts:  u.contracts,
		schedules:  u.schedules,
		rates:      u.rates,
	}, u.audit)
	if err == nil {
		u.committed = true
	}
	return err
}

// --- helpers -----------------------------------------------------------------

func inScopeContract(id string) *repository.Contract {
	rate := 0.05
	return &repository.Contract{
		ID:                id,
		CommencementDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseEndDate:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseScope:        "in_scope",
		DiscountRateValue: &rate,
	}
}

func newService(uow *fakeUOW, locks fakeLocks, contracts fakeContracts, schedules fakeSchedules) *Service {
	uow.writer.locked = locks.locked
	uow.writer.lockErr = locks.err
	uow.contracts = contracts
	uow.schedules = schedules
	uow.rates = fakeRates{value: 0}
	return &Service{uow: uow}
}

func contains(calls []string, name string) bool {
	for _, c := range calls {
		if c == name {
			return true
		}
	}
	return false
}

func indexOf(calls []string, name string) int {
	for i, c := range calls {
		if c == name {
			return i
		}
	}
	return -1
}

const period = "2026-01"

// --- tests -------------------------------------------------------------------

func TestClose_HappyPath_CommitsAndAudits(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{inScopeContract("c1")}}, fakeSchedules{})

	res, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1"})
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !uow.committed {
		t.Fatal("expected transaction to commit")
	}
	if !uow.audit.logged {
		t.Fatal("expected audit to be written inside the transaction")
	}
	if res.Status != "completed" {
		t.Errorf("status = %q, want completed", res.Status)
	}
	if res.ProcessedContracts != 1 || res.FailedContracts != 0 {
		t.Errorf("processed=%d failed=%d, want 1/0", res.ProcessedContracts, res.FailedContracts)
	}
	if res.TotalEntries == 0 {
		t.Error("expected at least one journal entry for an in-scope contract")
	}
}

func TestClose_RollsBackOnWriteFailure(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{failOn: "SaveMeasurementResult"}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{inScopeContract("c1")}}, fakeSchedules{})

	res, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1"})
	if err == nil {
		t.Fatal("expected error when a write fails")
	}
	if res != nil {
		t.Errorf("expected nil result on failure, got %+v", res)
	}
	if uow.committed {
		t.Fatal("transaction must NOT commit when a write fails (no partial batch)")
	}
	if uow.audit.logged {
		t.Error("audit must not be written when the close fails")
	}
}

func TestClose_IdempotentReplaceBeforeInsert(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{inScopeContract("c1")}}, fakeSchedules{})

	if _, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1"}); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	calls := uow.writer.calls
	del := indexOf(calls, "DeleteDraftEntriesByTypes")
	ins := indexOf(calls, "CreateJournalEntry")
	if del == -1 {
		t.Fatal("expected draft entries to be cleared for idempotency")
	}
	if ins != -1 && del > ins {
		t.Errorf("delete (%d) must precede insert (%d) so a re-run is idempotent", del, ins)
	}
	if !contains(calls, "LinkDraftEntriesToBatch") {
		t.Error("expected event-adjustment entries to be linked")
	}
}

func TestClose_RejectsRerunAfterApprovalOrPosting(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{finalized: true}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{inScopeContract("c1")}}, fakeSchedules{})

	result, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1"})
	if !errors.Is(err, ErrCloseAlreadyFinalized) {
		t.Fatalf("expected ErrCloseAlreadyFinalized, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected no result, got %+v", result)
	}
	if contains(uow.writer.calls, "CreateBatch") {
		t.Fatal("a finalized close must not create another batch")
	}
}

func TestClose_ReusesDraftBatchForSameScope(t *testing.T) {
	existing := &repository.MonthlyClosingBatch{ID: "existing-batch", BatchNumber: "BATCH-EXISTING"}
	uow := &fakeUOW{writer: &fakeWriter{reusableBatch: existing}, audit: &fakeAudit{}}
	contract := inScopeContract("c1")
	contract.ApprovalStatus = "approved"
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{contract}}, fakeSchedules{})

	result, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1", ContractID: "c1"})
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if result.BatchID != existing.ID {
		t.Fatalf("batch_id = %q, want %q", result.BatchID, existing.ID)
	}
	if contains(uow.writer.calls, "CreateBatch") {
		t.Fatal("draft rerun must not create a second batch")
	}
	if !contains(uow.writer.calls, "ResetBatch") {
		t.Fatal("draft rerun must reset the reusable batch")
	}
}

func TestClose_RejectsNonApprovedSingleContract(t *testing.T) {
	contract := inScopeContract("c1")
	contract.ApprovalStatus = "draft"
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, fakeContracts{list: []*repository.Contract{contract}}, fakeSchedules{})

	result, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1", ContractID: contract.ID})
	if !errors.Is(err, ErrContractNotApproved) {
		t.Fatalf("expected ErrContractNotApproved, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected no result, got %+v", result)
	}
	if contains(uow.writer.calls, "CreateBatch") {
		t.Fatal("non-approved contract must not create a close batch")
	}
}

func TestClose_PeriodLocked(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{locked: true}, fakeContracts{}, fakeSchedules{})

	_, err := svc.Close(context.Background(), Command{AccountingPeriod: period})
	if !errors.Is(err, ErrPeriodLocked) {
		t.Fatalf("expected ErrPeriodLocked, got %v", err)
	}
	if !uow.ran {
		t.Error("lock validation must run inside the serialized close transaction")
	}
}

func TestClose_SoftFailureContinues(t *testing.T) {
	contracts := fakeContracts{list: []*repository.Contract{inScopeContract("good"), inScopeContract("bad")}}
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{}, contracts, fakeSchedules{failFor: map[string]bool{"bad": true}})

	res, err := svc.Close(context.Background(), Command{AccountingPeriod: period, LegalEntityID: "le1"})
	if err != nil {
		t.Fatalf("a per-contract soft failure must not abort the batch: %v", err)
	}
	if !uow.committed {
		t.Fatal("expected transaction to commit despite one soft failure")
	}
	if res.ProcessedContracts != 1 || res.FailedContracts != 1 {
		t.Errorf("processed=%d failed=%d, want 1/1", res.ProcessedContracts, res.FailedContracts)
	}
	if res.Status != "completed_with_errors" {
		t.Errorf("status = %q, want completed_with_errors", res.Status)
	}
}
