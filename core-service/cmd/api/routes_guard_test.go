package main

// Route registration guard.
//
// Gin panics at startup when the same method+path is registered twice
// ("handlers are already registered for path ..."). That failure is invisible
// to `go build`, `go vet` and `go test ./...`, because route registration only
// runs inside main() — so a duplicated line ships green and the API simply
// refuses to boot.
//
// This happened for real: W5-5 duplicated the POST /ai/files/upload block
// verbatim (comment included) and the whole test suite stayed green. The guard
// below parses this file's sibling main.go and fails on any repeated
// method+path pair, so the next copy-paste goes red in CI instead of in
// production.
//
// Why AST and not grep: a regex over source text is defeated by line breaks,
// gofmt choices and string concatenation. Walking the call expressions reads
// the same structure the compiler does.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// routeMethods are the gin shorthand registrars (r.GET("/x", ...)).
var routeMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

type routeRegistration struct {
	method string
	path   string
	line   int
}

func TestNoDuplicateRouteRegistration(t *testing.T) {
	routes := parseRoutes(t, "main.go")

	if len(routes) == 0 {
		t.Fatal("no route registrations found — the parser is broken, not the routing table")
	}

	seen := map[string][]int{}
	for _, r := range routes {
		key := r.method + " " + r.path
		seen[key] = append(seen[key], r.line)
	}

	duplicates := make([]string, 0)
	for key, lines := range seen {
		if len(lines) > 1 {
			sort.Ints(lines)
			parts := make([]string, 0, len(lines))
			for _, l := range lines {
				parts = append(parts, strconv.Itoa(l))
			}
			duplicates = append(duplicates, key+" registered at main.go lines "+strings.Join(parts, ", "))
		}
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		t.Fatalf("gin panics on duplicate registration — the API would not start:\n  %s",
			strings.Join(duplicates, "\n  "))
	}
}

// parseRoutes collects every method+path pair registered in the given file.
//
// Two call shapes are recognised:
//
//	x.Handle(http.MethodPost, "/path", ...)   — including ProtectedRouter.Handle
//	x.GET("/path", ...)                        — gin shorthand
//
// A registration whose path is not a plain string literal is reported rather
// than skipped: an unreadable route is a hole in this guard, not a pass.
func parseRoutes(t *testing.T, filename string) []routeRegistration {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	routes := make([]routeRegistration, 0, 256)

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		line := fset.Position(call.Pos()).Line

		switch {
		case sel.Sel.Name == "Handle" && len(call.Args) >= 2:
			method, ok := httpMethodConst(call.Args[0])
			if !ok {
				return true
			}
			path, ok := stringLit(call.Args[1])
			if !ok {
				t.Errorf("main.go:%d: Handle path is not a string literal — this guard cannot see it", line)
				return true
			}
			routes = append(routes, routeRegistration{method: method, path: path, line: line})

		case routeMethods[sel.Sel.Name] && len(call.Args) >= 1:
			path, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			routes = append(routes, routeRegistration{method: sel.Sel.Name, path: path, line: line})
		}
		return true
	})

	return routes
}

// httpMethodConst reads http.MethodPost and friends into "POST".
func httpMethodConst(expr ast.Expr) (string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "http" {
		return "", false
	}
	if !strings.HasPrefix(sel.Sel.Name, "Method") {
		return "", false
	}
	return strings.ToUpper(strings.TrimPrefix(sel.Sel.Name, "Method")), true
}

func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
