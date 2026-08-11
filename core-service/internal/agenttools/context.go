package agenttools

import (
	"context"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
)

type executionContextKey struct{}
type delegationCredentialKey struct{}

// Principal is resolved by the Core Service from JWT/session/capability
// context. It is not deserialised from ToolCall arguments.
type Principal struct {
	UserID           string
	SubjectType      string
	Role             string
	Permissions      []string
	Scope            access.Scope
	CapabilityGrants []string
	CapabilityActive bool
	AgentMode        string
}

type ExecutionContext struct {
	Principal    Principal
	RunID        string
	TraceID      string
	SkillID      string
	SkillVersion string
}

func WithExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	// Preserve the request scope when middleware has already installed one;
	// otherwise project the resolved Principal scope into the shared context so
	// existing repository adapters apply the same guard.
	if _, ok := access.ScopeFromContext(ctx); !ok {
		ctx = access.WithScope(ctx, execution.Principal.Scope)
	}
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func ExecutionContextFromContext(ctx context.Context) (ExecutionContext, bool) {
	execution, ok := ctx.Value(executionContextKey{}).(ExecutionContext)
	return execution, ok
}

func RequireExecutionContext(ctx context.Context) (ExecutionContext, error) {
	execution, ok := ExecutionContextFromContext(ctx)
	if !ok || strings.TrimSpace(execution.Principal.UserID) == "" {
		return ExecutionContext{}, ErrExecutionContextRequired
	}
	return execution, nil
}

// WithDelegationCredential carries a server-resolved downstream credential
// (for example, the AI Service Authorization header) without putting it in a
// ToolCall. It is intentionally excluded from ToolExecutionAudit.
func WithDelegationCredential(ctx context.Context, credential string) context.Context {
	return context.WithValue(ctx, delegationCredentialKey{}, credential)
}

func DelegationCredentialFromContext(ctx context.Context) string {
	credential, _ := ctx.Value(delegationCredentialKey{}).(string)
	return credential
}

func (p Principal) HasPermission(required Permission) bool {
	resource := strings.ToLower(strings.TrimSpace(required.Resource))
	action := strings.ToLower(strings.TrimSpace(required.Action))
	if resource == "" || action == "" {
		return false
	}
	for _, permission := range p.Permissions {
		permission = strings.ToLower(strings.TrimSpace(permission))
		if permission == "*:*" || permission == resource+":*" || permission == resource+":"+action {
			return true
		}
	}
	return false
}

func (p Principal) HasCapability(toolName string) bool {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	for _, grant := range p.CapabilityGrants {
		grant = strings.ToLower(strings.TrimSpace(grant))
		if grant == "*" || grant == toolName || grant == "tool:"+toolName {
			return true
		}
	}
	return false
}
