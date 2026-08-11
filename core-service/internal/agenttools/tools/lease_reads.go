package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

// MeasurementReader, JournalReader and EventReader are intentionally narrow
// seams. The Tool handlers never receive a database handle and never expose
// repository structs directly to an Agent.
type MeasurementReader interface {
	GetMeasurementResults(context.Context, string, string) ([]*repository.MeasurementResult, error)
}

type JournalReader interface {
	GetJournalEntries(context.Context, string, string, string) ([]*repository.JournalEntry, error)
}

type EventReader interface {
	GetByContractID(context.Context, string) ([]*repository.LeaseEvent, error)
}

type MeasurementListArguments struct {
	ContractID string `json:"contract_id"`
	Period     string `json:"period,omitempty"`
}

type MeasurementView struct {
	AccountingPeriod   string  `json:"accounting_period"`
	OpeningLiability   float64 `json:"opening_liability"`
	ClosingLiability   float64 `json:"closing_liability"`
	InterestExpense    float64 `json:"interest_expense"`
	PrincipalRepayment float64 `json:"principal_repayment"`
	Depreciation       float64 `json:"depreciation"`
	ClosingROUAsset    float64 `json:"closing_rou_asset"`
}

type MeasurementListData struct {
	Items []MeasurementView `json:"items"`
	Total int               `json:"total"`
}

type EventListArguments struct {
	ContractID string `json:"contract_id"`
}

type EventView struct {
	EventType      string    `json:"event_type"`
	EffectiveDate  time.Time `json:"effective_date"`
	Status         string    `json:"status"`
	ApprovalStatus string    `json:"approval_status"`
	ChangeReason   string    `json:"change_reason,omitempty"`
}

type EventListData struct {
	Items []EventView `json:"items"`
	Total int         `json:"total"`
}

type JournalListArguments struct {
	ContractID string `json:"contract_id"`
	Period     string `json:"period,omitempty"`
	Status     string `json:"status,omitempty"`
}

type JournalView struct {
	AccountingPeriod string  `json:"accounting_period"`
	EntryType        string  `json:"entry_type"`
	DebitAccount     string  `json:"debit_account"`
	CreditAccount    string  `json:"credit_account"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
	PostingStatus    string  `json:"posting_status"`
	Description      string  `json:"description,omitempty"`
}

type JournalListData struct {
	Items []JournalView `json:"items"`
	Total int           `json:"total"`
}

func NewMeasurementListDefinition(contractReader ContractReader, reader MeasurementReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.measurement.list",
			Version:     "v1",
			DisplayName: "读取租赁计量结果",
			Description: "读取当前身份和数据范围内合同的租赁负债及使用权资产计量结果",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "calculations", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["contract_id"],
  "properties":{"contract_id":{"type":"string","minLength":1},"period":{"type":"string"}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["items","total"],
  "properties":{"items":{"type":"array"},"total":{"type":"integer"}}
}`),
			SupportsDryRun: true,
			MaxRows:        500,
		},
		SkillIDs: []string{"contract_review", "audit_pack"},
		Handler:  measurementListHandler(contractReader, reader),
	}
}

func NewEventListDefinition(contractReader ContractReader, reader EventReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.event.list",
			Version:     "v1",
			DisplayName: "读取租赁变更事件",
			Description: "读取当前身份和数据范围内合同的租赁变更、重估和减值事件",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "events", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["contract_id"],
  "properties":{"contract_id":{"type":"string","minLength":1}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["items","total"],
  "properties":{"items":{"type":"array"},"total":{"type":"integer"}}
}`),
			SupportsDryRun: true,
			MaxRows:        500,
		},
		SkillIDs: []string{"contract_review", "audit_pack"},
		Handler:  eventListHandler(contractReader, reader),
	}
}

func NewJournalListDefinition(contractReader ContractReader, reader JournalReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.journal.list",
			Version:     "v1",
			DisplayName: "读取租赁会计分录",
			Description: "读取当前身份和数据范围内合同的租赁会计分录及过账状态",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "monthly_closing", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["contract_id"],
  "properties":{"contract_id":{"type":"string","minLength":1},"period":{"type":"string"},"status":{"type":"string"}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["items","total"],
  "properties":{"items":{"type":"array"},"total":{"type":"integer"}}
}`),
			SupportsDryRun: true,
			MaxRows:        500,
		},
		SkillIDs: []string{"contract_review", "audit_pack"},
		Handler:  journalListHandler(contractReader, reader),
	}
}

func measurementListHandler(contractReader ContractReader, reader MeasurementReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeStrict[MeasurementListArguments](call.Arguments)
		if err != nil || args.ContractID == "" {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "contract_id is required"), nil
		}
		if err := requireReadContract(ctx, args.ContractID, contractReader); err != nil {
			if errors.Is(err, agenttools.ErrContractNotFound) || errors.Is(err, agenttools.ErrContractOutOfScope) {
				return rejected(call.CallID, agenttools.ErrorNotFound, "contract not found"), nil
			}
			return agenttools.ToolResult{}, err
		}
		results, err := reader.GetMeasurementResults(ctx, args.ContractID, args.Period)
		if err != nil {
			return agenttools.ToolResult{}, err
		}
		data := MeasurementListData{Items: make([]MeasurementView, 0, len(results)), Total: len(results)}
		for _, result := range results {
			if result == nil {
				continue
			}
			data.Items = append(data.Items, MeasurementView{
				AccountingPeriod: result.AccountingPeriod, OpeningLiability: result.OpeningLiability,
				ClosingLiability: result.ClosingLiability, InterestExpense: result.InterestExpense,
				PrincipalRepayment: result.PrincipalRepayment, Depreciation: result.Depreciation,
				ClosingROUAsset: result.ClosingROUAsset,
			})
		}
		data.Total = len(data.Items)
		return agenttools.ToolResult{
			CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
			Sources: []agenttools.ToolSource{{Type: "measurement", ID: args.ContractID, Title: "计量结果", Locator: "measurement_results:contract:" + args.ContractID}},
		}, nil
	}
}

func eventListHandler(contractReader ContractReader, reader EventReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeStrict[EventListArguments](call.Arguments)
		if err != nil || args.ContractID == "" {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "contract_id is required"), nil
		}
		if err := requireReadContract(ctx, args.ContractID, contractReader); err != nil {
			if errors.Is(err, agenttools.ErrContractNotFound) || errors.Is(err, agenttools.ErrContractOutOfScope) {
				return rejected(call.CallID, agenttools.ErrorNotFound, "contract not found"), nil
			}
			return agenttools.ToolResult{}, err
		}
		events, err := reader.GetByContractID(ctx, args.ContractID)
		if err != nil {
			return agenttools.ToolResult{}, err
		}
		data := EventListData{Items: make([]EventView, 0, len(events)), Total: len(events)}
		for _, event := range events {
			if event == nil {
				continue
			}
			reason := ""
			if event.ChangeReason != nil {
				reason = *event.ChangeReason
			}
			data.Items = append(data.Items, EventView{
				EventType: event.EventType, EffectiveDate: event.EffectiveDate,
				Status: event.Status, ApprovalStatus: event.ApprovalStatus, ChangeReason: reason,
			})
		}
		data.Total = len(data.Items)
		return agenttools.ToolResult{
			CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
			Sources: []agenttools.ToolSource{{Type: "event", ID: args.ContractID, Title: "变更事件", Locator: "lease_events:contract:" + args.ContractID}},
		}, nil
	}
}

func journalListHandler(contractReader ContractReader, reader JournalReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeStrict[JournalListArguments](call.Arguments)
		if err != nil || args.ContractID == "" {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "contract_id is required"), nil
		}
		if err := requireReadContract(ctx, args.ContractID, contractReader); err != nil {
			if errors.Is(err, agenttools.ErrContractNotFound) || errors.Is(err, agenttools.ErrContractOutOfScope) {
				return rejected(call.CallID, agenttools.ErrorNotFound, "contract not found"), nil
			}
			return agenttools.ToolResult{}, err
		}
		entries, err := reader.GetJournalEntries(ctx, args.ContractID, args.Period, args.Status)
		if err != nil {
			return agenttools.ToolResult{}, err
		}
		data := JournalListData{Items: make([]JournalView, 0, len(entries)), Total: len(entries)}
		for _, entry := range entries {
			if entry == nil {
				continue
			}
			description := ""
			if entry.Description != nil {
				description = *entry.Description
			}
			data.Items = append(data.Items, JournalView{
				AccountingPeriod: entry.AccountingPeriod, EntryType: entry.EntryType,
				DebitAccount: entry.DebitAccount, CreditAccount: entry.CreditAccount,
				Amount: entry.Amount, Currency: entry.Currency, PostingStatus: entry.PostingStatus,
				Description: description,
			})
		}
		data.Total = len(data.Items)
		return agenttools.ToolResult{
			CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
			Sources: []agenttools.ToolSource{{Type: "journal", ID: args.ContractID, Title: "会计分录", Locator: "journal_entries:contract:" + args.ContractID}},
		}, nil
	}
}

func requireReadContract(ctx context.Context, contractID string, reader ContractReader) error {
	_, err := agenttools.RequireContractAccess(ctx, contractID, reader)
	return err
}

func decodeStrict[T any](raw json.RawMessage) (T, error) {
	var value T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return value, errors.New("multiple JSON values")
		}
		return value, err
	}
	return value, nil
}
