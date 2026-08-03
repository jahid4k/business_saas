package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var methodsToCheck = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Patch": true, "Delete": true, "All": true,
}

func TestRouting_NoDuplicates(t *testing.T) {
	backendDir := "../../../.."
	internalDir := filepath.Join(backendDir, "internal")

	type RouteEntry struct {
		File string
		Line int
	}
	// map of "Method /path" -> RouteEntry
	seenRoutes := make(map[string]RouteEntry)

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "routes.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			return nil
		}

		var currentFunc string

		ast.Inspect(node, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				currentFunc = funcDecl.Name.Name
				// We still return true to visit the children of this function
			}

			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if !methodsToCheck[sel.Sel.Name] {
				return true
			}

			if len(call.Args) < 1 {
				return true
			}
			pathLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathLit.Kind != token.STRING {
				return true
			}

			routePath := strings.Trim(pathLit.Value, `"`)
			
			var receiverName string
			if ident, ok := sel.X.(*ast.Ident); ok {
				receiverName = ident.Name
			} else {
				receiverName = "unknown"
			}

			// Normalize parameter names so "/:id" and "/:userId" are treated as overlaps
			// (A basic normalization: replace any segment starting with ':' with ':param')
			segments := strings.Split(routePath, "/")
			for i, seg := range segments {
				if strings.HasPrefix(seg, ":") {
					segments[i] = ":param"
				}
			}
			normalizedPath := strings.Join(segments, "/")

			// Create a unique key for the route within this file's context
			// (Since we don't resolve the full group prefix, we include the package/directory name, function, and receiver)
			relDir := filepath.Dir(path)
			key := fmt.Sprintf("[%s] %s::%s.%s %s", relDir, currentFunc, receiverName, sel.Sel.Name, normalizedPath)

			pos := fset.Position(call.Pos())

			if existing, found := seenRoutes[key]; found {
				t.Errorf("🚨 ROUTING OVERLAP in %s:%d\nRoute '%s %s' is already registered at %s:%d!\n",
					path, pos.Line, sel.Sel.Name, routePath, existing.File, existing.Line)
			} else {
				seenRoutes[key] = RouteEntry{File: path, Line: pos.Line}
			}

			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk internal directory: %v", err)
	}
}
