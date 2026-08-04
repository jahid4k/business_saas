// backend/internal/tests/integration/hrm_scope_test.go
// Proves the recursive manager-chain CTE in internal/hrm/scope against a real
// Postgres — this cannot be meaningfully proven with mocks. Gate: INTEGRATION=1
package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/organizations"
)

// seedScopeTestOrg creates an org, an "active" employee status, and returns
// the org id, status id, and the org owner's user id (used as created_by for
// seeded rows) plus a cleanup function.
func seedScopeTestOrg(t *testing.T, env *testEnv) (orgID, statusID, ownerUserID string) {
	t.Helper()
	ctx := context.Background()

	email := uniqueEmail("scope-owner")
	owner, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: email, Password: "OwnerPass123!"})
	if err != nil {
		t.Fatalf("signup org owner: %v", err)
	}
	org, err := env.orgSvc.Create(ctx, owner.ID, organizations.CreateBusinessRequest{
		Name: "Scope Test Org", Slug: uniqueSlug("scope-org"),
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	err = env.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_statuses (org_id, name, category) VALUES ($1, 'Active', 'active') RETURNING id`,
		org.ID,
	).Scan(&statusID)
	if err != nil {
		t.Fatalf("seed employee status: %v", err)
	}

	t.Cleanup(func() {
		cleanupUser(t, env, owner.ID)
		_, _ = env.db.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org.ID)
	})

	return org.ID, statusID, owner.ID
}

// seedEmployee inserts a bare hrm_employees row and returns its id.
// userID may be empty (no linked platform account).
func seedEmployee(t *testing.T, env *testEnv, orgID, statusID, createdBy, userID, name string, managerID *string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	var uid any
	if userID != "" {
		uid = userID
	}
	err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_employees (org_id, status_id, user_id, first_name, hire_date, manager_id, created_by)
		 VALUES ($1, $2, $3, $4, CURRENT_DATE, $5, $6) RETURNING id`,
		orgID, statusID, uid, name, managerID, createdBy,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed employee %s: %v", name, err)
	}
	return id
}

func fetchScopedIDs(t *testing.T, env *testEnv, tier authz.Scope, orgID, callerUserID string, maxDepth int) []string {
	t.Helper()
	ctx := context.Background()
	frag, args := scope.Predicate(tier, "id", 1, orgID, callerUserID, maxDepth)
	q := fmt.Sprintf(`SELECT id::text FROM hrm_employees WHERE org_id = $1 AND (%s) ORDER BY id`, frag)
	allArgs := append([]any{orgID}, args...)

	rows, err := env.db.Query(ctx, q, allArgs...)
	if err != nil {
		t.Fatalf("scoped query: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func containsExactly(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// TestIntegration_HRMScope_DirectReportsOnlyAtDefaultDepth proves the shipped
// default (maxDepth=1): a 4-level chain A->B->C->D only exposes A's direct
// report B to A, not the whole subtree.
func TestIntegration_HRMScope_DirectReportsOnlyAtDefaultDepth(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	rootEmail := uniqueEmail("scope-root")
	root, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: rootEmail, Password: "RootPass123!"})
	if err != nil {
		t.Fatalf("signup root: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, root.ID) })

	aID := seedEmployee(t, env, orgID, statusID, ownerID, root.ID, "A", nil)
	bID := seedEmployee(t, env, orgID, statusID, ownerID, "", "B", &aID)
	cID := seedEmployee(t, env, orgID, statusID, ownerID, "", "C", &bID)
	_ = seedEmployee(t, env, orgID, statusID, ownerID, "", "D", &cID)

	got := fetchScopedIDs(t, env, authz.ScopeTeam, orgID, root.ID, scope.DefaultMaxDepth)
	if !containsExactly(got, aID, bID) {
		t.Errorf("depth=1: expected exactly [A(own), B(direct report)], got %v", got)
	}
}

// TestIntegration_HRMScope_WideningDepthWidensVisibility proves that raising
// maxDepth alone (no query-shape change) widens ScopeTeam to multi-level.
func TestIntegration_HRMScope_WideningDepthWidensVisibility(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	rootEmail := uniqueEmail("scope-root2")
	root, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: rootEmail, Password: "RootPass123!"})
	if err != nil {
		t.Fatalf("signup root: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, root.ID) })

	aID := seedEmployee(t, env, orgID, statusID, ownerID, root.ID, "A", nil)
	bID := seedEmployee(t, env, orgID, statusID, ownerID, "", "B", &aID)
	cID := seedEmployee(t, env, orgID, statusID, ownerID, "", "C", &bID)
	dID := seedEmployee(t, env, orgID, statusID, ownerID, "", "D", &cID)

	got := fetchScopedIDs(t, env, authz.ScopeTeam, orgID, root.ID, 3)
	if !containsExactly(got, aID, bID, cID, dID) {
		t.Errorf("depth=3: expected the whole chain [A,B,C,D], got %v", got)
	}
}

// TestIntegration_HRMScope_CycleTerminatesAndStaysBounded proves the path
// guard: a 3-node cycle X->Y->Z->X (legal today — only direct self-reference
// is blocked at the app layer) must not cause runaway recursion or an
// inflated result set when maxDepth is larger than the cycle itself.
func TestIntegration_HRMScope_CycleTerminatesAndStaysBounded(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	cycleEmail := uniqueEmail("scope-cycle")
	cycleUser, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: cycleEmail, Password: "CyclePass123!"})
	if err != nil {
		t.Fatalf("signup cycle user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, cycleUser.ID) })

	xID := seedEmployee(t, env, orgID, statusID, ownerID, cycleUser.ID, "X", nil)
	yID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Y", &xID)
	zID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Z", &yID)

	// Close the cycle: X now reports to Z. Only direct self-reference is
	// blocked at the app layer (employees.ErrSelfManager) — a longer cycle
	// like this is a legally-insertable row shape today.
	if _, err := env.db.Exec(context.Background(),
		`UPDATE hrm_employees SET manager_id = $1 WHERE id = $2`, zID, xID); err != nil {
		t.Fatalf("close cycle: %v", err)
	}

	got := fetchScopedIDs(t, env, authz.ScopeTeam, orgID, cycleUser.ID, 5)
	if !containsExactly(got, xID, yID, zID) {
		t.Errorf("cycle with maxDepth=5: expected exactly the 3 cycle members [X,Y,Z], got %v (path guard did not bound the cycle correctly)", got)
	}
}

// TestIntegration_HRMScope_NoLinkedEmployee_FailsClosed proves that a caller
// with no hrm_employees.user_id link gets zero rows under ScopeOwn/ScopeTeam,
// not an error and not everything.
func TestIntegration_HRMScope_NoLinkedEmployee_FailsClosed(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	_ = seedEmployee(t, env, orgID, statusID, ownerID, "", "Unowned", nil)

	strangerEmail := uniqueEmail("scope-stranger")
	stranger, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: strangerEmail, Password: "StrangerPass123!"})
	if err != nil {
		t.Fatalf("signup stranger: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, stranger.ID) })

	if got := fetchScopedIDs(t, env, authz.ScopeOwn, orgID, stranger.ID, scope.DefaultMaxDepth); len(got) != 0 {
		t.Errorf("ScopeOwn for a user with no linked employee record should return zero rows, got %v", got)
	}
	if got := fetchScopedIDs(t, env, authz.ScopeTeam, orgID, stranger.ID, scope.DefaultMaxDepth); len(got) != 0 {
		t.Errorf("ScopeTeam for a user with no linked employee record should return zero rows, got %v", got)
	}
}

// TestIntegration_HRMScope_AuthorizeRecordAccess_TeamTier proves the
// single-record (path-param) authorization shape: a manager can access their
// direct report's record but not an unrelated employee's.
func TestIntegration_HRMScope_AuthorizeRecordAccess_TeamTier(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmail := uniqueEmail("scope-manager")
	manager, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: managerEmail, Password: "ManagerPass123!"})
	if err != nil {
		t.Fatalf("signup manager: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, manager.ID) })

	managerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, manager.ID, "Manager", nil)
	reportEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmpID)
	strangerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	resolver := scope.NewResolver(env.db)

	ok, err := resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeTeam, orgID, manager.ID, reportEmpID)
	if err != nil {
		t.Fatalf("AuthorizeRecordAccess (own report): %v", err)
	}
	if !ok {
		t.Error("expected manager to be authorized for their direct report's record")
	}

	ok, err = resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeTeam, orgID, manager.ID, strangerEmpID)
	if err != nil {
		t.Fatalf("AuthorizeRecordAccess (stranger): %v", err)
	}
	if ok {
		t.Error("SECURITY: manager must not be authorized for an unrelated employee's record")
	}
}

// TestIntegration_HRMScope_AuthorizeRecordAccess_OwnTier proves ScopeOwn only
// authorizes the caller's own record, never a colleague's, and that
// ScopeAll/ScopeNone short-circuit without a query.
func TestIntegration_HRMScope_AuthorizeRecordAccess_OwnTier(t *testing.T) {
	env := newTestEnv(t)
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	selfEmail := uniqueEmail("scope-self")
	self, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: selfEmail, Password: "SelfPass123!"})
	if err != nil {
		t.Fatalf("signup self: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, self.ID) })

	selfEmpID := seedEmployee(t, env, orgID, statusID, ownerID, self.ID, "Self", nil)
	colleagueEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Colleague", nil)

	resolver := scope.NewResolver(env.db)

	if ok, err := resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeOwn, orgID, self.ID, selfEmpID); err != nil || !ok {
		t.Errorf("expected self access to be authorized, got ok=%v err=%v", ok, err)
	}
	if ok, err := resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeOwn, orgID, self.ID, colleagueEmpID); err != nil || ok {
		t.Errorf("SECURITY: ScopeOwn must not authorize a colleague's record, got ok=%v err=%v", ok, err)
	}
	if ok, err := resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeAll, orgID, self.ID, colleagueEmpID); err != nil || !ok {
		t.Errorf("expected ScopeAll to authorize any record without a query, got ok=%v err=%v", ok, err)
	}
	if ok, err := resolver.AuthorizeRecordAccess(context.Background(), authz.ScopeNone, orgID, self.ID, selfEmpID); err != nil || ok {
		t.Errorf("expected ScopeNone to deny even the caller's own record, got ok=%v err=%v", ok, err)
	}
}
