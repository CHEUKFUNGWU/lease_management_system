package aiagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	llm "github.com/lease-management-system/core-service/internal/llm"
)

// round-one asks for the triage tool; round-two must observe the folded
// tool-result row in the request body before answering.
func TestAgentFilePipelineRunsMultipleRounds(t *testing.T) {
	var round atomic.Int32
	var secondRoundSawToolResult atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		n := round.Add(1)
		switch n {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id":   "call-1",
							"type": "function",
							"function": map[string]any{
								"name":      "lease.file.triage",
								"arguments": `{"file_id":"f1","object_name":"o.xlsx","content_type":"spreadsheet"}`,
							},
						}},
					},
				}},
			})
		default:
			if strings.Contains(string(body), fileResultMarker) {
				secondRoundSawToolResult.Store(true)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "这是门店日事实 Excel，请去 /retail-data-import 确认导入"}}},
			})
		}
	}))
	defer server.Close()

	client, err := llm.NewClient(llm.Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "test-model", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	triageDef := agenttooldefs.NewDocTriageDefinition(nil)
	registry := agenttools.NewRegistry()
	if err := registry.Register(triageDef); err != nil {
		t.Fatalf("register triage: %v", err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	loop := NewAgentLLMLoop(client, runtime)
	loop.defs = []agenttools.ToolDefinition{triageDef}

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "u1", Role: "admin", Permissions: []string{"*:*"}},
		RunID:     "run-file-loop",
		SkillID:   "excel_ledger",
	})
	answer, chain, runErr := loop.Run(ctx, "分诊系统提示", nil, "帮我看看这个文件")
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	if !secondRoundSawToolResult.Load() {
		t.Fatal("round two never saw the folded tool result — multi-round is not real")
	}
	if answer == "" || !strings.Contains(answer, "/retail-data-import") {
		t.Fatalf("final answer missing: %q", answer)
	}
	foundTriage := false
	for _, call := range chain {
		if call.Tool == "lease.file.triage" {
			foundTriage = true
		}
	}
	if !foundTriage {
		t.Fatalf("triage call missing from executed chain: %+v", chain)
	}
}
