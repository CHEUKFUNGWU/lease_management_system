// Package charts is BG1: the shared waterfall renderer. One value type and
// one function — no Options, no palette, no dimensions parameters (D-B17:
// adding them turns "a business concept" into "a drawing library"; size is
// the SVG viewBox's job, handed to frontend CSS).
//
// 确定性（D-B3）：同一输入逐字节相同。三条纪律：
//   - 小数位显式格式化（禁 %v / %f 默认精度）；
//   - 元素 id 由输入派生的稳定哈希产生（禁自增计数器——并发或顺序微调会漂）；
//   - 不嵌入生成时间戳。
//
// 渲染层不做任何业务算术（D-B4）：各段值全部来自调用方，这里只有坐标、
// 路径与转义。残差不画——精确连环替代下它是浮点噪声，画出来等于暗示它
// 有意义。
package charts

import (
	"fmt"
	"strconv"
	"strings"
)

// Step is one ordered contribution between start and end. 顺序即语义。
type Step struct {
	Label string
	Delta float64
}

// Waterfall is the only shared vocabulary between the business layers and
// this renderer. Business packages translate their results into it; the
// renderer knows neither varianceattribution nor finmodel.
type Waterfall struct {
	StartLabel string
	StartValue float64
	Steps      []Step
	EndLabel   string
	EndValue   float64
	Currency   string
	// Classification is a property of the DATA, not a rendering option
	// (production | simulated | mixed). Callers cannot render a simulated
	// chart without its marker — 底线 2 made unrepresentable.
	Classification string
	// OrderNote echoes the decomposition order verbatim (Story 4).
	OrderNote string
}

const (
	svgWidth  = 960
	svgHeight = 420
	padLeft   = 64
	padRight  = 24
	padTop    = 56
	padBottom = 96
	barGap    = 18
)

// Render produces the deterministic SVG document. Same input → same bytes.
func Render(w Waterfall) (string, error) {
	if err := validate(w); err != nil {
		return "", err
	}
	values := seriesValues(w)
	minV, maxV := bounds(values)
	lo, hi := niceBounds(minV, maxV)
	count := len(w.Steps) + 2 // start + steps + end
	plotH := float64(svgHeight - padTop - padBottom)
	y := func(v float64) float64 {
		if hi == lo {
			return padTop + plotH/2
		}
		return padTop + plotH*(hi-v)/(hi-lo)
	}

	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` +
		itoa(svgWidth) + " " + itoa(svgHeight) + `" role="img">`)
	b.WriteString(`<title>` + escapeText(w.StartLabel+" → "+w.EndLabel) + `</title>`)

	if w.Classification == "simulated" || w.Classification == "mixed" {
		// 底线 2：模拟标识是数据属性，调用方无法省略。
		b.WriteString(`<text class="wf-watermark" x="` + f2(float64(svgWidth)-16) +
			`" y="28" text-anchor="end">SIMULATED · 模拟数据（` + escapeText(w.Classification) + `）</text>`)
	}
	if w.OrderNote != "" {
		b.WriteString(`<text class="wf-order" x="16" y="28">替代顺序: ` +
			escapeText(w.OrderNote) + `</text>`)
	}
	b.WriteString(`<line class="wf-baseline" x1="` + f2(padLeft) + `" y1="` + f2(y(0)) +
		`" x2="` + f2(float64(svgWidth)-padRight) + `" y2="` + f2(y(0)) + `"/>`)

	writeTotalBar(&b, w.StartLabel+"#start", w.StartLabel, w.StartValue, 0, count, y)
	cursor := w.StartValue
	for i, step := range w.Steps {
		next := cursor + step.Delta
		x, width := slot(i+1, count)
		id := elementID(step.Label, i)
		class := "wf-bar-up"
		if next < cursor {
			class = "wf-bar-down"
		}
		top, bottom := cursor, next
		if next < cursor {
			top, bottom = next, cursor
		}
		b.WriteString(`<rect class="` + class + `" id="` + id + `" x="` + f2(x) + `" y="` + f2(y(bottom)) +
			`" width="` + f2(width) + `" height="` + f2(y(top)-y(bottom)) + `"/>`)
		valueY := y(top) - 8
		if next < cursor {
			valueY = y(bottom) + 20
		}
		writeValueLabel(&b, id+"-v", formatSigned(next-cursor), x+width/2, valueY)
		writeLabel(&b, id+"-l", step.Label, x+width/2)
		// 连接线：上一段终点 → 本段起点（纯坐标路径）。
		prevXEnd := x - barGap
		b.WriteString(`<line class="wf-connector" x1="` + f2(prevXEnd) + `" y1="` + f2(y(cursor)) +
			`" x2="` + f2(x) + `" y2="` + f2(y(cursor)) + `"/>`)
		cursor = next
	}
	endIndex := len(w.Steps) + 1
	writeTotalBar(&b, w.EndLabel+"#end", w.EndLabel, w.EndValue, endIndex, count, y)

	b.WriteString(`<text class="wf-currency" x="16" y="` + f2(float64(svgHeight)-12) +
		`">币种: ` + escapeText(w.Currency) + `</text>`)
	b.WriteString(`</svg>`)
	return b.String(), nil
}

func slot(index, count int) (x, width float64) {
	plotW := float64(svgWidth-padLeft-padRight) / float64(count)
	return padLeft + float64(index)*plotW + barGap/2, plotW - barGap
}

func writeTotalBar(b *strings.Builder, idSeed, label string, value float64, index, count int, y func(float64) float64) {
	x, width := slot(index, count)
	id := elementID(idSeed, index)
	top, bottom := value, 0.0
	if value < 0 {
		top, bottom = 0, value
	}
	b.WriteString(`<rect class="wf-bar-total" id="` + id + `" x="` + f2(x) + `" y="` + f2(y(bottom)) +
		`" width="` + f2(width) + `" height="` + f2(y(top)-y(bottom)) + `"/>`)
	writeValueLabel(b, id+"-v", formatAmount(value), x+width/2, y(top)-8)
	writeLabel(b, id+"-l", label, x+width/2)
}

func writeValueLabel(b *strings.Builder, id, text string, cx, cy float64) {
	b.WriteString(`<text class="wf-value" id="` + id + `" x="` + f2(cx) +
		`" y="` + f2(cy) + `" text-anchor="middle">` + escapeText(text) + `</text>`)
}

func writeLabel(b *strings.Builder, id, label string, cx float64) {
	b.WriteString(`<text class="wf-label" id="` + id + `" x="` + f2(cx) + `" y="` +
		f2(float64(svgHeight)-padBottom+22) + `" text-anchor="middle">` + escapeText(label) + `</text>`)
}

func seriesValues(w Waterfall) []float64 {
	out := make([]float64, 0, len(w.Steps)*2+3)
	out = append(out, w.StartValue, w.EndValue)
	cursor := w.StartValue
	for _, step := range w.Steps {
		cursor += step.Delta
		out = append(out, cursor, step.Delta)
	}
	return out
}

func bounds(values []float64) (float64, float64) {
	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

// niceBounds pads the domain outward on magnitude/4 steps so bars never touch
// the frame. Deterministic tick behaviour without floating-point surprises.
func niceBounds(lo, hi float64) (float64, float64) {
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		span := magnitude(absF(hi)) / 4
		if span == 0 {
			span = 1
		}
		return lo - span, hi + span
	}
	step := magnitude(hi-lo) / 4
	return floorTo(lo, step), ceilTo(hi, step)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func magnitude(v float64) float64 {
	v = absF(v)
	m := 1.0
	for v >= 10 {
		v /= 10
		m *= 10
	}
	for v > 0 && v < 1 {
		v *= 10
		m /= 10
	}
	return m
}

func floorTo(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	floored := float64(int64(v/step)) * step
	return floored
}

func ceilTo(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	floored := float64(int64(v/step)) * step
	if floored >= v {
		return floored
	}
	return floored + step
}

// elementID derives a stable id purely from the input: label plus the exact
// delta it represents (disambiguates duplicate labels; stable under reordering).
func elementID(label string, index int) string {
	return "wf-" + strconv.FormatUint(fnv64a(label+"\x00"+strconv.Itoa(index)), 36)
}

func fnv64a(s string) uint64 {
	const offset64 = uint64(14695981039346656037)
	const prime64 = uint64(1099511628211)
	hash := offset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}

// formatAmount renders two fixed decimals — 禁 %v / %f 默认精度（D-B3）。
func formatAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatSigned(delta float64) string {
	if delta >= 0 {
		return "+" + strconv.FormatFloat(delta, 'f', 2, 64)
	}
	return strconv.FormatFloat(delta, 'f', 2, 64)
}

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func itoa(v int) string { return strconv.Itoa(v) }

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
)

func escapeText(s string) string { return xmlEscaper.Replace(s) }

// validate rejects structurally impossible or unclassified input. An unknown
// classification is refused outright — callers cannot render a chart that
// dodges the 模拟标识 contract (底线 2).
func validate(w Waterfall) error {
	if strings.TrimSpace(w.StartLabel) == "" || strings.TrimSpace(w.EndLabel) == "" {
		return fmt.Errorf("charts: start and end labels are required")
	}
	switch w.Classification {
	case "production", "simulated", "mixed":
	default:
		return fmt.Errorf("charts: classification must be production, simulated or mixed")
	}
	if strings.TrimSpace(w.Currency) == "" {
		return fmt.Errorf("charts: currency is required")
	}
	return nil
}
