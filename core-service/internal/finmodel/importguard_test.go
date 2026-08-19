package finmodel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportGuard is the D-S3 architecture test: the finmodel engine must
// never import the ifrs16 service — lease numbers enter only through the
// LeaseRollforwardReader projection port. No amount of code review replaces
// this check.
func TestImportGuard(t *testing.T) {
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain unavailable")
	}
	root := findModuleRoot(t)
	cmd := exec.Command("go", "list", "-f", "{{range .Deps}}{{println .}}{{end}}",
		"github.com/lease-management-system/core-service/internal/finmodel")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "internal/services/ifrs16") {
			t.Fatalf("finmodel must not import the ifrs16 service (D-S3), found %s", line)
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
