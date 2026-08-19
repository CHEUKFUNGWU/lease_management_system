package agentcore

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// BeforeContext is what a before-hook may inspect before a tool executes.
// The call has already passed parameter validation at this point.
type BeforeContext struct {
	Call       agenttools.ToolCall
	Descriptor agenttools.ToolDescriptor
	State      *State
	Principal  agenttools.Principal
}

// BeforeResult lets a hook block the call, rewrite its arguments, or replace
// execution entirely with a short-circuit result (the Review Gate shape).
type BeforeResult struct {
	Block  bool
	Reason string          // surfaced in audit and to the user
	Args   json.RawMessage // non-empty rewrites the call arguments
	Short  *agenttools.ToolResult
}

// BeforeToolCall runs before a tool executes, after parameter validation.
type BeforeToolCall func(ctx context.Context, bc BeforeContext) (BeforeResult, error)

// AfterContext is what an after-hook may inspect after a tool finished.
// Result and Err are the raw outcome; both may be nil.
type AfterContext struct {
	Call       agenttools.ToolCall
	Descriptor agenttools.ToolDescriptor
	Result     *agenttools.ToolResult
	Err        error
	State      *State
	Principal  agenttools.Principal
}

// AfterResult is reserved for future after-hook mutations.
type AfterResult struct{}

// AfterToolCall runs after a tool executed, before ToolExecutionEnd is emitted.
type AfterToolCall func(ctx context.Context, ac AfterContext) (AfterResult, error)

// ChainBefore runs hooks in order and stops at the first Block or Short. The
// asymmetry with ChainAfter is deliberate: blocking must happen as early as
// possible, while audit-style after-hooks must all run.
func ChainBefore(hooks ...BeforeToolCall) BeforeToolCall {
	return func(ctx context.Context, bc BeforeContext) (BeforeResult, error) {
		for _, h := range hooks {
			if h == nil {
				continue
			}
			res, err := h(ctx, bc)
			if err != nil {
				return res, err
			}
			if res.Block || res.Short != nil {
				return res, nil
			}
			if len(res.Args) > 0 {
				bc.Call.Arguments = res.Args
			}
		}
		return BeforeResult{}, nil
	}
}

// ChainAfter runs every hook and aggregates their errors with errors.Join.
func ChainAfter(hooks ...AfterToolCall) AfterToolCall {
	return func(ctx context.Context, ac AfterContext) (AfterResult, error) {
		var errs []error
		for _, h := range hooks {
			if h == nil {
				continue
			}
			if _, err := h(ctx, ac); err != nil {
				errs = append(errs, err)
			}
		}
		return AfterResult{}, errors.Join(errs...)
	}
}
