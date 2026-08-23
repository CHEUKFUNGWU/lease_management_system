package gateway

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

// ── fakes ───────────────────────────────────────────────────────────────────

type fakeBindings struct {
	userID  string
	findErr error
	calls   int
}

func (f *fakeBindings) FindInternalUserID(_ context.Context, channel, externalUserID string) (string, error) {
	f.calls++
	if f.findErr != nil {
		return "", f.findErr
	}
	return f.userID, nil
}

type fakeUsers struct {
	user *repository.User
}

func (f *fakeUsers) GetByID(_ context.Context, _ string) (*repository.User, error) {
	return f.user, nil
}

type fakeRoles struct{}

func (fakeRoles) GetUserRoleCodes(_ context.Context, _ string) ([]string, error) {
	return []string{"editor"}, nil
}

func (fakeRoles) GetUserPermissions(_ context.Context, _ string) ([]*repository.Permission, error) {
	return []*repository.Permission{
		{Resource: "contracts", Action: "read"},
		{Resource: "*", Action: "*"},
	}, nil
}

func (fakeRoles) GetUserDataScopes(_ context.Context, _ string) ([]*repository.DataScope, error) {
	return []*repository.DataScope{
		{Dimension: "store", TargetID: "store-1"},
		{Dimension: "region", TargetID: "east"},
	}, nil
}

var testEntity = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func activeUser() *repository.User {
	entity := testEntity
	return &repository.User{ID: "u-1", Role: "editor", LegalEntityID: &entity, IsActive: true}
}

// ── happy path ──────────────────────────────────────────────────────────────

func TestResolveReturnsCompletePrincipal(t *testing.T) {
	resolver := NewIdentityResolver(&fakeBindings{userID: "u-1"}, &fakeUsers{user: activeUser()}, fakeRoles{})

	principal, err := resolver.Resolve(context.Background(), ChannelRef{Channel: "Feishu", ExternalUserID: "ou-123"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if principal.UserID != "u-1" || principal.Role != "editor" || principal.SubjectType != "channel_identity" {
		t.Fatalf("principal identity incomplete: %+v", principal)
	}
	if principal.AgentMode != "assist" {
		t.Fatalf("channels run assist mode, got %q", principal.AgentMode)
	}
	// permissions are the normalized ones from the shared loader
	found := false
	for _, permission := range principal.Permissions {
		if permission == "contracts:read" {
			found = true
		}
	}
	if !found {
		t.Fatalf("normalized permission missing: %+v", principal.Permissions)
	}
	// scope is exactly what the shared builder produces from the same inputs —
	// same function as DataScopeMiddleware runs for JWT requests.
	want := middleware.BuildAccessScope([]string{"contracts:read", "*:*"},
		map[string][]string{"store": {"store-1"}, "region": {"east"}}, testEntity)
	if principal.Scope.Global != want.Global ||
		principal.Scope.LegalEntityID != want.LegalEntityID ||
		len(principal.Scope.StoreIDs) != len(want.StoreIDs) ||
		len(principal.Scope.Regions) != len(want.Regions) ||
		(len(principal.Scope.StoreIDs) > 0 && principal.Scope.StoreIDs[0] != want.StoreIDs[0]) {
		t.Fatalf("scope diverges from the JWT builder: %+v vs %+v", principal.Scope, want)
	}
}

// ── D-B14: unbound rejects, no fallback anywhere ────────────────────────────

func TestResolveUnboundIsNamedErrorAndNeverYieldsAPrincipal(t *testing.T) {
	bindings := &fakeBindings{findErr: repository.ErrChannelIdentityUnbound}
	resolver := NewIdentityResolver(bindings, &fakeUsers{user: activeUser()}, fakeRoles{})

	for _, ref := range []ChannelRef{
		{Channel: "feishu", ExternalUserID: "unknown-sender"},
		{Channel: "", ExternalUserID: "x"},
		{Channel: "slack", ExternalUserID: "x"},
		{Channel: "feishu", ExternalUserID: ""},
	} {
		principal, err := resolver.Resolve(context.Background(), ref)
		if err == nil || principal.UserID != "" || len(principal.Permissions) > 0 {
			t.Fatalf("ref %+v: expected no principal, got user=%q perms=%v err=%v", ref, principal.UserID, principal.Permissions, err)
		}
		switch {
		case ref.Channel == "" || ref.Channel == "slack" || ref.ExternalUserID == "":
			if !errors.Is(err, ErrInvalidChannelRef) {
				t.Fatalf("ref %+v: want ErrInvalidChannelRef, got %v", ref, err)
			}
		default:
			if !errors.Is(err, ErrUnbound) {
				t.Fatalf("ref %+v: want ErrUnbound, got %v", ref, err)
			}
		}
	}
}

func TestResolveInactiveBoundUserFailsClosed(t *testing.T) {
	inactive := activeUser()
	inactive.IsActive = false
	resolver := NewIdentityResolver(&fakeBindings{userID: "u-1"}, &fakeUsers{user: inactive}, fakeRoles{})

	principal, err := resolver.Resolve(context.Background(), ChannelRef{Channel: "feishu", ExternalUserID: "ou-123"})
	if !errors.Is(err, ErrUnbound) || principal.UserID != "" || len(principal.Permissions) > 0 {
		t.Fatalf("inactive bound user must fail closed: principal=%+v err=%v", principal, err)
	}
}

// ── integration (real DB) ───────────────────────────────────────────────────

func gatewayTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
