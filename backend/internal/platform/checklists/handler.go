// backend/internal/platform/checklists/handler.go
package checklists

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/pagination"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles checklist engine HTTP endpoints.
//
// Deliberately absent: a generic POST .../instances handler. Instantiation
// requires a SubjectContext this package cannot validate on its own — a
// generic route would have to trust a client-supplied subject_user_id /
// manager_user_id, an impersonation vector. Instantiation is reachable only
// through module-owned endpoints (e.g. internal/hrm/onboarding) that resolve
// the subject server-side and call Service.Instantiate directly.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

func callerUserID(c fiber.Ctx) (string, error) {
	id, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return "", errUnauthenticated
	}
	return id, nil
}

var errUnauthenticated = errors.New("unauthenticated")

// mapError maps a service-layer sentinel error to an HTTP response. Inline
// per handler (not a shared mapErr helper) — the HRM convention this package
// otherwise does not follow, kept here anyway since every handler below
// needs the same mapping and duplicating a switch nine times invites drift.
func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return response.NotFound(c, "TEMPLATE_NOT_FOUND", "Checklist template not found")
	case errors.Is(err, ErrTemplateItemNotFound):
		return response.NotFound(c, "TEMPLATE_ITEM_NOT_FOUND", "Checklist template item not found")
	case errors.Is(err, ErrInstanceNotFound):
		return response.NotFound(c, "INSTANCE_NOT_FOUND", "Checklist instance not found")
	case errors.Is(err, ErrInstanceItemNotFound):
		return response.NotFound(c, "INSTANCE_ITEM_NOT_FOUND", "Checklist item not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrInvalidChecklistType):
		return response.BadRequest(c, "INVALID_CHECKLIST_TYPE", "Invalid checklist_type")
	case errors.Is(err, ErrInvalidOwnerType):
		return response.BadRequest(c, "INVALID_OWNER_TYPE", "Invalid owner_type")
	case errors.Is(err, ErrInvalidSubjectType):
		return response.BadRequest(c, "INVALID_SUBJECT_TYPE", "Invalid subject_type")
	case errors.Is(err, ErrOwnerRoleRequired):
		return response.BadRequest(c, "OWNER_ROLE_REQUIRED", "owner_role is required when owner_type is 'role'")
	case errors.Is(err, ErrOwnerUserRequired):
		return response.BadRequest(c, "OWNER_USER_REQUIRED", "owner_user_id is required when owner_type is 'specific_user'")
	case errors.Is(err, ErrUnknownRole):
		return response.BadRequest(c, "UNKNOWN_ROLE", "owner_role does not name an existing role")
	case errors.Is(err, ErrTemplateHasNoItems):
		return response.BadRequest(c, "TEMPLATE_HAS_NO_ITEMS", "Template has no active items to instantiate")
	case errors.Is(err, ErrTemplateInactive):
		return response.BadRequest(c, "TEMPLATE_INACTIVE", "Template is not active")
	case errors.Is(err, ErrNotItemOwner):
		return response.Forbidden(c, "NOT_ITEM_OWNER", "You are not authorized to act on this item")
	case errors.Is(err, ErrItemAlreadyTerminal):
		return response.Conflict(c, "ITEM_ALREADY_TERMINAL", "Item is already completed or skipped")
	case errors.Is(err, ErrItemNotTerminal):
		return response.Conflict(c, "ITEM_NOT_TERMINAL", "Item is not completed or skipped")
	case errors.Is(err, ErrSkipReasonRequired):
		return response.BadRequest(c, "SKIP_REASON_REQUIRED", "reason is required to skip an item")
	case errors.Is(err, ErrAttachmentRequired):
		return response.BadRequest(c, "ATTACHMENT_REQUIRED", "This item requires an attachment before it can be completed")
	case errors.Is(err, ErrInstanceAlreadyClosed):
		return response.Conflict(c, "INSTANCE_ALREADY_CLOSED", "Instance is already completed or cancelled")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("checklists: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ============================================================
// Templates
// ============================================================

// ListTemplates handles GET /organizations/:orgId/checklists/templates
func (h *Handler) ListTemplates(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var ct *ChecklistType
	if raw := strings.TrimSpace(c.Query("checklist_type")); raw != "" {
		v := ChecklistType(raw)
		ct = &v
	}
	list, err := h.service.ListTemplates(c.Context(), orgID(c), ct)
	if err != nil {
		return mapError(c, log, "ListTemplates", err)
	}
	return response.OK(c, fiber.Map{"templates": list}, "OK")
}

// GetTemplate handles GET /organizations/:orgId/checklists/templates/:templateId
func (h *Handler) GetTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	t, err := h.service.GetTemplate(c.Context(), orgID(c), c.Params("templateId"))
	if err != nil {
		return mapError(c, log, "GetTemplate", err)
	}
	return response.OK(c, fiber.Map{"template": t}, "OK")
}

// CreateTemplate handles POST /organizations/:orgId/checklists/templates
func (h *Handler) CreateTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreateTemplate", err)
	}
	var req CreateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.CreateTemplate(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreateTemplate", err)
	}
	return response.Created(c, fiber.Map{"template": t}, "Checklist template created")
}

// UpdateTemplate handles PATCH /organizations/:orgId/checklists/templates/:templateId
func (h *Handler) UpdateTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.UpdateTemplate(c.Context(), orgID(c), c.Params("templateId"), req)
	if err != nil {
		return mapError(c, log, "UpdateTemplate", err)
	}
	return response.OK(c, fiber.Map{"template": t}, "Checklist template updated")
}

// DeleteTemplate handles DELETE /organizations/:orgId/checklists/templates/:templateId
func (h *Handler) DeleteTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteTemplate(c.Context(), orgID(c), c.Params("templateId")); err != nil {
		return mapError(c, log, "DeleteTemplate", err)
	}
	return response.NoContent(c)
}

// ============================================================
// Template items
// ============================================================

// ListTemplateItems handles GET /organizations/:orgId/checklists/templates/:templateId/items
func (h *Handler) ListTemplateItems(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	items, err := h.service.ListTemplateItems(c.Context(), orgID(c), c.Params("templateId"))
	if err != nil {
		return mapError(c, log, "ListTemplateItems", err)
	}
	return response.OK(c, fiber.Map{"items": items}, "OK")
}

// CreateTemplateItem handles POST /organizations/:orgId/checklists/templates/:templateId/items
func (h *Handler) CreateTemplateItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateTemplateItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.CreateTemplateItem(c.Context(), orgID(c), c.Params("templateId"), req)
	if err != nil {
		return mapError(c, log, "CreateTemplateItem", err)
	}
	return response.Created(c, fiber.Map{"item": item}, "Checklist template item created")
}

// UpdateTemplateItem handles PATCH /organizations/:orgId/checklists/templates/:templateId/items/:itemId
func (h *Handler) UpdateTemplateItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateTemplateItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.UpdateTemplateItem(c.Context(), orgID(c), c.Params("templateId"), c.Params("itemId"), req)
	if err != nil {
		return mapError(c, log, "UpdateTemplateItem", err)
	}
	return response.OK(c, fiber.Map{"item": item}, "Checklist template item updated")
}

// DeleteTemplateItem handles DELETE /organizations/:orgId/checklists/templates/:templateId/items/:itemId
func (h *Handler) DeleteTemplateItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteTemplateItem(c.Context(), orgID(c), c.Params("templateId"), c.Params("itemId")); err != nil {
		return mapError(c, log, "DeleteTemplateItem", err)
	}
	return response.NoContent(c)
}

// ============================================================
// Instances
// ============================================================

// ListInstances handles GET /organizations/:orgId/checklists/instances
// Query: subject_type, subject_id, status, limit, offset
func (h *Handler) ListInstances(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	p := pagination.FromCtx(c)
	f := InstanceFilter{Limit: p.Limit, Offset: p.Offset}

	if raw := strings.TrimSpace(c.Query("subject_type")); raw != "" {
		v := SubjectType(raw)
		f.SubjectType = &v
	}
	if raw := strings.TrimSpace(c.Query("subject_id")); raw != "" {
		f.SubjectID = &raw
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		v := InstanceStatus(raw)
		f.Status = &v
	}

	result, err := h.service.ListInstances(c.Context(), orgID(c), f)
	if err != nil {
		return mapError(c, log, "ListInstances", err)
	}
	return response.OK(c, result, "OK")
}

// GetInstance handles GET /organizations/:orgId/checklists/instances/:instanceId
func (h *Handler) GetInstance(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	inst, err := h.service.GetInstance(c.Context(), orgID(c), c.Params("instanceId"))
	if err != nil {
		return mapError(c, log, "GetInstance", err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "OK")
}

// CancelInstance handles POST /organizations/:orgId/checklists/instances/:instanceId/cancel
func (h *Handler) CancelInstance(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CancelInstanceRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	inst, err := h.service.CancelInstance(c.Context(), orgID(c), c.Params("instanceId"), req)
	if err != nil {
		return mapError(c, log, "CancelInstance", err)
	}
	return response.OK(c, fiber.Map{"instance": inst}, "Checklist instance cancelled")
}

// ============================================================
// Items
// ============================================================

// ListMyItems handles GET /organizations/:orgId/checklists/items/mine
// Query: status
func (h *Handler) ListMyItems(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListMyItems", err)
	}
	var status *ItemStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		v := ItemStatus(raw)
		status = &v
	}
	items, err := h.service.ListMyItems(c.Context(), orgID(c), userID, status)
	if err != nil {
		return mapError(c, log, "ListMyItems", err)
	}
	return response.OK(c, fiber.Map{"items": items}, "OK")
}

// CompleteItem handles POST /organizations/:orgId/checklists/items/:itemId/complete
func (h *Handler) CompleteItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CompleteItem", err)
	}
	var req CompleteItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.CompleteItem(c.Context(), orgID(c), c.Params("itemId"), userID, req)
	if err != nil {
		return mapError(c, log, "CompleteItem", err)
	}
	return response.OK(c, fiber.Map{"item": item}, "Item completed")
}

// ReopenItem handles POST /organizations/:orgId/checklists/items/:itemId/reopen
func (h *Handler) ReopenItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ReopenItem", err)
	}
	item, err := h.service.ReopenItem(c.Context(), orgID(c), c.Params("itemId"), userID)
	if err != nil {
		return mapError(c, log, "ReopenItem", err)
	}
	return response.OK(c, fiber.Map{"item": item}, "Item reopened")
}

// SkipItem handles POST /organizations/:orgId/checklists/items/:itemId/skip
func (h *Handler) SkipItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "SkipItem", err)
	}
	var req SkipItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.SkipItem(c.Context(), orgID(c), c.Params("itemId"), userID, req)
	if err != nil {
		return mapError(c, log, "SkipItem", err)
	}
	return response.OK(c, fiber.Map{"item": item}, "Item skipped")
}
