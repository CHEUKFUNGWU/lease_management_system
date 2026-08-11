package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/aiintake"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// applyReviewDraftInStore is the transaction-aware path used by production
// Review commits. It intentionally rolls back the wider transaction on a
// failed item; the existing applyReviewDraft path remains available for
// batch-resume adapters that need to retain successful items independently.
func (h *AIChatHandler) applyReviewDraftInStore(
	ctx context.Context,
	artifact *repository.AIChatArtifact,
	actionType string,
	payload map[string]interface{},
	actorID string,
	store draftapp.DraftStore,
) (*draftapp.BatchResult, error) {
	if artifact == nil || h == nil || h.draftService == nil {
		return nil, errors.New("draft application service is unavailable")
	}
	selection, err := parseReviewSelection(payload)
	if err != nil {
		return nil, err
	}
	validateBatch := func(operation string) error {
		if strings.TrimSpace(selection.BatchID) == "" {
			return nil
		}
		reader, ok := store.(draftapp.BatchReader)
		if !ok {
			return draftapp.ErrDraftBatchNotFound
		}
		batch, readErr := reader.GetDraftBatch(ctx, selection.BatchID, actorID)
		if readErr != nil {
			return readErr
		}
		if batch == nil || batch.Operation != operation {
			return errors.New("retry batch operation does not match the artifact")
		}
		return nil
	}
	finalize := func(result *draftapp.BatchResult, actor, runID string) (*draftapp.BatchResult, error) {
		if result == nil {
			return nil, errors.New("draft batch result is unavailable")
		}
		if result.FailedCount == 0 {
			result.Status = "completed"
		} else if result.CreatedCount+result.ReplayedCount > 0 {
			result.Status = "partial_failed"
		} else {
			result.Status = "failed"
		}
		if batchStore, ok := store.(draftapp.BatchStore); ok {
			if err := batchStore.SaveDraftBatch(ctx, draftapp.DraftBatch{
				BatchID: result.BatchID, Operation: result.Operation, Status: result.Status,
				Items: append([]draftapp.ItemResult(nil), result.Items...), CreatedBy: actor, RunID: runID,
			}); err != nil {
				return nil, err
			}
		}
		if result.FailedCount > 0 {
			return result, errors.New("one or more draft items failed")
		}
		return result, nil
	}

	switch artifact.ArtifactType {
	case "contract_draft":
		if err := validateBatch(draftapp.OperationContractDraft); err != nil {
			return nil, err
		}
		var data artifactContractData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode contract draft artifact: %w", err)
		}
		commands, err := h.contractDraftCommands(ctx, artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		batchID := selection.BatchID
		if batchID == "" {
			batchID = uuid.NewString()
		}
		result := &draftapp.BatchResult{BatchID: batchID, Operation: draftapp.OperationContractDraft, Status: "in_progress"}
		for index, command := range commands {
			item := h.draftService.CreateContractDraftInStore(ctx, store, command)
			item.Index = index
			result.AddItem(item)
		}
		return finalize(result, actorID, artifact.RunID)
	case "payment_schedule_draft":
		if err := validateBatch(draftapp.OperationPaymentScheduleDraft); err != nil {
			return nil, err
		}
		var data artifactPaymentScheduleData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode payment schedule draft artifact: %w", err)
		}
		commands, err := paymentScheduleDraftCommands(artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		batchID := selection.BatchID
		if batchID == "" {
			batchID = uuid.NewString()
		}
		result := &draftapp.BatchResult{BatchID: batchID, Operation: draftapp.OperationPaymentScheduleDraft, Status: "in_progress"}
		for index, command := range commands {
			item := h.draftService.CreatePaymentScheduleDraftInStore(ctx, store, command)
			item.Index = index
			result.AddItem(item)
		}
		return finalize(result, actorID, artifact.RunID)
	case "event_draft":
		if err := validateBatch(draftapp.OperationEventDraft); err != nil {
			return nil, err
		}
		var data artifactEventData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode event draft artifact: %w", err)
		}
		commands, err := eventDraftCommands(artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		batchID := selection.BatchID
		if batchID == "" {
			batchID = uuid.NewString()
		}
		result := &draftapp.BatchResult{BatchID: batchID, Operation: draftapp.OperationEventDraft, Status: "in_progress"}
		for index, command := range commands {
			item := h.draftService.CreateEventDraftInStore(ctx, store, command)
			item.Index = index
			result.AddItem(item)
		}
		return finalize(result, actorID, artifact.RunID)
	default:
		return nil, nil
	}
}

const (
	contractDraftActionKey        = "contract_draft"
	paymentScheduleDraftActionKey = "payment_schedule_draft"
)

func isDraftReviewAction(actionType string) bool {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "confirm", "import", "create_draft":
		return true
	default:
		return false
	}
}

// commitReviewTransaction is installed on the production AI Chat Runtime.
// For draft actions it owns one transaction spanning business Draft rows,
// idempotency, batch envelope, Review Action, Artifact status and Run Event.
// Non-draft review actions continue to use the runtime repository's own atomic
// action/status transaction.
func (h *AIChatHandler) commitReviewTransaction(
	ctx context.Context,
	artifact *repository.AIChatArtifact,
	action *repository.AIChatReviewAction,
	status string,
	command aichat.ReviewCommand,
) (any, error) {
	runtimeRepo, ok := h.runtimeRepo.(*repository.AIChatRuntimeRepository)
	if !ok || runtimeRepo == nil {
		return nil, errors.New("AI chat runtime does not support transactional review commits")
	}
	if !isDraftReviewAction(command.ActionType) {
		return nil, runtimeRepo.CommitReviewAction(ctx, action, status)
	}
	if h.draftService == nil {
		return nil, errors.New("draft application service is unavailable")
	}
	var draftResult *draftapp.BatchResult
	err := h.draftService.ExecuteTransactional(ctx, func(store draftapp.DraftStore, db repository.DBTX) error {
		result, err := h.applyReviewDraftInStore(ctx, artifact, command.ActionType, command.ActionPayload, command.UserID, store)
		if err != nil {
			return err
		}
		draftResult = result
		txRuntime := runtimeRepo.WithTx(db)
		if err := txRuntime.RecordReviewAction(ctx, action); err != nil {
			return err
		}
		if err := txRuntime.UpdateArtifactStatus(ctx, artifact.ID, status); err != nil {
			return err
		}
		if draftResult != nil {
			if err := txRuntime.CreateRunAuditLinks(
				ctx, artifact.RunID, artifact.ID, "review_draft_created", auditLinkInputs(draftResult),
			); err != nil {
				return err
			}
			return appendDraftBatchEventToWriter(ctx, txRuntime, artifact, command.ActionType, draftResult)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return draftResult, nil
}

func auditLinkInputs(result *draftapp.BatchResult) []repository.AgentRunAuditLinkInput {
	if result == nil {
		return nil
	}
	tableName := ""
	switch result.Operation {
	case draftapp.OperationContractDraft:
		tableName = "lease_contracts"
	case draftapp.OperationPaymentScheduleDraft:
		tableName = "payment_schedules"
	case draftapp.OperationEventDraft:
		tableName = "lease_events"
	}
	if tableName == "" {
		return nil
	}
	links := make([]repository.AgentRunAuditLinkInput, 0, len(result.Items))
	for _, item := range result.Items {
		if strings.TrimSpace(item.ID) == "" || item.Status == draftapp.ItemFailed {
			continue
		}
		links = append(links, repository.AgentRunAuditLinkInput{
			BusinessTable: tableName, BusinessRecordID: item.ID,
			Relation: "review_draft_created", ItemStatus: string(item.Status),
		})
	}
	return links
}

type artifactContractData struct {
	Contracts []aiintake.ContractDraftData `json:"contracts"`
	Summary   artifactBatchSummary         `json:"summary"`
}

type artifactPaymentScheduleData struct {
	Schedules []artifactPaymentSchedule `json:"schedules"`
	Summary   artifactBatchSummary      `json:"summary"`
}

type artifactEventData struct {
	Event struct {
		ContractID         string          `json:"contract_id"`
		EventType          string          `json:"event_type"`
		EffectiveDate      string          `json:"effective_date"`
		OriginalValue      *string         `json:"original_value,omitempty"`
		NewValue           *string         `json:"new_value,omitempty"`
		ChangeReason       string          `json:"change_reason"`
		JudgmentBasis      string          `json:"judgment_basis,omitempty"`
		RevisionParameters json.RawMessage `json:"revision_parameters,omitempty"`
	} `json:"event"`
}

type artifactBatchSummary struct {
	IntakeID         string `json:"intake_id"`
	ContractID       string `json:"contract_id"`
	EvidenceComplete bool   `json:"evidence_complete"`
}

type artifactPaymentSchedule struct {
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	DueDate          string  `json:"due_date"`
	Amount           float64 `json:"amount"`
	PaymentTiming    string  `json:"payment_timing"`
	IsFixed          bool    `json:"is_fixed"`
	IsLeaseComponent bool    `json:"is_lease_component"`
	AmountType       string  `json:"amount_type"`
	Currency         string  `json:"currency"`
}

type reviewSelection struct {
	Indexes         []int    `json:"selected_indexes"`
	ContractNumbers []string `json:"contract_numbers"`
	BatchID         string   `json:"batch_id"`
}

func (h *AIChatHandler) applyReviewDraft(
	ctx context.Context,
	artifact *repository.AIChatArtifact,
	actionType string,
	payload map[string]interface{},
	actorID string,
) (*draftapp.BatchResult, error) {
	if artifact == nil {
		return nil, errors.New("artifact is required")
	}
	action := strings.ToLower(strings.TrimSpace(actionType))
	if action != "confirm" && action != "import" && action != "create_draft" {
		return nil, nil
	}
	if h == nil || h.draftService == nil {
		return nil, errors.New("draft application service is unavailable")
	}
	selection, err := parseReviewSelection(payload)
	if err != nil {
		return nil, err
	}

	switch artifact.ArtifactType {
	case "contract_draft":
		if h.contractRepo == nil {
			return nil, errors.New("contract draft application service is unavailable")
		}
		var data artifactContractData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode contract draft artifact: %w", err)
		}
		commands, err := h.contractDraftCommands(ctx, artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		var result draftapp.BatchResult
		if strings.TrimSpace(selection.BatchID) != "" {
			if err := h.validateRetryBatch(ctx, selection.BatchID, draftapp.OperationContractDraft, actorID); err != nil {
				return nil, err
			}
			result = h.draftService.ResumeContractDraftBatch(ctx, selection.BatchID, commands)
		} else {
			result = h.draftService.CreateContractDraftBatch(ctx, commands)
		}
		return &result, nil
	case "payment_schedule_draft":
		var data artifactPaymentScheduleData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode payment schedule draft artifact: %w", err)
		}
		commands, err := paymentScheduleDraftCommands(artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		var result draftapp.BatchResult
		if strings.TrimSpace(selection.BatchID) != "" {
			if err := h.validateRetryBatch(ctx, selection.BatchID, draftapp.OperationPaymentScheduleDraft, actorID); err != nil {
				return nil, err
			}
			result = h.draftService.ResumePaymentScheduleDraftBatch(ctx, selection.BatchID, commands)
		} else {
			result = h.draftService.CreatePaymentScheduleDraftBatch(ctx, commands)
		}
		return &result, nil
	case "event_draft":
		var data artifactEventData
		if err := json.Unmarshal(artifact.Data, &data); err != nil {
			return nil, fmt.Errorf("decode event draft artifact: %w", err)
		}
		commands, err := eventDraftCommands(artifact, data, selection, actorID)
		if err != nil {
			return nil, err
		}
		result := draftapp.BatchResult{BatchID: strings.TrimSpace(selection.BatchID), Operation: draftapp.OperationEventDraft}
		if result.BatchID == "" {
			result = h.draftService.CreateEventDraftBatch(ctx, commands)
		} else {
			if err := h.validateRetryBatch(ctx, result.BatchID, draftapp.OperationEventDraft, actorID); err != nil {
				return nil, err
			}
			result = h.draftService.ResumeEventDraftBatch(ctx, result.BatchID, commands)
		}
		return &result, nil
	default:
		return nil, nil
	}
}

func eventDraftCommands(
	artifact *repository.AIChatArtifact,
	data artifactEventData,
	selection reviewSelection,
	actorID string,
) ([]draftapp.EventDraftCommand, error) {
	if len(selection.Indexes) > 0 && !containsInt(selection.Indexes, 0) {
		return []draftapp.EventDraftCommand{}, nil
	}
	effectiveDate := parseDraftDate(data.Event.EffectiveDate)
	if strings.TrimSpace(data.Event.ContractID) == "" || strings.TrimSpace(data.Event.EventType) == "" || effectiveDate.IsZero() {
		return nil, errors.New("event artifact requires contract_id, event_type and effective_date")
	}
	if strings.TrimSpace(data.Event.ChangeReason) == "" {
		return nil, errors.New("event artifact requires change_reason")
	}
	return []draftapp.EventDraftCommand{{
		IdempotencyKey: fmt.Sprintf("artifact:%s:%s:0", artifact.ID, "event_draft"),
		ActorID:        actorID, RunID: artifact.RunID,
		Event: &repository.LeaseEvent{
			ContractID: data.Event.ContractID, EventType: data.Event.EventType,
			EffectiveDate: effectiveDate, OriginalValue: data.Event.OriginalValue,
			NewValue: data.Event.NewValue, ChangeReason: stringPointer(data.Event.ChangeReason),
			JudgmentBasis: stringPointer(data.Event.JudgmentBasis), RevisionParameters: data.Event.RevisionParameters,
		},
		EvidenceRef:     map[string]interface{}{"artifact_id": artifact.ID, "run_id": artifact.RunID, "item_index": 0},
		RequireEvidence: true,
	}}, nil
}

func containsInt(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (h *AIChatHandler) validateRetryBatch(ctx context.Context, batchID, operation, actorID string) error {
	batch, err := h.draftService.GetDraftBatch(ctx, batchID, actorID)
	if err != nil {
		return fmt.Errorf("load retry batch: %w", err)
	}
	if batch == nil || batch.Operation != operation {
		return errors.New("retry batch operation does not match the artifact")
	}
	return nil
}

func (h *AIChatHandler) contractDraftCommands(
	ctx context.Context,
	artifact *repository.AIChatArtifact,
	data artifactContractData,
	selection reviewSelection,
	actorID string,
) ([]draftapp.ContractDraftCommand, error) {
	indexes := selectedIndexes(len(data.Contracts), selection, func(index int) string {
		return data.Contracts[index].ContractNumber
	})
	commands := make([]draftapp.ContractDraftCommand, 0, len(indexes))
	legalEntityHint := tenantIDFromContext(ctx)
	for _, index := range indexes {
		draft := data.Contracts[index]
		currency := strings.ToUpper(strings.TrimSpace(draft.Currency))
		legalEntityID, err := h.contractRepo.ResolveLegalEntityID(ctx, legalEntityHint, currency)
		if err != nil {
			return nil, fmt.Errorf("resolve contract %d legal entity: %w", index, err)
		}
		storeID, err := h.contractRepo.ResolveOrCreateStoreID(ctx, draft.StoreName, draft.StoreAddress, legalEntityID)
		if err != nil {
			return nil, fmt.Errorf("resolve contract %d store: %w", index, err)
		}
		landlordID, err := h.contractRepo.ResolveOrCreateLandlordID(ctx, draft.Lessor)
		if err != nil {
			return nil, fmt.Errorf("resolve contract %d landlord: %w", index, err)
		}
		commencement := parseDraftDate(draft.CommencementDate)
		leaseStart := parseDraftDate(draft.LeaseStartDate)
		leaseEnd := parseDraftDate(draft.LeaseEndDate)
		leaseScope := firstNonEmpty(draft.LeaseScope, draft.SuggestedScope)
		discountRate := draft.DiscountRate
		if discountRate > 1 {
			discountRate /= 100
		}
		var discountRatePtr *float64
		if discountRate > 0 {
			discountRatePtr = &discountRate
		}
		now := time.Now().UTC()
		contract := &repository.Contract{
			ContractNumber: draft.ContractNumber, ContractName: draft.ContractName,
			LegalEntityID: legalEntityID, StoreID: storeID, LandlordID: landlordID,
			LesseeName: draft.Lessee, LessorName: draft.Lessor,
			StoreName: draft.StoreName, StoreAddress: draft.StoreAddress,
			AssetType: strings.TrimSpace(draft.AssetType), Currency: currency,
			CommencementDate: commencement, LeaseStartDate: leaseStart, LeaseEndDate: leaseEnd,
			DiscountRateType: stringPointer(draft.DiscountRateType), DiscountRateValue: discountRatePtr,
			DiscountRateMissing: discountRatePtr == nil, LeaseScope: leaseScope,
			ExemptionReason: stringPointer(draft.ExemptionReason), ScopeSource: stringPointer(firstNonEmpty(draft.ScopeSource, "ai")),
			ScopeConfidence: floatPointerOrNil(draft.ScopeConfidence), AIConfidenceScore: floatPointerOrNil(draft.Confidence),
			ScopeClassifiedBy: &actorID, ScopeClassifiedAt: &now,
			SourceReferenceLocator: map[string]interface{}{
				"artifact_id": artifact.ID, "run_id": artifact.RunID,
				"intake_id": data.Summary.IntakeID, "item_index": index,
			},
		}
		attrs := &access.ContractAttributes{LegalEntityID: valueOrEmpty(legalEntityID), StoreID: valueOrEmpty(storeID)}
		commands = append(commands, draftapp.ContractDraftCommand{
			IdempotencyKey: fmt.Sprintf("artifact:%s:%s:%d", artifact.ID, contractDraftActionKey, index),
			ActorID:        actorID, RunID: artifact.RunID, Contract: contract, AccessAttrs: attrs, RequireEvidence: true,
		})
	}
	return commands, nil
}

func paymentScheduleDraftCommands(
	artifact *repository.AIChatArtifact,
	data artifactPaymentScheduleData,
	selection reviewSelection,
	actorID string,
) ([]draftapp.PaymentScheduleDraftCommand, error) {
	contractID := strings.TrimSpace(data.Summary.ContractID)
	if contractID == "" {
		return nil, errors.New("payment schedule artifact is not bound to a contract")
	}
	indexes := selectedIndexes(len(data.Schedules), selection, func(index int) string {
		return fmt.Sprintf("%d", index)
	})
	commands := make([]draftapp.PaymentScheduleDraftCommand, 0, len(indexes))
	for _, index := range indexes {
		schedule := data.Schedules[index]
		periodStart := parseDraftDate(schedule.PeriodStart)
		periodEnd := parseDraftDate(schedule.PeriodEnd)
		dueDate := parseDraftDate(schedule.DueDate)
		if periodStart.IsZero() {
			periodStart = dueDate
		}
		if periodEnd.IsZero() {
			periodEnd = dueDate
		}
		commands = append(commands, draftapp.PaymentScheduleDraftCommand{
			IdempotencyKey: fmt.Sprintf("artifact:%s:%s:%d", artifact.ID, paymentScheduleDraftActionKey, index),
			ActorID:        actorID, RunID: artifact.RunID,
			Schedule: &repository.PaymentSchedule{
				ContractID: contractID, EffectiveStartDate: periodStart, EffectiveEndDate: periodEnd,
				CoverageStartDate: periodStart, CoverageEndDate: periodEnd, DueDate: dueDate,
				PaymentTiming: strings.TrimSpace(schedule.PaymentTiming), Amount: schedule.Amount,
				Currency: strings.ToUpper(strings.TrimSpace(schedule.Currency)), AmountType: firstNonEmpty(schedule.AmountType, "fixed_rent"),
				IsFixed: schedule.IsFixed, IsVariable: !schedule.IsFixed,
				IsLeaseComponent: schedule.IsLeaseComponent, IsNonLeaseComponent: !schedule.IsLeaseComponent,
				IncludedInLiabilityPV: schedule.IsLeaseComponent && schedule.IsFixed,
			},
			EvidenceRef:     map[string]interface{}{"artifact_id": artifact.ID, "run_id": artifact.RunID, "item_index": index},
			RequireEvidence: true,
		})
	}
	return commands, nil
}

func parseReviewSelection(payload map[string]interface{}) (reviewSelection, error) {
	if payload == nil {
		return reviewSelection{}, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return reviewSelection{}, fmt.Errorf("encode review selection: %w", err)
	}
	var selection reviewSelection
	if err := json.Unmarshal(raw, &selection); err != nil {
		return reviewSelection{}, fmt.Errorf("decode review selection: %w", err)
	}
	return selection, nil
}

func selectedIndexes(length int, selection reviewSelection, label func(int) string) []int {
	if len(selection.Indexes) > 0 {
		result := make([]int, 0, len(selection.Indexes))
		seen := make(map[int]struct{}, len(selection.Indexes))
		for _, index := range selection.Indexes {
			if index < 0 || index >= length {
				continue
			}
			if _, exists := seen[index]; exists {
				continue
			}
			seen[index] = struct{}{}
			result = append(result, index)
		}
		return result
	}
	if len(selection.ContractNumbers) > 0 {
		wanted := make(map[string]struct{}, len(selection.ContractNumbers))
		for _, number := range selection.ContractNumbers {
			wanted[strings.TrimSpace(number)] = struct{}{}
		}
		result := make([]int, 0, len(wanted))
		for index := 0; index < length; index++ {
			if _, ok := wanted[strings.TrimSpace(label(index))]; ok {
				result = append(result, index)
			}
		}
		return result
	}
	result := make([]int, length)
	for index := range result {
		result[index] = index
	}
	return result
}

func parseDraftDate(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02", strings.TrimSpace(value))
	return parsed
}

func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func floatPointerOrNil(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func tenantIDFromContext(ctx context.Context) string {
	if scope, scoped := access.ScopeFromContext(ctx); scoped {
		return scope.LegalEntityID
	}
	return ""
}
