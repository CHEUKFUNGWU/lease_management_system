package eventaccounting

import (
	"context"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type fakePersistenceStore struct {
	failOn string
	calls  []string
}

func (s *fakePersistenceStore) record(name string) error {
	s.calls = append(s.calls, name)
	if s.failOn == name {
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func (s *fakePersistenceStore) GetEventAdjustmentByEventID(context.Context, string) (*repository.EventAdjustment, error) {
	return nil, s.record("GetEventAdjustmentByEventID")
}

func (s *fakePersistenceStore) CreateEventAdjustment(_ context.Context, adjustment *repository.EventAdjustment) (*repository.EventAdjustment, error) {
	adjustment.ID = "adjustment-1"
	return adjustment, s.record("CreateEventAdjustment")
}

func (s *fakePersistenceStore) SaveMeasurementResult(context.Context, *repository.MeasurementResult) error {
	return s.record("SaveMeasurementResult")
}

func (s *fakePersistenceStore) CreateJournalEntry(context.Context, *repository.JournalEntry) error {
	return s.record("CreateJournalEntry")
}

func (s *fakePersistenceStore) LinkRecalculationBatch(context.Context, string, string) error {
	return s.record("LinkRecalculationBatch")
}

type fakeAuditSink struct {
	logged bool
	fail   bool
}

func (a *fakeAuditSink) LogEvent(context.Context, string, string, string, interface{}, interface{}, audit.Metadata) error {
	a.logged = true
	if a.fail {
		return errors.New("injected audit failure")
	}
	return nil
}

type fakePersistenceUOW struct {
	store     *fakePersistenceStore
	audit     *fakeAuditSink
	committed bool
}

func (u *fakePersistenceUOW) Do(ctx context.Context, _ string, body func(persistenceStore, auditSink) error) error {
	if err := body(u.store, u.audit); err != nil {
		return err
	}
	u.committed = true
	return nil
}

func TestCommitRollsBackEveryRecalculationWriteWhenJournalPersistenceFails(t *testing.T) {
	uow := &fakePersistenceUOW{
		store: &fakePersistenceStore{failOn: "CreateJournalEntry"},
		audit: &fakeAuditSink{},
	}
	service := &PersistenceService{uow: uow}
	result := accountingResultForPersistenceTest(t)

	adjustment, err := service.Commit(context.Background(), result, audit.Metadata{ChangedBy: "actor-1"})
	if err == nil {
		t.Fatal("Commit() error = nil, want journal persistence failure")
	}
	if adjustment != nil {
		t.Fatalf("adjustment = %#v, want nil on rollback", adjustment)
	}
	if uow.committed {
		t.Fatal("transaction committed after a persistence failure")
	}
	if uow.audit.logged {
		t.Fatal("audit was written for a rolled-back recalculation")
	}
	if containsCall(uow.store.calls, "LinkRecalculationBatch") {
		t.Fatalf("event was linked before all accounting writes succeeded: %v", uow.store.calls)
	}
}

func TestCommitPersistsCompletePlanAndAuditInOneTransaction(t *testing.T) {
	uow := &fakePersistenceUOW{store: &fakePersistenceStore{}, audit: &fakeAuditSink{}}
	service := &PersistenceService{uow: uow}

	adjustment, err := service.Commit(context.Background(), accountingResultForPersistenceTest(t), audit.Metadata{ChangedBy: "actor-1"})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if adjustment == nil || adjustment.ID != "adjustment-1" {
		t.Fatalf("adjustment = %#v", adjustment)
	}
	if !uow.committed || !uow.audit.logged {
		t.Fatalf("committed=%v audited=%v, want both true", uow.committed, uow.audit.logged)
	}
	for _, required := range []string{
		"CreateEventAdjustment", "SaveMeasurementResult", "CreateJournalEntry", "LinkRecalculationBatch",
	} {
		if !containsCall(uow.store.calls, required) {
			t.Errorf("missing %s call: %v", required, uow.store.calls)
		}
	}
}

func TestCommitRollsBackAccountingAndEventLinkWhenAuditFails(t *testing.T) {
	uow := &fakePersistenceUOW{store: &fakePersistenceStore{}, audit: &fakeAuditSink{fail: true}}
	service := &PersistenceService{uow: uow}

	adjustment, err := service.Commit(context.Background(), accountingResultForPersistenceTest(t), audit.Metadata{ChangedBy: "actor-1"})
	if err == nil {
		t.Fatal("Commit() error = nil, want audit failure")
	}
	if adjustment != nil {
		t.Fatalf("adjustment = %#v, want nil on rollback", adjustment)
	}
	if uow.committed {
		t.Fatal("transaction committed when its audit write failed")
	}
	if !containsCall(uow.store.calls, "LinkRecalculationBatch") {
		t.Fatalf("test did not reach the final event-link write: %v", uow.store.calls)
	}
}

func accountingResultForPersistenceTest(t *testing.T) Result {
	t.Helper()
	newEndDate := "2025-07-01"
	result, err := Calculate(Input{
		EventID: "event-1", ContractID: "contract-1", EventType: "early_termination",
		EffectiveDate: date("2025-01-01"), CommencementDate: date("2024-01-01"),
		LeaseEndDate: date("2026-01-01"), NewValue: &newEndDate,
		Currency: "CNY", LeaseScope: ifrs16.LeaseScopeInScope, DiscountRate: 0.05,
		Payments: fixedPaymentsForPersistenceTest(),
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	return result
}

func fixedPaymentsForPersistenceTest() []ifrs16.LeasePayment {
	return []ifrs16.LeasePayment{
		{Date: date("2025-06-30"), Amount: money.NewFromInt64(100000), Timing: "postpaid", Type: "fixed"},
		{Date: date("2025-12-31"), Amount: money.NewFromInt64(100000), Timing: "postpaid", Type: "fixed"},
	}
}

func containsCall(calls []string, name string) bool {
	for _, call := range calls {
		if call == name {
			return true
		}
	}
	return false
}
