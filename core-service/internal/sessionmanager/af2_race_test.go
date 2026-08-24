package sessionmanager

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ── AF2：并发 Acquire/Close 的数据竞争 ──────────────────────────────────────
//
// AR2's test surface never called Close concurrently — the module exists for
// concurrent ownership, yet this cell was empty, and the race the review
// found (entry.closing / entry.refs read outside m.mu in Close) lived exactly
// there. This test drives 32 goroutines through interleaved Acquire/Close on
// the SAME key; under -race it must be clean.
//
// Mutation check: removing the m.mu critical section around Close's
// check-and-set must bring the race warning back.

func TestConcurrentAcquireCloseIsRaceFree(t *testing.T) {
	store := newFakeStore()
	m := newTestManager(store, func(p *Policy) { p.SweepInterval = 0 })
	key := mustKey(t, entityA, userA, session)

	const goroutines = 32
	const rounds = 25

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < rounds; i++ {
				session, release, err := m.Acquire(ctx, key)
				if err != nil {
					t.Errorf("goroutine %d acquire: %v", g, err)
					return
				}
				if session == nil || release == nil {
					t.Errorf("goroutine %d: nil acquire result", g)
					return
				}
				release()
				// Interleave closes from everyone: the second-and-later
				// closers must no-op instead of racing the settler.
				if err := m.Close(ctx, key); err != nil {
					t.Errorf("goroutine %d close: %v", g, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	// The manager must still be fully usable afterwards: a fresh Acquire
	// rebuilds from the store and works, proving no entry got corrupted or
	// permanently wedged by the racing closes.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, release, err := m.Acquire(ctx, key)
	if err != nil {
		t.Fatalf("post-race acquire: %v", err)
	}
	if s == nil {
		t.Fatal("post-race acquire returned nil session")
	}
	release()
	if err := m.Close(ctx, key); err != nil {
		t.Fatalf("post-race close: %v", err)
	}
	m.Stop()
}
