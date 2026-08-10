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
	if descriptor.Review.Required || (descriptor.Level == LevelDraft && policy.RequireDraftReview) {
		decision.RequiresReview = true
		decision.Reasons = append(decision.Reasons, descriptor.Review.Reasons...)
		if len(decision.Reasons) == 0 {
			decision.Reasons = []string{"tool policy requires human review"}
		}
	}
	return decision, nil
}
