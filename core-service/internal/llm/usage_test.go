package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseUsageMissingUsageYieldsNil(t *testing.T) {
	cfg := Config{Provider: "deepseek", Model: "deepseek-v4-flash"}
	if got := ParseUsage(cfg, nil); got != nil {
		t.Fatalf("nil usage must yield nil metadata, got %#v", got)
	}
	if got := ParseUsage(cfg, "not-a-dict"); got != nil {
		t.Fatalf("non-dict usage must yield nil metadata, got %#v", got)
	}
}

func TestParseUsageTokenFallbackKeysAndSum(t *testing.T) {
	cfg := Config{Provider: "deepseek", Model: "m", PricingVersion: "unconfigured"}

	// prefer prompt_tokens over input_tokens; total missing -> sum
	meta := ParseUsage(cfg, map[string]any{
		"prompt_tokens": 100, "input_tokens": 999, "completion_tokens": 25,
	})
	if meta.InputTokens == nil || *meta.InputTokens != 100 {
		t.Fatalf("prompt_tokens must win over input_tokens: %#v", meta.InputTokens)
	}
	if meta.TotalTokens == nil || *meta.TotalTokens != 125 {
		t.Fatalf("total must be summed from both dims: %#v", meta.TotalTokens)
	}
	if meta.CostStatus != "unavailable" || meta.CostMicros != nil {
		t.Fatalf("unconfigured pricing must stay unavailable: %#v", meta)
	}

	// bool values are skipped like python's isinstance check
	meta = ParseUsage(cfg, map[string]any{"prompt_tokens": true})
	if meta.InputTokens != nil {
		t.Fatalf("bool token count must be skipped, got %#v", meta.InputTokens)
	}
}

func TestParseUsageNegativeAndNonNumericIgnored(t *testing.T) {
	cfg := Config{Provider: "deepseek", Model: "m", PricingVersion: "unconfigured"}
	meta := ParseUsage(cfg, map[string]any{
		"prompt_tokens": -5, "completion_tokens": "not-a-number", "total_tokens": 42,
	})
	if meta.InputTokens != nil || meta.OutputTokens != nil {
		t.Fatalf("negative/non-numeric tokens must be ignored: %#v", meta)
	}
	if meta.TotalTokens == nil || *meta.TotalTokens != 42 {
		t.Fatalf("total present must survive: %#v", meta.TotalTokens)
	}
}

func TestParseUsageMeasuredCost(t *testing.T) {
	cfg := Config{Provider: "deepseek", Model: "m", PricingVersion: "unconfigured"}
	meta := ParseUsage(cfg, map[string]any{
		"prompt_tokens": 200, "completion_tokens": 10, "total_tokens": 210,
		"cost_usd": 0.00042,
	})
	if meta.CostStatus != "measured" || meta.PricingSource != "provider_usage" {
		t.Fatalf("provider-measured cost must be preserved: %#v", meta)
	}
	if meta.CostMicros == nil || *meta.CostMicros != 420 {
		t.Fatalf("cost_micros = %v, want 420", meta.CostMicros)
	}
}

func TestParseUsageCalculatedCostRequiresFullBook(t *testing.T) {
	book := Config{Provider: "deepseek", Model: "m",
		PricingVersion: "prices-2026-08", InputPriceUSDPerMillion: 0.5, OutputPriceUSDPerMillion: 1.5}
	meta := ParseUsage(book, map[string]any{"prompt_tokens": 1_000_000, "completion_tokens": 1_000_000})
	if meta.CostStatus != "calculated" {
		t.Fatalf("full price book must calculate: %#v", meta)
	}
	if meta.CostMicros == nil || *meta.CostMicros != 2_000_000 {
		t.Fatalf("cost_micros = %v, want 2000000", meta.CostMicros)
	}

	// Half a book is not a book.
	partial := Config{Provider: "deepseek", Model: "m",
		PricingVersion: "prices", InputPriceUSDPerMillion: 0.5, OutputPriceUSDPerMillion: 0}
	if got := ParseUsage(partial, map[string]any{"prompt_tokens": 100, "completion_tokens": 100}); got.CostStatus != "unavailable" {
		t.Fatalf("zero output price must leave cost unavailable: %#v", got)
	}

	// Cost key check must treat a present-but-null cost_usd like python.
	meta = ParseUsage(book, map[string]any{
		"prompt_tokens": 100, "completion_tokens": 100, "cost_usd": nil,
	})
	if meta.CostStatus != "calculated" {
		t.Fatalf("null cost_usd with full book must fall back to calculation: %#v", meta)
	}
}

func TestParseUsageNumericStringCoercion(t *testing.T) {
	cfg := Config{Provider: "deepseek", Model: "m", PricingVersion: "unconfigured"}
	meta := ParseUsage(cfg, map[string]any{
		"prompt_tokens": "845", "completion_tokens": "96", "cost_usd": "0.00042",
	})
	if meta.InputTokens == nil || *meta.InputTokens != 845 {
		t.Fatalf("string token coercion failed: %#v", meta.InputTokens)
	}
	if meta.CostStatus != "measured" || meta.CostMicros == nil || *meta.CostMicros != 420 {
		t.Fatalf("string cost coercion failed: %#v", meta)
	}
}

func TestUsageMetadataJSONNullShape(t *testing.T) {
	// The agent-metrics page reads these fields; absent counts must serialize
	// to null, never to zero or omitted.
	cfg := Config{Provider: "deepseek", Model: "m", PricingVersion: "unconfigured"}
	meta := ParseUsage(cfg, map[string]any{"prompt_tokens": 1})
	out, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, field := range []string{`"schema_version":"llm-usage.v1"`, `"input_tokens":1`, `"output_tokens":null`, `"total_tokens":null`, `"cost_status":"unavailable"`} {
		if !strings.Contains(s, field) {
			t.Errorf("marshal missing %s in %s", field, s)
		}
	}
	if strings.Contains(s, "cost_micros") {
		t.Errorf("cost_micros must be absent for unavailable cost: %s", s)
	}
}