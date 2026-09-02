package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var moneyTerms = []string{
	"amount", "pay", "salary", "price", "cost", "bonus", "severance",
}

func isMoneyField(name string) bool {
	lowerName := strings.ToLower(name)
	for _, term := range moneyTerms {
		if strings.Contains(lowerName, term) {
			return true
		}
	}
	return false
}

func isFloatType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "float32" || t.Name == "float64"
	case *ast.StarExpr:
		return isFloatType(t.X)
	}
	return false
}

func TestHygiene_NoFloatMoneyFields(t *testing.T) {
	// We run this from backend/internal/tests/unit/architecture
	// So we need to look at ../../../.. to get to backend/
	backendDir := "../../../.."

	err := filepath.Walk(backendDir+"/internal", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "model.go") && !strings.HasSuffix(info.Name(), "models.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			return nil
		}

		ast.Inspect(node, func(n ast.Node) bool {
			// Look for struct types
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			// Iterate over fields
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if isMoneyField(name.Name) && isFloatType(field.Type) {
						// Extract position
						pos := fset.Position(field.Pos())
						t.Errorf("🚨 HYGIENE VIOLATION in %s:%d\nField '%s' in struct '%s' appears to represent money but uses a float type instead of decimal.Decimal.\n",
							path, pos.Line, name.Name, typeSpec.Name.Name)
					}
				}
			}

			return true
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk backend directory: %v", err)
	}
}

// aiArtifactPatterns catches leftover AI-conversation internal monologue that should
// never have shipped in source comments — e.g. "// Wait, we don't have it open. Let me
// run a query to check..." found (and removed) in capture/public/handler.go.
var aiArtifactPatterns = []string{
	"wait,", "wait!", "actually,", "hmm,", "let me ", "i'll ", "i think", "as an ai",
}

func hasAIArtifact(text string) bool {
	lower := strings.ToLower(text)
	for _, p := range aiArtifactPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func TestHygiene_NoAIConversationArtifacts(t *testing.T) {
	backendDir := "../../../.."

	err := filepath.Walk(backendDir+"/internal", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			t.Errorf("Failed to parse %s: %v", path, perr)
			return nil
		}

		for _, cg := range node.Comments {
			for _, c := range cg.List {
				if hasAIArtifact(c.Text) {
					pos := fset.Position(c.Pos())
					relPath, _ := filepath.Rel(backendDir, path)
					t.Errorf("🚨 AI-CONVERSATION ARTIFACT in %s:%d\nComment reads like leftover internal monologue rather than documentation: %q\n",
						relPath, pos.Line, strings.TrimSpace(c.Text))
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk backend directory: %v", err)
	}
}
