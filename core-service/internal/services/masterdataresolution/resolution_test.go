package masterdataresolution

import (
	"context"
	"testing"
)

type mockPersistentCache struct {
	confirmed map[string]Candidate
}

func (m *mockPersistentCache) GetConfirmedMappings(ctx context.Context, kind EntityKind, raws []string) (map[string]Candidate, error) {
	out := make(map[string]Candidate)
	for _, r := range raws {
		if c, ok := m.confirmed[r]; ok {
			out[r] = c
		}
	}
	return out, nil
}

type mockCountingSource struct {
	calledWith []string
	rules      map[string]Candidate
}

func (m *mockCountingSource) Suggest(ctx context.Context, kind EntityKind, unknownRaws []string) ([]Candidate, error) {
	m.calledWith = unknownRaws
	var res []Candidate
	for _, r := range unknownRaws {
		if c, ok := m.rules[r]; ok {
			res = append(res, c)
		}
	}
	return res, nil
}

func TestResolve_CostConvergenceAndCacheHit(t *testing.T) {
	ctx := context.Background()

	// Persistent cache already has "SKU_001" confirmed
	cache := &mockPersistentCache{
		confirmed: map[string]Candidate{
			"SKU_001": {RawIdentifier: "SKU_001", CanonicalID: "CANON_001", CanonicalName: "有机牛奶", Confidence: 1.0},
		},
	}

	source := &mockCountingSource{
		rules: map[string]Candidate{
			"SKU_002": {RawIdentifier: "SKU_002", CanonicalID: "CANON_002", CanonicalName: "全脂牛奶", Confidence: 0.95, Source: "ai"},
			"SKU_003": {RawIdentifier: "SKU_003", CanonicalID: "CANON_003", CanonicalName: "未知商品", Confidence: 0.60, Source: "ai"}, // Below threshold 0.85
		},
	}

	raws := []string{"SKU_001", "SKU_002", "SKU_003", "SKU_004"}
	res, err := Resolve(ctx, KindSKU, raws, cache, source, 0.85)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Verify source was ONLY called with unknown items: SKU_002, SKU_003, SKU_004 (NOT SKU_001)
	if len(source.calledWith) != 3 {
		t.Fatalf("expected source called with 3 unknowns, got %d", len(source.calledWith))
	}

	// 2. SKU_001 must be resolved from cache
	c1, ok1 := res.Resolved["SKU_001"]
	if !ok1 || c1.Source != "cached" {
		t.Fatalf("expected SKU_001 resolved as cached, got %+v", c1)
	}

	// 3. SKU_002 must be resolved from AI (confidence 0.95 >= 0.85)
	c2, ok2 := res.Resolved["SKU_002"]
	if !ok2 || c2.CanonicalID != "CANON_002" {
		t.Fatalf("expected SKU_002 resolved as CANON_002, got %+v", c2)
	}

	// 4. SKU_003 must be in Ambiguous (confidence 0.60 < 0.85)
	if len(res.Ambiguous) != 1 || res.Ambiguous[0].RawIdentifier != "SKU_003" {
		t.Fatalf("expected SKU_003 in ambiguous, got %+v", res.Ambiguous)
	}

	// 5. SKU_004 must be in Unknown
	foundUnknown := false
	for _, u := range res.Unknown {
		if u == "SKU_004" {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		t.Fatalf("expected SKU_004 in unknown list")
	}
}
