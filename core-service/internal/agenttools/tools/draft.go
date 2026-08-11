package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

type contractDraftToolArguments struct {
	Contract        repository.Contract        `json:"contract"`
	AccessAttrs     *access.ContractAttributes `json:"access_attrs,omitempty"`
	RequireEvidence bool                       `json:"require_evidence,omitempty"`
}

type paymentScheduleDraftToolArguments struct {
	Schedule        repository.PaymentSchedule `json:"schedule"`
	EvidenceRef     map[string]any             `json:"evidence_ref,omitempty"`
	RequireEvidence bool                       `json:"require_evidence,omitempty"`
}

type eventDraftToolArguments struct {
	Event           eventDraftInput `json:"event"`
	EvidenceRef     map[string]any  `json:"evidence_ref,omitempty"`
	RequireEvidence bool            `json:"require_evidence,omitempty"`
}

type eventDraftInput struct {
	ContractID         string          `json:"contract_id"`
	EventType          string          `json:"event_type"`
	EffectiveDate      string          `json:"effective_date"`
	OriginalValue      *string         `json:"original_value,omitempty"`
	NewValue           *string         `json:"new_value,omitempty"`
	ChangeReason       string          `json:"change_reason"`
	JudgmentBasis      string          `json:"judgment_basis,omitempty"`
	RevisionParameters json.RawMessage `json:"revision_parameters,omitempty"`
}

func NewContractDraftDefinition(service *draftapp.Service) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.contract.draft.create",
			Version:     "v1",
			DisplayName: "创建合同草稿",
			Description: "将已识别并带证据的合同写入 AI 草稿层，不进入正式报表或会计计量",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "contracts", Action: "create"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["contract"],
  "properties":{
    "contract":{"type":"object"},
    "access_attrs":{"type":"object"},
    "require_evidence":{"type":"boolean"}
  }
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["operation","idempotency_key","status"],
  "properties":{"operation":{"type":"string"},"idempotency_key":{"type":"string"},"status":{"type":"string"},"id":{"type":"string"},"error":{"type":"string"}}
}`),
			Review: agenttools.ReviewPolicy{
				Required:      true,
				Reasons:       []string{"合同写入草稿层后仍需 Editor/Reviewer 确认"},
				ConfirmAction: "review_ai_draft",
			},
			Retry:               agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"contract_review", "excel_ledger", "contract_batch_intake"},
		Handler:  contractDraftHandler(service),
	}
}

func NewPaymentScheduleDraftDefinition(service *draftapp.Service) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.payment_schedule.draft.create",
			Version:     "v1",
			DisplayName: "创建付款计划草稿",
			Description: "将带来源证据的付款计划写入 AI 草稿层，变量租金和非租赁成分不得资本化",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "payment_schedules", Action: "create"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["schedule","evidence_ref"],
  "properties":{"schedule":{"type":"object"},"evidence_ref":{"type":"object"},"require_evidence":{"type":"boolean"}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["operation","idempotency_key","status"],
  "properties":{"operation":{"type":"string"},"idempotency_key":{"type":"string"},"status":{"type":"string"},"id":{"type":"string"},"error":{"type":"string"}}
}`),
			Review: agenttools.ReviewPolicy{
				Required:      true,
				Reasons:       []string{"付款计划写入草稿层后仍需逐条复核"},
				ConfirmAction: "review_ai_draft",
			},
			Retry:               agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"payment_schedule", "payment_schedule_intake"},
		Handler:  paymentScheduleDraftHandler(service),
	}
}

func NewEventDraftDefinition(service *draftapp.Service) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.event.draft.create",
			Version:     "v1",
			DisplayName: "创建事件草稿",
			Description: "将合同变更事件写入草稿层，后续必须经过事件复核和审批才能触发重算",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "events", Action: "create"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["event","evidence_ref"],
  "properties":{"event":{"type":"object"},"evidence_ref":{"type":"object"},"require_evidence":{"type":"boolean"}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["operation","idempotency_key","status"],
  "properties":{"operation":{"type":"string"},"idempotency_key":{"type":"string"},"status":{"type":"string"},"id":{"type":"string"},"error":{"type":"string"}}
}`),
			Review: agenttools.ReviewPolicy{
				Required:      true,
				Reasons:       []string{"事件写入草稿层后仍需确认会计处理和生效日期"},
				ConfirmAction: "review_ai_draft",
			},
			Retry:               agenttools.RetryPolicy{Retryable: true, MaxAttempts: 2},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			TimeoutSeconds:      30,
		},
		SkillIDs: []string{"event_change"},
		Handler:  eventDraftHandler(service),
	}
}

func contractDraftHandler(service *draftapp.Service) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeContractDraftArguments(call.Arguments)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		result := service.CreateContractDraft(ctx, draftapp.ContractDraftCommand{
			IdempotencyKey: call.IdempotencyKey,
			ActorID:        execution.Principal.UserID,
			Contract:       &args.Contract,
			AccessAttrs:    args.AccessAttrs,
			// This tool is an AI-facing write boundary. The boolean in the
			// payload is not trusted to disable evidence enforcement.
			RequireEvidence: true,
		})
		return draftToolResult(call.CallID, result), nil
	}
}

func paymentScheduleDraftHandler(service *draftapp.Service) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodePaymentScheduleDraftArguments(call.Arguments)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		result := service.CreatePaymentScheduleDraft(ctx, draftapp.PaymentScheduleDraftCommand{
			IdempotencyKey:  call.IdempotencyKey,
			ActorID:         execution.Principal.UserID,
			Schedule:        &args.Schedule,
			EvidenceRef:     args.EvidenceRef,
			RequireEvidence: true,
		})
		return draftToolResult(call.CallID, result), nil
	}
}

func eventDraftHandler(service *draftapp.Service) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeEventDraftArguments(call.Arguments)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return draftRejected(call.CallID, err), nil
		}
		result := service.CreateEventDraft(ctx, draftapp.EventDraftCommand{
			IdempotencyKey:  call.IdempotencyKey,
			ActorID:         execution.Principal.UserID,
			Event:           eventFromDraftInput(args.Event),
			EvidenceRef:     args.EvidenceRef,
			RequireEvidence: true,
		})
		return draftToolResult(call.CallID, result), nil
	}
}

func draftToolResult(callID string, result draftapp.ItemResult) agenttools.ToolResult {
	toolResult := agenttools.ToolResult{CallID: callID, Data: result}
	if result.Status == draftapp.ItemFailed {
		toolResult.Status = agenttools.StatusFailed
		toolResult.Error = &agenttools.ToolError{Code: agenttools.ErrorBusinessFailure, Message: result.Error}
		return toolResult
	}
	toolResult.Status = agenttools.StatusCompleted
	return toolResult
}

func draftRejected(callID string, err error) agenttools.ToolResult {
	message := "invalid draft arguments"
	if err != nil {
		message = err.Error()
	}
	return agenttools.ToolResult{
		CallID: callID,
		Status: agenttools.StatusRejected,
		Error:  &agenttools.ToolError{Code: agenttools.ErrorInvalidArguments, Message: message},
	}
}

func decodeContractDraftArguments(raw json.RawMessage) (contractDraftToolArguments, error) {
	var args contractDraftToolArguments
	if err := decodeDraftArguments(raw, &args); err != nil {
		return contractDraftToolArguments{}, fmt.Errorf("invalid contract draft arguments: %w", err)
	}
	if strings.TrimSpace(args.Contract.ContractNumber) == "" {
		return contractDraftToolArguments{}, errors.New("contract.contract_number is required")
	}
	return args, nil
}

func decodePaymentScheduleDraftArguments(raw json.RawMessage) (paymentScheduleDraftToolArguments, error) {
	var args paymentScheduleDraftToolArguments
	if err := decodeDraftArguments(raw, &args); err != nil {
		return paymentScheduleDraftToolArguments{}, fmt.Errorf("invalid payment schedule draft arguments: %w", err)
	}
	if strings.TrimSpace(args.Schedule.ContractID) == "" {
		return paymentScheduleDraftToolArguments{}, errors.New("schedule.contract_id is required")
	}
	if strings.TrimSpace(asString(args.EvidenceRef["artifact_id"])) == "" &&
		strings.TrimSpace(asString(args.EvidenceRef["source_file_id"])) == "" {
		return paymentScheduleDraftToolArguments{}, errors.New("evidence_ref.artifact_id or source_file_id is required")
	}
	return args, nil
}

func decodeEventDraftArguments(raw json.RawMessage) (eventDraftToolArguments, error) {
	var args eventDraftToolArguments
	if err := decodeDraftArguments(raw, &args); err != nil {
		return eventDraftToolArguments{}, fmt.Errorf("invalid event draft arguments: %w", err)
	}
	if strings.TrimSpace(args.Event.ContractID) == "" {
		return eventDraftToolArguments{}, errors.New("event.contract_id is required")
	}
	if strings.TrimSpace(args.Event.EventType) == "" {
		return eventDraftToolArguments{}, errors.New("event.event_type is required")
	}
	if strings.TrimSpace(args.Event.ChangeReason) == "" {
		return eventDraftToolArguments{}, errors.New("event.change_reason is required")
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(args.Event.EffectiveDate)); err != nil {
		return eventDraftToolArguments{}, errors.New("event.effective_date must be YYYY-MM-DD")
	}
	if strings.TrimSpace(asString(args.EvidenceRef["artifact_id"])) == "" &&
		strings.TrimSpace(asString(args.EvidenceRef["source_file_id"])) == "" {
		return eventDraftToolArguments{}, errors.New("evidence_ref.artifact_id or source_file_id is required")
	}
	return args, nil
}

func eventFromDraftInput(input eventDraftInput) *repository.LeaseEvent {
	effectiveDate, _ := time.Parse("2006-01-02", strings.TrimSpace(input.EffectiveDate))
	return &repository.LeaseEvent{
		ContractID: input.ContractID, EventType: input.EventType,
		EffectiveDate: effectiveDate, OriginalValue: input.OriginalValue,
		NewValue: input.NewValue, ChangeReason: stringPtr(input.ChangeReason),
		JudgmentBasis: stringPtr(input.JudgmentBasis), RevisionParameters: input.RevisionParameters,
	}
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func decodeDraftArguments(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func asString(value any) string {
	valueString, _ := value.(string)
	return valueString
}
