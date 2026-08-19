package template

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// The DSL is a whitelist grammar (PRD S3-3 / D-S9): row references
// (rows.<key>), cross-period references (lag(rows.<key>, n)), arithmetic,
// sum/avg/min/max, if with comparisons, and division that yields nil on a
// zero divisor. SQL keywords, arbitrary identifiers, cross-legal-entity
// references and numeric literals other than 0/1 (outside lag distances) are
// rejected at Parse time with the offending row attached.

var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "insert": true, "update": true,
	"delete": true, "into": true, "union": true, "join": true, "drop": true,
	"alter": true, "create": true, "exec": true, "execute": true, "script": true,
	"grant": true, "revoke": true, "truncate": true, "merge": true, "call": true,
}

var callNames = map[string]bool{"sum": true, "avg": true, "min": true, "max": true, "if": true, "lag": true}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokIdent
	tokDot
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokLParen
	tokRParen
	tokComma
	tokLt
	tokLe
	tokGt
	tokGe
	tokEq
	tokNe
)

type token struct {
	kind tokenKind
	text string
	num  float64
}

type lexer struct {
	src string
	pos int
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.src) {
		r := rune(l.src[l.pos])
		if unicode.IsSpace(r) {
			l.pos++
			continue
		}
		break
	}
	if l.pos >= len(l.src) {
		return token{kind: tokEOF}, nil
	}
	start := l.pos
	c := l.src[l.pos]
	switch c {
	case '(':
		l.pos++
		return token{kind: tokLParen, text: "("}, nil
	case ')':
		l.pos++
		return token{kind: tokRParen, text: ")"}, nil
	case ',':
		l.pos++
		return token{kind: tokComma, text: ","}, nil
	case '+':
		l.pos++
		return token{kind: tokPlus, text: "+"}, nil
	case '-':
		l.pos++
		return token{kind: tokMinus, text: "-"}, nil
	case '*':
		l.pos++
		return token{kind: tokStar, text: "*"}, nil
	case '/':
		l.pos++
		return token{kind: tokSlash, text: "/"}, nil
	case '<':
		if l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.pos += 2
			return token{kind: tokLe, text: "<="}, nil
		}
		l.pos++
		return token{kind: tokLt, text: "<"}, nil
	case '>':
		if l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.pos += 2
			return token{kind: tokGe, text: ">="}, nil
		}
		l.pos++
		return token{kind: tokGt, text: ">"}, nil
	case '=':
		if l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.pos += 2
			return token{kind: tokEq, text: "=="}, nil
		}
		return token{}, fmt.Errorf("unexpected '=' (use == for equality)")
	case '!':
		if l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.pos += 2
			return token{kind: tokNe, text: "!="}, nil
		}
		return token{}, fmt.Errorf("unexpected '!' (use != for inequality)")
	case '.':
		l.pos++
		return token{kind: tokDot, text: "."}, nil
	}
	if c >= '0' && c <= '9' || c == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9' {
		rest := l.src[l.pos:]
		i := 0
		for i < len(rest) && (rest[i] >= '0' && rest[i] <= '9' || rest[i] == '.') {
			i++
		}
		num, err := strconv.ParseFloat(rest[:i], 64)
		if err != nil {
			return token{}, fmt.Errorf("bad number %q", rest[:i])
		}
		l.pos += i
		return token{kind: tokNumber, text: rest[:i], num: num}, nil
	}
	if isIdentStart(rune(c)) {
		rest := l.src[l.pos:]
		i := 0
		for i < len(rest) && isIdentPart(rune(rest[i])) {
			i++
		}
		text := rest[:i]
		l.pos += i
		return token{kind: tokIdent, text: text}, nil
	}
	return token{}, fmt.Errorf("unexpected character %q at %d", string(c), start)
}

func isIdentStart(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || r >= '0' && r <= '9'
}

type parser struct {
	src  string
	lex  *lexer
	tok  token
	peek *token
}

func (p *parser) advance() (token, error) {
	if p.peek != nil {
		t := *p.peek
		p.peek = nil
		p.tok = t
		return t, nil
	}
	t, err := p.lex.next()
	if err != nil {
		return token{}, err
	}
	p.tok = t
	return t, nil
}

func (p *parser) lookahead() (token, error) {
	if p.peek != nil {
		return *p.peek, nil
	}
	t, err := p.lex.next()
	if err != nil {
		return token{}, err
	}
	p.peek = &t
	return t, nil
}

// compile turns a formula string into an AST, validating every reference
// against the template's own key set.
func compile(formula string, byKey map[string]int, rowKey string) (node, error) {
	p := &parser{src: formula, lex: &lexer{src: formula}}
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	n, err := p.parseExpr(byKey, rowKey, false)
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokEOF {
		return nil, fmt.Errorf("unexpected trailing input %q", p.tok.text)
	}
	return n, nil
}

// parseExpr := comparison
func (p *parser) parseExpr(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	return p.parseComparison(byKey, rowKey, inLag)
}

func (p *parser) parseComparison(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	left, err := p.parseAdditive(byKey, rowKey, inLag)
	if err != nil {
		return nil, err
	}
	switch p.tok.kind {
	case tokLt, tokLe, tokGt, tokGe, tokEq, tokNe:
		op := p.tok.text
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseAdditive(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		return binaryNode{op: op, left: left, right: right}, nil
	}
	return left, nil
}

func (p *parser) parseAdditive(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	left, err := p.parseMultiplicative(byKey, rowKey, inLag)
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokPlus || p.tok.kind == tokMinus {
		op := p.tok.text
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseMultiplicative(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	left, err := p.parseUnary(byKey, rowKey, inLag)
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokStar || p.tok.kind == tokSlash {
		op := p.tok.text
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	if p.tok.kind == tokMinus {
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		operand, err := p.parseUnary(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		return unaryNode{op: "-", operand: operand}, nil
	}
	return p.parsePrimary(byKey, rowKey, inLag)
}

func (p *parser) parsePrimary(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	switch p.tok.kind {
	case tokNumber:
		num := p.tok.num
		if !inLag && num != 0 && num != 1 {
			return nil, fmt.Errorf("numeric literal %s is forbidden in formulas: reference the assumption row instead (only 0 and 1 are allowed)", p.tok.text)
		}
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		return literalNode{value: num}, nil
	case tokLParen:
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseExpr(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tokRParen {
			return nil, fmt.Errorf("expected ')', got %q", p.tok.text)
		}
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		return inner, nil
	case tokIdent:
		return p.parseIdent(byKey, rowKey, inLag)
	default:
		return nil, fmt.Errorf("unexpected token %q", p.tok.text)
	}
}

func (p *parser) parseIdent(byKey map[string]int, rowKey string, inLag bool) (node, error) {
	name := p.tok.text
	lower := strings.ToLower(name)
	if sqlKeywords[lower] {
		return nil, fmt.Errorf("SQL keyword %q is forbidden in formulas", name)
	}
	if name == "rows" {
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokDot {
			return nil, fmt.Errorf("expected '.' after rows, got %q", p.tok.text)
		}
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent {
			return nil, fmt.Errorf("expected row key after rows., got %q", p.tok.text)
		}
		key := p.tok.text
		if _, ok := byKey[key]; !ok {
			return nil, fmt.Errorf("reference to unknown row %q", key)
		}
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		return refNode{key: key}, nil
	}
	if callNames[lower] {
		args, err := p.parseCall(byKey, rowKey, inLag)
		if err != nil {
			return nil, err
		}
		if lower == "lag" {
			if len(args) != 2 {
				return nil, fmt.Errorf("lag takes exactly (row, n) arguments")
			}
			ref, ok := args[0].(refNode)
			if !ok {
				return nil, fmt.Errorf("lag's first argument must be a rows.<key> reference")
			}
			lit, ok := args[1].(literalNode)
			if !ok || lit.value < 0 || lit.value != float64(int(lit.value)) {
				return nil, fmt.Errorf("lag's second argument must be a non-negative integer")
			}
			return refNode{key: ref.key, lag: int(lit.value)}, nil
		}
		if lower == "if" {
			if len(args) != 3 {
				return nil, fmt.Errorf("if takes exactly (condition, then, else) arguments")
			}
		}
		if len(args) == 0 {
			return nil, fmt.Errorf("%s takes at least one argument", lower)
		}
		return callNode{fn: lower, args: args}, nil
	}
	return nil, fmt.Errorf("unknown reference %q in row %q — formulas may reference rows.<key>, lag() and the whitelist functions only", name, rowKey)
}

func (p *parser) parseCall(byKey map[string]int, rowKey string, inLag bool) ([]node, error) {
	// Consume '('.
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind != tokLParen {
		return nil, fmt.Errorf("expected '(' after function name")
	}
	var args []node
	if _, err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind == tokRParen {
		if _, err := p.advance(); err != nil {
			return nil, err
		}
		return args, nil
	}
	for {
		if p.tok.kind == tokIdent && lowerEq(p.tok.text, "lag") {
			arg, err := p.parseIdent(byKey, rowKey, true)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		} else {
			arg, err := p.parseExpr(byKey, rowKey, inLag)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		if p.tok.kind == tokComma {
			if _, err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		if p.tok.kind == tokRParen {
			if _, err := p.advance(); err != nil {
				return nil, err
			}
			return args, nil
		}
		return nil, fmt.Errorf("expected ',' or ')', got %q", p.tok.text)
	}
}

func lowerEq(s, want string) bool { return strings.ToLower(s) == want }

// evalNode computes an AST node's value. Missing operands produce nil:
// arithmetic and aggregates propagate nil (D-S4 — never zero-fill).
func evalNode(n node, refs func(string) *float64, lagged func(string, int) *float64) *float64 {
	switch v := n.(type) {
	case literalNode:
		val := v.value
		return &val
	case refNode:
		if v.lag > 0 {
			return lagged(v.key, v.lag)
		}
		return refs(v.key)
	case unaryNode:
		operand := evalNode(v.operand, refs, lagged)
		if operand == nil {
			return nil
		}
		val := -*operand
		return &val
	case binaryNode:
		switch v.op {
		case "+", "-", "*", "/":
			l, r := evalNode(v.left, refs, lagged), evalNode(v.right, refs, lagged)
			if l == nil || r == nil {
				return nil
			}
			if v.op == "/" && *r == 0 {
				return nil // division guard: zero divisor yields missing, not NaN
			}
			val := 0.0
			switch v.op {
			case "+":
				val = *l + *r
			case "-":
				val = *l - *r
			case "*":
				val = *l * *r
			case "/":
				val = *l / *r
			}
			return &val
		case "<", "<=", ">", ">=", "==", "!=":
			l, r := evalNode(v.left, refs, lagged), evalNode(v.right, refs, lagged)
			if l == nil || r == nil {
				return nil
			}
			out := 0.0
			switch v.op {
			case "<":
				if *l < *r {
					out = 1
				}
			case "<=":
				if *l <= *r {
					out = 1
				}
			case ">":
				if *l > *r {
					out = 1
				}
			case ">=":
				if *l >= *r {
					out = 1
				}
			case "==":
				if *l == *r {
					out = 1
				}
			case "!=":
				if *l != *r {
					out = 1
				}
			}
			return &out
		}
	case callNode:
		return evalCall(v, refs, lagged)
	}
	return nil
}

func evalCall(c callNode, refs func(string) *float64, lagged func(string, int) *float64) *float64 {
	if c.fn == "if" {
		cond := evalNode(c.args[0], refs, lagged)
		if cond == nil {
			return nil
		}
		if *cond != 0 {
			return evalNode(c.args[1], refs, lagged)
		}
		return evalNode(c.args[2], refs, lagged)
	}
	vals := make([]float64, 0, len(c.args))
	for _, arg := range c.args {
		v := evalNode(arg, refs, lagged)
		if v == nil {
			return nil // a missing member makes the aggregate missing
		}
		vals = append(vals, *v)
	}
	var out float64
	switch c.fn {
	case "sum":
		for _, v := range vals {
			out += v
		}
	case "avg":
		for _, v := range vals {
			out += v
		}
		out /= float64(len(vals))
	case "min":
		out = vals[0]
		for _, v := range vals[1:] {
			if v < out {
				out = v
			}
		}
	case "max":
		out = vals[0]
		for _, v := range vals[1:] {
			if v > out {
				out = v
			}
		}
	default:
		return nil
	}
	return &out
}
