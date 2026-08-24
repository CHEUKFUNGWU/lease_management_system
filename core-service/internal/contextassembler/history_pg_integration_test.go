package contextassembler

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// AR3 acceptance 4: HistorySource ownership enforcement runs against real
// Postgres. SKIP DOES NOT COUNT AS EVIDENCE — these only mean something under
// make test-integration.

func ar3Pool(t *testing.T) *pgxpool.Pool {
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

// ar3UniqueSuffix gives seeded codes/users a unique label per run (legal_
// entities.code has a uniqueness constraint and crashed runs leave rows
// behind until manual cleanup).
func ar3UniqueSuffix() string {
	b := make([]byte, 6)
	if _, err := crand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func ar3SeedTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (entityID, userID string) {
	t.Helper()
	suffix := ar3UniqueSuffix()
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"ar3-"+suffix, "AR3 entity "+suffix,
	).Scan(&entityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"ar3-"+suffix, "ar3-"+suffix+"@test.local", entityID,
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

func ar3SeedSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, ownerUser string, ownerEntity *string) (sessionID string) {
	t.Helper()
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions (user_id, legal_entity_id, title)
		VALUES ($1::uuid, $2::uuid, 'ar3 history source test') RETURNING id`,
		ownerUser, ownerEntity,
	).Scan(&sessionID); err != nil {
		t.Fatalf("seed ai_chat_session: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE id = $1::uuid`, sessionID)
	})
	return sessionID
}

func ar3SeedMessage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sessionID, role, content string, sequence, measuredTokens int) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO ai_chat_messages (session_id, role, message_type, sequence_no, content, measured_tokens)
		VALUES ($1::uuid, $2, 'text', $3, $4, $5)`,
		sessionID, role, sequence, content, measuredTokens,
	); err != nil {
		t.Fatalf("seed ai_chat_message: %v", err)
	}
}

func ar3Key(t *testing.T, entityID, userID, sessionID string) agentcontext.ContextKey {
	t.Helper()
	key, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: userID, Scope: access.Scope{LegalEntityID: entityID},
	}, sessionID, agentcontext.ClassificationProduction)
	if err != nil {
		t.Fatalf("construct key: %v", err)
	}
	return key
}

// 验收 4（正向）：归属匹配时按会话序返回 Role/Text/MeasuredTokens/Ref。
func TestPgHistorySourceReadsOwnedSessionInOrder(t *testing.T) {
	pool := ar3Pool(t)
	ctx := context.Background()

	entityID, userID := ar3SeedTenant(t, ctx, pool)
	sessionID := ar3SeedSession(t, ctx, pool, userID, &entityID)
	ar3SeedMessage(t, ctx, pool, sessionID, "user", "第一条", 1, 15)
	ar3SeedMessage(t, ctx, pool, sessionID, "assistant", "第二条", 2, 42)
	ar3SeedMessage(t, ctx, pool, sessionID, "user", "未测量的尾部", 3, 0)

	source := NewPgHistorySource(pool)
	messages, err := source.Read(ctx, ar3Key(t, entityID, userID, sessionID))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(messages))
	}
	wantTexts := []string{"第一条", "第二条", "未测量的尾部"}
	for i, m := range messages {
		if m.Text != wantTexts[i] {
			t.Errorf("message[%d].Text = %q, want %q (conversation order)", i, m.Text, wantTexts[i])
		}
		if m.Kind != KindText {
			t.Errorf("message[%d].Kind = %q; stored chat rows are text today", i, m.Kind)
		}
		if m.Ref == "" {
			t.Errorf("message[%d].Ref empty — dropped refs would be unresolvable", i)
		}
	}
	if messages[0].MeasuredTokens != 15 || messages[1].MeasuredTokens != 42 {
		t.Errorf("measured tokens not mapped: %d/%d, want 15/42",
			messages[0].MeasuredTokens, messages[1].MeasuredTokens)
	}
	if messages[2].MeasuredTokens != 0 {
		t.Errorf("unmeasured row must stay at the 0 sentinel, got %d", messages[2].MeasuredTokens)
	}
}

// 验收 4（拒绝）：跨法人 / 跨用户读取保持 scope_denied，不软化成空历史。
func TestPgHistorySourceRefusesForeignSessionWithScopeDenied(t *testing.T) {
	pool := ar3Pool(t)
	ctx := context.Background()

	entityA, userA := ar3SeedTenant(t, ctx, pool)
	entityB, userB := ar3SeedTenant(t, ctx, pool)
	sessionA := ar3SeedSession(t, ctx, pool, userA, &entityA)
	ar3SeedMessage(t, ctx, pool, sessionA, "user", "法人 A 的秘密", 1, 10)

	source := NewPgHistorySource(pool)

	// cross-entity: user B of entity B reads A's conversation
	if _, err := source.Read(ctx, ar3Key(t, entityB, userB, sessionA)); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("cross-entity read returned %v; want ErrScopeDenied (never softened)", err)
	}
	// same entity, different user
	if _, err := source.Read(ctx, ar3Key(t, entityA, userB, sessionA)); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("cross-user read returned %v; want ErrScopeDenied", err)
	}
}

func TestPgHistorySourceRefusesLegacyNullEntitySession(t *testing.T) {
	pool := ar3Pool(t)
	ctx := context.Background()

	entityID, userID := ar3SeedTenant(t, ctx, pool)
	sessionID := ar3SeedSession(t, ctx, pool, userID, nil) // legacy NULL-entity row

	source := NewPgHistorySource(pool)
	if _, err := source.Read(ctx, ar3Key(t, entityID, userID, sessionID)); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("NULL-entity session returned %v; a key always carries an entity and must refuse", err)
	}
}

// AR1 D-C9b 消费方策略：AR3 显式拒绝 global 键——记忆/压缩摘要不跨法人搬运。
// 无论行内法人为何值（NULL 或任何实体），global 上下文在 AR3 里没有可归属
// 的记忆，拒绝保持 scope_denied 不软化。
func TestPgHistorySourceRefusesGlobalKey(t *testing.T) {
	pool := ar3Pool(t)
	ctx := context.Background()

	entityID, userID := ar3SeedTenant(t, ctx, pool)
	sessionID := ar3SeedSession(t, ctx, pool, userID, &entityID)
	ar3SeedMessage(t, ctx, pool, sessionID, "user", "法人内的对话", 1, 10)

	globalKey, err := agentcontext.KeyFrom(agenttools.Principal{
		UserID: userID, Scope: access.Scope{Global: true},
	}, sessionID, agentcontext.ClassificationProduction)
	if err != nil {
		t.Fatalf("construct global key: %v", err)
	}
	if !globalKey.IsGlobal() {
		t.Fatal("expected global key")
	}

	source := NewPgHistorySource(pool)
	if _, err := source.Read(ctx, globalKey); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("global-key read returned %v; AR3 consumer policy refuses global keys", err)
	}

	// 反向验证：同用户同会话的 scoped 键（+对应法人）仍可读——拒绝是键形态
	// 触发的，不是行数据触发的。
	if _, err := source.Read(ctx, ar3Key(t, entityID, userID, sessionID)); err != nil {
		t.Fatalf("scoped key read failed after global refusal: %v", err)
	}
}

func TestPgHistorySourceUnknownSessionIsEmptyNotDenied(t *testing.T) {
	pool := ar3Pool(t)
	ctx := context.Background()

	entityID, userID := ar3SeedTenant(t, ctx, pool)
	source := NewPgHistorySource(pool)

	messages, err := source.Read(ctx, ar3Key(t, entityID, userID, "00000000-0000-4000-8000-000000000001"))
	if err != nil {
		t.Fatalf("unknown locator returned %v; want empty history", err)
	}
	if len(messages) != 0 {
		t.Fatalf("unknown locator returned %d messages; want 0", len(messages))
	}
}
