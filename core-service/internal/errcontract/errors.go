// Package errcontract is the single error vocabulary for the HTTP seam and
// the Agent Tool seam. Both adapters speak the same codes; only the envelope
// differs (HTTP: {"code","error","details"}, Agent Tool: {"code","message",
// "retryable","details"}). Callers branch on the code, never on the
// human-readable text.
package errcontract

import (
	"errors"
	"strings"
)

// Code is stable across the Web Runtime, CLI Adapter and future Agent Runner.
type Code string

const (
	CodeInvalidArguments Code = "invalid_arguments"
	CodeUnauthenticated  Code = "unauthenticated"
	CodePermissionDenied Code = "permission_denied"
	CodeScopeDenied      Code = "scope_denied"
	CodeReviewRequired   Code = "review_required"
	CodeCapabilityDenied Code = "capability_denied"
	CodeBusinessFailure  Code = "business_failure"
	CodeSystemFailure    Code = "system_failure"
	CodeTimeout          Code = "timeout"
	CodeCancelled        Code = "cancelled"
	CodeNotFound         Code = "not_found"
	CodeConflict         Code = "conflict"
	CodeDataUnavailable  Code = "data_unavailable"
)

// Error is safe to return to a client. Internal database errors, SQL text,
// stack traces and internal file paths must never reach the Message field —
// adapters route unknown errors through SafeMessage, which hides them behind
// a generic fallback.
type Error struct {
	Code      Code           `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func (e *Error) WithRetryable(retryable bool) *Error {
	e.Retryable = retryable
	return e
}

func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// CodeOf extracts the contract code carried by err or any error it wraps.
// Errors that carry no contract code are system_failure by default, so a
// caller that forgets to classify can never invent a code.
func CodeOf(err error) Code {
	var contractErr *Error
	if errors.As(err, &contractErr) && contractErr.Code != "" {
		return contractErr.Code
	}
	return CodeSystemFailure
}

// SafeMessage returns the client-safe message of a coded error. For anything
// that is not a contract error — raw database errors, wrapped internals —
// it returns a generic fallback so SQL fragments and internal paths never
// reach the response body.
func SafeMessage(err error) string {
	var contractErr *Error
	if errors.As(err, &contractErr) && strings.TrimSpace(contractErr.Message) != "" {
		return contractErr.Message
	}
	return "internal server error"
}
