// backend/internal/tests/integration/authz_role_directory_test.go
// Proves authz.RoleExists / authz.UserRoleName against real membership data —
// specifically the OR-join from GetUserPermissions (repository.go:127) that
// both new queries must reproduce. This is exactly the class of bug a stub
// repo cannot catch: the join has to be tested against rows a stub wouldn't
// think to construct (role_id NULL but role_key set, two roles matching the
// same membership row via the OR). Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
)

// insertRawMembership bypasses authz.Service entirely so tests can construct
// row shapes the service layer would never produce on its own (NULL role_id,
// pending invitations, etc.) — exactly the shapes the OR-join has to handle.
func insertRawMembership(t *testing.T, env *testEnv, orgID, userID string, roleID *string, roleKey, status, invitationStatus string) {
	t.Helper()
	_, err := env.db.Exec(context.Background(),
		`INSERT INTO organization_members (org_id, user_id, role_id, role_key, status, invitation_status)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		orgID, userID, roleID, roleKey, status, invitationStatus,
	)
	if err != nil {
		t.Fatalf("insert raw membership: %v", err)
	}
}

// TestIntegration_AuthzRoleDirectory_UserRoleName_NullRoleID proves a
// membership whose role_id is NULL (role deleted via ON DELETE SET NULL, or
// the row was inserted directly by seed SQL) is still found via role_key
// matching a global system role by name — the OR branch of the join.
func TestIntegration_AuthzRoleDirectory_UserRoleName_NullRoleID(t *testing.T) {
	env := newTestEnv(t)
	orgID, _, _ := seedScopeTestOrg(t, env)

	email := uniqueEmail("roledir-nullrole")
	u, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: email, Password: "NullRolePass123!"})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })

	insertRawMembership(t, env, orgID, u.ID, nil, "member", "active", "accepted")

	name, err := env.authzSvc.UserRoleName(context.Background(), orgID, u.ID)
	if err != nil {
		t.Fatalf("UserRoleName: %v", err)
	}
	if name != "member" {
		t.Errorf("expected 'member' via role_key OR-join with role_id NULL, got %q", name)
	}
}

// TestIntegration_AuthzRoleDirectory_UserRoleName_ExcludesInactiveAndPending
// proves the liveness predicates (status='active', invitation_status='accepted')
// are enforced, not just documented.
func TestIntegration_AuthzRoleDirectory_UserRoleName_ExcludesInactiveAndPending(t *testing.T) {
	env := newTestEnv(t)
	orgID, _, _ := seedScopeTestOrg(t, env)

	inactiveEmail := uniqueEmail("roledir-inactive")
	inactiveUser, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: inactiveEmail, Password: "InactivePass123!"})
	if err != nil {
		t.Fatalf("signup inactive: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, inactiveUser.ID) })
	insertRawMembership(t, env, orgID, inactiveUser.ID, nil, "member", "inactive", "accepted")

	pendingEmail := uniqueEmail("roledir-pending")
	pendingUser, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: pendingEmail, Password: "PendingPass123!"})
	if err != nil {
		t.Fatalf("signup pending: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, pendingUser.ID) })
	insertRawMembership(t, env, orgID, pendingUser.ID, nil, "member", "active", "pending")

	if name, err := env.authzSvc.UserRoleName(context.Background(), orgID, inactiveUser.ID); err != nil || name != "" {
		t.Errorf("inactive membership: expected \"\" (no error), got name=%q err=%v", name, err)
	}
	if name, err := env.authzSvc.UserRoleName(context.Background(), orgID, pendingUser.ID); err != nil || name != "" {
		t.Errorf("pending invitation: expected \"\" (no error), got name=%q err=%v", name, err)
	}
}

// TestIntegration_AuthzRoleDirectory_UserRoleName_PrefersOrgOwnedRole proves
// the ORDER BY CASE tie-break: when a membership row matches two roles via the
// OR-join (its own org-owned custom role by role_id, AND a same-named global
// system role by role_key), UserRoleName must return the org-owned one.
func TestIntegration_AuthzRoleDirectory_UserRoleName_PrefersOrgOwnedRole(t *testing.T) {
	env := newTestEnv(t)
	orgID, _, _ := seedScopeTestOrg(t, env)

	email := uniqueEmail("roledir-preference")
	u, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: email, Password: "PreferPass123!"})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })

	// The service's CreateRole rejects names that collide with a system role
	// ("reserved role name") — a real and correct guard, so this fixture is
	// inserted directly via SQL instead. That is itself the realistic case
	// this test protects: UserRoleName's doc comment notes the OR-join must
	// handle "the row was inserted directly by seed SQL". The custom role's
	// name is deliberately NOT 'owner' — the two joined rows must differ in
	// name for the assertion below to distinguish which one won.
	const customRoleName = "org-owner-shadow"
	var customRoleID string
	err = env.db.QueryRow(context.Background(),
		`INSERT INTO roles (org_id, name, permissions, is_system, is_custom) VALUES ($1, $2, ARRAY[]::TEXT[], FALSE, TRUE) RETURNING id`,
		orgID, customRoleName,
	).Scan(&customRoleID)
	if err != nil {
		t.Fatalf("insert shadow custom role: %v", err)
	}

	// role_id points at the org-owned custom role (matches the join's first
	// branch unconditionally); role_key is left at the global system role's
	// name so the join's second branch ALSO matches the global 'owner' row —
	// two rows for one membership, which ORDER BY must resolve.
	insertRawMembership(t, env, orgID, u.ID, &customRoleID, "owner", "active", "accepted")

	name, err := env.authzSvc.UserRoleName(context.Background(), orgID, u.ID)
	if err != nil {
		t.Fatalf("UserRoleName: %v", err)
	}
	if name != customRoleName {
		t.Errorf("expected the org-owned role %q to win over the global 'owner' role, got %q", customRoleName, name)
	}
}

// TestIntegration_AuthzRoleDirectory_UserRoleName_CrossOrgIsolation proves a
// membership in org A is invisible when queried against org B.
func TestIntegration_AuthzRoleDirectory_UserRoleName_CrossOrgIsolation(t *testing.T) {
	env := newTestEnv(t)
	orgAID, _, _ := seedScopeTestOrg(t, env)
	orgBID, _, _ := seedScopeTestOrg(t, env)

	email := uniqueEmail("roledir-crossorg")
	u, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{Email: email, Password: "CrossOrgPass123!"})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })
	insertRawMembership(t, env, orgAID, u.ID, nil, "member", "active", "accepted")

	if name, err := env.authzSvc.UserRoleName(context.Background(), orgAID, u.ID); err != nil || name != "member" {
		t.Errorf("expected 'member' in org A, got name=%q err=%v", name, err)
	}
	if name, err := env.authzSvc.UserRoleName(context.Background(), orgBID, u.ID); err != nil || name != "" {
		t.Errorf("SECURITY: membership in org A must not resolve when queried against org B, got name=%q err=%v", name, err)
	}
}

// TestIntegration_AuthzRoleDirectory_RoleExists proves RoleExists sees both
// global system roles (org_id IS NULL) and org-owned custom roles, is
// case-insensitive, and does not leak a role scoped to a different org.
func TestIntegration_AuthzRoleDirectory_RoleExists(t *testing.T) {
	env := newTestEnv(t)
	orgAID, _, _ := seedScopeTestOrg(t, env)
	orgBID, _, _ := seedScopeTestOrg(t, env)

	if ok, err := env.authzSvc.RoleExists(context.Background(), orgAID, "OWNER"); err != nil || !ok {
		t.Errorf("expected global system role 'owner' to exist (case-insensitive), got ok=%v err=%v", ok, err)
	}

	customRole, err := env.authzSvc.CreateRole(context.Background(), orgAID, authz.CreateRoleRequest{
		Name: "Payroll Approver", Description: "custom role for RoleExists test",
		PermissionKeys: []string{"hrm.employees.view"},
	})
	if err != nil {
		t.Fatalf("create custom role: %v", err)
	}

	if ok, err := env.authzSvc.RoleExists(context.Background(), orgAID, "payroll approver"); err != nil || !ok {
		t.Errorf("expected org-owned custom role to exist (case-insensitive), got ok=%v err=%v", ok, err)
	}
	if ok, err := env.authzSvc.RoleExists(context.Background(), orgBID, customRole.Name); err != nil || ok {
		t.Errorf("SECURITY: a custom role scoped to org A must not exist when checked against org B, got ok=%v err=%v", ok, err)
	}
	if ok, err := env.authzSvc.RoleExists(context.Background(), orgAID, "definitely-not-a-real-role"); err != nil || ok {
		t.Errorf("expected a nonexistent role name to return false, got ok=%v err=%v", ok, err)
	}
}
