package aiintake

// Unit tests for the ten migrated business rules (W5-3). Every rule that gates
// human confirmation or evidence must have a reverse test: the validation must
// be provable red, or a future deletion of the check would pass silently.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleDiscountRateMissing(t *testing.T) {
	// positive: a contract with a rate is not flagged
	missing, warnings := checkDiscountRateMissing(map[string]any{"discount_rate_type": "ibr", "discount_rate": 3.5})
	if missing || len(warnings) != 0 {
		t.Fatalf("present discount rate must not be flagged: %v %v", missing, warnings)
	}
	missing, warnings = checkDiscountRateMissing(map[string]any{"discount_rate": 0.035})
	if missing || len(warnings) != 0 {
		t.Fatalf("numeric discount_rate must satisfy the check")
	}
	// reverse: absent rate (and no type) must be flagged
	missing, warnings = checkDiscountRateMissing(map[string]any{"contract_number": "C-1", "currency": "CNY"})
	if !missing {
		t.Fatal("contract without discount rate must be flagged as discount_rate_missing")
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "AI 不得猜测折现率") {
		t.Fatalf("warnings must carry the AI-must-not-guess message, got %v", warnings)
	}
}

func TestRuleCurrencyMissing(t *testing.T) {
	if missing, _ := checkCurrencyMissing(map[string]any{"currency": "CNY"}); missing {
		t.Fatal("CNY must not be flagged")
	}
	for _, bad := range []string{"unknown", "null", "none", ""} {
		if missing, _ := checkCurrencyMissing(map[string]any{"currency": bad}); !missing {
			t.Fatalf("currency %q must be flagged", bad)
		}
	}
}

func TestRuleCriticalFields(t *testing.T) {
	missing, warnings := checkCriticalFields(map[string]any{"contract_number": "C-1", "lessee": "a", "lessor": "b"})
	joined := strings.Join(missing, ",")
	if !strings.Contains(joined, "commencement_date") || !strings.Contains(joined, "payment_timing") {
		t.Fatalf("critical missing detection incomplete: %q", joined)
	}
	if len(warnings) != len(missing) {
		t.Fatalf("one warning per missing field expected: %d vs %d", len(warnings), len(missing))
	}
}

func TestRuleNormalizeLeaseScope(t *testing.T) {
	m := map[string]any{"suggested_scope": "not_a_lease", "scope_confidence": 0.95}
	scope, warnings := normalizeLeaseScope(m)
	if scope != "not_a_lease" || len(warnings) != 0 {
		t.Fatalf("valid scope must survive: scope=%q warnings=%v", scope, warnings)
	}
	if m["scope_source"] != "ai_suggested" {
		t.Fatalf("scope must be marked ai_suggested: %v", m["scope_source"])
	}
	// reverse: invalid value -> in_scope + warning
	m2 := map[string]any{"suggested_scope": "maybe", "scope_confidence": 0.9}
	scope, warnings = normalizeLeaseScope(m2)
	if scope != "in_scope" || len(warnings) == 0 {
		t.Fatalf("invalid scope must default to in_scope with a warning: %q %v", scope, warnings)
	}
	// reverse: low confidence -> must-confirm warning
	m3 := map[string]any{"suggested_scope": "in_scope", "scope_confidence": 0.4}
	scope, warnings = normalizeLeaseScope(m3)
	if scope != "in_scope" || len(warnings) == 0 {
		t.Fatalf("low scope confidence must add a confirmation warning: %q %v", scope, warnings)
	}
}

func TestRuleSanitizeConfidence(t *testing.T) {
	if got := sanitizeConfidence(1.5); got != 1.0 {
		t.Fatalf(">1 must be clamped: %v", got)
	}
	if got := sanitizeConfidence(-2.0); got != 0.0 {
		t.Fatalf("negative must be clamped: %v", got)
	}
	if got := sanitizeConfidence(0.77); got != 0.77 {
		t.Fatalf("in-range must pass: %v", got)
	}
	scores := sanitizeConfidenceScores(map[string]any{"a": 150.0, "b": -3.0, "c": 0.5})
	if scores["a"] != 1.0 || scores["b"] != 0.0 || scores["c"] != 0.5 {
		t.Fatalf("sanitize scores: %v", scores)
	}
}

func TestRuleEvidenceQuoteMatches(t *testing.T) {
	if !evidenceQuoteMatches("承租方：示例零售有限公司", "  承租方：\n示例零售有限公司  （乙方）") {
		t.Fatal("whitespace-tolerant quote must match")
	}
	// reverse: a fabricated quote the source never contained must be rejected
	if evidenceQuoteMatches("押金三倍赔偿条款", "承租方：示例零售有限公司") {
		t.Fatal("fabricated quote must be rejected (底3 来源追溯)")
	}
	if evidenceQuoteMatches("", "text") || evidenceQuoteMatches("text", "") {
		t.Fatal("empty quotes must never match")
	}
}

func TestRuleResolveEvidenceOnlyAgainstAdapters(t *testing.T) {
	available := []EvidenceLocator{
		{Field: "x", Source: "page1", Page: intPtr(1), Quote: "租赁合同 编号 001"},
	}
	// positive: proposed quote matches an adapter locator and coordinates are
	// copied from the adapter, not the model.
	resolved := resolveLLMEvidence([]any{
		map[string]any{"field": "extracted_data.contract_number", "page": 1, "quote": "租赁合同编号001"},
	}, available)
	if len(resolved) != 1 || resolved[0].Source != "page1" {
		t.Fatalf("matched evidence must copy the adapter source: %+v", resolved)
	}
	// reverse: a quote (or page) that matches nothing is rejected
	resolved = resolveLLMEvidence([]any{
		map[string]any{"field": "extracted_data.contract_number", "page": 1, "quote": "完全伪造的引用"},
	}, available)
	if len(resolved) != 0 {
		t.Fatalf("unmatched quote must be rejected: %+v", resolved)
	}
	resolved = resolveLLMEvidence([]any{
		map[string]any{"field": "extracted_data.contract_number", "page": 2, "quote": "租赁合同 编号 001"},
	}, available)
	if len(resolved) != 0 {
		t.Fatalf("page mismatch must reject the locator: %+v", resolved)
	}
}

func TestRuleValidatePaymentSchedules(t *testing.T) {
	parsed := map[string]any{"schedules": []any{
		map[string]any{"period_start": "2026-01-01", "period_end": "2026-01-31", "due_date": "2026-01-01", "amount": 1000.0, "payment_timing": "prepaid"},
		map[string]any{"period_start": "2026-02-01", "period_end": "2026-02-28", "due_date": "2026-02-01", "amount": -100.0, "payment_timing": "prepaid"},
		map[string]any{"period_start": "2026-03-01", "period_end": "2026-03-31", "due_date": "2026-03-01", "amount": "not-a-number", "payment_timing": "prepaid"},
		map[string]any{"period_start": "2026-04-01", "period_end": "2026-04-30", "due_date": "2026-04-01", "amount": 500.0, "payment_timing": ""},
		map[string]any{"period_start": "2026-05-01", "period_end": "2026-05-31", "due_date": "2026-5-1", "amount": 600.0, "payment_timing": "prepaid"},
	}, "overall_confidence": 0.9}
	schedules, missing, warnings := validatePaymentSchedules(parsed)
	if len(schedules) != 2 {
		t.Fatalf("only the two valid prepaid rows survive: %d", len(schedules))
	}
	joined := strings.Join(warnings, "\n")
	for _, want := range []string{"金额 <= 0", "金额无法解析为数字", "缺少有效付款时点", "日期格式可能不正确"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warning %q missing in %s", want, joined)
		}
	}
	if !containsStr(missing, "payment_timing") {
		t.Fatal("missing payment_timing must be surfaced")
	}
}

func TestRuleFallbackParsePaymentScheduleText(t *testing.T) {
	text := "| 覆盖期间起始日 | 覆盖期间结束日 | 应付日期 | 金额 | 付款时点 | 金额类型 |\n" +
		"| 2026-01-01 | 2026-01-31 | 2026-01-01 | 50,000 | 先付 | fixed_rent |\n"
	parsed := fallbackParsePaymentScheduleText(text, "llm-down")
	scheds := asList(parsed["schedules"])
	if len(scheds) != 1 {
		t.Fatalf("table fallback must find the row: %d", len(scheds))
	}
	first := asMap(scheds[0])
	if first["amount"].(float64) != 50000 || first["payment_timing"] != "prepaid" {
		t.Fatalf("fallback row parse wrong: %+v", first)
	}
	if _, ok := parsed["warnings"]; !ok {
		t.Fatal("fallback must warn the reader to review")
	}
	// reverse: unrecognized header -> no schedules + missing columns
	parsed = fallbackParsePaymentScheduleText("日期\n2026-01-01\n", "llm-down")
	if len(asList(parsed["schedules"])) != 0 {
		t.Fatal("unrecognized header must yield zero schedules")
	}
}

func TestRuleApplyExcelEvidenceSafetyChecks(t *testing.T) {
	c := map[string]any{"contract_number": "L-E1", "termination_option": true, "renewal_option": true}
	text := "Row 1: A1=L-E1 | H1=合同终止选择权：不行使；续租选择权：合同到期后不会续租"
	out := applyExcelEvidenceSafetyChecks(c, text)
	if out["termination_option"] != false || out["renewal_option"] != false {
		t.Fatalf("negation clauses must flip the options: %v", out)
	}
	// reverse: a row without negation keywords keeps the model's claim
	c2 := map[string]any{"contract_number": "L-E2", "termination_option": true, "renewal_option": true}
	out = applyExcelEvidenceSafetyChecks(c2, "Row 2: A2=L-E2 | H2=合同正常履行，无终止/续租条款")
	if out["termination_option"] != true || out["renewal_option"] != true {
		t.Fatalf("absent negation must not flip options: %v", out)
	}
}

func TestRuleAssistModeGate(t *testing.T) {
	_, err := Produce(t.Context(), "auto-post", "contract", nil, SourceMaterial{}, nil)
	if err == nil || !strings.Contains(err.Error(), "Assist Mode") {
		t.Fatalf("auto-post mode must be rejected before any adapter runs: %v", err)
	}
}

func TestContractFixturesStillValidate(t *testing.T) {
	// The ai-intake.v1 contract fixtures must continue to decode through the
	// consumption side (契约不变，只换实现方 — 决策登记 §1.3).
	cases := []struct {
		name   string
		decode func(t *testing.T, raw []byte) error
	}{
		{"contract", func(t *testing.T, raw []byte) error {
			_, err := DecodeContract(closer{bytes.NewReader(raw)})
			return err
		}},
		{"contract-batch", func(t *testing.T, raw []byte) error {
			_, err := DecodeContractBatch(closer{bytes.NewReader(raw)})
			return err
		}},
		{"payment-schedule", func(t *testing.T, raw []byte) error {
			_, err := DecodePaymentSchedule(closer{bytes.NewReader(raw)})
			return err
		}},
		{"event", func(t *testing.T, raw []byte) error { _, err := DecodeEvent(closer{bytes.NewReader(raw)}); return err }},
	}
	for _, c := range cases {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", c.name+".json"))
		if err != nil {
			t.Fatalf("%s fixture read: %v", c.name, err)
		}
		if err := c.decode(t, raw); err != nil {
			t.Fatalf("fixture %s must keep validating: %v", c.name, err)
		}
	}
}

type closer struct{ *bytes.Reader }

func (c closer) Close() error { return nil }

func intPtr(v int) *int { return &v }

func containsStr(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
