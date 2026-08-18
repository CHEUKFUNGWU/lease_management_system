package periodutil

import (
	"testing"
)

func TestGenerateMonthlyPeriods(t *testing.T) {
	periods, err := GenerateMonthlyPeriods("2026-01", "2026-04")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"2026-01", "2026-02", "2026-03", "2026-04"}
	if len(periods) != len(expected) {
		t.Fatalf("expected %d periods, got %d", len(expected), len(periods))
	}
	for i, p := range periods {
		if p != expected[i] {
			t.Errorf("period[%d] = %s, expected %s", i, p, expected[i])
		}
	}

	// Cross year
	crossYear, err := GenerateMonthlyPeriods("2025-11", "2026-02")
	if err != nil {
		t.Fatalf("unexpected error on cross year: %v", err)
	}
	if len(crossYear) != 4 || crossYear[0] != "2025-11" || crossYear[3] != "2026-02" {
		t.Fatalf("unexpected cross year output: %v", crossYear)
	}

	// Invalid range
	_, err = GenerateMonthlyPeriods("2026-05", "2026-01")
	if err == nil {
		t.Fatalf("expected error for inverted periods")
	}
}

func TestDatesOverlap(t *testing.T) {
	// Overlapping
	if !DatesOverlap("2026-06-01", "2026-06-15", "2026-06-10", "2026-06-20") {
		t.Errorf("expected overlap true")
	}
	// Non-overlapping
	if DatesOverlap("2026-06-01", "2026-06-05", "2026-06-06", "2026-06-10") {
		t.Errorf("expected overlap false")
	}
	// Identical
	if !DatesOverlap("2026-06-01", "2026-06-10", "2026-06-01", "2026-06-10") {
		t.Errorf("expected identical overlap true")
	}
}

func TestDaysBetween(t *testing.T) {
	days, err := DaysBetween("2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if days != 30 {
		t.Fatalf("expected 30 days, got %d", days)
	}
}
