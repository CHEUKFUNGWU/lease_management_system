package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

// FIX-001 F3/F4: the assistant message confidence and its degradation reason
// round-trip through ai_chat_messages, and messages written before the columns
// existed (NULL) still load cleanly with confidence absent.
func TestAIChatMessageConfidenceRoundTrip(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	repo := NewAIChatRuntimeRepository(pool)

	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "confidence-user-"+uuidSuffix(), "confidence@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed confidence user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_messages WHERE session_id IN (SELECT id FROM ai_chat_sessions WHERE user_id = $1)`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	session := &AIChatSession{
		UserID: userID, Title: "confidence round-trip", CreatedAt: time.Now(),
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// F3: an assistant message with confidence and a degradation reason.
	confidence := 0.5
	reason := "AI 服务暂不可用，以下为系统数据摘要"
	withConfidence := &AIChatMessage{
		SessionID: session.ID, Role: "assistant", MessageType: "text",
		SequenceNo: 1, Content: "摘要", Model: ptrString("fallback"),
		Confidence: &confidence, ConfidenceReason: &reason,
	}
	if err := repo.CreateMessage(ctx, withConfidence); err != nil {
		t.Fatalf("create message with confidence: %v", err)
	}

	// F4: a message written the way the pre-migration code wrote it — no
	// confidence columns populated — must still load without error.
	legacy := &AIChatMessage{
		SessionID: session.ID, Role: "assistant", MessageType: "text",
		SequenceNo: 2, Content: "旧会话消息", Model: ptrString("gpt-4o-mini"),
	}
	if err := repo.CreateMessage(ctx, legacy); err != nil {
		t.Fatalf("create legacy message: %v", err)
	}

	messages, err := repo.ListMessagesBySession(ctx, session.ID, userID, access.GlobalEntityFilter(), 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}

	// The list is ordered by sequence descending: the legacy message first.
	if messages[0].Confidence != nil || messages[0].ConfidenceReason != nil {
		t.Fatalf("legacy message must load with confidence absent, got %+v", messages[0])
	}
	reloaded := messages[1]
	if reloaded.Confidence == nil || *reloaded.Confidence != 0.5 {
		t.Fatalf("confidence = %v, want 0.5", reloaded.Confidence)
	}
	if reloaded.ConfidenceReason == nil || *reloaded.ConfidenceReason != reason {
		t.Fatalf("confidence reason = %v, want %q", reloaded.ConfidenceReason, reason)
	}

	byID, err := repo.GetMessageByID(ctx, withConfidence.ID, userID, access.GlobalEntityFilter())
	if err != nil {
		t.Fatalf("get message by id: %v", err)
	}
	if byID.Confidence == nil || *byID.Confidence != 0.5 || byID.ConfidenceReason == nil {
		t.Fatalf("get by id confidence = %+v", byID)
	}
}

func ptrString(value string) *string { return &value }
