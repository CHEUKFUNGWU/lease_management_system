package newstorefeasibility

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportGuard 是 D-R4/RH4 的架构测试：newstorefeasibility 包——根或
// 任何子包——不得 import ifrs16 服务。租赁数字只能经 LeaseProjectionReader
// 投影端口进入。守卫照 finmodel/importguard_test.go 写，遍历全部子包
// 不只根包——那份测试当初就差点栽在只查根包上。
func TestImportGuard(t *testing.T) {
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain unavailable")
	}
	root := findModuleRoot(t)
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \",\"}}",
		"github.com/lease-management-system/core-service/internal/services/newstorefeasibility/...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		pkg := parts[0]
		for _, dep := range strings.Split(parts[1], ",") {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if strings.Contains(dep, "internal/services/ifrs16") {
				t.Fatalf("%s imports %s: lease numbers must enter only through the LeaseProjectionReader port", pkg, dep)
			}
		}
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}
