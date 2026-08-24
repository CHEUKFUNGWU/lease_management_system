package agentkernel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Architecture guard for the agent-kernel vendor slice (ADR-0028 §1/§5):
// vendored code must not import first-party business packages — the dependency
// direction is outer-wraps-vendor only. Same mechanism as the gateway and
// finmodel guards.

var bannedBusinessImports = []string{
	"internal/repository",
	"internal/services",
	"internal/agenttools",
	"internal/access",
	"internal/handlers",
	"internal/middleware",
	"internal/aichat",
}

func thirdPartyRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("third_party")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestVendorDoesNotImportBusinessPackages scans every non-test Go file under
// the agentkernel third_party tree.
func TestVendorDoesNotImportBusinessPackages(t *testing.T) {
	violations := scanImports(thirdPartyRoot(t))
	if len(violations) > 0 {
		t.Fatalf("vendored kernel code must not import business packages:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// Reverse fixture: plant a business import beside real sources, require red,
// then remove it so CI sees only the clean tree.
func TestVendorImportGuardDetectsViolations(t *testing.T) {
	path := filepath.Join(thirdPartyRoot(t), "picoclaw", "zz_fixture_business_import.go")
	fixture := "package picoclaw\n\nimport _ \"github.com/lease-management-system/core-service/internal/repository\"\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	violations := scanImports(thirdPartyRoot(t))
	if len(violations) == 0 {
		t.Fatal("vendor import guard failed to detect a planted business import")
	}
}

func scanImports(dir string) []string {
	violations := []string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, banned := range bannedBusinessImports {
			pattern := `"github.com/lease-management-system/core-service/` + banned + `"`
			for index, line := range strings.Split(string(content), "\n") {
				if strings.Contains(line, pattern) {
					violations = append(violations,
						path+":"+strings.Repeat("", 0)+fmtInt(index+1)+" imports "+banned)
				}
			}
		}
		return nil
	})
	return violations
}

func fmtInt(v int) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}
