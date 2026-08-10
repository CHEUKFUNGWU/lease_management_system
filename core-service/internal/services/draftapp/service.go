// Package draftapp owns the write boundary for AI- and user-assisted lease
// drafts. It deliberately accepts domain repositories through narrow ports so
// HTTP handlers, Agent Tools and future CLI adapters cannot bypass draft
// policy, scope checks or idempotency.
package draftapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

const (
	OperationContractDraft        = "lease.contract.draft.create"
	OperationPaymentScheduleDraft = "lease.payment_schedule.draft.create"
	OperationEventDraft           = "lease.event.draft.create"

	ItemCreated  ItemStatus = "created"
	ItemReplayed ItemStatus = "replayed"
	ItemFailed   ItemStatus = "failed"
)

var (
	ErrDependenciesRequired = errors.New("draft application service dependencies are required")
	ErrTransactionalUOW     = errors.New("draft application service does not support caller-owned transactions")
	ErrScopeRequired        = errors.New("draft creation requires an authenticated data scope")
	ErrActorRequired        = errors.New("draft creation requires an actor")
	ErrIdempotencyRequired  = errors.New("draft creation requires an idempotency key")
	ErrContractNotFound     = errors.New("target contract not found")
	ErrDraftBatchNotFound   = errors.New("draft batch not found")
)

// ItemStatus is intentionally small. Consumers should not infer approval from
// a successful write: every successful result from this service is a draft.
type ItemStatus string

type ItemResult struct {
	Index          int        `json:"index"`
	Operation      string     `json:"operation"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         ItemStatus `json:"status"`
	ID             string     `json:"id,omitempty"`
	Error          string     `json:"error,omitempty"`
}

type BatchResult struct {
	BatchID       string       `json:"batch_id"`
	Operation     string       `json:"operation"`
	Status        string       `json:"status"`
	Items         []ItemResult `json:"items"`
	CreatedCount  int          `json:"created_count"`
	ReplayedCount int          `json:"replayed_count"`
	FailedCount   int          `json:"failed_count"`
}

type DraftBatch struct {
	BatchID   string       `json:"batch_id"`
	Operation string       `json:"operation"`
	Status    string       `json:"status"`
	Items     []ItemResult `json:"items"`
	CreatedBy string       `json:"created_by,omitempty"`
	RunID     string       `json:"run_id,omitempty"`
}

// BatchStore is optional at the service seam so existing in-memory/unit
// adapters remain lightweight. The PostgreSQL adapter persists the batch
// envelope and item results in the same server-owned application boundary.
type BatchStore interface {
	SaveDraftBatch(context.Context, DraftBatch) error
}

// BatchReader is the read-side seam for resumable AI draft batches. It is
// optional so in-memory/unit stores do not need a database query; production
// adapters enforce ownership with actorID.
type BatchReader interface {
	GetDraftBatch(context.Context, string, string) (*DraftBatch, error)
}

// ContractDraftCommand contains already-normalized domain data. Master-data
// resolution remains outside this module because it is an explicit lookup
// policy, but the final write must still pass through this service.
type ContractDraftCommand struct {
	IdempotencyKey  string
	ActorID         string
	RunID           string
	Contract        *repository.Contract
	AccessAttrs     *access.ContractAttributes
	RequireEvidence bool
}

type PaymentScheduleDraftCommand struct {
	IdempotencyKey  string
	ActorID         string
	RunID           string
	Schedule        *repository.PaymentSchedule
	EvidenceRef     any
	RequireEvidence bool
}

type EventDraftCommand struct {
	IdempotencyKey  string
	ActorID         string
	RunID           string
	Event           *repository.LeaseEvent
	EvidenceRef     any
	RequireEvidence bool
}

// ContractReader is optional. When present, it gives the service a trusted
// contract currency for payment-schedule validation. Implementations must
// apply the request scope while reading.
type ContractReader interface {
	GetContract(ctx context.Context, contractID string) (*repository.Contract, error)
}

// DraftStore is the unit-of-work-local persistence port. A production adapter
// must implement Execute transactionally: the idempotency lookup, business
// write, and idempotency save are one atomic operation.
type DraftStore interface {
	LookupIdempotency(ctx context.Context, operation, key string) (*ItemResult, bool, error)
	CreateContractDraft(ctx context.Context, contract *repository.Contract) (*repository.Contract, error)
	CreatePaymentScheduleDraft(ctx context.Context, schedule *repository.PaymentSchedule) (*repository.PaymentSchedule, error)
	SaveIdempotency(ctx context.Context, operation, key string, result ItemResult) error
}

// EventDraftStore is an optional extension so existing lightweight contract
// and payment-schedule test adapters remain source-compatible. Production
// PostgreSQL stores implement it when event draft support is enabled.
type EventDraftStore interface {
	CreateEventDraft(ctx context.Context, event *repository.LeaseEvent) (*repository.LeaseEvent, error)
}

type UnitOfWork interface {
	Execute(ctx context.Context, fn func(DraftStore) error) error
}

// TransactionalUnitOfWork lets a higher-level review coordinator include the
// Draft write and its Artifact/Run audit records in one database transaction.
// It is intentionally an optional extension so in-memory adapters keep the
// small UnitOfWork contract.
type TransactionalUnitOfWork interface {
	ExecuteTransactional(context.Context, func(DraftStore, repository.DBTX) error) error
}

type Service struct {
	uow            UnitOfWork
	contractReader ContractReader
}

func NewService(uow UnitOfWork, contractReader ContractReader) *Service {
	return &Service{uow: uow, contractReader: contractReader}
}

// ExecuteTransactional runs a callback against the production Draft store
// and the exact DB transaction backing it. Callers must not retain either
// value after the callback returns.
func (s *Service) ExecuteTransactional(ctx context.Context, fn func(DraftStore, repository.DBTX) error) error {
	if s == nil || s.uow == nil || fn == nil {
		return ErrDependenciesRequired
	}
	uow, ok := s.uow.(TransactionalUnitOfWork)
	if !ok {
		return ErrTransactionalUOW
	}
	return uow.ExecuteTransactional(ctx, fn)
}

func (s *Service) CreateContractDraft(ctx context.Context, command ContractDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationContractDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := validateContractDraft(ctx, command); err != nil {
		return failed(result, err)
	}

	contract := prepareContractDraft(command.Contract, command.ActorID)
	return s.executeItem(ctx, result, func(store DraftStore) (string, error) {
		created, err := store.CreateContractDraft(ctx, contract)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("contract draft writer returned no id")
		}
		return created.ID, nil
	})
}

func (s *Service) CreatePaymentScheduleDraft(ctx context.Context, command PaymentScheduleDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationPaymentScheduleDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := s.validatePaymentScheduleDraft(ctx, command); err != nil {
		return failed(result, err)
	}

	schedule := preparePaymentScheduleDraft(command.Schedule)
	return s.executeItem(ctx, result, func(store DraftStore) (string, error) {
		created, err := store.CreatePaymentScheduleDraft(ctx, schedule)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("payment schedule draft writer returned no id")
		}
		return created.ID, nil
	})
}

func (s *Service) CreateEventDraft(ctx context.Context, command EventDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationEventDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := s.validateEventDraft(ctx, command); err != nil {
		return failed(result, err)
	}

	event := prepareEventDraft(command.Event, command.ActorID, command.EvidenceRef)
	return s.executeItem(ctx, result, func(store DraftStore) (string, error) {
		eventStore, ok := store.(EventDraftStore)
		if !ok {
			return "", ErrDependenciesRequired
		}
		created, err := eventStore.CreateEventDraft(ctx, event)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("event draft writer returned no id")
		}
		return created.ID, nil
	})
}

// CreateContractDraftInStore applies one already-normalized draft command to
// a caller-owned transaction. It is the transaction-aware counterpart of
// CreateContractDraft and is used only by the review coordinator.
func (s *Service) CreateContractDraftInStore(ctx context.Context, store DraftStore, command ContractDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationContractDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := validateContractDraft(ctx, command); err != nil {
		return failed(result, err)
	}
	contract := prepareContractDraft(command.Contract, command.ActorID)
	return s.executeItemInStore(ctx, store, result, func(store DraftStore) (string, error) {
		created, err := store.CreateContractDraft(ctx, contract)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("contract draft writer returned no id")
		}
		return created.ID, nil
	})
}

// CreatePaymentScheduleDraftInStore applies one payment-schedule draft to a
// caller-owned transaction while preserving all normal validation rules.
func (s *Service) CreatePaymentScheduleDraftInStore(ctx context.Context, store DraftStore, command PaymentScheduleDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationPaymentScheduleDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := s.validatePaymentScheduleDraft(ctx, command); err != nil {
		return failed(result, err)
	}
	schedule := preparePaymentScheduleDraft(command.Schedule)
	return s.executeItemInStore(ctx, store, result, func(store DraftStore) (string, error) {
		created, err := store.CreatePaymentScheduleDraft(ctx, schedule)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("payment schedule draft writer returned no id")
		}
		return created.ID, nil
	})
}

// CreateEventDraftInStore applies one event draft to a caller-owned
// transaction while preserving the event-specific evidence and scope rules.
func (s *Service) CreateEventDraftInStore(ctx context.Context, store DraftStore, command EventDraftCommand) ItemResult {
	result := ItemResult{Operation: OperationEventDraft, IdempotencyKey: strings.TrimSpace(command.IdempotencyKey)}
	if err := validateCommon(ctx, result.IdempotencyKey, command.ActorID); err != nil {
		return failed(result, err)
	}
	if err := s.validateEventDraft(ctx, command); err != nil {
		return failed(result, err)
	}
	event := prepareEventDraft(command.Event, command.ActorID, command.EvidenceRef)
	return s.executeItemInStore(ctx, store, result, func(store DraftStore) (string, error) {
		eventStore, ok := store.(EventDraftStore)
		if !ok {
			return "", ErrDependenciesRequired
		}
		created, err := eventStore.CreateEventDraft(ctx, event)
		if err != nil {
			return "", err
		}
		if created == nil || strings.TrimSpace(created.ID) == "" {
			return "", errors.New("event draft writer returned no id")
		}
		return created.ID, nil
	})
}

func (s *Service) CreateContractDraftBatch(ctx context.Context, commands []ContractDraftCommand) BatchResult {
	return s.createContractDraftBatch(ctx, uuid.NewString(), commands)
}

func (s *Service) ResumeContractDraftBatch(ctx context.Context, batchID string, commands []ContractDraftCommand) BatchResult {
	return s.createContractDraftBatch(ctx, strings.TrimSpace(batchID), commands)
}

func (s *Service) createContractDraftBatch(ctx context.Context, batchID string, commands []ContractDraftCommand) BatchResult {
	if batchID == "" {
		batchID = uuid.NewString()
	}
	result := BatchResult{BatchID: batchID, Operation: OperationContractDraft, Status: "in_progress", Items: make([]ItemResult, 0, len(commands))}
	s.persistBatch(ctx, result, commandsActor(commands), commandsRunID(commands))
	for index, command := range commands {
		item := s.CreateContractDraft(ctx, command)
		item.Index = index
		result.add(item)
		result.Status = batchStatus(result)
		s.persistBatch(ctx, result, command.ActorID, command.RunID)
	}
	result.Status = batchStatus(result)
	s.persistBatch(ctx, result, commandsActor(commands), commandsRunID(commands))
	return result
}

func (s *Service) CreatePaymentScheduleDraftBatch(ctx context.Context, commands []PaymentScheduleDraftCommand) BatchResult {
	return s.createPaymentScheduleDraftBatch(ctx, uuid.NewString(), commands)
}

func (s *Service) ResumePaymentScheduleDraftBatch(ctx context.Context, batchID string, commands []PaymentScheduleDraftCommand) BatchResult {
	return s.createPaymentScheduleDraftBatch(ctx, strings.TrimSpace(batchID), commands)
}

func (s *Service) CreateEventDraftBatch(ctx context.Context, commands []EventDraftCommand) BatchResult {
	return s.createEventDraftBatch(ctx, uuid.NewString(), commands)
}

func (s *Service) ResumeEventDraftBatch(ctx context.Context, batchID string, commands []EventDraftCommand) BatchResult {
	return s.createEventDraftBatch(ctx, strings.TrimSpace(batchID), commands)
}

// GetDraftBatch returns a persisted batch envelope for progress inspection or
// retry UI. It never returns business rows outside the batch's owning actor.
func (s *Service) GetDraftBatch(ctx context.Context, batchID, actorID string) (*DraftBatch, error) {
	if s == nil || s.uow == nil {
		return nil, ErrDependenciesRequired
	}
	if strings.TrimSpace(batchID) == "" {
		return nil, ErrDraftBatchNotFound
	}
	if strings.TrimSpace(actorID) == "" {
		return nil, ErrActorRequired
	}
	if _, scoped := access.ScopeFromContext(ctx); !scoped {
		return nil, ErrScopeRequired
	}
	var batch *DraftBatch
	err := s.uow.Execute(ctx, func(store DraftStore) error {
		reader, ok := store.(BatchReader)
		if !ok {
			return ErrDraftBatchNotFound
		}
		loaded, err := reader.GetDraftBatch(ctx, strings.TrimSpace(batchID), strings.TrimSpace(actorID))
		if err != nil {
			return err
		}
		batch = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	if batch == nil {
		return nil, ErrDraftBatchNotFound
	}
	return batch, nil
}

func (s *Service) createPaymentScheduleDraftBatch(ctx context.Context, batchID string, commands []PaymentScheduleDraftCommand) BatchResult {
	if batchID == "" {
		batchID = uuid.NewString()
	}
	result := BatchResult{BatchID: batchID, Operation: OperationPaymentScheduleDraft, Status: "in_progress", Items: make([]ItemResult, 0, len(commands))}
	s.persistBatch(ctx, result, paymentCommandsActor(commands), paymentCommandsRunID(commands))
	for index, command := range commands {
		item := s.CreatePaymentScheduleDraft(ctx, command)
		item.Index = index
		result.add(item)
		result.Status = batchStatus(result)
		s.persistBatch(ctx, result, command.ActorID, command.RunID)
	}
	result.Status = batchStatus(result)
	s.persistBatch(ctx, result, paymentCommandsActor(commands), paymentCommandsRunID(commands))
	return result
}

func (s *Service) createEventDraftBatch(ctx context.Context, batchID string, commands []EventDraftCommand) BatchResult {
	if batchID == "" {
		batchID = uuid.NewString()
	}
	result := BatchResult{BatchID: batchID, Operation: OperationEventDraft, Status: "in_progress", Items: make([]ItemResult, 0, len(commands))}
	s.persistBatch(ctx, result, eventCommandsActor(commands), eventCommandsRunID(commands))
	for index, command := range commands {
		item := s.CreateEventDraft(ctx, command)
		item.Index = index
		result.add(item)
		result.Status = batchStatus(result)
		s.persistBatch(ctx, result, command.ActorID, command.RunID)
	}
	result.Status = batchStatus(result)
	s.persistBatch(ctx, result, eventCommandsActor(commands), eventCommandsRunID(commands))
	return result
}

func (s *Service) persistBatch(ctx context.Context, result BatchResult, actorID, runID string) {
	if s == nil || s.uow == nil {
		return
	}
	_ = s.uow.Execute(ctx, func(store DraftStore) error {
		batchStore, ok := store.(BatchStore)
		if !ok {
			return nil
		}
		return batchStore.SaveDraftBatch(ctx, DraftBatch{
			BatchID: result.BatchID, Operation: result.Operation, Status: result.Status,
			Items: append([]ItemResult(nil), result.Items...), CreatedBy: actorID, RunID: runID,
		})
	})
}

func batchStatus(result BatchResult) string {
	if len(result.Items) == 0 {
		return "in_progress"
	}
	if result.FailedCount == 0 {
		return "completed"
	}
	if result.CreatedCount+result.ReplayedCount > 0 {
		return "partial_failed"
	}
	return "failed"
}

func commandsActor(commands []ContractDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].ActorID
	}
	return ""
}

func commandsRunID(commands []ContractDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].RunID
	}
	return ""
}

func paymentCommandsActor(commands []PaymentScheduleDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].ActorID
	}
	return ""
}

func paymentCommandsRunID(commands []PaymentScheduleDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].RunID
	}
	return ""
}

func eventCommandsActor(commands []EventDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].ActorID
	}
	return ""
}

func eventCommandsRunID(commands []EventDraftCommand) string {
	if len(commands) > 0 {
		return commands[0].RunID
	}
	return ""
}

func (r *BatchResult) add(item ItemResult) {
	r.Items = append(r.Items, item)
	switch item.Status {
	case ItemCreated:
		r.CreatedCount++
	case ItemReplayed:
		r.ReplayedCount++
	case ItemFailed:
		r.FailedCount++
	}
}

// AddItem lets an orchestration boundary accumulate results while it owns a
// wider transaction. Normal batch methods use the private helper above; this
// seam keeps result accounting identical for atomic Artifact review commits.
func (r *BatchResult) AddItem(item ItemResult) {
	if r == nil {
		return
	}
	r.add(item)
}

func (s *Service) executeItem(ctx context.Context, result ItemResult, write func(DraftStore) (string, error)) ItemResult {
	if s == nil || s.uow == nil {
		return failed(result, ErrDependenciesRequired)
	}

	err := s.uow.Execute(ctx, func(store DraftStore) error {
		var itemErr error
		result = s.executeItemInStore(ctx, store, result, write)
		if result.Status == ItemFailed {
			itemErr = errors.New(result.Error)
		}
		return itemErr
	})
	if err != nil {
		return failed(result, err)
	}
	return result
}

func (s *Service) executeItemInStore(ctx context.Context, store DraftStore, result ItemResult, write func(DraftStore) (string, error)) ItemResult {
	if store == nil {
		return failed(result, ErrDependenciesRequired)
	}
	cached, found, err := store.LookupIdempotency(ctx, result.Operation, result.IdempotencyKey)
	if err != nil {
		return failed(result, fmt.Errorf("lookup draft idempotency: %w", err))
	}
	if found {
		if cached == nil {
			return failed(result, errors.New("idempotency store returned an empty result"))
		}
		result = *cached
		result.Operation = resultOrDefault(result.Operation, cached.Operation)
		result.IdempotencyKey = resultOrDefault(result.IdempotencyKey, cached.IdempotencyKey)
		result.Status = ItemReplayed
		return result
	}
	id, err := write(store)
	if err != nil {
		return failed(result, err)
	}
	result.ID = id
	result.Status = ItemCreated
	result.Error = ""
	if err := store.SaveIdempotency(ctx, result.Operation, result.IdempotencyKey, result); err != nil {
		return failed(result, fmt.Errorf("save draft idempotency: %w", err))
	}
	return result
}

func resultOrDefault(current, fallback string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return fallback
}

func validateCommon(ctx context.Context, idempotencyKey, actorID string) error {
	if _, scoped := access.ScopeFromContext(ctx); !scoped {
		return ErrScopeRequired
	}
	if strings.TrimSpace(actorID) == "" {
		return ErrActorRequired
	}
	if idempotencyKey == "" {
		return ErrIdempotencyRequired
	}
	return nil
}

func validateContractDraft(ctx context.Context, command ContractDraftCommand) error {
	contract := command.Contract
	if contract == nil {
		return errors.New("contract draft is required")
	}
	if strings.TrimSpace(contract.ContractNumber) == "" || strings.TrimSpace(contract.ContractName) == "" {
		return errors.New("contract number and name are required")
	}
	if strings.TrimSpace(contract.Currency) == "" {
		return errors.New("contract currency is required")
	}
	if strings.TrimSpace(contract.AssetType) == "" {
		return errors.New("contract asset type is required")
	}
	if !validLeaseScope(contract.LeaseScope) {
		return errors.New("contract lease scope is invalid")
	}
	if contract.CommencementDate.IsZero() || contract.LeaseStartDate.IsZero() || contract.LeaseEndDate.IsZero() {
		return errors.New("contract commencement and lease dates are required")
	}
	if contract.LeaseEndDate.Before(contract.LeaseStartDate) {
		return errors.New("contract lease end date must not precede lease start date")
	}
	if contract.CommencementDate.After(contract.LeaseEndDate) {
		return errors.New("contract commencement date must not follow lease end date")
	}
	if contract.DiscountRateValue != nil && (*contract.DiscountRateValue <= 0 || *contract.DiscountRateValue >= 1) {
		return errors.New("discount rate value must be a decimal between 0 and 1")
	}

	attrs, err := contractAttributes(contract, command.AccessAttrs)
	if err != nil {
		return err
	}
	scope, _ := access.ScopeFromContext(ctx)
	if !scope.AllowsContract(attrs) {
		return errors.New("contract is outside the assigned data scope")
	}
	if command.RequireEvidence && contract.SourceReferenceLocator == nil {
		return errors.New("AI draft requires source evidence")
	}
	return nil
}

func (s *Service) validatePaymentScheduleDraft(ctx context.Context, command PaymentScheduleDraftCommand) error {
	schedule := command.Schedule
	if schedule == nil {
		return errors.New("payment schedule draft is required")
	}
	if strings.TrimSpace(schedule.ContractID) == "" {
		return errors.New("payment schedule contract id is required")
	}
	if schedule.EffectiveStartDate.IsZero() || schedule.EffectiveEndDate.IsZero() ||
		schedule.CoverageStartDate.IsZero() || schedule.CoverageEndDate.IsZero() || schedule.DueDate.IsZero() {
		return errors.New("payment schedule dates are required")
	}
	if schedule.EffectiveEndDate.Before(schedule.EffectiveStartDate) {
		return errors.New("payment schedule effective end date must not precede start date")
	}
	if schedule.CoverageEndDate.Before(schedule.CoverageStartDate) {
		return errors.New("payment schedule coverage end date must not precede start date")
	}
	if schedule.Amount <= 0 {
		return errors.New("payment schedule amount must be greater than zero")
	}
	if strings.TrimSpace(schedule.Currency) == "" {
		return errors.New("payment schedule currency is required")
	}
	if schedule.PaymentTiming != "prepaid" && schedule.PaymentTiming != "postpaid" {
		return errors.New("payment schedule payment timing must be prepaid or postpaid")
	}
	if strings.TrimSpace(schedule.AmountType) == "" {
		return errors.New("payment schedule amount type is required")
	}
	if schedule.IsFixed && schedule.IsVariable {
		return errors.New("payment schedule cannot be both fixed and variable")
	}
	if schedule.IsLeaseComponent && schedule.IsNonLeaseComponent {
		return errors.New("payment schedule cannot be both lease and non-lease component")
	}

	if s.contractReader != nil {
		contract, err := s.contractReader.GetContract(ctx, schedule.ContractID)
		if err != nil {
			return fmt.Errorf("verify target contract: %w", err)
		}
		if contract == nil {
			return ErrContractNotFound
		}
		if !sameCurrency(schedule.Currency, contract.Currency) {
			return errors.New("payment schedule currency must match contract currency")
		}
	}
	if command.RequireEvidence && command.EvidenceRef == nil {
		return errors.New("AI payment schedule draft requires source evidence")
	}
	return nil
}

func (s *Service) validateEventDraft(ctx context.Context, command EventDraftCommand) error {
	event := command.Event
	if event == nil {
		return errors.New("event draft is required")
	}
	if strings.TrimSpace(event.ContractID) == "" {
		return errors.New("event contract id is required")
	}
	if strings.TrimSpace(event.EventType) == "" {
		return errors.New("event type is required")
	}
	if event.EffectiveDate.IsZero() {
		return errors.New("event effective date is required")
	}
	if event.ChangeReason == nil || strings.TrimSpace(*event.ChangeReason) == "" {
		return errors.New("event change reason is required")
	}
	if s.contractReader != nil {
		contract, err := s.contractReader.GetContract(ctx, event.ContractID)
		if err != nil {
			return fmt.Errorf("verify target contract: %w", err)
		}
		if contract == nil {
			return ErrContractNotFound
		}
	}
	if command.RequireEvidence && command.EvidenceRef == nil {
		return errors.New("AI event draft requires source evidence")
	}
	return nil
}

func prepareContractDraft(contract *repository.Contract, actorID string) *repository.Contract {
	copy := *contract
	copy.Status = "draft"
	copy.ApprovalStatus = "draft"
	copy.IsOfficialVersion = false
	copy.IncludedInReporting = false
	copy.ReportMode = "working"
	copy.ReviewedBy = nil
	copy.ReviewedAt = nil
	copy.ApprovedBy = nil
	copy.ApprovedAt = nil
	copy.SubmittedAt = nil
	if copy.DraftVersionNo <= 0 {
		copy.DraftVersionNo = 1
	}
	if copy.CreatedBy == nil {
		copy.CreatedBy = &actorID
	}
	return &copy
}

func preparePaymentScheduleDraft(schedule *repository.PaymentSchedule) *repository.PaymentSchedule {
	copy := *schedule
	copy.Currency = strings.ToUpper(strings.TrimSpace(copy.Currency))
	copy.ApprovalStatus = "draft"
	copy.IsOfficialVersion = false
	copy.ReviewedBy = nil
	copy.ApprovedBy = nil
	if copy.IsVariable || copy.IsNonLeaseComponent || !copy.IsLeaseComponent {
		copy.IncludedInLiabilityPV = false
	}
	return &copy
}

func prepareEventDraft(event *repository.LeaseEvent, actorID string, evidenceRef any) *repository.LeaseEvent {
	copy := *event
	copy.Status = "pending"
	copy.ApprovalStatus = "draft"
	copy.IsOfficialVersion = false
	copy.ReviewedBy = nil
	copy.ReviewedAt = nil
	copy.ApprovedBy = nil
	copy.ApprovalDate = nil
	copy.RejectedReason = nil
	copy.CreatedBy = &actorID
	copy.SourceReferenceLocator = evidenceRef
	return &copy
}

func contractAttributes(contract *repository.Contract, explicit *access.ContractAttributes) (access.ContractAttributes, error) {
	attrs := access.ContractAttributes{}
	if contract.LegalEntityID != nil {
		attrs.LegalEntityID = strings.TrimSpace(*contract.LegalEntityID)
	}
	if contract.StoreID != nil {
		attrs.StoreID = strings.TrimSpace(*contract.StoreID)
	}
	if explicit != nil {
		explicitAttrs := *explicit
		explicitAttrs.LegalEntityID = strings.TrimSpace(explicitAttrs.LegalEntityID)
		explicitAttrs.StoreID = strings.TrimSpace(explicitAttrs.StoreID)
		explicitAttrs.Region = strings.TrimSpace(explicitAttrs.Region)
		explicitAttrs.Brand = strings.TrimSpace(explicitAttrs.Brand)
		if attrs.LegalEntityID != "" && explicitAttrs.LegalEntityID != "" && attrs.LegalEntityID != explicitAttrs.LegalEntityID {
			return access.ContractAttributes{}, errors.New("contract legal entity does not match access attributes")
		}
		if attrs.StoreID != "" && explicitAttrs.StoreID != "" && attrs.StoreID != explicitAttrs.StoreID {
			return access.ContractAttributes{}, errors.New("contract store does not match access attributes")
		}
		attrs = explicitAttrs
	}
	return attrs, nil
}

func validLeaseScope(value string) bool {
	switch value {
	case "in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease":
		return true
	default:
		return false
	}
}

func sameCurrency(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func failed(result ItemResult, err error) ItemResult {
	result.Status = ItemFailed
	if err != nil {
		result.Error = err.Error()
	}
	return result
}
