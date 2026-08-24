// Package agentcontext holds the C1 runtime's isolation key. AR1 is the
// foundation module: every later consumer (session manager, context assembler,
// memory, compression cache) accepts this key type and never bare strings.
//
// 隔离语义（D-C9/D-C12/D-C20）：一个 ContextKey 标识「这份上下文属于谁」。
// 构造器只接受已解析的 Principal——「有 Key」即证明「走过权限解析」；
// 法人、用户、会话、scope 指纹、数据分类五个维度缺一即构造失败。
package agentcontext

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// classification 词表沿用 store-day 事实信封的三值（底线 2）。
const (
	ClassificationProduction = "production"
	ClassificationSimulated  = "simulated"
	ClassificationMixed      = "mixed"
)

var validClassifications = map[string]bool{
	ClassificationProduction: true,
	ClassificationSimulated:  true,
	ClassificationMixed:      true,
}

// ErrIncompleteKey marks a refused construction: the caller lacked a fully
// resolved principal or passed an unusable identifier.
var ErrIncompleteKey = errors.New("context key requires a resolved principal and complete identifiers")

// ContextKey identifies whose context this is. Every field is unexported:
// a partial or hand-assembled key is not expressible outside this package.
//
// D-C11: the type deliberately has no String() method — an implicit %v would
// scatter tenant and user identifiers into logs. Cache() is explicit and its
// output is only ever a cache key.
type ContextKey struct {
	legalEntityID  string
	userID         string
	sessionID      string
	scopeFinger    string // D-C12: stable fingerprint of the full access.Scope
	classification string // production | simulated | mixed (D-C20)
}

// KeyFrom is the only constructor. It demands a resolved Principal, so holding
// a key proves the permission resolver ran on this identity.
func KeyFrom(p agenttools.Principal, sessionID string, classification string) (ContextKey, error) {
	entityID := strings.TrimSpace(p.Scope.LegalEntityID)
	if entityID == "" {
		// 全局管理员（无法人）同样拒绝：隔离键必须五维俱全，
		// 缺任何一维都意味着这条上下文无法被安全地归属。
		return ContextKey{}, fmt.Errorf("%w: principal scope carries no legal entity", ErrIncompleteKey)
	}
	if strings.TrimSpace(p.UserID) == "" {
		return ContextKey{}, fmt.Errorf("%w: principal carries no user id", ErrIncompleteKey)
	}
	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return ContextKey{}, fmt.Errorf("%w: session id is required", ErrIncompleteKey)
	}
	if !validClassifications[classification] {
		return ContextKey{}, fmt.Errorf("%w: unknown data classification %q", ErrIncompleteKey, classification)
	}
	return ContextKey{
		legalEntityID:  entityID,
		userID:         strings.TrimSpace(p.UserID),
		sessionID:      sid,
		scopeFinger:    fingerprint(p.Scope),
		classification: classification,
	}, nil
}

// Cache returns the cache key. All five dimensions participate in a fixed
// order; AR1-G1's reflection guard fails if a future field stops influencing
// this output.
func (k ContextKey) Cache() string {
	return strings.Join([]string{
		k.classification,
		k.legalEntityID,
		k.userID,
		k.sessionID,
		k.scopeFinger,
	}, "\x1e")
}

// Dimension readers. AR2 Session Manager is the first consumer: it must
// compare a loaded session row's ownership against the key's entity and user
// (cross-tenant reads refuse with scope_denied), and its per-session lease is
// keyed on the three LOCATOR dimensions. The fields themselves stay
// unexported — readers expose facts, they do not enable hand-assembled keys.
func (k ContextKey) LegalEntityID() string    { return k.legalEntityID }
func (k ContextKey) UserID() string           { return k.userID }
func (k ContextKey) SessionID() string        { return k.sessionID }
func (k ContextKey) Classification() string   { return k.classification }
func (k ContextKey) ScopeFingerprint() string { return k.scopeFinger }

// dimension separators are ASCII control characters that cannot appear in
// UUIDs or scope codes, so joins stay unambiguous without escaping.
const (
	dimSeparator = "\x1f" // between elements inside one dimension
	dimBoundary  = "\x1e" // between dimensions
)

// fingerprint renders the full access.Scope as a stable string: fixed field
// order, slices sorted, nil and empty slices rendered identically. A changed
// scope therefore produces a different key with no invalidation logic.
// Every dimension participates — Global, LegalEntityID and the six ID lists —
// so any narrowing or widening changes the key (D-C12).
func fingerprint(s access.Scope) string {
	parts := []string{
		fmt.Sprintf("global=%t", s.Global),
		"legal=" + s.LegalEntityID,
		"stores=" + sortedJoin(s.StoreIDs),
		"regions=" + sortedJoin(s.Regions),
		"brands=" + sortedJoin(s.Brands),
		"plants=" + sortedJoin(s.Plants),
		"lines=" + sortedJoin(s.ProductionLines),
		"equipment=" + sortedJoin(s.EquipmentIDs),
	}
	return strings.Join(parts, dimBoundary)
}

// sortedJoin normalises nil and empty to "" and sorts the rest, so the same
// scope always yields the same fingerprint regardless of slice capacity,
// ordering or nil-vs-empty distinction.
func sortedJoin(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, dimSeparator)
}
