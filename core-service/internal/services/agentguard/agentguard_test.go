package agentguard

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGuardRateLimitRefusalKeepsReason(t *testing.T) {
	store := NewMemoryStore(2, 10)
	guard := New(store, Config{})
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := guard.Check(ctx, "user-a", "chat"); err != nil {
			t.Fatalf("check %d: %v", i, err)
		}
		if err := guard.Record(ctx, "user-a", "chat", 0); err != nil {
			t.Fatal(err)
		}
	}
	err := guard.Check(ctx, "user-a", "chat")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v, want rate limit", err)
	}
	// A different user is unaffected.
	if err := guard.Check(ctx, "user-b", "chat"); err != nil {
		t.Fatalf("user-b: %v", err)
	}
}

func TestGuardDailyCostCeiling(t *testing.T) {
	store := NewMemoryStore(100, 0.01)
	guard := New(store, Config{})
	ctx := context.Background()
	if err := guard.Record(ctx, "user-a", "chat", 5000); err != nil { // 5000 * 2e-6 = 0.01
		t.Fatal(err)
	}
	if err := guard.Check(ctx, "user-a", "chat"); !errors.Is(err, ErrCostExceeded) {
		t.Fatalf("err=%v, want cost exceeded", err)
	}
}

func TestBoundHistoryKeepsNewestWithinBudget(t *testing.T) {
	guard := New(NewMemoryStore(10, 10), Config{HistoryMessageLimit: 3, HistoryCharBudget: 20})
	// Message limit keeps the 3 newest (10+1+20=31 chars); the char budget
	// then drops oldest-first until ≤20 — only the 20-char newest entry fits.
	history := []string{"a", "bbbbbbbbbb", "c", "dddddddddddddddddddd"}
	bound := BoundHistory(guard, history, func(entry string) string { return entry })
	if len(bound) != 1 || bound[0] != "dddddddddddddddddddd" {
		t.Fatalf("bound=%v", bound)
	}
}

func TestMemoryStoreWindowExpires(t *testing.T) {
	store := NewMemoryStore(2, 10)
	now := time.Now()
	store.now = func() time.Time { return now }
	store.minuteEvents["u|chat"] = []time.Time{now.Add(-2 * time.Minute), now.Add(-90 * time.Second)}
	allowed, _, err := store.Allow(context.Background(), "u", "chat")
	if err != nil || !allowed {
		t.Fatalf("expired events should not count: allowed=%v err=%v", allowed, err)
	}
}
