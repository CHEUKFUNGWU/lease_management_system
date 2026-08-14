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
