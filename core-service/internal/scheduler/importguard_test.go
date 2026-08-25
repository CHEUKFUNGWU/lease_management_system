package scheduler

// Architecture guard for the scheduler package (RT1-L3-C, condition ①):
// Type A jobs must be STRUCTURALLY incapable of reaching the tool runtime —
// a ruling recorded in code, not in meeting notes. If a future job "grows"
// tool-calling branches while living outside the governance chain, this guard
// is what makes that visible instead of gradual.
//
// The package imports only the standard library. Any first-party import here
// is a violation: narrow ports (like LeaseRecoveryQueue) are declared locally
// and satisfied implicitly by the owning packages.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var bannedImports = []string{
	"internal/agenttools",
	"internal/agentkernel",
	"internal/agentcore",
	"internal/aiagent",
	"internal/handlers",
	"internal/repository",
	"internal/services",
	"internal/aichat",
	"internal/middleware",
	"internal/access",
	"internal/sessionmanager",
	"internal/llm",
	"internal/miniostore",
	"internal/db",
}

func schedulerRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSchedulerDoesNotImportBusinessOrToolPackages scans every non-test Go
// file of this package.
func TestSchedulerDoesNotImportBusinessOrToolPackages(t *testing.T) {
	violations := scanSchedulerImports(schedulerRoot(t))
	if len(violations) > 0 {
		t.Fatalf("scheduler must stay stdlib-only: Type A jobs reach maintenance operations through narrow ports declared here, never through business or tool packages:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// Reverse fixture: plant a tool-runtime import beside real sources, require
// red, then remove it. A guard that has never detected anything guards
// nothing.
func TestSchedulerImportGuardDetectsViolations(t *testing.T) {
	path := filepath.Join(schedulerRoot(t), "zz_fixture_tool_import.go")
	fixture := "package scheduler\n\nimport _ \"github.com/lease-management-system/core-service/internal/agenttools\"\n"
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	violations := scanSchedulerImports(schedulerRoot(t))
	if len(violations) == 0 {
		t.Fatal("scheduler import guard failed to detect a planted tool-runtime import — it cannot be trusted")
	}
}

func scanSchedulerImports(dir string) []string {
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
		for _, banned := range bannedImports {
			pattern := `"github.com/lease-management-system/core-service/` + banned + `"`
			for index, line := range strings.Split(string(content), "\n") {
				if strings.Contains(line, pattern) {
					violations = append(violations,
						path+":"+strconv.Itoa(index+1)+" imports "+banned)
				}
			}
		}
		return nil
	})
	return violations
}
