package aiagent

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

func TestRetailPaperRequested(t *testing.T) {
	req := Request{
		Message: "帮我把这周的经营情况出一份底稿",
		PageContext: &PageContext{Filters: map[string]string{
			"as_of": "2026-08-18", "window_days": "7", "data_classification": "production",
		}},
	}
	if !retailPaperRequested(req) {
		t.Fatal("底稿 request with valid filters must route to the paper tool")
	}
	plain := Request{Message: "今天客流怎么样？", PageContext: req.PageContext}
	if retailPaperRequested(plain) {
		t.Fatal("non-paper retail question must not route to the paper tool")
	}
	noFilters := Request{Message: "出一份底稿"}
	if retailPaperRequested(noFilters) {
		t.Fatal("paper request without as_of/classification/window must not route")
	}
}

func TestExecuteRetailPaperEndToEnd(t *testing.T) {
	reader := &agentRetailReader{set: agentRetailSet()}
	agent := NewAgent(AgentPorts{RetailKPI: reader})
	if agent.ToolRuntime() == nil {
		t.Fatal("agent runtime must be registered with the retail paper tool")
	}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}, Scope: access.Scope{LegalEntityID: "entity-a"}},
		RunID:     "run-retail-paper-1",
	})
	req := Request{
		Message: "出一份经营底稿",
		PageContext: &PageContext{Filters: map[string]string{
			"as_of": "2026-06-14", "window_days": "7", "classification": "simulated", "dataset_version": "planA-v1",
		}},
	}
	resp, err := agent.executeRetailPaper(ctx, req, func(ctx context.Context, eventType string, payload any) error { return nil }, agent.toolRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if resp.WorkingPaper == nil {
		t.Fatal("response must carry the retail working paper")
	}
	if len(resp.ReviewPrompts) == 0 {
		t.Fatal("review prompt required")
	}

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
		t.Fatal("ProjectResult must map the retail paper to a working_paper artifact")
	}
}

func TestExecuteRetailPaperMissingFiltersNeedsInput(t *testing.T) {
	reader := &agentRetailReader{set: agentRetailSet()}
	agent := NewAgent(AgentPorts{RetailKPI: reader})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}, Scope: access.Scope{LegalEntityID: "entity-a"}},
		RunID:     "run-retail-paper-2",
	})
	resp, err := agent.executeRetailPaper(ctx, Request{Message: "出一份底稿"}, func(ctx context.Context, eventType string, payload any) error { return nil }, agent.toolRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if resp.WorkingPaper != nil {
		t.Fatal("missing filters must not produce a paper")
	}
	if resp.RetailOperations == nil || !resp.RetailOperations.NeedsInput {
		t.Fatalf("missing filters must surface as needs-input guidance, got %+v", resp)
	}
}
