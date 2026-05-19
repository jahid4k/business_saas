// backend/internal/authz/handler.go
package authz

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles member management, role listing, and permission listing.
type Handler struct {
	service Service
}

// NewHandler creates a new authz Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// ListMembers handles GET /api/v1/members
// Returns all members of the current organization with their user profile and role.
// Requires: roles.assign permission
func (h *Handler) ListMembers(c fiber.Ctx) error {
	organizationID, ok := c.Locals("organization_id").(string)
	if !ok || organizationID == "" {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	members, err := h.service.ListMembers(c.Context(), organizationID)
	if err != nil {
		slog.Error("authz: ListMembers error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"members": members}, "OK")
}

// MyMembership handles GET /api/v1/members/me
// Returns the current user's membership, role, and full permission list.
// Used by the frontend to know which actions the user can perform.
// Requires: JWT + organization context only (no special permission needed)
func (h *Handler) MyMembership(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	organizationID, ok := c.Locals("organization_id").(string)
	if !ok || organizationID == "" {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	myMembership, err := h.service.MyMembership(c.Context(), userID, organizationID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return response.Forbidden(c, "NOT_A_MEMBER", "You are not a member of this organization")
		}
		slog.Error("authz: MyMembership error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"membership": myMembership}, "OK")
}

// AssignRole handles POST /api/v1/members/:userId/role
// Changes the role of a member within the current organization.
// Requires: roles.assign permission
//
// Rules enforced by the service:
//   - Owner role cannot be assigned via API
//   - Caller cannot change their own role
//   - Target must be a member of the organization
func (h *Handler) AssignRole(c fiber.Ctx) error {
	callerID, ok := c.Locals("user_id").(string)
	if !ok || callerID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	organizationID, ok := c.Locals("organization_id").(string)
	if !ok || organizationID == "" {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	targetUserID := c.Params("userId")
	if targetUserID == "" {
		return response.BadRequest(c, "MISSING_USER_ID", "User ID is required")
	}

	var req AssignRoleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	roleName := strings.ToLower(strings.TrimSpace(req.Role))
	if roleName == "" {
		return response.BadRequest(c, "ROLE_REQUIRED", "Role is required")
	}

	// Validate role name is one of the allowed values
	switch roleName {
	case RoleAdmin, RoleManager, RoleMember, RoleViewer:
		// valid
	case RoleOwner:
		return response.BadRequest(c, "CANNOT_ASSIGN_OWNER", "The owner role cannot be assigned via API")
	default:
		return response.BadRequest(c, "INVALID_ROLE", "Role must be one of: admin, manager, member, viewer")
	}

	err := h.service.AssignRole(c.Context(), callerID, targetUserID, organizationID, roleName)
	if err != nil {
		switch {
		case errors.Is(err, ErrCannotAssignOwner):
			return response.BadRequest(c, "CANNOT_ASSIGN_OWNER", "The owner role cannot be assigned via API")
		case errors.Is(err, ErrCannotChangeOwnRole):
			return response.BadRequest(c, "CANNOT_CHANGE_OWN_ROLE", "You cannot change your own role")
		case errors.Is(err, ErrMemberNotFound):
			return response.NotFound(c, "MEMBER_NOT_FOUND", "User is not a member of this organization")
		case errors.Is(err, ErrRoleNotFound):
			return response.BadRequest(c, "INVALID_ROLE", "Role must be one of: admin, manager, member, viewer")
		default:
			slog.Error("authz: AssignRole error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, fiber.Map{
		"user_id":         targetUserID,
		"organization_id": organizationID,
		"role":            roleName,
	}, "Role assigned successfully")
}

// ListRoles handles GET /api/v1/roles
// Returns all system roles with their associated permissions.
// Requires: JWT only (any authenticated user can see the role definitions)
func (h *Handler) ListRoles(c fiber.Ctx) error {
	roles, err := h.service.ListRoles(c.Context())
	if err != nil {
		slog.Error("authz: ListRoles error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"roles": roles}, "OK")
}

// ListPermissions handles GET /api/v1/permissions
// Returns all defined permissions in the system.
// Requires: JWT only
func (h *Handler) ListPermissions(c fiber.Ctx) error {
	perms, err := h.service.ListPermissions(c.Context())
	if err != nil {
		slog.Error("authz: ListPermissions error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"permissions": perms}, "OK")
}
