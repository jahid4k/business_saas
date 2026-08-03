package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var openRoutesAllowlist = map[string]bool{
	// Example: "employees/routes.go:GET /my-profile": true,
}

var httpMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Patch": true, "Delete": true, "All": true,
}

func TestPermissions_AllRoutesProtected(t *testing.T) {
	backendDir := "../../../.."
	hrmDir := filepath.Join(backendDir, "internal", "hrm")

	err := filepath.Walk(hrmDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "routes.go" {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if !httpMethods[sel.Sel.Name] {
				return true
			}

			// We found a route registration like `router.Get(...)` or `group.Post(...)`
			// Check if it's actually on a router (simple heuristic: first arg is string path)
			if len(call.Args) < 2 {
				return true
			}
			pathLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathLit.Kind != token.STRING {
				return true
			}

			routePath := strings.Trim(pathLit.Value, `"`)
			hasPermFn := false
			permString := ""

			for _, arg := range call.Args[1:] {
				if argCall, ok := arg.(*ast.CallExpr); ok {
					if argIdent, ok := argCall.Fun.(*ast.Ident); ok && argIdent.Name == "permFn" {
						hasPermFn = true
						if len(argCall.Args) > 0 {
							if pl, ok := argCall.Args[0].(*ast.BasicLit); ok && pl.Kind == token.STRING {
								permString = strings.Trim(pl.Value, `"`)
							}
						}
					}
				}
			}

			relPath, _ := filepath.Rel(hrmDir, path)
			allowlistKey := relPath + ":" + sel.Sel.Name + " " + routePath

			if !hasPermFn {
				if !openRoutesAllowlist[allowlistKey] {
					pos := fset.Position(call.Pos())
					t.Errorf("🚨 PERMISSION MISSING in %s:%d\nRoute '%s %s' does not have authz.RequirePermission (via permFn).\nIf this is intentionally public, add '%s' to openRoutesAllowlist in permissions_test.go.\n",
						relPath, pos.Line, sel.Sel.Name, routePath, allowlistKey)
				}
			} else {
				if !strings.HasPrefix(permString, "hrm.") {
					pos := fset.Position(call.Pos())
					t.Errorf("🚨 INVALID PERMISSION SCOPE in %s:%d\nRoute '%s %s' requires permission '%s', which does not start with 'hrm.'.\n",
						relPath, pos.Line, sel.Sel.Name, routePath, permString)
				}
			}

			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk HRM directory: %v", err)
	}
}

// permissionKeyPattern matches the first field of a permission INSERT tuple, e.g.
// ('hrm.employees.view', 'hrm.employees', 'view', '...') — every seed migration in
// this repo follows this shape.
var permissionKeyPattern = regexp.MustCompile(`\(\s*'([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)+)'\s*,\s*'`)

// seededPermissionKeys scans every migration for permission keys inserted into the
// `permissions` table, across the whole app — not just HRM.
func seededPermissionKeys(t *testing.T, migrationsDir string) map[string]bool {
	t.Helper()
	keys := make(map[string]bool)
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range permissionKeyPattern.FindAllStringSubmatch(string(content), -1) {
			keys[m[1]] = true
		}
	}
	return keys
}

// TestPermissions_UsedStringsExistInMigrations is the bidirectional check: every
// permFn("...") string referenced anywhere in the app (not just HRM) must exist as a
// seeded key in some migration. This is the check that would have caught the
// capture.visitors.view / crm.view class of mismatch — a permission string used in
// code that was never actually seeded, so it can never be granted to any role.
func TestPermissions_UsedStringsExistInMigrations(t *testing.T) {
	backendDir := "../../../.."
	internalDir := filepath.Join(backendDir, "internal")
	migrationsDir := filepath.Join(backendDir, "internal", "migrations")

	seeded := seededPermissionKeys(t, migrationsDir)
	if len(seeded) == 0 {
		t.Fatal("no permission keys found in migrations — the scan regex or migrations path is broken")
	}

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() != "routes.go" {
			return nil
		}

		fset := token.NewFileSet()
		node, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Errorf("Failed to parse %s: %v", path, perr)
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "permFn" {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			permString := strings.Trim(lit.Value, `"`)

			if !seeded[permString] {
				relPath, _ := filepath.Rel(internalDir, path)
				pos := fset.Position(call.Pos())
				t.Errorf("🚨 UNSEEDED PERMISSION in %s:%d\nRoute requires permission '%s' but no migration seeds this key into the `permissions` table — it can never be granted to any role.\n",
					relPath, pos.Line, permString)
			}
			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk internal directory: %v", err)
	}
}
