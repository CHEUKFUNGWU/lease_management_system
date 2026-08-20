package finmodel

// P1-2 (D-S2): the single-write-entry architecture test. Writes to
// fin_model_runs are allowed in exactly two places: the repository seam
// (internal/repository, the SQL layer) and the sanctioned entry
// (internal/finmodel/persist, RunWriter). Any other package calling the
// repository's run-write methods is a second write path and must fail here —
// the same shape as the N10 retailingest constraint test.
//
// This test greps the source tree, not just this package's dependencies:
// handlers wiring a direct CreateModelRun / status flip is a structural
// violation even if it compiles.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runWriteCalls are the repository methods that physically write the model
// run tables. Only persist (and the repository itself) may call them.
var runWriteCalls = []string{
	".CreateModelRun(",
	".UpdateModelRunStatus(",
	".FailModelRun(",
	".CancelModelRun(",
	".InsertRunLines(",
	".InsertTieOuts(",
}

func TestFinModelRunSingleWriteEntry(t *testing.T) {
	root := moduleRoot(t, "../..") // internal/finmodel -> core-service
	allowedDirs := []string{"internal/repository", "internal/finmodel/persist"}
	var violations []string
	err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// 测试文件可以合法调用仓库方法（不存在路线上锁定范围）；守卫扫的是
		// 生产代码。
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if inAny(rel, allowedDirs) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, call := range runWriteCalls {
			if strings.Contains(string(data), call) {
				violations = append(violations, rel+" calls "+strings.TrimPrefix(call, "."))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("fin_model_runs writes must go through persist.RunWriter (D-S2); found direct writes outside %v:\n  %s",
			allowedDirs, strings.Join(violations, "\n  "))
	}
}

func inAny(rel string, dirs []string) bool {
	for _, dir := range dirs {
		if rel == dir || strings.HasPrefix(rel, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
