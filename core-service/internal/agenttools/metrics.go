package agenttools

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RuntimeMetrics is an in-process, low-cardinality metrics sink for Tool
// Runtime executions. It aggregates only server-owned tool names and result
// statuses; run IDs, users and contract IDs must never become metric labels.
type RuntimeMetrics struct {
	mu         sync.RWMutex
	startedAt  time.Time
	total      uint64
	durationMs int64
	byTool     map[string]RuntimeToolMetric
}

type RuntimeToolMetric struct {
	ToolName          string     `json:"tool_name"`
	Status            ToolStatus `json:"status"`
	Executions        uint64     `json:"executions"`
	Failures          uint64     `json:"failures"`
	ReviewRequired    uint64     `json:"review_required"`
	DurationMillisSum int64      `json:"duration_millis_sum"`
	DurationMillisMax int64      `json:"duration_millis_max"`
}

type RuntimeMetricsSnapshot struct {
	StartedAt               time.Time           `json:"started_at"`
	CollectedAt             time.Time           `json:"collected_at"`
	TotalExecutions         uint64              `json:"total_executions"`
	TotalDurationMillis     int64               `json:"total_duration_millis"`
	ToolMetrics             []RuntimeToolMetric `json:"tool_metrics"`
	CostAccountingAvailable bool                `json:"cost_accounting_available"`
	CostAccountingNote      string              `json:"cost_accounting_note"`
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{startedAt: time.Now().UTC(), byTool: make(map[string]RuntimeToolMetric)}
}

// Observe records one completed Tool attempt. Duration is clamped at zero so
// a clock adjustment cannot produce a negative aggregate.
func (m *RuntimeMetrics) Observe(audit ToolExecutionAudit) {
	if m == nil {
		return
	}
	duration := audit.DurationMillis
	if duration < 0 {
		duration = 0
	}
	tool := strings.TrimSpace(audit.ToolName)
	if tool == "" {
		tool = "unknown"
	}
	status := audit.Status
	if status == "" {
		status = StatusFailed
	}
	key := tool + "\x00" + string(status)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startedAt.IsZero() {
		m.startedAt = time.Now().UTC()
	}
	metric := m.byTool[key]
	metric.ToolName = tool
	metric.Status = status
	metric.Executions++
	if audit.ErrorCode != "" || status == StatusFailed || status == StatusRejected {
		metric.Failures++
	}
	if audit.ReviewRequired {
		metric.ReviewRequired++
	}
	metric.DurationMillisSum += duration
	if duration > metric.DurationMillisMax {
		metric.DurationMillisMax = duration
	}
	m.byTool[key] = metric
	m.total++
	m.durationMs += duration
}

func (m *RuntimeMetrics) Snapshot(now time.Time) RuntimeMetricsSnapshot {
	if m == nil {
		return RuntimeMetricsSnapshot{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]RuntimeToolMetric, 0, len(m.byTool))
	for _, metric := range m.byTool {
		items = append(items, metric)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ToolName == items[j].ToolName {
			return items[i].Status < items[j].Status
		}
		return items[i].ToolName < items[j].ToolName
	})
	return RuntimeMetricsSnapshot{
		StartedAt:               m.startedAt,
		CollectedAt:             now,
		TotalExecutions:         m.total,
		TotalDurationMillis:     m.durationMs,
		ToolMetrics:             items,
		CostAccountingAvailable: false,
		CostAccountingNote:      "Tool protocol does not carry model token usage or provider pricing; monetary cost must be supplied by the model gateway before it is reported.",
	}
}

// Prometheus renders a bounded-cardinality scrape payload. Monetary cost is
// intentionally not emitted because the current Tool contract has no usage
// or pricing fields.
func (m *RuntimeMetrics) Prometheus(now time.Time) string {
	snapshot := m.Snapshot(now)
	var builder strings.Builder
	builder.WriteString("# HELP lease_agent_tool_executions_total Total Tool Runtime execution attempts.\n")
	builder.WriteString("# TYPE lease_agent_tool_executions_total counter\n")
	builder.WriteString("# HELP lease_agent_tool_execution_duration_milliseconds_sum Total Tool Runtime execution duration in milliseconds.\n")
	builder.WriteString("# TYPE lease_agent_tool_execution_duration_milliseconds_sum counter\n")
	builder.WriteString("# HELP lease_agent_tool_execution_duration_milliseconds_max Maximum observed Tool Runtime duration in milliseconds.\n")
	builder.WriteString("# TYPE lease_agent_tool_execution_duration_milliseconds_max gauge\n")
	builder.WriteString("# HELP lease_agent_tool_failures_total Failed or rejected Tool Runtime executions.\n")
	builder.WriteString("# TYPE lease_agent_tool_failures_total counter\n")
	builder.WriteString("# HELP lease_agent_tool_review_required_total Tool Runtime executions that reached a Review Gate.\n")
	builder.WriteString("# TYPE lease_agent_tool_review_required_total counter\n")
	for _, metric := range snapshot.ToolMetrics {
		labels := `tool="` + prometheusEscape(metric.ToolName) + `",status="` + prometheusEscape(string(metric.Status)) + `"`
		fmt.Fprintf(&builder, "lease_agent_tool_executions_total{%s} %d\n", labels, metric.Executions)
		fmt.Fprintf(&builder, "lease_agent_tool_execution_duration_milliseconds_sum{%s} %d\n", labels, metric.DurationMillisSum)
		fmt.Fprintf(&builder, "lease_agent_tool_execution_duration_milliseconds_max{%s} %d\n", labels, metric.DurationMillisMax)
		fmt.Fprintf(&builder, "lease_agent_tool_failures_total{%s} %d\n", labels, metric.Failures)
		fmt.Fprintf(&builder, "lease_agent_tool_review_required_total{%s} %d\n", labels, metric.ReviewRequired)
	}
	builder.WriteString("# HELP lease_agent_tool_cost_accounting_available Whether model/provider cost accounting is available (0 in the current Tool protocol).\n")
	builder.WriteString("# TYPE lease_agent_tool_cost_accounting_available gauge\n")
	builder.WriteString("lease_agent_tool_cost_accounting_available ")
	builder.WriteString(strconv.Itoa(boolInt(snapshot.CostAccountingAvailable)))
	builder.WriteByte('\n')
	return builder.String()
}

func prometheusEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
