package runtime_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestRateLimitDeadlineSchedulingDoesNotReadStateCache(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "rate_limit.go", nil, 0)
	if err != nil {
		t.Fatalf("parse rate_limit.go: %v", err)
	}

	found := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "nextRateLimitDeadline" {
			continue
		}
		found = true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Load" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "state" {
				t.Errorf("nextRateLimitDeadline must use the in-memory cooldown index, not state.Load")
			}
			return true
		})
	}
	if !found {
		t.Fatal("nextRateLimitDeadline function not found")
	}
}
