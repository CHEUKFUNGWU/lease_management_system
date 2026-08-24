package hooks

import (
	"context"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ExecutionGuard adapts the agentcore before/after chains to the
// agenttools.ExecutionGuard seam, keeping agenttools free of any agentcore
// dependency. The shared State is empty: none of the governance hooks read
// it, but the BeforeContext contract requires one.
type ExecutionGuard struct {
	state  *agentcore.State
	before agentcore.BeforeToolCall
	after  agentcore.AfterToolCall
}

// NewExecutionGuard wraps a before/after chain pair (typically from
// Governance) into the runtime seam.
func NewExecutionGuard(before agentcore.BeforeToolCall, after agentcore.AfterToolCall) *ExecutionGuard {
	return &ExecutionGuard{state: agentcore.NewState(), before: before, after: after}
}

func (g *ExecutionGuard) Before(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal) (agenttools.GuardResult, error) {
	if g == nil || g.before == nil {
		return agenttools.GuardResult{}, nil
	}
	res, err := g.before(ctx, agentcore.BeforeContext{
		Call: call, Descriptor: descriptor, State: g.state, Principal: principal,
	})
	return agenttools.GuardResult{Block: res.Block, Reason: res.Reason, Short: res.Short, Code: res.Code}, err
}

func (g *ExecutionGuard) After(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, result *agenttools.ToolResult, principal agenttools.Principal) error {
	if g == nil || g.after == nil {
		return nil
	}
	_, err := g.after(ctx, agentcore.AfterContext{
		Call: call, Descriptor: descriptor, Result: result, State: g.state, Principal: principal,
	})
	return err
}
