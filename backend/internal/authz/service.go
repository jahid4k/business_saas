// backend/internal/authz/service.go
package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// permCacheTTL is how long a user's permission set is cached in Redis.
// Short enough that role changes propagate quickly.
const permCacheTTL = 5 * time.Minute

// Service defines the authorization interface.
type Service interface {
	// Can checks whether userID has the given permission within businessID.
	// Checks Redis cache first; falls back to the database on a miss.
	Can(ctx context.Context, userID, businessID, resource, action string) (bool, error)

	// GetMembership returns the user's membership in a business.
	GetMembership(ctx context.Context, userID, businessID string) (*Membership, error)

	// MyMembership returns the current user's membership enriched with their
	// full permission list. Used by GET /api/v1/members/me.
	MyMembership(ctx context.Context, userID, businessID string) (*MyMembershipResponse, error)

	// ListMembers returns all members of a business with their user profile + role.
	ListMembers(ctx context.Context, businessID string) ([]*MemberWithUser, error)

	// AssignRole changes a member's role within a business.
	// The owner role cannot be assigned via API.
	// Invalidates the Redis permission cache for the affected user.
	AssignRole(ctx context.Context, callerID, targetUserID, businessID, roleName string) error

	// ListRoles returns all system roles with their associated permissions.
	ListRoles(ctx context.Context) ([]*RoleWithPermissions, error)

	// ListPermissions returns all defined permissions.
	ListPermissions(ctx context.Context) ([]*Permission, error)
}

type serviceImpl struct {
	repo  Repository
	redis *redis.Client
}

// NewService creates a new authz service.
func NewService(repo Repository, redisClient *redis.Client) Service {
	return &serviceImpl{
		repo:  repo,
		redis: redisClient,
	}
}

// ----------------------------------------------------------
// Can — the core permission check
// ----------------------------------------------------------

// Can returns true if the user holds the given permission in the business.
//
// Cache strategy (Redis Set per user+business):
//  1. SISMEMBER "perm:<userID>:<businessID>" "<resource>.<action>"
//  2. On hit: return immediately — no DB query
//  3. On miss: query DB for all permissions, write them all to Redis Set with TTL
//  4. Return whether the requested permission is in the set
//
// Invalidation: AssignRole deletes the key immediately on role change.
func (s *serviceImpl) Can(ctx context.Context, userID, businessID, resource, action string) (bool, error) {
	permKey := resource + "." + action
	cacheKey := permCacheKey(userID, businessID)

	// Check Redis first
	cached, err := s.redis.SIsMember(ctx, cacheKey, permKey).Result()
	if err == nil {
		return cached, nil
	}
	// Redis unavailable — fall through to DB (fail open, log warning)
	if !errors.Is(err, redis.Nil) {
		slog.Warn("authz: redis unavailable, falling back to DB",
			slog.String("key", cacheKey),
			slog.Any("error", err),
		)
	}

	// DB fallback
	perms, err := s.repo.GetUserPermissions(ctx, userID, businessID)
	if err != nil {
		return false, fmt.Errorf("authz: Can: %w", err)
	}

	// Populate cache
	if len(perms) > 0 {
		members := make([]any, len(perms))
		for i, p := range perms {
			members[i] = p.Key()
		}
		pipe := s.redis.Pipeline()
		pipe.SAdd(ctx, cacheKey, members...)
		pipe.Expire(ctx, cacheKey, permCacheTTL)
		if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
			slog.Warn("authz: failed to populate permission cache",
				slog.String("key", cacheKey),
				slog.Any("error", pipeErr),
			)
		}
	}

	// Check result
	for _, p := range perms {
		if p.Resource == resource && p.Action == action {
			return true, nil
		}
	}
	return false, nil
}

// ----------------------------------------------------------
// GetMembership
// ----------------------------------------------------------

func (s *serviceImpl) GetMembership(ctx context.Context, userID, businessID string) (*Membership, error) {
	m, err := s.repo.GetMembership(ctx, userID, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: GetMembership: %w", err)
	}
	return m, nil
}

// ----------------------------------------------------------
// MyMembership
// ----------------------------------------------------------

// MyMembership returns the caller's membership + full permission list for a business.
// This is what the frontend uses to know which buttons to show/hide.
func (s *serviceImpl) MyMembership(ctx context.Context, userID, businessID string) (*MyMembershipResponse, error) {
	membership, err := s.repo.GetMembership(ctx, userID, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: MyMembership: get membership: %w", err)
	}
	if membership == nil {
		return nil, ErrMemberNotFound
	}

	role, err := s.repo.GetRoleByID(ctx, membership.RoleID)
	if err != nil {
		return nil, fmt.Errorf("authz: MyMembership: get role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("authz: MyMembership: role not found for membership")
	}

	perms, err := s.repo.GetUserPermissions(ctx, userID, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: MyMembership: get permissions: %w", err)
	}

	permKeys := make([]string, len(perms))
	for i, p := range perms {
		permKeys[i] = p.Key()
	}

	return &MyMembershipResponse{
		MembershipID: membership.ID,
		BusinessID:   businessID,
		Role:         role.Name,
		Permissions:  permKeys,
		JoinedAt:     membership.CreatedAt,
	}, nil
}

// ----------------------------------------------------------
// ListMembers
// ----------------------------------------------------------

func (s *serviceImpl) ListMembers(ctx context.Context, businessID string) ([]*MemberWithUser, error) {
	members, err := s.repo.ListMembers(ctx, businessID)
	if err != nil {
		return nil, fmt.Errorf("authz: ListMembers: %w", err)
	}
	if members == nil {
		members = []*MemberWithUser{}
	}
	return members, nil
}

// ----------------------------------------------------------
// AssignRole
// ----------------------------------------------------------

// AssignRole changes a member's role within a business.
//
// Rules:
//   - Owner role cannot be assigned via API (set only at business creation)
//   - Caller cannot downgrade themselves (prevents accidental lockout)
//   - Target user must be a member of the business
//
// After changing the role, the Redis permission cache for the target user
// is immediately invalidated so the new role takes effect on the next request.
func (s *serviceImpl) AssignRole(ctx context.Context, callerID, targetUserID, businessID, roleName string) error {
	// Prevent assigning owner via API
	if strings.EqualFold(roleName, RoleOwner) {
		return ErrCannotAssignOwner
	}

	// Prevent caller from downgrading themselves (safety guard)
	if callerID == targetUserID {
		return ErrCannotChangeOwnRole
	}

	// Look up the target role
	role, err := s.repo.GetRoleByName(ctx, roleName)
	if err != nil {
		return fmt.Errorf("authz: AssignRole: get role: %w", err)
	}
	if role == nil {
		return ErrRoleNotFound
	}

	// Verify the target is a member of this business
	membership, err := s.repo.GetMembership(ctx, targetUserID, businessID)
	if err != nil {
		return fmt.Errorf("authz: AssignRole: get membership: %w", err)
	}
	if membership == nil {
		return ErrMemberNotFound
	}

	// Update membership role
	if err := s.repo.UpdateMembershipRole(ctx, targetUserID, businessID, role.ID); err != nil {
		return fmt.Errorf("authz: AssignRole: update: %w", err)
	}

	// Invalidate Redis cache — next request will re-query the DB
	cacheKey := permCacheKey(targetUserID, businessID)
	if delErr := s.redis.Del(ctx, cacheKey).Err(); delErr != nil {
		// Non-fatal — cache will expire on its own within permCacheTTL
		slog.Warn("authz: failed to invalidate permission cache",
			slog.String("key", cacheKey),
			slog.Any("error", delErr),
		)
	}

	slog.Info("authz: role assigned",
		slog.String("caller_id", callerID),
		slog.String("target_user", targetUserID),
		slog.String("business_id", businessID),
		slog.String("role", roleName),
	)

	return nil
}

// ----------------------------------------------------------
// ListRoles / ListPermissions
// ----------------------------------------------------------

func (s *serviceImpl) ListRoles(ctx context.Context) ([]*RoleWithPermissions, error) {
	result, err := s.repo.ListRolesWithPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("authz: ListRoles: %w", err)
	}
	if result == nil {
		result = []*RoleWithPermissions{}
	}
	return result, nil
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

// ----------------------------------------------------------
// Helpers
// ----------------------------------------------------------

func permCacheKey(userID, businessID string) string {
	return "perm:" + userID + ":" + businessID
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

var ErrCannotAssignOwner = errors.New("owner role cannot be assigned via API")
var ErrCannotChangeOwnRole = errors.New("you cannot change your own role")
var ErrRoleNotFound = errors.New("role not found")
var ErrMemberNotFound = errors.New("member not found in this business")
