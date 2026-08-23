package tools

// TOOL-001: every InputSchema / OutputSchema literal in this package must be
// valid JSON.
//
// Why this guard exists. `fpna.assumptions.suggest` shipped with one extra
// closing brace in its InputSchema. `Registry.Register` rejected it with
// "input_schema must be a JSON object", but every registration site in
// aiagent/agent.go is written as `if err := registry.Register(d); err == nil`,
// so the error was discarded and the tool was silently absent from the runtime
// registry — documented in AGENTS.md, covered by its own unit tests, and never
// once reachable by the agent. Unit tests call the handler directly, so they
// pass regardless of whether registration succeeded.
//
// A source-level check is deliberate: it needs no constructor arguments, so it
// covers every definition in the package including ones whose ports are not
// wired in any given configuration, and it cannot drift out of date the way a
// hand-maintained list of constructors would.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSchemaLiteralsAreValidJSON(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"InputSchema", "OutputSchema"} {
			for _, literal := range rawMessageLiterals(string(src), field) {
				checked++
				var v any
				if err := json.Unmarshal([]byte(literal), &v); err != nil {
					t.Errorf("%s: %s literal is not valid JSON: %v\n%s", file, field, err, literal)
					continue
				}
				if _, ok := v.(map[string]any); !ok {
					t.Errorf("%s: %s literal is not a JSON object", file, field)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("guard found no schema literals — the extraction below stopped matching, fix it rather than deleting this test")
	}
	t.Logf("validated %d schema literals", checked)
}

// rawMessageLiterals returns the contents of every
// `<field>: json.RawMessage(`...`)` backtick literal in src. The gap after the
// colon is variable because gofmt aligns struct fields.
func rawMessageLiterals(src, field string) []string {
	re := regexp.MustCompile(field + ":\\s*json\\.RawMessage\\(`([^`]*)`\\)")
	matches := re.FindAllStringSubmatch(src, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}
