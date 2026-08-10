package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type ContractReader interface {
	agenttools.ContractAccessReader
	GetByID(context.Context, string, string) (*repository.Contract, error)
}

type ContractGetArguments struct {
	ContractID string `json:"contract_id"`
}

// ContractView is deliberately narrower than repository.Contract. Tool output
// is model-readable and should not become an accidental dump of every ledger
// or internal metadata column.
type ContractView struct {
	ID                  string    `json:"id"`
	ContractNumber      string    `json:"contract_number"`
	ContractName        string    `json:"contract_name"`
	Status              string    `json:"status"`
	ApprovalStatus      string    `json:"approval_status"`
	LesseeName          string    `json:"lessee_name,omitempty"`
	LessorName          string    `json:"lessor_name,omitempty"`
	StoreName           string    `json:"store_name,omitempty"`
	Currency            string    `json:"currency"`
	CommencementDate    time.Time `json:"commencement_date"`
	LeaseEndDate        time.Time `json:"lease_end_date"`
	DiscountRateValue   *float64  `json:"discount_rate_value,omitempty"`
	DiscountRateMissing bool      `json:"discount_rate_missing"`
	LeaseScope          string    `json:"lease_scope"`
}

func NewContractGetDefinition(reader ContractReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.contract.get",
			Version:     "v1",
			DisplayName: "读取合同详情",
			Description: "读取当前身份和数据范围内的合同关键字段",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "contracts", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["contract_id"],
  "properties":{"contract_id":{"type":"string","minLength":1}}
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["id","contract_number","contract_name","approval_status"],
  "properties":{"id":{"type":"string"},"contract_number":{"type":"string"},"contract_name":{"type":"string"},"approval_status":{"type":"string"}}
}`),
			SupportsDryRun: true,
		},
		SkillIDs: []string{"contract_review", "audit_pack"},
		Handler:  contractGetHandler(reader),
	}
}

func contractGetHandler(reader ContractReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeArguments(call.Arguments)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "contract_id is required"), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if _, err := agenttools.RequireContractAccess(ctx, args.ContractID, reader); err != nil {
			// Not-found and out-of-scope intentionally share one public result so
			// a forged ID cannot be used as an existence oracle.
			if errors.Is(err, agenttools.ErrContractNotFound) || errors.Is(err, agenttools.ErrContractOutOfScope) {
				return rejected(call.CallID, agenttools.ErrorNotFound, "contract not found"), nil
			}
			return agenttools.ToolResult{}, err
		}
		contract, err := reader.GetByID(ctx, args.ContractID, execution.Principal.Scope.LegalEntityID)
		if err != nil {
			return agenttools.ToolResult{}, err
		}
		if contract == nil {
			return rejected(call.CallID, agenttools.ErrorNotFound, "contract not found"), nil
		}
		return agenttools.ToolResult{
			CallID: call.CallID,
			Status: agenttools.StatusCompleted,
			Data: ContractView{
				ID: contract.ID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
				Status: contract.Status, ApprovalStatus: contract.ApprovalStatus,
				LesseeName: contract.LesseeName, LessorName: contract.LessorName,
				StoreName: contract.StoreName, Currency: contract.Currency,
				CommencementDate: contract.CommencementDate, LeaseEndDate: contract.LeaseEndDate,
				DiscountRateValue: contract.DiscountRateValue, DiscountRateMissing: contract.DiscountRateMissing,
				LeaseScope: contract.LeaseScope,
			},
			Sources: []agenttools.ToolSource{{
				Type: "contract", ID: contract.ID, Title: contract.ContractName,
				Locator: "lease_contracts:" + contract.ID,
			}},
		}, nil
	}
}

func decodeArguments(raw json.RawMessage) (ContractGetArguments, error) {
	var args ContractGetArguments
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil || args.ContractID == "" {
		return ContractGetArguments{}, errors.New("invalid contract get arguments")
	}
	return args, nil
}

func rejected(callID string, code agenttools.ErrorCode, message string) agenttools.ToolResult {
	return agenttools.ToolResult{
		CallID: callID,
		Status: agenttools.StatusRejected,
		Error:  &agenttools.ToolError{Code: code, Message: message},
	}
}
