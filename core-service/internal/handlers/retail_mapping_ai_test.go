package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/llm"
	"github.com/lease-management-system/core-service/internal/services/retailingest"
)

// W5-4 GUARD-001: the mapping suggestion now runs through the in-process LLM
// client. A mock /chat/completions returns a mapping object and the adapter
// must (a) parse it, (b) strip unknown/out-of-list fields to null, (c) never
// leak raw column values to the wire (D13 — only masked profiles go out).

func TestRetailMappingAIUsesInProcessLLM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("mapping must use the in-process LLM client: got %q", r.URL.Path)
		}
		// The request body must carry column profiles (header + counts +
		// masked sample), NOT raw values (D13).
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		for _, want := range []string{"store", "masked_sample", "含门店"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("request body missing %q in %s", want, body)
			}
		}
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"门店名称\":\"store\",\"营业额\":\"revenue\",\"未知列\":\"totally_invalid_field\",\"日期\":null}"}}]}`))
	}))
	defer server.Close()
	client, err := llm.NewClient(llm.Config{Provider: "deepseek", APIKey: "k", BaseURL: server.URL, Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	mapping, err := NewRetailMappingAI().WithClient(client).SuggestMapping(context.Background(),
		[]string{"门店名称", "营业额", "日期", "未知列"},
		[]retailingest.ColumnProfile{
			{Header: "门店名称", NonEmpty: 12, MaskedSample: "示例*店"},
			{Header: "营业额", NonEmpty: 12, Numeric: 12},
			{Header: "日期", DateLike: 12},
			{Header: "未知列", Numeric: 5},
		})
	if err != nil {
		t.Fatalf("SuggestMapping: %v", err)
	}
	if mapping["门店名称"] != "store" || mapping["营业额"] != "revenue" {
		t.Fatalf("mapping = %#v", mapping)
	}
	// 日期 null → the adapter must not invent a mapping; 未知列 → field not in
	// the standard list must be dropped entirely.
	if _, ok := mapping["日期"]; ok {
		t.Fatalf("null mapping must not surface: %#v", mapping)
	}
	if _, ok := mapping["未知列"]; ok {
		t.Fatalf("out-of-list field must be dropped: %#v", mapping)
	}
}

func TestRetailMappingAIFailsClosedWithoutClient(t *testing.T) {
	if _, err := (&RetailMappingAI{}).SuggestMapping(context.Background(), []string{"a"}, nil); err == nil {
		t.Fatal("a nil LLM client must fail closed (old path 502)")
	}
}

func TestRetailMappingAIRestyleFencedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"```json\\n{\\\"店铺\\\":\\\"store\\\"}\\n```\"}}]}\n"))
	}))
	defer server.Close()
	client, err := llm.NewClient(llm.Config{Provider: "deepseek", APIKey: "k", BaseURL: server.URL, Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	mapping, err := NewRetailMappingAI().WithClient(client).SuggestMapping(context.Background(), []string{"店铺"}, []retailingest.ColumnProfile{{Header: "店铺"}})
	if err != nil {
		t.Fatalf("SuggestMapping (fenced): %v", err)
	}
	if mapping["店铺"] != "store" {
		t.Fatalf("fenced JSON must parse: %#v", mapping)
	}
}
