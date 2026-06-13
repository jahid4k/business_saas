// backend/internal/authz/service.go
package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mridha/businesssaas/pkg/token"
)

const permCacheTTL = 5 * time.Minute
const invitationTTL = 7 * 24 * time.Hour

var roleNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{1,49}$`)

type Service interface {
	Can(ctx context.Context, userID, organizationID, resource, action string) (bool, error)
	GetMembership(ctx context.Context, userID, organizationID string) (*Membership, error)
	MyMembership(ctx context.Context, userID, organizationID string) (*MyMembershipResponse, error)
	ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error)
	GetMember(ctx context.Context, organizationID, memberRef string) (*MemberWithUser, error)
	AssignRole(ctx context.Context, callerID, targetUserID, organizationID, roleName string) error
	UpdateMember(ctx context.Context, callerID, organizationID, memberRef string, req UpdateMemberRequest) (*Membership, error)
	InviteMember(ctx context.Context, callerID, organizationID string, req InviteMemberRequest) (*InviteMemberResponse, error)
	AcceptInvitation(ctx context.Context, userID, organizationID, rawToken string) (*Membership, *OrganizationInvitation, error)
	ResendInvitation(ctx context.Context, organizationID, invitationRef string) (*ResendInvitationResponse, error)
	RevokeInvitation(ctx context.Context, organizationID, invitationRef string) error
	ListRoles(ctx context.Context) ([]*RoleWithPermissions, error)
	ListRolesForOrg(ctx context.Context, organizationID string) ([]*RoleWithPermissions, error)
	CreateRole(ctx context.Context, organizationID string, req CreateRoleRequest) (*Role, error)
	GetRole(ctx context.Context, organizationID, roleRef string) (*RoleWithPermissions, error)
	UpdateRole(ctx context.Context, organizationID, roleRef string, req UpdateRoleRequest) (*Role, error)
	DeleteRole(ctx context.Context, organizationID, roleRef string) error
	UpdateRolePermissions(ctx context.Context, organizationID, roleRef string, req UpdateRolePermissionsRequest) (*Role, error)
	CloneRole(ctx context.Context, organizationID, roleRef string, req CloneRoleRequest) (*Role, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
	GetMemberPermissions(ctx context.Context, organizationID, memberRef string) (*MemberPermissionsResponse, error)
	UpdateMemberPermissions(ctx context.Context, organizationID, memberRef string, req UpdateMemberPermissionsRequest) (*MemberPermissionsResponse, error)
	CheckMember(ctx context.Context, organizationID string, req CheckMemberPermissionRequest) (*CheckMemberPermissionResponse, error)
	PermissionMatrix(ctx context.Context, organizationID string) (*PermissionMatrixResponse, error)
}

type serviceImpl struct {
	repo  Repository
	redis *redis.Client
}

func NewService(repo Repository, redisClient *redis.Client) Service {
	return &serviceImpl{repo: repo, redis: redisClient}
}

// Can checks whether the user holds the given resource.action permission in the organization.
// FIX: uses SISMEMBER (O(1)) instead of SMEMBERS + linear scan.
// The old implementation fetched every permission for the user on every auth check.
func (s *serviceImpl) Can(ctx context.Context, userID, organizationID, resource, action string) (bool, error) {
	permKey := strings.ToLower(strings.TrimSpace(resource + "." + action))
	cacheKey := permCacheKey(userID, organizationID)

	exists, err := s.redis.Exists(ctx, cacheKey).Result()
	if err != nil {
		slog.Warn("authz: redis unavailable, falling back to DB", slog.String("key", cacheKey), slog.Any("error", err))
	} else if exists > 0 {
		isMember, simErr := s.redis.SIsMember(ctx, cacheKey, permKey).Result()
		if simErr == nil {
			return isMember, nil
		}
		slog.Warn("authz: redis unavailable, falling back to DB", slog.String("key", cacheKey), slog.Any("error", simErr))
	}

	perms, err := s.repo.GetUserPermissions(ctx, userID, organizationID)
	if err != nil {
		return false, fmt.Errorf("authz: Can: %w", err)
	}
	if len(perms) > 0 {
		members := make([]any, 0, len(perms))
		for _, p := range perms {
			members = append(members, p.Key())
		}
		pipe := s.redis.Pipeline()
		pipe.SAdd(ctx, cacheKey, members...)
		pipe.Expire(ctx, cacheKey, permCacheTTL)
		if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
			slog.Warn("authz: failed to populate permission cache", slog.String("key", cacheKey), slog.Any("error", pipeErr))
		}
	}
	for _, p := range perms {
		if p.Key() == permKey {
			return true, nil
		}
	}
	return false, nil
}

func (s *serviceImpl) GetMembership(ctx context.Context, userID, organizationID string) (*Membership, error) {
	m, err := s.repo.GetMembership(ctx, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: GetMembership: %w", err)
	}
	return m, nil
}

func (s *serviceImpl) MyMembership(ctx context.Context, userID, organizationID string) (*MyMembershipResponse, error) {
	membership, err := s.repo.GetMembership(ctx, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: MyMembership: get membership: %w", err)
	}
	if membership == nil {
		return nil, ErrMemberNotFound
	}
	roleName := membership.RoleKey
	if membership.RoleID != nil {
		role, err := s.repo.GetRoleByID(ctx, *membership.RoleID)
		if err != nil {
			return nil, fmt.Errorf("authz: MyMembership: get role: %w", err)
		}
		if role != nil {
			roleName = role.Name
		}
	}
	perms, err := s.repo.GetUserPermissions(ctx, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: MyMembership: get permissions: %w", err)
	}
	permKeys := permissionsToKeys(perms)
	return &MyMembershipResponse{MembershipID: membership.ID, OrganizationID: organizationID, Role: roleName, Permissions: permKeys, JoinedAt: membership.JoinedAt}, nil
}

func (s *serviceImpl) ListMembers(ctx context.Context, organizationID string) ([]*MemberWithUser, error) {
	members, err := s.repo.ListMembers(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListMembers: %w", err)
	}
	if members == nil {
		members = []*MemberWithUser{}
	}
	return members, nil
}

func (s *serviceImpl) GetMember(ctx context.Context, organizationID, memberRef string) (*MemberWithUser, error) {
	m, err := s.repo.GetMemberWithUserByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, fmt.Errorf("authz: GetMember: %w", err)
	}
	if m == nil {
		return nil, ErrMemberNotFound
	}
	return m, nil
}

func (s *serviceImpl) AssignRole(ctx context.Context, callerID, targetUserID, organizationID, roleName string) error {
	if strings.EqualFold(roleName, RoleOwner) {
		return ErrCannotAssignOwner
	}
	if callerID == targetUserID {
		return ErrCannotChangeOwnRole
	}
	role, err := s.repo.GetRoleByRef(ctx, organizationID, roleName)
	if err != nil {
		return fmt.Errorf("authz: AssignRole: get role: %w", err)
	}
	if role == nil {
		return ErrRoleNotFound
	}
	membership, err := s.repo.GetMembership(ctx, targetUserID, organizationID)
	if err != nil {
		return fmt.Errorf("authz: AssignRole: get membership: %w", err)
	}
	if membership == nil {
		return ErrMemberNotFound
	}
	if err := s.repo.UpdateMembershipRole(ctx, targetUserID, organizationID, role.ID); err != nil {
		return fmt.Errorf("authz: AssignRole: update: %w", err)
	}
	s.invalidateUser(ctx, targetUserID, organizationID)
	return nil
}

func (s *serviceImpl) UpdateMember(ctx context.Context, callerID, organizationID, memberRef string, req UpdateMemberRequest) (*Membership, error) {
	member, err := s.repo.GetMemberByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateMember: get member: %w", err)
	}
	if member == nil {
		return nil, ErrMemberNotFound
	}
	if callerID == member.UserID && (req.Role != "" || req.RoleID != "" || req.Status != "") {
		return nil, ErrCannotChangeOwnRole
	}
	if member.RoleKey == RoleOwner && (req.Role != "" || req.RoleID != "" || strings.EqualFold(req.Status, MemberStatusInactive) || strings.EqualFold(req.Status, MemberStatusSuspended)) {
		return nil, ErrCannotModifyOwner
	}
	if req.Status != "" && !validMemberStatus(req.Status) {
		return nil, ErrInvalidMemberStatus
	}
	var role *Role
	if req.Role != "" || req.RoleID != "" {
		roleRef := req.RoleID
		if roleRef == "" {
			roleRef = req.Role
		}
		if strings.EqualFold(roleRef, RoleOwner) {
			return nil, ErrCannotAssignOwner
		}
		role, err = s.repo.GetRoleByRef(ctx, organizationID, roleRef)
		if err != nil {
			return nil, fmt.Errorf("authz: UpdateMember: get role: %w", err)
		}
		if role == nil {
			return nil, ErrRoleNotFound
		}
	}
	updated, err := s.repo.UpdateMembership(ctx, organizationID, memberRef, role, req)
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateMember: %w", err)
	}
	s.invalidateUser(ctx, member.UserID, organizationID)
	return updated, nil
}

func (s *serviceImpl) InviteMember(ctx context.Context, callerID, organizationID string, req InviteMemberRequest) (*InviteMemberResponse, error) {
	email := normaliseEmail(req.Email)
	if email == "" {
		return nil, ErrInvalidInvitationEmail
	}
	role, err := s.resolveRole(ctx, organizationID, req.RoleID, req.Role)
	if err != nil {
		return nil, err
	}
	rawTok, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("authz: InviteMember: generate token: %w", err)
	}
	inv := &OrganizationInvitation{
		OrganizationID: organizationID, Email: email,
		RoleID: func() *string {
			if role != nil {
				return &role.ID
			}
			return nil
		}(),
		RoleKey: func() string {
			if role != nil {
				return role.Name
			}
			return RoleMember
		}(),
		Title:             strings.TrimSpace(req.Title),
		Department:        strings.TrimSpace(req.Department),
		CustomPermissions: req.CustomPermissions,
		DeniedPermissions: req.DeniedPermissions,
		TokenHash:         tokenHash,
		InvitedBy:         &callerID,
		ExpiresAt:         time.Now().Add(invitationTTL),
	}
	if inv.CustomPermissions == nil {
		inv.CustomPermissions = []string{}
	}
	if inv.DeniedPermissions == nil {
		inv.DeniedPermissions = []string{}
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, fmt.Errorf("authz: InviteMember: %w", err)
	}
	return &InviteMemberResponse{Invitation: inv, Token: rawTok}, nil
}

func (s *serviceImpl) AcceptInvitation(ctx context.Context, userID, organizationID, rawToken string) (*Membership, *OrganizationInvitation, error) {
	tokenHash := token.Hash(rawToken)
	member, inv, err := s.repo.AcceptInvitation(ctx, organizationID, tokenHash, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("authz: AcceptInvitation: %w", err)
	}
	s.invalidateUser(ctx, userID, organizationID)
	return member, inv, nil
}

func (s *serviceImpl) ResendInvitation(ctx context.Context, organizationID, invitationRef string) (*ResendInvitationResponse, error) {
	rawTok, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("authz: ResendInvitation: generate token: %w", err)
	}
	inv, err := s.repo.ResendInvitation(ctx, organizationID, invitationRef, tokenHash, time.Now().Add(invitationTTL))
	if err != nil {
		return nil, fmt.Errorf("authz: ResendInvitation: %w", err)
	}
	return &ResendInvitationResponse{Invitation: inv, Token: rawTok}, nil
}

func (s *serviceImpl) RevokeInvitation(ctx context.Context, organizationID, invitationRef string) error {
	if err := s.repo.RevokeInvitation(ctx, organizationID, invitationRef); err != nil {
		return fmt.Errorf("authz: RevokeInvitation: %w", err)
	}
	return nil
}

func (s *serviceImpl) ListRoles(ctx context.Context) ([]*RoleWithPermissions, error) {
	roles, err := s.repo.ListRolesWithPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRoles: %w", err)
	}
	if roles == nil {
		roles = []*RoleWithPermissions{}
	}
	return roles, nil
}

func (s *serviceImpl) ListRolesForOrg(ctx context.Context, organizationID string) ([]*RoleWithPermissions, error) {
	roles, err := s.repo.ListRolesForOrg(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRolesForOrg: %w", err)
	}
	result := make([]*RoleWithPermissions, 0, len(roles))
	for _, r := range roles {
		result = append(result, &RoleWithPermissions{Role: r, Permissions: []*Permission{}})
	}
	return result, nil
}

func (s *serviceImpl) CreateRole(ctx context.Context, organizationID string, req CreateRoleRequest) (*Role, error) {
	if err := validateRoleRequest(req.Name, req.PermissionKeys); err != nil {
		return nil, err
	}
	if err := s.validatePermissionKeys(ctx, req.PermissionKeys); err != nil {
		return nil, err
	}
	role, err := s.repo.CreateRole(ctx, organizationID, req)
	if err != nil {
		return nil, fmt.Errorf("authz: CreateRole: %w", err)
	}
	return role, nil
}

func (s *serviceImpl) GetRole(ctx context.Context, organizationID, roleRef string) (*RoleWithPermissions, error) {
	role, err := s.repo.GetRoleByRef(ctx, organizationID, roleRef)
	if err != nil {
		return nil, fmt.Errorf("authz: GetRole: %w", err)
	}
	if role == nil {
		return nil, ErrRoleNotFound
	}
	rolePerms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("authz: GetRole: list permissions: %w", err)
	}
	permSet := map[string]struct{}{}
	for _, k := range role.Permissions {
		permSet[k] = struct{}{}
	}
	var perms []*Permission
	for _, p := range rolePerms {
		if _, ok := permSet[p.Key()]; ok {
			perms = append(perms, p)
		}
	}
	return &RoleWithPermissions{Role: role, Permissions: perms}, nil
}

func (s *serviceImpl) UpdateRole(ctx context.Context, organizationID, roleRef string, req UpdateRoleRequest) (*Role, error) {
	if req.Name != "" && !roleNamePattern.MatchString(strings.TrimSpace(req.Name)) {
		return nil, ErrInvalidRoleName
	}
	if req.PermissionKeys != nil {
		if err := s.validatePermissionKeys(ctx, req.PermissionKeys); err != nil {
			return nil, err
		}
	}
	role, err := s.repo.UpdateRole(ctx, organizationID, roleRef, req)
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateRole: %w", err)
	}
	s.invalidateOrg(ctx, organizationID)
	return role, nil
}

func (s *serviceImpl) DeleteRole(ctx context.Context, organizationID, roleRef string) error {
	if err := s.repo.DeleteRole(ctx, organizationID, roleRef); err != nil {
		return fmt.Errorf("authz: DeleteRole: %w", err)
	}
	s.invalidateOrg(ctx, organizationID)
	return nil
}

func (s *serviceImpl) UpdateRolePermissions(ctx context.Context, organizationID, roleRef string, req UpdateRolePermissionsRequest) (*Role, error) {
	if err := s.validatePermissionKeys(ctx, req.PermissionKeys); err != nil {
		return nil, err
	}
	role, err := s.repo.UpdateRolePermissions(ctx, organizationID, roleRef, req.PermissionKeys)
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateRolePermissions: %w", err)
	}
	s.invalidateOrg(ctx, organizationID)
	return role, nil
}

func (s *serviceImpl) CloneRole(ctx context.Context, organizationID, roleRef string, req CloneRoleRequest) (*Role, error) {
	if err := validateRoleName(req.Name); err != nil {
		return nil, err
	}
	role, err := s.repo.CloneRole(ctx, organizationID, roleRef, req.Name, req.Description)
	if err != nil {
		return nil, fmt.Errorf("authz: CloneRole: %w", err)
	}
	return role, nil
}

func (s *serviceImpl) ListPermissions(ctx context.Context) ([]*Permission, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("authz: ListPermissions: %w", err)
	}
	if perms == nil {
		perms = []*Permission{}
	}
	return perms, nil
}

func (s *serviceImpl) GetMemberPermissions(ctx context.Context, organizationID, memberRef string) (*MemberPermissionsResponse, error) {
	m, err := s.repo.GetMemberByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMemberNotFound
	}
	return s.memberPermissionsResponse(ctx, organizationID, m)
}

func (s *serviceImpl) UpdateMemberPermissions(ctx context.Context, organizationID, memberRef string, req UpdateMemberPermissionsRequest) (*MemberPermissionsResponse, error) {
	custom := req.CustomPermissions
	if custom == nil {
		custom = req.Grant
	}
	denied := req.DeniedPermissions
	if denied == nil {
		denied = req.Deny
	}
	if err := s.validatePermissionKeys(ctx, append(custom, denied...)); err != nil {
		return nil, err
	}
	m, err := s.repo.UpdateMemberPermissions(ctx, organizationID, memberRef, custom, denied)
	if err != nil {
		return nil, fmt.Errorf("authz: UpdateMemberPermissions: %w", err)
	}
	s.invalidateUser(ctx, m.UserID, organizationID)
	return s.memberPermissionsResponse(ctx, organizationID, m)
}

func (s *serviceImpl) CheckMember(ctx context.Context, organizationID string, req CheckMemberPermissionRequest) (*CheckMemberPermissionResponse, error) {
	permission := strings.ToLower(strings.TrimSpace(req.Permission))
	if permission == "" && req.Resource != "" && req.Action != "" {
		permission = strings.ToLower(strings.TrimSpace(req.Resource + "." + req.Action))
	}
	if permission == "" || !strings.Contains(permission, ".") {
		return nil, ErrInvalidPermissionKey
	}
	memberRef := strings.TrimSpace(req.MemberID)
	if memberRef == "" {
		memberRef = strings.TrimSpace(req.UserID)
	}
	if memberRef == "" {
		return nil, ErrMemberNotFound
	}
	m, err := s.repo.GetMemberByRef(ctx, organizationID, memberRef)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMemberNotFound
	}
	parts := strings.SplitN(permission, ".", 2)
	allowed, err := s.Can(ctx, m.UserID, organizationID, parts[0], parts[1])
	if err != nil {
		return nil, err
	}
	return &CheckMemberPermissionResponse{Allowed: allowed, Permission: permission, MemberID: m.ID, UserID: m.UserID}, nil
}

func (s *serviceImpl) PermissionMatrix(ctx context.Context, organizationID string) (*PermissionMatrixResponse, error) {
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.repo.ListRolesForOrg(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	matrix := make([]*RolePermissionMatrixRow, 0, len(roles))
	for _, role := range roles {
		row := &RolePermissionMatrixRow{Role: role, PermissionKeys: map[string]bool{}}
		for _, p := range perms {
			row.PermissionKeys[p.Key()] = false
		}
		for _, key := range role.Permissions {
			row.PermissionKeys[key] = true
		}
		matrix = append(matrix, row)
	}
	return &PermissionMatrixResponse{Permissions: perms, Roles: roles, Matrix: matrix}, nil
}

func (s *serviceImpl) memberPermissionsResponse(ctx context.Context, organizationID string, m *Membership) (*MemberPermissionsResponse, error) {
	rolePerms := []string{}
	if m.RoleID != nil {
		role, err := s.repo.GetRoleByID(ctx, *m.RoleID)
		if err != nil {
			return nil, err
		}
		if role != nil {
			rolePerms = append(rolePerms, role.Permissions...)
		}
	} else if m.RoleKey != "" {
		role, err := s.repo.GetRoleByRef(ctx, organizationID, m.RoleKey)
		if err != nil {
			return nil, err
		}
		if role != nil {
			rolePerms = append(rolePerms, role.Permissions...)
		}
	}
	perms, err := s.repo.GetUserPermissions(ctx, m.UserID, organizationID)
	if err != nil {
		return nil, err
	}
	return &MemberPermissionsResponse{
		MemberID: m.ID, UserID: m.UserID,
		RolePermissionKeys: uniqueSorted(rolePerms),
		CustomPermissions:  uniqueSorted(m.CustomPermissions),
		DeniedPermissions:  uniqueSorted(m.DeniedPermissions),
		Effective:          permissionsToKeys(perms),
	}, nil
}

func (s *serviceImpl) resolveRole(ctx context.Context, organizationID, roleID, roleName string) (*Role, error) {
	ref := roleID
	if ref == "" {
		ref = roleName
	}
	if ref == "" {
		return nil, nil
	}
	role, err := s.repo.GetRoleByRef(ctx, organizationID, ref)
	if err != nil {
		return nil, fmt.Errorf("authz: resolveRole: %w", err)
	}
	return role, nil
}

func (s *serviceImpl) validatePermissionKeys(ctx context.Context, keys []string) error {
	keys = normaliseKeys(keys)
	if len(keys) == 0 {
		return nil
	}
	perms, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return err
	}
	allowed := map[string]struct{}{}
	for _, p := range perms {
		allowed[p.Key()] = struct{}{}
	}
	for _, key := range keys {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: %s", ErrInvalidPermissionKey, key)
		}
	}
	return nil
}

func (s *serviceImpl) invalidateUser(ctx context.Context, userID, organizationID string) {
	cacheKey := permCacheKey(userID, organizationID)
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		slog.Warn("authz: failed to invalidate permission cache", slog.String("key", cacheKey), slog.Any("error", err))
	}
}

func (s *serviceImpl) invalidateOrg(ctx context.Context, organizationID string) {
	pattern := "perm:*:" + organizationID
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		if err := s.redis.Del(ctx, keys...).Err(); err != nil {
			slog.Warn("authz: failed to invalidate organization permission cache", slog.String("organization_id", organizationID), slog.Any("error", err))
		}
	}
}

func validateRoleRequest(name string, keys []string) error {
	if err := validateRoleName(name); err != nil {
		return err
	}
	if keys == nil {
		return ErrPermissionRequired
	}
	return nil
}

func validateRoleName(name string) error {
	name = strings.TrimSpace(name)
	if !roleNamePattern.MatchString(name) {
		return ErrInvalidRoleName
	}
	reserved := map[string]bool{RoleOwner: true, RoleAdmin: true, RoleManager: true, RoleMember: true, RoleViewer: true}
	if reserved[strings.ToLower(name)] {
		return ErrReservedRoleName
	}
	return nil
}

func validMemberStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case MemberStatusActive, MemberStatusInactive, MemberStatusSuspended:
		return true
	default:
		return false
	}
}

func permissionsToKeys(perms []*Permission) []string {
	keys := make([]string, 0, len(perms))
	for _, p := range perms {
		keys = append(keys, p.Key())
	}
	return uniqueSorted(keys)
}

func uniqueSorted(keys []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, key := range keys {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func normaliseEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func permCacheKey(userID, organizationID string) string {
	return "perm:" + userID + ":" + organizationID
}

func normaliseKeys(keys []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" {
			continue
		}
		if _, exists := seen[k]; exists {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

var (
	ErrCannotAssignOwner       = errors.New("owner role cannot be assigned via API")
	ErrCannotChangeOwnRole     = errors.New("you cannot change your own role")
	ErrCannotModifyOwner       = errors.New("owner membership cannot be modified this way")
	ErrCannotModifySystemRole  = errors.New("system roles cannot be modified")
	ErrRoleNotFound            = errors.New("role not found")
	ErrMemberNotFound          = errors.New("member not found in this organization")
	ErrInvalidMemberStatus     = errors.New("invalid member status")
	ErrInvalidRoleName         = errors.New("invalid role name")
	ErrReservedRoleName        = errors.New("reserved role name")
	ErrPermissionRequired      = errors.New("at least one permission must be provided")
	ErrInvalidPermissionKey    = errors.New("invalid permission key")
	ErrInvalidInvitationEmail  = errors.New("invalid invitation email")
	ErrInvitationNotFound      = errors.New("invitation not found")
	ErrInvitationNotPending    = errors.New("invitation is not pending")
	ErrInvitationExpired       = errors.New("invitation has expired")
	ErrInvitationEmailMismatch = errors.New("invitation email does not match authenticated user")
)
