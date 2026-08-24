package aichat

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/sessionmanager"
)

// SI1 Part B Q1 收敛：OpenSession 与 prepare 两条创建路径都必须经过
// sessionmanager（单一所有者），且标题语义正确收敛：
//   - OpenSession：显式 command.Title 优先；
//   - prepare（首条消息隐式创建）：summarizeTitle(message) 兜底；
//   - 模块默认 "新会话" 只在两者都缺时出现。
//
// 跑 make test-integration 实跑；skip 不算证据。
func TestOpenSessionAndPrepareConvergeThroughOwner(t *testing.T) {
	pool := postgresPool(t)
	baseCtx := context.Background()

	entityA, _, userID, _, _ := seedContinuationTenant(t, baseCtx, pool)

	repo := repository.NewAIChatRuntimeRepository(pool)
	mgr := sessionmanager.New(sessionmanager.NewPostgresStore(pool), sessionmanager.Policy{})
	defer mgr.Stop()
	owner := NewSessionOwner(mgr, repo)

	runtime := newRuntime(
		repo,
		PlannerFunc(func(Input, *repository.AIChatRun) Plan { return Plan{AgentMode: true} }),
		ExecutorFunc[testResponse](func(_ context.Context, _ Execution) (testResponse, error) {
			return testResponse{Answer: "done", Model: "test"}, nil
		}),
		func(response testResponse) Result {
			return Result{Answer: response.Answer, Model: response.Model}
		},
		Options{Dispatch: func(task func()) { task() }},
	).WithSessionOwner(owner)

	ctx := access.WithScope(baseCtx, access.Scope{LegalEntityID: entityA})
	scopedA, err := access.EntityFilterFor(entityA)
	if err != nil {
		t.Fatalf("build scoped filter: %v", err)
	}

	// 路径一：OpenSession 显式标题。
	explicit, err := runtime.OpenSession(ctx, SessionCommand{
		UserID:        userID,
		LegalEntityID: entityA,
		Title:         "显式标题会话",
		Initiator:     "user",
	})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	if explicit.ID == "" {
		t.Fatal("OpenSession did not assign a session id")
	}
	sessionTitleAt(t, pool, explicit.ID, "显式标题会话")

	// 路径二：prepare 首条消息隐式创建（Run 无 SessionID）。
	longMessage := strings.Repeat("这篇很长的消息用于测试自动摘要标题", 3)
	started, err := runtime.Run(ctx, Input{
		Message: longMessage,
		UserID:  userID,
	})
	if err != nil {
		t.Fatalf("Run (prepare create): %v", err)
	}
	if started == nil || started.Started == nil || started.Started.Run == nil {
		t.Fatal("no started run")
	}
	sessionTitleAt(t, pool, started.Started.Run.SessionID, summarizeTitle(longMessage))

	// 隐式创建会话可经 repository 以 scoped 边界读取，且归属正确。
	got, err := repo.GetSessionByID(ctx, started.Started.Run.SessionID, userID, scopedA)
	if err != nil {
		t.Fatalf("load auto-created session: %v", err)
	}
	if got.LegalEntityID == nil || *got.LegalEntityID != entityA {
		t.Fatalf("auto-created session entity = %v; want %s", got.LegalEntityID, entityA)
	}
}

// sessionTitleAt asserts the stored title for a session row, bypassing the
// ownership-checked GetSessionByID (which needs the owning user).
func sessionTitleAt(t *testing.T, pool *pgxpool.Pool, sessionID, want string) {
	t.Helper()
	var title string
	if err := pool.QueryRow(context.Background(),
		`SELECT title FROM ai_chat_sessions WHERE id = $1::uuid`, sessionID,
	).Scan(&title); err != nil {
		t.Fatalf("query title for %s: %v", sessionID, err)
	}
	if title != want {
		t.Fatalf("session %s title = %q, want %q", sessionID, title, want)
	}
}
