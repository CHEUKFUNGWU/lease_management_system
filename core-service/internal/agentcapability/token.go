package agentcapability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
)

const TokenType = "agent_capability"

var (
	ErrInvalidToken      = errors.New("invalid agent capability token")
	ErrInvalidIssuer     = errors.New("invalid agent capability issuer")
	ErrInvalidRequest    = errors.New("invalid agent capability request")
	ErrRunBindingMissing = errors.New("agent capability run binding is required")
)

// Claims are intentionally narrower than a login token. They describe the
// exact Agent run, tools and data scope for which the caller was delegated.
// The normal JWT remains the source of user authentication.
type Claims struct {
	jwt.RegisteredClaims
	TokenType     string   `json:"token_type"`
	UserID        string   `json:"user_id"`
	SessionID     string   `json:"session_id,omitempty"`
	RunID         string   `json:"run_id"`
	SkillID       string   `json:"skill_id,omitempty"`
	SkillVersion  string   `json:"skill_version,omitempty"`
	LegalEntityID string   `json:"legal_entity_id,omitempty"`
	Global        bool     `json:"global,omitempty"`
	StoreIDs      []string `json:"store_ids,omitempty"`
	Regions       []string `json:"regions,omitempty"`
	Brands        []string `json:"brands,omitempty"`
	Permissions   []string `json:"permissions"`
	AllowedTools  []string `json:"allowed_tools"`
}

type IssueRequest struct {
	UserID       string
	SessionID    string
	RunID        string
	SkillID      string
	SkillVersion string
	Scope        access.Scope
	Permissions  []string
	AllowedTools []string
	TTL          time.Duration
}

type Issuer struct {
	secret   []byte
	audience string
	ttl      time.Duration
	now      func() time.Time
	state    *revocationState
	store    RevocationStore
}

// RevocationStore is the optional cross-instance lifecycle port. It stores
// only token identifiers, owner/run metadata and revocation timestamps; raw
// capability JWTs never cross this boundary.
type RevocationStore interface {
	Register(context.Context, string, string, string, time.Time) error
	RevokeToken(context.Context, string, string, time.Time) error
	RevokeRun(context.Context, string, string, time.Time) error
	IsRevoked(context.Context, string, string, time.Time) (bool, error)
	TokenOwner(context.Context, string) (string, error)
	RunOwner(context.Context, string) (string, error)
}

type revocationState struct {
	mu        sync.RWMutex
	byJTI     map[string]time.Time
	owners    map[string]string
	runs      map[string]time.Time
	runOwners map[string]string
}

func NewIssuer(secret, audience string, ttl time.Duration) (*Issuer, error) {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(audience) == "" {
		return nil, ErrInvalidIssuer
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl must be positive", ErrInvalidIssuer)
	}
	return &Issuer{
		secret:   []byte(secret),
		audience: strings.TrimSpace(audience),
		ttl:      ttl,
		now:      time.Now,
		state:    &revocationState{byJTI: make(map[string]time.Time), owners: make(map[string]string), runs: make(map[string]time.Time), runOwners: make(map[string]string)},
	}, nil
}

func (i *Issuer) WithClock(now func() time.Time) *Issuer {
	if i == nil || now == nil {
		return i
	}
	clone := *i
	clone.now = now
	return &clone
}

func (i *Issuer) WithRevocationStore(store RevocationStore) *Issuer {
	if i == nil {
		return i
	}
	clone := *i
	clone.store = store
	return &clone
}

func (i *Issuer) TTL() time.Duration {
	if i == nil {
		return 0
	}
	return i.ttl
}

func (i *Issuer) Issue(request IssueRequest) (string, Claims, error) {
	if i == nil || len(i.secret) == 0 {
		return "", Claims{}, ErrInvalidIssuer
	}
	userID := strings.TrimSpace(request.UserID)
	runID := strings.TrimSpace(request.RunID)
	if userID == "" {
		return "", Claims{}, fmt.Errorf("%w: user_id is required", ErrInvalidRequest)
	}
	if runID == "" {
		return "", Claims{}, ErrRunBindingMissing
	}
	allowedTools, err := normalizeNonEmpty(request.AllowedTools)
	if err != nil {
		return "", Claims{}, fmt.Errorf("%w: allowed_tools: %v", ErrInvalidRequest, err)
	}
	permissions := normalize(request.Permissions)
	if len(permissions) == 0 {
		return "", Claims{}, fmt.Errorf("%w: permissions are required", ErrInvalidRequest)
	}
	ttl := request.TTL
	if ttl <= 0 {
		ttl = i.ttl
	}
	if ttl > i.ttl {
		return "", Claims{}, fmt.Errorf("%w: ttl exceeds issuer maximum", ErrInvalidRequest)
	}
	now := time.Now
	if i.now != nil {
		now = i.now
	}
	issuedAt := now().UTC()
	expiresAt := issuedAt.Add(ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.audience,
			Subject:   userID,
			ID:        uuid.NewString(),
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
		},
		TokenType:     TokenType,
		UserID:        userID,
		SessionID:     strings.TrimSpace(request.SessionID),
		RunID:         runID,
		SkillID:       strings.TrimSpace(request.SkillID),
		SkillVersion:  strings.TrimSpace(request.SkillVersion),
		LegalEntityID: strings.TrimSpace(request.Scope.LegalEntityID),
		Global:        request.Scope.Global,
		StoreIDs:      normalize(request.Scope.StoreIDs),
		Regions:       normalize(request.Scope.Regions),
		Brands:        normalize(request.Scope.Brands),
		Permissions:   permissions,
		AllowedTools:  allowedTools,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", Claims{}, fmt.Errorf("sign capability token: %w", err)
	}
	if i.store != nil {
		if err := i.store.Register(context.Background(), claims.ID, claims.RunID, claims.UserID, expiresAt); err != nil {
			return "", Claims{}, fmt.Errorf("register capability token: %w", err)
		}
	}
	i.state.mu.Lock()
	i.state.owners[claims.ID] = claims.UserID
	i.state.runOwners[claims.RunID] = claims.UserID
	i.state.mu.Unlock()
	return signed, claims, nil
}

func (i *Issuer) Parse(raw string) (Claims, error) {
	if i == nil || len(i.secret) == 0 {
		return Claims{}, ErrInvalidIssuer
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	parserOptions := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithAudience(i.audience),
		jwt.WithIssuer(i.audience),
	}
	if i.now != nil {
		parserOptions = append(parserOptions, jwt.WithTimeFunc(i.now))
	}
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (interface{}, error) {
		return i.secret, nil
	}, parserOptions...)
	if err != nil || token == nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}
	if claims.TokenType != TokenType || claims.UserID == "" || claims.Subject != claims.UserID || claims.RunID == "" || len(claims.AllowedTools) == 0 || len(claims.Permissions) == 0 {
		return Claims{}, ErrInvalidToken
	}
	if i.isRevoked(claims) {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

// RevokeToken invalidates one capability token by its JWT ID until its
// original expiration. The raw token is never persisted.
func (i *Issuer) RevokeToken(tokenID string) error {
	if i == nil || i.state == nil || strings.TrimSpace(tokenID) == "" {
		return ErrInvalidToken
	}
	i.state.mu.Lock()
	i.state.byJTI[strings.TrimSpace(tokenID)] = i.currentTime()
	owner := i.state.owners[strings.TrimSpace(tokenID)]
	i.state.mu.Unlock()
	if i.store != nil {
		if err := i.store.RevokeToken(context.Background(), strings.TrimSpace(tokenID), owner, i.currentTime().Add(i.ttl)); err != nil {
			return fmt.Errorf("persist token revocation: %w", err)
		}
	}
	return nil
}

// RevokeRun invalidates all capabilities bound to a run. This is the server
// side hook for run cancellation/completion and does not require token storage.
func (i *Issuer) RevokeRun(runID string) error {
	if i == nil || i.state == nil || strings.TrimSpace(runID) == "" {
		return ErrRunBindingMissing
	}
	i.state.mu.Lock()
	i.state.runs[strings.TrimSpace(runID)] = i.currentTime()
	owner := i.state.runOwners[strings.TrimSpace(runID)]
	i.state.mu.Unlock()
	if i.store != nil {
		if err := i.store.RevokeRun(context.Background(), strings.TrimSpace(runID), owner, i.currentTime().Add(i.ttl)); err != nil {
			return fmt.Errorf("persist run revocation: %w", err)
		}
	}
	return nil
}

func (i *Issuer) RevokeTokenForUser(tokenID, userID string) error {
	if i == nil || i.state == nil || strings.TrimSpace(tokenID) == "" || strings.TrimSpace(userID) == "" {
		return ErrInvalidToken
	}
	i.state.mu.RLock()
	owner := i.state.owners[strings.TrimSpace(tokenID)]
	i.state.mu.RUnlock()
	if owner == "" && i.store != nil {
		storedOwner, err := i.store.TokenOwner(context.Background(), strings.TrimSpace(tokenID))
		if err != nil {
			return fmt.Errorf("load token owner: %w", err)
		}
		owner = storedOwner
	}
	if owner == "" || owner != strings.TrimSpace(userID) {
		return ErrInvalidToken
	}
	return i.RevokeToken(tokenID)
}

func (i *Issuer) RevokeRunForUser(runID, userID string) error {
	if i == nil || i.state == nil || strings.TrimSpace(runID) == "" || strings.TrimSpace(userID) == "" {
		return ErrRunBindingMissing
	}
	i.state.mu.RLock()
	owner := i.state.runOwners[strings.TrimSpace(runID)]
	i.state.mu.RUnlock()
	if owner == "" && i.store != nil {
		storedOwner, err := i.store.RunOwner(context.Background(), strings.TrimSpace(runID))
		if err != nil {
			return fmt.Errorf("load run owner: %w", err)
		}
		owner = storedOwner
	}
	if owner == "" || owner != strings.TrimSpace(userID) {
		return ErrRunBindingMissing
	}
	return i.RevokeRun(runID)
}

func (i *Issuer) isRevoked(claims Claims) bool {
	if i == nil || i.state == nil {
		return false
	}
	now := i.currentTime()
	i.state.mu.RLock()
	defer i.state.mu.RUnlock()
	if issued, ok := i.state.runs[claims.RunID]; ok && !now.Before(issued) {
		return true
	}
	if claims.ID != "" {
		if issued, ok := i.state.byJTI[claims.ID]; ok && !now.Before(issued) {
			return true
		}
	}
	if i.store != nil {
		revoked, err := i.store.IsRevoked(context.Background(), claims.ID, claims.RunID, now)
		if err != nil {
			// A failed revocation lookup fails closed. A capability must never
			// remain usable merely because the persistence layer is unavailable.
			return true
		}
		if revoked {
			return true
		}
	}
	return false
}

func (i *Issuer) currentTime() time.Time {
	if i != nil && i.now != nil {
		return i.now().UTC()
	}
	return time.Now().UTC()
}

func (c Claims) Scope() access.Scope {
	return access.Scope{
		Global:        c.Global,
		LegalEntityID: c.LegalEntityID,
		StoreIDs:      append([]string(nil), c.StoreIDs...),
		Regions:       append([]string(nil), c.Regions...),
		Brands:        append([]string(nil), c.Brands...),
	}
}

func (c Claims) AllowsTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, allowed := range c.AllowedTools {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "*" || allowed == name {
			return true
		}
	}
	return false
}

func normalize(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeNonEmpty(values []string) ([]string, error) {
	result := normalize(values)
	if len(result) == 0 {
		return nil, errors.New("at least one tool is required")
	}
	return result, nil
}
