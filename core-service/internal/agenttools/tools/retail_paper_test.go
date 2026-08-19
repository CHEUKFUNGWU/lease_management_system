package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// fakeFactReader implements RetailOperationsReader deterministically.
type fakeFactReader struct{}

func (fakeFactReader) QueryFacts(ctx context.Context, legalEntityID, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (*repository.RetailKPIFactSet, error) {
	// A minimal fact set: the services only need population + a handful of
	// fact rows to aggregate (a full coverage graduate set per store-day is
	// too heavy for this test — aggregate correctness is the services' own
	// test domain; here we assert the paper carries whatever engines emit).
	return &repository.RetailKPIFactSet{
		ExpectedStoreCount: 1,
		ExpectedStores:     []retailkpi.StorePopulation{{StoreID: storeUUID, StoreCode: "ST001", StoreName: "旗舰店"}},
		MinFactVersion:     1, MaxFactVersion: 1,
	}, nil
}

const storeUUID = "11111111-1111-4111-8111-111111111111"

func paperArguments(t *testing.T, withStore bool) json.RawMessage {
	t.Helper()
	payload := map[string]any{
		"as_of":               "2026-08-18",
		"window_days":         7,
		"data_classification": "production",
	}
	if withStore {
		payload["store_id"] = storeUUID
		payload["horizon_months"] = 12
		payload["confirmed_by"] = "bp-zhang"
		payload["confirmed_at"] = "2026-08-19T10:00:00Z"
		payload["assumptions"] = map[string]any{
			"revenue_change_pct":                 0,
			"gross_margin_rate_change_pp":        1.5,
			"labor_cost_change_pct":              -10,
			"fixed_rent_change_pct":              -5,
			"variable_rent_rate_change_pp":       0,
			"non_lease_cost_change_pct":          0,
			"other_controllable_cost_change_pct": 0,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func paperExecutionContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "bp-zhang",
			Scope:  access.Scope{LegalEntityID: "LE-1"},
		},
		RunID: "run-retail-paper",
	})
}

func TestRetailPaperToolDefinition(t *testing.T) {
	def := NewRetailPaperDefinition(fakeFactReader{})
	if def.Descriptor.Name != "retail.working_paper.store.generate" || def.Descriptor.Level != agenttools.LevelDraft {
		t.Fatalf("descriptor wrong: %+v", def.Descriptor)
	}

	call := agenttools.ToolCall{
		CallID:    "call-retail-paper-1",
		ToolName:  "retail.working_paper.store.generate",
		Arguments: paperArguments(t, true),
	}
	result, err := def.Handler(paperExecutionContext(), call)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["side_effects"] != false {
		t.Fatalf("paper data must declare no side effects, got %+v", result.Data)
	}
	paper, ok := data["paper"].(workingpaper.Paper)
	if !ok {
		t.Fatalf("paper missing: %+v", result.Data)
	}
	if len(paper.Sections) < 2 {
		t.Fatalf("expected scope+pulse sections at minimum, got %d sections", len(paper.Sections))
	}
	for _, c := range paper.AllCells() {
		if c.Provenance.Basis == workingpaper.BasisCertified && c.Provenance.ToolCallID != "call-retail-paper-1" {
			t.Fatalf("cell %s must trace to the paper tool call", c.Ref)
		}
		if c.Provenance.Basis == workingpaper.BasisExploratory {
			t.Fatalf("retail paper must carry no exploratory cells, cell %s", c.Ref)
		}
	}
}

// Portfolio paper: no store_id still produces the base paper, and the
// missing diagnostics/scenario are recorded as gaps.
func TestRetailPaperToolPortfolioScope(t *testing.T) {
	def := NewRetailPaperDefinition(fakeFactReader{})
	call := agenttools.ToolCall{
		CallID:    "call-retail-paper-2",
		ToolName:  "retail.working_paper.store.generate",
		Arguments: paperArguments(t, false),
	}
	result, err := def.Handler(paperExecutionContext(), call)
	if err != nil {
		t.Fatal(err)
	}
	paper := result.Data.(map[string]any)["paper"].(workingpaper.Paper)
	found := false
	for _, g := range paper.DataGaps {
		if len(g) > 0 && (stringContains(g, "情景测算缺失") || stringContains(g, "store_id")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("portfolio paper must record the missing store-level sections: %v", paper.DataGaps)
	}
}

func TestRetailPaperToolValidation(t *testing.T) {
	def := NewRetailPaperDefinition(fakeFactReader{})
	// Missing context fields.
	if _, err := def.Handler(paperExecutionContext(), agenttools.ToolCall{Arguments: json.RawMessage(`{"as_of":"2026-08-18"}`)}); err == nil {
		t.Fatal("missing window/classification must fail")
	}
	// production + dataset_version is contradictory.
	bad := paperArguments(t, false)
	if _, err := def.Handler(paperExecutionContext(), agenttools.ToolCall{Arguments: replaceJSON(bad, `"data_classification": "production"`, `"data_classification": "production", "dataset_version": "ds-1"`)}); err == nil {
		t.Fatal("production with dataset_version must fail")
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func replaceJSON(raw json.RawMessage, old, new string) json.RawMessage {
	return json.RawMessage(strings.ReplaceAll(string(raw), old, new))
}
