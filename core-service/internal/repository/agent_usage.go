package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AgentUsageQuery is deliberately scoped at the session tenant boundary. It
// never accepts a contract, run or user filter from the client; the handler
// derives these values from the authenticated principal.
type AgentUsageQuery struct {
	UserID        string
	LegalEntityID string
	Global        bool
	From          time.Time
	To            time.Time
}

type AgentUsageRollup struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	PricingVersion string `json:"pricing_version"`
	CostStatus     string `json:"cost_status"`
	PlannerCalls   int64  `json:"planner_calls"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
	TotalTokens    int64  `json:"total_tokens"`
	CostMicros     int64  `json:"cost_micros"`
}

type AgentUsageSummary struct {
	From                    time.Time          `json:"from"`
	To                      time.Time          `json:"to"`
	PlannerCalls            int64              `json:"planner_calls"`
	InputTokens             int64              `json:"input_tokens"`
	OutputTokens            int64              `json:"output_tokens"`
	TotalTokens             int64              `json:"total_tokens"`
	CostMicros              int64              `json:"cost_micros"`
	CostAccountingAvailable bool               `json:"cost_accounting_available"`
	UnavailableUsageCount   int64              `json:"unavailable_usage_count"`
	Rollups                 []AgentUsageRollup `json:"rollups"`
}

// SummarizePlannerUsage aggregates only the server-generated planner_usage
// events. Numeric JSON values are guarded before casting so malformed or
// provider-specific payloads remain visible as unavailable usage instead of
// making the operator endpoint fail.
func (r *AIChatRuntimeRepository) SummarizePlannerUsage(ctx context.Context, query AgentUsageQuery) (*AgentUsageSummary, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("agent usage repository unavailable")
	}
	if strings.TrimSpace(query.UserID) == "" {
		return nil, fmt.Errorf("agent usage user scope is required")
	}
	if query.From.IsZero() {
		query.From = time.Now().UTC().Add(-24 * time.Hour)
	}
	if query.To.IsZero() {
		query.To = time.Now().UTC()
	}
	if !query.To.After(query.From) {
		return nil, fmt.Errorf("agent usage time range is invalid")
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			COALESCE(NULLIF(e.payload->>'provider', ''), 'unknown'),
			COALESCE(NULLIF(e.payload->>'model', ''), 'unknown'),
			COALESCE(NULLIF(e.payload->>'pricing_version', ''), 'unconfigured'),
			COALESCE(NULLIF(e.payload->>'cost_status', ''), 'unavailable'),
			COUNT(*)::bigint,
			COALESCE(SUM(CASE WHEN e.payload->>'input_tokens' ~ '^[0-9]+$' THEN (e.payload->>'input_tokens')::bigint ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN e.payload->>'output_tokens' ~ '^[0-9]+$' THEN (e.payload->>'output_tokens')::bigint ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN e.payload->>'total_tokens' ~ '^[0-9]+$' THEN (e.payload->>'total_tokens')::bigint ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN e.payload->>'cost_micros' ~ '^[0-9]+$' THEN (e.payload->>'cost_micros')::bigint ELSE 0 END), 0)::bigint
		FROM ai_chat_run_events e
		JOIN ai_chat_sessions s ON s.id = e.session_id
		WHERE e.event_type = 'planner_usage'
		  AND e.created_at >= $4 AND e.created_at < $5
		  AND (
			$1::boolean
			OR s.user_id = $2
			OR ($3 <> '' AND s.legal_entity_id::text = $3)
		  )
		GROUP BY 1, 2, 3, 4
		ORDER BY 1, 2, 3, 4
	`, query.Global, query.UserID, strings.TrimSpace(query.LegalEntityID), query.From, query.To)
	if err != nil {
		return nil, fmt.Errorf("summarize agent planner usage: %w", err)
	}
	defer rows.Close()

	summary := &AgentUsageSummary{From: query.From, To: query.To, Rollups: make([]AgentUsageRollup, 0)}
	for rows.Next() {
		var item AgentUsageRollup
		if err := rows.Scan(&item.Provider, &item.Model, &item.PricingVersion, &item.CostStatus, &item.PlannerCalls, &item.InputTokens, &item.OutputTokens, &item.TotalTokens, &item.CostMicros); err != nil {
			return nil, fmt.Errorf("scan agent planner usage: %w", err)
		}
		summary.Rollups = append(summary.Rollups, item)
		summary.PlannerCalls += item.PlannerCalls
		summary.InputTokens += item.InputTokens
		summary.OutputTokens += item.OutputTokens
		summary.TotalTokens += item.TotalTokens
		summary.CostMicros += item.CostMicros
		if !strings.EqualFold(item.CostStatus, "measured") && !strings.EqualFold(item.CostStatus, "calculated") {
			summary.UnavailableUsageCount += item.PlannerCalls
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent planner usage: %w", err)
	}
	summary.CostAccountingAvailable = summary.PlannerCalls > 0 && summary.UnavailableUsageCount == 0
	return summary, nil
}
