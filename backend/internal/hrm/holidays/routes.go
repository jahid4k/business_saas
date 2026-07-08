// backend/internal/hrm/holidays/routes.go
package holidays

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HRM holiday calendar routes.
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	base := router.Group("/organizations/:orgId/hrm/setup/holiday-calendars", requireAuth, requireOrgMatch)

	// Calendar assignment — before /:calendarId
	base.Post("/assignments", permFn("hrm.holidays.manage"), handler.AssignCalendar)

	base.Get("/", permFn("hrm.holidays.view"), handler.ListCalendars)
	base.Post("/", permFn("hrm.holidays.manage"), handler.CreateCalendar)
	base.Get("/:calendarId", permFn("hrm.holidays.view"), handler.GetCalendar)
	base.Patch("/:calendarId", permFn("hrm.holidays.manage"), handler.UpdateCalendar)
	base.Delete("/:calendarId", permFn("hrm.holidays.manage"), handler.DeleteCalendar)

	// Holidays — nested under calendar
	base.Get("/:calendarId/holidays", permFn("hrm.holidays.view"), handler.ListHolidays)
	base.Post("/:calendarId/holidays", permFn("hrm.holidays.manage"), handler.CreateHoliday)
	base.Patch("/:calendarId/holidays/:holidayId", permFn("hrm.holidays.manage"), handler.UpdateHoliday)
	base.Delete("/:calendarId/holidays/:holidayId", permFn("hrm.holidays.manage"), handler.DeleteHoliday)
}
