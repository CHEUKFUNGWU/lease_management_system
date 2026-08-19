package pagefill

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func TestExploratoryCannotEnterPayload(t *testing.T) {
	f := New("retail-data-import", "POST /retail/operating-facts/store-days/import/preview", "/retail-data-import")
	err := f.PutPayload("mapping", map[string]string{"col": "revenue"}, workingpaper.Provenance{Basis: workingpaper.BasisExploratory})
	if err == nil {
		t.Fatal("Exploratory must be refused at the payload gate (I5)")
	}
	f.Suggest("mapping", map[string]string{"col": "revenue"}, workingpaper.Provenance{Basis: workingpaper.BasisExploratory})
	if len(f.Suggestions) != 1 {
		t.Fatal("suggestions are the only legal home for Exploratory")
	}
	if len(f.Payload) != 0 {
		t.Fatalf("payload must stay empty after two refused writes: %+v", f.Payload)
	}
}

func TestConfirmPromotesWithHumanInput(t *testing.T) {
	f := New("retail-data-import", "POST /retail/operating-facts/store-days/import/preview", "/x")
	f.Suggest("mapping", map[string]string{"col": "revenue"}, workingpaper.Provenance{Basis: workingpaper.BasisExploratory})
	f.PutPayload("source_system", "pos-a", workingpaper.Provenance{Basis: workingpaper.BasisHumanInput, ConfirmedBy: "bp"})
	if refs := f.ExploratoryRefs(); len(refs) != 1 || refs[0] != "mapping" {
		t.Fatalf("ExploratoryRefs must list the suggestion, got %v", refs)
	}
	if err := f.Confirm("mapping", map[string]string{"col": "revenue"}, "bp-zhang", "2026-08-19T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if len(f.Suggestions) != 0 {
		t.Fatalf("confirmed suggestion must leave the suggestion area: %+v", f.Suggestions)
	}
	v, ok := f.Payload["mapping"]
	if !ok || v.Provenance.Basis != workingpaper.BasisHumanInput || v.Provenance.ConfirmedBy != "bp-zhang" {
		t.Fatalf("confirmed field must carry HumanInput provenance, got %+v", v)
	}
	if refs := f.ExploratoryRefs(); len(refs) != 0 {
		t.Fatalf("confirmed field must not stay exploratory: %v", refs)
	}
	if err := f.AssertNoExploratoryInPayload(); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmRequiresPendingSuggestion(t *testing.T) {
	f := New("retail-data-import", "POST /x", "/x")
	if err := f.Confirm("never-suggested", 1, "bp", "now"); err == nil {
		t.Fatal("confirming a field without a suggestion must fail")
	}
	if err := f.Confirm("mapping", 1, "", "now"); err == nil {
		t.Fatal("confirming without a confirmer must fail")
	}
}

func TestAssertNoExploratoryGuardsCommitSeam(t *testing.T) {
	f := New("retail-data-import", "POST /x", "/x")
	// Simulate a corrupted payload that somehow carries Exploratory — the
	// commit-seam assertion must catch it (ACORE-12).
	f.Payload["mapping"] = FillValue{Value: 1, Provenance: workingpaper.Provenance{Basis: workingpaper.BasisExploratory, SandboxRunID: "r"}}
	if err := f.AssertNoExploratoryInPayload(); err == nil {
		t.Fatal("commit-seam assertion must refuse Exploratory payload values")
	}
}

func TestValidateEnvelope(t *testing.T) {
	f := New("", "", "/x")
	if err := f.Validate(); err == nil {
		t.Fatal("missing target must fail validation")
	}
	f = New("retail-data-import", "POST /x", "/x")
	f.SchemaVersion = "wrong"
	if err := f.Validate(); err == nil {
		t.Fatal("wrong schema version must fail")
	}
	f = New("retail-data-import", "POST /x", "/x")
	f.PutPayload("a", 1, workingpaper.Provenance{Basis: workingpaper.BasisSystemFact})
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
}
