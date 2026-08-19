package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func cleanPaperJSON(t *testing.T) json.RawMessage {
	t.Helper()
	p := workingpaper.Build(workingpaper.Paper{
		Title:            "S1 签约前决策底稿",
		Period:           "2026-08",
		LegalEntityScope: "LE-1",
		Sections: []workingpaper.Section{
			{
				ID:    "overview",
				Title: "概览",
				Kind:  workingpaper.KindTable,
				Cells: []workingpaper.Cell{
					{
						Ref: "A1", Label: "年租金", Value: 1200000, Currency: "CNY",
						Provenance: workingpaper.Provenance{Basis: workingpaper.BasisSystemFact, SourceTable: "contracts", SourceRecordID: "C-1"},
					},
				},
			},
		},
	}, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRenderArtifactExportXLSXAndDOCX(t *testing.T) {
	raw := cleanPaperJSON(t)
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)

	xlsx, ct, err := RenderArtifactExport(raw, agentartifact.ArtifactWorkingPaper, "xlsx", now)
	if err != nil {
		t.Fatal(err)
	}
	if ct != exportContentTypeXLSX || !bytes.HasPrefix(xlsx, []byte("PK")) {
		t.Fatalf("xlsx export wrong: content-type=%s prefix=%x", ct, xlsx[:4])
	}

	docx, ct2, err := RenderArtifactExport(raw, agentartifact.ArtifactWorkingPaper, "docx", now)
	if err != nil {
		t.Fatal(err)
	}
	if ct2 != exportContentTypeDOCX || !bytes.HasPrefix(docx, []byte("PK")) {
		t.Fatalf("docx export wrong: content-type=%s prefix=%x", ct2, docx[:4])
	}
}

// The lint gate must fail closed: a paper carrying an exploratory protected
// measure cannot be exported, and the violations are surfaced.
func TestRenderArtifactExportLintGateFailsClosed(t *testing.T) {
	p := workingpaper.Build(workingpaper.Paper{
		Title: "bad",
		Sections: []workingpaper.Section{
			{
				ID: "s1", Kind: workingpaper.KindTable,
				Cells: []workingpaper.Cell{
					{
						Ref: "A1", Label: "期末租赁负债", MeasureID: "lease_liability", Value: 100,
						Provenance: workingpaper.Provenance{Basis: workingpaper.BasisExploratory, SandboxRunID: "r1"},
					},
				},
			},
		},
	}, time.Now())
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = RenderArtifactExport(raw, agentartifact.ArtifactWorkingPaper, "xlsx", time.Now())
	var lintErr *LintRejected
	if !errors.As(err, &lintErr) {
		t.Fatalf("expected LintRejected, got %v", err)
	}
	if len(lintErr.Violations) != 1 || !strings.Contains(lintErr.Violations[0].Code, "protected") {
		t.Fatalf("expected protected-measure violation, got %+v", lintErr.Violations)
	}
}

func TestRenderArtifactExportRejectsNonPaper(t *testing.T) {
	if _, _, err := RenderArtifactExport(json.RawMessage(`{}`), agentartifact.ArtifactGeneric, "xlsx", time.Now()); err == nil {
		t.Fatal("non-working-paper artifact must be rejected")
	}
	if _, _, err := RenderArtifactExport(cleanPaperJSON(t), agentartifact.ArtifactWorkingPaper, "pdf", time.Now()); err == nil {
		t.Fatal("unsupported format must be rejected")
	}
	if _, _, err := RenderArtifactExport(json.RawMessage(`{not json`), agentartifact.ArtifactWorkingPaper, "xlsx", time.Now()); err == nil {
		t.Fatal("broken artifact data must be rejected")
	}
}
