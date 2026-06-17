// backend/internal/platform/engagement/routes.go
package engagement

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all engagement sub-resource routes under /organizations/:orgId/crm.
// The handler is pre-bound to module="crm" so every record is tagged correctly.
// When HRM or ERP arrive, call this function again with a different module-bound handler
// and a different base path — the same tables receive hrm/erp tagged records automatically.
//
// Notes (scoped to an entity):
//
//	GET    /organizations/:orgId/crm/notes?related_type=&related_id=  <- crm.notes.view
//	POST   /organizations/:orgId/crm/notes                            <- crm.notes.create
//	GET    /organizations/:orgId/crm/notes/:noteId                    <- crm.notes.view
//	PATCH  /organizations/:orgId/crm/notes/:noteId                    <- crm.notes.update
//	DELETE /organizations/:orgId/crm/notes/:noteId                    <- crm.notes.delete
//
// Tasks (org-wide + scoped):
//
//	GET    /organizations/:orgId/crm/tasks                            <- crm.tasks.view
//	POST   /organizations/:orgId/crm/tasks                            <- crm.tasks.create
//	GET    /organizations/:orgId/crm/tasks/:taskId                    <- crm.tasks.view
//	PATCH  /organizations/:orgId/crm/tasks/:taskId                    <- crm.tasks.update
//	DELETE /organizations/:orgId/crm/tasks/:taskId                    <- crm.tasks.delete
//	POST   /organizations/:orgId/crm/tasks/:taskId/complete           <- crm.tasks.update
//	POST   /organizations/:orgId/crm/tasks/:taskId/reopen             <- crm.tasks.update
//	POST   /organizations/:orgId/crm/tasks/:taskId/assign             <- crm.tasks.assign
//
// Activities, email logs follow the same pattern.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	crm := router.Group("/organizations/:orgId/crm", requireAuth, requireOrgMatch)

	// ---- Notes ----
	notes := crm.Group("/notes")
	notes.Get("", permFn("crm.notes.view"), handler.ListNotesByRelated)
	notes.Post("", permFn("crm.notes.create"), handler.CreateNote)
	notes.Get("/:noteId", permFn("crm.notes.view"), handler.GetNote)
	notes.Patch("/:noteId", permFn("crm.notes.update"), handler.UpdateNote)
	notes.Delete("/:noteId", permFn("crm.notes.delete"), handler.DeleteNote)

	// ---- Tasks ----
	tasks := crm.Group("/tasks")
	tasks.Get("", permFn("crm.tasks.view"), handler.ListTasks)
	tasks.Post("", permFn("crm.tasks.create"), handler.CreateTask)
	tasks.Get("/:taskId", permFn("crm.tasks.view"), handler.GetTask)
	tasks.Patch("/:taskId", permFn("crm.tasks.update"), handler.UpdateTask)
	tasks.Delete("/:taskId", permFn("crm.tasks.delete"), handler.DeleteTask)
	tasks.Post("/:taskId/complete", permFn("crm.tasks.update"), handler.CompleteTask)
	tasks.Post("/:taskId/reopen", permFn("crm.tasks.update"), handler.ReopenTask)
	tasks.Post("/:taskId/assign", permFn("crm.tasks.assign"), handler.AssignTask)

	// ---- Activities ----
	activities := crm.Group("/activities")
	activities.Get("", permFn("crm.activities.view"), handler.ListActivities)
	activities.Post("", permFn("crm.activities.create"), handler.CreateActivity)
	activities.Get("/:activityId", permFn("crm.activities.view"), handler.GetActivity)
	activities.Patch("/:activityId", permFn("crm.activities.update"), handler.UpdateActivity)
	activities.Delete("/:activityId", permFn("crm.activities.delete"), handler.DeleteActivity)

	// ---- Email Logs ----
	emails := crm.Group("/emails")
	emails.Get("", permFn("crm.emails.view"), handler.ListEmailLogs)
	emails.Post("", permFn("crm.emails.create"), handler.CreateEmailLog)
	emails.Get("/:emailId", permFn("crm.emails.view"), handler.GetEmailLog)
	emails.Delete("/:emailId", permFn("crm.emails.delete"), handler.DeleteEmailLog)

	// ---- Timeline (entity-level unified feed) ----
	// GET /organizations/:orgId/crm/timeline?related_type=crm.deal&related_id=<id>
	crm.Get("/timeline", permFn("crm.notes.view"), handler.GetTimeline)
}
