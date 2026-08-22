package retailstore360

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// RH1（R1-1，坑 5）：retailstore360 包内不得再出现第二张指标中文名标签表。
// 唯一真相源在 retailkpi（Label / chineseNames）；本文件是 grep 式源码级断言：
// 有人把 labels map 加回来、或把某个指标中文名字面量写死在本包，即红。
func TestNoSecondLabelMapInPackage(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("*.go"))
	if err != nil {
		t.Fatal(err)
	}
	labelMapDecl := regexp.MustCompile(`labels\s*:?=\s*map\[string\]string`)
	// 这批字面量只允许住在 retailkpi 的唯一标签表里
	canonicalLabels := []string{"期间坪效", "经营占用现金成本", "门店经营利润率", "销售人效", "单均工时", "期末在岗人数"}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue // 测试文件里的断言字符串不算第二真相源
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if labelMapDecl.Match(src) {
			t.Fatalf("%s declares a second label map; metric display names live only in retailkpi.Label", p)
		}
		for _, literal := range canonicalLabels {
			if strings.Contains(string(src), "\""+literal+"\"") {
				t.Fatalf("%s hardcodes metric display name %q; it must come from retailkpi.Label", p, literal)
			}
		}
	}
}

// RH1：两个 Surface 的闭包在包初始化时已由 ValidateSurface 校验（init panic）。
// 这里再显式断言一次错误路径，保证「改坏清单会启动失败」这件事本身可测。
func TestSurfacesValidate(t *testing.T) {
	if err := retailkpi.ValidateSurface(summarySurface); err != nil {
		t.Fatalf("summary surface must validate: %v", err)
	}
	if err := retailkpi.ValidateSurface(benchmarkSurface); err != nil {
		t.Fatalf("benchmark surface must validate: %v", err)
	}
	broken := summarySurface
	broken.Codes = append(append([]string{}, summarySurface.Codes...), "not_a_metric")
	if err := retailkpi.ValidateSurface(broken); err == nil {
		t.Fatal("surface with undefined code must fail validation")
	}
}
