package scheduler

// RT1-L3-C 集成测试：lease recovery job 的端到端行为（带库实跑；SKIP 不构成证据）。
//
// 锁定的验收：
//  1. 过期租约的 run 被一轮调度拉回 queued，worker/lease 字段清空，
//     queue_update（reason=lease_expired）trace 落库——调度痕迹与用户发起可区分；
//  2. 同一触发时刻第二次执行：0 行、无第二条 trace——幂等在行级状态机上
//     （WHERE status='running' AND leased_until < NOW()），不在请求键里；
//  3. **status 谓词单独承担正确性职责**（复验发现，2026-08-26）：completed +
//     过期 leased_until 是可达状态——UpdateClaimedRunStatus 只写 status 不清
//     租约，ReleaseRunLease 才清。recovery 不得复活非 running 的 run，否则已
//     完成的工作会被重跑。变异自检：去掉 status 谓词，本文件
//     TestLeaseRecoveryJobDoesNotResurrectCompletedRuns 必须变红。
//     注：三个谓词对「同刻双发只执行一次」确实互为冗余，但 status 谓词的
//     正确性职责无人兜底——它不是防御性冗余。
//
// 跨法人说明（决策 D39）：本 job 有意不按法人过滤——它修的是队列健康这一全局
// 状态，只重置 status/lease 列、不读任何租户业务字段。测试用两个法人各造一条
// 过期租约并断言都被回收，正是这条豁免的行为化表达。

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/repository"
)

// postgresPool connects to the real test database named by TEST_DATABASE_URL
// and is skipped when unset. SKIP DOES NOT COUNT AS EVIDENCE: the recovery
// proof below only means something when these tests actually RUN
// (make test-integration).
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

// shortID gives seeding a short unique suffix that fits VARCHAR(50) codes.
func shortID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])[:10]
}

func seedExpiredLeaseRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) (entityID, userID, sessionID, runID string) {
	t.Helper()
	suffix := shortID() + "-" + label
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id`,
		"l3c-"+suffix, "L3C entity "+suffix,
	).Scan(&entityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role, legal_entity_id)
		VALUES ($1, $2, 'x', 'editor', $3) RETURNING id`,
		"l3c-"+suffix, "l3c-"+suffix+"@test.local", entityID,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions (user_id, legal_entity_id) VALUES ($1, $2) RETURNING id`,
		userID, entityID,
	).Scan(&sessionID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO ai_chat_runs (session_id, status, worker_id, lease_token, leased_until, heartbeat_at)
		VALUES ($1, 'running', 'worker-gone', 'lease-token-x', NOW() - INTERVAL '5 minutes', NOW() - INTERVAL '6 minutes')
		RETURNING id`,
		sessionID,
	).Scan(&runID); err != nil {
		t.Fatalf("seed expired-lease run: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE id = $1`, sessionID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = $1`, entityID)
	})
	return entityID, userID, sessionID, runID
}

func runState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, runID string) (status string, worker *string, leaseToken *string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		SELECT status, worker_id, lease_token FROM ai_chat_runs WHERE id = $1`, runID,
	).Scan(&status, &worker, &leaseToken)
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	return status, worker, leaseToken
}

func countLeaseExpiredEvents(t *testing.T, ctx context.Context, pool *pgxpool.Pool, runID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM ai_chat_run_events
		WHERE run_id = $1 AND event_type = 'queue_update'
		  AND payload->>'reason' = 'lease_expired'`, runID,
	).Scan(&n); err != nil {
		t.Fatalf("count queue_update events: %v", err)
	}
	return n
}

func TestLeaseRecoveryJobRequeuesExpiredRunOnce(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	// 两个法人各一条过期租约——Type A 的跨法人豁免是定义本身（D39），行为化断言。
	_, _, _, runA := seedExpiredLeaseRun(t, ctx, pool, "entA")
	_, _, _, runB := seedExpiredLeaseRun(t, ctx, pool, "entB")

	queue := repository.NewAgentRunQueueRepository(pool)
	job := LeaseRecovery(queue, time.Minute)

	reg := job
	if reg.Auth != AuthSystemMaintenance {
		t.Fatalf("lease recovery must be declared system_maintenance (no tools, no runs), got %q", reg.Auth)
	}

	// 第一轮：两条都回队列。
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("first recovery tick failed: %v", err)
	}
	for _, id := range []string{runA, runB} {
		status, worker, token := runState(t, ctx, pool, id)
		if status != "queued" || worker != nil || token != nil {
			t.Fatalf("run %s must be requeued with lease cleared, got status=%s worker=%v token=%v", id, status, worker, token)
		}
		if n := countLeaseExpiredEvents(t, ctx, pool, id); n != 1 {
			t.Fatalf("exactly one lease_expired trace expected for %s, got %d", id, n)
		}
	}

	// 第二轮（同一触发时刻语义）：行级状态机幂等 → 0 行、无重复 trace。
	recovered, err := queue.RecoverExpiredRunLeases(ctx)
	if err != nil {
		t.Fatalf("second recovery tick failed: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("immediate second recovery must find nothing to do (idempotent row-state guard), recovered %d", recovered)
	}
	for _, id := range []string{runA, runB} {
		if n := countLeaseExpiredEvents(t, ctx, pool, id); n != 1 {
			t.Fatalf("duplicate lease_expired trace for %s after second tick: %d", id, n)
		}
	}
}

// 复验缺陷回归（RT1-L3-C review，2026-08-26）：completed + 过期 leased_until
// 是可达状态。可达路径：worker 把 run 标成 completed 之后、释放租约之前崩溃
// ——UpdateClaimedRunStatus 只写 status 不清租约，ReleaseRunLease 才清。
// status 谓词是唯一挡住「已完成的工作被重跑」的东西，它承重而非冗余。
func TestLeaseRecoveryJobDoesNotResurrectCompletedRuns(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	_, _, _, runID := seedExpiredLeaseRun(t, ctx, pool, "done")
	// 走 UpdateClaimedRunStatus 的真实语义到同一终态：completed 但租约原样挂着
	// （随后自然过期）。
	if _, err := pool.Exec(ctx, `
		UPDATE ai_chat_runs SET status = 'completed', completed_at = NOW(),
		       summary_text = 'done before worker crashed'
		WHERE id = $1`, runID); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	queue := repository.NewAgentRunQueueRepository(pool)
	if err := LeaseRecovery(queue, time.Minute).Run(ctx); err != nil {
		t.Fatalf("recovery tick failed: %v", err)
	}

	status, worker, token := runState(t, ctx, pool, runID)
	if status != "completed" || worker == nil || token == nil {
		t.Fatalf("a completed run must not be resurrected by recovery — finished work reruns double-executes, got status=%s worker=%v token=%v", status, worker, token)
	}
	if n := countLeaseExpiredEvents(t, ctx, pool, runID); n != 0 {
		t.Fatalf("no lease_expired trace may be written for a completed run, got %d", n)
	}
}

// 未过期租约不得被误伤——幂等谓词的另一面。
func TestLeaseRecoveryJobIgnoresLiveLeases(t *testing.T) {
	pool := postgresPool(t)
	ctx := context.Background()

	_, _, _, runID := seedExpiredLeaseRun(t, ctx, pool, "live")
	// 改成未过期的健康租约。
	if _, err := pool.Exec(ctx, `
		UPDATE ai_chat_runs SET leased_until = NOW() + INTERVAL '10 minutes',
		       heartbeat_at = NOW(), status = 'running'
		WHERE id = $1`, runID); err != nil {
		t.Fatalf("re-arm live lease: %v", err)
	}

	queue := repository.NewAgentRunQueueRepository(pool)
	if err := LeaseRecovery(queue, time.Minute).Run(ctx); err != nil {
		t.Fatalf("recovery tick failed: %v", err)
	}
	status, worker, token := runState(t, ctx, pool, runID)
	if status != "running" || worker == nil || token == nil {
		t.Fatalf("live lease must not be touched, got status=%s worker=%v token=%v", status, worker, token)
	}
}
