// backend/internal/tests/unit/hrm/scope/predicate_test.go
package scope_test

import (
	"strings"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

func TestPredicate_ScopeAll_ReturnsTrueLiteral(t *testing.T) {
	frag, args := scope.Predicate(authz.ScopeAll, "employee_id", 0, "org-1", "user-1", scope.DefaultMaxDepth)
	if frag != "TRUE" {
		t.Errorf("expected literal TRUE for ScopeAll, got %q", frag)
	}
	if len(args) != 0 {
		t.Errorf("expected zero args for ScopeAll, got %v", args)
	}
}

func TestPredicate_ScopeNone_ReturnsFalseLiteral(t *testing.T) {
	frag, args := scope.Predicate(authz.ScopeNone, "employee_id", 0, "org-1", "user-1", scope.DefaultMaxDepth)
	if frag != "FALSE" {
		t.Errorf("expected literal FALSE for ScopeNone, got %q", frag)
	}
	if len(args) != 0 {
		t.Errorf("expected zero args for ScopeNone, got %v", args)
	}
}

func TestPredicate_ScopeOwn_PlaceholderNumberingAtOffset(t *testing.T) {
	frag, args := scope.Predicate(authz.ScopeOwn, "employee_id", 3, "org-1", "user-1", scope.DefaultMaxDepth)

	if !strings.Contains(frag, "$4") || !strings.Contains(frag, "$5") {
		t.Errorf("expected placeholders $4 and $5 at argOffset=3, got fragment: %s", frag)
	}
	if strings.Contains(frag, "$1") || strings.Contains(frag, "$2") || strings.Contains(frag, "$3") {
		t.Errorf("fragment must not reuse placeholders below argOffset+1, got: %s", frag)
	}
	if !strings.Contains(frag, "employee_id =") {
		t.Errorf("expected the predicate to filter on the given column name, got: %s", frag)
	}
	if len(args) != 2 || args[0] != "org-1" || args[1] != "user-1" {
		t.Errorf("expected args [org-1, user-1], got %v", args)
	}
}

func TestPredicate_ScopeTeam_PlaceholderNumberingAtOffset(t *testing.T) {
	frag, args := scope.Predicate(authz.ScopeTeam, "employee_id", 3, "org-1", "user-1", 2)

	for _, ph := range []string{"$4", "$5", "$6"} {
		if !strings.Contains(frag, ph) {
			t.Errorf("expected placeholder %s in ScopeTeam fragment at argOffset=3, got: %s", ph, frag)
		}
	}
	if len(args) != 3 || args[0] != "org-1" || args[1] != "user-1" || args[2] != 2 {
		t.Errorf("expected args [org-1, user-1, 2], got %v", args)
	}
}

func TestPredicate_ScopeTeam_ContainsCycleGuardAndDepthBound(t *testing.T) {
	frag, _ := scope.Predicate(authz.ScopeTeam, "employee_id", 0, "org-1", "user-1", scope.DefaultMaxDepth)

	if !strings.Contains(frag, "s.depth <") {
		t.Errorf("expected an explicit depth bound (s.depth < $N) in the recursive term, got: %s", frag)
	}
	if !strings.Contains(frag, "<> ALL(s.path)") {
		t.Errorf("expected an explicit cycle/path guard (he.id <> ALL(s.path)) in the recursive term, got: %s", frag)
	}
	if !strings.Contains(frag, "WITH RECURSIVE") {
		t.Errorf("expected a recursive CTE, got: %s", frag)
	}
}

func TestPredicate_ScopeTeam_IsInclusiveOfOwn(t *testing.T) {
	frag, _ := scope.Predicate(authz.ScopeTeam, "employee_id", 0, "org-1", "user-1", scope.DefaultMaxDepth)

	if !strings.Contains(frag, "UNION ALL") {
		t.Errorf("expected the team predicate to UNION in the caller's own employee id, got: %s", frag)
	}
}

func TestPredicate_ColumnNameIsRespected(t *testing.T) {
	frag, _ := scope.Predicate(authz.ScopeOwn, "id", 0, "org-1", "user-1", scope.DefaultMaxDepth)
	if !strings.HasPrefix(frag, "id =") {
		t.Errorf("expected fragment to filter on column %q for the employees table itself, got: %s", "id", frag)
	}
}
