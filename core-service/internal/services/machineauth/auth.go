package machineauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid client_id or client_secret")
	ErrCredentialRevoked  = errors.New("machine credential has been revoked")
	ErrCredentialExpired  = errors.New("machine credential has expired")
	ErrInsufficientScope  = errors.New("insufficient scope for requested operation")
)

type Credential struct {
	ID            string     `json:"id"`
	LegalEntityID string     `json:"legal_entity_id"`
	Name          string     `json:"name"`
	ClientID      string     `json:"client_id"`
	SecretHash    string     `json:"-"`
	Scopes        []string   `json:"scopes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// GenerateCredentials generates a cryptographically secure client_id and client_secret.
func GenerateCredentials(prefix string) (clientID string, clientSecret string, secretHash string, err error) {
	if prefix == "" {
		prefix = "mch"
	}
	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", "", fmt.Errorf("generate id: %w", err)
	}
	clientID = fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(idBytes))

	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", "", fmt.Errorf("generate secret: %w", err)
	}
	clientSecret = fmt.Sprintf("sec_%s", hex.EncodeToString(secretBytes))
	secretHash = HashSecret(clientSecret)
	return clientID, clientSecret, secretHash, nil
}

// HashSecret computes SHA-256 hex digest of secret.
func HashSecret(rawSecret string) string {
	h := sha256.Sum256([]byte(rawSecret))
	return hex.EncodeToString(h[:])
}

// VerifySecret compares rawSecret with expected hash in constant time.
func VerifySecret(rawSecret, expectedHash string) bool {
	computed := HashSecret(rawSecret)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(expectedHash)) == 1
}

// Verify evaluates validity of a credential against required scope and expiry.
func Verify(c *Credential, rawSecret string, requiredScope string, now time.Time) error {
	if c == nil || !VerifySecret(rawSecret, c.SecretHash) {
		return ErrInvalidCredentials
	}
	if c.RevokedAt != nil && !c.RevokedAt.After(now) {
		return ErrCredentialRevoked
	}
	if c.ExpiresAt != nil && c.ExpiresAt.Before(now) {
		return ErrCredentialExpired
	}
	if requiredScope != "" && !HasScope(c.Scopes, requiredScope) {
		return ErrInsufficientScope
	}
	return nil
}

// HasScope checks if requiredScope is present in scopes or wildcard is held.
func HasScope(scopes []string, requiredScope string) bool {
	for _, s := range scopes {
		if s == "*" || s == requiredScope {
			return true
		}
		// Prefix matching e.g. "operating_facts:*" matches "operating_facts:write"
		if strings.HasSuffix(s, ":*") {
			prefix := strings.TrimSuffix(s, ":*")
			if strings.HasPrefix(requiredScope, prefix+":") {
				return true
			}
		}
	}
	return false
}
