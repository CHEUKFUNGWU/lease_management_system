package agentcapability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

func testIssuer(t *testing.T) *Issuer {
	t.Helper()
	issuer, err := NewIssuer("capability-secret", "lease-agent-gateway", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return issuer.WithClock(func() time.Time { return time.Date(2026, 8, 10, 1, 2, 3, 0, time.UTC) })
}

func TestIssueAndParseBindsUserRunToolsAndScope(t *testing.T) {
	issuer := testIssuer(t)
	raw, issued, err := issuer.Issue(IssueRequest{
		UserID: "user-1", SessionID: "session-1", RunID: "run-1",
		Scope:        access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}},
		Permissions:  []string{"contracts:read"},
		AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := issuer.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.UserID != issued.UserID || parsed.RunID != "run-1" || parsed.LegalEntityID != "le-001" {
		t.Fatalf("parsed claims=%+v issued=%+v", parsed, issued)
	}
	if !parsed.AllowsTool("lease.contract.get") || parsed.AllowsTool("lease.event.list") {
		t.Fatalf("unexpected tool grants=%v", parsed.AllowedTools)
	}
	if !parsed.Scope().AllowsContract(access.ContractAttributes{LegalEntityID: "le-001", StoreID: "store-1"}) {
		t.Fatalf("parsed scope does not allow expected contract: %+v", parsed.Scope())
	}
}

func TestParseRejectsTamperedAndExpiredToken(t *testing.T) {
	issuer := testIssuer(t)
	raw, _, err := issuer.Issue(IssueRequest{
		UserID: "user-1", RunID: "run-1", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read"}, AllowedTools: []string{"lease.contract.get"}, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(raw + "tampered"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered error=%v", err)
	}
	expired := issuer.WithClock(func() time.Time { return time.Date(2026, 8, 10, 1, 4, 4, 0, time.UTC) })
	if _, err := expired.Parse(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired error=%v", err)
	}
}

func TestRevokeTokenAndRunInvalidateCapabilitiesWithoutPersistingRawToken(t *testing.T) {
	issuer := testIssuer(t)
	rawToken, claims, err := issuer.Issue(IssueRequest{
		UserID: "user-1", RunID: "run-1", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read"}, AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.RevokeToken(claims.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(rawToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token error=%v", err)
	}

	second, _, err := issuer.Issue(IssueRequest{
		UserID: "user-1", RunID: "run-1", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read"}, AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.RevokeRun("run-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(second); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("run-revoked token error=%v", err)
	}
}

func TestIssueRequiresRunAndTools(t *testing.T) {
	issuer := testIssuer(t)
	base := IssueRequest{UserID: "user-1", RunID: "run-1", Scope: access.Scope{LegalEntityID: "le-001"}, Permissions: []string{"contracts:read"}}
	if _, _, err := issuer.Issue(base); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("missing tools error=%v", err)
	}
	base.AllowedTools = []string{"lease.contract.get"}
	base.RunID = ""
	if _, _, err := issuer.Issue(base); !errors.Is(err, ErrRunBindingMissing) {
		t.Fatalf("missing run error=%v", err)
	}
}

func TestIntersectScopesNarrowsDimensions(t *testing.T) {
	left := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1", "store-2"}}
	right := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-2"}}
	merged, ok := access.IntersectScopes(left, right)
	if !ok || len(merged.StoreIDs) != 1 || merged.StoreIDs[0] != "store-2" {
		t.Fatalf("merged=%+v ok=%v", merged, ok)
	}
	if _, ok := access.IntersectScopes(left, access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-9"}}); ok {
		t.Fatal("expected disjoint scopes to be denied")
	}
}

func TestRevocationStoreAllowsCrossIssuerRunRevokeWithoutRawJWTStorage(t *testing.T) {
	store := newMemoryRevocationStore()
	issuer := testIssuer(t).WithRevocationStore(store)
	raw, _, err := issuer.Issue(IssueRequest{
		UserID: "user-1", RunID: "run-cross-instance", Scope: access.Scope{LegalEntityID: "le-001"},
		Permissions: []string{"contracts:read"}, AllowedTools: []string{"lease.contract.get"},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondIssuer := testIssuer(t).WithRevocationStore(store)
	if err := secondIssuer.RevokeRunForUser("run-cross-instance", "user-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := secondIssuer.Parse(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("cross-issuer revoked token error=%v", err)
	}
}

type memoryRevocationStore struct {
	mu     sync.Mutex
	grants map[string]memoryGrant
	byRun  map[string]string
}

type memoryGrant struct {
	userID    string
	runID     string
	expiresAt time.Time
	revoked   bool
}

func newMemoryRevocationStore() *memoryRevocationStore {
	return &memoryRevocationStore{grants: map[string]memoryGrant{}, byRun: map[string]string{}}
}

func (s *memoryRevocationStore) Register(_ context.Context, tokenID, runID, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants[tokenID] = memoryGrant{userID: userID, runID: runID, expiresAt: expiresAt}
	s.byRun[runID] = userID
	return nil
}

func (s *memoryRevocationStore) RevokeToken(_ context.Context, tokenID, userID string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[tokenID]
	if !ok || (userID != "" && grant.userID != userID) {
		return fmt.Errorf("token not found")
	}
	grant.revoked = true
	s.grants[tokenID] = grant
	return nil
}

func (s *memoryRevocationStore) RevokeRun(_ context.Context, runID, userID string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tokenID, grant := range s.grants {
		if grant.runID == runID && (userID == "" || grant.userID == userID) {
			grant.revoked = true
			s.grants[tokenID] = grant
		}
	}
	return nil
}

func (s *memoryRevocationStore) IsRevoked(_ context.Context, tokenID, runID string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for grantTokenID, grant := range s.grants {
		matches := (tokenID != "" && grantTokenID == tokenID) || (runID != "" && grant.runID == runID)
		if matches && (grant.revoked || !now.Before(grant.expiresAt)) {
			return true, nil
		}
	}
	return false, nil
}

func (s *memoryRevocationStore) TokenOwner(_ context.Context, tokenID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.grants[tokenID].userID, nil
}

func (s *memoryRevocationStore) RunOwner(_ context.Context, runID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byRun[runID], nil
}
