// backend/internal/hrm/announcements/handler.go
package announcements

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

// List godoc
//
//	@Summary		List announcements
//	@Description	Returns announcements. Filter by category and status.
//	@Description	Published announcements are visible to all employees.
//	@Description
//	@Description	**Required permission:** `hrm.announcements.view`
//	@Tags			HRM / Announcements
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			category	query		string	false	"general|policy|event|award|reminder|emergency|hr_update"
//	@Param			status		query		string	false	"draft|scheduled|published|expired|archived"
//	@Success		200			{object}	response.OK{data=AnnouncementListResponse}
//	@Router			/organizations/{orgId}/hrm/announcements [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.List(c.Context(), orgID, c.Query("category"), c.Query("status"))
	if err != nil {
		log.Error("announcements: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create announcement
//	@Description	Creates a draft announcement. Publish via POST .../publish.
//	@Description	When `requires_acknowledgement=true` and published, C4 ack requests are auto-created.
//	@Description
//	@Description	**Required permission:** `hrm.announcements.manage`
//	@Tags			HRM / Announcements
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string						true	"Organization ID"
//	@Param			body	body		CreateAnnouncementRequest	true	"Announcement details"
//	@Success		201		{object}	response.Created{data=object{announcement=Announcement}}
//	@Router			/organizations/{orgId}/hrm/announcements [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateAnnouncementRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"announcement": a}, "Announcement created")
}

func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	a, err := h.service.Get(c.Context(), orgID, c.Params("announcementId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"announcement": a}, "OK")
}

func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateAnnouncementRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.Update(c.Context(), orgID, c.Params("announcementId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"announcement": a}, "Announcement updated")
}

// Publish godoc
//
//	@Summary		Publish announcement
//	@Description	Publishes the announcement immediately. If `requires_acknowledgement=true`,
//	@Description	creates C4 acknowledgement requests for all target employees.
//	@Description
//	@Description	**Required permission:** `hrm.announcements.publish`
//	@Tags			HRM / Announcements
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			announcementId	path	string	true	"Announcement public ID (ann_*)"
//	@Success		200				{object}	response.OK{data=object{announcement=Announcement}}
//	@Failure		409				{object}	response.Error	"ALREADY_PUBLISHED or WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/announcements/{announcementId}/publish [post]
func (h *Handler) Publish(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	a, err := h.service.Publish(c.Context(), orgID, c.Params("announcementId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"announcement": a}, "Announcement published")
}

func (h *Handler) Schedule(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	a, err := h.service.Schedule(c.Context(), orgID, c.Params("announcementId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"announcement": a}, "Announcement scheduled")
}

func (h *Handler) Archive(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	a, err := h.service.Archive(c.Context(), orgID, c.Params("announcementId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"announcement": a}, "Announcement archived")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "ANNOUNCEMENT_NOT_FOUND", "Announcement not found")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrContentRequired):
		return response.BadRequest(c, "CONTENT_REQUIRED", "content is required")
	case errors.Is(err, ErrInvalidCategory):
		return response.BadRequest(c, "INVALID_CATEGORY", "invalid category")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current announcement status")
	case errors.Is(err, ErrAlreadyPublished):
		return response.Conflict(c, "ALREADY_PUBLISHED", "Announcement is already published")
	default:
		log.Error("announcements: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
