// backend/internal/platform/forms/routes.go
package forms

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the generic form surface.
//
// Mounted under /organizations/:orgId with requireOrgMatch, the checklists
// shape — NOT the /platform/scheduler shape, which has no :orgId and which
// checklists' own routes.go calls out as a tenant-isolation hole not to copy.
//
// ⚠ There is deliberately NO instantiate route. Instantiation needs a subject
// and a respondent, and a generic endpoint would have to trust the client for
// both — an impersonation vector, and a form response is attributable
// evidence about a person. Consumers instantiate from their own endpoints,
// having resolved the subject from their own domain. Same reasoning
// checklists records for its own missing instantiate route.
//
//	Templates  GET/POST         /organizations/:orgId/forms/templates
//	           GET/PATCH/DELETE .../forms/templates/:templateId
//	           POST             .../forms/templates/:templateId/sections
//	Sections   PATCH/DELETE     .../forms/sections/:sectionId
//	           POST             .../forms/sections/:sectionId/questions
//	Questions  PATCH/DELETE     .../forms/questions/:questionId
//	Instances  GET              .../forms/instances
//	           GET              .../forms/instances/mine     (before /:instanceId)
//	           GET              .../forms/instances/:instanceId
//	           POST             .../forms/instances/:instanceId/{answers,submit,cancel}
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/forms", requireAuth, requireOrgMatch)

	// ── Template authoring ──────────────────────────────────────────────
	templates := base.Group("/templates")
	templates.Get("", permFn("platform.forms.view"), handler.ListTemplates)
	templates.Post("", permFn("platform.forms.manage"), handler.CreateTemplate)
	templates.Get("/:templateId", permFn("platform.forms.view"), handler.GetTemplate)
	templates.Patch("/:templateId", permFn("platform.forms.manage"), handler.UpdateTemplate)
	templates.Delete("/:templateId", permFn("platform.forms.manage"), handler.DeleteTemplate)
	templates.Post("/:templateId/sections", permFn("platform.forms.manage"), handler.CreateSection)

	// ── Sections ────────────────────────────────────────────────────────
	// Its own group variable, not a sub-path of templates: TestRouting_
	// NoDuplicates normalizes every ":x" to ":param" and keys on the
	// receiver, so /:templateId and /:sectionId on one group would collide.
	sections := base.Group("/sections")
	sections.Patch("/:sectionId", permFn("platform.forms.manage"), handler.UpdateSection)
	sections.Delete("/:sectionId", permFn("platform.forms.manage"), handler.DeleteSection)
	sections.Post("/:sectionId/questions", permFn("platform.forms.manage"), handler.CreateQuestion)

	// ── Questions ───────────────────────────────────────────────────────
	questions := base.Group("/questions")
	questions.Patch("/:questionId", permFn("platform.forms.manage"), handler.UpdateQuestion)
	questions.Delete("/:questionId", permFn("platform.forms.manage"), handler.DeleteQuestion)

	// ── Instances ───────────────────────────────────────────────────────
	// /mine registers BEFORE /:instanceId — a literal segment loses to a
	// param when registered after it (the /stages/reorder precedent).
	//
	// The write routes gate on .respond, which reaches 'member'; the service
	// then narrows to the instance's own respondent, since the route gate
	// cannot express "is this YOUR form".
	instances := base.Group("/instances")
	instances.Get("", permFn("platform.forms.view"), handler.ListInstances)
	instances.Get("/mine", permFn("platform.forms.respond"), handler.ListMyInstances)
	instances.Get("/:instanceId", permFn("platform.forms.view"), handler.GetInstance)
	instances.Post("/:instanceId/answers", permFn("platform.forms.respond"), handler.SaveAnswers)
	instances.Post("/:instanceId/submit", permFn("platform.forms.respond"), handler.SubmitInstance)
	instances.Post("/:instanceId/cancel", permFn("platform.forms.manage"), handler.CancelInstance)
}
