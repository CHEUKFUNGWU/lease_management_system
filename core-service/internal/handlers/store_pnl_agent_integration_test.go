package handlers

// T5 跨法人隔离集成测试（底线 1）：fpna.store_pnl.read 的 Agent 读路径在真实
// MasterDataRepository 上把 store-scope 闸门跑一遍。法人 B 的 principal 读
// 法人 A 的门店必须等于「不存在」——scope_denied 措辞、无存在性泄漏；真实
// 存在的异法人门店与根本不存在的门店对外不可区分。带库跑（TEST_DATABASE_URL），
// 未设则 skip。

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// storePnlSpreadKPI serves per-store aggregates for the projection.
type storePnlSpreadKPI map[string]storepnl.KPIAggregates

func (m storePnlSpreadKPI) Operating(_ context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	aggregates, ok := m[ref.StoreID]
	if !ok {
		return storepnl.KPIAggregates{}, os.ErrNotExist
	}
	return aggregates, nil
}

func TestStorePnlAgentCrossLegalEntityIsolation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	suffix := uuid.NewString()[:8]
	entityA := uuid.NewString()
	entityB := uuid.NewString()
	storeA1 := uuid.NewString()
	storeB1 := uuid.NewString()
	ghost := uuid.NewString() // 不存在的门店

	seed := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v (%s)", err, sql)
		}
	}
	seed(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES
		($1,$2,$3,'CN','CNY'), ($4,$5,$6,'CN','CNY')`,
		entityA, "PNL-A-"+suffix, "Pnl A "+suffix, entityB, "PNL-B-"+suffix, "Pnl B "+suffix)
	seed(`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES
		($1,$2,'Pnl A1',$3,'east','b1'), ($4,$5,'Pnl B1',$6,'east','b1')`,
		storeA1, "PNL-S1-"+suffix, entityA, storeB1, "PNL-S2-"+suffix, entityB)
	t.Cleanup(func() {
		cleanup := context.Background()
		if _, err := pool.Exec(cleanup, `DELETE FROM stores WHERE id = ANY($1)`, []string{storeA1, storeB1}); err != nil {
			t.Errorf("cleanup stores: %v", err)
		}
		if _, err := pool.Exec(cleanup, `DELETE FROM legal_entities WHERE id = ANY($1)`, []string{entityA, entityB}); err != nil {
			t.Errorf("cleanup legal_entities: %v", err)
		}
	})

	revenue := 1000.0
	kpi := storePnlSpreadKPI{
		storeA1: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY"},
		storeB1: {Revenue: &revenue, DecisionReady: true, Classification: "production", Currency: "CNY"},
	}
	handler := NewStorePnlHandler(kpi, nil, nil).WithMasterData(repository.NewMasterDataRepository(pool))
	seam := NewStorePnlAgentReader(handler)
	def := agenttooldefs.NewStorePnlReadDefinition(seam)

	invoke := func(legalEntityID, storeID string) agenttools.ToolResult {
		t.Helper()
		callCtx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
			Principal: agenttools.Principal{
				UserID: "bp-pg", Permissions: []string{"reports:read"},
				Scope: access.Scope{LegalEntityID: legalEntityID},
			},
			RunID: "agent-pg",
		})
		args, _ := json.Marshal(map[string]any{
			"store_id":            storeID,
			"period":              "2026-06",
			"data_classification": "production",
		})
		result, err := def.Handler(callCtx, agenttools.ToolCall{
			CallID: "c", RunID: "agent-pg", ToolName: "fpna.store_pnl.read", ToolVersion: "v1", Arguments: args,
		})
		if err != nil {
			t.Fatalf("handler returned Go error: %v", err)
		}
		return result
	}

	// 1. 同法人读自己的门店 → completed。
	inScope := invoke(entityA, storeA1)
	if inScope.Status != agenttools.StatusCompleted {
		t.Fatalf("entity A must read its own store, got status=%s err=%+v", inScope.Status, inScope.Error)
	}

	// 2. 法人 B 读法人 A 的门店 → 拒绝，scope_denied 措辞，无存在性泄漏。
	foreign := invoke(entityB, storeA1)
	if foreign.Status == agenttools.StatusCompleted {
		t.Fatal("entity B read entity A store — cross-tenant leak")
	}
	if foreign.Error == nil || foreign.Error.Code != agenttools.ErrorScopeDenied {
		t.Fatalf("foreign store must be scope_denied, got %+v", foreign.Error)
	}
	if !strings.Contains(foreign.Error.Message, "scope_denied") {
		t.Fatalf("scope_denied reason must be preserved: %q", foreign.Error.Message)
	}
	if strings.Contains(foreign.Error.Message, "store not found") || strings.Contains(foreign.Error.Message, "outside caller tenant") {
		t.Fatalf("foreign-store denial must not leak existence: %q", foreign.Error.Message)
	}

	// 3. 根本不存在（ghost store）与异法人门店对外完全同形（无存在性泄漏）。
	ghostResult := invoke(entityA, ghost)
	if ghostResult.Status == agenttools.StatusCompleted {
		t.Fatal("ghost store must not project")
	}
	if foreign.Error == nil || ghostResult.Error == nil {
		t.Fatal("both denials must carry errors")
	}
	if foreign.Error.Code != ghostResult.Error.Code || foreign.Error.Message != ghostResult.Error.Message {
		t.Fatalf("foreign vs ghost must be indistinguishable: foreign=%q ghost=%q", foreign.Error.Message, ghostResult.Error.Message)
	}
}
