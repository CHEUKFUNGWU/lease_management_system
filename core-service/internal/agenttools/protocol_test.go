package agenttools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
)

func TestToolDescriptorRejectsWriteToolWithoutReviewAndIdempotency(t *testing.T) {
	descriptor := ToolDescriptor{
		Name:        "lease.contract.create_draft",
		Version:     "v1",
		Description: "create a contract draft",
		Level:       LevelDraft,
		Permissions: []Permission{{Resource: "contracts", Action: "create"}},
	}

	if err := descriptor.Validate(); !errors.Is(err, ErrInvalidToolDescriptor) {
		t.Fatalf("expected invalid descriptor error, got %v", err)
	}
}

func TestToolCallDoesNotAcceptNonObjectArguments(t *testing.T) {
	call := ToolCall{
		CallID:      "call-1",
		RunID:       "run-1",
		ToolName:    "lease.contract.get",
		ToolVersion: "v1",
		Arguments:   json.RawMessage(`["foreign-contract"]`),
	}

	if err := call.Validate(); !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("expected invalid call error, got %v", err)
	}
}

func TestDefaultPolicyAllowsReadAndRequiresReviewForDraft(t *testing.T) {
	ctx := WithExecutionContext(context.Background(), ExecutionContext{
		Principal: Principal{
			UserID:      "user-1",
			Permissions: []string{"contracts:read", "contracts:create"},
		},
		RunID: "run-1",
	})

	read := ToolDescriptor{
		Name:        "lease.contract.get",
		Version:     "v1",
		Description: "get a contract",
		Level:       LevelRead,
		ReadOnly:    true,
		Permissions: []Permission{{Resource: "contracts", Action: "read"}},
	}
	readDecision, err := Evaluate(ctx, read, ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: read.Name, ToolVersion: read.Version,
		Arguments: json.RawMessage(`{"contract_id":"contract-1"}`),
	}, DefaultPolicy())
	if err != nil || !readDecision.Allowed || readDecision.RequiresReview {
		t.Fatalf("read decision = %+v, err=%v", readDecision, err)
	}

	draft := ToolDescriptor{
		Name:                "lease.contract.create_draft",
		Version:             "v1",
		Description:         "create a contract draft",
		Level:               LevelDraft,
		Permissions:         []Permission{{Resource: "contracts", Action: "create"}},
		Review:              ReviewPolicy{Required: true, Reasons: []string{"draft must be confirmed"}},
		SupportsIdempotency: true,
	}
	draftDecision, err := Evaluate(ctx, draft, ToolCall{
		CallID: "call-2", RunID: "run-1", ToolName: draft.Name, ToolVersion: draft.Version,
		Arguments: json.RawMessage(`{"contract_name":"draft"}`), IdempotencyKey: "idem-1",
	}, DefaultPolicy())
	if err != nil || !draftDecision.Allowed || !draftDecision.RequiresReview {
		t.Fatalf("draft decision = %+v, err=%v", draftDecision, err)
	}
}

func TestDefaultPolicyRejectsCommandWithoutCapability(t *testing.T) {
	ctx := WithExecutionContext(context.Background(), ExecutionContext{
		Principal: Principal{UserID: "user-1", Permissions: []string{"monthly_closing:post"}},
	})
	descriptor := ToolDescriptor{
		Name:                "lease.monthly_close.post",
		Version:             "v1",
		Description:         "post a journal",
		Level:               LevelCommand,
		Permissions:         []Permission{{Resource: "monthly_closing", Action: "post"}},
		Review:              ReviewPolicy{Required: true},
		SupportsIdempotency: true,
	}
	_, err := Evaluate(ctx, descriptor, ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: descriptor.Name, ToolVersion: descriptor.Version,
		Arguments: json.RawMessage(`{}`), IdempotencyKey: "idem-1",
	}, DefaultPolicy())
	if !errors.Is(err, ErrToolCapabilityRequired) {
		t.Fatalf("expected capability rejection, got %v", err)
	}
}

type fakeContractAccessReader struct {
	attributes access.ContractAttributes
	found      bool
}

func (f fakeContractAccessReader) GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error) {
	return f.attributes, f.found, nil
}

func TestRequireContractAccessRejectsForeignStoreBeforeLinkedRead(t *testing.T) {
	ctx := access.WithScope(context.Background(), access.Scope{
		LegalEntityID: "le-001",
		StoreIDs:      []string{"store-allowed"},
	})
	_, err := RequireContractAccess(ctx, "contract-foreign", fakeContractAccessReader{
		attributes: access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-foreign"},
		found:      true,
	})
	if !errors.Is(err, ErrContractOutOfScope) {
		t.Fatalf("expected out-of-scope error, got %v", err)
	}
}

func TestRequireContractAccessRejectsMissingScope(t *testing.T) {
	_, err := RequireContractAccess(context.Background(), "contract-1", fakeContractAccessReader{found: true})
	if !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("expected missing scope error, got %v", err)
	}
}
