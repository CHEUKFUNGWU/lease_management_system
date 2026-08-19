package aiagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
)

const s1MessageBlock = `帮我算一下这个签约前方案：
{"draft":{"name":"方案A","commencement_date":"2027-01-01T00:00:00Z","term_months":36,"monthly_rent":50000,"rent_free_months":2,"annual_escalation_percent":3,"discount_rate":0.0485,"currency":"CNY","initial_direct_cost":80000,"early_exit_penalty_months":3},"shocks_percent":[-0.01,0.005]}`

func TestExtractS1Input(t *testing.T) {
	if _, ok := extractS1Input(s1MessageBlock); !ok {
		t.Fatal("valid assumption block must be detected")
	}
	// The discount rate is never guessed: AI must not proceed without it.
	noRate := `{"draft":{"name":"X","commencement_date":"2027-01-01T00:00:00Z","term_months":36,"monthly_rent":50000}}`
	if _, ok := extractS1Input(noRate); ok {
		t.Fatal("block without discount_rate must not trigger the S1 path")
	}
	if _, ok := extractS1Input("今天天气不错"); ok {
		t.Fatal("plain chat must not trigger the S1 path")
	}
	if _, ok := extractS1Input("{\"draft\":{\"commencement_date\":\"2027-01-01T00:00:00Z\",\"monthly_rent\":5,\"discount_rate\":0.05,\"term_months\":12}} extra"); !ok {
		t.Fatal("trailing text must not break detection")
	}
}

func TestExecuteS1PaperEndToEnd(t *testing.T) {
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttooldefs.NewS1GenerateDefinition()); err != nil {
		t.Fatal(err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	agent := &Agent{}

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}},
		RunID:     "run-s1-1",
	})
	block, ok := extractS1Input(s1MessageBlock)
	if !ok {
		t.Fatal("block missing")
	}
	resp, err := agent.executeS1Paper(ctx, Request{Message: s1MessageBlock}, block, func(ctx context.Context, eventType string, payload any) error { return nil }, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if resp.WorkingPaper == nil {
		t.Fatal("response must carry the working paper")
	}
	if len(resp.WorkingPaper.Sections) < 4 {
		t.Fatalf("expected IFRS/bridge/exit/sensitivity sections, got %d", len(resp.WorkingPaper.Sections))
	}
	// Review gate: the draft-level paper must surface as review-gated.
	if len(resp.ReviewPrompts) == 0 {
		t.Fatal("review prompt required for the S1 paper")
	}
	// The paper lands as a working_paper artifact through ProjectResult.
	projected := ProjectResult(resp)
	var found bool
	for _, a := range projected.Artifacts {
		if string(a.Type) == "working_paper" {
			found = true
			if !a.ReviewRequired {
				t.Fatal("working_paper artifact must require review")
			}
		}
	}
	if !found {
		t.Fatal("ProjectResult must map the paper to a working_paper artifact")
	}
}

func TestExecuteS1PaperBrokenDraftNoDiscountedRateGuess(t *testing.T) {
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttooldefs.NewS1GenerateDefinition()); err != nil {
		t.Fatal(err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	agent := &Agent{}

	block := json.RawMessage(`{"draft":{"name":"坏方案","commencement_date":"2027-01-01T00:00:00Z","term_months":0,"monthly_rent":100,"discount_rate":0.05,"currency":"CNY"}}`)
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}},
		RunID:     "run-s1-2",
	})
	resp, err := agent.executeS1Paper(ctx, Request{Message: "算一下"}, block, func(ctx context.Context, eventType string, payload any) error { return nil }, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if resp.WorkingPaper != nil {
		t.Fatal("broken draft must not produce a paper")
	}
	if !strings.Contains(resp.Answer, "失败") {
		t.Fatalf("failure must be surfaced honestly, got %q", resp.Answer)
	}
}
