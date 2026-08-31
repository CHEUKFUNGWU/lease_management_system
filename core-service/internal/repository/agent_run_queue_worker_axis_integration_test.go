package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

// RT1-B worker 轴断言：worker 只能访问自己持租约的那一个 run。
//
// worker 是受信任机器身份（agent_runtime:worker 权限），租约边界 =
// worker_id + lease_token 绑定到具体 run row。本测试证明的是这个更弱的命题：
// 两个不同法人的 run 各被不同 worker 租走，各自的 GetClaimedRun 只能取自己
// 的 run；用 A 的租约取 B 的 run → ErrAgentRunLeaseLost。
//
// 注意不要把本测试往强了读：它证的不是「worker 池不跨法人」——
// ClaimQueuedRun 从共享队列按 created_at 取队，一个 worker 能连续领到
// 不同法人的 run（部署级信任的选择，不是结构保证）。本测试只证「租约绑死
// 具体 run row，claim 不改所有权」。
//
// 注：这不是把 SI2 的用户法人轴套到 worker 上——worker 没有会话用户语义，
// 它是部署级信任的通用执行器。worker→法人绑定的授权策略（如需分租户 worker）
// 是独立开放决策（未随 G9 关闭，见 tmp/delivery-RT1.md）。跑 make test-integration 实跑。
func TestClaimedRunLeaseBindsTheRunRowAcrossEntities(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()

	// 两个法人、一个横跨用户（SI1 形状），各一个会话 + 一个 queued run。
	// created_at 使用一个远早于测试数据的固定时间并显式拉开一分钟：
	// 测试不清理共享数据库中的其他队列行，也不会误删别人的工作；FIFO
	// 断言由时间戳钉死，而不是由时钟精度或残留行决定。
	entityA, entityB, userID := seedChatIsolationTenants(t, ctx, pool, "wax")
	sessionA := seedChatSession(t, ctx, pool, userID, &entityA, "A 会话")
	sessionB := seedChatSession(t, ctx, pool, userID, &entityB, "B 会话")

	mkRun := func(sessionID string, createdAt time.Time) string {
		t.Helper()
		var runID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO ai_chat_runs (session_id, status, agent_mode, created_at)
			VALUES ($1, 'queued', true, $2) RETURNING id`, sessionID, createdAt).Scan(&runID); err != nil {
			t.Fatalf("seed queued run: %v", err)
		}
		return runID
	}
	queueBase := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	runA := mkRun(sessionA, queueBase)
	runB := mkRun(sessionB, queueBase.Add(time.Minute))
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_runs WHERE session_id = ANY($1::uuid[])`, []string{sessionA, sessionB})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`, []string{entityA, entityB})
	})

	queue := NewAgentRunQueueRepository(pool)

	// 两个 worker 各租一个 run：worker-a 租 runA，worker-b 租 runB。
	claimedA, tokenA, err := queue.ClaimQueuedRun(ctx, "worker-a", time.Minute)
	if err != nil || claimedA == nil || claimedA.ID != runA {
		t.Fatalf("worker-a claim: run=%+v err=%v (queue is FIFO oldest-first)", claimedA, err)
	}
	claimedB, tokenB, err := queue.ClaimQueuedRun(ctx, "worker-b", time.Minute)
	if err != nil || claimedB == nil || claimedB.ID != runB {
		t.Fatalf("worker-b claim: run=%+v err=%v", claimedB, err)
	}

	// 各 worker 取自己的 run：成功。
	if got, err := queue.GetClaimedRun(ctx, runA, "worker-a", tokenA); err != nil || got == nil || got.ID != runA {
		t.Fatalf("worker-a owns runA: err=%v", err)
	}
	if got, err := queue.GetClaimedRun(ctx, runB, "worker-b", tokenB); err != nil || got == nil || got.ID != runB {
		t.Fatalf("worker-b owns runB: err=%v", err)
	}

	// 交叉：worker-a 拿 runA 的租约去取 runB → lease lost（run row 不匹配）。
	// 证实的是「worker 只能访问自己持租约的那个 run」，不是「跨法人不可能」——
	// worker 池本身不按法人分区（部署级信任的选择），租约只绑死具体 run row。
	if _, err := queue.GetClaimedRun(ctx, runB, "worker-a", tokenA); !errors.Is(err, ErrAgentRunLeaseLost) {
		t.Fatalf("worker-a with runA lease fetched runB: err=%v; want ErrAgentRunLeaseLost", err)
	}
	if _, err := queue.GetClaimedRun(ctx, runA, "worker-b", tokenB); !errors.Is(err, ErrAgentRunLeaseLost) {
		t.Fatalf("worker-b with runB lease fetched runA: err=%v; want ErrAgentRunLeaseLost", err)
	}
}

// TestClaimedRunSessionEntityIsRunBoundary pins that the claimed run's session
// carries the entity it was created under — the claim does not alter ownership.
func TestClaimedRunSessionEntityIsRunBoundary(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()

	entityA, _, userID := seedChatIsolationTenants(t, ctx, pool, "ownr")
	repo := NewAIChatRuntimeRepository(pool)
	sessionA := seedChatSession(t, ctx, pool, userID, &entityA, "A 会话")
	var runID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_runs (session_id, status, agent_mode, created_at)
		VALUES ($1, 'queued', true, $2) RETURNING id`, sessionA, time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)).Scan(&runID); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_runs WHERE id = $1`, runID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = $1`, entityA)
	})

	queue := NewAgentRunQueueRepository(pool)
	claimed, token, err := queue.ClaimQueuedRun(ctx, "worker-a", time.Minute)
	if err != nil || claimed == nil || claimed.ID != runID {
		t.Fatalf("claim: %+v err=%v", claimed, err)
	}
	// 租约取回后，run 的 session 法人仍是创建时的 A——claim 不改所有权。
	ownerRun, err := queue.GetClaimedRun(ctx, runID, "worker-a", token)
	if err != nil {
		t.Fatalf("get claimed: %v", err)
	}
	sess, err := repo.GetSessionByID(ctx, ownerRun.SessionID, userID, access.GlobalEntityFilter())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if sess.LegalEntityID == nil || *sess.LegalEntityID != entityA {
		t.Fatalf("claimed run's session entity = %v; want %s (claim must not change ownership)", sess.LegalEntityID, entityA)
	}
}
