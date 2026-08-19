package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel/memo"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestBatchSuggestionToolDryRunComputesNoWrite(t *testing.T) {
	store := suggestion.NewMemoryStore("LE-1")
	def := NewAssumptionSuggestionBatchDefinition(memSuggestionWriter{store: store})
	args := json.RawMessage(`{
		"target_period":"2026-06",
		"historical":{"revenue":[90,92,91,95,93,96,94,97,98,99,100,101]},
		"seasonality":{"revenue":[1,1,1,1,1,1.2,1,1,1,1,1,1]},
		"registered":{"tax_rate":0.25}
	}`)
	result, err := def.Handler(suggestionCtx(), agenttools.ToolCall{CallID: "tcall-b1", Arguments: args, IdempotencyKey: "k-b1", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	blocks, ok := data["blocks"].([]suggestion.BatchBlock)
	if !ok || len(blocks) == 0 {
		t.Fatalf("dry run must return blocks, got %+v", data)
	}
	if data["side_effects"] != false {
		t.Fatalf("dry run must not write: %+v", data)
	}
	if len(store.List()) != 0 {
		t.Fatalf("dry run wrote drafts: %+v", store.List())
	}
	unsuggestable, _ := data["unsuggestable"].([]suggestion.UnSuggestableItem)
	for _, item := range unsuggestable {
		if item.Reason == "" {
			t.Fatalf("unsuggestable item must name its reason: %+v", item)
		}
	}
}

func TestBatchSuggestionToolWritesDraftOnly(t *testing.T) {
	store := suggestion.NewMemoryStore("LE-1")
	def := NewAssumptionSuggestionBatchDefinition(memSuggestionWriter{store: store})
	args := json.RawMessage(`{"target_period":"2026-06","historical":{"revenue":[90,92,91]}}`)
	result, err := def.Handler(suggestionCtx(), agenttools.ToolCall{CallID: "tcall-b2", Arguments: args, IdempotencyKey: "k-b2"})
	if err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	if data["side_effects"] != true {
		t.Fatalf("write path must persist drafts: %+v", data)
	}
	for _, draft := range store.List() {
		if draft.Status != "draft" || draft.SourceTag != "ai_suggestion" {
			t.Fatalf("stored row must be an ai draft: %+v", draft)
		}
	}
}

func TestBatchSuggestionToolHonestRefusal(t *testing.T) {
	def := NewAssumptionSuggestionBatchDefinition(nil)
	if _, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("nil writer must refuse honestly")
	}
}

type memoCapturingWriter struct {
	created *repository.FPnADecisionMemo
}

func (m *memoCapturingWriter) CreateMemo(ctx context.Context, item *repository.FPnADecisionMemo) (*repository.FPnADecisionMemo, error) {
	m.created = item
	item.ID = "memo-1"
	return item, nil
}

func TestModelDiffMemoToolFourLayers(t *testing.T) {
	writer := &memoCapturingWriter{}
	def := NewModelDiffMemoDefinition(writer)
	args := json.RawMessage(`{
		"title":"6 月模型差异：收入归因",
		"left_lines":{"rev@2026-06":120,"cost@2026-06":60},
		"right_lines":{"rev@2026-06":100,"cost@2026-06":50},
		"narratives":[
			{"key":"rev@2026-06","explanation":"客流回升","amount_covered":20},
			{"key":"cost@2026-06","explanation":"人工部分被优化","amount_covered":6}
		],
		"system_facts":{"data_version":"d7"},
		"data_version":"d7"
	}`)
	result, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: args, IdempotencyKey: "k-m1"})
	if err != nil {
		t.Fatal(err)
	}
	if writer.created == nil {
		t.Fatal("memo must be persisted as draft")
	}
	if writer.created.Status != "draft" || writer.created.MemoType != "model_diff" {
		t.Fatalf("memo row = %+v", writer.created)
	}
	var det memo.Deterministic
	if err := json.Unmarshal(writer.created.DeterministicCalculations, &det); err != nil {
		t.Fatal(err)
	}
	if det.Residuals["cost@2026-06"] != 4 {
		t.Fatalf("residual must stay explicit, got %+v", det.Residuals)
	}
	data := result.Data.(map[string]any)
	if data["review_required"] != true || data["formal_state"] != "decision_memo_draft" {
		t.Fatalf("result must demand review: %+v", data)
	}
}

func TestModelDiffMemoToolRejectsUnbridgedNarrative(t *testing.T) {
	def := NewModelDiffMemoDefinition(&memoCapturingWriter{})
	args := json.RawMessage(`{
		"title":"x","left_lines":{"rev@2026-06":120},"right_lines":{"rev@2026-06":100},
		"narratives":[{"key":"made_up@2026-06","explanation":"猜","amount_covered":1}]
	}`)
	if _, err := def.Handler(suggestionCtx(), agenttools.ToolCall{Arguments: args, IdempotencyKey: "k-m2"}); err == nil {
		t.Fatal("narrative on an unbridged key must be rejected")
	}
}
