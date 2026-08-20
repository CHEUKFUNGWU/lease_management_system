package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The parity fixtures were recorded BEFORE this Go implementation existed:
// each .json is a raw OpenAI-compatible chat-completions response, and each
// .expected.json is what ai-service's old path produced from that response —
// answer/model via app/routers/chat.py (answer = choices[0].message.content,
// model = f"{provider}/{model}") and usage via app/services/llm.py's
// usage_metadata. The test below proves the Go client reproduces the old
// path's output bit for bit (GUARD-001: the replacement actually works).
func TestParityWithRecordedPythonPath(t *testing.T) {
	cfg := Config{
		Provider: "deepseek", APIKey: "test-key",
		BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	}
	for _, name := range []string{
		"parity-chat-plain",
		"parity-chat-tools",
		"parity-chat-cost-measured",
		"parity-chat-no-usage",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
			if err != nil {
				t.Fatalf("read raw fixture: %v", err)
			}
			expectedRaw, err := os.ReadFile(filepath.Join("testdata", name+".expected.json"))
			if err != nil {
				t.Fatalf("read expected fixture: %v", err)
			}

			result, err := ParseChatCompletion(raw, cfg)
			if err != nil {
				t.Fatalf("ParseChatCompletion: %v", err)
			}

			var expected struct {
				Answer        string          `json:"answer"`
				Model         string          `json:"model"`
				ToolCalls     json.RawMessage `json:"tool_calls"`
				UsageMetadata json.RawMessage `json:"usage_metadata"`
			}
			if err := json.Unmarshal(expectedRaw, &expected); err != nil {
				t.Fatalf("decode expected fixture: %v", err)
			}
			if result.Answer != expected.Answer {
				t.Errorf("answer differs:\n  go  = %q\n  old = %q", result.Answer, expected.Answer)
			}
			if result.Model != expected.Model {
				t.Errorf("model differs:\n  go  = %q\n  old = %q", result.Model, expected.Model)
			}

			gotToolCalls, err := json.Marshal(result.ToolCalls)
			if err != nil {
				t.Fatalf("marshal tool calls: %v", err)
			}
			if !jsonEqual(gotToolCalls, expected.ToolCalls) {
				t.Errorf("tool_calls differ:\n  go  = %s\n  old = %s", gotToolCalls, expected.ToolCalls)
			}

			gotUsage, err := json.Marshal(result.Usage)
			if err != nil {
				t.Fatalf("marshal usage: %v", err)
			}
			if !jsonEqual(gotUsage, expected.UsageMetadata) {
				t.Errorf("usage_metadata differs:\n  go  = %s\n  old = %s", gotUsage, expected.UsageMetadata)
			}
		})
	}
}

func jsonEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	aj, _ := json.Marshal(av)
	bj, _ := json.Marshal(bv)
	return string(aj) == string(bj)
}