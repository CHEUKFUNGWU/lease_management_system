package agentcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportGuard is ACORE-1: the agentcore package must not import
// database/sql, net/http, any repository or the object-store client. It uses
// `go list` so transitive imports are covered, and skips when the Go toolchain
// is unavailable in the test environment.
func TestImportGuard(t *testing.T) {
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain unavailable")
	}
	root := findModuleRoot(t)
	cmd := exec.Command("go", "list", "-f", "{{range .Deps}}{{println .}}{{end}}",
		"github.com/lease-management-system/core-service/internal/agentcore")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}
	forbidden := []string{
		"database/sql",
		"net/http",
		"github.com/lease-management-system/core-service/internal/repository",
		"github.com/minio",
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, f := range forbidden {
			if line == f || strings.HasPrefix(line, f+"/") {
				t.Fatalf("agentcore must not depend on %s (found %s)", f, line)
			}
		}
	}
}

func findModuleRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found")
	return ""
}
