package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/leasescenario"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func s1Arguments(t *testing.T) json.RawMessage {
	t.Helper()
	payload := map[string]any{
		"input": map[string]any{
			"draft": map[string]any{
				"name":                      "方案A",
				"commencement_date":         "2027-01-01T00:00:00Z",
				"term_months":               36,
				"monthly_rent":              50000,
				"rent_free_months":          2,
				"annual_escalation_percent": 3,
				"discount_rate":             0.0485,
				"currency":                  "CNY",
				"initial_direct_cost":       80000,
				"early_exit_penalty_months": 3,
			},
			"shocks_percent": []float64{-0.01, 0.005},
			"confirmed_by":   "bp-zhang",
			"confirmed_at":   "2026-08-19T10:00:00Z",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestS1GenerateToolProducesCleanPaper(t *testing.T) {
	def := NewS1GenerateDefinition()
	call := agenttools.ToolCall{
		CallID:    "call-s1-tool-1",
		ToolName:  "lease.working_paper.s1.generate",
		Arguments: s1Arguments(t),
	}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang"},
		RunID:     "run-1",
	})
	result, err := def.Handler(ctx, call)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["side_effects"] != false {
		t.Fatalf("tool data must declare no side effects, got %+v", result.Data)
	}
	paper, ok := data["paper"].(workingpaper.Paper)
	if !ok {
		t.Fatalf("paper missing from tool data: %+v", result.Data)
	}
	if len(paper.Sections) < 4 {
		t.Fatalf("expected IFRS/bridge/exit/sensitivity sections, got %d", len(paper.Sections))
	}
	// I2: every certified cell must trace to THIS audited call.
	for _, c := range paper.AllCells() {
		if c.Provenance.Basis == workingpaper.BasisCertified && c.Provenance.ToolCallID != "call-s1-tool-1" {
			t.Fatalf("cell %s must trace to the tool call, got %s", c.Ref, c.Provenance.ToolCallID)
		}
	}
	// I3/I5: protected measures certified, zero exploratory.
	for _, c := range paper.AllCells() {
		if c.MeasureID == "lease_liability" || c.MeasureID == "rou_asset" || c.MeasureID == "discount_rate_applied" {
			if c.Provenance.Basis != workingpaper.BasisCertified {
				t.Fatalf("protected cell %s must be Certified, got %s", c.Ref, c.Provenance.Basis)
			}
		}
		if c.Provenance.Basis == workingpaper.BasisExploratory {
			t.Fatalf("S1 paper must contain no exploratory cells, cell %s", c.Ref)
		}
	}
}

func TestS1GenerateToolConfirmerFallback(t *testing.T) {
	payload := map[string]any{
		"input": map[string]any{
			"draft": map[string]any{
				"name":              "方案B",
				"commencement_date": "2027-02-01T00:00:00Z",
				"term_months":       24,
				"monthly_rent":      40000,
				"discount_rate":     0.05,
				"currency":          "CNY",
			},
		},
	}
	raw, _ := json.Marshal(payload)
	def := NewS1GenerateDefinition()
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-li"},
		RunID:     "run-1",
	})
	call := agenttools.ToolCall{CallID: "call-s1-tool-2", ToolName: "lease.working_paper.s1.generate", Arguments: raw}
	result, err := def.Handler(ctx, call)
	if err != nil {
		t.Fatal(err)
	}
	paper := result.Data.(map[string]any)["paper"].(workingpaper.Paper)
	for _, c := range paper.AllCells() {
		if c.Provenance.Basis == workingpaper.BasisHumanInput && c.Provenance.ConfirmedBy != "bp-li" {
			t.Fatalf("human cells must carry the authenticated caller as confirmer, cell %s got %q", c.Ref, c.Provenance.ConfirmedBy)
		}
	}
}

func TestS1GenerateToolRejectsBrokenDraft(t *testing.T) {
	def := NewS1GenerateDefinition()
	payload := map[string]any{
		"input": map[string]any{
			"draft": map[string]any{
				"name":              "坏方案",
				"commencement_date": "2027-01-01T00:00:00Z",
				"term_months":       0, // engine validation must reject
				"monthly_rent":      100,
				"discount_rate":     0.05,
				"currency":          "CNY",
			},
			"confirmed_by": "bp-zhang",
			"confirmed_at": "2026-08-19T10:00:00Z",
		},
	}
	raw, _ := json.Marshal(payload)
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u"}})
	if _, err := def.Handler(ctx, agenttools.ToolCall{CallID: "c", Arguments: raw}); err == nil {
		t.Fatal("invalid draft must fail through the engine, not be papered over")
	}
}

// engineConsistencyForTest anchors CORR-1's engine-consistency evaluation
// entries: given identical input, the tool's paper cells must equal a direct
// engine run.
func engineConsistencyForTest(t *testing.T) {
	t.Helper()
	in := leasescenario.Draft{
		Name:                    "方案C",
		CommencementDate:        time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC),
		TermMonths:              36,
		MonthlyRent:             50000,
		RentFreeMonths:          2,
		AnnualEscalationPercent: 3,
		DiscountRate:            0.0485,
		Currency:                "CNY",
	}
	payload := map[string]any{
		"input": map[string]any{
			"draft": map[string]any{
				"name":                      in.Name,
				"commencement_date":         in.CommencementDate.Format(time.RFC3339),
				"term_months":               in.TermMonths,
				"monthly_rent":              in.MonthlyRent,
				"rent_free_months":          in.RentFreeMonths,
				"annual_escalation_percent": in.AnnualEscalationPercent,
				"discount_rate":             in.DiscountRate,
				"currency":                  in.Currency,
			},
			"confirmed_by": "bp-zhang",
			"confirmed_at": "2026-08-19T10:00:00Z",
		},
	}
	raw, _ := json.Marshal(payload)
	def := NewS1GenerateDefinition()
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "u"}})
	result, err := def.Handler(ctx, agenttools.ToolCall{CallID: "call-x", Arguments: raw})
	if err != nil {
		t.Fatal(err)
	}
	paper := result.Data.(map[string]any)["paper"].(workingpaper.Paper)
	direct, err := leasescenario.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range paper.AllCells() {
		if c.Ref == "IF-1" && c.Value.(float64) != direct.BalanceSheet.InitialLiability {
			t.Fatalf("tool paper IF-1 = %v, direct engine says %v", c.Value, direct.BalanceSheet.InitialLiability)
		}
	}
}
