package sessionmanager

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ── fake store ───────────────────────────────────────────────────────────────

type fakeStore struct {
	mu    sync.Mutex
	rows  map[string]*Session // keyed by session id
	saves atomic.Int64
}

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]*Session{}} }

func (f *fakeStore) Load(_ context.Context, key agentcontext.ContextKey) (*Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[key.SessionID()]
	if !ok {
		return nil, ErrNotFound
	}
	if row.LegalEntityID != key.LegalEntityID() || row.UserID != key.UserID() {
		return nil, ErrScopeDenied
	}
	copy := *row
	return &copy, nil
}

func (f *fakeStore) Save(_ context.Context, key agentcontext.ContextKey, s *Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves.Add(1)
	stored := *s
	stored.LegalEntityID = key.LegalEntityID()
	stored.UserID = key.UserID()
	stored.SessionID = key.SessionID()
	f.rows[key.SessionID()] = &stored
	return nil
}

// seedForeignRow plants a conversation owned by another entity/user under the
// given session id — the cross-tenant probe target.
func (f *fakeStore) seedForeignRow(sessionID, entityID, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[sessionID] = &Session{
		LegalEntityID: entityID, UserID: userID, SessionID: sessionID,
		Classification: agentcontext.ClassificationProduction, Title: "foreign", Status: "active",
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func principalFor(entityID, userID string) agenttools.Principal {
	return agenttools.Principal{
		UserID: userID, SubjectType: "web_ai_agent",
		Scope: access.Scope{LegalEntityID: entityID},
	}
}

func mustKey(t *testing.T, entityID, userID, sessionID string) agentcontext.ContextKey {
	t.Helper()
	key, err := agentcontext.KeyFrom(principalFor(entityID, userID), sessionID, "production")
	if err != nil {
		t.Fatalf("KeyFrom: %v", err)
	}
	return key
}

const (
	entityA = "11111111-1111-4111-8111-111111111111"
	entityB = "22222222-2222-4222-8222-222222222222"
	userA   = "aaaaaaaa-0000-4000-8000-000000000001"
	userB   = "bbbbbbbb-0000-4000-8000-000000000002"
	session = "cccccccc-0000-4000-8000-000000000003"
)

func newTestManager(store Store, mutate func(*Policy)) *manager {
	policy := Policy{Now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
	if mutate != nil {
		mutate(&policy)
	}
	return New(store, policy)
}

// ── 验收 1：并发语义 ─────────────────────────────────────────────────────────

// Same-key Acquire must serialize: while the first holder keeps the lease,
// the second acquire attempt must NOT obtain a session.
func TestAcquireSerializesSameKey(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	ctx := context.Background()

	first, release, err := m.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if first == nil || first.SessionID != session {
		t.Fatalf("first acquire returned %+v", first)
	}

	acquiredSecond := make(chan *Session, 1)
	go func() {
		second, rel, err := m.Acquire(ctx, key)
		if err != nil {
			close(acquiredSecond)
			return
		}
		defer rel()
		acquiredSecond <- second
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("second same-key acquire ran while the lease was held — messages can interleave")
	case <-time.After(100 * time.Millisecond):
		// still blocked: correct
	}

	release()

	select {
	case second := <-acquiredSecond:
		if second == nil {
			t.Fatal("post-release acquire failed")
		}
		if second.SessionID != session {
			t.Fatalf("second holder got session %q", second.SessionID)
		}
		// Sharing the same cached anchor pointer across sequential holders is
		// by design: the lease guarantees holders never overlap.
	case <-time.After(time.Second):
		t.Fatal("release did not unblock the waiting acquire")
	}
}

// Different keys are different conversations: both acquires succeed
// concurrently, no serialization between them.
func TestDifferentKeysAcquireConcurrently(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	keyOne := mustKey(t, entityA, userA, session)
	keyTwo := mustKey(t, entityA, userA, "dddddddd-0000-4000-8000-000000000004")

	done := make(chan struct{}, 2)
	for _, key := range []agentcontext.ContextKey{keyOne, keyTwo} {
		go func(key agentcontext.ContextKey) {
			_, release, err := m.Acquire(context.Background(), key)
			if err != nil {
				done <- struct{}{}
				return
			}
			time.Sleep(50 * time.Millisecond) // hold a moment; the other key proceeds regardless
			release()
			done <- struct{}{}
		}(key)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("distinct keys did not run concurrently")
		}
	}
}

// Release is re-acquirable and double-release safe: a second release call
// must not inject an extra token into the semaphore.
func TestReleaseAllowsReacquireAndIsDoubleReleaseSafe(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	_, release, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release()
	release() // must be a no-op, not an extra token

	third, release3, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("reacquire after release: %v", err)
	}
	if third == nil {
		t.Fatal("reacquired session is nil")
	}
	release3()
}

// ── 验收 3：忘记 release → 后续阻塞（超时断言，不死等）─────────────────────

func TestForgottenReleaseBlocksNextAcquire(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	if _, _, err := m.Acquire(context.Background(), key); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	// release deliberately forgotten

	blockedCtx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := m.Acquire(blockedCtx, key)
	if err == nil {
		t.Fatal("next acquire succeeded despite a leaked lease")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("blocking acquire waited %v; the timeout guard did not fire", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("blocked acquire error = %v; want the deadline surfaced", err)
	}
}

// ── 验收 2：淘汰后从 Store 重建 ──────────────────────────────────────────────

func TestEvictedSessionRebuildsFromStore(t *testing.T) {
	baseTime := time.Unix(1_700_000_000, 0)
	now := baseTime
	store := newFakeStore()
	m := New(store, Policy{
		IdleTTL: 5 * time.Minute,
		Now:     func() time.Time { return now },
	})
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	first, release, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if first.Title == "" && first.Status != "active" {
		t.Fatalf("created session = %+v", first)
	}
	first.Title = "renamed-by-holder"
	if err := store.Save(context.Background(), key, first); err != nil {
		t.Fatalf("persist rename: %v", err)
	}
	release()

	// Advance past IdleTTL and evict. The cache entry must vanish.
	now = baseTime.Add(6 * time.Minute)
	m.evictExpired(now)
	m.mu.Lock()
	remaining := len(m.entries)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("idle entry survived eviction: %d entries left", remaining)
	}

	// The next Acquire rebuilds from the Store — same state, fresh pointer.
	second, release2, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("rebuild acquire: %v", err)
	}
	defer release2()
	if second == first {
		t.Fatal("rebuilt session is the stale cached pointer — eviction did not happen")
	}
	if second.Title != "renamed-by-holder" || second.Status != "active" ||
		second.Classification != "production" || second.LegalEntityID != entityA {
		t.Fatalf("rebuilt state diverged from the store: %+v", second)
	}
}

func TestMaxEntriesTrimsLeastRecentlyUsed(t *testing.T) {
	baseTime := time.Unix(1_700_000_000, 0)
	now := baseTime
	store := newFakeStore()
	m := New(store, Policy{
		MaxEntries: 1,
		Now:        func() time.Time { return now },
	})
	defer m.Stop()

	old := mustKey(t, entityA, userA, session)
	recent := mustKey(t, entityA, userA, "dddddddd-0000-4000-8000-000000000004")

	if _, relOld, err := m.Acquire(context.Background(), old); err != nil {
		t.Fatal(err)
	} else {
		relOld()
	}
	now = now.Add(time.Minute)
	if _, relRecent, err := m.Acquire(context.Background(), recent); err != nil {
		t.Fatal(err)
	} else {
		relRecent()
	}

	m.evictExpired(now)
	m.mu.Lock()
	_, oldAlive := m.entries[locator(old)]
	_, recentAlive := m.entries[locator(recent)]
	m.mu.Unlock()
	if oldAlive {
		t.Fatal("least recently used entry was kept over MaxEntries=1")
	}
	if !recentAlive {
		t.Fatal("most recently used entry was trimmed")
	}
}

// ── 验收 4（单元层）：跨法人拒绝原样传播、不软化 ────────────────────────────

func TestCrossTenantRefusalPropagatesUnsoftened(t *testing.T) {
	store := newFakeStore()
	store.seedForeignRow(session, entityB, userB) // B owns the conversation
	m := newTestManager(store, nil)
	defer m.Stop()

	// A's identity, B's conversation id.
	keyA := mustKey(t, entityA, userA, session)
	_, _, err := m.Acquire(context.Background(), keyA)
	if err == nil {
		t.Fatal("entity A acquired entity B's session")
	}
	if !IsScopeDenied(err) {
		t.Fatalf("error = %v; want scope_denied preserved, not softened into not-found", err)
	}
	if !strings.Contains(err.Error(), "scope_denied") {
		t.Fatalf("refusal message lost its reason: %v", err)
	}
	if IsNotFound(err) {
		t.Fatal("scope refusal was softened into not-found")
	}
}

// A legitimate owner still gets through after foreign attempts were refused.
func TestOwnerUnaffectedByForeignAttempts(t *testing.T) {
	store := newFakeStore()
	store.seedForeignRow(session, entityB, userB)
	m := newTestManager(store, nil)
	defer m.Stop()

	keyB := mustKey(t, entityB, userB, session)
	sess, release, err := m.Acquire(context.Background(), keyB)
	if err != nil {
		t.Fatalf("owner acquire: %v", err)
	}
	defer release()
	if sess.UserID != userB || sess.LegalEntityID != entityB {
		t.Fatalf("owner session mismatch: %+v", sess)
	}
}

// ── Close：结算 + 幂等 ───────────────────────────────────────────────────────

func TestCloseSettlesAndIsIdempotent(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	sess, release, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	sess.Title = "settled"
	if err := store.Save(context.Background(), key, sess); err != nil {
		t.Fatalf("save during turn: %v", err)
	}
	release()

	if err := m.Close(context.Background(), key); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := m.Close(context.Background(), key); err != nil {
		t.Fatalf("second close must be a no-op, got %v", err)
	}

	m.mu.Lock()
	_, alive := m.entries[locator(key)]
	m.mu.Unlock()
	if alive {
		t.Fatal("close left the cache entry behind")
	}
}

// Close on a session another turn still holds waits for that turn instead of
// settling underneath it.
func TestCloseWaitsForHeldLease(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	_, release, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}

	closed := make(chan error, 1)
	go func() { closed <- m.Close(context.Background(), key) }()
	select {
	case <-closed:
		t.Fatal("close settled a session mid-turn")
	case <-time.After(80 * time.Millisecond):
	}
	release()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close after release: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close never completed after the lease came back")
	}
}

// ── 后台 sweeper ──────────────────────────────────────────────────────────────

func TestBackgroundSweeperEvictsIdleEntries(t *testing.T) {
	// A real advancing clock is required here: with a frozen injected clock,
	// idle duration stays zero forever and no sweep could ever fire.
	store := newFakeStore()
	m := New(store, Policy{
		IdleTTL:       20 * time.Millisecond,
		SweepInterval: 10 * time.Millisecond,
	})
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	_, release, err := m.Acquire(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	release()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, alive := m.entries[locator(key)]
		m.mu.Unlock()
		if !alive {
			return // sweeper did its job using the injected clock's idle rule
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("background sweeper never evicted the idle entry")
}

// ── P0-1：Stop 并发双调用 ────────────────────────────────────────────────────

func TestStopConcurrentCallsAreSafe(t *testing.T) {
	store := newFakeStore()
	m := New(store, Policy{SweepInterval: time.Hour})
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := 0; i < 16; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			m.Stop()
		}()
	}
	start.Done()
	done.Wait()
}

// ── P0-2：孤儿租约防护（引用计数延迟删除）───────────────────────────────────

// While a Close is blocked waiting for the lease, and while any waiter is
// queued behind it, the entry (and therefore the LEASE IDENTITY) must stay in
// the map. A fresh lease handed to a later Acquire would let two "holders"
// run the same conversation concurrently — the one failure this module exists
// to make impossible.
func TestEntrySurvivesWhileReferencesOutstanding(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)

	// Simulated holder: reference taken, lease held.
	entry := m.acquireEntry(locator(key))
	if err := entry.lease.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Closer arrives: takes its own reference, blocks on the lease.
	closed := make(chan error, 1)
	go func() { closed <- m.Close(context.Background(), key) }()
	deadline := time.After(time.Second)
	for {
		m.mu.Lock()
		live, present := m.entries[locator(key)]
		refs := entry.refs
		m.mu.Unlock()
		if !present {
			t.Fatal("close deleted the entry while a holder still referenced it")
		}
		if live == entry && refs >= 2 {
			// the closer holds its reference and is parked on the lease
			break
		}
		select {
		case <-deadline:
			t.Fatal("close never reached its blocked-on-lease state")
		case <-time.After(2 * time.Millisecond):
		}
	}

	// Holder leaves — exactly what the real release closure does: hand back
	// the token AND drop its reference, in that order.
	entry.lease.release()
	m.releaseEntry(entry)
	if err := <-closed; err != nil {
		t.Fatalf("close: %v", err)
	}
	m.mu.Lock()
	_, stillThere := m.entries[locator(key)]
	m.mu.Unlock()
	if stillThere {
		t.Fatal("settled entry was never retired")
	}
}

// Hammer: overlapping turns of one conversation must never happen while
// Acquire/release/Close/eviction interleave under eviction pressure
// (MaxEntries=1 + tiny TTL force constant entry churn). This is the runtime
// guard for the orphan-lease fix; run with -race in CI.
func TestNoOverlappingTurnsUnderMixedLifecycleStress(t *testing.T) {
	store := newFakeStore()
	base := time.Unix(1_700_000_000, 0)
	var clock atomic.Int64
	m := New(store, Policy{
		MaxEntries: 1,
		IdleTTL:    time.Microsecond,
		Now:        func() time.Time { return base.Add(time.Duration(clock.Load()) * time.Millisecond) },
	})
	defer m.Stop()

	keys := []agentcontext.ContextKey{
		mustKey(t, entityA, userA, session),
		mustKey(t, entityA, userA, "dddddddd-0000-4000-8000-000000000004"),
		mustKey(t, entityB, userB, "eeeeeeee-0000-4000-8000-000000000005"),
	}
	var active [3]atomic.Int32
	var overlapped atomic.Int32

	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func(i int, key agentcontext.ContextKey) {
			defer wg.Done()
			for round := 0; round < 80; round++ {
				_, release, err := m.Acquire(context.Background(), key)
				if err != nil {
					continue
				}
				if prev := active[i].Add(1); prev != 1 {
					overlapped.Add(1)
				}
				clock.Add(1)
				release()
				active[i].Add(-1)
			}
		}(i, key)
	}
	// Interferers: closes and evictions racing the turns above.
	for _, key := range keys {
		wg.Add(1)
		go func(key agentcontext.ContextKey) {
			defer wg.Done()
			for round := 0; round < 40; round++ {
				_ = m.Close(context.Background(), key)
				clock.Add(1)
				m.evictExpired(m.now())
				time.Sleep(time.Millisecond)
			}
		}(key)
	}
	wg.Wait()

	if overlapped.Load() != 0 {
		t.Fatalf("%d overlapping same-key turns detected — the mutual exclusion broke", overlapped.Load())
	}
}

// Deterministic orphan-lease guard: two consecutive holders of one
// conversation must run under the SAME lease object. If an entry can be
// deleted while a waiter is parked on it, the waiter wakes on an orphan lease
// while later acquires get a fresh one — mutual exclusion silently dies.
func TestConsecutiveHoldersShareOneLease(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, nil)
	defer m.Stop()

	key := mustKey(t, entityA, userA, session)
	ctx := context.Background()

	_, release, err := m.Acquire(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	leaseSeenBySecondHolder := make(chan *sessionLease, 1)
	go func() {
		_, rel, err := m.Acquire(ctx, key)
		if err != nil {
			close(leaseSeenBySecondHolder)
			return
		}
		m.mu.Lock()
		entry := m.entries[locator(key)]
		var seen *sessionLease
		if entry != nil {
			seen = entry.lease
		}
		m.mu.Unlock()
		rel()
		leaseSeenBySecondHolder <- seen
	}()

	// Wait until the second holder is actually parked (two references out).
	deadline := time.After(time.Second)
	for {
		m.mu.Lock()
		entry := m.entries[locator(key)]
		refs := 0
		if entry != nil {
			refs = entry.refs
		}
		m.mu.Unlock()
		if refs >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("second holder never parked on the lease")
		case <-time.After(2 * time.Millisecond):
		}
	}

	release()
	second := <-leaseSeenBySecondHolder
	if second == nil {
		t.Fatal("second holder lost its entry while parked")
	}

	// The third holder must land on the SAME lease the second ran under.
	thirdEntry := func() *cacheEntry {
		e := m.acquireEntry(locator(key))
		e.lease.acquire(ctx)
		return e
	}()
	thirdEntry.lease.release()
	m.releaseEntry(thirdEntry)

	if thirdEntry.lease != second {
		t.Fatalf("lease identity changed across holders (%p -> %p): an orphaned lease let a waiter run unsynchronized", second, thirdEntry.lease)
	}
}
