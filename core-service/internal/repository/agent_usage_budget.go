package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

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

func (s *AgentUsageStore) Consume(ctx context.Context, userID, kind string, tokens int, costUSD float64) (bool, string, error) {
	// A per-user advisory lock serializes concurrent consumes; the window
	// count, the daily cost and the event insert happen in one transaction
	// so the ceiling can never be slipped between check and record.
	beginner, ok := s.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return false, "", fmt.Errorf("agent usage budgeting requires a PostgreSQL pool")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return false, "", fmt.Errorf("begin agent usage consume: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, userID+"|"+kind); err != nil {
		return false, "", fmt.Errorf("lock agent usage consume: %w", err)
	}
	var recent int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM agent_usage_events WHERE user_id=$1::uuid AND kind=$2 AND created_at > NOW() - INTERVAL '1 minute'`, userID, kind).Scan(&recent); err != nil {
		return false, "", fmt.Errorf("count agent usage window: %w", err)
	}
	if recent >= s.perMinute {
		return false, "rate", nil
	}
	var dailyCost float64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(cost_usd), 0) FROM agent_usage_events WHERE user_id=$1::uuid AND kind=$2 AND created_at >= date_trunc('day', NOW())`, userID, kind).Scan(&dailyCost); err != nil {
		return false, "", fmt.Errorf("sum agent daily cost: %w", err)
	}
	if dailyCost+costUSD > s.dailyLimit {
		return false, "cost", nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_usage_events (user_id,kind,tokens,cost_usd) VALUES ($1::uuid,$2,$3,$4)`, userID, kind, tokens, costUSD); err != nil {
		return false, "", fmt.Errorf("record agent usage: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, "", fmt.Errorf("commit agent usage consume: %w", err)
	}
	return true, "", nil
}

var _ agentguard.BudgetStore = (*AgentUsageStore)(nil)
