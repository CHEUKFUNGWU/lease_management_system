// Package sessionmanager is the AR2 module: the single owner of AI chat
// session lifecycle (Spec C1 D-C4). Seven ai_chat_* tables exist, but the
// lifecycle logic used to be spread across six files; this package is where
// "what state is this session in, can two messages of one session cross, when
// should an idle entry leave memory" gets ONE answer.
//
// Interface discipline (module design §4):
//
//   - Two methods only: Acquire (get-or-create AND hold the exclusive lease —
//     one operation, so forgetting to lock is structurally impossible) and
//     Close (settle and release, idempotent).
//   - Eviction is NOT on the interface (D-C13): it is the module managing its
//     own memory. Policy arrives via constructor parameters.
//   - Storage goes through the Store port; the module does no IO.
//
// Isolation (D-C9/D-C12/D-C20): both methods take agentcontext.ContextKey and
// nothing else. The key's legal entity and user were produced by the same
// resolver as the JWT — a caller cannot pass them in, so a session cannot be
// acquired under an identity the caller assembled by hand. A loaded row whose
// ownership disagrees with the key refuses with ErrScopeDenied; the rejection
// keeps its reason and is never softened into "not found" (AGENTS.md: 权限拒
// 绝必须保持原因).
package sessionmanager

import (
	"context"
	"errors"
	"time"

	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// ErrNotFound reports that no session row exists for the key's locator
// dimensions. Acquire treats it as create-or-load.
var ErrNotFound = errors.New("ai chat session not found")

// ErrScopeDenied reports that the session exists but belongs to another legal
// entity or user than the key carries. The message always contains
// "scope_denied" so logs and API surfaces keep the real reason instead of a
// softened "no data".
var ErrScopeDenied = errors.New("scope_denied: session belongs to another legal entity or user")

// Session is the stored conversation anchor. It mirrors the columns the
// module owns on ai_chat_sessions; richer run/message projections stay with
// the existing repositories until the wiring tickets move them behind this
// seam.
type Session struct {
	LegalEntityID  string
	UserID         string
	SessionID      string
	Classification string // production | simulated | mixed
	Title          string
	Status         string // active | archived | closed
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Manager owns session lifecycle. See the package comment for the interface
// discipline.
type Manager interface {
	// Acquire returns the session for key (creating it through the Store when
	// absent) while holding the exclusive per-session lease. The returned
	// release MUST be called; a second Acquire of the same key blocks until
	// it is. Forgetting release therefore fails loudly — later requests hang
	// — which is the intended shape: a visible stall beats silent interleaved
	// execution of one conversation's messages.
	Acquire(ctx context.Context, key agentcontext.ContextKey) (*Session, func(), error)

	// Close settles the session (flushing any cached state back to the Store)
	// and drops its in-memory entry. Idempotent: closing an unknown key is a
	// no-op returning nil.
	Close(ctx context.Context, key agentcontext.ContextKey) error
}

// Store is the persistence port. The module does no IO; one Postgres adapter
// plus one test fake are the only implementations, which makes every
// lifecycle branch testable without a database.
//
// Ownership enforcement lives HERE, at the data boundary: Load locates the
// row by the key's session id and must refuse with ErrScopeDenied when the
// row's legal entity or user differs from the key. Manager propagates that
// refusal verbatim.
type Store interface {
	Load(ctx context.Context, key agentcontext.ContextKey) (*Session, error)
	Save(ctx context.Context, key agentcontext.ContextKey, s *Session) error
}

// locator renders the three LOCATOR dimensions of the key. The scope
// fingerprint and classification deliberately do NOT participate:
// D-C20 keeps within-session history stable under the session id ("会话内按
// session_id 取，分类升级不孤立自身历史"), and a lease keyed on all five
// dimensions would let one physical conversation run two concurrent message
// turns after a scope change or classification escalation — exactly the
// interleaving this module exists to prevent. The full five-dimension key
// belongs to context assembly (AR3), not to session location.
func locator(key agentcontext.ContextKey) string {
	return key.LegalEntityID() + "\x1f" + key.UserID() + "\x1f" + key.SessionID()
}
