package sessionmanager

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// postgresPool connects to the real test database named by TEST_DATABASE_URL
// and is skipped when unset — matching the repository package convention.
// SKIP DOES NOT COUNT AS EVIDENCE: the cross-tenant proof below only means
// something when these tests actually RUN (make test-integration).
func postgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedSessionTenant creates one legal entity plus one user in it, with a
// uniquely labelled cleanup.
func seedSessionTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (entityID, userID string) {
	t.Helper()
	suffix := shortID() + "-" + label // legal_entities.code is VARCHAR(50): keep it short
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"ar2-"+suffix, "AR2 entity "+suffix,
	).Scan(&entityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"ar2-"+suffix, "ar2-"+suffix+"@test.local", entityID,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = $1`, entityID)
	})
	return entityID, userID
}

func mustKeyFor(t *testing.T, entityID, userID, sessionID string) agentcontext.ContextKey {
	t.Helper()
	key, err := agentcontext.KeyFrom(
		principalFor(entityID, userID), sessionID, agentcontext.ClassificationProduction,
	)
	if err != nil {
		t.Fatalf("build key: %v", err)
	}
	return key
}

// shortID gives seeding a short unique suffix that fits VARCHAR(50) codes.
func shortID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])[:10]
}

// ── 验收 4：跨法人（带库）——法人 A 取不到法人 B 的会话，拒绝保持 scope_denied ──

func TestCrossTenantAcquireRefusedWithScopeDenied(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	entityAID, userAID := seedSessionTenant(t, ctx, pool, "entA")
	entityBID, userBID := seedSessionTenant(t, ctx, pool, "entB")

	sessionB := "77777777-0000-4000-8000-0000000000b7"
	store := NewPostgresStore(pool)
	m := New(store, Policy{})
	defer m.Stop()

	keyB := mustKeyFor(t, entityBID, userBID, sessionB)
	_, release, err := m.Acquire(ctx, keyB)
	if err != nil {
		t.Fatalf("owner acquire: %v", err)
	}
	release()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionB)
	})

	// Entity A's identity, B's conversation id: refused, reason intact.
	keyA := mustKeyFor(t, entityAID, userAID, sessionB)
	_, _, err = m.Acquire(ctx, keyA)
	if err == nil {
		t.Fatal("entity A acquired entity B's conversation through the real store")
	}
	if !IsScopeDenied(err) || !strings.Contains(err.Error(), "scope_denied") {
		t.Fatalf("error = %v; want scope_denied preserved", err)
	}
	if IsNotFound(err) {
		t.Fatal("scope refusal was softened into not-found")
	}

	// The same refusal holds for a raw Store.Load outside the manager.
	if _, loadErr := store.Load(ctx, keyA); !IsScopeDenied(loadErr) {
		t.Fatalf("raw Load error = %v; want scope_denied", loadErr)
	}
}

// Save refuses to edit a foreign conversation: the upsert's ownership
// predicate matches zero rows instead of silently rewriting another tenant's
// session title/status.
func TestSaveCannotEditForeignSession(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	entityAID, userAID := seedSessionTenant(t, ctx, pool, "own")
	entityBID, userBID := seedSessionTenant(t, ctx, pool, "frn")

	sessionID := "88888888-0000-4000-8000-0000000000c8"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionID)
	})

	store := NewPostgresStore(pool)

	// B owns the row.
	keyB := mustKeyFor(t, entityBID, userBID, sessionID)
	foreign := &Session{Title: "B 的工作会话", Status: "active"}
	if err := store.Save(ctx, keyB, foreign); err != nil {
		t.Fatalf("seed owner row: %v", err)
	}

	// A tries to save under the same session id: zero rows match → refusal,
	// and B's title survives untouched.
	keyA := mustKeyFor(t, entityAID, userAID, sessionID)
	intruder := &Session{Title: "A 的篡改", Status: "closed"}
	err := store.Save(ctx, keyA, intruder)
	if !IsScopeDenied(err) {
		t.Fatalf("cross-tenant save error = %v; want scope_denied", err)
	}
	var storedTitle string
	if err := pool.QueryRow(ctx,
		`SELECT title FROM ai_chat_sessions WHERE id = $1::uuid`, sessionID,
	).Scan(&storedTitle); err != nil || storedTitle != "B 的工作会话" {
		t.Fatalf("foreign save mutated the row: title=%q err=%v", storedTitle, err)
	}
}

// ── Round trip：Save→Load 字段一致；classification 落库 ─────────────────────

func TestPostgresStoreRoundTrip(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	entityID, userID := seedSessionTenant(t, ctx, pool, "rt")
	sessionID := "99999999-0000-4000-8000-000000000099"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionID)
	})

	key := mustKeyFor(t, entityID, userID, sessionID)
	store := NewPostgresStore(pool)

	if _, err := store.Load(ctx, key); !IsNotFound(err) {
		t.Fatalf("loading an unknown session returned %v; want ErrNotFound", err)
	}

	created := &Session{Title: "roundtrip", Status: "active"}
	if err := store.Save(ctx, key, created); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.SessionID != sessionID || loaded.UserID != userID || loaded.LegalEntityID != entityID ||
		loaded.Title != "roundtrip" || loaded.Status != "active" ||
		loaded.Classification != agentcontext.ClassificationProduction {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

// ── 验收 4b：global 键（D-C9b）—— 全局管理员收下，NULL 法人行归 global ───────

// TestGlobalKeySavesAndLoadsNullEntityRow pins the AR1 D-C9b consumer
// contract for AR2: a global admin key (Scope.Global==true, 无具体法人)
// creates/loads a NULL-legal-entity session row — the shape admin chat
// sessions actually take (空租户创建 → legal_entity_id NULL, SEC-004)。
// The Save must write SQL NULL (not an empty uuid string, which errors),
// and the Load round trip must succeed.
func TestGlobalKeySavesAndLoadsNullEntityRow(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	// 全局管理员用户（users.legal_entity_id 可空，角色无关紧要）。
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "ar2-global-"+shortID(), "ar2-global@test.local").Scan(&userID); err != nil {
		t.Fatalf("seed global admin user: %v", err)
	}
	sessionID := "aaaaaaaa-0000-4000-8000-0000000000aa"
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
	})

	globalKey, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: userID, SubjectType: "web_ai_agent",
		Scope: access.Scope{Global: true},
	}, sessionID, agentcontext.ClassificationProduction)
	if err != nil {
		t.Fatalf("build global key: %v", err)
	}
	if !globalKey.IsGlobal() {
		t.Fatal("expected a global key")
	}

	store := NewPostgresStore(pool)
	created := &Session{Title: "admin 会话", Status: "active"}
	if err := store.Save(ctx, globalKey, created); err != nil {
		t.Fatalf("global key Save: %v", err)
	}

	// 行内 legal_entity_id 必须是 SQL NULL（不是空串）。
	var storedEntity *string
	if err := pool.QueryRow(ctx,
		`SELECT legal_entity_id FROM ai_chat_sessions WHERE id = $1::uuid`, sessionID,
	).Scan(&storedEntity); err != nil {
		t.Fatalf("read stored entity: %v", err)
	}
	if storedEntity != nil {
		t.Fatalf("global-key session stored legal_entity_id=%q; want NULL", *storedEntity)
	}

	loaded, err := store.Load(ctx, globalKey)
	if err != nil {
		t.Fatalf("global key Load: %v", err)
	}
	if loaded.SessionID != sessionID || loaded.UserID != userID || loaded.LegalEntityID != "" {
		t.Fatalf("global round trip mismatch: %+v", loaded)
	}

	// 反向：另一法人（scoped key）不得拿 NULL 行。
	entityAID, _ := seedSessionTenant(t, ctx, pool, "globA")
	_ = entityAID
	scopedA, err := agentcontext.KeyFrom(
		principalFor(entityAID, userID), sessionID, agentcontext.ClassificationProduction,
	)
	if err != nil {
		t.Fatalf("build scoped-A key with same user: %v", err)
	}
	if _, err := store.Load(ctx, scopedA); !IsScopeDenied(err) {
		t.Fatalf("scoped key loaded the global NULL row: err=%v; want scope_denied", err)
	}
}

// ── 验收 6：migration 062 与 01_init 等价，且 init 基线自身具备约束 ─────────

func TestMigration062MatchesInitBaseline(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	// 1. The init baseline ITSELF must carry the constraint (lesson 27ccdd2:
	//    a column without its CHECK on the baseline is a silent drift).
	var checkCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname = 'ai_chat_sessions_classification_check'
		  AND pg_get_constraintdef(oid) LIKE '%simulated%'
		  AND pg_get_constraintdef(oid) LIKE '%mixed%'`,
	).Scan(&checkCount); err != nil || checkCount != 1 {
		t.Fatalf("init baseline lacks ai_chat_sessions_classification_check (count=%d err=%v) — "+
			"a fresh volume would boot without the classification vocabulary enforced", checkCount, err)
	}

	// 2. The schema_migrations baseline must record 062 (lesson 32aac80),
	//    otherwise migrate.sh --status reports a pending migration forever.
	rawInit, readErr := os.ReadFile("../../../db/init/01_init.sql")
	if readErr != nil {
		t.Fatalf("read 01_init.sql: %v", readErr)
	}
	if !strings.Contains(string(rawInit), "'062_session_data_classification'") {
		t.Fatal("schema_migrations baseline does not register 062_session_data_classification")
	}

	// 3. Replaying 062 onto the init baseline must be a clean no-op.
	raw, err := os.ReadFile("../../../db/migrations/062_session_data_classification.sql")
	if err != nil {
		t.Fatalf("read migration 062: %v", err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatalf("replaying 062 onto the init baseline failed: %v", err)
	}

	// 4. Column shape after replay: NOT NULL with the production default.
	var dataType, nullable, columnDefault string
	if err := pool.QueryRow(ctx, `
		SELECT data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_name = 'ai_chat_sessions' AND column_name = 'data_classification'`,
	).Scan(&dataType, &nullable, &columnDefault); err != nil {
		t.Fatalf("inspect data_classification column: %v", err)
	}
	if dataType != "character varying" || nullable != "NO" ||
		!strings.Contains(columnDefault, "production") {
		t.Fatalf("data_classification column drifted: type=%q nullable=%q default=%q",
			dataType, nullable, columnDefault)
	}
}
