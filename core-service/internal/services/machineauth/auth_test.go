package machineauth

import (
	"testing"
	"time"
)

func TestMachineAuth_LifecycleAndScope(t *testing.T) {
	clientID, clientSecret, secretHash, err := GenerateCredentials("pos")
	if err != nil {
		t.Fatalf("unexpected error generating credentials: %v", err)
	}

	if len(clientID) == 0 || len(clientSecret) == 0 || len(secretHash) == 0 {
		t.Fatalf("generated fields cannot be empty")
	}

	cred := &Credential{
		ClientID:   clientID,
		SecretHash: secretHash,
		Scopes:     []string{"operating_facts:write", "store:read"},
	}

	now := time.Now()

	// 1. Correct verification
	if err := Verify(cred, clientSecret, "operating_facts:write", now); err != nil {
		t.Fatalf("expected verify success, got %v", err)
	}

	// 2. Wrong secret
	if err := Verify(cred, "wrong_secret", "operating_facts:write", now); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// 3. Insufficient scope
	if err := Verify(cred, clientSecret, "contracts:delete", now); err != ErrInsufficientScope {
		t.Fatalf("expected ErrInsufficientScope, got %v", err)
	}

	// 4. Expired credential
	expired := now.Add(-1 * time.Hour)
	cred.ExpiresAt = &expired
	if err := Verify(cred, clientSecret, "operating_facts:write", now); err != ErrCredentialExpired {
		t.Fatalf("expected ErrCredentialExpired, got %v", err)
	}

	// 5. Revoked credential
	cred.ExpiresAt = nil
	revoked := now.Add(-5 * time.Minute)
	cred.RevokedAt = &revoked
	if err := Verify(cred, clientSecret, "operating_facts:write", now); err != ErrCredentialRevoked {
		t.Fatalf("expected ErrCredentialRevoked, got %v", err)
	}
}

func TestHasScope_Wildcards(t *testing.T) {
	if !HasScope([]string{"*"}, "anything") {
		t.Fatalf("expected * to match anything")
	}
	if !HasScope([]string{"operating_facts:*"}, "operating_facts:write") {
		t.Fatalf("expected prefix wildcard to match")
	}
	if HasScope([]string{"operating_facts:*"}, "contracts:write") {
		t.Fatalf("expected prefix mismatch to fail")
	}
}
