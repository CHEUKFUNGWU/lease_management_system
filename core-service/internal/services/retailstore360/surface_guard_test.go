package retailstore360

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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

// R1-1 引入的用户可见行为变化：sales_per_labor_hour 进同群对标后，peer
// 事实缺 labor_hours 时该行显式降级为 insufficient_peers / peer_count_below_minimum，
// 而不是从列表里消失或填 0（retail-kpi-v1：同群样本不足必须显式降级）。
// 两条断言分开写，别混成一条：
//  1. 降级本身——sph 行的状态与原因、数值字段为 nil；
//  2. 这不是授权失败——同一响应里 revenue 基准 PeerCount=4 且 complete，
//     说明 peer 还在、没被范围过滤掉；降级是数据形状驱动的。
//
// 自检句：把降级逻辑改掉（比如把无值 code 从清单里剔掉），第 1 条红；
// 把 peer 授权过滤改坏，第 2 条红。
func TestPeerBenchmarkLaborHoursMissingDegradesExplicitly(t *testing.T) {
	reader := &fakeReader{set: fixture()} // fixture 事实不带 LaborHours
	result, err := NewService(reader).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var sph, revenue *PeerBenchmark
	for i := range result.PeerBenchmark {
		switch result.PeerBenchmark[i].Code {
		case "sales_per_labor_hour":
			sph = &result.PeerBenchmark[i]
		case "revenue":
			revenue = &result.PeerBenchmark[i]
		}
	}
	if sph == nil {
		t.Fatal("surfaced metric must stay listed in peer benchmarks even when peers lack labor_hours")
	}
	// 断言 1：显式降级，不是空白、不是 0、不是从清单消失
	if sph.Status != "insufficient_peers" || sph.Reason != "peer_count_below_minimum" {
		t.Fatalf("sph benchmark must degrade explicitly, got status=%q reason=%q", sph.Status, sph.Reason)
	}
	if sph.Median != nil || sph.Target != nil || sph.TargetMinusMedian != nil {
		t.Fatalf("degraded sph benchmark must carry no numbers, got %+v", sph)
	}
	// 断言 2：同响应里授权的 peer 照常在别的基准上计数——降级不是权限问题
	if revenue == nil || revenue.PeerCount != 4 || revenue.Status != "complete" {
		t.Fatalf("revenue benchmark must remain complete with all 4 authorized peers, got %+v", revenue)
	}
}
