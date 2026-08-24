package contextassembler

import (
	"fmt"
	"strconv"
)

// Budget geometry defaults per model (AF1-c: window sizes are configuration,
// not guesses — but a model actually running must ALWAYS have an entry, or
// every turn dies on ErrBudgetUnconfigured after startup).
//
// Sources for the defaults (verified against provider docs 2026-08-24):
//   - deepseek-v4-flash: 1M context window (api-docs.deepseek.com Models &
//     Pricing; V4 family ships 1M as standard)
//   - gpt-4o: 128K context window
//
// ReserveTokens covers the maximum expected answer length: both providers'
// chat calls cap MaxTokens at 2000, so 4096 leaves headroom.
//
// CONTEXT_BUDGET_WINDOW_TOKENS / CONTEXT_BUDGET_RESERVE_TOKENS override ALL
// entries and additionally register an entry for models missing from the
// default table — a single-provider deployment with a non-default model name
// gets a working geometry from the override alone.
func BudgetSpecsFromEnv(getenv func(string) string) map[string]BudgetSpec {
	specs := map[string]BudgetSpec{
		"deepseek-v4-flash": {Window: 1_000_000, ReserveTokens: 4096},
		"gpt-4o":            {Window: 128_000, ReserveTokens: 4096},
	}
	window := positiveEnvInt(getenv, "CONTEXT_BUDGET_WINDOW_TOKENS")
	reserve := positiveEnvInt(getenv, "CONTEXT_BUDGET_RESERVE_TOKENS")
	for model, spec := range specs {
		if window > 0 {
			spec.Window = window
		}
		if reserve > 0 {
			spec.ReserveTokens = reserve
		}
		specs[model] = spec
	}
	// The window override is itself a declared geometry: let it cover models
	// outside the default table too.
	effReserve := reserve
	if effReserve <= 0 {
		effReserve = 4096 // default headroom when the operator sets only a window
	}
	if model := getenv("DEEPSEEK_MODEL"); model != "" && window > 0 {
		if _, ok := specs[model]; !ok {
			specs[model] = BudgetSpec{Window: window, ReserveTokens: effReserve}
		}
	}
	if model := getenv("OPENAI_MODEL"); model != "" && window > 0 {
		if _, ok := specs[model]; !ok {
			specs[model] = BudgetSpec{Window: window, ReserveTokens: effReserve}
		}
	}
	return specs
}

// ValidateModelCoverage refuses startup when the running model has no budget
// geometry. ErrBudgetUnconfigured's loud failure semantics are unchanged —
// this just moves the discovery from the first user request to process start
// (fail-fast, collector.fail() discipline).
func ValidateModelCoverage(specs map[string]BudgetSpec, model string) error {
	if _, ok := specs[model]; ok {
		return nil
	}
	return fmt.Errorf(
		"no context budget configured for running model %q: register it via CONTEXT_BUDGET_WINDOW_TOKENS "+
			"(and optionally CONTEXT_BUDGET_RESERVE_TOKENS) or add it to the default budget table — "+
			"window sizes are configuration, not guesses", model)
}

func positiveEnvInt(getenv func(string) string, key string) int {
	raw := getenv(key)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
