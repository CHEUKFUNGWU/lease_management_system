package repository

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
)

// TestAIChatSessionTenantFilterNullLegalEntity pins down how
// ai_chat_sessions rows with a NULL legal_entity_id behave under the
// EntityFilter conversion (SEC-004, the 39th escape hatch):
//
//   - a scoped filter must exclude NULL rows — a session without a legal
//     entity has no Legal Entity Access for a non-admin, exactly like the old
//     COALESCE predicate ('' = LE is false);
//   - a global (admin) filter must include NULL rows — the admin's "no
//     clause" reads everything, exactly like the old `$2=''` short-circuit.
func TestAIChatSessionTenantFilterNullLegalEntity(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "aichat")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "aichat-user-"+uuidSuffix(), "aichat@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed chat user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	seed := func(legalEntityID *string, title string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO ai_chat_sessions (user_id, legal_entity_id, title)
			VALUES ($1, $2, $3)
		`, userID, legalEntityID, title); err != nil {
			t.Fatalf("seed session %s: %v", title, err)
		}
	}
	entityA := pair.entityA
	seed(&entityA, "session-in-A")
	seed(nil, "session-without-entity")

	repo := NewAIChatRuntimeRepository(pool)

	// Scoped to entity A: only the session that belongs to A, never the
	// NULL-entity session.
	scopedFilter, err := access.EntityFilterFor(entityA)
	if err != nil {
		t.Fatalf("build scoped filter: %v", err)
	}
	scoped, err := repo.ListSessions(ctx, AIChatSessionFilter{UserID: userID, Entity: scopedFilter, Limit: 50})
	if err != nil {
		t.Fatalf("list sessions scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Title != "session-in-A" {
		t.Fatalf("scoped sessions = %+v; want exactly the entity-A session", scoped)
	}

	// Scoped to another entity: nothing, including the NULL-entity session.
	otherFilter, err := access.EntityFilterFor(pair.entityB)
	if err != nil {
		t.Fatalf("build other-entity filter: %v", err)
	}
	other, err := repo.ListSessions(ctx, AIChatSessionFilter{UserID: userID, Entity: otherFilter, Limit: 50})
	if err != nil {
		t.Fatalf("list sessions scoped to B: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("entity B saw entity A or NULL-entity sessions: %+v", other)
	}

	// Global: both sessions, including the NULL-entity one.
	global, err := repo.ListSessions(ctx, AIChatSessionFilter{UserID: userID, Entity: access.GlobalEntityFilter(), Limit: 50})
	if err != nil {
		t.Fatalf("list sessions global: %v", err)
	}
	if len(global) != 2 {
		t.Fatalf("global sessions = %d; want 2 (entity-A + NULL-entity)", len(global))
	}
}

// TestAIChatSessionInitiatorFiltering pins CHAT-001: system-initiated
// sessions (home brief auto-runs) must be invisible to the user-facing
// session list while their runs and audit records stay fully intact. The
// session row itself is the audit anchor — filtering is a list projection,
// not a delete.
func TestAIChatSessionInitiatorFiltering(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "aichatinit")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "aichat-init-"+uuidSuffix(), "aichat-init@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed chat user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	repo := NewAIChatRuntimeRepository(pool)
	globalFilter := access.GlobalEntityFilter()

	// Three system-initiated sessions, exactly what three home-brief runs
	// create when the caller sends initiator="system".
	userSession := &AIChatSession{UserID: userID, Title: "user chat", Initiator: "user"}
	if err := repo.CreateSession(ctx, userSession); err != nil {
		t.Fatalf("create user session: %v", err)
	}
	for i := 0; i < 3; i++ {
		s := &AIChatSession{UserID: userID, Title: "brief", Initiator: "system"}
		if err := repo.CreateSession(ctx, s); err != nil {
			t.Fatalf("create system session %d: %v", i, err)
		}
	}

	// Default list (ExcludeInitiator="system"): only the user session.
	visible, err := repo.ListSessions(ctx, AIChatSessionFilter{
		UserID: userID, Entity: globalFilter, ExcludeInitiator: "system", Limit: 50,
	})
	if err != nil {
		t.Fatalf("list visible sessions: %v", err)
	}
	if len(visible) != 1 || visible[0].Initiator != "user" {
		t.Fatalf("visible sessions = %d (initiators %v); want exactly the user session",
			len(visible), sessionInitiators(visible))
	}

	// include_system view: all four.
	all, err := repo.ListSessions(ctx, AIChatSessionFilter{
		UserID: userID, Entity: globalFilter, Limit: 50,
	})
	if err != nil {
		t.Fatalf("list all sessions: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("all sessions = %d; want 4 (1 user + 3 system)", len(all))
	}

	// The system sessions remain retrievable by ID — runs, messages and
	// audit trail hang off them and must stay reachable.
	systemSessions := 0
	for _, s := range all {
		if s.Initiator != "system" {
			continue
		}
		systemSessions++
		got, err := repo.GetSessionByID(ctx, s.ID, userID)
		if err != nil {
			t.Fatalf("get system session %s: %v", s.ID, err)
		}
		if got.Initiator != "system" || got.Title != "brief" {
			t.Fatalf("retrieved system session = %+v", got)
		}
	}
	if systemSessions != 3 {
		t.Fatalf("system sessions created = %d; want 3", systemSessions)
	}
}

func sessionInitiators(sessions []*AIChatSession) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.Initiator
	}
	return out
}
