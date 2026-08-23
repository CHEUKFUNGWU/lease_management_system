package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Architecture guard (ADR-0026 §5): nothing under internal/gateway/** —
// vendor included, once it lands — may construct access.Scope or mention
// the tenant column. The channel layer has no materials to assemble
// permissions; its only way in is Resolve. Same mechanism as the finmodel /
// agentcore import guards: with one human and a fleet of agents, controls
// that depend on someone noticing do not hold.
var (
	bannedScopeLiteral  = "access.Scope{"
	bannedTenantLiteral = "legal_entity_id"
)

func gatewayRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// scanGatewaySources walks dir for non-test Go files and reports every file
// and line carrying a banned shape.
func scanGatewaySources(dir string) []string {
	violations := []string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
		for index, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
				continue // comments may discuss the ban itself
			}
			if strings.Contains(line, bannedScopeLiteral) {
				violations = append(violations, path+":"+itoa(index+1)+" constructs access.Scope")
			}
			if strings.Contains(line, bannedTenantLiteral) {
				violations = append(violations, path+":"+itoa(index+1)+" mentions legal_entity_id")
			}
		}
		return nil
	})
	return violations
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// TestGatewayHasNoScopeAssemblyMaterials scans the real package tree.
func TestGatewayHasNoScopeAssemblyMaterials(t *testing.T) {
	root := gatewayRoot(t)
	violations := scanGatewaySources(root)
	if len(violations) > 0 {
		t.Fatalf("channel layer must not assemble permissions (ADR-0026 §3/§5):\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestGatewayGuardDetectsViolations is the reverse test: the scanner must
// catch every banned shape when one actually appears in a source file under
// this tree. It writes throwaway fixtures next to the real sources and then
// deletes them, so CI sees only the clean tree.
func TestGatewayGuardDetectsViolations(t *testing.T) {
	fixtures := []struct {
		name    string
		content string
	}{
		{
			name:    "zz_fixture_scope_literal.go",
			content: "package gateway\n\nimport \"github.com/lease-management-system/core-service/internal/access\"\n\nvar _ = access.Scope{Global: true}\n",
		},
		{
			name:    "zz_fixture_tenant_literal.go",
			content: "package gateway\n\nconst probe = \"legal_entity_id\"\n",
		},
	}
	for _, fixture := range fixtures {
		path := filepath.Join(gatewayRoot(t), fixture.name)
		if err := os.WriteFile(path, []byte(fixture.content), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		defer os.Remove(path)
	}

	violations := scanGatewaySources(gatewayRoot(t))
	if len(violations) < len(fixtures) {
		t.Fatalf("guard failed to detect planted violations: %+v", violations)
	}
	sawScope, sawTenant := false, false
	for _, violation := range violations {
		if strings.Contains(violation, "constructs access.Scope") {
			sawScope = true
		}
		if strings.Contains(violation, "mentions legal_entity_id") {
			sawTenant = true
		}
	}
	if !sawScope || !sawTenant {
		t.Fatalf("guard missed shapes: scope=%v tenant=%v in %+v", sawScope, sawTenant, violations)
	}
}

// filepathAbs / writeFile / removeFile are small os shims shared with the
// runner tests.
func filepathAbs(rel string) (string, error) {
	return filepath.Abs(rel)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func removeFile(path string) {
	_ = os.Remove(path)
}

// bannedBusinessImports lists the packages vendored code must never depend on;
// the dependency direction is outer-wraps-vendor only (ADR-0026 §5).
var bannedBusinessImports = []string{
	"internal/repository",
	"internal/services",
	"internal/agenttools",
	"internal/access",
	"internal/handlers",
	"internal/middleware",
}

// scanThirdPartyImports walks dir for non-test Go files importing business
// packages.
func scanThirdPartyImports(dir string) []string {
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
			for index, line := range strings.Split(string(content), "\n") {
				if strings.Contains(line, `"github.com/lease-management-system/core-service/`+banned+`"`) ||
					strings.Contains(line, `"github.com/lease-management-system/core-service/`+banned+`/`) {
					violations = append(violations, path+":"+itoa(index+1)+" imports "+banned)
				}
			}
		}
		return nil
	})
	return violations
}
