// backend/internal/hrm/holidays/handler.go
package holidays

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// ListCalendars godoc
//
//	@Summary		List holiday calendars
//	@Description	Returns holiday calendar definitions for the organization.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.view`
//	@Tags			HRM / Holiday Calendars
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			active	query		bool	false	"When true, return only active calendars"
//	@Success		200		{object}	response.OK{data=CalendarListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars [get]
func (h *Handler) ListCalendars(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListCalendars(c.Context(), orgID, strings.ToLower(c.Query("active")) == "true")
	if err != nil {
		log.Error("holidays: ListCalendars", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// CreateCalendar godoc
//
//	@Summary		Create holiday calendar
//	@Description	Creates a new annual holiday calendar.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateCalendarRequest	true	"Calendar details"
//	@Success		201		{object}	response.Created{data=object{calendar=HolidayCalendar}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars [post]
func (h *Handler) CreateCalendar(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateCalendarRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cal, err := h.service.CreateCalendar(c.Context(), orgID, userID, req)
	if err != nil {
		return h.calError(c, err)
	}
	return response.Created(c, fiber.Map{"calendar": cal}, "Holiday calendar created")
}

// GetCalendar godoc
//
//	@Summary		Get holiday calendar
//	@Description	Returns a holiday calendar including all its holidays.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.view`
//	@Tags			HRM / Holiday Calendars
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			calendarId	path		string	true	"Calendar public ID (hc_*)"
//	@Success		200			{object}	response.OK{data=object{calendar=HolidayCalendar}}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"CALENDAR_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId} [get]
func (h *Handler) GetCalendar(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cal, err := h.service.GetCalendar(c.Context(), orgID, c.Params("calendarId"))
	if err != nil {
		return h.calError(c, err)
	}
	return response.OK(c, fiber.Map{"calendar": cal}, "OK")
}

// UpdateCalendar godoc
//
//	@Summary		Update holiday calendar
//	@Description	Partially updates a calendar's metadata (not its holidays).
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			calendarId	path		string					true	"Calendar public ID (hc_*)"
//	@Param			body		body		UpdateCalendarRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{calendar=HolidayCalendar}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId} [patch]
func (h *Handler) UpdateCalendar(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateCalendarRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cal, err := h.service.UpdateCalendar(c.Context(), orgID, c.Params("calendarId"), req)
	if err != nil {
		return h.calError(c, err)
	}
	return response.OK(c, fiber.Map{"calendar": cal}, "Calendar updated")
}

// DeleteCalendar godoc
//
//	@Summary		Delete holiday calendar
//	@Description	Permanently deletes a holiday calendar and all its holidays.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			calendarId	path	string	true	"Calendar public ID (hc_*)"
//	@Success		204			"Calendar deleted"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId} [delete]
func (h *Handler) DeleteCalendar(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteCalendar(c.Context(), orgID, c.Params("calendarId")); err != nil {
		return h.calError(c, err)
	}
	return response.NoContent(c)
}

// ListHolidays godoc
//
//	@Summary		List holidays in calendar
//	@Description	Returns all holidays in a specific calendar, ordered by date.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.view`
//	@Tags			HRM / Holiday Calendars
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			calendarId	path		string	true	"Calendar public ID (hc_*)"
//	@Success		200			{object}	response.OK{data=HolidayListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId}/holidays [get]
func (h *Handler) ListHolidays(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListHolidays(c.Context(), orgID, c.Params("calendarId"))
	if err != nil {
		log.Error("holidays: ListHolidays", slog.Any("error", err))
		return h.calError(c, err)
	}
	return response.OK(c, res, "OK")
}

// CreateHoliday godoc
//
//	@Summary		Add holiday to calendar
//	@Description	Adds a holiday entry to an existing calendar.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			calendarId	path		string				true	"Calendar public ID (hc_*)"
//	@Param			body		body		CreateHolidayRequest	true	"Holiday entry"
//	@Success		201			{object}	response.Created{data=object{holiday=Holiday}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		409			{object}	response.Error	"DATE_CONFLICT"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId}/holidays [post]
func (h *Handler) CreateHoliday(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateHolidayRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	hol, err := h.service.CreateHoliday(c.Context(), orgID, c.Params("calendarId"), req)
	if err != nil {
		return h.holError(c, err)
	}
	return response.Created(c, fiber.Map{"holiday": hol}, "Holiday added")
}

// UpdateHoliday godoc
//
//	@Summary		Update holiday
//	@Description	Updates a holiday entry in a calendar.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			calendarId	path		string				true	"Calendar public ID (hc_*)"
//	@Param			holidayId	path		string				true	"Holiday public ID (hd_*)"
//	@Param			body		body		UpdateHolidayRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{holiday=Holiday}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId}/holidays/{holidayId} [patch]
func (h *Handler) UpdateHoliday(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateHolidayRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	hol, err := h.service.UpdateHoliday(c.Context(), orgID, c.Params("calendarId"), c.Params("holidayId"), req)
	if err != nil {
		return h.holError(c, err)
	}
	return response.OK(c, fiber.Map{"holiday": hol}, "Holiday updated")
}

// DeleteHoliday godoc
//
//	@Summary		Delete holiday
//	@Description	Removes a holiday entry from a calendar.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			calendarId	path	string	true	"Calendar public ID (hc_*)"
//	@Param			holidayId	path	string	true	"Holiday public ID (hd_*)"
//	@Success		204			"Holiday deleted"
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/{calendarId}/holidays/{holidayId} [delete]
func (h *Handler) DeleteHoliday(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteHoliday(c.Context(), orgID, c.Params("calendarId"), c.Params("holidayId")); err != nil {
		return h.holError(c, err)
	}
	return response.NoContent(c)
}

// AssignCalendar godoc
//
//	@Summary		Assign calendar to entity
//	@Description	Assigns a holiday calendar to an organization, department, or employee.
//	@Description	Upserts — if the assignee already has a calendar, it is replaced.
//	@Description	Lookup priority: employee → department → organization.
//	@Description
//	@Description	**Required permission:** `hrm.holidays.manage`
//	@Tags			HRM / Holiday Calendars
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		AssignCalendarRequest	true	"Assignment details"
//	@Success		200		{object}	response.OK{data=object{assignment=CalendarAssignment}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/holiday-calendars/assignments [post]
func (h *Handler) AssignCalendar(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AssignCalendarRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.AssignCalendar(c.Context(), orgID, userID, req)
	if err != nil {
		return h.calError(c, err)
	}
	return response.OK(c, fiber.Map{"assignment": a}, "Calendar assigned")
}

func (h *Handler) calError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrCalendarNotFound):
		return response.NotFound(c, "CALENDAR_NOT_FOUND", "Holiday calendar not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "CALENDAR_NAME_CONFLICT", "A calendar with this name and year already exists")
	case errors.Is(err, ErrInvalidYear):
		return response.BadRequest(c, "INVALID_YEAR", "year must be between 2000 and 2100")
	case errors.Is(err, ErrInvalidAssigneeType):
		return response.BadRequest(c, "INVALID_ASSIGNEE_TYPE", "assignee_type must be: organization, department, or employee")
	case errors.Is(err, ErrAssigneeIDRequired):
		return response.BadRequest(c, "ASSIGNEE_ID_REQUIRED", "assignee_id is required")
	case errors.Is(err, ErrEffectiveDateRequired):
		return response.BadRequest(c, "EFFECTIVE_DATE_REQUIRED", "effective_date is required in YYYY-MM-DD format")
	default:
		log.Error("holidays: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) holError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrHolidayNotFound):
		return response.NotFound(c, "HOLIDAY_NOT_FOUND", "Holiday not found")
	case errors.Is(err, ErrCalendarNotFound):
		return response.NotFound(c, "CALENDAR_NOT_FOUND", "Holiday calendar not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrDateRequired):
		return response.BadRequest(c, "DATE_REQUIRED", "date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrDateConflict):
		return response.Conflict(c, "DATE_CONFLICT", "A holiday on this date already exists in this calendar")
	case errors.Is(err, ErrInvalidHolidayType):
		return response.BadRequest(c, "INVALID_HOLIDAY_TYPE", "holiday_type must be: public, company, or optional")
	default:
		log.Error("holidays: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
