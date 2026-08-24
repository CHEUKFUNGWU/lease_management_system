// Package hooks is the governance middleware chain (W2): the six ordered
// before-hooks and three after-hooks that every tool call crosses. Each hook
// is a tiny constructor taking narrow dependencies; Governance assembles them
// in the fixed order of the Agent Core design §6. The chain itself is the
// deliverable — a new control must have exactly one mounting point.
package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// TenantScope is the first and cheapest gate: an authenticated execution
// context must exist. CapabilityCheck below deliberately does not re-require
// the context, so this hook remains the unique owner of the check (ACORE-2:
// removing it must be detectable).
func TenantScope() agentcore.BeforeToolCall {
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		if _, err := agenttools.RequireExecutionContext(ctx); err != nil {
			return agentcore.BeforeResult{}, err
		}
		return agentcore.BeforeResult{}, nil
	}
}

// CapabilityCheck replicates policy.Evaluate's level, capability, permission
// and dry-run decisions, returning the same sentinel errors so existing error
// mapping stays intact. It trusts that TenantScope has already run.
func CapabilityCheck(policy agenttools.Policy) agentcore.BeforeToolCall {
	if policy.AllowedLevels == nil {
		policy = agenttools.DefaultPolicy()
	}
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		desc, call := bc.Descriptor, bc.Call
		if desc.Name != call.ToolName || desc.Version != call.ToolVersion {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: tool descriptor version mismatch", agenttools.ErrInvalidToolCall)
		}
		if !policy.AllowedLevels[desc.Level] {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: level %s is disabled", agenttools.ErrToolCapabilityRequired, desc.Level)
		}
		if bc.Principal.CapabilityActive && !bc.Principal.HasCapability(desc.Name) {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: %s", agenttools.ErrToolCapabilityRequired, desc.Name)
		}
		if desc.Level == agenttools.LevelCommand && (!policy.AllowCommand || !bc.Principal.HasCapability(desc.Name)) {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: %s", agenttools.ErrToolCapabilityRequired, desc.Name)
		}
		for _, permission := range desc.Permissions {
			if !bc.Principal.HasPermission(permission) {
				return agentcore.BeforeResult{}, fmt.Errorf("%w: %s:%s", agenttools.ErrToolNotPermitted, permission.Resource, permission.Action)
			}
		}
		if call.DryRun && !desc.SupportsDryRun {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: tool does not support dry_run", agenttools.ErrInvalidToolCall)
		}
		return agentcore.BeforeResult{}, nil
	}
}

// MeasureResolver tells the ProtectedMeasure hook which measurement semantics
// a tool produces and which tools are certified engines. No resolver means no
// protected-measure declarations anywhere (pass-through).
type MeasureResolver interface {
	MeasuresFor(toolName string) []string
	IsCertified(toolName string) bool
}

// ProtectedMeasure applies the request-time routing rule (design §4.2 step 2):
// a call whose measures intersect the protected list may only proceed through
// a certified engine tool; anything else is blocked with a helpful reason —
// never routed to an exploratory path.
func ProtectedMeasure(resolver MeasureResolver) agentcore.BeforeToolCall {
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		if resolver == nil {
			return agentcore.BeforeResult{}, nil
		}
		measures := resolver.MeasuresFor(bc.Call.ToolName)
		if len(measures) == 0 {
			return agentcore.BeforeResult{}, nil
		}
		decision := agenttools.RouteMeasures(measures, resolver.IsCertified(bc.Call.ToolName))
		if decision.Tier == "Reject" {
			// Domain-policy refusal (ADR-0025): business_failure keeps the authored guidance.
			return agentcore.BeforeResult{Block: true, Reason: decision.RejectReason, Code: agenttools.ErrorBusinessFailure}, nil
		}
		return agentcore.BeforeResult{}, nil
	}
}

// Budget counts tool calls per chain instance. A nil budget disables the
// guard.
type Budget struct {
	MaxToolCalls int
	count        int
}

func (b *Budget) used() bool      { return b != nil && b.MaxToolCalls > 0 }
func (b *Budget) exhausted() bool { return b.used() && b.count >= b.MaxToolCalls }
func (b *Budget) take() {
	if b.used() {
		b.count++
	}
}

// BudgetGuard blocks once the call budget is exhausted. It runs before the
// request consumes real resources (design §6 order #4).
func BudgetGuard(b *Budget) agentcore.BeforeToolCall {
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		if b == nil || !b.used() {
			return agentcore.BeforeResult{}, nil
		}
		if b.exhausted() {
			// rate_limited per M6.3 precedent; NOT retryable — this budget only grows,
			// so agentrunner auto-retry would burn calls guaranteed to fail.
			return agentcore.BeforeResult{Block: true, Reason: fmt.Sprintf("tool call budget exhausted (%d)", b.MaxToolCalls), Code: agenttools.ErrorRateLimited}, nil
		}
		b.take()
		return agentcore.BeforeResult{}, nil
	}
}

// ReplayStore is the optional idempotency replay lookup. A hit short-circuits
// the call with the stored result.
type ReplayStore interface {
	Lookup(ctx context.Context, key string) (*agenttools.ToolResult, bool)
}

// IdempotencyGuard enforces the write-key requirement (parity with policy)
// and replays a previously stored result when the store has one.
func IdempotencyGuard(replay ReplayStore) agentcore.BeforeToolCall {
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		if bc.Descriptor.Level != agenttools.LevelRead && strings.TrimSpace(bc.Call.IdempotencyKey) == "" {
			return agentcore.BeforeResult{}, fmt.Errorf("%w: write-capable tool requires idempotency_key", agenttools.ErrInvalidToolCall)
		}
		if replay != nil && strings.TrimSpace(bc.Call.IdempotencyKey) != "" {
			if stored, ok := replay.Lookup(ctx, bc.Call.IdempotencyKey); ok && stored != nil {
				return agentcore.BeforeResult{Short: stored}, nil
			}
		}
		return agentcore.BeforeResult{}, nil
	}
}

// ReviewGate short-circuits command-level calls that require review. Draft
// calls are deliberately passed through: drafts must be produced first and
// only then marked needs_review (design decision D-B1; the post-execution
// forcing stays with the runtime until W6).
func ReviewGate(requireDraftReview bool) agentcore.BeforeToolCall {
	return func(ctx context.Context, bc agentcore.BeforeContext) (agentcore.BeforeResult, error) {
		desc := bc.Descriptor
		requires := agenttools.RequiresReviewDecision(desc, agenttools.Policy{RequireDraftReview: requireDraftReview})
		if !requires || desc.Level != agenttools.LevelCommand {
			return agentcore.BeforeResult{}, nil
		}
		reasons := append([]string(nil), desc.Review.Reasons...)
		if len(reasons) == 0 {
			reasons = []string{"tool policy requires human review"}
		}
		short := &agenttools.ToolResult{
			CallID: bc.Call.CallID,
			Status: agenttools.StatusNeedsReview,
			Review: agenttools.ReviewResult{
				Required: true,
				Reasons:  reasons,
				Actions:  append([]string(nil), desc.Review.ConfirmAction),
			},
		}
		return agentcore.BeforeResult{Short: short}, nil
	}
}
