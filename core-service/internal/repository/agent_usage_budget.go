package repository

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/services/agentguard"
)

// AgentUsageStore adapts the append-only agent_usage_events table to the
// agentguard.BudgetStore seam — the cross-instance adapter next to the
// in-memory one. It lives apart from the usage-query reader
// (agent_usage.go): that reads the session runs, this writes/reads the
// budget events.
type AgentUsageStore struct {
	db         DBTX
	perMinute  int
	dailyLimit float64
}

func NewAgentUsageStore(db DBTX, perMinute int, dailyLimitUSD float64) *AgentUsageStore {
	return &AgentUsageStore{db: db, perMinute: perMinute, dailyLimit: dailyLimitUSD}
}

func (s *AgentUsageStore) Allow(ctx context.Context, userID, kind string) (bool, string, error) {
	var recent int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM agent_usage_events WHERE user_id=$1::uuid AND kind=$2 AND created_at > NOW() - INTERVAL '1 minute'`, userID, kind).Scan(&recent); err != nil {
		return false, "", fmt.Errorf("count agent usage window: %w", err)
	}
	if recent >= s.perMinute {
		return false, "rate", nil
	}
	var dailyCost float64
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(SUM(cost_usd), 0) FROM agent_usage_events WHERE user_id=$1::uuid AND kind=$2 AND created_at >= date_trunc('day', NOW())`, userID, kind).Scan(&dailyCost); err != nil {
		return false, "", fmt.Errorf("sum agent daily cost: %w", err)
	}
	if dailyCost >= s.dailyLimit {
		return false, "cost", nil
	}
	return true, "", nil
}

func (s *AgentUsageStore) Record(ctx context.Context, userID, kind string, tokens int, costUSD float64) error {
	if _, err := s.db.Exec(ctx, `INSERT INTO agent_usage_events (user_id,kind,tokens,cost_usd) VALUES ($1::uuid,$2,$3,$4)`, userID, kind, tokens, costUSD); err != nil {
		return fmt.Errorf("record agent usage: %w", err)
	}
	return nil
}

var _ agentguard.BudgetStore = (*AgentUsageStore)(nil)
