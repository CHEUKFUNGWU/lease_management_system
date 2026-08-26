package agenttools

import (
	"strings"
	"testing"
	"time"
)

// TestContextMetricsKeepsMeasuredAndEstimatedSeparate pins RT1-A core
// acceptance: the prometheus render MUST expose measured and estimated as two
// distinct series, never a single merged number (AF1-a lesson — a merged
// number would look like measurement when part of it is a guess).
func TestContextMetricsKeepsMeasuredAndEstimatedSeparate(t *testing.T) {
	m := NewContextMetrics()
	m.ObserveContext("m1", 800, 200, 1000, false, false) // measured 800, estimated 200
	payload := m.Prometheus(time.Now().UTC())

	if !strings.Contains(payload, "lease_agent_context_tokens_measured_total") ||
		!strings.Contains(payload, "lease_agent_context_tokens_estimated_total") {
		t.Fatalf("render lacks separate measured/estimated series:\n%s", payload)
	}
	measuredLine := lineWith(payload, "lease_agent_context_tokens_measured_total")
	estimatedLine := lineWith(payload, "lease_agent_context_tokens_estimated_total")
	if !strings.HasSuffix(measuredLine, " 800") {
		t.Fatalf("measured series = %q; want 800", measuredLine)
	}
	if !strings.HasSuffix(estimatedLine, " 200") {
		t.Fatalf("estimated series = %q; want 200", estimatedLine)
	}
	// No series may contain a merged 1000 total.
	if strings.Contains(payload, "lease_agent_context_tokens_total") ||
		strings.Contains(payload, "_tokens_total") && !strings.Contains(measuredLine, "_measured") && !strings.Contains(estimatedLine, "_estimated") {
		t.Fatalf("render must never merge measured+estimated; found a merged token series:\n%s", payload)
	}
}

// TestContextMetricsPrometheusOmitsTenantIdentifiers is RT1-A label discipline
// (reverse fixture): legal_entity_id / user_id / session_id must never appear
// in the scrape payload. Deleting the discipline must make this red.
func TestContextMetricsPrometheusOmitsTenantIdentifiers(t *testing.T) {
	m := NewContextMetrics()
	m.ObserveContext("m1", 100, 20, 200, false, false)
	payload := m.Prometheus(time.Now().UTC())

	for _, forbidden := range []string{"legal_entity", "user_id", "session_id", "legal_entity_id"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("scrape payload leaks tenant identifier %q:\n%s", forbidden, payload)
		}
	}
	// Model label is allowed (budget geometry is per-model) and present.
	if !strings.Contains(payload, `model="m1"`) {
		t.Fatalf("per-model label missing:\n%s", payload)
	}
}

// TestContextMetricsOverBudgetAndCompactedAreDistinct pins that the prewarning
// (over_budget) and the post-hoc record (compacted) are separate counters.
func TestContextMetricsOverBudgetAndCompactedAreDistinct(t *testing.T) {
	m := NewContextMetrics()
	m.ObserveContext("m1", 900, 100, 1000, false, true) // count mode: would compact, did not
	m.ObserveContext("m2", 920, 80, 1000, true, true)   // on mode: compacted
	payload := m.Prometheus(time.Now().UTC())

	overTotal := sumField(payload, "lease_agent_context_over_budget_total")
	compactedTotal := sumField(payload, "lease_agent_context_compacted_total")
	if overTotal != 2 {
		t.Fatalf("over_budget counter sum = %d; want 2 (both turns exceeded)", overTotal)
	}
	if compactedTotal != 1 {
		t.Fatalf("compacted counter sum = %d; want 1 (only on mode actually dropped)", compactedTotal)
	}
}

func lineWith(payload, name string) string {
	for _, line := range strings.Split(payload, "\n") {
		if strings.HasPrefix(line, name+"{") {
			return line
		}
	}
	return ""
}

// sumField sums the numeric suffix across every series line of the given
// metric name (per-model series aggregate into one counter view).
func sumField(payload, name string) int {
	total := 0
	for _, line := range strings.Split(payload, "\n") {
		if !strings.HasPrefix(line, name+"{") {
			continue
		}
		idx := strings.LastIndex(line, " ")
		if idx < 0 {
			continue
		}
		value := 0
		fmtSscanf(line[idx+1:], &value)
		total += value
	}
	return total
}

func fmtSscanf(value string, out *int) {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	*out = n
}
