package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
)

// AR3 acceptance 3 + migration 063 consistency (062 precedent: assert the
// init baseline itself carries the column, then prove the migration replays
// clean). SKIP DOES NOT COUNT AS EVIDENCE — these only mean something under
// make test-integration.

func TestMigration063MeasuredTokensMatchesInitBaseline(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()

	// 1. The init baseline ITSELF carries the column with the 0 sentinel
	//    default (lesson 27ccdd2: a column missing on the baseline is silent
	//    drift — a fresh volume would boot without it).
	var dataType, nullable, columnDefault string
	if err := pool.QueryRow(ctx, `
		SELECT data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_name = 'ai_chat_messages' AND column_name = 'measured_tokens'`,
	).Scan(&dataType, &nullable, &columnDefault); err != nil {
		t.Fatalf("init baseline lacks ai_chat_messages.measured_tokens: %v", err)
	}
	if dataType != "integer" || nullable != "NO" || !strings.Contains(columnDefault, "0") {
		t.Fatalf("measured_tokens column drifted: type=%q nullable=%q default=%q",
			dataType, nullable, columnDefault)
	}

	// 2. The schema_migrations baseline must register 063 (lesson 32aac80),
	//    otherwise migrate.sh --status reports a pending migration forever.
	rawInit, readErr := os.ReadFile("../../../db/init/01_init.sql")
	if readErr != nil {
		t.Fatalf("read 01_init.sql: %v", readErr)
	}
	if !strings.Contains(string(rawInit), "'063_ai_chat_messages_tokens'") {
		t.Fatal("schema_migrations baseline does not register 063_ai_chat_messages_tokens")
	}

	// 3. Replaying 063 onto the init baseline must be a clean no-op.
	raw, err := os.ReadFile("../../../db/migrations/063_ai_chat_messages_tokens.sql")
	if err != nil {
		t.Fatalf("read migration 063: %v", err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatalf("replaying 063 onto the init baseline failed: %v", err)
	}
}

func TestAIChatMessageMeasuredTokensRoundTrip(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	repo := NewAIChatRuntimeRepository(pool)

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "tokens-user-"+uuidSuffix(), "tokens@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed tokens user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_messages WHERE session_id IN (SELECT id FROM ai_chat_sessions WHERE user_id = $1)`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	session := &AIChatSession{UserID: userID, Title: "tokens round-trip"}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	userMsg := &AIChatMessage{
		SessionID: session.ID, Role: "user", MessageType: "text",
		SequenceNo: 1, Content: "问题", CreatedBy: &userID,
	}
	if err := repo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatalf("create user message: %v", err)
	}
	const measured = 13318
	assistantMsg := &AIChatMessage{
		SessionID: session.ID, Role: "assistant", MessageType: "text",
		SequenceNo: 2, Content: "回答", MeasuredTokens: measured, CreatedBy: &userID,
	}
	if err := repo.CreateMessage(ctx, assistantMsg); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	messages, err := repo.ListMessagesBySession(ctx, session.ID, userID, access.GlobalEntityFilter(), 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	bySeq := map[int]*AIChatMessage{}
	for _, m := range messages {
		bySeq[m.SequenceNo] = m
	}
	if got := bySeq[2].MeasuredTokens; got != measured {
		t.Errorf("assistant row MeasuredTokens = %d, want %d (truth backfill)", got, measured)
	}
	// The user row never went through an LLM round — its sentinel stays 0.
	if got := bySeq[1].MeasuredTokens; got != 0 {
		t.Errorf("user row MeasuredTokens = %d, want the 0 sentinel", got)
	}

	single, err := repo.GetMessageByID(ctx, assistantMsg.ID, userID, access.GlobalEntityFilter())
	if err != nil {
		t.Fatalf("get message by id: %v", err)
	}
	if single.MeasuredTokens != measured {
		t.Errorf("GetMessageByID MeasuredTokens = %d, want %d", single.MeasuredTokens, measured)
	}
}
