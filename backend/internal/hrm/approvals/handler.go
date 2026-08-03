// backend/internal/hrm/approvals/handler.go
package approvals

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM approval template and instance HTTP endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// ListTemplates godoc
//
//	@Summary		List approval templates
//	@Description	Returns all active approval chain templates for the organization.
//	@Description	Filter by action_type (leave, promotion, transfer, etc.) to narrow results.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.view`
//	@Tags			HRM / Approvals
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			action_type	query		string	false	"Filter by action type (leave|resignation|promotion|transfer|warning|document|termination)"
//	@Success		200			{object}	response.OK{data=TemplateListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals [get]
func (h *Handler) ListTemplates(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	result, err := h.service.ListTemplates(c.Context(), orgID, c.Query("action_type"))
	if err != nil {
		log.Error("approvals: ListTemplates", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateTemplate godoc
//
//	@Summary		Create approval template
//	@Description	Creates a new approval chain template with ordered levels.
//	@Description
//	@Description	Levels must be provided in sequential order (1, 2, 3...). Each level
//	@Description	specifies who approves and what happens if the SLA expires.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.manage`
//	@Description
//	@Description	**Error codes:**
//	@Description	- `NAME_REQUIRED` · `INVALID_ACTION_TYPE` · `NO_LEVELS` · `INVALID_LEVEL` · `INVALID_APPROVER_TYPE`
//	@Tags			HRM / Approvals
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateTemplateRequest	true	"Template with levels"
//	@Success		201		{object}	response.Created{data=object{template=ApprovalTemplate}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals [post]
func (h *Handler) CreateTemplate(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.CreateTemplate(c.Context(), orgID, userID, req)
	if err != nil { return h.tmplError(c, err) }
	return response.Created(c, fiber.Map{"template": t}, "Approval template created")
}

// GetTemplate godoc
//
//	@Summary		Get approval template
//	@Description	Returns an approval template including its full level chain.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.view`
//	@Tags			HRM / Approvals
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			templateId	path		string	true	"Template public ID (apt_*)"
//	@Success		200			{object}	response.OK{data=object{template=ApprovalTemplate}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"APPROVAL_TEMPLATE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/{templateId} [get]
func (h *Handler) GetTemplate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.GetTemplate(c.Context(), orgID, c.Params("templateId"))
	if err != nil { return h.tmplError(c, err) }
	return response.OK(c, fiber.Map{"template": t}, "OK")
}

// UpdateTemplate godoc
//
//	@Summary		Update approval template
//	@Description	Updates template metadata. To change levels, delete and recreate the template.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.manage`
//	@Tags			HRM / Approvals
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			templateId	path		string					true	"Template public ID (apt_*)"
//	@Param			body		body		UpdateTemplateRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{template=ApprovalTemplate}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/{templateId} [patch]
func (h *Handler) UpdateTemplate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.UpdateTemplate(c.Context(), orgID, c.Params("templateId"), req)
	if err != nil { return h.tmplError(c, err) }
	return response.OK(c, fiber.Map{"template": t}, "Approval template updated")
}

// DeleteTemplate godoc
//
//	@Summary		Delete approval template
//	@Description	Permanently deletes an approval template. Active instances are not affected
//	@Description	(they hold a snapshot). Consider deactivating instead.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.manage`
//	@Tags			HRM / Approvals
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			templateId	path	string	true	"Template public ID (apt_*)"
//	@Success		204			"Template deleted"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/{templateId} [delete]
func (h *Handler) DeleteTemplate(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	if err := h.service.DeleteTemplate(c.Context(), orgID, c.Params("templateId")); err != nil {
		return h.tmplError(c, err)
	}
	return response.NoContent(c)
}

// ListInstances godoc
//
//	@Summary		List approval instances
//	@Description	Returns a list of approval instances.
//	@Description	Filter by status (pending|approved|rejected|cancelled) to narrow results.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.view`
//	@Tags			HRM / Approvals
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			requester_id query		string	false	"Filter by requester ID"
//	@Param			limit		query		int		false	"Limit (default 50)"
//	@Param			offset		query		int		false	"Offset (default 0)"
//	@Success		200			{object}	response.OK{data=InstanceListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/instances [get]
func (h *Handler) ListInstances(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	
	limitStr := c.Query("limit", "50")
	offsetStr := c.Query("offset", "0")
	
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit == 0 { limit = 50 }
	
	status := c.Query("status")
	requesterID := c.Query("requester_id")
	
	result, err := h.service.ListInstances(c.Context(), orgID, limit, offset, status, requesterID)
	if err != nil {
		log.Error("approvals: ListInstances", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// GetInstance godoc
//
//	@Summary		Get approval instance
//	@Description	Returns a live approval instance including its snapshot and decisions.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.view`
//	@Tags			HRM / Approvals
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			instanceId	path		string	true	"Instance public ID (api_*)"
//	@Success		200			{object}	response.OK{data=object{instance=ApprovalInstance}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"APPROVAL_INSTANCE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/instances/{instanceId} [get]
func (h *Handler) GetInstance(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	inst, err := h.service.GetInstance(c.Context(), orgID, c.Params("instanceId"))
	if err != nil { return h.instError(c, err) }
	return response.OK(c, fiber.Map{"instance": inst}, "OK")
}

// Approve godoc
//
//	@Summary		Approve approval instance
//	@Description	Records an approved decision on the current level and advances to the next.
//	@Description	If this is the final level, the instance status becomes `approved`.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.action`
//	@Tags			HRM / Approvals
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			instanceId	path		string			true	"Instance public ID (api_*)"
//	@Param			body		body		DecisionRequest	false	"Optional approval note"
//	@Success		200			{object}	response.OK{data=object{instance=ApprovalInstance}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ALREADY_COMPLETED"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/instances/{instanceId}/approve [post]
func (h *Handler) Approve(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req DecisionRequest
	req.Action = "approved"
	_ = c.Bind().JSON(&req)
	req.Action = "approved" // enforce correct action
	inst, err := h.service.Decide(c.Context(), orgID, c.Params("instanceId"), userID, req)
	if err != nil { return h.instError(c, err) }
	return response.OK(c, fiber.Map{"instance": inst}, "Approved")
}

// Reject godoc
//
//	@Summary		Reject approval instance
//	@Description	Records a rejected decision. The instance status becomes `rejected` immediately.
//	@Description
//	@Description	**Required permission:** `hrm.approvals.action`
//	@Tags			HRM / Approvals
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			instanceId	path		string			true	"Instance public ID (api_*)"
//	@Param			body		body		DecisionRequest	false	"Rejection note (recommended)"
//	@Success		200			{object}	response.OK{data=object{instance=ApprovalInstance}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ALREADY_COMPLETED"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/approvals/instances/{instanceId}/reject [post]
func (h *Handler) Reject(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req DecisionRequest
	req.Action = "rejected"
	_ = c.Bind().JSON(&req)
	req.Action = "rejected"
	inst, err := h.service.Decide(c.Context(), orgID, c.Params("instanceId"), userID, req)
	if err != nil { return h.instError(c, err) }
	return response.OK(c, fiber.Map{"instance": inst}, "Rejected")
}

func (h *Handler) tmplError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return response.NotFound(c, "APPROVAL_TEMPLATE_NOT_FOUND", "Approval template not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrInvalidActionType):
		return response.BadRequest(c, "INVALID_ACTION_TYPE", "Invalid action_type value")
	case errors.Is(err, ErrNoLevels):
		return response.BadRequest(c, "NO_LEVELS", "At least one approval level is required")
	case errors.Is(err, ErrInvalidLevel):
		return response.BadRequest(c, "INVALID_LEVEL", "Levels must be sequential starting from 1")
	case errors.Is(err, ErrInvalidApproverType):
		return response.BadRequest(c, "INVALID_APPROVER_TYPE", "Invalid approver_type value")
	default:
		log.Error("approvals: template error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) instError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrInstanceNotFound):
		return response.NotFound(c, "APPROVAL_INSTANCE_NOT_FOUND", "Approval instance not found")
	case errors.Is(err, ErrAlreadyCompleted):
		return response.Conflict(c, "ALREADY_COMPLETED", "This approval instance is already completed")
	default:
		log.Error("approvals: instance error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
