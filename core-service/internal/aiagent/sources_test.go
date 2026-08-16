package aiagent

import "testing"

// M6.2 citation fidelity: citations are exactly the model-declared sources
// intersecting the known set — never widened to "all known sources".
func TestExtractSourcesFromAnswerCitationFidelity(t *testing.T) {
	known := []Source{
		{ID: "contract-1", Title: "租赁合同A", Snippet: "合同记录 2026"},
		{ID: "report-1", Title: "负债滚动表", Snippet: "2026-01"},
	}
	cited := extractSourcesFromAnswer("结论如下（来源：租赁合同A）", known)
	if len(cited) != 1 || cited[0].ID != "contract-1" {
		t.Fatalf("cited=%+v", cited)
	}
	// No citations → empty, never all known sources.
	if got := extractSourcesFromAnswer("结论如下", known); len(got) != 0 {
		t.Fatalf("no-citation answer returned sources: %+v", got)
	}
	// Citations matching nothing → empty, never widened.
	if got := extractSourcesFromAnswer("结论如下（来源：不存在的文件）", known); len(got) != 0 {
		t.Fatalf("unmatched citation widened: %+v", got)
	}
	// Empty known set stays empty.
	if got := extractSourcesFromAnswer("结论如下（来源：租赁合同A）", nil); len(got) != 0 {
		t.Fatalf("empty known set returned sources: %+v", got)
	}
}
