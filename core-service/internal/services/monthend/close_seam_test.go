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
	return nil, nil // empty -> mockPayments path
}

type fakeRates struct{ value float64 }

func (f fakeRates) GetFloat64(context.Context, string, float64) float64 { return f.value }

// fakeWriter records every write in order and can be told to fail a named method.
type fakeWriter struct {
	calls    []string
	failOn   string
	batchSeq int
}

func (w *fakeWriter) record(name string) error {
	w.calls = append(w.calls, name)
	if w.failOn == name {
		return errors.New("write failed: " + name)
	}
	return nil
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
	committed bool
	ran       bool
}

func (u *fakeUOW) Do(ctx context.Context, body func(w closeWriter, a auditSink) error) error {
	u.ran = true
	err := body(u.writer, u.audit)
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
	return &Service{
		locks:     locks,
		contracts: contracts,
		schedules: schedules,
		rates:     fakeRates{value: 0},
		uow:       uow,
	}
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

func TestClose_PeriodLocked(t *testing.T) {
	uow := &fakeUOW{writer: &fakeWriter{}, audit: &fakeAudit{}}
	svc := newService(uow, fakeLocks{locked: true}, fakeContracts{}, fakeSchedules{})

	_, err := svc.Close(context.Background(), Command{AccountingPeriod: period})
	if !errors.Is(err, ErrPeriodLocked) {
		t.Fatalf("expected ErrPeriodLocked, got %v", err)
	}
	if uow.ran {
		t.Error("no transaction should be opened for a locked period")
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
