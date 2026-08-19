package aiagent

import "testing"

func TestFoldExecutedIntoRunbookBackfillsCards(t *testing.T) {
	duration := int64(1234)
	runbook := &AgentRunbook{
		AgentPlan: []AgentPlanStep{
			{ID: "plan-1", Title: "解析文件", Status: "pending"},
			{ID: "plan-2", Title: "生成草稿", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "lease.file.parse_contract", Status: "pending"},
			{Tool: "lease.contract.draft.create", Status: "pending"},
		},
	}
	executed := []AgentToolCall{
		{Tool: "lease.file.parse_contract", Status: "completed", DurationMs: &duration, OutputSummary: "按合同解析完成"},
	}
	plan, calls := foldExecutedIntoRunbook(runbook, executed)

	if plan[0].Status != "completed" || plan[1].Status != "completed" {
		t.Fatalf("plan steps must reflect that work executed, got %+v", plan)
	}
	if calls[0].Tool != "lease.file.parse_contract" || calls[0].Status != "completed" || calls[0].DurationMs == nil || *calls[0].DurationMs != 1234 {
		t.Fatalf("executed card must carry the real record, got %+v", calls[0])
	}
	if calls[1].Status != "pending" {
		t.Fatalf("unexecuted card must stay pending — it did not run: %+v", calls[1])
	}
}

func TestFoldExecutedMarksFailure(t *testing.T) {
	runbook := &AgentRunbook{
		AgentPlan: []AgentPlanStep{{ID: "p1", Title: "解析", Status: "pending"}},
		ToolCalls: []AgentToolCall{{Tool: "lease.file.parse_contract", Status: "pending"}},
	}
	plan, _ := foldExecutedIntoRunbook(runbook, []AgentToolCall{
		{Tool: "lease.file.parse_contract", Status: "failed", OutputSummary: "解析失败"},
	})
	if plan[0].Status != "failed" {
		t.Fatalf("a failed execution must surface on the plan, got %+v", plan[0])
	}
}

func TestFoldExecutedNoExecutionKeepsDisplayCards(t *testing.T) {
	runbook := &AgentRunbook{
		AgentPlan: []AgentPlanStep{{ID: "p1", Title: "解析", Status: "pending"}},
		ToolCalls: []AgentToolCall{{Tool: "lease.file.parse_contract", Status: "pending"}},
	}
	plan, calls := foldExecutedIntoRunbook(runbook, nil)
	if plan[0].Status != "pending" || calls[0].Status != "pending" {
		t.Fatalf("without execution the cards stay honest pending: %+v %+v", plan, calls)
	}
}
