// backend/internal/hrm/calendar/handler.go
package calendar

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
//	@Summary		List HR calendar events
//	@Description	Filter by event_type, status, from_date (YYYY-MM-DD), to_date.
//	@Description
//	@Description	**Required permission:** `hrm.calendar.view`
//	@Tags			HRM / Calendar
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			event_type	query		string	false	"Filter by event type"
//	@Param			status		query		string	false	"upcoming|ongoing|completed|cancelled"
//	@Param			from_date	query		string	false	"Start of date range (YYYY-MM-DD)"
//	@Param			to_date		query		string	false	"End of date range (YYYY-MM-DD)"
//	@Success		200			{object}	response.OK{data=EventListResponse}
//	@Router			/organizations/{orgId}/hrm/calendar [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	res, err := h.service.List(c.Context(), orgID, c.Query("event_type"), c.Query("status"), c.Query("from_date"), c.Query("to_date"))
	if err != nil { log.Error("calendar: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create calendar event
//	@Description	Creates a calendar event. If `requires_rsvp=true`, C4 acknowledgement requests
//	@Description	are created immediately for all target employees.
//	@Description
//	@Description	**Required permission:** `hrm.calendar.manage`
//	@Tags			HRM / Calendar
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateEventRequest	true	"Event details"
//	@Success		201		{object}	response.Created{data=object{event=CalendarEvent}}
//	@Router			/organizations/{orgId}/hrm/calendar [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateEventRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	e, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"event": e}, "Calendar event created")
}

func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	e, err := h.service.Get(c.Context(), orgID, c.Params("eventId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"event": e}, "OK")
}

func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateEventRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	e, err := h.service.Update(c.Context(), orgID, c.Params("eventId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"event": e}, "Event updated")
}

func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	e, err := h.service.Cancel(c.Context(), orgID, c.Params("eventId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"event": e}, "Event cancelled")
}

// RequestRSVP godoc
//
//	@Summary		Send RSVP requests for calendar event
//	@Description	Creates C4 acknowledgement requests (acknowledgeable_type=calendar_event) for all target employees.
//	@Description	Safe to call multiple times — uses ON CONFLICT DO NOTHING.
//	@Description
//	@Description	**Required permission:** `hrm.calendar.manage`
//	@Tags			HRM / Calendar
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			eventId	path	string	true	"Event public ID (cal_*)"
//	@Success		200		{object}	response.OK{data=object{event=CalendarEvent}}
//	@Router			/organizations/{orgId}/hrm/calendar/{eventId}/rsvp [post]
func (h *Handler) RequestRSVP(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	e, err := h.service.RequestRSVP(c.Context(), orgID, c.Params("eventId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"event": e}, "RSVP requests sent")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "EVENT_NOT_FOUND", "Calendar event not found")
	case errors.Is(err, ErrTitleRequired): return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrInvalidEventType): return response.BadRequest(c, "INVALID_EVENT_TYPE", "invalid event_type")
	case errors.Is(err, ErrStartDateRequired): return response.BadRequest(c, "START_DATE_REQUIRED", "start_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrEndDateRequired): return response.BadRequest(c, "END_DATE_REQUIRED", "end_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrEndBeforeStart): return response.BadRequest(c, "END_BEFORE_START", "end_date must be >= start_date")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current event status")
	default: log.Error("calendar: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
