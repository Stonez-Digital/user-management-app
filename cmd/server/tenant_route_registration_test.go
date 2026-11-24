package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

func TestTenantScopedRoutesRegisterTenantTransaction(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse server routes: %v", err)
	}

	type routeKey struct {
		group, method, path string
	}
	registered := make(map[routeKey]bool)

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		group, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if group.Name != "admin" && group.Name != "student" && group.Name != "parent" {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		pathLiteral, ok := call.Args[0].(*ast.BasicLit)
		if !ok || pathLiteral.Kind != token.STRING {
			return true
		}
		path, err := strconv.Unquote(pathLiteral.Value)
		if err != nil {
			return true
		}
		hasTenantTransaction := false
		for _, arg := range call.Args[1:] {
			ast.Inspect(arg, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "TenantTransaction" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if ok && pkg.Name == "middleware" {
					hasTenantTransaction = true
				}
				return true
			})
		}
		registered[routeKey{group.Name, selector.Sel.Name, path}] = hasTenantTransaction
		return true
	})

	required := []routeKey{
		{"admin", "GET", "/school"},
		{"admin", "PUT", "/school"},
		{"admin", "POST", "/school/logo"},
		{"admin", "GET", "/blog/posts"},
		{"admin", "POST", "/blog/posts"},
		{"admin", "GET", "/gallery/albums"},
		{"admin", "POST", "/gallery/albums/:id/images"},
		{"admin", "GET", "/advertisements"},
		{"admin", "GET", "/enquiries"},
		{"admin", "GET", "/audit-logs"},
		{"student", "GET", "/careerpath/assessment"},
		{"student", "POST", "/careerpath/attempts"},
		{"student", "GET", "/careerpath/attempts/:id"},
		{"student", "PUT", "/careerpath/attempts/:id/responses"},
		{"student", "POST", "/careerpath/attempts/:id/submit"},
		{"parent", "GET", "/notifications"},
		{"parent", "POST", "/notifications/:id/read"},
	}

	for _, route := range required {
		t.Run(route.group+"_"+route.method+"_"+route.path, func(t *testing.T) {
			hasTenantTransaction, exists := registered[route]
			if !exists {
				t.Fatalf("required route is not registered: %s %s on %s group", route.method, route.path, route.group)
			}
			if !hasTenantTransaction {
				t.Fatalf("route is missing middleware.TenantTransaction: %s %s on %s group", route.method, route.path, route.group)
			}
		})
	}
}
