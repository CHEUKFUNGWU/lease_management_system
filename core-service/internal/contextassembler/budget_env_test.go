package contextassembler

import (
	"strings"
	"testing"
)

// ── AF1-c：实际在跑的模型一定有预算几何；缺配置在启动期发现，不由第一个用户发现 ──

func TestBudgetSpecsFromEnvDefaults(t *testing.T) {
	specs := BudgetSpecsFromEnv(func(string) string { return "" })
	for _, model := range []string{"deepseek-v4-flash", "gpt-4o"} {
		spec, ok := specs[model]
		if !ok {
			t.Fatalf("default table missing %q", model)
		}
		if spec.Window <= 0 || spec.ReserveTokens <= 0 || spec.Window <= spec.ReserveTokens {
			t.Fatalf("budget for %q is not a usable geometry: %+v", model, spec)
		}
	}
}

func TestBudgetSpecsFromEnvOverrideAppliesAndCoversUnknownModel(t *testing.T) {
	env := map[string]string{
		"CONTEXT_BUDGET_WINDOW_TOKENS":  "8192",
		"CONTEXT_BUDGET_RESERVE_TOKENS": "1024",
		"DEEPSEEK_MODEL":                "deepseek-v4-pro",
	}
	specs := BudgetSpecsFromEnv(func(k string) string { return env[k] })

	// The override rewrites the default entries...
	if got := specs["gpt-4o"]; got.Window != 8192 || got.ReserveTokens != 1024 {
		t.Fatalf("override did not rewrite gpt-4o: %+v", got)
	}
	// ...AND registers the actually-running non-default model, which the old
	// main.go helper did not do (AF1-c: the override used to be useless there).
	spec, ok := specs["deepseek-v4-pro"]
	if !ok {
		t.Fatal("window override failed to register a non-default running model")
	}
	if spec.Window != 8192 || spec.ReserveTokens != 1024 {
		t.Fatalf("registered geometry drifted: %+v", spec)
	}
}

func TestValidateModelCoverageRefusesUnregisteredModelByName(t *testing.T) {
	specs := BudgetSpecsFromEnv(func(string) string { return "" })

	// deepseek-v4-pro / deepseek-chat / deepseek-reasoner are all legal
	// DEEPSEEK_MODEL values (.env.example) absent from the default table.
	err := ValidateModelCoverage(specs, "deepseek-v4-reasoner")
	if err == nil {
		t.Fatal("unregistered running model must refuse startup coverage")
	}
	if !strings.Contains(err.Error(), "deepseek-v4-reasoner") {
		t.Fatalf("error must name the model so the operator can fix it: %v", err)
	}
	if !strings.Contains(err.Error(), "CONTEXT_BUDGET_WINDOW_TOKENS") {
		t.Fatalf("error must point at the remedy: %v", err)
	}

	if err := ValidateModelCoverage(specs, "deepseek-v4-flash"); err != nil {
		t.Fatalf("registered model refused: %v", err)
	}
}
