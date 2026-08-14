package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// escapeHatchPattern matches the legal-entity escape-hatch signatures that
// SEC-003/SEC-004 eliminated: an empty-string legal-entity parameter that
// short-circuits to "no filtering". The value type makes the shapes
// unrepresentable in code; this test is the CI-able belt-and-braces guard
// that catches anyone reintroducing them in repository SQL.
//
// The pattern is deliberately broader than the plain `legal_entity_id` form:
// SEC-004 widened it because the original shapes carried table aliases
// (`v.` / `l.` / `s.`) and a COALESCE wrapper, all of which sit between the
// `OR` and the column name. Every form is pinned by
// TestEscapeHatchPatternCapturesEveryKnownForm below.
var escapeHatchPattern = regexp.MustCompile(`''\s*OR\s+[^)]*legal_entity_id`)

// TestEscapeHatchPatternCapturesEveryKnownForm pins the four shapes the
// guard must catch. A shape listed here is a regression test for the guard
// itself, not for the repository sources.
func TestEscapeHatchPatternCapturesEveryKnownForm(t *testing.T) {
	forms := map[string]string{
		"plain":    `($2='' OR legal_entity_id::text=$2)`,
		"alias-v":  `($2='' OR v.legal_entity_id::text=$2)`,
		"alias-l":  `($2='' OR l.legal_entity_id::text=$2)`,
		"coalesce": `($2 = '' OR COALESCE(legal_entity_id::text, '') = $2)`,
	}
	for name, form := range forms {
		if !escapeHatchPattern.MatchString(form) {
			t.Errorf("escape hatch form %q (%s) is not captured by the guard", name, form)
		}
	}
}

// TestNoLegalEntityEscapeHatchInRepositorySQL scans the repository package
// sources (non-test files) for any reintroduced escape-hatch shape.
func TestNoLegalEntityEscapeHatchInRepositorySQL(t *testing.T) {
	matches := []string{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for lineIndex, line := range strings.Split(string(content), "\n") {
			if escapeHatchPattern.MatchString(line) {
				matches = append(matches, path+":"+fmt.Sprint(lineIndex+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan repository sources: %v", err)
	}
	if len(matches) > 0 {
		t.Fatalf("legal-entity escape hatch reintroduced at:\n  %s\n", strings.Join(matches, "\n  "))
	}
}
