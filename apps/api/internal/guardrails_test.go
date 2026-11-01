// Package internal holds static architecture checks for the guardrails in
// .agent/rules/project_guardrails.md §1. The checks are syntax-only.
package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rawRouteMethods are the chi methods that mount a route outside huma.Register.
var rawRouteMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true,
	"Mount": true, "Handle": true, "HandleFunc": true,
}

type goFile struct {
	path string
	ast  *ast.File
	fset *token.FileSet
}

func (f goFile) line(n ast.Node) int { return f.fset.Position(n.Pos()).Line }

// parseGoFiles parses non-test .go files under root, skipping any directory in
// skip. It fails on a tree with no files so a moved path cannot pass silently.
func parseGoFiles(t *testing.T, root string, skip ...string) []goFile {
	t.Helper()
	var files []goFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, dir := range skip {
				if path == dir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files = append(files, goFile{path: path, ast: parsed, fset: fset})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) == 0 {
		t.Fatalf("no Go files found under %s", root)
	}
	return files
}

// isChiExpr reports whether e is written as a chi type or constructor call.
func isChiExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.StarExpr:
		return isChiExpr(x.X)
	case *ast.CallExpr:
		return isChiExpr(x.Fun)
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		return ok && pkg.Name == "chi"
	}
	return false
}

// routerNames returns the file's identifiers bound to a chi router. Syntax
// alone cannot follow a router through a call result, so "router" is always in.
func routerNames(file *ast.File) map[string]bool {
	names := map[string]bool{"router": true, "chi": true}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) != len(x.Rhs) {
				return true
			}
			for i, rhs := range x.Rhs {
				if id, ok := x.Lhs[i].(*ast.Ident); ok && isChiExpr(rhs) {
					names[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			if x.Type != nil && isChiExpr(x.Type) {
				for _, id := range x.Names {
					names[id.Name] = true
				}
			}
			for i, value := range x.Values {
				if i < len(x.Names) && isChiExpr(value) {
					names[x.Names[i].Name] = true
				}
			}
		case *ast.Field:
			if isChiExpr(x.Type) {
				for _, id := range x.Names {
					names[id.Name] = true
				}
			}
		}
		return true
	})
	return names
}

// receiverName resolves router, s.router, and router.With(m) to "router".
func receiverName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			return receiverName(sel.X)
		}
	}
	return ""
}

func TestNoRawChiRoutes(t *testing.T) {
	files := parseGoFiles(t, "../cmd")
	files = append(files, parseGoFiles(t, ".")...)
	for _, f := range files {
		names := routerNames(f.ast)
		ast.Inspect(f.ast, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok && rawRouteMethods[sel.Sel.Name] && names[receiverName(sel.X)] {
				t.Errorf("%s:%d: raw chi route .%s bypasses huma.Register", f.path, f.line(call), sel.Sel.Name)
			}
			return true
		})
	}
}

func TestLayerImportsAndSQLBoundaries(t *testing.T) {
	importPath := func(spec *ast.ImportSpec) string { return strings.Trim(spec.Path.Value, `"`) }

	for _, root := range []string{"handler", "service", "domain"} {
		for _, f := range parseGoFiles(t, root) {
			for _, spec := range f.ast.Imports {
				path := importPath(spec)
				if path == "database/sql" || path == "github.com/jackc/pgx" || strings.HasPrefix(path, "github.com/jackc/pgx/") {
					t.Errorf("%s:%d: %s must not import %s; depend on domain interfaces", f.path, f.line(spec), root, path)
				}
			}
		}
	}

	repository := "repository"
	for _, f := range parseGoFiles(t, repository, filepath.Join(repository, "db")) {
		for _, spec := range f.ast.Imports {
			if importPath(spec) == "database/sql" {
				t.Errorf("%s:%d: repository must not import database/sql; use sqlc", f.path, f.line(spec))
			}
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, _ := sel.X.(*ast.Ident)
			switch name := sel.Sel.Name; {
			case name == "Query" || name == "QueryRow" || name == "Exec":
				t.Errorf("%s:%d: raw SQL call .%s; delegate to internal/repository/db (sqlc)", f.path, f.line(call), name)
			case name == "Sprintf" && pkg != nil && pkg.Name == "fmt":
				t.Errorf("%s:%d: fmt.Sprintf is banned in repositories; write SQL in sql/queries", f.path, f.line(call))
			}
			return true
		})
	}
}
