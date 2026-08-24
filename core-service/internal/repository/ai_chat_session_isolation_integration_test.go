package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
)

// seedChatIsolationTenants creates two legal entities and ONE user who spans
// both (集团 FP&A / 大区 BP 形态:同一个自然人横跨多个法人). The session rows
// hang off that single user id, which is exactly the shape that made the old
// GetSessionByID leak: user_id 相同,只能靠 legal_entity_id 区分归属.
func seedChatIsolationTenants(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (entityA, entityB, userID string) {
	t.Helper()
	suffix := uuidSuffix()
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "ISOL-A-"+suffix, "Isolation entity A "+label).Scan(&entityA); err != nil {
		t.Fatalf("seed entity A: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "ISOL-B-"+suffix, "Isolation entity B "+label).Scan(&entityB); err != nil {
		t.Fatalf("seed entity B: %v", err)
	}
	// One natural person, legal_entity_id NULL — the account can act under
	// both entities via JWT scope switching.
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "isol-"+suffix, "isol-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed spanning user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`, []string{entityA, entityB})
	})
	return entityA, entityB, userID
}

// seedChatSession inserts one session for the given user/entity and returns
// its id.
func seedChatSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, entityID *string, title string) string {
	t.Helper()
	var sessionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions (user_id, legal_entity_id, title)
		VALUES ($1, $2, $3) RETURNING id
	`, userID, entityID, title).Scan(&sessionID); err != nil {
		t.Fatalf("seed chat session %s: %v", title, err)
	}
	return sessionID
}

// TestAIChatSessionGetByIDRefusesCrossEntity pins the SI1 contract at the
// repository boundary: the same user brings an entity A session id into an
// entity B context. The load must refuse with scope_denied — never return the
// session, never soften to not_found.
func TestAIChatSessionGetByIDRefusesCrossEntity(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	entityA, entityB, userID := seedChatIsolationTenants(t, ctx, pool, "xent")

	repo := NewAIChatRuntimeRepository(pool)
	sessionInA := seedChatSession(t, ctx, pool, userID, &entityA, "A 的会话")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionInA)
	})

	scopedB, err := access.EntityFilterFor(entityB)
	if err != nil {
		t.Fatalf("build scoped-B filter: %v", err)
	}
	_, err = repo.GetSessionByID(ctx, sessionInA, userID, scopedB)
	if err == nil {
		t.Fatal("cross-entity session load succeeded: 法人 B 上下文取到了法人 A 的会话")
	}
	if got := errcontract.CodeOf(err); got != errcontract.CodeScopeDenied {
		t.Fatalf("cross-entity refusal code = %q; want %q (scope_denied must be preserved, not softened)",
			got, errcontract.CodeScopeDenied)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("cross-entity refusal was softened into not_found")
	}
}

// TestAIChatSessionGetByIDRefusesWrongUser pins the user ownership axis: even
// a global admin only loads their own sessions.
func TestAIChatSessionGetByIDRefusesWrongUser(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	entityA, _, userID := seedChatIsolationTenants(t, ctx, pool, "wusr")

	repo := NewAIChatRuntimeRepository(pool)
	sessionInA := seedChatSession(t, ctx, pool, userID, &entityA, "A 的会话")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionInA)
	})

	// A different user id, same entity: must refuse on the user axis.
	_, err := repo.GetSessionByID(ctx, sessionInA, "some-other-user", access.GlobalEntityFilter())
	if err == nil || errcontract.CodeOf(err) != errcontract.CodeScopeDenied {
		t.Fatalf("wrong-user load error = %v; want scope_denied", err)
	}
}

// TestAIChatSessionGetByIDScopedMatchLoads pins the happy path: same entity
// boundary, same user → session loads.
func TestAIChatSessionGetByIDScopedMatchLoads(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	entityA, _, userID := seedChatIsolationTenants(t, ctx, pool, "smatch")

	repo := NewAIChatRuntimeRepository(pool)
	sessionInA := seedChatSession(t, ctx, pool, userID, &entityA, "A 的会话")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionInA)
	})

	scopedA, err := access.EntityFilterFor(entityA)
	if err != nil {
		t.Fatalf("build scoped-A filter: %v", err)
	}
	got, err := repo.GetSessionByID(ctx, sessionInA, userID, scopedA)
	if err != nil {
		t.Fatalf("same-entity session load failed: %v", err)
	}
	if got.ID != sessionInA || got.LegalEntityID == nil || *got.LegalEntityID != entityA {
		t.Fatalf("loaded session mismatch: %+v", got)
	}
}

// TestAIChatSessionGetByIDGlobalAdminReadsAcrossEntities pins the AR5d
// decision: a GLOBAL admin (Scope.Global=true, JWT 空租户 id) may load any
// session — including another entity's and NULL-legal-entity legacy rows.
// The global state comes from the resolved access scope, not from an empty
// string, so this forgiveness does not weaken the scoped check.
func TestAIChatSessionGetByIDGlobalAdminReadsAcrossEntities(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	entityA, entityB, userID := seedChatIsolationTenants(t, ctx, pool, "gadm")

	repo := NewAIChatRuntimeRepository(pool)
	sessionInA := seedChatSession(t, ctx, pool, userID, &entityA, "A 的会话")
	sessionInB := seedChatSession(t, ctx, pool, userID, &entityB, "B 的会话")
	sessionNull := seedChatSession(t, ctx, pool, userID, nil, "无法人存量会话")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM ai_chat_sessions WHERE id = ANY($1::uuid[])`,
			[]string{sessionInA, sessionInB, sessionNull})
	})

	global := access.GlobalEntityFilter()
	for _, sid := range []string{sessionInA, sessionInB, sessionNull} {
		got, err := repo.GetSessionByID(ctx, sid, userID, global)
		if err != nil {
			t.Fatalf("global admin failed to load session %s: %v", sid, err)
		}
		if got == nil || got.ID != sid {
			t.Fatalf("global admin loaded wrong session for %s: %+v", sid, got)
		}
	}
}

// TestAIChatSessionGetByIDNullLegacyRowScopedRefused pins the NULL-entity
// legacy-row decision: a SCOPED user's boundary never matches a NULL
// legal_entity_id row (exactly like sessionmanager.PostgresStore.Load), so
// the load refuses with scope_denied instead of returning an unownable
// legacy conversation into a tenant context.
func TestAIChatSessionGetByIDNullLegacyRowScopedRefused(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	_, entityB, userID := seedChatIsolationTenants(t, ctx, pool, "nrow")

	repo := NewAIChatRuntimeRepository(pool)
	sessionNull := seedChatSession(t, ctx, pool, userID, nil, "无法人存量会话")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionNull)
	})

	scopedB, err := access.EntityFilterFor(entityB)
	if err != nil {
		t.Fatalf("build scoped-B filter: %v", err)
	}
	_, err = repo.GetSessionByID(ctx, sessionNull, userID, scopedB)
	if err == nil {
		t.Fatal("scoped user loaded a NULL-legal-entity legacy session")
	}
	if got := errcontract.CodeOf(err); got != errcontract.CodeScopeDenied {
		t.Fatalf("NULL-entity refusal code = %q; want %q", got, errcontract.CodeScopeDenied)
	}
}
