package charts

// Ch1 BG1 golden 与反向测试：
//   - golden 文件逐字节比对；改一个 Delta 的符号 golden 必须红；
//   - 同一输入两次渲染逐字节相同（D-B3）；
//   - Classification 缺失/未知 → 拒绝渲染（底线 2 不可表达）。

import (
	"os"
	"strings"
	"testing"
)

func f1Sample() Waterfall {
	return Waterfall{
		StartLabel: "基期利润", StartValue: 120000,
		Steps: []Step{
			{Label: "客流", Delta: 15000},
			{Label: "转化率", Delta: -8000},
			{Label: "客单价", Delta: 6000},
			{Label: "占用成本", Delta: -4500},
		},
		EndLabel: "当期利润", EndValue: 128500,
		Currency:       "CNY",
		Classification: "production",
		OrderNote:      "decomposition_order: footfall → conversion → atv → occupancy",
	}
}

const goldenFile = "testdata/waterfall_profit.golden.svg"

func TestRenderGoldenByteForByte(t *testing.T) {
	got, err := Render(f1Sample())
	if err != nil {
		t.Fatal(err)
	}
	want, err := osReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != want {
		t.Fatalf("SVG diverged from golden:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// 反向：翻转任一 Delta 的符号，golden 必须红。
func TestGoldenRedOnSignFlip(t *testing.T) {
	w := f1Sample()
	flipped := make([]Step, len(w.Steps))
	copy(flipped, w.Steps)
	for i := range flipped {
		if flipped[i].Delta == -8000 {
			flipped[i].Delta = 8000
		}
	}
	w.Steps = flipped
	got, err := Render(w)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := osReadFile(goldenFile)
	if got == want {
		t.Fatal("sign flip must change the rendered bytes (reverse test)")
	}
}

func TestRenderDeterministicAcrossRuns(t *testing.T) {
	a, err := Render(f1Sample())
	if err != nil {
		t.Fatal(err)
	}
	b, err2 := Render(f1Sample())
	if err2 != nil {
		t.Fatal(err2)
	}
	if a != b {
		t.Fatal("same input must render byte-identical SVG (D-B3)")
	}
	for _, banned := range []string{"generated_at", "timestamp", "<date>", "2026-08-2"} {
		if strings.Contains(a, banned) {
			t.Fatalf("determinism violation: output contains %q", banned)
		}
	}
}

// 底线 2：Classification 是数据字段。缺失或未知直接拒绝——调用方无法渲染
// 一张不带模拟标识的模拟图。
func TestRenderRefusesUnclassifiedWaterfall(t *testing.T) {
	w := f1Sample()
	w.Classification = ""
	if _, err := Render(w); err == nil {
		t.Fatal("empty classification must be refused")
	}
	w.Classification = "internal-test"
	if _, err := Render(w); err == nil {
		t.Fatal("unknown classification must be refused")
	}
	// simulated 必须带水印；production 不带。
	sim := f1Sample()
	sim.Classification = "simulated"
	simSVG, err := Render(sim)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(simSVG, "SIMULATED") {
		t.Fatal("simulated chart must carry the marker")
	}
	prod := f1Sample()
	prod.Classification = "production"
	prodSVG, err := Render(prod)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prodSVG, "SIMULATED") {
		t.Fatal("production chart must not carry the simulated marker")
	}
}

// 残差不画成一段（D-B4）：渲染输出里不存在 residual 段的痕迹由调用方保证，
// 这里锁的是「渲染器不产出 residual 段元素」这一半。
func TestRenderNeverEmitsResidualSegment(t *testing.T) {
	svg, err := Render(f1Sample())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(svg, "residual") || strings.Contains(svg, "残差") {
		t.Fatal("renderer must not invent a residual segment")
	}
}

func osReadFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
