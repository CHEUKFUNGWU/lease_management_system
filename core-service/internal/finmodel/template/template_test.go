package template

import (
	"strings"
	"testing"
)

func TestParseAcceptsUsableTemplate(t *testing.T) {
	tmpl, err := Parse(TemplateDef{
		Name: "t", Version: 1,
		Rows: []RowDef{
			{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "fact.revenue"},
			{Key: "cost", Label: "成本", Kind: RowLink, Basis: BasisShared, Source: "fact.cost"},
			{Key: "margin", Label: "毛利", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"rev", "cost"}, Subtract: []string{"cost"}},
			{Key: "ratio", Label: "毛利率", Kind: RowFormula, Basis: BasisShared, Formula: "rows.margin / rows.rev"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Major != 1 || len(tmpl.Rows) != 4 {
		t.Fatalf("parsed template wrong: %+v", tmpl)
	}
	if got := tmpl.Rows[2].ChildSign("cost"); got != -1 {
		t.Fatalf("subtract child must sign -1, got %v", got)
	}
	refs := func(key string) *float64 {
		m := map[string]float64{"rev": 100, "margin": 40}
		v, ok := m[key]
		if !ok {
			return nil
		}
		return &v
	}
	lagged := func(key string, n int) *float64 { return nil }
	got := tmpl.Rows[3].Formula.Eval(refs, lagged)
	if got == nil || *got != 0.4 {
		if got == nil {
			t.Fatal("ratio eval = nil, want 0.4")
		}
		t.Fatalf("ratio eval = %v, want 0.4", *got)
	}
}

func TestParseRejectsNumericLiteral(t *testing.T) {
	_, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "bad", Label: "坏行", Kind: RowFormula, Basis: BasisShared, Formula: "rows.rev * 1.05"},
	}})
	if err == nil || !strings.Contains(err.Error(), "literal") {
		t.Fatalf("numeric literal must be rejected at parse, got %v", err)
	}
}

func TestParseRejectsSQLKeywordAndUnknownRef(t *testing.T) {
	_, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "bad", Label: "坏行", Kind: RowFormula, Basis: BasisShared, Formula: "select 1"},
	}})
	if err == nil || !strings.Contains(err.Error(), "SQL keyword") {
		t.Fatalf("SQL keyword must be rejected, got %v", err)
	}
	_, err = Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "bad", Label: "坏行", Kind: RowFormula, Basis: BasisShared, Formula: "rows.nonexistent"},
	}})
	if err == nil || !strings.Contains(err.Error(), "unknown row") {
		t.Fatalf("unknown row reference must be rejected, got %v", err)
	}
	_, err = Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "bad", Label: "坏行", Kind: RowFormula, Basis: BasisShared, Formula: "legal.other.rev"},
	}})
	if err == nil {
		t.Fatal("cross-legal-entity references must be rejected")
	}
}

func TestParseRejectsCycle(t *testing.T) {
	_, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "a", Label: "a", Kind: RowFormula, Basis: BasisShared, Formula: "rows.b"},
		{Key: "b", Label: "b", Kind: RowFormula, Basis: BasisShared, Formula: "rows.a"},
	}})
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("cycle must be rejected, got %v", err)
	}
	// lag breaks same-period cycles: b referencing lag(a) is legal.
	_, err = Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "a", Label: "a", Kind: RowFormula, Basis: BasisShared, Formula: "rows.b"},
		{Key: "b", Label: "b", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.a, 1)"},
	}})
	if err != nil {
		t.Fatalf("lag cycles must be legal, got %v", err)
	}
}

func TestParseRejectsBasisMixing(t *testing.T) {
	_, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "op", Label: "经营行", Kind: RowLink, Basis: BasisOperating, Source: "f"},
		{Key: "ifr", Label: "IFRS 行", Kind: RowLink, Basis: BasisIFRS16, Source: "f"},
		{Key: "bad", Label: "混行小计", Kind: RowSubtotal, Basis: BasisOperating, Children: []string{"op", "ifr"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "mixes basis") {
		t.Fatalf("basis mixing must be rejected, got %v", err)
	}
	// shared subtotal over non-shared children also rejected.
	_, err = Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "op", Label: "经营行", Kind: RowLink, Basis: BasisOperating, Source: "f"},
		{Key: "bad", Label: "共享小计", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"op"}},
	}})
	if err == nil {
		t.Fatal("shared subtotal over operating rows must be rejected")
	}
}

func TestParseRejectsStructuralErrors(t *testing.T) {
	cases := []TemplateDef{
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowLink, Basis: BasisShared, Children: []string{"y"}}}},                              // children on link
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowSubtotal, Basis: BasisShared}}},                                                   // subtotal without children
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowLink, Basis: BasisShared}}},                                                       // link without source
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowFormula, Basis: BasisShared}}},                                                    // formula without text
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowSubtotal, Basis: BasisShared, Children: []string{"x"}, Subtract: []string{"z"}}}}, // subtract not in children
		{Name: "t", Version: 0, Rows: []RowDef{{Key: "x", Label: "x", Kind: RowLink, Basis: BasisShared, Source: "f"}}},                                          // bad version
		{Name: "t", Version: 1, Rows: []RowDef{{Key: "x", Label: "x", Kind: "sql", Basis: BasisShared, Source: "f"}}},                                            // bad kind
	}
	for i, def := range cases {
		if _, err := Parse(def); err == nil {
			t.Fatalf("case %d must be rejected", i)
		}
	}
}

func TestEvalDivisionAndMissingSemantics(t *testing.T) {
	tmpl, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "a", Label: "a", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "b", Label: "b", Kind: RowLink, Basis: BasisShared, Source: "f"},
		{Key: "ratio", Label: "ratio", Kind: RowFormula, Basis: BasisShared, Formula: "rows.a / rows.b"},
		{Key: "guarded", Label: "guarded", Kind: RowFormula, Basis: BasisShared, Formula: "if(rows.a > 0, rows.a, 0)"},
		{Key: "nilsum", Label: "nilsum", Kind: RowFormula, Basis: BasisShared, Formula: "rows.a + rows.b"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ratio := tmpl.Rows[2].Formula
	guarded := tmpl.Rows[3].Formula
	nilsum := tmpl.Rows[4].Formula

	// b = 0 → nil, not NaN / Inf.
	got := ratio.Eval(func(k string) *float64 { v := map[string]float64{"a": 10, "b": 0}[k]; return &v }, nil)
	if got != nil {
		t.Fatalf("division by zero must yield missing, got %v", *got)
	}
	// missing operand propagates nil.
	got = nilsum.Eval(func(k string) *float64 {
		if k == "a" {
			return nil
		}
		v := 5.0
		return &v
	}, nil)
	if got != nil {
		t.Fatalf("nil operand must propagate nil, got %v", *got)
	}
	// if guards nil-free branches.
	got = guarded.Eval(func(k string) *float64 { v := 10.0; return &v }, nil)
	if got == nil || *got != 10 {
		t.Fatalf("guarded branch = %v, want 10", got)
	}
}

func TestEvalLag(t *testing.T) {
	tmpl, err := Parse(TemplateDef{Name: "t", Version: 1, Rows: []RowDef{
		{Key: "cash", Label: "现金", Kind: RowFormula, Basis: BasisShared, Formula: "lag(rows.cash, 1) + rows.net"},
		{Key: "net", Label: "净变动", Kind: RowLink, Basis: BasisShared, Source: "f"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// First period: lag missing → nil (缺失而非 0).
	got := tmpl.Rows[0].Formula.Eval(func(k string) *float64 { v := 5.0; return &v }, func(k string, n int) *float64 { return nil })
	if got != nil {
		t.Fatalf("first-period lag must yield missing, got %v", *got)
	}
	got = tmpl.Rows[0].Formula.Eval(func(k string) *float64 { v := 5.0; return &v }, func(k string, n int) *float64 { v := 100.0; return &v })
	if got == nil || *got != 105 {
		t.Fatalf("lagged eval = %v, want 105", got)
	}
}

func TestDefaultTemplatesParse(t *testing.T) {
	store, err := DefaultStorePnlTemplate()
	if err != nil {
		t.Fatalf("store P&L default must parse: %v", err)
	}
	if len(store.Rows) < 18 {
		t.Fatalf("store P&L default too small: %d rows", len(store.Rows))
	}
	stmt, err := DefaultStatementTemplate()
	if err != nil {
		t.Fatalf("statement default must parse: %v", err)
	}
	if len(stmt.Rows) < 40 {
		t.Fatalf("statement default too small: %d rows", len(stmt.Rows))
	}
	// 口径隔离（T15 的结构承载）：两个口径的 subtotal 各自只含本口径/共享子行。
	for _, row := range stmt.Rows {
		if row.Kind != RowSubtotal {
			continue
		}
		for _, child := range row.Children {
			childBasis := basisOf(stmt, child)
			if row.Basis != BasisShared && childBasis != BasisShared && row.Basis != childBasis {
				t.Fatalf("template violates basis isolation: subtotal %q (%s) child %q (%s)", row.Key, row.Basis, child, childBasis)
			}
		}
	}
}

func basisOf(tmpl *Template, key string) Basis {
	for _, r := range tmpl.Rows {
		if r.Key == key {
			return r.Basis
		}
	}
	return Basis("")
}
