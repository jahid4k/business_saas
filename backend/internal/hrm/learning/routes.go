// backend/internal/hrm/learning/routes.go
package learning

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM Learning & Development route.
//
// Conventions this file must honour, all enforced by tests in
// internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL. TestPermissions_AllRoutesProtected parses the
//     AST and reads Args[0].(*ast.BasicLit), so a named constant fails even
//     though it compiles.
//   - `courses`, `versions`, `modules`, `lessons`, `enrollments` and
//     `attempts` are separate group variables. TestRouting_NoDuplicates
//     normalizes every ":x" to ":param" and keys on the receiver identifier,
//     so /:courseId and /:versionId on one shared group would collide.
//   - /enrollments/self registers BEFORE /enrollments/:enrollmentId — a
//     literal segment loses to a param when registered after it (the
//     /instances/mine precedent).
//
// ⚠ The answer-key split is the module's security boundary, not decoration:
//
//	POST .../lessons/:lessonId/answer-keys  <- hrm.courses.manage     (write)
//	GET  .../lessons/:lessonId/answer-keys  <- hrm.enrollments.grade  (read)
//
// The READ gates on a permission 'manager' does not hold, while the WRITE
// gates on course authoring. Nobody who merely runs a team can read the
// answers to a quiz they are assigning. Every learner-facing route below —
// attempts included — returns QuestionForAttempt, which has no correct-answer
// field at all.
//
// Note that hrm.enrollments.manage never appears as the gate on a per-record
// write. It cannot: the route cannot know whether the target enrollment falls
// inside the caller's reporting line, so the service narrows that, checking
// CanManage together with AuthorizeRecordAccess. Those routes gate on .view
// and the service refuses — the hrm.goals.manage and hrm.pips.manage
// precedent.
//
//	Courses      GET/POST         /organizations/:orgId/hrm/learning/courses
//	             GET/PATCH/DELETE .../courses/:courseId
//	             GET/POST         .../courses/:courseId/versions
//	Versions     GET/PATCH/DELETE .../versions/:versionId
//	             POST             .../versions/:versionId/{publish,archive,modules}
//	Modules      PATCH/DELETE     .../modules/:moduleId
//	             POST             .../modules/:moduleId/lessons
//	Lessons      PATCH/DELETE     .../lessons/:lessonId
//	             GET/POST         .../lessons/:lessonId/answer-keys
//	Enrollments  GET/POST         .../enrollments
//	             POST             .../enrollments/self
//	             GET/PATCH        .../enrollments/:enrollmentId
//	             POST             .../enrollments/:enrollmentId/cancel
//	             POST             .../enrollments/:enrollmentId/lessons/:lessonId
//	             POST             .../enrollments/:enrollmentId/lessons/:lessonId/attempts
//	Attempts     POST             .../attempts/:attemptId/submit
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/learning", requireAuth, requireOrgMatch)

	// ── Courses ─────────────────────────────────────────────────────────
	// .view reaches every role including 'viewer': a catalogue is not
	// sensitive, and a learner must see what exists before being assigned it.
	courses := base.Group("/courses")
	courses.Get("", permFn("hrm.courses.view"), handler.ListCourses)
	courses.Post("", permFn("hrm.courses.manage"), handler.CreateCourse)
	courses.Get("/:courseId", permFn("hrm.courses.view"), handler.GetCourse)
	courses.Patch("/:courseId", permFn("hrm.courses.manage"), handler.UpdateCourse)
	courses.Delete("/:courseId", permFn("hrm.courses.manage"), handler.DeleteCourse)
	courses.Get("/:courseId/versions", permFn("hrm.courses.view"), handler.ListVersions)
	courses.Post("/:courseId/versions", permFn("hrm.courses.manage"), handler.CreateVersion)

	// ── Versions ────────────────────────────────────────────────────────
	// Publishing is NOT a separate permission, unlike hrm.appraisals.publish:
	// it is reversible by publishing another version, and it discloses
	// nothing about a person.
	versions := base.Group("/versions")
	versions.Get("/:versionId", permFn("hrm.courses.view"), handler.GetVersion)
	versions.Patch("/:versionId", permFn("hrm.courses.manage"), handler.UpdateVersion)
	versions.Delete("/:versionId", permFn("hrm.courses.manage"), handler.DeleteVersion)
	versions.Post("/:versionId/publish", permFn("hrm.courses.manage"), handler.PublishVersion)
	versions.Post("/:versionId/archive", permFn("hrm.courses.manage"), handler.ArchiveVersion)
	versions.Post("/:versionId/modules", permFn("hrm.courses.manage"), handler.CreateModule)

	// ── Modules ─────────────────────────────────────────────────────────
	modules := base.Group("/modules")
	modules.Patch("/:moduleId", permFn("hrm.courses.manage"), handler.UpdateModule)
	modules.Delete("/:moduleId", permFn("hrm.courses.manage"), handler.DeleteModule)
	modules.Post("/:moduleId/lessons", permFn("hrm.courses.manage"), handler.CreateLesson)

	// ── Lessons and answer keys ─────────────────────────────────────────
	lessons := base.Group("/lessons")
	lessons.Patch("/:lessonId", permFn("hrm.courses.manage"), handler.UpdateLesson)
	lessons.Delete("/:lessonId", permFn("hrm.courses.manage"), handler.DeleteLesson)
	lessons.Post("/:lessonId/answer-keys", permFn("hrm.courses.manage"), handler.SetAnswerKey)
	// The one route that returns correct answers. .grade, not .manage.
	lessons.Get("/:lessonId/answer-keys", permFn("hrm.enrollments.grade"), handler.GetAnswerKeys)

	// ── Enrollments ─────────────────────────────────────────────────────
	enrollments := base.Group("/enrollments")
	enrollments.Get("", permFn("hrm.enrollments.view"), handler.ListEnrollments)
	enrollments.Post("", permFn("hrm.enrollments.manage"), handler.Enroll)
	// Literal before param.
	enrollments.Post("/self", permFn("hrm.enrollments.enroll_self"), handler.SelfEnroll)
	enrollments.Get("/:enrollmentId", permFn("hrm.enrollments.view"), handler.GetEnrollment)
	enrollments.Patch("/:enrollmentId", permFn("hrm.enrollments.view"), handler.UpdateEnrollment)
	enrollments.Post("/:enrollmentId/cancel", permFn("hrm.enrollments.view"), handler.CancelEnrollment)
	// Progressing is the learner's own act; the service narrows to the
	// enrollment's learner, which the route gate cannot express.
	enrollments.Post("/:enrollmentId/lessons/:lessonId", permFn("hrm.enrollments.attempt"), handler.MarkLesson)
	enrollments.Post("/:enrollmentId/lessons/:lessonId/attempts", permFn("hrm.enrollments.attempt"), handler.StartAttempt)

	// ── Attempts ────────────────────────────────────────────────────────
	attempts := base.Group("/attempts")
	attempts.Post("/:attemptId/submit", permFn("hrm.enrollments.attempt"), handler.SubmitAttempt)
}
