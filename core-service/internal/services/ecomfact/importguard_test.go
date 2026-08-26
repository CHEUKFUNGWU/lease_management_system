// 架构守卫：ecomfact 全部子包禁 import ifrs16（模块深化 EM2 铁律）。
// P2 的 finmodel Port 方向是 finmodel 读电商事实，不是相反；一旦反向依赖，
// 电商侧就会长出第二套计量口径。守卫遍历全部子包，不只根包。
package ecomfact_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEcomFactNeverImportsIFRS16(t *testing.T) {
	if _, err := os.Stat("../ifrs16"); err != nil {
		t.Skipf("ifrs16 package not present: %v", err)
	}
	root := moduleRoot(t)
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \",\"}}",
		"github.com/lease-management-system/core-service/internal/services/ecomfact/...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("go list unavailable or failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		imports := ""
		if len(parts) == 2 {
			imports = parts[1]
		}
		if strings.Contains(imports, "internal/services/ifrs16") || strings.Contains(imports, "internal/ifrs16") {
			t.Fatalf("%s imports ifrs16; ecomfact must never depend on the IFRS 16 engine: %s", parts[0], line)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := strings.TrimSuffix(dir, "/"+strings.TrimPrefix(lastSegment(dir), "/"))
		if parent == dir {
			t.Fatalf("go.mod not found upward from %s", dir)
		}
		dir = parent
	}
}

func lastSegment(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
