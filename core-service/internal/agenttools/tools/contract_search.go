package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type ContractSearchReader interface {
	ListPaged(context.Context, string, repository.ListContractsFilter) ([]*repository.Contract, int, error)
}

type ContractSearchArguments struct {
	Search    string   `json:"search,omitempty"`
	Status    string   `json:"status,omitempty"`
	Statuses  []string `json:"statuses,omitempty"`
	Page      int      `json:"page,omitempty"`
	PageSize  int      `json:"page_size,omitempty"`
	SortBy    string   `json:"sort_by,omitempty"`
	SortOrder string   `json:"sort_order,omitempty"`
}

type ContractSearchData struct {
	Items    []ContractView `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

func NewContractSearchDefinition(reader ContractSearchReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "lease.contract.search",
			Version:     "v1",
			DisplayName: "搜索合同",
			Description: "在当前身份和数据范围内分页搜索合同，返回裁剪后的合同摘要和审批状态",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "contracts", Action: "read"}},
			InputSchema: json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "properties":{
    "search":{"type":"string"},"status":{"type":"string"},"statuses":{"type":"array","items":{"type":"string"}},
    "page":{"type":"integer","minimum":1},"page_size":{"type":"integer","minimum":1,"maximum":100},
    "sort_by":{"type":"string","enum":["contract_number","commencement_date","lease_end_date","approval_status","created_at"]},
    "sort_order":{"type":"string","enum":["asc","desc"]}
  }
}`),
			OutputSchema: json.RawMessage(`{
  "type":"object",
  "required":["items","total","page","page_size"],
  "properties":{"items":{"type":"array"},"total":{"type":"integer"},"page":{"type":"integer"},"page_size":{"type":"integer"}}
}`),
			SupportsDryRun: true,
			MaxRows:        100,
		},
		SkillIDs: []string{"contract_review", "audit_pack", "event_change"},
		Handler:  contractSearchHandler(reader),
	}
}

func contractSearchHandler(reader ContractSearchReader) agenttools.ToolHandler {
	return func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		args, err := decodeStrict[ContractSearchArguments](call.Arguments)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "invalid contract search arguments"), nil
		}
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize == 0 {
			args.PageSize = 20
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			return rejected(call.CallID, agenttools.ErrorInvalidArguments, "page_size must be between 1 and 100"), nil
		}
		execution, err := agenttools.RequireExecutionContext(ctx)
		if err != nil {
			return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
		}
		if reader == nil {
			return rejected(call.CallID, agenttools.ErrorSystemFailure, "contract search reader is unavailable"), nil
		}
		contracts, total, err := reader.ListPaged(ctx, execution.Principal.Scope.LegalEntityID, repository.ListContractsFilter{
			Search: args.Search, Status: args.Status, Statuses: args.Statuses,
			Page: args.Page, PageSize: args.PageSize, SortBy: args.SortBy, SortOrder: args.SortOrder,
		})
		if err != nil {
			return agenttools.ToolResult{}, err
		}
		data := ContractSearchData{Items: make([]ContractView, 0, len(contracts)), Total: total, Page: args.Page, PageSize: args.PageSize}
		for _, contract := range contracts {
			if contract == nil {
				continue
			}
			data.Items = append(data.Items, ContractView{
				ID: contract.ID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
				Status: contract.Status, ApprovalStatus: contract.ApprovalStatus,
				LesseeName: contract.LesseeName, LessorName: contract.LessorName, StoreName: contract.StoreName,
				Currency: contract.Currency, CommencementDate: contract.CommencementDate, LeaseEndDate: contract.LeaseEndDate,
				DiscountRateValue: contract.DiscountRateValue, DiscountRateMissing: contract.DiscountRateMissing,
				LeaseScope: strings.TrimSpace(contract.LeaseScope),
			})
		}
		data.Total = total
		return agenttools.ToolResult{
			CallID: call.CallID, Status: agenttools.StatusCompleted, Data: data,
			Sources: []agenttools.ToolSource{{Type: "contract", ID: "search", Title: "合同搜索", Locator: "lease_contracts:search"}},
		}, nil
	}
}
