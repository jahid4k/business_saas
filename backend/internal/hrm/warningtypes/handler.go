// backend/internal/hrm/warningtypes/handler.go
package warningtypes

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM warning type configuration endpoints.
type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// ListTypes godoc
//
//	@Summary		List warning types
//	@Description	Returns all warning type definitions configured for the organization.
//	@Description	These are the config-layer categories (Verbal, Written, Final, PIP, etc.).
//	@Description	Actual issued warnings are managed separately in the Warnings module (Group C1).
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.view`
//	@Tags			HRM / Warning Types
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			active	query		bool	false	"When true, return only active types"
//	@Success		200		{object}	response.OK{data=WarningTypeListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types [get]
func (h *Handler) ListTypes(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	result, err := h.service.ListTypes(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("warningtypes: ListTypes", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateType godoc
//
//	@Summary		Create warning type
//	@Description	Defines a new warning category for the organization.
//	@Description
//	@Description	**severity_level:** 1 (minor counselling) to 10 (final warning before termination).
//	@Description	**can_be_issued_by:** array of role names (e.g. ["manager","hr_manager"]).
//	@Description	**valid_duration_days:** 0 = permanent. >0 = expires after N days from issue.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Description
//	@Description	**Error codes:** `NAME_REQUIRED` · `NAME_TOO_LONG` · `NAME_CONFLICT` · `INVALID_SEVERITY_LEVEL`
//	@Tags			HRM / Warning Types
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateWarningTypeRequest	true	"Warning type definition"
//	@Success		201		{object}	response.Created{data=object{warning_type=WarningType}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error	"NAME_CONFLICT"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types [post]
func (h *Handler) CreateType(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateWarningTypeRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	wt, err := h.service.CreateType(c.Context(), orgID, userID, req)
	if err != nil { return h.typeError(c, err) }
	return response.Created(c, fiber.Map{"warning_type": wt}, "Warning type created")
}

// GetType godoc
//
//	@Summary		Get warning type
//	@Description	Returns a single warning type by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.view`
//	@Tags			HRM / Warning Types
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			typeId	path		string	true	"Warning type public ID (wt_*)"
//	@Success		200		{object}	response.OK{data=object{warning_type=WarningType}}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"WARNING_TYPE_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/{typeId} [get]
func (h *Handler) GetType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	wt, err := h.service.GetType(c.Context(), orgID, c.Params("typeId"))
	if err != nil { return h.typeError(c, err) }
	return response.OK(c, fiber.Map{"warning_type": wt}, "OK")
}

// UpdateType godoc
//
//	@Summary		Update warning type
//	@Description	Partially updates a warning type definition.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Tags			HRM / Warning Types
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			typeId	path		string					true	"Warning type public ID (wt_*)"
//	@Param			body	body		UpdateWarningTypeRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{warning_type=WarningType}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		409		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/{typeId} [patch]
func (h *Handler) UpdateType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateWarningTypeRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	wt, err := h.service.UpdateType(c.Context(), orgID, c.Params("typeId"), req)
	if err != nil { return h.typeError(c, err) }
	return response.OK(c, fiber.Map{"warning_type": wt}, "Warning type updated")
}

// DeleteType godoc
//
//	@Summary		Delete warning type
//	@Description	Permanently deletes a warning type. Fails if active warning records reference it.
//	@Description	Consider setting `is_active: false` to deactivate instead of deleting.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Tags			HRM / Warning Types
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			typeId	path	string	true	"Warning type public ID (wt_*)"
//	@Success		204		"Warning type deleted"
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/{typeId} [delete]
func (h *Handler) DeleteType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	if err := h.service.DeleteType(c.Context(), orgID, c.Params("typeId")); err != nil { return h.typeError(c, err) }
	return response.NoContent(c)
}

// ListEscalationRules godoc
//
//	@Summary		List escalation rules
//	@Description	Returns escalation rules for the organization, optionally filtered by warning type.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.view`
//	@Tags			HRM / Warning Types
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			warning_type_id	query		string	false	"Filter by warning type UUID"
//	@Success		200				{object}	response.OK{data=EscalationRuleListResponse}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/escalations [get]
func (h *Handler) ListEscalationRules(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	result, err := h.service.ListEscalationRules(c.Context(), orgID, c.Query("warning_type_id"))
	if err != nil {
		log.Error("warningtypes: ListEscalationRules", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateEscalationRule godoc
//
//	@Summary		Create escalation rule
//	@Description	Defines a threshold rule that alerts HR when a warning count is reached.
//	@Description
//	@Description	**By design, escalation only flags HR — it never auto-creates a new warning.**
//	@Description	HR must review the situation and decide the next action manually.
//	@Description
//	@Description	**within_days:** 0 = count all-time. >0 = sliding window.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Tags			HRM / Warning Types
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string						true	"Organization ID"
//	@Param			body	body		CreateEscalationRuleRequest	true	"Escalation rule definition"
//	@Success		201		{object}	response.Created{data=object{rule=WarningEscalationRule}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"WARNING_TYPE_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/escalations [post]
func (h *Handler) CreateEscalationRule(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateEscalationRuleRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	ru, err := h.service.CreateEscalationRule(c.Context(), orgID, userID, req)
	if err != nil { return h.ruleError(c, err) }
	return response.Created(c, fiber.Map{"rule": ru}, "Escalation rule created")
}

// UpdateEscalationRule godoc
//
//	@Summary		Update escalation rule
//	@Description	Partially updates an escalation rule.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Tags			HRM / Warning Types
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string						true	"Organization ID"
//	@Param			ruleId	path		string						true	"Rule public ID (wer_*)"
//	@Param			body	body		UpdateEscalationRuleRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{rule=WarningEscalationRule}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/escalations/{ruleId} [patch]
func (h *Handler) UpdateEscalationRule(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateEscalationRuleRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	ru, err := h.service.UpdateEscalationRule(c.Context(), orgID, c.Params("ruleId"), req)
	if err != nil { return h.ruleError(c, err) }
	return response.OK(c, fiber.Map{"rule": ru}, "Escalation rule updated")
}

// DeleteEscalationRule godoc
//
//	@Summary		Delete escalation rule
//	@Description	Permanently deletes an escalation rule.
//	@Description
//	@Description	**Required permission:** `hrm.warning_types.manage`
//	@Tags			HRM / Warning Types
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			ruleId	path	string	true	"Rule public ID (wer_*)"
//	@Success		204		"Rule deleted"
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/warning-types/escalations/{ruleId} [delete]
func (h *Handler) DeleteEscalationRule(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	if err := h.service.DeleteEscalationRule(c.Context(), orgID, c.Params("ruleId")); err != nil { return h.ruleError(c, err) }
	return response.NoContent(c)
}

func (h *Handler) typeError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrWarningTypeNotFound):
		return response.NotFound(c, "WARNING_TYPE_NOT_FOUND", "Warning type not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameTooLong):
		return response.BadRequest(c, "NAME_TOO_LONG", "Name must not exceed 100 characters")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "NAME_CONFLICT", "A warning type with this name already exists")
	case errors.Is(err, ErrInvalidSeverityLevel):
		return response.BadRequest(c, "INVALID_SEVERITY_LEVEL", "severity_level must be between 1 and 10")
	default:
		log.Error("warningtypes: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) ruleError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrEscalationRuleNotFound):
		return response.NotFound(c, "ESCALATION_RULE_NOT_FOUND", "Escalation rule not found")
	case errors.Is(err, ErrWarningTypeNotFound):
		return response.NotFound(c, "WARNING_TYPE_NOT_FOUND", "The referenced warning type was not found")
	case errors.Is(err, ErrTriggerTypeRequired):
		return response.BadRequest(c, "TRIGGER_TYPE_REQUIRED", "trigger_warning_type_id is required")
	case errors.Is(err, ErrTriggerCountRequired):
		return response.BadRequest(c, "TRIGGER_COUNT_REQUIRED", "trigger_count must be >= 1")
	case errors.Is(err, ErrInvalidAction):
		return response.BadRequest(c, "INVALID_ACTION", "action must be: notify_hr, notify_management, or flag_termination_review")
	default:
		log.Error("warningtypes: rule error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
