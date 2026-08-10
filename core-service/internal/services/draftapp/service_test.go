package draftapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestCreateContractDraftEnforcesDraftBoundaryAndPreservesEvidence(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, nil)
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1", StoreIDs: []string{"store-1"}})
	approvedBy := "approver-1"
	approvedAt := time.Now().UTC()
	contract := validContract()
	contract.ApprovalStatus = "approved"
	contract.IsOfficialVersion = true
	contract.IncludedInReporting = true
	contract.ApprovedBy = &approvedBy
	contract.ApprovedAt = &approvedAt

	result := service.CreateContractDraft(ctx, ContractDraftCommand{
		IdempotencyKey:  "run-1:contract:0",
		ActorID:         "editor-1",
		Contract:        contract,
		AccessAttrs:     &access.ContractAttributes{LegalEntityID: "le-1", StoreID: "store-1"},
		RequireEvidence: true,
	})

	if result.Status != ItemCreated || result.ID == "" {
		t.Fatalf("expected created result, got %+v", result)
	}
	if len(store.contracts) != 1 {
		t.Fatalf("expected one persisted contract, got %d", len(store.contracts))
	}
	persisted := store.contracts[0]
	if persisted.Status != "draft" || persisted.ApprovalStatus != "draft" || persisted.IsOfficialVersion {
		t.Fatalf("draft boundary was not enforced: %+v", persisted)
	}
	if persisted.IncludedInReporting || persisted.ReportMode != "working" {
		t.Fatalf("draft must not enter official reporting: %+v", persisted)
	}
	if persisted.ApprovedBy != nil || persisted.ApprovedAt != nil {
		t.Fatal("approval metadata must not be copied into a new draft")
	}
	if persisted.SourceReferenceLocator == nil {
		t.Fatal("source evidence was dropped")
	}
}

func TestCreateContractDraftRejectsMissingScopeAndDimensionMismatch(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, nil)
	contract := validContract()

	result := service.CreateContractDraft(context.Background(), ContractDraftCommand{
		IdempotencyKey: "no-scope",
		ActorID:        "editor-1",
		Contract:       contract,
	})
	if result.Status != ItemFailed || !strings.Contains(result.Error, ErrScopeRequired.Error()) {
		t.Fatalf("expected scope failure, got %+v", result)
	}

	ctx := scopedContext(access.Scope{LegalEntityID: "le-1", Regions: []string{"north"}})
	result = service.CreateContractDraft(ctx, ContractDraftCommand{
		IdempotencyKey: "wrong-region",
		ActorID:        "editor-1",
		Contract:       contract,
		AccessAttrs:    &access.ContractAttributes{LegalEntityID: "le-1", StoreID: "store-1", Region: "south"},
	})
	if result.Status != ItemFailed || !strings.Contains(result.Error, "outside the assigned data scope") {
		t.Fatalf("expected dimension scope failure, got %+v", result)
	}
	if len(store.contracts) != 0 {
		t.Fatal("invalid drafts must not reach the writer")
	}
}

func TestCreatePaymentScheduleDraftNormalizesAccountingAttributesAndCurrency(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, fakeContractReader{contract: validContract()})
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1"})
	schedule := validPaymentSchedule()
	schedule.Currency = "cny"
	schedule.IsVariable = true
	schedule.IsFixed = false
	schedule.IsLeaseComponent = true
	schedule.IncludedInLiabilityPV = true

	result := service.CreatePaymentScheduleDraft(ctx, PaymentScheduleDraftCommand{
		IdempotencyKey:  "run-1:payment:0",
		ActorID:         "editor-1",
		Schedule:        schedule,
		RequireEvidence: true,
		EvidenceRef:     map[string]any{"artifact_id": "artifact-1"},
	})
	if result.Status != ItemCreated {
		t.Fatalf("expected created result, got %+v", result)
	}
	persisted := store.schedules[0]
	if persisted.Currency != "CNY" || persisted.ApprovalStatus != "draft" || persisted.IsOfficialVersion {
		t.Fatalf("draft schedule boundary was not enforced: %+v", persisted)
	}
	if persisted.IncludedInLiabilityPV {
		t.Fatal("variable rent must not enter lease-liability PV")
	}
}

func TestCreatePaymentScheduleDraftRejectsCurrencyMismatchBeforeWrite(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, fakeContractReader{contract: validContract()})
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1"})
	schedule := validPaymentSchedule()
	schedule.Currency = "USD"

	result := service.CreatePaymentScheduleDraft(ctx, PaymentScheduleDraftCommand{
		IdempotencyKey: "wrong-currency",
		ActorID:        "editor-1",
		Schedule:       schedule,
	})
	if result.Status != ItemFailed || !strings.Contains(result.Error, "currency must match") {
		t.Fatalf("expected currency failure, got %+v", result)
	}
	if len(store.schedules) != 0 {
		t.Fatal("currency-invalid schedule must not reach the writer")
	}
}

func TestCreateEventDraftEnforcesReviewBoundaryAndEvidence(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, fakeContractReader{contract: validContract()})
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1"})
	reason := "门店面积发生变化"
	event := &repository.LeaseEvent{
		ContractID: "contract-1", EventType: "area_adjustment",
		EffectiveDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), ChangeReason: &reason,
		ApprovalStatus: "approved", IsOfficialVersion: true,
	}

	missingEvidence := service.CreateEventDraft(ctx, EventDraftCommand{
		IdempotencyKey: "event-no-evidence", ActorID: "editor-1", Event: event, RequireEvidence: true,
	})
	if missingEvidence.Status != ItemFailed || !strings.Contains(missingEvidence.Error, "source evidence") {
		t.Fatalf("expected evidence failure, got %+v", missingEvidence)
	}

	result := service.CreateEventDraft(ctx, EventDraftCommand{
		IdempotencyKey: "event-1", ActorID: "editor-1", Event: event,
		EvidenceRef: map[string]any{"artifact_id": "artifact-1"}, RequireEvidence: true,
	})
	if result.Status != ItemCreated || result.ID == "" {
		t.Fatalf("expected event draft creation, got %+v", result)
	}
	persisted := store.events[0]
	if persisted.ApprovalStatus != "draft" || persisted.IsOfficialVersion || persisted.Status != "pending" {
		t.Fatalf("event draft boundary was not enforced: %+v", persisted)
	}
	if persisted.SourceReferenceLocator == nil || persisted.ApprovedBy != nil {
		t.Fatalf("event evidence or approval reset was lost: %+v", persisted)
	}
}

func TestCreateContractDraftIsIdempotentAndBatchReportsPartialFailure(t *testing.T) {
	store := newFakeStore()
	service := NewService(fakeUOW{store: store}, nil)
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1"})
	command := ContractDraftCommand{
		IdempotencyKey: "same-key",
		ActorID:        "editor-1",
		Contract:       validContract(),
	}

	first := service.CreateContractDraft(ctx, command)
	second := service.CreateContractDraft(ctx, command)
	if first.Status != ItemCreated || second.Status != ItemReplayed || first.ID != second.ID {
		t.Fatalf("expected idempotent replay, first=%+v second=%+v", first, second)
	}
	if len(store.contracts) != 1 {
		t.Fatalf("idempotent retry created %d rows", len(store.contracts))
	}

	batch := service.CreateContractDraftBatch(ctx, []ContractDraftCommand{
		{IdempotencyKey: "batch-valid", ActorID: "editor-1", Contract: validContract()},
		{IdempotencyKey: "batch-invalid", ActorID: "editor-1", Contract: &repository.Contract{}},
	})
	if batch.CreatedCount != 1 || batch.FailedCount != 1 || len(batch.Items) != 2 {
		t.Fatalf("unexpected partial batch result: %+v", batch)
	}
	if batch.BatchID == "" || batch.Status != "partial_failed" {
		t.Fatalf("batch recovery metadata missing: %+v", batch)
	}
	if batch.Items[0].Index != 0 || batch.Items[1].Index != 1 {
		t.Fatalf("batch indexes were not assigned: %+v", batch.Items)
	}
}

func TestCreateDraftRollsBackResultWhenPersistenceFails(t *testing.T) {
	store := newFakeStore()
	store.saveErr = errors.New("idempotency unavailable")
	service := NewService(fakeUOW{store: store}, nil)
	ctx := scopedContext(access.Scope{LegalEntityID: "le-1"})

	result := service.CreateContractDraft(ctx, ContractDraftCommand{
		IdempotencyKey: "save-fails",
		ActorID:        "editor-1",
		Contract:       validContract(),
	})
	if result.Status != ItemFailed || !strings.Contains(result.Error, "save draft idempotency") {
		t.Fatalf("expected persistence failure, got %+v", result)
	}
}

func TestGetDraftBatchRequiresScopeAndReturnsOwnerEnvelope(t *testing.T) {
	store := newFakeStore()
	store.batches["batch-1"] = &DraftBatch{
		BatchID: "batch-1", Operation: OperationContractDraft, Status: "partial_failed",
		CreatedBy: "editor-1", Items: []ItemResult{{Index: 0, Status: ItemCreated}},
	}
	service := NewService(fakeUOW{store: store}, nil)
	if _, err := service.GetDraftBatch(context.Background(), "batch-1", "editor-1"); !errors.Is(err, ErrScopeRequired) {
		t.Fatalf("expected scope requirement, got %v", err)
	}
	batch, err := service.GetDraftBatch(scopedContext(access.Scope{LegalEntityID: "le-1"}), "batch-1", "editor-1")
	if err != nil || batch == nil || batch.Status != "partial_failed" {
		t.Fatalf("expected owner batch, batch=%+v err=%v", batch, err)
	}
	if _, err := service.GetDraftBatch(scopedContext(access.Scope{LegalEntityID: "le-1"}), "batch-1", "other-user"); !errors.Is(err, ErrDraftBatchNotFound) {
		t.Fatalf("expected owner check, got %v", err)
	}
}

func scopedContext(scope access.Scope) context.Context {
	return access.WithScope(context.Background(), scope)
}

func validContract() *repository.Contract {
	legalEntityID := "le-1"
	storeID := "store-1"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &repository.Contract{
		ContractNumber:         "LC-001",
		ContractName:           "Sydney flagship",
		LegalEntityID:          &legalEntityID,
		StoreID:                &storeID,
		LesseeName:             "Retail Group",
		LessorName:             "Landlord Pty Ltd",
		AssetType:              "real_estate",
		Currency:               "CNY",
		CommencementDate:       start,
		LeaseStartDate:         start,
		LeaseEndDate:           start.AddDate(3, 0, 0),
		LeaseScope:             "in_scope",
		SourceReferenceLocator: map[string]any{"page": 1, "bbox": []float64{0, 0, 10, 10}},
	}
}

func validPaymentSchedule() *repository.PaymentSchedule {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &repository.PaymentSchedule{
		ContractID:            "contract-1",
		EffectiveStartDate:    start,
		EffectiveEndDate:      start.AddDate(1, 0, 0),
		CoverageStartDate:     start,
		CoverageEndDate:       start.AddDate(0, 1, 0).Add(-24 * time.Hour),
		DueDate:               start,
		PaymentTiming:         "prepaid",
		Amount:                1000,
		Currency:              "CNY",
		AmountType:            "fixed",
		IsFixed:               true,
		IsLeaseComponent:      true,
		IncludedInLiabilityPV: true,
	}
}

type fakeUOW struct {
	store *fakeStore
}

func (u fakeUOW) Execute(_ context.Context, fn func(DraftStore) error) error {
	return fn(u.store)
}

type fakeStore struct {
	contracts   []*repository.Contract
	schedules   []*repository.PaymentSchedule
	events      []*repository.LeaseEvent
	idempotency map[string]ItemResult
	batches     map[string]*DraftBatch
	saveErr     error
}

func newFakeStore() *fakeStore {
	return &fakeStore{idempotency: make(map[string]ItemResult), batches: make(map[string]*DraftBatch)}
}

func (s *fakeStore) LookupIdempotency(_ context.Context, operation, key string) (*ItemResult, bool, error) {
	result, ok := s.idempotency[operation+"\x00"+key]
	if !ok {
		return nil, false, nil
	}
	copy := result
	return &copy, true, nil
}

func (s *fakeStore) CreateContractDraft(_ context.Context, contract *repository.Contract) (*repository.Contract, error) {
	copy := *contract
	copy.ID = fmt.Sprintf("contract-%d", len(s.contracts)+1)
	s.contracts = append(s.contracts, &copy)
	return &copy, nil
}

func (s *fakeStore) CreatePaymentScheduleDraft(_ context.Context, schedule *repository.PaymentSchedule) (*repository.PaymentSchedule, error) {
	copy := *schedule
	copy.ID = fmt.Sprintf("payment-%d", len(s.schedules)+1)
	s.schedules = append(s.schedules, &copy)
	return &copy, nil
}

func (s *fakeStore) CreateEventDraft(_ context.Context, event *repository.LeaseEvent) (*repository.LeaseEvent, error) {
	copy := *event
	copy.ID = fmt.Sprintf("event-%d", len(s.events)+1)
	s.events = append(s.events, &copy)
	return &copy, nil
}

func (s *fakeStore) SaveIdempotency(_ context.Context, operation, key string, result ItemResult) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.idempotency[operation+"\x00"+key] = result
	return nil
}

func (s *fakeStore) GetDraftBatch(_ context.Context, batchID, actorID string) (*DraftBatch, error) {
	batch, ok := s.batches[batchID]
	if !ok || batch.CreatedBy != actorID {
		return nil, ErrDraftBatchNotFound
	}
	copy := *batch
	copy.Items = append([]ItemResult(nil), batch.Items...)
	return &copy, nil
}

type fakeContractReader struct {
	contract *repository.Contract
}

func (r fakeContractReader) GetContract(_ context.Context, _ string) (*repository.Contract, error) {
	return r.contract, nil
}
