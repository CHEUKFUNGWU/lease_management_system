package agenttools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContextMetrics is the RT1-A observability sink for context budget usage.
// It parallels RuntimeMetrics (accumulate → Snapshot → Prometheus text) but
// counts context occupancy instead of tool executions.
//
// LABEL DISCIPLINE (RT1-A hard constraint): per-model only. legal_entity_id,
// user_id and session_id must NEVER become labels — two independent reasons:
// cardinality explosion, and the same discipline that keeps ContextKey from
// implementing String() (agentcontext D-C11): tenant/user identifiers must
// not leak into logs or scrape payloads through an implicit %v.
type ContextMetrics struct {
	mu        sync.Mutex
	startedAt time.Time

	// per-model aggregates, keyed by model name (fits the budget geometry,
	// which is configured per model).
	byModel map[string]*ContextModelMetric
}

// ContextModelMetric is the per-model aggregate. Measured and estimated are
// kept SEPARATE (RT1-A core acceptance / AF1-a lesson): a single summed
// number would look like measurement when part of it is a guess.
type ContextModelMetric struct {
	Model           string `json:"model"`
	Turns           uint64 `json:"turns"`
	TokensMeasured  uint64 `json:"tokens_measured"`
	TokensEstimated uint64 `json:"tokens_estimated"`
	// OverBudgetCount counts turns whose occupancy exceeded the budget —
	// regardless of whether compression actually ran (count mode reports
	// the would-be signal; on mode reports the compaction).
	OverBudgetCount uint64 `json:"over_budget_count"`
	CompactedCount  uint64 `json:"compacted_count"`
	// BudgetTokens is the last-seen effective budget for the model (window
	// minus output reserve), exposed as a gauge so alerts can compute real
	// occupancy ratios by model instead of hardcoding a window.
	BudgetTokens int `json:"budget_tokens"`
}

// NewContextMetrics constructs an empty sink.
func NewContextMetrics() *ContextMetrics {
	return &ContextMetrics{startedAt: time.Now().UTC(), byModel: make(map[string]*ContextModelMetric)}
}

// ObserveContext records one assembled turn. measured and estimated arrive
// ALREADY SEPARATED by the caller; this sink keeps them apart.
func (m *ContextMetrics) ObserveContext(model string, tokensMeasured, tokensEstimated, budget int, compacted, overBudget bool) {
	if m == nil {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "unknown"
	}
	if tokensMeasured < 0 {
		tokensMeasured = 0
	}
	if tokensEstimated < 0 {
		tokensEstimated = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startedAt.IsZero() {
		m.startedAt = time.Now().UTC()
	}
	metric := m.byModel[model]
	if metric == nil {
		metric = &ContextModelMetric{Model: model}
		m.byModel[model] = metric
	}
	metric.Turns++
	metric.TokensMeasured += uint64(tokensMeasured)
	metric.TokensEstimated += uint64(tokensEstimated)
	if budget > 0 {
		metric.BudgetTokens = budget
	}
	if overBudget {
		metric.OverBudgetCount++
	}
	if compacted {
		metric.CompactedCount++
	}
}

// Snapshot returns a stable, sorted snapshot for rendering or tests.
func (m *ContextMetrics) Snapshot() []ContextModelMetric {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ContextModelMetric, 0, len(m.byModel))
	for _, metric := range m.byModel {
		copy := *metric
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// Prometheus renders the bounded-cardinality scrape fragment. Measured and
// estimated are separate series; over_budget (the pre-warning counter) and
// compacted (the post-hoc counter) are separate series too.
func (m *ContextMetrics) Prometheus(now time.Time) string {
	if m == nil {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("# HELP lease_agent_context_turns_total Model-context turns assembled.\n")
	builder.WriteString("# TYPE lease_agent_context_turns_total counter\n")
	builder.WriteString("# HELP lease_agent_context_tokens_measured_total Provider-measured tokens in assembled context (measured truth, NEVER mixed with estimates).\n")
	builder.WriteString("# TYPE lease_agent_context_tokens_measured_total counter\n")
	builder.WriteString("# HELP lease_agent_context_tokens_estimated_total Tail-estimated tokens in assembled context (estimates, kept separate from measured by RT1-A).\n")
	builder.WriteString("# TYPE lease_agent_context_tokens_estimated_total counter\n")
	builder.WriteString("# HELP lease_agent_context_over_budget_total Turns whose occupancy exceeded the budget (pre-warning; fires before any drop).\n")
	builder.WriteString("# TYPE lease_agent_context_over_budget_total counter\n")
	builder.WriteString("# HELP lease_agent_context_compacted_total Turns where content was actually dropped by compaction.\n")
	builder.WriteString("# TYPE lease_agent_context_compacted_total counter\n")
	builder.WriteString("# HELP lease_agent_context_budget_tokens Effective context budget per model (window minus output reserve); the denominator for occupancy alerts.\n")
	builder.WriteString("# TYPE lease_agent_context_budget_tokens gauge\n")
	for _, metric := range m.Snapshot() {
		label := `model="` + prometheusEscape(metric.Model) + `"`
		fmt.Fprintf(&builder, "lease_agent_context_turns_total{%s} %d\n", label, metric.Turns)
		fmt.Fprintf(&builder, "lease_agent_context_tokens_measured_total{%s} %d\n", label, metric.TokensMeasured)
		fmt.Fprintf(&builder, "lease_agent_context_tokens_estimated_total{%s} %d\n", label, metric.TokensEstimated)
		fmt.Fprintf(&builder, "lease_agent_context_over_budget_total{%s} %d\n", label, metric.OverBudgetCount)
		fmt.Fprintf(&builder, "lease_agent_context_compacted_total{%s} %d\n", label, metric.CompactedCount)
		fmt.Fprintf(&builder, "lease_agent_context_budget_tokens{%s} %d\n", label, metric.BudgetTokens)
	}
	return builder.String()
}
