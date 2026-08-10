package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeContractReader struct {
	attributes access.ContractAttributes
	contract   *repository.Contract
	getCalls   int
	listCalls  int
}

func (f *fakeContractReader) GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error) {
	return f.attributes, f.contract != nil, nil
}

func (f *fakeContractReader) GetByID(context.Context, string, string) (*repository.Contract, error) {
	f.getCalls++
	return f.contract, nil
}

func (f *fakeContractReader) ListPaged(_ context.Context, _ string, _ repository.ListContractsFilter) ([]*repository.Contract, int, error) {
	f.listCalls++
	if f.contract == nil {
		return nil, 0, nil
	}
	return []*repository.Contract{f.contract}, 1, nil
}

func contractToolContext(scope access.Scope) context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID:      "user-1",
			Permissions: []string{"contracts:read"},
			Scope:       scope,
		},
		RunID: "run-1",
	})
}

func TestContractGetToolReturnsScopedContractViewAndEvidence(t *testing.T) {
	reader := &fakeContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-1"},
		contract: &repository.Contract{
			ID: "contract-1", ContractNumber: "LEASE-001", ContractName: "Store Lease",
			Status: "active", ApprovalStatus: "approved", Currency: "CNY",
			CommencementDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			LeaseEndDate:     time.Date(2029, 12, 31, 0, 0, 0, 0, time.UTC), LeaseScope: "in_scope",
		},
	}
	definition := NewContractGetDefinition(reader)
	call := agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: definition.Descriptor.Version,
		Arguments: json.RawMessage(`{"contract_id":"contract-1"}`),
	}
	result, err := definition.Handler(contractToolContext(access.Scope{LegalEntityID: "le-001"}), call)
	if err != nil || result.Status != agenttools.StatusCompleted {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	view, ok := result.Data.(ContractView)
	if !ok || view.ContractNumber != "LEASE-001" || view.ApprovalStatus != "approved" {
		t.Fatalf("view = %#v", result.Data)
	}
	if len(result.Sources) != 1 || result.Sources[0].Locator != "lease_contracts:contract-1" {
		t.Fatalf("sources = %#v", result.Sources)
	}
	if reader.getCalls != 1 {
		t.Fatalf("full contract reads = %d, want 1", reader.getCalls)
	}
}

func TestContractSearchToolReturnsPagedScopedSummary(t *testing.T) {
	reader := &fakeContractReader{contract: &repository.Contract{
		ID: "contract-1", ContractNumber: "LEASE-001", ContractName: "Store Lease",
		Status: "active", ApprovalStatus: "approved", Currency: "CNY", LeaseScope: "in_scope",
	}}
	definition := NewContractSearchDefinition(reader)
	result, err := definition.Handler(contractToolContext(access.Scope{LegalEntityID: "le-001"}), agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: definition.Descriptor.Version,
		Arguments: json.RawMessage(`{"search":"LEASE","page":2,"page_size":10,"sort_by":"created_at","sort_order":"asc"}`),
	})
	if err != nil || result.Status != agenttools.StatusCompleted {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	data, ok := result.Data.(ContractSearchData)
	if !ok || data.Total != 1 || data.Page != 2 || data.PageSize != 10 || len(data.Items) != 1 {
		t.Fatalf("search data=%#v", result.Data)
	}
	if reader.listCalls != 1 {
		t.Fatalf("ListPaged calls=%d, want 1", reader.listCalls)
	}
}

func TestContractGetToolChecksScopeBeforeLoadingFullContract(t *testing.T) {
	reader := &fakeContractReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-foreign"},
		contract:   &repository.Contract{ID: "contract-foreign"},
	}
	definition := NewContractGetDefinition(reader)
	result, err := definition.Handler(contractToolContext(access.Scope{
		LegalEntityID: "le-001", StoreIDs: []string{"store-allowed"},
	}), agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: definition.Descriptor.Version,
		Arguments: json.RawMessage(`{"contract_id":"contract-foreign"}`),
	})
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorNotFound {
		t.Fatalf("out-of-scope result=%#v err=%v", result, err)
	}
	if reader.getCalls != 0 {
		t.Fatalf("full contract loaded %d times before scope rejection", reader.getCalls)
	}
}

func TestContractGetToolDoesNotExposeUnknownArguments(t *testing.T) {
	reader := &fakeContractReader{contract: &repository.Contract{ID: "contract-1"}, attributes: access.ContractAttributes{LegalEntityID: "le-001"}}
	definition := NewContractGetDefinition(reader)
	result, err := definition.Handler(contractToolContext(access.Scope{LegalEntityID: "le-001"}), agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: definition.Descriptor.Version,
		Arguments: json.RawMessage(`{"contract_id":"contract-1","legal_entity_id":"le-999"}`),
	})
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("unknown argument result=%#v err=%v", result, err)
	}
}

func TestContractGetToolEnforcesAllAssignedScopeDimensions(t *testing.T) {
	attributes := access.ContractAttributes{
		LegalEntityID: "le-001", StoreID: "store-1", Region: "east", Brand: "brand-a",
	}
	tests := []struct {
		name  string
		scope access.Scope
		allow bool
	}{
		{name: "matching tenant and dimensions", scope: access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}, Regions: []string{"east"}, Brands: []string{"brand-a"}}, allow: true},
		{name: "foreign legal entity", scope: access.Scope{LegalEntityID: "le-002"}, allow: false},
		{name: "foreign store", scope: access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-2"}}, allow: false},
		{name: "foreign region", scope: access.Scope{LegalEntityID: "le-001", Regions: []string{"west"}}, allow: false},
		{name: "foreign brand", scope: access.Scope{LegalEntityID: "le-001", Brands: []string{"brand-b"}}, allow: false},
		{name: "global admin", scope: access.Scope{Global: true}, allow: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &fakeContractReader{
				attributes: attributes,
				contract:   &repository.Contract{ID: "contract-1", ContractName: "Scoped Lease"},
			}
			definition := NewContractGetDefinition(reader)
			result, err := definition.Handler(contractToolContext(test.scope), agenttools.ToolCall{
				CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name,
				ToolVersion: definition.Descriptor.Version, Arguments: json.RawMessage(`{"contract_id":"contract-1"}`),
			})
			if test.allow {
				if err != nil || result.Status != agenttools.StatusCompleted {
					t.Fatalf("allowed result=%#v err=%v", result, err)
				}
				return
			}
			if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorNotFound {
				t.Fatalf("denied result=%#v err=%v", result, err)
			}
			if reader.getCalls != 0 {
				t.Fatalf("full contract loaded %d times after scope denial", reader.getCalls)
			}
		})
	}
}
