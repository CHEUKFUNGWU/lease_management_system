package llm

import (
	"encoding/json"
	"math"
)

// UsageMetadata carries provider token usage and the cost state. It replicates
// ai-service/app/services/llm.py::usage_metadata exactly, including its
// refusal to invent missing counts: when the provider reports no usage block
// the whole metadata is nil, and monetary cost stays "unavailable" unless a
// provider-measured amount or a fully configured local price book exists.
//
// The model gateway owns the provider response and the configured price book;
// this metadata is operational only, never an accounting amount, and must not
// be used for financial posting.
type UsageMetadata struct {
	SchemaVersion  string `json:"schema_version"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	InputTokens    *int   `json:"input_tokens"`
	OutputTokens   *int   `json:"output_tokens"`
	TotalTokens    *int   `json:"total_tokens"`
	PricingVersion string `json:"pricing_version"`
	PricingSource  string `json:"pricing_source"`
	CostCurrency   string `json:"cost_currency"`
	CostStatus     string `json:"cost_status"` // unavailable | measured | calculated
	CostMicros     *int64 `json:"cost_micros,omitempty"`
}

// ParseUsage normalizes a provider usage object. raw is the decoded "usage"
// field of a chat completion response; a missing or non-object usage yields
// nil metadata (cost display becomes unavailable, never invented).
func ParseUsage(cfg Config, raw any) *UsageMetadata {
	usage, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	inputTokens := nonNegativeInt(usage, "prompt_tokens", "input_tokens")
	outputTokens := nonNegativeInt(usage, "completion_tokens", "output_tokens")
	totalTokens := nonNegativeInt(usage, "total_tokens")
	if totalTokens == nil && inputTokens != nil && outputTokens != nil {
		sum := *inputTokens + *outputTokens
		totalTokens = &sum
	}

	meta := &UsageMetadata{
		SchemaVersion:  "llm-usage.v1",
		Provider:       cfg.Provider,
		Model:          cfg.Model,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		TotalTokens:    totalTokens,
		PricingVersion: cfg.PricingVersion,
		PricingSource:  "configured_settings",
		CostCurrency:   "USD",
		CostStatus:     "unavailable",
	}

	costUSD := -1.0
	if costVal, ok := usage["cost_usd"]; ok {
		costUSD = coerceFloatOr(costVal, -1.0)
	} else if costVal, ok := usage["cost"]; ok {
		costUSD = coerceFloatOr(costVal, -1.0)
	}
	if costUSD >= 0 {
		micros := int64(math.Round(costUSD * 1_000_000))
		meta.CostMicros = &micros
		meta.CostStatus = "measured"
		meta.PricingSource = "provider_usage"
	} else if inputTokens != nil && outputTokens != nil &&
		cfg.InputPriceUSDPerMillion > 0 && cfg.OutputPriceUSDPerMillion > 0 &&
		cfg.PricingVersion != "unconfigured" {
		costUSD = (float64(*inputTokens)*cfg.InputPriceUSDPerMillion +
			float64(*outputTokens)*cfg.OutputPriceUSDPerMillion) / 1_000_000
		micros := int64(math.Round(costUSD * 1_000_000))
		meta.CostMicros = &micros
		meta.CostStatus = "calculated"
	}

	return meta
}

// nonNegativeInt implements the Python helper of the same name: the first key
// whose value coerces to a non-negative int wins; booleans are skipped.
func nonNegativeInt(m map[string]any, keys ...string) *int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		if _, isBool := v.(bool); isBool {
			continue
		}
		parsed, err := coerceInt(v)
		if err != nil {
			continue
		}
		if parsed >= 0 {
			return &parsed
		}
	}
	return nil
}

// coerceInt mirrors Python int(value): JSON numbers, numeric strings and
// integers all coerce.
func coerceInt(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case json.Number:
		i, err := t.Int64()
		return int(i), err
	case string:
		var f float64
		if err := json.Unmarshal([]byte(t), &f); err != nil {
			return 0, err
		}
		return int(f), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	default:
		return 0, &coerceError{}
	}
}

// coerceFloatOr mirrors Python float(value) with a fallback for values that
// do not coerce (Python would raise ValueError/TypeError, caught to -1.0).
func coerceFloatOr(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return fallback
		}
		return f
	case string:
		var f float64
		if err := json.Unmarshal([]byte(t), &f); err != nil {
			return fallback
		}
		return f
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return fallback
	}
}

type coerceError struct{}

func (*coerceError) Error() string { return "value does not coerce to int" }
