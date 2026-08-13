package agenttools

import "errors"

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

// ErrorCode is stable across the Web Runtime, CLI Adapter and future Agent
// Runner. Callers should branch on the code, not on the human-readable text.
type ErrorCode string

const (
	ErrorInvalidArguments ErrorCode = "invalid_arguments"
	ErrorUnauthenticated  ErrorCode = "unauthenticated"
	ErrorPermissionDenied ErrorCode = "permission_denied"
	ErrorScopeDenied      ErrorCode = "scope_denied"
	ErrorReviewRequired   ErrorCode = "review_required"
	ErrorCapabilityDenied ErrorCode = "capability_denied"
	ErrorBusinessFailure  ErrorCode = "business_failure"
	ErrorSystemFailure    ErrorCode = "system_failure"
	ErrorTimeout          ErrorCode = "timeout"
	ErrorCancelled        ErrorCode = "cancelled"
	ErrorNotFound         ErrorCode = "not_found"
	ErrorConflict         ErrorCode = "conflict"
	ErrorDataUnavailable  ErrorCode = "data_unavailable"
)

// ToolError is safe to return to an Agent client. Internal database errors,
// SQL text and cross-tenant existence details must stay in server logs.
type ToolError struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}
