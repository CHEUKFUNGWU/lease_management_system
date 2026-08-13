package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// escapeHatchPattern matches the `($N=” OR legal_entity_id::text=$N)`
// signature that SEC-003 eliminated: an empty-string legal-entity parameter
// that short-circuits to "no filtering". The value type makes the shape
// unrepresentable in code; this test is the CI-able belt-and-braces guard
// that catches anyone reintroducing it in repository SQL.
var escapeHatchPattern = regexp.MustCompile(`''\s*OR\s+legal_entity_id`)

// TestNoLegalEntityEscapeHatchInRepositorySQL scans the repository package
// sources (non-test files) for the reintroduced escape hatch. It must stay
// green even after the 38 replacement sites were converted to
// access.EntityFilter.
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
