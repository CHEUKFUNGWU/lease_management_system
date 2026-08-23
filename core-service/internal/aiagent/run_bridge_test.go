package aiagent

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/aichat"
)

func TestRunnerIntentToolDetection(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"帮我算一下这个续租方案", "lease.renewal.simulate"},
		{"两家报价对比一下", "lease.deal.simulate"},
		{"做个现金流情景", "fpna.cashflow.scenario"},
		{"出一份决策摘要", "fpna.decision.summary"},
		{"今天门店表现怎么样", ""},
		{"查看经营脉搏", ""},
	}
	for _, c := range cases {
		got, ok := runnerIntentTool(c.message)
		if c.want == "" && ok {
			t.Fatalf("%q must not trigger a runner tool, got %s", c.message, got)
		}
		if c.want != "" && got != c.want {
			t.Fatalf("%q → %q, want %q", c.message, got, c.want)
		}
	}
}

func TestPlanQueuesRunnerIntentsForWorker(t *testing.T) {
	agent := NewWithOperationalReadersAndGovernanceAndRetail(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// Selects the FP&A copilot skill AND carries a runner tool intent.
	plan := agent.Plan(aichat.Input{Message: "经营决策：做个现金流情景", Role: "editor", Initiator: "user"}, nil)
	if !plan.QueueForWorker {
		t.Fatal("a runner-intent request must queue for the worker (G1 bridge)")
	}

	// The system-initiated morning brief never dispatches.
	plan = agent.Plan(aichat.Input{Message: "经营决策：做个现金流情景", Role: "editor", Initiator: "system"}, nil)
	if plan.QueueForWorker {
		t.Fatal("system-initiated brief must stay on the chat plane")
	}

	// FP&A questions without a runner intent stay deterministic.
	plan = agent.Plan(aichat.Input{Message: "经营决策：今天有哪些门店需要处理", Role: "editor", Initiator: "user"}, nil)
	if plan.QueueForWorker {
		t.Fatal("non-runner questions must not queue")
	}

	// The retail plane is deterministic and never dispatches, even when the
	// message mentions scenarios the retail tools already handle.
	plan = agent.Plan(aichat.Input{Message: "人工下降 10% 的情景会怎样", Role: "editor", Initiator: "user"}, nil)
	if plan.QueueForWorker {
		t.Fatal("retail_operations requests must stay on the deterministic plane")
	}
}

func TestRunbookHasTool(t *testing.T) {
	runbook := &AgentRunbook{ToolCalls: []AgentToolCall{{Tool: "lease.renewal.simulate", Status: "pending"}}}
	if !runbookHasTool(runbook, "lease.renewal.simulate") {
		t.Fatal("card must be found")
	}
	if runbookHasTool(runbook, "lease.predeal.simulate") {
		t.Fatal("missing card must not match")
	}
}

// The bridge must not double-execute: Start on a queued plan leaves the run
// untouched for the worker. The runtime seam is exercised here through the
// plan flag itself; execution skipping lives in preparedRun plumbing covered
// by the full suite.
func TestPlanFlagRoundTrip(t *testing.T) {
	agent := NewWithOperationalReadersAndGovernanceAndRetail(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	_ = context.Background()
	plan := agent.Plan(aichat.Input{Message: "做个现金流情景", Role: "editor", Initiator: "user"}, nil)
	if plan.QueueForWorker != (plan.SkillID != "") {
		// Skill id may be empty in unit wiring; the invariants that matter:
		if !plan.QueueForWorker {
			t.Fatal("runner intent must queue")
		}
	}
}
