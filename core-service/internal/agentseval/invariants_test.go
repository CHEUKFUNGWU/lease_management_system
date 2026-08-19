package agentseval

import "testing"

func TestProductionInvariantCasesAllPass(t *testing.T) {
	cases, err := ProductionInvariantCases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 8 {
		t.Fatalf("expected a meaningful dataset, got %d cases", len(cases))
	}
	report := EvaluateInvariantCases(cases)
	if report.Failed > 0 {
		for _, r := range report.Results {
			if !r.Passed {
				t.Errorf("case %s failed: %s", r.CaseID, r.Detail)
			}
		}
		t.Fatalf("%d of %d invariant cases failed", report.Failed, report.Total)
	}
	if report.Passed != report.Total {
		t.Fatalf("report accounting broken: %+v", report)
	}
}

// The dataset must stay regression-safe: every case category is known and
// every provenance case asserts either a clean pass or the exact expected
// violation set.
func TestInvariantCaseShape(t *testing.T) {
	cases, err := ProductionInvariantCases()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		switch c.Category {
		case "provenance":
			if c.WorkingPaper == nil {
				t.Fatalf("case %s: working_paper required", c.ID)
			}
		case "triage_refusal":
			if c.Triage == nil || c.ExpectedDocClass == "" {
				t.Fatalf("case %s: triage + expected_doc_class required", c.ID)
			}
		case "s1_engine_consistency":
			if c.S1Input == nil {
				t.Fatalf("case %s: s1_input required", c.ID)
			}
		default:
			t.Fatalf("case %s: unknown category %q", c.ID, c.Category)
		}
	}
}
