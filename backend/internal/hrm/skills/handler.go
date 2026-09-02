// backend/internal/hrm/skills/handler.go
package skills

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM skills taxonomy HTTP endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

var errUnauthenticated = errors.New("authentication required")

func requestOrg(c fiber.Ctx) (string, bool) {
	return middleware.OrganizationIDFromCtx(c)
}

func (h *Handler) resolveCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.skills")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.skills", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage}, nil
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func boolQuery(c fiber.Ctx, key string) *bool {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1"
	return &b
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrSkillNotFound):
		return response.NotFound(c, "SKILL_NOT_FOUND", "Skill not found")
	case errors.Is(err, ErrEmployeeSkillNotFound):
		return response.NotFound(c, "EMPLOYEE_SKILL_NOT_FOUND", "Employee skill not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")

	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "SKILL_ACCESS_DENIED", err.Error())
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	case errors.Is(err, ErrNameTaken):
		return response.Conflict(c, "SKILL_NAME_TAKEN", err.Error())
	case errors.Is(err, ErrAlreadyGranted):
		return response.Conflict(c, "SKILL_ALREADY_GRANTED", err.Error())
	case errors.Is(err, ErrSkillInUse):
		return response.Conflict(c, "SKILL_IN_USE", err.Error())
	case errors.Is(err, ErrSkillInactive):
		return response.Conflict(c, "SKILL_INACTIVE", err.Error())

	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", err.Error())
	case errors.Is(err, ErrInvalidProfic):
		return response.BadRequest(c, "INVALID_PROFICIENCY", err.Error())
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", err.Error())
	}

	log.Error("skills error", "error", err)
	return response.InternalServerError(c)
}

// ListSkills godoc
//
//	@Summary	List the skills taxonomy
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId	path		string	true	"Organization ID"
//	@Success	200		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/skills [get]
func (h *Handler) ListSkills(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListSkills(c.Context(), orgID, SkillListFilter{
		Category: c.Query("category"),
		IsActive: boolQuery(c, "is_active"),
		Search:   c.Query("search"),
		Limit:    atoiOr(c.Query("limit"), 0),
		Offset:   atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Skills retrieved")
}

// GetSkill godoc
//
//	@Summary	Get a skill
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId	path		string	true	"Organization ID"
//	@Param		skillId	path		string	true	"Skill ID"
//	@Success	200		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/skills/{skillId} [get]
func (h *Handler) GetSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.GetSkill(c.Context(), orgID, c.Params("skillId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"skill": out}, "Skill retrieved")
}

// CreateSkill godoc
//
//	@Summary	Add a skill to the taxonomy
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId	path		string				true	"Organization ID"
//	@Param		body	body		CreateSkillRequest	true	"Skill"
//	@Success	201		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/skills [post]
func (h *Handler) CreateSkill(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateSkillRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateSkill(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"skill": out}, "Skill created")
}

// UpdateSkill godoc
//
//	@Summary	Update a skill
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId	path		string				true	"Organization ID"
//	@Param		skillId	path		string				true	"Skill ID"
//	@Param		body	body		UpdateSkillRequest	true	"Changes"
//	@Success	200		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/skills/{skillId} [patch]
func (h *Handler) UpdateSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateSkillRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateSkill(c.Context(), orgID, c.Params("skillId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"skill": out}, "Skill updated")
}

// DeleteSkill godoc
//
//	@Summary		Delete a skill nobody holds
//	@Description	Refused when any employee has it recorded — deactivate instead, since the FK would cascade every record away.
//	@Tags			HRM - Skills
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			skillId	path	string	true	"Skill ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/skills/{skillId} [delete]
func (h *Handler) DeleteSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteSkill(c.Context(), orgID, c.Params("skillId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ListEmployeeSkills godoc
//
//	@Summary	List employee skills, filtered by the caller's scope tier
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		employee_id	query		string	false	"Filter by employee"
//	@Param		skill_id	query		string	false	"Filter by skill"
//	@Param		source		query		string	false	"manual | course | certification"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/employee-skills [get]
func (h *Handler) ListEmployeeSkills(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	res, err := h.service.ListEmployeeSkills(c.Context(), orgID, caller, EmployeeSkillListFilter{
		EmployeeID: c.Query("employee_id"),
		SkillID:    c.Query("skill_id"),
		Source:     c.Query("source"),
		Limit:      atoiOr(c.Query("limit"), 0),
		Offset:     atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Employee skills retrieved")
}

// GrantSkill godoc
//
//	@Summary	Record a skill against an employee
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId	path		string				true	"Organization ID"
//	@Param		body	body		GrantSkillRequest	true	"Grant"
//	@Success	201		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/employee-skills [post]
func (h *Handler) GrantSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req GrantSkillRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.GrantSkill(c.Context(), orgID, caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"employee_skill": out}, "Skill recorded")
}

// UpdateEmployeeSkill godoc
//
//	@Summary		Update a recorded skill
//	@Description	source and its provenance ids are deliberately not editable — a skill granted by passing a course must not be re-labelled as self-asserted.
//	@Tags			HRM - Skills
//	@Security		BearerAuth
//	@Param			orgId			path		string						true	"Organization ID"
//	@Param			employeeSkillId	path		string						true	"Employee skill ID"
//	@Param			body			body		UpdateEmployeeSkillRequest	true	"Changes"
//	@Success		200				{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/employee-skills/{employeeSkillId} [patch]
func (h *Handler) UpdateEmployeeSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req UpdateEmployeeSkillRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateEmployeeSkill(c.Context(), orgID, c.Params("employeeSkillId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"employee_skill": out}, "Employee skill updated")
}

// RevokeSkill godoc
//
//	@Summary	Remove a recorded skill
//	@Tags		HRM - Skills
//	@Security	BearerAuth
//	@Param		orgId			path	string	true	"Organization ID"
//	@Param		employeeSkillId	path	string	true	"Employee skill ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/employee-skills/{employeeSkillId} [delete]
func (h *Handler) RevokeSkill(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	if err := h.service.RevokeSkill(c.Context(), orgID, c.Params("employeeSkillId"), caller); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}
