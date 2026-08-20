package agentseval

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// corr2 fixture replay is a pure read of input + expected: no Python is
// executed, no LLM is called. This is the W5-2 fixture-self-sufficiency gate
// (the task instruction's "门 A 一条断言 fixture 自足的测试"). W5-3 adds the
// actual parity comparison between the Go producer's output and `expected`.

type corr2Case struct {
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Input    json.RawMessage `json:"input"`
	Expected json.RawMessage `json:"expected"`
}

type corr2Manifest struct {
	SchemaVersion string   `json:"schema_version"`
	Defined       int      `json:"defined"`
	Recorded      int      `json:"recorded"`
	Cases         []string `json:"cases"`
}

func TestCorr2BaselineSelfContained(t *testing.T) {
	dir := filepath.Join("testdata", "corr2")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corr2 testdata: %v", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no corr2 fixtures recorded — run ai-service/scripts/record_corr2_baseline.py")
	}

	manifestRaw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("missing manifest.json: %v", err)
	}
	var manifest corr2Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("manifest decode: %v", err)
	}
	if manifest.SchemaVersion != "corr2.v1" {
		t.Fatalf("manifest schema_version = %q, want corr2.v1", manifest.SchemaVersion)
	}

	// Fail-loud audit: the number of recorded cases must equal the defined
	// count AND the number of fixture files. A silent skip would make the
	// manifest self-reported count look healthy while a fixture is missing.
	fixtureFiles := 0
	kindCount := map[string]int{}
	for _, name := range files {
		if name == "manifest.json" {
			continue
		}
		fixtureFiles++
		var c corr2Case
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s decode: %v", name, err)
		}
		if c.Name == "" || c.Kind == "" {
			t.Fatalf("%s: name/kind required", name)
		}
		if len(c.Input) == 0 || len(c.Expected) == 0 {
			t.Fatalf("%s: both input and expected are required (fixture self-sufficiency)", name)
		}
		assertCorr2InputShape(t, name, c.Input)
		kindCount[c.Kind]++
	}
	if fixtureFiles != manifest.Recorded || manifest.Recorded != manifest.Defined {
		t.Fatalf("fixture audit: files=%d recorded=%d defined=%d — a silent skip is hiding in the recorder",
			fixtureFiles, manifest.Recorded, manifest.Defined)
	}
	if len(manifest.Cases) != fixtureFiles {
		t.Fatalf("manifest case list = %d, actual fixtures = %d", len(manifest.Cases), fixtureFiles)
	}

	// Each of the four kinds must carry >= 10 cases (门 A requirement).
	for _, kind := range []string{"contract", "payment_schedule", "event", "contract_batch"} {
		if kindCount[kind] < 10 {
			t.Errorf("kind %s has %d cases (< 10)", kind, kindCount[kind])
		}
	}

	// The review's anti-pattern guard: a baseline of only positive cases would
	// let W5-3 delete its validation logic and still pass. At least one case
	// per kind must assert evidence rejection (negative quote) or an explicit
	// error, and the discount/scope negatives must exist.
	for _, kind := range []string{"contract", "payment_schedule", "event", "contract_batch"} {
		if !hasKindNegativeShape(t, dir, kind) {
			t.Errorf("kind %s: no negative-evidence or expected-error fixture found — a 全正例 baseline is not a gate", kind)
		}
	}
	for _, want := range []string{"contract-no-discount-rate", "contract-no-currency", "contract-missing-critical",
		"contract-invalid-scope", "contract-unsafe-confidence", "contract-low-scope-confidence"} {
		if _, err := os.Stat(filepath.Join(dir, want+".json")); err != nil {
			t.Errorf("required negative fixture %s missing: %v", want, err)
		}
	}
}

func assertCorr2InputShape(t *testing.T, name string, input json.RawMessage) {
	t.Helper()
	var in struct {
		Text        string `json:"text"`
		XLSXBase64  string `json:"xlsx_base64"`
		ContentType string `json:"content_type"`
		ContractID  string `json:"contract_id"`
		LLMResponse string `json:"llm_response"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		t.Fatalf("%s input decode: %v", name, err)
	}
	if in.ContentType == "" || in.LLMResponse == "" {
		t.Fatalf("%s: input must carry content_type and llm_response", name)
	}
	if in.Text == "" && in.XLSXBase64 == "" {
		t.Fatalf("%s: input must carry text or xlsx_base64", name)
	}
	if in.XLSXBase64 != "" {
		bytes, err := base64.StdEncoding.DecodeString(in.XLSXBase64)
		if err != nil || len(bytes) == 0 {
			t.Fatalf("%s: xlsx_base64 must be decodable to non-empty bytes", name)
		}
		if !strings.HasPrefix(in.XLSXBase64, "UEs") { // PK\x03\x04 zip magic
			t.Fatalf("%s: xlsx_base64 must be a real xlsx (zip) payload", name)
		}
	}
}

// hasKindNegativeShape reports whether the kind has a fixture whose expected
// output is an explicit error, or whose evidence rejects a proposed locator.
func hasKindNegativeShape(t *testing.T, dir, kind string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || e.Name() == "manifest.json" {
			continue
		}
		var c corr2Case
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		if c.Kind != kind {
			continue
		}
		var exp struct {
			Error    string `json:"error"`
			Evidence struct {
				Complete bool `json:"complete"`
			} `json:"evidence"`
		}
		if err := json.Unmarshal(c.Expected, &exp); err != nil {
			continue
		}
		if exp.Error != "" {
			return true
		}
		if (kind == "contract" && strings.Contains(e.Name(), "negative-evidence")) ||
			(kind == "payment_schedule" && strings.Contains(e.Name(), "negative-evidence")) ||
			(kind == "event" && strings.Contains(e.Name(), "negative-evidence")) ||
			(kind == "contract_batch" && strings.Contains(e.Name(), "negative-evidence")) {
			if !exp.Evidence.Complete {
				return true
			}
		}
	}
	return false
}

func TestCorr2GoldenPromptsRecorded(t *testing.T) {
	dir := filepath.Join("testdata", "corr2", "golden")
	for _, name := range []string{"prompt-contract", "prompt-payment_schedule", "prompt-event", "prompt-contract_batch"} {
		raw, err := os.ReadFile(filepath.Join(dir, name+".golden.txt"))
		if err != nil {
			t.Fatalf("%s golden file missing (door B): %v", name, err)
		}
		if len(raw) == 0 {
			t.Fatalf("%s golden file is empty", name)
		}
	}
}
