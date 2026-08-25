package agenttools

import (
	"context"
	"fmt"
	"strings"
)

// Policy is deliberately explicit. The default policy permits facts and
// reviewable drafts, while command-level state transitions require a separate
// capability grant and an opt-in policy.
type Policy struct {
	AllowedLevels      map[ToolLevel]bool
	AllowCommand       bool
	RequireDraftReview bool
}

func DefaultPolicy() Policy {
	return Policy{
		AllowedLevels: map[ToolLevel]bool{
			LevelRead:  true,
			LevelDraft: true,
		},
		RequireDraftReview: true,
	}
}

type PolicyDecision struct {
	Allowed        bool
	RequiresReview bool
	Reasons        []string
}

func Evaluate(ctx context.Context, descriptor ToolDescriptor, call ToolCall, policy Policy) (PolicyDecision, error) {
	if err := descriptor.Validate(); err != nil {
		return PolicyDecision{}, err
	}
	if err := call.Validate(); err != nil {
		return PolicyDecision{}, err
	}
	execution, err := RequireExecutionContext(ctx)
	if err != nil {
		return PolicyDecision{}, err
	}
	if descriptor.Name != call.ToolName || descriptor.Version != call.ToolVersion {
		return PolicyDecision{}, fmt.Errorf("%w: tool descriptor version mismatch", ErrInvalidToolCall)
	}
	if !policy.AllowedLevels[descriptor.Level] {
		return PolicyDecision{}, fmt.Errorf("%w: level %s is disabled", ErrToolCapabilityRequired, descriptor.Level)
	}
	if execution.Principal.CapabilityActive && !execution.Principal.HasCapability(descriptor.Name) {
		return PolicyDecision{}, fmt.Errorf("%w: %s", ErrToolCapabilityRequired, descriptor.Name)
	}
	if descriptor.Level == LevelCommand && (!policy.AllowCommand || !execution.Principal.HasCapability(descriptor.Name)) {
		return PolicyDecision{}, fmt.Errorf("%w: %s", ErrToolCapabilityRequired, descriptor.Name)
	}
	for _, permission := range descriptor.Permissions {
		if !execution.Principal.HasPermission(permission) {
			return PolicyDecision{}, fmt.Errorf("%w: %s:%s", ErrToolNotPermitted, permission.Resource, permission.Action)
		}
	}
	if call.DryRun && !descriptor.SupportsDryRun {
		return PolicyDecision{}, fmt.Errorf("%w: tool does not support dry_run", ErrInvalidToolCall)
	}
	if descriptor.Level != LevelRead && strings.TrimSpace(call.IdempotencyKey) == "" {
		return PolicyDecision{}, fmt.Errorf("%w: write-capable tool requires idempotency_key", ErrInvalidToolCall)
	}

	decision := PolicyDecision{Allowed: true}
	if RequiresReviewDecision(descriptor, policy) {
		decision.RequiresReview = true
		decision.Reasons = reviewReasons(descriptor)
	}
	return decision, nil
}

// RequiresReviewDecision is the review half of Evaluate, factored out so the
// governance chain (governance.ReviewGate) and the runtime's
// post-execution forcing share the same rule.
func RequiresReviewDecision(descriptor ToolDescriptor, policy Policy) bool {
	return descriptor.Review.Required || (descriptor.Level == LevelDraft && policy.RequireDraftReview)
}

// ReviewDecision returns the review decision for a descriptor without the
// gate checks — used by the runtime after the governance chain has passed.
func ReviewDecision(descriptor ToolDescriptor, policy Policy) PolicyDecision {
	if !RequiresReviewDecision(descriptor, policy) {
		return PolicyDecision{Allowed: true}
	}
	return PolicyDecision{Allowed: true, RequiresReview: true, Reasons: reviewReasons(descriptor)}
}

func reviewReasons(descriptor ToolDescriptor) []string {
	reasons := append([]string(nil), descriptor.Review.Reasons...)
	if len(reasons) == 0 {
		reasons = []string{"tool policy requires human review"}
	}
	return reasons
}
