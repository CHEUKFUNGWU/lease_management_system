// Package gateway is the IM channel layer (Ch3). This ticket ships ONLY the
// tenant layer: channel identity → internal user → the exact same scope
// resolver the JWT path uses (ADR-0026 §3). No vendor code, no network, no
// wiring — those arrive in later tickets against an already-sealed boundary.
//
// The package-wide invariant enforced by importguard_test.go:
// nothing under internal/gateway/** constructs access.Scope or mentions
// legal_entity_id. The only way in is Resolve, which returns a complete
// agenttools.Principal — callers have no materials to assemble permissions.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

// ErrUnbound means the channel identity has no binding to an internal user.
// There is no default tenant, no anonymous fallback, and no parameter through
// which one could be supplied — Resolve's signature makes it unrepresentable
// (D-B14).
var ErrUnbound = repository.ErrChannelIdentityUnbound

// ErrInvalidChannelRef rejects malformed refs before any lookup. It is a
// caller bug (empty ids, unsupported channel), distinct from unbound.
var ErrInvalidChannelRef = errors.New("invalid channel identity reference")

// derefOrEmpty mirrors how the JWT chain reads the tenant claim: a NULL
// column is an empty tenant id, which BuildAccessScope turns into a
// zero-entity scope that matches no data (fail-closed downstream).
func derefOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// supportedChannels mirrors the DB CHECK constraint on channel_identity_bindings.
// Adding a channel is a migration + this slice, never a caller-side string.
var supportedChannels = map[string]bool{"feishu": true, "wecom": true}

// ChannelRef identifies one sender on one IM channel.
type ChannelRef struct {
	Channel        string // "feishu" | "wecom"
	ExternalUserID string // open_id / userid
}

// BindingStore reads the binding table.
type BindingStore interface {
	FindInternalUserID(ctx context.Context, channel, externalUserID string) (string, error)
}

// UserReader loads the bound internal user.
type UserReader interface {
	GetByID(ctx context.Context, id string) (*repository.User, error)
}

// IdentityResolver is the tenant layer. Construct it with the same
// repositories the JWT chain uses; Resolve is the single entry point from a
// channel into the system.
type IdentityResolver struct {
	bindings BindingStore
	users    UserReader
	roles    middleware.UserAccessRepo
}

func NewIdentityResolver(bindings BindingStore, users UserReader, roles middleware.UserAccessRepo) *IdentityResolver {
	return &IdentityResolver{bindings: bindings, users: users, roles: roles}
}

func (r *IdentityResolver) validate(ref ChannelRef) error {
	if !supportedChannels[strings.ToLower(strings.TrimSpace(ref.Channel))] {
		return fmt.Errorf("%w: unsupported channel %q", ErrInvalidChannelRef, ref.Channel)
	}
	if strings.TrimSpace(ref.ExternalUserID) == "" {
		return fmt.Errorf("%w: external user id is required", ErrInvalidChannelRef)
	}
	return nil
}

// Resolve maps one channel identity to a complete agenttools.Principal, or
// fails. It never returns a Scope fragment, a bare entity id, or an
// intermediate "user exists" result — and it never falls back to a default
// tenant.
//
// Step 3 delegates to middleware.LoadUserAccess + middleware.BuildAccessScope:
// literally the same functions DataScopeMiddleware runs for JWT requests, not
// a re-implementation.
func (r *IdentityResolver) Resolve(ctx context.Context, ref ChannelRef) (agenttools.Principal, error) {
	if err := r.validate(ref); err != nil {
		return agenttools.Principal{}, err
	}
	userID, err := r.bindings.FindInternalUserID(ctx, ref.Channel, ref.ExternalUserID)
	if err != nil {
		return agenttools.Principal{}, err // includes the named unbound error
	}
	user, err := r.users.GetByID(ctx, userID)
	if err != nil {
		return agenttools.Principal{}, fmt.Errorf("resolve channel identity user: %w", err)
	}
	if user == nil || !user.IsActive {
		// A binding pointing at a missing or deactivated account resolves to
		// nobody: fail closed with the unbound reason (D-B16 keeps this apart
		// from transport errors; it is still an operational condition).
		return agenttools.Principal{}, fmt.Errorf("%w: bound user %s is inactive", ErrUnbound, userID)
	}

	_, permissions, dataScopes, err := middleware.LoadUserAccess(ctx, r.roles, userID)
	if err != nil {
		return agenttools.Principal{}, fmt.Errorf("load channel identity access: %w", err)
	}
	scope := middleware.BuildAccessScope(permissions, dataScopes, derefOrEmpty(user.LegalEntityID))

	return agenttools.Principal{
		UserID:      userID,
		SubjectType: "channel_identity",
		Role:        user.Role,
		Permissions: append([]string(nil), permissions...),
		Scope:       scope,
		AgentMode:   "assist",
	}, nil
}
