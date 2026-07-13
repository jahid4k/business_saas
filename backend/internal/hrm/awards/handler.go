// backend/internal/hrm/awards/handler.go
package awards

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }
func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Query("employee_id"), c.Query("status"))
	if err != nil { log.Error("awards: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	a, err := h.service.Get(c.Context(), orgID, c.Params("awardId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"award": a}, "OK")
}

// Create godoc
//
//	@Summary		Create award record
//	@Description	Creates an award in draft status. Issue via POST .../issue.
//	@Description
//	@Description	**Required permission:** `hrm.awards.manage`
//	@Tags			HRM / Awards
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateAwardRequest	true	"Award details"
//	@Success		201		{object}	response.Created{data=object{award=Award}}
//	@Router			/organizations/{orgId}/hrm/awards [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateAwardRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	a, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"award": a}, "Award created")
}

func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateAwardRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	a, err := h.service.Update(c.Context(), orgID, c.Params("awardId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"award": a}, "Award updated")
}

func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	a, err := h.service.Submit(c.Context(), orgID, c.Params("awardId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"award": a}, "Award submitted for approval")
}

// Issue godoc
//
//	@Summary		Issue award to employee
//	@Description	Formally issues the award. Pass create_announcement=true to auto-publish an E2 announcement.
//	@Description
//	@Description	**Required permission:** `hrm.awards.issue`
//	@Tags			HRM / Awards
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			awardId	path		string			true	"Award public ID (awd_*)"
//	@Param			body	body		IssueRequest	false	"Issue options"
//	@Success		200		{object}	response.OK{data=object{award=Award}}
//	@Failure		409		{object}	response.Error	"ALREADY_ISSUED or WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/awards/{awardId}/issue [post]
func (h *Handler) Issue(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req IssueRequest
	_ = c.Bind().JSON(&req)
	a, err := h.service.Issue(c.Context(), orgID, c.Params("awardId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"award": a}, "Award issued")
}

func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	a, err := h.service.Cancel(c.Context(), orgID, c.Params("awardId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"award": a}, "Award cancelled")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "AWARD_NOT_FOUND", "Award not found")
	case errors.Is(err, ErrEmployeeIDRequired): return response.BadRequest(c, "EMPLOYEE_ID_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrTitleRequired): return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrDescriptionRequired): return response.BadRequest(c, "DESCRIPTION_REQUIRED", "description is required")
	case errors.Is(err, ErrInvalidAwardType): return response.BadRequest(c, "INVALID_AWARD_TYPE", "invalid award_type")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current award status")
	case errors.Is(err, ErrAlreadyIssued): return response.Conflict(c, "ALREADY_ISSUED", "Award has already been issued")
	default: log.Error("awards: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
