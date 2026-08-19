package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
)

type memSuggestionWriter struct {
	store *suggestion.MemoryStore
}

func (m memSuggestionWriter) SaveDrafts(ctx context.Context, legalEntityID string, drafts []suggestion.SuggestionDraft, key string) ([]string, error) {
	return m.store.SaveDrafts(ctx, legalEntityID, drafts, key)
}

func suggestionCtx() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "fpna-u1", Permissions: []string{"*:*"}, Scope: access.Scope{LegalEntityID: "LE-1"}},
		RunID:     "run-sug-1",
	})
}

func TestAssumptionSuggestToolRequiresBasis(t *testing.T) {
	store := suggestion.NewMemoryStore("LE-1")
	def := NewAssumptionSuggestionDefinition(memSuggestionWriter{store: store})
	// 无依据建议必须被结构性拒绝。
	noBasis := json.RawMessage(`{"suggestions":[{"assumption_key":"sssg","category":"revenue","value":0.03,"confidence":0.8,"basis":[]}]}`)
	if _, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: noBasis, IdempotencyKey: "k1"}); !errors.Is(err, suggestion.ErrMissingBasis) {
		t.Fatalf("basis-less suggestions must be refused, got %v", err)
	}
	// 有依据 → draft 落库，绝不 approved。
	withBasis := json.RawMessage(`{"suggestions":[{"assumption_key":"sssg","category":"revenue","value":0.03,"confidence":0.7,"basis":[{"tool_call_id":"tcall-1","scope":"LE-1","period":"2026-01"}]}]}`)
	result, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: withBasis, IdempotencyKey: "k2"})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	if int(data["draft_count"].(int)) != 1 && data["draft_count"] != 1 {
		t.Fatalf("one draft expected, got %+v", data)
	}
	stored := store.List()
	if len(stored) != 1 || stored[0].Status != "draft" {
		t.Fatalf("drafts must store with status draft, got %+v", stored)
	}
	if stored[0].SourceTag != "ai_suggestion" {
		t.Fatalf("source must be ai_suggestion, got %q", stored[0].SourceTag)
	}
}

func TestAssumptionSuggestToolWriterUnavailable(t *testing.T) {
	def := NewAssumptionSuggestionDefinition(nil)
	if _, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: json.RawMessage(`{"suggestions":[{}]}`)}); err == nil {
		t.Fatal("missing writer must refuse honestly")
	}
}
