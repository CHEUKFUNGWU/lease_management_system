package aiagent

// Test helpers for the W5-3 in-process intake parse path (GUARD-001: prove the
// new path works, not just that the Python hop is gone). A stub file reader
// serves the CORR-2-recorded inputs and a mock /chat/completions serves the
// recorded LLM response, so the producer produces deterministic drafts that
// the same consumption seam decodes.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lease-management-system/core-service/internal/docparse"
	"github.com/lease-management-system/core-service/internal/llm"
)

type corr2Case struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Input struct {
		Text        string `json:"text"`
		XLSXBase64  string `json:"xlsx_base64"`
		ContentType string `json:"content_type"`
		LLMResponse string `json:"llm_response"`
	} `json:"input"`
}

func loadCorr2(t *testing.T, name string) (inputText, contentType, llmResponse string, xlsxBytes []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "agentseval", "testdata", "corr2", name+".json"))
	if err != nil {
		t.Fatalf("read corr2 %s: %v", name, err)
	}
	var c corr2Case
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode corr2 %s: %v", name, err)
	}
	if c.Input.XLSXBase64 != "" {
		b, err := base64.StdEncoding.DecodeString(c.Input.XLSXBase64)
		if err != nil {
			t.Fatalf("decode xlsx: %v", err)
		}
		xlsxBytes = b
	}
	return c.Input.Text, c.Input.ContentType, c.Input.LLMResponse, xlsxBytes
}

// intakeLLMServer serves the recorded LLM response at /chat/completions.
func intakeLLMServer(t *testing.T, response string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Unexpected LLM path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":` + mustJSONString(response) + `}}]}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func mustJSONString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// textDocParser returns the raw bytes as markdown so PDF/docx fixture text can
// flow into the producer without a real anydoc binary in tests.
type textDocParser struct{}

func (textDocParser) Parse(_ context.Context, src docparse.Source) (docparse.ParsedDocument, error) {
	return docparse.ParsedDocument{Markdown: string(src.Data), Format: docparse.DetectFormat(src.Filename, src.Data)}, nil
}

// newIntakeAgent wires an Agent for the in-process producer path: a file
// reader returning the given bytes per object name and an LLM client pointed
// at the recorded-response server, plus the text doc parser.
func newIntakeAgent(t *testing.T, llmResponse string, files map[string][]byte) *Agent {
	t.Helper()
	server := intakeLLMServer(t, llmResponse)
	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	agent := &Agent{}
	agent.SetLLMClient(client)
	agent.SetDocumentParser(textDocParser{})
	agent.SetFileBytesReader(func(_ context.Context, objectName string) ([]byte, error) {
		if b, ok := files[objectName]; ok {
			return b, nil
		}
		return nil, os.ErrNotExist
	})
	return agent
}
