package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

func TestDraftDefinitionsRequireReviewAndIdempotency(t *testing.T) {
	contract := NewContractDraftDefinition(nil).Descriptor
	if err := contract.Validate(); err != nil {
		t.Fatalf("contract draft descriptor invalid: %v", err)
	}
	if contract.Level != agenttools.LevelDraft || !contract.Review.Required || !contract.SupportsIdempotency {
		t.Fatalf("contract draft policy is incomplete: %+v", contract)
	}

	payment := NewPaymentScheduleDraftDefinition(nil).Descriptor
	if err := payment.Validate(); err != nil {
		t.Fatalf("payment draft descriptor invalid: %v", err)
	}
	if payment.Level != agenttools.LevelDraft || !payment.Review.Required || !payment.SupportsIdempotency {
		t.Fatalf("payment draft policy is incomplete: %+v", payment)
	}

	event := NewEventDraftDefinition(nil).Descriptor
	if err := event.Validate(); err != nil {
		t.Fatalf("event draft descriptor invalid: %v", err)
	}
	if event.Level != agenttools.LevelDraft || !event.Review.Required || !event.SupportsIdempotency {
		t.Fatalf("event draft policy is incomplete: %+v", event)
	}
}

func TestDecodePaymentScheduleDraftRequiresEvidenceReference(t *testing.T) {
	schedule := repository.PaymentSchedule{ContractID: "contract-1"}
	raw, err := json.Marshal(paymentScheduleDraftToolArguments{Schedule: schedule, EvidenceRef: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePaymentScheduleDraftArguments(raw); err == nil || !strings.Contains(err.Error(), "evidence_ref") {
		t.Fatalf("expected evidence reference validation, got %v", err)
	}
}

func TestDecodeEventDraftRequiresEvidenceAndNormalizesDate(t *testing.T) {
	raw, err := json.Marshal(eventDraftToolArguments{Event: eventDraftInput{
		ContractID: "contract-1", EventType: "modification", EffectiveDate: "2026-02-01", ChangeReason: "rent changed",
	}, EvidenceRef: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeEventDraftArguments(raw); err == nil || !strings.Contains(err.Error(), "evidence_ref") {
		t.Fatalf("expected evidence reference validation, got %v", err)
	}

	raw, err = json.Marshal(eventDraftToolArguments{Event: eventDraftInput{
		ContractID: "contract-1", EventType: "modification", EffectiveDate: "2026-02-01", ChangeReason: "rent changed",
	}, EvidenceRef: map[string]any{"artifact_id": "artifact-1"}})
	if err != nil {
		t.Fatal(err)
	}
	args, err := decodeEventDraftArguments(raw)
	if err != nil || eventFromDraftInput(args.Event).EffectiveDate.Format("2006-01-02") != "2026-02-01" {
		t.Fatalf("expected valid event draft, args=%+v err=%v", args, err)
	}
}

func TestContractDraftHandlerUsesAuthenticatedActorAndOuterIdempotencyKey(t *testing.T) {
	store := &toolFakeStore{ids: map[string]draftapp.ItemResult{}}
	service := draftapp.NewService(toolFakeUOW{store: store}, nil)
	definition := NewContractDraftDefinition(service)
	contract := toolValidContract()
	raw, err := json.Marshal(contractDraftToolArguments{Contract: *contract, RequireEvidence: false})
	if err != nil {
		t.Fatal(err)
	}
	ctx := agenttools.WithExecutionContext(
		access.WithScope(context.Background(), access.Scope{LegalEntityID: "le-1"}),
		agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"contracts:create"}, Scope: access.Scope{LegalEntityID: "le-1"}}},
	)
	result, err := definition.Handler(ctx, agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name,
		ToolVersion: definition.Descriptor.Version, IdempotencyKey: "outer-key", Arguments: raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed tool result, got %+v", result)
	}
	if len(store.contracts) != 1 || store.contracts[0].CreatedBy == nil || *store.contracts[0].CreatedBy != "user-1" {
		t.Fatalf("authenticated actor was not used: %+v", store.contracts)
	}
	item, ok := result.Data.(draftapp.ItemResult)
	if !ok || item.IdempotencyKey != "outer-key" {
		t.Fatalf("outer idempotency key was not passed to service: %#v", result.Data)
	}
}

func toolValidContract() *repository.Contract {
	legalEntityID := "le-1"
	storeID := "store-1"
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return &repository.Contract{
		ContractNumber: "LC-TOOL-001", ContractName: "Tool contract",
		LegalEntityID: &legalEntityID, StoreID: &storeID,
		AssetType: "real_estate", Currency: "CNY",
		CommencementDate: start, LeaseStartDate: start, LeaseEndDate: start.AddDate(1, 0, 0),
		LeaseScope: "in_scope", SourceReferenceLocator: map[string]any{"page": 1},
	}
}

type toolFakeUOW struct{ store *toolFakeStore }

func (u toolFakeUOW) Execute(_ context.Context, fn func(draftapp.DraftStore) error) error {
	return fn(u.store)
}

type toolFakeStore struct {
	contracts []*repository.Contract
	ids       map[string]draftapp.ItemResult
}

func (s *toolFakeStore) LookupIdempotency(_ context.Context, operation, key string) (*draftapp.ItemResult, bool, error) {
	result, ok := s.ids[operation+"\x00"+key]
	if !ok {
		return nil, false, nil
	}
	copy := result
	return &copy, true, nil
}

func (s *toolFakeStore) CreateContractDraft(_ context.Context, contract *repository.Contract) (*repository.Contract, error) {
	copy := *contract
	copy.ID = "contract-tool-1"
	s.contracts = append(s.contracts, &copy)
	return &copy, nil
}

func (s *toolFakeStore) CreatePaymentScheduleDraft(_ context.Context, schedule *repository.PaymentSchedule) (*repository.PaymentSchedule, error) {
	copy := *schedule
	copy.ID = "payment-tool-1"
	return &copy, nil
}

func (s *toolFakeStore) SaveIdempotency(_ context.Context, operation, key string, result draftapp.ItemResult) error {
	s.ids[operation+"\x00"+key] = result
	return nil
}
