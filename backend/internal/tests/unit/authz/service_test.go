// backend/internal/tests/unit/authz/service_test.go
// Full RBAC service unit tests — no DB, no Redis.
// authz.NewService takes a nil redis client; the service falls back to DB on every check.
package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/authz"
)

// ── In-memory stub repository ─────────────────────────────────────────────────

type stubAuthzRepo struct {
	memberships map[string]*authz.Membership           // key: userID+":"+orgID
	members     map[string]*authz.Membership           // key: orgID+":"+memberRef
	roles       map[string]*authz.Role                 // key: roleID or name
	permissions []*authz.Permission
	invitations map[string]*authz.OrganizationInvitation // key: tokenHash
	seq         int
}

func newStubAuthzRepo() *stubAuthzRepo {
	r := &stubAuthzRepo{
		memberships: map[string]*authz.Membership{},
		members:     map[string]*authz.Membership{},
		roles:       map[string]*authz.Role{},
		invitations: map[string]*authz.OrganizationInvitation{},
		permissions: []*authz.Permission{
			{ID: "perm_1", PublicID: "pub_1", KeyName: "crm.contacts.view", Resource: "crm.contacts", Action: "view", IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "perm_2", PublicID: "pub_2", KeyName: "crm.contacts.create", Resource: "crm.contacts", Action: "create", IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "perm_3", PublicID: "pub_3", KeyName: "tasks.view", Resource: "tasks", Action: "view", IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	// Seed system roles
	r.seedRole(&authz.Role{ID: "role_owner", Name: "owner", IsSystem: true, Permissions: []string{"crm.contacts.view", "crm.contacts.create", "tasks.view"}})
	r.seedRole(&authz.Role{ID: "role_admin", Name: "admin", IsSystem: true, Permissions: []string{"crm.contacts.view", "crm.contacts.create", "tasks.view"}})
	r.seedRole(&authz.Role{ID: "role_manager", Name: "manager", IsSystem: true, Permissions: []string{"crm.contacts.view", "tasks.view"}})
	r.seedRole(&authz.Role{ID: "role_member", Name: "member", IsSystem: true, Permissions: []string{"crm.contacts.view", "tasks.view"}})
	r.seedRole(&authz.Role{ID: "role_viewer", Name: "viewer", IsSystem: true, Permissions: []string{"crm.contacts.view"}})
	return r
}

func (r *stubAuthzRepo) seedRole(role *authz.Role) {
	r.roles[role.ID] = role
	r.roles[role.Name] = role
}

func (r *stubAuthzRepo) seedMembership(userID, orgID, roleKey string) *authz.Membership {
	role := r.roles[roleKey]
	var roleID *string
	if role != nil {
		roleID = &role.ID
	}
	m := &authz.Membership{
		ID:                "mem_" + userID + "_" + orgID,
		PublicID:          "pub_mem_" + userID,
		UserID:            userID,
		OrganizationID:    orgID,
		RoleID:            roleID,
		RoleKey:           roleKey,
		Status:            "active",
		InvitationStatus:  "accepted",
		JoinedAt:          time.Now(),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CustomPermissions: []string{},
		DeniedPermissions: []string{},
	}
	r.memberships[userID+":"+orgID] = m
	r.members[orgID+":"+m.ID] = m
	r.members[orgID+":"+userID] = m
	return m
}

func (r *stubAuthzRepo) nextSeq() string {
	r.seq++
	n := r.seq
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// ── Repository interface implementation ──────────────────────────────────────

func (r *stubAuthzRepo) GetUserPermissions(_ context.Context, userID, organizationID string) ([]*authz.Permission, error) {
	m := r.memberships[userID+":"+organizationID]
	if m == nil {
		return []*authz.Permission{}, nil
	}
	role := r.roles[m.RoleKey]
	if role == nil {
		return []*authz.Permission{}, nil
	}
	permSet := map[string]bool{}
	for _, k := range role.Permissions {
		permSet[k] = true
	}
	for _, k := range m.CustomPermissions {
		permSet[k] = true
	}
	for _, k := range m.DeniedPermissions {
		delete(permSet, k)
	}
	var out []*authz.Permission
	for _, p := range r.permissions {
		if permSet[p.Key()] {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *stubAuthzRepo) GetMembership(_ context.Context, userID, orgID string) (*authz.Membership, error) {
	return r.memberships[userID+":"+orgID], nil
}

func (r *stubAuthzRepo) GetMemberByRef(_ context.Context, orgID, ref string) (*authz.Membership, error) {
	return r.members[orgID+":"+ref], nil
}

func (r *stubAuthzRepo) GetMemberWithUserByRef(_ context.Context, _, _ string) (*authz.MemberWithUser, error) {
	return nil, nil
}

func (r *stubAuthzRepo) ListMembers(_ context.Context, orgID string) ([]*authz.MemberWithUser, error) {
	seen := map[string]bool{}
	var out []*authz.MemberWithUser
	for _, m := range r.memberships {
		if m.OrganizationID != orgID || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, &authz.MemberWithUser{
			MembershipID: m.ID,
			UserID:       m.UserID,
			Role:         m.RoleKey,
			Status:       m.Status,
			JoinedAt:     m.JoinedAt,
		})
	}
	return out, nil
}

func (r *stubAuthzRepo) CountActiveMembers(_ context.Context, _ string) (int, error) {
	return 1, nil
}

func (r *stubAuthzRepo) GetOrganizationMaxSeats(_ context.Context, _ string) (*int, error) {
	val := 100
	return &val, nil
}

func (r *stubAuthzRepo) SetUserPasswordHash(_ context.Context, _, _ string) error {
	return nil
}

func (r *stubAuthzRepo) GetRoleByName(_ context.Context, name string) (*authz.Role, error) {
	return r.roles[name], nil
}

func (r *stubAuthzRepo) GetRoleByID(_ context.Context, id string) (*authz.Role, error) {
	return r.roles[id], nil
}

func (r *stubAuthzRepo) GetRoleByRef(_ context.Context, _, ref string) (*authz.Role, error) {
	return r.roles[ref], nil
}

func (r *stubAuthzRepo) UpdateMembershipRole(_ context.Context, userID, orgID, roleID string) error {
	m := r.memberships[userID+":"+orgID]
	if m == nil {
		return authz.ErrMemberNotFound
	}
	role := r.roles[roleID]
	if role == nil {
		return authz.ErrRoleNotFound
	}
	m.RoleID = &roleID
	m.RoleKey = role.Name
	return nil
}

func (r *stubAuthzRepo) UpdateMembership(_ context.Context, _, _ string, _ *authz.Role, _ authz.UpdateMemberRequest) (*authz.Membership, error) {
	return nil, nil
}

func (r *stubAuthzRepo) UpdateMemberPermissions(_ context.Context, orgID, ref string, custom, denied []string) (*authz.Membership, error) {
	m := r.members[orgID+":"+ref]
	if m == nil {
		return nil, authz.ErrMemberNotFound
	}
	m.CustomPermissions = custom
	m.DeniedPermissions = denied
	return m, nil
}

func (r *stubAuthzRepo) CreateMembership(_ context.Context, m *authz.Membership) error {
	seq := r.nextSeq()
	m.ID = "mem_new_" + seq
	m.PublicID = "pub_" + m.ID
	m.JoinedAt = time.Now()
	r.memberships[m.UserID+":"+m.OrganizationID] = m
	r.members[m.OrganizationID+":"+m.ID] = m
	r.members[m.OrganizationID+":"+m.UserID] = m
	return nil
}

// CreateMembershipTx uses the real pgx.Tx signature from the interface.
func (r *stubAuthzRepo) CreateMembershipTx(_ context.Context, _ pgx.Tx, m *authz.Membership) error {
	return r.CreateMembership(context.Background(), m)
}

func (r *stubAuthzRepo) ListRoles(_ context.Context) ([]*authz.Role, error) {
	seen := map[string]bool{}
	var out []*authz.Role
	for _, role := range r.roles {
		if !seen[role.ID] {
			seen[role.ID] = true
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *stubAuthzRepo) ListRolesForOrg(ctx context.Context, _ string) ([]*authz.Role, error) {
	return r.ListRoles(ctx)
}

func (r *stubAuthzRepo) ListRolesWithPermissions(ctx context.Context) ([]*authz.RoleWithPermissions, error) {
	roles, _ := r.ListRoles(ctx)
	out := make([]*authz.RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		out = append(out, &authz.RoleWithPermissions{Role: role, Permissions: []*authz.Permission{}})
	}
	return out, nil
}

func (r *stubAuthzRepo) ListPermissions(_ context.Context) ([]*authz.Permission, error) {
	return r.permissions, nil
}

func (r *stubAuthzRepo) CreateRole(_ context.Context, orgID string, req authz.CreateRoleRequest) (*authz.Role, error) {
	seq := r.nextSeq()
	role := &authz.Role{
		ID:          "role_custom_" + seq,
		PublicID:    "pub_role_" + seq,
		Name:        req.Name,
		Permissions: req.PermissionKeys,
		IsCustom:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if orgID != "" {
		role.OrgID = &orgID
	}
	r.seedRole(role)
	return role, nil
}

func (r *stubAuthzRepo) UpdateRole(_ context.Context, _, ref string, req authz.UpdateRoleRequest) (*authz.Role, error) {
	role := r.roles[ref]
	if role == nil {
		return nil, authz.ErrRoleNotFound
	}
	if role.IsSystem {
		return nil, authz.ErrCannotModifySystemRole
	}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.PermissionKeys != nil {
		role.Permissions = req.PermissionKeys
	}
	return role, nil
}

func (r *stubAuthzRepo) UpdateRolePermissions(_ context.Context, _, ref string, keys []string) (*authz.Role, error) {
	role := r.roles[ref]
	if role == nil {
		return nil, authz.ErrRoleNotFound
	}
	role.Permissions = keys
	return role, nil
}

func (r *stubAuthzRepo) DeleteRole(_ context.Context, _, ref string) error {
	role := r.roles[ref]
	if role == nil {
		return authz.ErrRoleNotFound
	}
	if role.IsSystem {
		return authz.ErrCannotModifySystemRole
	}
	delete(r.roles, ref)
	delete(r.roles, role.ID)
	return nil
}

func (r *stubAuthzRepo) CloneRole(ctx context.Context, _, ref, name, desc string) (*authz.Role, error) {
	src := r.roles[ref]
	if src == nil {
		return nil, authz.ErrRoleNotFound
	}
	return r.CreateRole(ctx, "", authz.CreateRoleRequest{
		Name:           name,
		Description:    desc,
		PermissionKeys: src.Permissions,
	})
}

func (r *stubAuthzRepo) CreateInvitation(_ context.Context, inv *authz.OrganizationInvitation) error {
	seq := r.nextSeq()
	inv.ID = "inv_" + seq
	inv.PublicID = "pub_inv_" + seq
	r.invitations[inv.TokenHash] = inv
	return nil
}

func (r *stubAuthzRepo) GetInvitationByRef(_ context.Context, _, ref string) (*authz.OrganizationInvitation, error) {
	for _, inv := range r.invitations {
		if inv.ID == ref || inv.PublicID == ref {
			return inv, nil
		}
	}
	return nil, nil
}

func (r *stubAuthzRepo) GetInvitationByTokenHash(_ context.Context, _, hash string) (*authz.OrganizationInvitation, error) {
	return r.invitations[hash], nil
}

func (r *stubAuthzRepo) ResendInvitation(_ context.Context, _, ref, newHash string, exp time.Time) (*authz.OrganizationInvitation, error) {
	inv, _ := r.GetInvitationByRef(context.Background(), "", ref)
	if inv == nil {
		return nil, authz.ErrInvitationNotFound
	}
	delete(r.invitations, inv.TokenHash)
	inv.TokenHash = newHash
	inv.ExpiresAt = exp
	r.invitations[newHash] = inv
	return inv, nil
}

func (r *stubAuthzRepo) RevokeInvitation(_ context.Context, _, ref string) error {
	inv, _ := r.GetInvitationByRef(context.Background(), "", ref)
	if inv == nil {
		return authz.ErrInvitationNotFound
	}
	delete(r.invitations, inv.TokenHash)
	return nil
}

func (r *stubAuthzRepo) AcceptInvitation(_ context.Context, _, hash, userID string) (*authz.Membership, *authz.OrganizationInvitation, error) {
	inv := r.invitations[hash]
	if inv == nil {
		return nil, nil, authz.ErrInvitationNotFound
	}
	return nil, inv, nil
}

// ── Service factory ──────────────────────────────────────────────────────────
// Pass nil redis — service falls back to DB on every permission check.

type mockAudit struct{}

func (m *mockAudit) Log(ctx context.Context, event audit.EventType, userID, businessID, ip, ua string, metadata any) {
}

type mockRevoker struct{}

func (m *mockRevoker) RevokeAllUserSessions(ctx context.Context, userID string) error {
	return nil
}

func newSvc(repo *stubAuthzRepo) authz.Service {
	return authz.NewService(repo, nil, &mockAudit{}, &mockRevoker{})
}

// ── Can() ────────────────────────────────────────────────────────────────────

func TestCan_UserHasPermission(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-1", "member")
	svc := newSvc(repo)

	allowed, err := svc.Can(context.Background(), "user-1", "org-1", "crm.contacts", "view")
	if err != nil {
		t.Fatalf("Can() error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true for member with crm.contacts.view")
	}
}

func TestCan_UserLacksPermission(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-2", "org-1", "viewer")
	svc := newSvc(repo)

	allowed, err := svc.Can(context.Background(), "user-2", "org-1", "crm.contacts", "create")
	if err != nil {
		t.Fatalf("Can() error: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false: viewer does not have crm.contacts.create")
	}
}

func TestCan_NoMembership_ReturnsFalse(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	allowed, err := svc.Can(context.Background(), "user-stranger", "org-1", "crm.contacts", "view")
	if err != nil {
		t.Fatalf("Can() error: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false for user with no membership")
	}
}

func TestCan_CrossOrgDoesNotGrantAccess(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-A", "admin")
	svc := newSvc(repo)

	allowed, err := svc.Can(context.Background(), "user-1", "org-B", "crm.contacts", "view")
	if err != nil {
		t.Fatalf("Can() error: %v", err)
	}
	if allowed {
		t.Error("SECURITY: permissions in org-A must not grant access in org-B")
	}
}

// ── GetMembership ─────────────────────────────────────────────────────────────

func TestGetMembership_Exists(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-1", "member")
	svc := newSvc(repo)

	m, err := svc.GetMembership(context.Background(), "user-1", "org-1")
	if err != nil {
		t.Fatalf("GetMembership() error: %v", err)
	}
	if m == nil {
		t.Fatal("expected membership, got nil")
	}
	if m.UserID != "user-1" {
		t.Errorf("expected UserID=user-1, got %s", m.UserID)
	}
}

func TestGetMembership_NotMember(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	m, err := svc.GetMembership(context.Background(), "user-stranger", "org-1")
	if err != nil {
		t.Fatalf("GetMembership() error: %v", err)
	}
	if m != nil {
		t.Error("expected nil membership for non-member")
	}
}

// ── ListMembers ───────────────────────────────────────────────────────────────

func TestListMembers_ReturnsOrgMembers(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-1", "admin")
	repo.seedMembership("user-2", "org-1", "member")
	repo.seedMembership("user-3", "org-2", "member")
	svc := newSvc(repo)

	members, err := svc.ListMembers(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("ListMembers() error: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members in org-1, got %d", len(members))
	}
}

func TestListMembers_EmptyOrgReturnsSliceNotNil(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	members, err := svc.ListMembers(context.Background(), "org-empty")
	if err != nil {
		t.Fatalf("ListMembers() error: %v", err)
	}
	if members == nil {
		t.Error("expected empty slice, not nil")
	}
}

// ── AssignRole ────────────────────────────────────────────────────────────────

func TestAssignRole_Success(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("caller", "org-1", "admin")
	repo.seedMembership("target", "org-1", "member")
	svc := newSvc(repo)

	if err := svc.AssignRole(context.Background(), "caller", "target", "org-1", "admin"); err != nil {
		t.Fatalf("AssignRole() error: %v", err)
	}
}

func TestAssignRole_CannotAssignOwner(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("caller", "org-1", "admin")
	repo.seedMembership("target", "org-1", "member")
	svc := newSvc(repo)

	err := svc.AssignRole(context.Background(), "caller", "target", "org-1", "owner")
	if !errors.Is(err, authz.ErrCannotAssignOwner) {
		t.Fatalf("expected ErrCannotAssignOwner, got %v", err)
	}
}

func TestAssignRole_CannotChangeOwnRole(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-1", "admin")
	svc := newSvc(repo)

	err := svc.AssignRole(context.Background(), "user-1", "user-1", "org-1", "member")
	if !errors.Is(err, authz.ErrCannotChangeOwnRole) {
		t.Fatalf("expected ErrCannotChangeOwnRole, got %v", err)
	}
}

func TestAssignRole_RoleNotFound(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("caller", "org-1", "admin")
	repo.seedMembership("target", "org-1", "member")
	svc := newSvc(repo)

	err := svc.AssignRole(context.Background(), "caller", "target", "org-1", "nonexistent-role")
	if !errors.Is(err, authz.ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestAssignRole_TargetNotMember(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("caller", "org-1", "admin")
	svc := newSvc(repo)

	err := svc.AssignRole(context.Background(), "caller", "non-member", "org-1", "member")
	if !errors.Is(err, authz.ErrMemberNotFound) {
		t.Fatalf("expected ErrMemberNotFound, got %v", err)
	}
}

// ── CreateRole ────────────────────────────────────────────────────────────────

func TestCreateRole_Success(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	role, err := svc.CreateRole(context.Background(), "org-1", authz.CreateRoleRequest{
		Name:           "Custom Support",
		PermissionKeys: []string{"crm.contacts.view"},
	})
	if err != nil {
		t.Fatalf("CreateRole() error: %v", err)
	}
	if role.Name != "Custom Support" {
		t.Errorf("expected name 'Custom Support', got %q", role.Name)
	}
}

func TestCreateRole_InvalidName_TooShort(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	_, err := svc.CreateRole(context.Background(), "org-1", authz.CreateRoleRequest{
		Name:           "x",
		PermissionKeys: []string{"crm.contacts.view"},
	})
	if !errors.Is(err, authz.ErrInvalidRoleName) {
		t.Fatalf("expected ErrInvalidRoleName for single-char name, got %v", err)
	}
}

func TestCreateRole_ReservedNames_Rejected(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	reserved := []string{"owner", "admin", "manager", "member", "viewer"}
	for _, name := range reserved {
		_, err := svc.CreateRole(context.Background(), "org-1", authz.CreateRoleRequest{
			Name:           name,
			PermissionKeys: []string{"crm.contacts.view"},
		})
		if !errors.Is(err, authz.ErrReservedRoleName) {
			t.Errorf("name %q: expected ErrReservedRoleName, got %v", name, err)
		}
	}
}

func TestCreateRole_InvalidPermissionKey_Rejected(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	_, err := svc.CreateRole(context.Background(), "org-1", authz.CreateRoleRequest{
		Name:           "My Custom Role",
		PermissionKeys: []string{"nonexistent.key"},
	})
	if err == nil {
		t.Fatal("expected error for invalid permission key, got nil")
	}
}

// ── DeleteRole ────────────────────────────────────────────────────────────────

func TestDeleteRole_SystemRoleCannotBeDeleted(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	err := svc.DeleteRole(context.Background(), "org-1", "member")
	if !errors.Is(err, authz.ErrCannotModifySystemRole) {
		t.Fatalf("expected ErrCannotModifySystemRole, got %v", err)
	}
}

func TestDeleteRole_CustomRoleCanBeDeleted(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	role, _ := svc.CreateRole(context.Background(), "org-1", authz.CreateRoleRequest{
		Name:           "Deletable Role",
		PermissionKeys: []string{"crm.contacts.view"},
	})
	if err := svc.DeleteRole(context.Background(), "org-1", role.ID); err != nil {
		t.Fatalf("DeleteRole() error: %v", err)
	}
}

// ── ListPermissions ───────────────────────────────────────────────────────────

func TestListPermissions_ReturnsAll(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	perms, err := svc.ListPermissions(context.Background())
	if err != nil {
		t.Fatalf("ListPermissions() error: %v", err)
	}
	if len(perms) == 0 {
		t.Error("expected at least one permission")
	}
}

// ── InviteMember ─────────────────────────────────────────────────────────────

func TestInviteMember_Success(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	resp, err := svc.InviteMember(context.Background(), "caller-1", "org-1", authz.InviteMemberRequest{
		Email: "invitee@example.com",
		Role:  "member",
	})
	if err != nil {
		t.Fatalf("InviteMember() error: %v", err)
	}
	if resp.Invitation == nil {
		t.Fatal("expected invitation in response")
	}
	if resp.Token == "" {
		t.Error("expected non-empty invitation token")
	}
}

func TestInviteMember_InvalidEmail_Rejected(t *testing.T) {
	svc := newSvc(newStubAuthzRepo())
	_, err := svc.InviteMember(context.Background(), "caller-1", "org-1", authz.InviteMemberRequest{
		Email: "",
		Role:  "member",
	})
	if !errors.Is(err, authz.ErrInvalidInvitationEmail) {
		t.Fatalf("expected ErrInvalidInvitationEmail, got %v", err)
	}
}

// ── Tenant isolation ─────────────────────────────────────────────────────────

func TestCan_OrgAPermissionsDoNotLeakToOrgB(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-A", "admin")
	svc := newSvc(repo)

	allowedA, _ := svc.Can(context.Background(), "user-1", "org-A", "crm.contacts", "create")
	if !allowedA {
		t.Fatal("precondition: user-1 should have create permission in org-A")
	}

	allowedB, _ := svc.Can(context.Background(), "user-1", "org-B", "crm.contacts", "create")
	if allowedB {
		t.Error("SECURITY VIOLATION: org-A permissions leaked to org-B")
	}
}

func TestListMembers_DoesNotCrossOrgs(t *testing.T) {
	repo := newStubAuthzRepo()
	repo.seedMembership("user-1", "org-A", "member")
	repo.seedMembership("user-2", "org-B", "member")
	svc := newSvc(repo)

	membersA, _ := svc.ListMembers(context.Background(), "org-A")
	for _, m := range membersA {
		if m.UserID == "user-2" {
			t.Error("SECURITY: org-B member user-2 appeared in org-A list")
		}
	}
}
