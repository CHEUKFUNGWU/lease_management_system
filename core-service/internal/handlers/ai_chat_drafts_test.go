package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

func TestApplyReviewDraftUsesDraftServiceForPaymentArtifact(t *testing.T) {
	store := &handlerDraftStore{results: make(map[string]draftapp.ItemResult)}
	handler := &AIChatHandler{
		contractRepo: repository.NewContractRepository(nil),
		draftService: draftapp.NewService(handlerDraftUOW{store: store}, nil),
	}
	artifact := &repository.AIChatArtifact{
		ID: "artifact-1", RunID: "run-1", ArtifactType: "payment_schedule_draft",
		Data: mustJSON(map[string]any{
			"schedules": []any{map[string]any{
				"period_start": "2026-01-01", "period_end": "2026-01-31", "due_date": "2026-01-01",
				"amount": 1000, "payment_timing": "prepaid", "is_fixed": true,
				"is_lease_component": true, "amount_type": "fixed_rent", "currency": "CNY",
			}},
			"summary": map[string]any{"contract_id": "contract-1", "intake_id": "intake-1"},
		}),
	}
	ctx := access.WithScope(context.Background(), access.Scope{LegalEntityID: "le-1"})

	result, err := handler.applyReviewDraft(ctx, artifact, "import", map[string]interface{}{
		"selected_indexes": []any{0},
	}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.CreatedCount != 1 || result.FailedCount != 0 {
		t.Fatalf("unexpected draft result: %+v", result)
	}
	if len(store.schedules) != 1 || store.schedules[0].ApprovalStatus != "draft" || store.schedules[0].IsOfficialVersion {
		t.Fatalf("review action bypassed draft boundary: %+v", store.schedules)
	}
}

func TestApplyReviewDraftUsesDraftServiceForEventArtifact(t *testing.T) {
	store := &handlerDraftStore{results: make(map[string]draftapp.ItemResult)}
	handler := &AIChatHandler{draftService: draftapp.NewService(handlerDraftUOW{store: store}, nil)}
	artifact := &repository.AIChatArtifact{
		ID: "artifact-event-1", RunID: "run-1", ArtifactType: "event_draft",
		Data: mustJSON(map[string]any{"event": map[string]any{
			"contract_id": "contract-1", "event_type": "modification", "effective_date": "2026-02-01",
			"change_reason": "租金条款发生变化", "judgment_basis": "出租方通知",
		}}),
	}
	ctx := access.WithScope(context.Background(), access.Scope{LegalEntityID: "le-1"})

	result, err := handler.applyReviewDraft(ctx, artifact, "confirm", map[string]interface{}{"selected_indexes": []any{0}}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.CreatedCount != 1 || result.Operation != draftapp.OperationEventDraft {
		t.Fatalf("unexpected event draft result: %+v", result)
	}
	if len(store.events) != 1 || store.events[0].ApprovalStatus != "draft" || store.events[0].IsOfficialVersion {
		t.Fatalf("review action bypassed event draft boundary: %+v", store.events)
	}
}

func TestApplyReviewDraftIgnoresNonWriteReviewActions(t *testing.T) {
	handler := &AIChatHandler{}
	result, err := handler.applyReviewDraft(context.Background(), &repository.AIChatArtifact{ArtifactType: "contract_draft"}, "skip", nil, "user-1")
	if err != nil || result != nil {
		t.Fatalf("skip should not invoke draft service: result=%+v err=%v", result, err)
	}
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

type handlerDraftUOW struct{ store *handlerDraftStore }

func (u handlerDraftUOW) Execute(_ context.Context, fn func(draftapp.DraftStore) error) error {
	return fn(u.store)
}

type handlerDraftStore struct {
	schedules []*repository.PaymentSchedule
	events    []*repository.LeaseEvent
	results   map[string]draftapp.ItemResult
}

func (s *handlerDraftStore) LookupIdempotency(_ context.Context, operation, key string) (*draftapp.ItemResult, bool, error) {
	result, ok := s.results[operation+"\x00"+key]
	if !ok {
		return nil, false, nil
	}
	copy := result
	return &copy, true, nil
}

func (s *handlerDraftStore) CreateContractDraft(_ context.Context, contract *repository.Contract) (*repository.Contract, error) {
	copy := *contract
	copy.ID = "contract-1"
	return &copy, nil
}

func (s *handlerDraftStore) CreatePaymentScheduleDraft(_ context.Context, schedule *repository.PaymentSchedule) (*repository.PaymentSchedule, error) {
	copy := *schedule
	copy.ID = "schedule-1"
	copy.CreatedAt = time.Now().UTC()
	s.schedules = append(s.schedules, &copy)
	return &copy, nil
}

func (s *handlerDraftStore) CreateEventDraft(_ context.Context, event *repository.LeaseEvent) (*repository.LeaseEvent, error) {
	copy := *event
	copy.ID = "event-1"
	s.events = append(s.events, &copy)
	return &copy, nil
}

func (s *handlerDraftStore) SaveIdempotency(_ context.Context, operation, key string, result draftapp.ItemResult) error {
	s.results[operation+"\x00"+key] = result
	return nil
}
