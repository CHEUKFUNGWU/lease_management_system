package aichat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/sessionmanager"
)

// SessionIntent carries what the chat plane knows about the conversation it
// is about to join or start. The resolved scope rides the request context —
// the adapter derives the boundary from it, never from caller-supplied
// fields (D-C4: 该值由与 JWT 同一个解析器产出).
type SessionIntent struct {
	// UserID is the acting natural person (must be non-empty).
	UserID string
	// LegalEntityID is the raw tenant string ("" for global admin). It feeds
	// key construction via the resolved scope; the scope's Global state
	// decides global vs scoped, never this string alone.
	LegalEntityID string
	// SessionID selects an existing session to load, or is empty to create a
	// fresh one. Get-or-create happens in one Acquire call.
	SessionID string
	// Create-only hints, applied after the row comes into existence.
	Title           string
	ContractID      string
	ContextSnapshot json.RawMessage
	Initiator       string
	// HoldLease keeps the exclusive per-session lease for the caller to hold
	// across a run's execution. OpenSession (pure create, no run) passes
	// false — create and release immediately, so an idle conversation never
	// carries a lease that would lock its own first message.
	HoldLease bool
}

// SessionOwner is the AR2 lifecycle seam injected into the chat runtime. It
// answers one question: "resolve the conversation anchor for this turn —
// get-or-create it AND (when asked) hold the exclusive per-session lease".
// A nil SessionOwner keeps the legacy store.CreateSession / GetSessionByID
// path (unit tests, pre-wiring); production injects the sessionmanager
// adapter so the chat plane really flows through AR2 (SI1 Part B).
type SessionOwner interface {
	ResolveSession(ctx context.Context, intent SessionIntent) (*repository.AIChatSession, func(), error)
}

// sessionOwnerAdapter is the production SessionOwner: sessionmanager owns
// lifecycle/ownership/lease; the repository owns the content face
// (bound_contract_id, context_snapshot, initiator, title). The two reads —
// module Acquire + repository content read — carry the SAME boundary, so
// their ownership judgements can never disagree (SI1 D2: 两次读要是对归属的
// 判断能不一致，就是把一个洞换成两个洞).
type sessionOwnerAdapter struct {
	mgr  sessionmanager.Manager
	repo *repository.AIChatRuntimeRepository
}

// NewSessionOwner wires the AR2 manager over the production store.
func NewSessionOwner(mgr sessionmanager.Manager, repo *repository.AIChatRuntimeRepository) SessionOwner {
	return &sessionOwnerAdapter{mgr: mgr, repo: repo}
}

func (a *sessionOwnerAdapter) ResolveSession(ctx context.Context, intent SessionIntent) (*repository.AIChatSession, func(), error) {
	scope, ok := access.ScopeFromContext(ctx)
	if !ok {
		return nil, nil, fmt.Errorf("resolve session ownership: no resolved access scope in request context")
	}
	boundary, err := access.FromScope(scope)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve session ownership boundary: %w", err)
	}

	sessionID := intent.SessionID
	creating := sessionID == ""
	if creating {
		sessionID = uuid.New().String()
	}
	key, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: intent.UserID,
		Scope:  scope,
	}, sessionID, agentcontext.ClassificationProduction)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve session ownership key: %w", err)
	}

	sms, release, err := a.mgr.Acquire(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire ai chat session %s: %w", sessionID, err)
	}

	if creating {
		if err := a.repo.UpdateSessionContent(ctx, sessionID, intent.UserID, boundary, repository.SessionContent{
			Title:           intent.Title,
			BoundContractID: intent.ContractID,
			ContextSnapshot: intent.ContextSnapshot,
			Initiator:       intent.Initiator,
		}); err != nil {
			release()
			return nil, nil, fmt.Errorf("enrich ai chat session %s: %w", sessionID, err)
		}
	}

	if !intent.HoldLease {
		release()
		release = nil
	}

	// 内容双读：模块给生命周期 face，repository 给内容 face，同一 boundary。
	full, err := a.repo.GetSessionByID(ctx, sessionID, intent.UserID, boundary)
	if err != nil {
		if release != nil {
			release()
		}
		return nil, nil, fmt.Errorf("load ai chat session %s content: %w", sessionID, err)
	}
	if sms.Status == "active" && full.Status == "" {
		full.Status = sms.Status
	}
	return full, release, nil
}
