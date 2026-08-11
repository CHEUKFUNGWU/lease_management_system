package handlers

import (
	"testing"
	"time"
)

func TestUsageTimeRangeRejectsInvalidAndOversizedRanges(t *testing.T) {
	if _, _, err := usageTimeRange("not-a-timestamp", ""); err == nil {
		t.Fatal("expected invalid from timestamp to be rejected")
	}
	if _, _, err := usageTimeRange("2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z"); err == nil {
		t.Fatal("expected non-increasing range to be rejected")
	}
	if _, _, err := usageTimeRange("2026-07-01T00:00:00Z", "2026-08-02T00:00:00Z"); err == nil {
		t.Fatal("expected range over 31 days to be rejected")
	}
	from, to, err := usageTimeRange("2026-08-01T00:00:00Z", "2026-08-02T00:00:00Z")
	if err != nil || !to.After(from) || to.Sub(from) != 24*time.Hour {
		t.Fatalf("valid range: from=%s to=%s err=%v", from, to, err)
	}
}
