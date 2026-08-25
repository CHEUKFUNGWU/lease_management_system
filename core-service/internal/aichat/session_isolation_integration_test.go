package aichat

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
)

// randSuffix is a short unique suffix for seeded identifiers.
func randSuffix() string {
	return uuid.NewString()[:8]
}

// postgresPool connects to the real test database named by TEST_DATABASE_URL
// and is skipped when unset — matching the repository and sessionmanager
// conventions. SKIP DOES NOT COUNT AS EVIDENCE: the cross-tenant proof below
// only means something when these tests actually RUN (make test-integration).
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

// seedContinuationTenant creates a UNIQUE user spanning two legal entities
// (集团 FP&A / 大区 BP 形态:同一个自然人在 JWT 切换下可以以法人 A 或法人 B
// 的身份行动). The chat session belongs to entity A.
func seedContinuationTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (entityA, entityB, userID, sessionID, runID string) {
	t.Helper()
	suffix := uuidSuffix()
	for _, side := range []struct {
		label    string
		entityID *string
	}{
		{"A", &entityA}, {"B", &entityB},
	} {
		id := ""
		if err := pool.QueryRow(ctx, `
			INSERT INTO legal_entities (code, name, country, currency, is_active)
			VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
		`, "CHAT-LE-"+side.label+"-"+suffix, "Chat tenant "+side.label).Scan(&id); err != nil {
			t.Fatalf("seed legal entity %s: %v", side.label, err)
		}
		*side.entityID = id
	}
	// 一个自然人, legal_entity_id NULL —— 该账号通过 JWT scope 切换行动。
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, 'bcrypt-placeholder') RETURNING id
	`, "chat-"+suffix, "chat-"+suffix+"@example.com").Scan(&userID); err != nil {
		t.Fatalf("seed chat user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`,
			[]string{entityA, entityB})
	})

	// 会话属于法人 A。
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions (user_id, legal_entity_id, title)
		VALUES ($1, $2, 'A 的会话') RETURNING id
	`, userID, entityA).Scan(&sessionID); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_runs (session_id, status, agent_mode)
		VALUES ($1, 'completed', true) RETURNING id
	`, sessionID).Scan(&runID); err != nil {
		t.Fatalf("seed chat run: %v", err)
	}
	return entityA, entityB, userID, sessionID, runID
}


// TestContinueCrossEntityRefusedWithScopeDenied is the SI1 gap test. It is
// written at the layer where the CALLER's legal entity is expressible on the
// existing API — the request context carries the resolved access.Scope the
// same way DataScopeMiddleware installs it in production, and the
// continuation path already accepts LegalEntityID on ContinueCommand.
//
// Scenario: 同一个自然人 U, 会话属于法人 A; U 在法人 B 的上下文里带着 A 的
// session id 走续接路径。续接必须拒绝, 且拒绝原因是 scope_denied —— 不是
// not_found, 不是空结果, 不是成功续上 A 的对话历史。
//
// Red-first discipline: this test is written BEFORE any source change, it
// compiles against the current signatures, and it is RED today because the
// session loads and the continuation proceeds. The signature evolution that
// makes it green is driven by this test, not the other way around.
func TestContinueCrossEntityRefusedWithScopeDenied(t *testing.T) {
	pool := postgresPool(t)
	baseCtx := context.Background()

	_, entityB, userID, _, runID := seedContinuationTenant(t, baseCtx, pool)

	store := repository.NewAIChatRuntimeRepository(pool)
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) {
			return testResponse{Answer: "continued", Model: "test"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	// U 的身份, 法人 B 的上下文, target 是法人 A 的 run。上下文 scope 与
	// command.LegalEntityID 都和 handler 在真实请求里注入的一致。
	ctx := access.WithScope(baseCtx, access.Scope{LegalEntityID: entityB})
	_, err := runtime.Continue(ctx, ContinueCommand{
		Target:        Target{Type: "run", ID: runID},
		UserID:        userID,
		LegalEntityID: entityB,
	})
	if err == nil {
		t.Fatal("法人 B 上下文续接了法人 A 的会话 —— 会话没有带进 prompt 的隔离缺口")
	}
	if got := errcontract.CodeOf(err); got != errcontract.CodeScopeDenied {
		t.Fatalf("cross-entity continuation error code = %q; want %q (scope_denied must be preserved, not softened)",
			got, errcontract.CodeScopeDenied)
	}
}

// TestContinueSameEntityProceeds pins the happy path at the same layer: the
// same caller in the owning entity's context may continue the session.
func TestContinueSameEntityProceeds(t *testing.T) {
	pool := postgresPool(t)
	baseCtx := context.Background()

	entityA, _, userID, _, runID := seedContinuationTenant(t, baseCtx, pool)

	store := repository.NewAIChatRuntimeRepository(pool)
	runtime := newRuntime(
		store,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(_ context.Context, _ Execution) (testResponse, error) {
			return testResponse{Answer: "continued", Model: "test"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	)

	ctx := access.WithScope(baseCtx, access.Scope{LegalEntityID: entityA})
	started, err := runtime.Continue(ctx, ContinueCommand{
		Target:        Target{Type: "run", ID: runID},
		UserID:        userID,
		LegalEntityID: entityA,
	})
	if err != nil {
		t.Fatalf("same-entity continuation failed: %v", err)
	}
	if started == nil || started.Continuation == nil {
		t.Fatalf("same-entity continuation returned no continuation: %+v", started)
	}
}

// uuidSuffix returns a short unique suffix for seeded identifiers. Same
// family as the repository package tests: uuid-based, fits VARCHAR(50).
func uuidSuffix() string {
	return "chat-" + randSuffix()
}

// ── SI2 写路径红测试 ──────────────────────────────────────────────────────
// 对另一法人会话的 artifact 执行审批动作必须拒绝（scope_denied，不软化）。
// 今天 Review 只按 user 校验 → 成功 → 本测试红；修复后绿。
func TestReviewRefusesCrossEntityArtifactAction(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	_, entityB, userID, sessionID, _ := seedContinuationTenant(t, ctx, pool)

	repo := repository.NewAIChatRuntimeRepository(pool)
	artifactID := ""
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_artifacts (session_id, run_id, artifact_type, title, data)
		VALUES ($1, (SELECT id FROM ai_chat_runs WHERE session_id = $1 LIMIT 1), 'contract_draft', 'A 的草稿', '{}'::jsonb)
		RETURNING id`, sessionID).Scan(&artifactID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ai_chat_artifacts WHERE id = $1`, artifactID)
	})

	runtime := newRuntime(
		repo,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{} }),
		ExecutorFunc[testResponse](func(context.Context, Execution) (testResponse, error) { return testResponse{}, nil }),
		func(response testResponse) Result { return Result{} },
		Options{},
	)

	// U 的身份，法人 B 的上下文，对一个法人 A 的 artifact 执行审批。
	ctxB := access.WithScope(ctx, access.Scope{LegalEntityID: entityB})
	_, err := runtime.Review(ctxB, ReviewCommand{
		ArtifactID: artifactID, ActionType: "confirm", UserID: userID,
	})
	if err == nil {
		t.Fatal("法人 B 上下文对法人 A 的 artifact 执行了审批 —— SI2 写路径跨法人缺口")
	}
	if got := errcontract.CodeOf(err); got != errcontract.CodeScopeDenied {
		t.Fatalf("cross-entity review error code = %q; want %q", got, errcontract.CodeScopeDenied)
	}
}
