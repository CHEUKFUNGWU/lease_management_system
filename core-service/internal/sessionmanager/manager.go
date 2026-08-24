package sessionmanager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// Policy configures the module's internal eviction (D-C13: eviction policy is
// a constructor parameter, never an interface method).
type Policy struct {
	// IdleTTL evicts cached entries whose lease has been released for longer
	// than this. 0 disables idle eviction.
	IdleTTL time.Duration
	// MaxEntries caps the cache; the least-recently-used unlocked entries go
	// first. 0 means unbounded.
	MaxEntries int
	// SweepInterval > 0 starts the internal background sweeper. Tests usually
	// leave it 0 and drive evictExpired through Stop-less manual calls.
	SweepInterval time.Duration
	// Now overrides the clock. Nil means time.Now.
	Now func() time.Time
}

// manager is the only Manager implementation. Its maps hold leases and cached
// session anchors — that state IS the module's job (D-C4 keeps concurrency
// inside the module), and no entry ever carries conversation content: a cache
// entry is ownership metadata plus a pointer to what the Store already holds.
type manager struct {
	store  Store
	policy Policy

	mu       sync.Mutex
	entries  map[string]*cacheEntry
	stopOnce sync.Once
	stopped  chan struct{}
	sweepWG  sync.WaitGroup
}

// cacheEntry is the per-locator lifecycle record. Its lease object is the
// single mutex of that conversation: entries leave the map ONLY at refs==0,
// so a waiter blocked on the lease can never wake up holding an orphan while
// a fresh entry hands out a second lease for the same conversation. Every
// holder, waiter and closer increments refs before leaving m.mu.
type cacheEntry struct {
	loc      string
	lease    *sessionLease
	session  *Session // nil until first Acquire under this entry loads/creates it
	lastUsed time.Time
	refs     int
	closing  bool
}

// sessionLease is a binary semaphore. The token lives in ch while the lease
// is FREE; acquiring takes it out. A held lease therefore blocks the next
// acquire on the same key until release puts the token back.
type sessionLease struct {
	ch chan struct{}
}

func newSessionLease() *sessionLease {
	l := &sessionLease{ch: make(chan struct{}, 1)}
	l.ch <- struct{}{}
	return l
}

func (l *sessionLease) acquire(ctx context.Context) error {
	select {
	case <-l.ch:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("acquire session lease: %w", ctx.Err())
	}
}

func (l *sessionLease) release() {
	l.ch <- struct{}{}
}

// held reports whether the lease is currently taken. The token sits in ch
// while the lease is FREE, so "can receive" means free; the probe puts the
// token straight back to stay side-effect free.
func (l *sessionLease) held() bool {
	select {
	case <-l.ch:
		l.ch <- struct{}{}
		return false
	default:
		return true
	}
}

// New constructs the manager. The concrete return type exposes Stop, which
// deliberately sits OUTSIDE the Manager interface (D-C13): lifecycle of the
// internal sweeper is wiring concern, not something callers of Acquire/Close
// should know about.
func New(store Store, policy Policy) *manager {
	if policy.Now == nil {
		policy.Now = time.Now
	}
	m := &manager{
		store:   store,
		policy:  policy,
		entries: map[string]*cacheEntry{},
		stopped: make(chan struct{}),
	}
	if policy.SweepInterval > 0 {
		m.sweepWG.Add(1)
		go m.sweepLoop()
	}
	return m
}

// Stop terminates the internal sweeper. Safe to call multiple times and from
// concurrent goroutines: the once-guarded close is what makes that promise
// true — a bare select-default check-then-close panicked under a 16-goroutine
// race (proven red at -count=200 -cpu=8 before this guard existed).
func (m *manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopped) })
	m.sweepWG.Wait()
}

func (m *manager) sweepLoop() {
	defer m.sweepWG.Done()
	ticker := time.NewTicker(m.policy.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.evictExpired(m.policy.Now())
		case <-m.stopped:
			return
		}
	}
}

// acquireEntry returns the live entry for loc with one reference taken. The
// reference pins the entry (and its lease identity) against eviction and
// Close-driven deletion until releaseEntry lets it go.
func (m *manager) acquireEntry(loc string) *cacheEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.entries[loc]
	if entry == nil {
		entry = &cacheEntry{loc: loc, lease: newSessionLease()}
		m.entries[loc] = entry
	}
	entry.refs++
	entry.lastUsed = m.now()
	return entry
}

// releaseEntry drops one reference and performs the deferred deletion a
// closer asked for once the last reference is gone.
func (m *manager) releaseEntry(entry *cacheEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.refs--
	if entry.refs <= 0 && entry.closing {
		delete(m.entries, entry.loc)
	}
}

// Acquire implements Manager. See interface docs for the contract.
func (m *manager) Acquire(ctx context.Context, key agentcontext.ContextKey) (*Session, func(), error) {
	entry := m.acquireEntry(locator(key))

	if err := entry.lease.acquire(ctx); err != nil {
		m.releaseEntry(entry)
		return nil, nil, err
	}

	session, err := m.sessionFor(ctx, key, entry)
	if err != nil {
		entry.lease.release()
		m.releaseEntry(entry)
		return nil, nil, err
	}

	var once sync.Once
	release := func() {
		once.Do(func() {
			entry.lease.release()
			m.releaseEntry(entry)
		})
	}
	return session, release, nil
}

// sessionFor resolves the session behind a held lease: cache hit, Store load,
// or Store create — in that order. The entry's reference pins it in the map,
// so the write-back below cannot land on a deleted entry. Ownership was
// enforced by the Store at the data boundary; ErrScopeDenied and every other
// store failure propagate verbatim — never softened into "not found".
func (m *manager) sessionFor(ctx context.Context, key agentcontext.ContextKey, entry *cacheEntry) (*Session, error) {
	if entry.session != nil {
		return entry.session, nil
	}
	loaded, err := m.store.Load(ctx, key)
	switch {
	case err == nil:
	case IsNotFound(err):
		created := newSessionFromKey(key, m.now())
		if saveErr := m.store.Save(ctx, key, created); saveErr != nil {
			return nil, fmt.Errorf("create ai chat session %s: %w", key.SessionID(), saveErr)
		}
		loaded = created
	default:
		return nil, err
	}
	m.mu.Lock()
	entry.session = loaded
	entry.lastUsed = m.now()
	m.mu.Unlock()
	return loaded, nil
}

// IsNotFound reports whether err is the port's not-found signal.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsScopeDenied reports whether err is the port's scope_denied refusal.
func IsScopeDenied(err error) bool { return errors.Is(err, ErrScopeDenied) }

func newSessionFromKey(key agentcontext.ContextKey, now time.Time) *Session {
	return &Session{
		LegalEntityID:  key.LegalEntityID(),
		UserID:         key.UserID(),
		SessionID:      key.SessionID(),
		Classification: key.Classification(),
		Title:          "新会话",
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Close implements Manager: settle the cached anchor back to the Store, then
// retire the entry once the last reference drops. Idempotent: closing an
// unknown locator — or one already closing with no references — is a no-op.
//
// The reference is taken BEFORE waiting on the lease: a closer therefore can
// never delete an entry a waiter is still blocked on (the orphan-lease race —
// waiter wakes on an old lease while fresh Acquires get a new one — is what
// this ordering makes unreachable).
func (m *manager) Close(ctx context.Context, key agentcontext.ContextKey) error {
	loc := locator(key)
	entry := m.acquireEntry(loc)
	if entry.closing && entry.refs > 1 {
		// A concurrent Close already owns settlement; treat as done.
		m.releaseEntry(entry)
		return nil
	}

	if err := entry.lease.acquire(ctx); err != nil {
		m.releaseEntry(entry)
		return err // someone else is mid-turn on this conversation
	}
	defer func() {
		entry.lease.release()
		m.releaseEntry(entry)
	}()

	m.mu.Lock()
	session := entry.session
	entry.closing = true
	m.mu.Unlock()

	if session == nil {
		return nil
	}
	return m.store.Save(ctx, key, session)
}

var _ Manager = (*manager)(nil)

// evictExpired drops cached entries idle beyond IdleTTL and trims to
// MaxEntries by last use. Entries with a HELD lease are never evicted: the
// turn holding them is still running. Eviction only discards the cache — the
// Store remains the source of truth, so a later Acquire rebuilds identical
// state from disk.
func (m *manager) evictExpired(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for loc, entry := range m.entries {
		if entry.refs > 0 || entry.lease.held() || entry.closing {
			continue
		}
		if m.policy.IdleTTL > 0 && now.Sub(entry.lastUsed) >= m.policy.IdleTTL {
			delete(m.entries, loc)
		}
	}
	if m.policy.MaxEntries > 0 && len(m.entries) > m.policy.MaxEntries {
		type victim struct {
			loc      string
			lastUsed time.Time
		}
		idle := make([]victim, 0, len(m.entries))
		for loc, entry := range m.entries {
			if entry.refs == 0 && !entry.lease.held() && !entry.closing {
				idle = append(idle, victim{loc, entry.lastUsed})
			}
		}
		for i := 0; i < len(idle)-1; i++ {
			for j := i + 1; j < len(idle); j++ {
				if idle[j].lastUsed.Before(idle[i].lastUsed) {
					idle[i], idle[j] = idle[j], idle[i]
				}
			}
		}
		excess := len(m.entries) - m.policy.MaxEntries
		for i := 0; i < excess && i < len(idle); i++ {
			delete(m.entries, idle[i].loc)
		}
	}
}

func (m *manager) now() time.Time {
	if m.policy.Now != nil {
		return m.policy.Now()
	}
	return time.Now()
}
