// backend/internal/authz/handler.go
package authz

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListMembers(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	members, err := h.service.ListMembers(c.Context(), orgID)
	if err != nil {
		slog.Error("authz: ListMembers error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"members": members}, "OK")
}

func (h *Handler) GetMember(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	member, err := h.service.GetMember(c.Context(), orgID, c.Params("memberId"))
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return response.NotFound(c, "MEMBER_NOT_FOUND", "Member not found")
		}
		slog.Error("authz: GetMember error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"member": member}, "OK")
}

func (h *Handler) MyMembership(c fiber.Ctx) error {
	userID, ok := userIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	myMembership, err := h.service.MyMembership(c.Context(), userID, orgID)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			return response.Forbidden(c, "NOT_A_MEMBER", "You are not a member of this organization")
		}
		slog.Error("authz: MyMembership error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"membership": myMembership}, "OK")
}

func (h *Handler) AssignRole(c fiber.Ctx) error {
	callerID, ok := userIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	targetUserID := c.Params("userId")
	if targetUserID == "" {
		memberRef := c.Params("memberId")
		if memberRef != "" {
			member, err := h.service.GetMember(c.Context(), orgID, memberRef)
			if err != nil {
				return h.authzError(c, err)
			}
			targetUserID = member.UserID
		}
	}
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
	err := h.service.AssignRole(c.Context(), callerID, targetUserID, orgID, roleName)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"user_id": targetUserID, "organization_id": orgID, "role": roleName}, "Role assigned successfully")
}

func (h *Handler) UpdateMember(c fiber.Ctx) error {
	callerID, ok := userIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateMemberRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	member, err := h.service.UpdateMember(c.Context(), callerID, orgID, c.Params("memberId"), req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"member": member}, "Member updated")
}

func (h *Handler) InviteMember(c fiber.Ctx) error {
	callerID, ok := userIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req InviteMemberRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	result, err := h.service.InviteMember(c.Context(), callerID, orgID, req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.Created(c, result, "Invitation created")
}

func (h *Handler) AcceptInvitation(c fiber.Ctx) error {
	userID, ok := userIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID := c.Params("orgId")
	if orgID == "" {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	member, inv, err := h.service.AcceptInvitation(c.Context(), userID, orgID, c.Params("token"))
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"member": member, "invitation": inv}, "Invitation accepted")
}

func (h *Handler) ResendInvitation(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	result, err := h.service.ResendInvitation(c.Context(), orgID, c.Params("invitationId"))
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, result, "Invitation resent")
}

func (h *Handler) RevokeInvitation(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.RevokeInvitation(c.Context(), orgID, c.Params("invitationId")); err != nil {
		return h.authzError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) ListRoles(c fiber.Ctx) error {
	roles, err := h.service.ListRoles(c.Context())
	if err != nil {
		slog.Error("authz: ListRoles error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"roles": roles}, "OK")
}

func (h *Handler) ListOrganizationRoles(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	roles, err := h.service.ListRolesForOrg(c.Context(), orgID)
	if err != nil {
		slog.Error("authz: ListOrganizationRoles error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"roles": roles}, "OK")
}

func (h *Handler) CreateRole(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRoleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	role, err := h.service.CreateRole(c.Context(), orgID, req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.Created(c, fiber.Map{"role": role}, "Role created")
}

func (h *Handler) GetRole(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	role, err := h.service.GetRole(c.Context(), orgID, c.Params("roleId"))
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"role": role}, "OK")
}

func (h *Handler) UpdateRole(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateRoleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	role, err := h.service.UpdateRole(c.Context(), orgID, c.Params("roleId"), req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"role": role}, "Role updated")
}

func (h *Handler) DeleteRole(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteRole(c.Context(), orgID, c.Params("roleId")); err != nil {
		return h.authzError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) UpdateRolePermissions(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateRolePermissionsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	role, err := h.service.UpdateRolePermissions(c.Context(), orgID, c.Params("roleId"), req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"role": role}, "Role permissions updated")
}

func (h *Handler) CloneRole(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CloneRoleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	role, err := h.service.CloneRole(c.Context(), orgID, c.Params("roleId"), req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.Created(c, fiber.Map{"role": role}, "Role cloned")
}

func (h *Handler) ListPermissions(c fiber.Ctx) error {
	perms, err := h.service.ListPermissions(c.Context())
	if err != nil {
		slog.Error("authz: ListPermissions error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"permissions": perms}, "OK")
}

func (h *Handler) GetMemberPermissions(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	perms, err := h.service.GetMemberPermissions(c.Context(), orgID, c.Params("memberId"))
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"permissions": perms}, "OK")
}

func (h *Handler) UpdateMemberPermissions(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateMemberPermissionsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	perms, err := h.service.UpdateMemberPermissions(c.Context(), orgID, c.Params("memberId"), req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, fiber.Map{"permissions": perms}, "Member permissions updated")
}

func (h *Handler) CheckMember(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CheckMemberPermissionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	result, err := h.service.CheckMember(c.Context(), orgID, req)
	if err != nil {
		return h.authzError(c, err)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) PermissionMatrix(c fiber.Ctx) error {
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	matrix, err := h.service.PermissionMatrix(c.Context(), orgID)
	if err != nil {
		slog.Error("authz: PermissionMatrix error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"matrix": matrix}, "OK")
}

func (h *Handler) authzError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrCannotAssignOwner):
		return response.BadRequest(c, "CANNOT_ASSIGN_OWNER", "The owner role cannot be assigned via API")
	case errors.Is(err, ErrCannotChangeOwnRole):
		return response.BadRequest(c, "CANNOT_CHANGE_OWN_ROLE", "You cannot change your own role/status")
	case errors.Is(err, ErrCannotModifyOwner):
		return response.BadRequest(c, "CANNOT_MODIFY_OWNER", "Owner membership cannot be modified this way")
	case errors.Is(err, ErrCannotModifySystemRole):
		return response.BadRequest(c, "SYSTEM_ROLE_LOCKED", "System roles cannot be modified")
	case errors.Is(err, ErrMemberNotFound):
		return response.NotFound(c, "MEMBER_NOT_FOUND", "Member not found")
	case errors.Is(err, ErrRoleNotFound):
		return response.NotFound(c, "ROLE_NOT_FOUND", "Role not found")
	case errors.Is(err, ErrInvalidMemberStatus):
		return response.BadRequest(c, "INVALID_MEMBER_STATUS", "Status must be active, inactive, or suspended")
	case errors.Is(err, ErrInvalidRoleName), errors.Is(err, ErrReservedRoleName):
		return response.BadRequest(c, "INVALID_ROLE_NAME", "Role name is invalid or reserved")
	case errors.Is(err, ErrPermissionRequired):
		return response.BadRequest(c, "PERMISSION_REQUIRED", "At least one permission is required")
	case errors.Is(err, ErrInvalidPermissionKey):
		return response.BadRequest(c, "INVALID_PERMISSION_KEY", err.Error())
	case errors.Is(err, ErrInvalidInvitationEmail):
		return response.BadRequest(c, "INVALID_INVITATION_EMAIL", "A valid email is required")
	case errors.Is(err, ErrInvitationNotFound):
		return response.NotFound(c, "INVITATION_NOT_FOUND", "Invitation not found")
	case errors.Is(err, ErrInvitationNotPending):
		return response.BadRequest(c, "INVITATION_NOT_PENDING", "Invitation is not pending")
	case errors.Is(err, ErrInvitationExpired):
		return response.BadRequest(c, "INVITATION_EXPIRED", "Invitation has expired")
	case errors.Is(err, ErrInvitationEmailMismatch):
		return response.Forbidden(c, "INVITATION_EMAIL_MISMATCH", "This invitation belongs to a different email address")
	default:
		slog.Error("authz error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func userIDFromCtx(c fiber.Ctx) (string, bool) {
	userID, ok := c.Locals("user_id").(string)
	return userID, ok && userID != ""
}

func organizationIDFromCtx(c fiber.Ctx) (string, bool) {
	orgID, _ := c.Locals("organization_id").(string)
	if orgID == "" {
		orgID, _ = c.Locals("business_id").(string)
	}
	return orgID, orgID != ""
}
