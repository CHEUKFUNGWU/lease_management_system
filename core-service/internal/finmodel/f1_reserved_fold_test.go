package finmodel

// F1 反向测试与数据层断言：
//   - 反向 #1：删掉保留键闸门，本文件的 Run 拒绝测试必须变红；
//   - 反向 #2（后端半）：不声明存量/流量的自定义行，年度折叠按求和得出
//     「看着合理的错数」——这条断言把危害钉在测试里，编辑器的必填校验
//     防的就是它；
//   - D-F2：用户主张的零（0 + human_input provenance）与缺失（nil + Gap）
//     在数据层是不同的值。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

func containsAll(values []string, wanted []string) bool {
	for _, w := range wanted {
		found := false
		for _, v := range values {
			if v == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// f1RowDefs converts a parsed template back to its declaration form so a
// test can drop rows and re-parse.
func f1RowDefs(t *testing.T, tmpl *template.Template, without ...string) []template.RowDef {
	t.Helper()
	rows := make([]template.RowDef, 0, len(tmpl.Rows))
	for _, row := range tmpl.Rows {
		skip := false
		for _, drop := range without {
			if row.Key == drop {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		rows = append(rows, template.RowDef{
			Key: row.Key, Label: row.Label, Kind: row.Kind, Basis: row.Basis,
			Source: row.Source, Formula: row.FormulaText, Children: row.Children,
			Subtract: row.Subtract, Format: row.Format, ActualSource: row.ActualSource,
			Fold: row.Fold,
		})
	}
	return rows
}

// 反向 #1：删掉任一保留键后 Run 必须以结构原因拒绝——不是权限措辞。
// （删除 cash 需同时删引用它的 total_assets 小计，缺失清单因此含两键。）
func TestRunRejectsMissingReservedKeyWithMechanicalReason(t *testing.T) {
	base := goldenTemplate(t)
	def := goldenDef(t)
	def.Template = nil
	parsed, err := template.Parse(template.TemplateDef{
		Name: "f1-no-cash", Version: 1,
		Rows: f1RowDefs(t, base, "total_assets", "cash"),
	})
	if err != nil {
		t.Fatalf("template without cash must still parse structurally: %v", err)
	}
	def.Template = parsed
	_, err = Run(context.Background(), def, goldenInputs())
	if err == nil {
		t.Fatal("run without the cash reserved key must be rejected (reverse #1: removing the gate turns this red)")
	}
	var missing *template.ErrMissingReservedKeys
	if !errors.As(err, &missing) || len(missing.Missing) != 2 || !containsAll(missing.Missing, []string{"cash", "total_assets"}) {
		t.Fatalf("rejection must carry structured missing keys, got %v", err)
	}
	if !strings.Contains(err.Error(), "T1–T16") {
		t.Fatalf("rejection must state the mechanical reason (T1–T16), got %q", err.Error())
	}
	if strings.Contains(err.Error(), "无权限") || strings.Contains(strings.ToLower(err.Error()), "permission") {
		t.Fatalf("rejection must never frame the constraint as a permission problem: %q", err.Error())
	}
}

// D-F2：用户主张的零与缺失在数据层是不同的值。
func TestHumanZeroDistinctFromMissing(t *testing.T) {
	inputRow := template.RowDef{Key: "custom_marketing", Label: "自定义营销费", Kind: template.RowInput, Basis: template.BasisShared, Fold: template.FoldFlow}
	base := goldenTemplate(t)
	rows := f1RowDefs(t, base)
	rows = append(rows, inputRow)
	parsed, err := template.Parse(template.TemplateDef{Name: "f1-human-zero", Version: 1, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	def := goldenDef(t)
	def.Template = parsed

	lineFor := func(in ModelInputs) (*float64, string, bool) {
		t.Helper()
		result, err := Run(context.Background(), def, in)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range result.Lines {
			if line.RowKey == "custom_marketing" && line.Period == "2026-01" {
				return line.Value, line.Provenance.SourceType, true
			}
		}
		return nil, "", false
	}

	// 缺失侧：nil 值 + assumption_missing Gap，provenance 不是 human_input。
	inMissing := goldenInputs()
	value, sourceType, found := lineFor(inMissing)
	if !found || value != nil {
		t.Fatalf("missing input must be nil, got %v", value)
	}
	if sourceType == "human_input" {
		t.Fatal("missing input must not claim human_input provenance")
	}
	gapFound := false
	for _, gap := range missingGaps(t, def, inMissing) {
		if gap.Kind == "assumption_missing" && strings.Contains(gap.Detail, "custom_marketing") {
			gapFound = true
		}
	}
	if !gapFound {
		t.Fatal("missing input must leave a named gap")
	}

	// 主张零侧：显式 0 + human_input provenance（含确认人），无缺失 Gap。
	inZero := goldenInputs()
	inZero.HumanZeros = map[string]HumanZero{"custom_marketing": {ConfirmedBy: "bp-f1"}}
	value, sourceType, found = lineFor(inZero)
	if !found || value == nil || *value != 0 {
		t.Fatalf("asserted zero must render as exact 0, got %v", value)
	}
	if sourceType != "human_input" {
		t.Fatalf("asserted zero provenance must be human_input, got %q", sourceType)
	}
	for _, gap := range missingGaps(t, def, inZero) {
		if gap.Kind == "assumption_missing" && strings.Contains(gap.Detail, "custom_marketing") {
			t.Fatal("asserted zero must not produce a missing gap — 已知为零不能说成未知")
		}
	}
}

func missingGaps(t *testing.T, def ModelDef, in ModelInputs) []DataGap {
	t.Helper()
	result, err := Run(context.Background(), def, in)
	if err != nil {
		t.Fatal(err)
	}
	return result.Gaps
}

// 反向 #2（后端半）：同一自定义行，声明存量按期末取值；不声明按默认流量
// 求和——后者就是「12 个月加出看似合理的错数」的危害本体。
func TestFoldDeclarationChangesYearFold(t *testing.T) {
	periods := make([]string, 0, 12)
	months := map[string]*float64{}
	for m := 1; m <= 12; m++ {
		label := "2026-01"
		if m >= 10 {
			label = "2026-" + string(rune('0'+m/10)) + string(rune('0'+m%10))
		} else {
			label = "2026-0" + string(rune('0'+m))
		}
		periods = append(periods, label)
		value := 10.0
		months[label] = &value
	}
	buckets := FoldBuckets(periods, FoldYear)
	if len(buckets) != 1 || buckets[0].Label == "" {
		t.Fatalf("year bucket wrong: %+v", buckets)
	}
	values := map[string]map[string]*float64{"prepaid_expense": months, "cash": months}

	flowFolded := FoldMonthValuesWithStocks(values, buckets, func(string) bool { return false })
	if got := flowFolded["prepaid_expense"][buckets[0].Label]; got == nil || *got != 120 {
		t.Fatalf("flow default must sum to 120 (the hazard): %v", got)
	}
	stockFolded := FoldMonthValuesWithStocks(values, buckets, func(string) bool { return true })
	if got := stockFolded["prepaid_expense"][buckets[0].Label]; got == nil || *got != 10 {
		t.Fatalf("declared stock must take period-end 10: %v", got)
	}
	// 保留键默认仍按存量：cash 不需要显式声明。
	reservedDefault := FoldMonthValues(values, buckets)
	if got := reservedDefault["cash"][buckets[0].Label]; got == nil || *got != 10 {
		t.Fatalf("reserved key default stock broken: %v", got)
	}
}

func TestParseRejectsInvalidFoldValue(t *testing.T) {
	_, err := template.Parse(template.TemplateDef{Name: "bad-fold", Version: 1, Rows: []template.RowDef{
		{Key: "x", Label: "X", Kind: template.RowInput, Basis: template.BasisShared, Fold: "banana"},
	}})
	if err == nil || !strings.Contains(err.Error(), "fold") {
		t.Fatalf("invalid fold must be rejected at parse, got %v", err)
	}
}
