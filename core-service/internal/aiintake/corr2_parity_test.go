package aiintake

// W5-3 门 A parity gate: replay every recorded CORR-2 baseline input through
// the Go producer and compare the envelope against the OLD Python producer's
// expected output. This test never executes Python and never calls an LLM —
// the recorded llm_response is injected verbatim. A mismatch here means the
// migration regressed a business rule; the task instruction forbids merging
// with this red.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type corr2Record struct {
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Input    corr2Input      `json:"input"`
	Expected json.RawMessage `json:"expected"`
}

type corr2Input struct {
	Text        string `json:"text"`
	XLSXBase64  string `json:"xlsx_base64"`
	ContentType string `json:"content_type"`
	ContractID  string `json:"contract_id"`
	LLMResponse string `json:"llm_response"`
}

// fixedCompleter returns the recorded LLM response verbatim.
type fixedCompleter struct{ response string }

func (f fixedCompleter) Complete(context.Context, string, string, float64, int, map[string]any) (string, error) {
	return f.response, nil
}

func loadCorr2Records(t *testing.T) []corr2Record {
	t.Helper()
	dir := filepath.Join("..", "agentseval", "testdata", "corr2")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corr2 dir: %v", err)
	}
	var records []corr2Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "manifest.json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var r corr2Record
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("decode %s: %v", e.Name(), err)
		}
		records = append(records, r)
	}
	return records
}

func TestCorr2ParityGoReproducesPythonBaseline(t *testing.T) {
	records := loadCorr2Records(t)
	if len(records) == 0 {
		t.Fatal("no CORR-2 fixtures recorded")
	}
	failures := 0
	for _, rec := range records {
		t.Run(rec.Name, func(t *testing.T) {
			// Build source material.
			material := SourceMaterial{Text: rec.Input.Text, ContentType: rec.Input.ContentType}
			if rec.Input.XLSXBase64 != "" {
				bytes, err := base64.StdEncoding.DecodeString(rec.Input.XLSXBase64)
				if err != nil {
					t.Fatalf("decode xlsx_base64: %v", err)
				}
				text, records, locators, err := readExcelContracts(bytes)
				if err != nil {
					t.Fatalf("read excel: %v", err)
				}
				material.Text = text
				material.FileData = bytes
				material.DeterministicRecords = records
				material.EvidenceLocators = locators
			}
			cmd := Command(rec.Kind, "file-"+rec.Name, rec.Name+extFor(rec.Input.ContentType), rec.Input.ContentType, rec.Input.ContractID)
			got, err := Produce(context.Background(), sizedProductMode, rec.Kind, cmd, material, fixedCompleter{response: rec.Input.LLMResponse})

			var expected map[string]any
			if err := json.Unmarshal(rec.Expected, &expected); err != nil {
				t.Fatalf("decode expected: %v", err)
			}
			if expectedErr, ok := expected["error"]; ok {
				// The old path hard-rejected this input; the Go producer must too.
				if err == nil {
					t.Fatalf("expected the old path's rejection (%v) but Go produced an output", expectedErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Go producer errored where old path produced output: %v", err)
			}

			// intake_id is a runtime identifier; normalize before comparing.
			got["intake_id"] = expected["intake_id"]
			// JSON round-trip both sides so Go's typed slices ([]string) and the
			// baseline's decoded []any compare as the same shape.
			var gotNorm, expectedNorm any
			gotJSON, _ := json.Marshal(got)
			expectedJSON, _ := json.Marshal(expected)
			if err := json.Unmarshal(gotJSON, &gotNorm); err != nil {
				t.Fatalf("normalize Go output: %v", err)
			}
			if err := json.Unmarshal(expectedJSON, &expectedNorm); err != nil {
				t.Fatalf("normalize expected: %v", err)
			}
			if !reflect.DeepEqual(gotNorm, expectedNorm) {
				t.Fatalf("parity mismatch:\n--- Go ---\n%s\n--- baseline ---\n%s",
					jsonIndent(got), jsonIndent(expected))
			}
		})
		if t.Failed() {
			// count per-run failures via the subtests
		}
	}
	_ = failures
}

func extFor(contentType string) string {
	if strings.Contains(contentType, "spreadsheetml") || contentType == "application/vnd.ms-excel" {
		return ".xlsx"
	}
	return ".pdf"
}

func jsonIndent(v map[string]any) string {
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
}
