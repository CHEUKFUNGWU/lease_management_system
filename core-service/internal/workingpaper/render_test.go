package workingpaper

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// PROV-7 (I7): rendering the same paper twice must produce byte-identical
// output for both formats.
func TestRenderDeterminism(t *testing.T) {
	p := samplePaper()
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)

	x1, err := RenderXLSX(p, now)
	if err != nil {
		t.Fatal(err)
	}
	x2, err := RenderXLSX(p, now)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(x1, x2) {
		t.Fatalf("xlsx render must be deterministic: %d vs %d bytes", len(x1), len(x2))
	}

	d1, err := RenderDOCX(p, now)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := RenderDOCX(p, now)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d1, d2) {
		t.Fatal("docx render must be deterministic")
	}
}

// The xlsx output must reopen cleanly, carry the cover sheet, one sheet per
// section and the trailing _来源 sheet with every provenance row.
func TestRenderXLSXStructure(t *testing.T) {
	p := samplePaper()
	out, err := RenderXLSX(p, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("rendered xlsx must reopen: %v", err)
	}
	defer f.Close()

	if _, err := f.GetSheetIndex(coverSheetName); err != nil {
		t.Fatalf("cover sheet missing: %v", err)
	}
	if _, err := f.GetSheetIndex("overview"); err != nil {
		t.Fatalf("section sheet missing: %v", err)
	}
	if _, err := f.GetSheetIndex(sourceSheetName); err != nil {
		t.Fatalf("_来源 sheet missing: %v", err)
	}

	// _来源 covers every cell: 1 header row + 4 cells.
	rows, err := f.GetRows(sourceSheetName)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("_来源 must have 5 rows (header + 4 cells), got %d", len(rows))
	}

	// The certified cell carries its audit comment (PROV-8 automation).
	comments, err := f.GetComments("overview")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cm := range comments {
		if strings.Contains(cm.Text, "call-1") && strings.Contains(cm.Text, "certified") {
			found = true
		}
	}
	if !found {
		t.Fatalf("certified cell must carry a comment tracing to call-1, got %+v", comments)
	}
}

// The docx output must be a valid zip with a document.xml containing the
// title, the exploratory provenance and the appendix.
func TestRenderDOCXStructure(t *testing.T) {
	p := samplePaper()
	out, err := RenderDOCX(p, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("docx must be a valid zip: %v", err)
	}
	var docXML []byte
	for _, zf := range zr.File {
		if zf.Name == "word/document.xml" {
			rc, err := zf.Open()
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatal(err)
			}
			rc.Close()
			docXML = buf.Bytes()
		}
	}
	if docXML == nil {
		t.Fatal("document.xml missing from docx")
	}
	for _, want := range []string{"S1 签约前决策底稿", "run-1", "call-1", "来源附录", "人工确认数"} {
		if !bytes.Contains(docXML, []byte(want)) {
			t.Fatalf("document.xml must contain %q", want)
		}
	}
}

// A lint-dirty paper must not be exportable through the renderer path: the
// renderers themselves require a clean paper by contract (the export seam
// checks Lint first — see the design decision D-B).
func TestExploratoryRefsDriveWriteGuard(t *testing.T) {
	p := samplePaper()
	refs := p.ExploratoryRefs()
	guarded := make(map[string]bool)
	for _, r := range refs {
		guarded[r] = true
	}
	if !guarded["A3"] {
		t.Fatal("write-path assertion must see the exploratory cell")
	}
}
