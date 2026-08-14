package agenttools

import (
	"errors"

	"github.com/lease-management-system/core-service/internal/errcontract"
)

var (
	ErrExecutionContextRequired = errors.New("agent tool execution context is required")
	ErrScopeUnavailable         = errors.New("agent tool data scope is unavailable")
	ErrContractIDRequired       = errors.New("contract id is required")
	ErrContractNotFound         = errors.New("contract not found")
	ErrContractOutOfScope       = errors.New("contract is outside the assigned data scope")
	ErrInvalidToolDescriptor    = errors.New("invalid tool descriptor")
	ErrInvalidToolCall          = errors.New("invalid tool call")
	ErrToolNotPermitted         = errors.New("tool is not permitted")
	ErrToolCapabilityRequired   = errors.New("tool capability is required")
)

// The error-code vocabulary lives in errcontract (shared with the HTTP seam).
// This file re-exports it so the Agent Tool seam keeps its public names and
// JSON shape without change; callers branch on the code, never on the
// human-readable text.
type ErrorCode = errcontract.Code

const (
	ErrorInvalidArguments = errcontract.CodeInvalidArguments
	ErrorUnauthenticated  = errcontract.CodeUnauthenticated
	ErrorPermissionDenied = errcontract.CodePermissionDenied
	ErrorScopeDenied      = errcontract.CodeScopeDenied
	ErrorReviewRequired   = errcontract.CodeReviewRequired
	ErrorCapabilityDenied = errcontract.CodeCapabilityDenied
	ErrorBusinessFailure  = errcontract.CodeBusinessFailure
	ErrorSystemFailure    = errcontract.CodeSystemFailure
	ErrorTimeout          = errcontract.CodeTimeout
	ErrorCancelled        = errcontract.CodeCancelled
	ErrorNotFound         = errcontract.CodeNotFound
	ErrorConflict         = errcontract.CodeConflict
	ErrorDataUnavailable  = errcontract.CodeDataUnavailable
)

// ToolError is safe to return to an Agent client. Internal database errors,
// SQL text and cross-tenant existence details must stay in server logs.
type ToolError = errcontract.Error
