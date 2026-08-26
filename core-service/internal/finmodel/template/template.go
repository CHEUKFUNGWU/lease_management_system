// Package template is SM1: the Statement Template value object. A template is
// parsed once, validated to death at parse time, and is thereafter an
// immutable, versioned value — the design decision D-S1 keeps calculation
// structure in exactly one place shared by the store P&L projection (SM3),
// the model engine (SM2) and exports.
//
// Validation happens in Parse, not at run time: an illegal template simply
// never becomes a Template. The engine and the projection receive only
// compile-valid structures.
package template

import (
	"fmt"
	"sort"
	"strings"
)

// Format is the per-row display contract (S3-7). It travels inside the
// frozen template version and is applied at render/export — storage never
// scales (金额单位缩放是显示层).
type Format struct {
	Scale    string `json:"scale,omitempty"`     // yuan | thousand | ten_thousand | million
	NegStyle string `json:"neg_style,omitempty"` // minus | parens | red
	Bold     bool   `json:"bold,omitempty"`
	Indent   int    `json:"indent,omitempty"` // 0..8
}

const (
	ScaleYuan        = "yuan"
	ScaleThousand    = "thousand"
	ScaleTenThousand = "ten_thousand"
	ScaleMillion     = "million"

	NegMinus  = "minus"
	NegParens = "parens"
	NegRed    = "red"
)

func validFormat(f Format) bool {
	switch f.Scale {
	case "", ScaleYuan, ScaleThousand, ScaleTenThousand, ScaleMillion:
	default:
		return false
	}
	if f.Scale == "" {
		f.Scale = ScaleYuan
	}
	switch f.NegStyle {
	case "", NegMinus, NegParens, NegRed:
	default:
		return false
	}
	return f.Indent >= 0 && f.Indent <= 8
}

// RowKind classifies a template row.
type RowKind string

const (
	RowInput    RowKind = "input"
	RowLink     RowKind = "link"
	RowFormula  RowKind = "formula"
	RowSubtotal RowKind = "subtotal"
	RowCheck    RowKind = "check"
)

// Basis labels the口径 a row belongs to. Shared runs in both blocks; the
// subtotal mixing rules keep operating and ifrs16 rows from summing.
type Basis string

const (
	BasisOperating Basis = "operating_basis"
	BasisIFRS16    Basis = "ifrs16_basis"
	BasisShared    Basis = "shared"
)

// Version identifies one immutable template version.
type Version struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// RowDef is the declared form of one row before validation.
type RowDef struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Kind     RowKind  `json:"kind"`
	Basis    Basis    `json:"basis"`
	Source   string   `json:"source,omitempty"`   // link rows: data source binding key
	Formula  string   `json:"formula,omitempty"`  // formula/check rows: DSL text
	Children []string `json:"children,omitempty"` // subtotal rows: child row keys
	// Subtract lists the children that are SUBTRACTED (T12 with C4's
	// positive-storage convention: 门店贡献 = 毛利 − 费用, stored values all
	// positive, direction decided here in the template).
	Subtract []string `json:"subtract,omitempty"`
	// Format is the S3-7 display contract (scale/negative/bold/indent).
	Format Format `json:"format,omitempty"`
	// ActualSource binds a formula row to a fact for the Actual window (PRD
	// C7): on the actual-cutoff left of the freeze line the row reads its fact
	// aggregate (retail-kpi-v1) instead of applying the driver formula — the
	// Actual 区永不来自假设. Empty on link rows (their Source already is the
	// fact binding).
	ActualSource string `json:"actual_source,omitempty"`
	// Fold declares 存量/流量 for custom rows (F1 D-F4): "stock" folds to the
	// period-end value, "flow" sums across the bucket. Empty is allowed at
	// parse time and means the fold default applies (reserved keys stock,
	// everything else flow) — but the EDITOR must not submit a balance-sheet
	// custom row without an explicit choice, because the flow default turns
	// twelve months into a plausible-looking wrong number.
	Fold string `json:"fold,omitempty"`
}

// TemplateDef is the declared form consumed by Parse.
type TemplateDef struct {
	Name    string   `json:"name"`
	Version int      `json:"version"`
	Rows    []RowDef `json:"rows"`
	// Source marks who authored the declaration (F1 D-F9): "ai_suggestion"
	// for AI-generated drafts, empty for human-authored. It rides in the
	// stored JSONB — no table change. AI has no approved path; the field
	// survives review so an audited template shows its origin.
	Source string `json:"source,omitempty"`
}

// Row is the validated row as seen by the engine.
type Row struct {
	Key      string
	Label    string
	Kind     RowKind
	Basis    Basis
	Source   string
	Children []string
	Subtract []string // ⊆ Children; subtotal = Σ Children − Σ Subtract
	Formula  *Expr    // nil unless kind is formula or check
	// FormulaText keeps the declared DSL text (Formula is the compiled AST):
	// the page's editor round-trips rows through the def, so the source text
	// must survive Parse (S1-9).
	FormulaText string
	Format      Format // S3-7 display contract, defaults to yuan/minus
	// ActualSource is the fact binding a formula row reads inside the Actual
	// window (PRD C7); empty means the formula applies in every period.
	ActualSource string
	// Fold is the declared 存量/流量 semantics (F1 D-F4). Empty inherits the
	// fold default (reserved keys stock, others flow).
	Fold string
}

// ChildSign reports whether a child is added (+1) or subtracted (−1) in a
// subtotal.
func (r Row) ChildSign(childKey string) float64 {
	for _, k := range r.Subtract {
		if strings.EqualFold(strings.TrimSpace(k), strings.TrimSpace(childKey)) {
			return -1
		}
	}
	return 1
}

// Template is the immutable parsed template.
type Template struct {
	Name  string
	Major int
	Rows  []Row
}

// Deps returns the same-period row references of the expression, in first-
// occurrence order. The engine uses it for topological evaluation order.
func (e *Expr) Deps() []string {
	seen := map[string]bool{}
	var out []string
	var walk func(n node)
	walk = func(n node) {
		switch v := n.(type) {
		case refNode:
			if v.lag == 0 && !seen[v.key] {
				seen[v.key] = true
				out = append(out, v.key)
			}
		case unaryNode:
			walk(v.operand)
		case binaryNode:
			walk(v.left)
			walk(v.right)
		case callNode:
			for _, arg := range v.args {
				walk(arg)
			}
		}
	}
	walk(e.node)
	return out
}

// Expr is the compiled formula AST. The concrete node kinds are private;
// the engine evaluates through Eval.
type Expr struct {
	node node
}

// Eval computes the formula for one period. refs resolves row references
// (rows.<key>) to the CURRENT period value; lagged resolves lag(rows.<key>,n)
// to the n-periods-earlier value. Missing values are nil — never zero
// (D-S4): any arithmetic consuming nil yields nil unless guarded by if().
func (e *Expr) Eval(refs func(key string) *float64, lagged func(key string, n int) *float64) *float64 {
	return evalNode(e.node, refs, lagged)
}

// node is the AST node union.
type node interface{ isNode() }

type literalNode struct{ value float64 }
type refNode struct {
	key string
	lag int
}
type unaryNode struct {
	op      string
	operand node
}
type binaryNode struct {
	op          string
	left, right node
}
type callNode struct {
	fn   string
	args []node
}

func (literalNode) isNode() {}
func (refNode) isNode()     {}
func (unaryNode) isNode()   {}
func (binaryNode) isNode()  {}
func (callNode) isNode()    {}

// Parse validates the declaration and compiles formulas. Every rejection
// carries the row key it occurred on.
func Parse(def TemplateDef) (*Template, error) {
	if strings.TrimSpace(def.Name) == "" {
		return nil, fmt.Errorf("template: name is required")
	}
	if def.Version <= 0 {
		return nil, fmt.Errorf("template: version must be positive")
	}
	if len(def.Rows) == 0 {
		return nil, fmt.Errorf("template: at least one row is required")
	}

	byKey := make(map[string]int, len(def.Rows))
	rows := make([]Row, len(def.Rows))
	for i, rd := range def.Rows {
		if strings.TrimSpace(rd.Key) == "" {
			return nil, fmt.Errorf("template: row %d has empty key", i)
		}
		key := strings.TrimSpace(rd.Key)
		if _, dup := byKey[key]; dup {
			return nil, fmt.Errorf("template: duplicate row key %q", key)
		}
		byKey[key] = i
		if rd.Basis != BasisOperating && rd.Basis != BasisIFRS16 && rd.Basis != BasisShared {
			return nil, fmt.Errorf("template: row %q has invalid basis %q", key, rd.Basis)
		}
		if !validFormat(rd.Format) {
			return nil, fmt.Errorf("template: row %q has invalid format %+v (scale in yuan|thousand|ten_thousand|million, neg_style in minus|parens|red, indent 0-8)", key, rd.Format)
		}
		if rd.Format.Scale == "" {
			rd.Format.Scale = ScaleYuan
		}
		if rd.Format.NegStyle == "" {
			rd.Format.NegStyle = NegMinus
		}
		rows[i] = Row{
			Key: key, Label: rd.Label, Kind: rd.Kind, Basis: rd.Basis,
			Source: strings.TrimSpace(rd.Source), Format: rd.Format,
			ActualSource: strings.TrimSpace(rd.ActualSource),
			Fold:         strings.TrimSpace(rd.Fold),
		}
		switch rows[i].Fold {
		case "", FoldStock, FoldFlow:
		default:
			return nil, fmt.Errorf("template: row %q has invalid fold %q (use stock or flow)", key, rows[i].Fold)
		}
	}

	// Second pass: children references, formula compilation, basis mixing and
	// cycle detection — once the key index is complete.
	for i := range rows {
		rd := def.Rows[i]
		rows[i].Children = append([]string(nil), rd.Children...)
		rows[i].Subtract = append([]string(nil), rd.Subtract...)
		children := make(map[string]bool, len(rd.Children))
		for _, child := range rd.Children {
			child = strings.TrimSpace(child)
			if _, ok := byKey[child]; !ok {
				return nil, fmt.Errorf("template: row %q references unknown child row %q", rd.Key, child)
			}
			children[child] = true
		}
		for _, sub := range rd.Subtract {
			sub = strings.TrimSpace(sub)
			if !children[sub] {
				return nil, fmt.Errorf("template: row %q subtracts %q which is not among its children", rd.Key, sub)
			}
		}
		if rows[i].Kind != RowSubtotal && len(rd.Children) > 0 {
			return nil, fmt.Errorf("template: row %q declares children but is %s, not subtotal", rd.Key, rows[i].Kind)
		}
		switch rows[i].Kind {
		case RowFormula, RowCheck:
			if strings.TrimSpace(rd.Formula) == "" {
				return nil, fmt.Errorf("template: row %q is %s but has no formula", rd.Key, rows[i].Kind)
			}
			expr, err := compile(rd.Formula, byKey, rd.Key)
			if err != nil {
				return nil, fmt.Errorf("template: row %q: %w", rd.Key, err)
			}
			rows[i].Formula = &Expr{node: expr}
			// S1-9：声明文本必须存活于 Parse——页面编辑器按 def 重建行时依赖它。
			rows[i].FormulaText = strings.TrimSpace(rd.Formula)
		case RowLink:
			if rows[i].Source == "" {
				return nil, fmt.Errorf("template: link row %q must declare a source", rd.Key)
			}
		case RowInput:
		case RowSubtotal:
			if len(rd.Children) == 0 {
				return nil, fmt.Errorf("template: subtotal row %q must declare children", rd.Key)
			}
		default:
			return nil, fmt.Errorf("template: row %q has invalid kind %q", rd.Key, rd.Kind)
		}
		// actual_source 只能落在公式/校验行，且必须绑定事实源（PRD C7：Actual
		// 冻结线左侧的行只读事实聚合，不得从假设推导）。
		if rows[i].ActualSource != "" {
			if rows[i].Kind != RowFormula && rows[i].Kind != RowCheck {
				return nil, fmt.Errorf("template: row %q declares actual_source but is %s, not formula", rd.Key, rows[i].Kind)
			}
			if !strings.HasPrefix(rows[i].ActualSource, "fact.") {
				return nil, fmt.Errorf("template: row %q actual_source must bind a fact.* source, got %q", rd.Key, rows[i].ActualSource)
			}
		}
	}

	if err := validateBasisMixing(def, byKey); err != nil {
		return nil, err
	}
	if err := validateNoCycles(def, byKey); err != nil {
		return nil, err
	}

	return &Template{Name: strings.TrimSpace(def.Name), Major: def.Version, Rows: rows}, nil
}

// validateBasisMixing enforces D-S9: a subtotal may not sum children across
// operating and ifrs16 basis rows; shared subtotals only sum shared rows.
func validateBasisMixing(def TemplateDef, byKey map[string]int) error {
	for _, rd := range def.Rows {
		if rd.Kind != RowSubtotal {
			continue
		}
		for _, child := range rd.Children {
			childKey := strings.TrimSpace(child)
			childRow := def.Rows[byKey[childKey]]
			if childRow.Basis == BasisShared {
				continue
			}
			if rd.Basis == BasisShared {
				return fmt.Errorf("template: subtotal row %q (shared) must not sum %s child %q", rd.Key, childRow.Basis, childKey)
			}
			if rd.Basis != childRow.Basis {
				return fmt.Errorf("template: subtotal row %q (%s) mixes basis child %q (%s)", rd.Key, rd.Basis, childKey, childRow.Basis)
			}
		}
	}
	return nil
}

// validateNoCycles rejects circular same-period references over the
// reference graph built from formula and subtotal rows.
//
// RH6（R2-4）：循环错误用 CycleError 携带完整引用链路，调用方（校验端点）
// 能结构化返回给前端展示，不必 split 错误字符串。
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("template: circular reference: %s", strings.Join(e.Path, " -> "))
}

func validateNoCycles(def TemplateDef, byKey map[string]int) error {
	deps := make(map[string][]string, len(def.Rows))
	for _, rd := range def.Rows {
		switch rd.Kind {
		case RowFormula, RowCheck:
			for _, dep := range refDeps(rd.Formula) {
				deps[rd.Key] = append(deps[rd.Key], dep)
			}
		case RowSubtotal:
			deps[rd.Key] = append(deps[rd.Key], rd.Children...)
		}
	}
	state := map[string]int{} // 0=white 1=grey 2=black
	var visit func(key string, path []string) error
	visit = func(key string, path []string) error {
		switch state[key] {
		case 1:
			return &CycleError{Path: append(append([]string(nil), path...), key)}
		case 2:
			return nil
		}
		state[key] = 1
		for _, dep := range deps[key] {
			dep = strings.TrimSpace(dep)
			if _, exists := byKey[dep]; !exists {
				continue
			}
			if err := visit(dep, append(path, key)); err != nil {
				return err
			}
		}
		state[key] = 2
		return nil
	}
	keys := make([]string, 0, len(def.Rows))
	for _, rd := range def.Rows {
		keys = append(keys, rd.Key)
	}
	sort.Strings(keys) // deterministic diagnostics
	for _, key := range keys {
		if err := visit(key, nil); err != nil {
			return err
		}
	}
	return nil
}

// refDeps extracts the non-lagged `rows.<key>` references from a formula —
// lag(x, n) is a cross-period reference and cannot form a same-period cycle,
// so lag call bodies are stripped before scanning.
func refDeps(formula string) []string {
	seen := map[string]bool{}
	var out []string
	rest := stripFunctionCalls(formula, "lag")
	for {
		idx := strings.Index(rest, "rows.")
		if idx < 0 {
			break
		}
		rest = rest[idx+len("rows."):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				b.WriteRune(r)
				continue
			}
			break
		}
		if key := b.String(); key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

// stripFunctionCalls removes every occurrence of name(...) from src,
// honoring parenthesis nesting.
func stripFunctionCalls(src, name string) string {
	var out strings.Builder
	rest := src
	for {
		idx := strings.Index(rest, name+"(")
		if idx < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:idx])
		// start points at the '(' after the function name; the very first
		// check must see depth 1, not 0.
		depth := 0
		start := idx + len(name)
		j := start
		for ; j < len(rest); j++ {
			switch rest[j] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 && j > start {
				break
			}
		}
		if j >= len(rest) {
			break
		}
		rest = rest[j+1:]
	}
	return out.String()
}
