package finmodel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportGuard is the D-S3 architecture test: no finmodel package — root
// or any subpackage (adapter/persist/suggestion/memo/opening/template/view) —
// may import the ifrs16 service. Lease numbers enter only through the
// LeaseRollforwardReader projection port. No amount of code review replaces
// this check.
func TestImportGuard(t *testing.T) {
	_, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain unavailable")
	}
	root := findModuleRoot(t)
	// .Imports（直接依赖）而不是 .Deps（传递闭包）：adapter 等子包经
	// repository 读计量行，repository 自己的 ifrs16 转换助手不属于 finmodel
	// 直接 import（D-S3 的语义：租赁数字只能经投影端口进入，不直接 import
	// 计量服务）。
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \",\"}}",
		"github.com/lease-management-system/core-service/internal/finmodel/...")
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
		// 每行 "<importpath> <deps 以逗号分隔>"；依赖项之间用逗号分隔，
		// import path 不含逗号，续行不会吞掉前缀。
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
				t.Fatalf("%s must not import the ifrs16 service (D-S3), found %s", pkg, dep)
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
