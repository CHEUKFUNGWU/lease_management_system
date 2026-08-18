// Package agentguard is the AI budget module (design M6.3, PRD P3-26/27/30):
// per-user rate (messages per minute), daily cost ceiling, and bounded
// history assembly live behind one Guard so every chat entry (Web chat now,
// agent-runner aligned later) enforces the same discipline. Rejections carry
// their reason verbatim — a 429 is never softened into "no data".
package agentguard

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrRateLimited / ErrCostExceeded are the two refusal reasons; callers map
// them to HTTP 429 with the message preserved.
var (
	ErrRateLimited  = errors.New("rate limit exceeded")
	ErrCostExceeded = errors.New("daily cost limit exceeded")
)

// Config carries the enforcement knobs. Zero values collapse to defaults.
// The daily cost ceiling is enforced by the BudgetStore (its constructor
// receives the limit) and is deliberately absent here — a knob nobody reads
// would only imply it did something.
type Config struct {
	PerMinuteMessages   int     // default 12 — used in the rate-limit message
	CostPerTokenUSD     float64 // default 2e-6 (~$2 per 1M tokens)
	HistoryMessageLimit int     // default 20 — client history kept, oldest dropped
	HistoryCharBudget   int     // default 40_000 chars across the kept history
}

// DefaultConfig returns the product defaults.
func DefaultConfig() Config {
	return Config{PerMinuteMessages: 12, CostPerTokenUSD: 2e-6, HistoryMessageLimit: 20, HistoryCharBudget: 40_000}
}

func (c Config) withDefaults() Config {
	defaults := DefaultConfig()
	if c.PerMinuteMessages <= 0 {
		c.PerMinuteMessages = defaults.PerMinuteMessages
	}
	if c.CostPerTokenUSD <= 0 {
		c.CostPerTokenUSD = defaults.CostPerTokenUSD
	}
	if c.HistoryMessageLimit <= 0 {
		c.HistoryMessageLimit = defaults.HistoryMessageLimit
	}
	if c.HistoryCharBudget <= 0 {
		c.HistoryCharBudget = defaults.HistoryCharBudget
	}
	return c
}

// BudgetStore is the counting seam: the memory adapter serves single
// instances and tests, the DB adapter spans instances (two adapters ⇒ a real
// seam). Consume atomically checks the user's rate and daily cost ceiling
// and books the usage event in one step — a concurrent caller can never
// slip past the ceiling between a check and a record.
type BudgetStore interface {
	Consume(ctx context.Context, userID, kind string, tokens int, costUSD float64) (bool, string, error)
}

// Guard is the enforced view over a store.
type Guard struct {
	store BudgetStore
	cfg   Config
}

func New(store BudgetStore, cfg Config) *Guard {
	return &Guard{store: store, cfg: cfg.withDefaults()}
}

// Consume atomically checks the budget and books the event; the reason is
// preserved for the caller's 429 body. tokens may be 0 when the caller
// cannot observe them (rate counting still applies, cost accrues once token
// usage is plumbed).
func (g *Guard) Consume(ctx context.Context, userID, kind string, tokens int) error {
	allowed, reason, err := g.store.Consume(ctx, userID, kind, tokens, float64(tokens)*g.cfg.CostPerTokenUSD)
	if err != nil {
		return err
	}
	if !allowed {
		switch reason {
		case "cost":
			return fmt.Errorf("%w for user %s (%s): %s", ErrCostExceeded, userID, kind, reason)
		default:
			return fmt.Errorf("%w for user %s (%s): %d messages per minute", ErrRateLimited, userID, kind, g.cfg.PerMinuteMessages)
		}
	}
	return nil
}

// HistoryMessageLimit / HistoryCharBudget expose the assembly budget.
func (g *Guard) HistoryMessageLimit() int { return g.cfg.HistoryMessageLimit }
func (g *Guard) HistoryCharBudget() int   { return g.cfg.HistoryCharBudget }

// BoundHistory truncates client-supplied history to the newest entries
// within the message count, then drops the oldest until the total char
// budget fits — the client-controlled injection/overflow vector (P3-30).
func BoundHistory[T any](g *Guard, history []T, content func(T) string) []T {
	if len(history) > g.cfg.HistoryMessageLimit {
		history = history[len(history)-g.cfg.HistoryMessageLimit:]
	}
	total := 0
	for _, entry := range history {
		total += len(content(entry))
	}
	for total > g.cfg.HistoryCharBudget && len(history) > 0 {
		total -= len(content(history[0]))
		history = history[1:]
	}
	return history
}

// MemoryStore counts per-user events in memory (single instance, tests).
type MemoryStore struct {
	mu           sync.Mutex
	perMinute    int
	dailyLimit   float64
	minuteEvents map[string][]time.Time
	dailyCost    map[string]float64
	dayKey       func() string
	now          func() time.Time
}

func NewMemoryStore(perMinute int, dailyLimitUSD float64) *MemoryStore {
	return &MemoryStore{
		perMinute: perMinute, dailyLimit: dailyLimitUSD,
		minuteEvents: map[string][]time.Time{}, dailyCost: map[string]float64{},
		dayKey: func() string { return time.Now().UTC().Format("2006-01-02") },
		now:    time.Now,
	}
}

func (s *MemoryStore) Consume(_ context.Context, userID, kind string, _ int, costUSD float64) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + "|" + kind
	day := s.dayKey()
	cutoff := s.now().Add(-time.Minute)
	kept := make([]time.Time, 0)
	for _, event := range s.minuteEvents[key] {
		if event.After(cutoff) {
			kept = append(kept, event)
		}
	}
	s.minuteEvents[key] = kept
	if len(kept) >= s.perMinute {
		return false, "rate", nil
	}
	if s.dailyCost[day] >= s.dailyLimit {
		return false, "cost", nil
	}
	s.minuteEvents[key] = append(kept, s.now())
	s.dailyCost[day] += costUSD
	return true, "", nil
}
