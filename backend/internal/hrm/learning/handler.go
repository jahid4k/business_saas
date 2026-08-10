// backend/internal/hrm/learning/handler.go
package learning

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM Learning & Development HTTP endpoints.
//
// It holds authz.Service because this module resolves FOUR authorization facts
// per request — the caller's scope tier, whether they may manage enrollments,
// whether they may grade, and their own employee id — and hands all four to the
// service on a Caller value. The performance.Handler / pip.Handler precedent.
//
// CanGrade is the one that matters: it is the only permission that unlocks an
// answer key, and 'manager' deliberately does not hold it.
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

// resolveCaller assembles the Caller. The caller's own employee id is resolved
// here, once, so the service can answer "is this YOUR enrollment" without a
// second lookup on every call.
func (h *Handler) resolveCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.enrollments")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.enrollments", "manage")
	if err != nil {
		return Caller{}, err
	}
	canGrade, err := h.authz.Can(c.Context(), userID, orgID, "hrm.enrollments", "grade")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage, CanGrade: canGrade}, nil
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

// err maps every sentinel this package raises onto an HTTP response. Anything
// unmapped is logged and 500s rather than leaking an internal message.
func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	// ── Not found ────────────────────────────────────────────────────────
	case errors.Is(err, ErrCourseNotFound):
		return response.NotFound(c, "COURSE_NOT_FOUND", "Course not found")
	case errors.Is(err, ErrVersionNotFound):
		return response.NotFound(c, "COURSE_VERSION_NOT_FOUND", "Course version not found")
	case errors.Is(err, ErrModuleNotFound):
		return response.NotFound(c, "MODULE_NOT_FOUND", "Course module not found")
	case errors.Is(err, ErrLessonNotFound):
		return response.NotFound(c, "LESSON_NOT_FOUND", "Lesson not found")
	case errors.Is(err, ErrEnrollmentNotFound):
		return response.NotFound(c, "ENROLLMENT_NOT_FOUND", "Enrollment not found")
	case errors.Is(err, ErrAttemptNotFound):
		return response.NotFound(c, "ATTEMPT_NOT_FOUND", "Quiz attempt not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")

	// ── Forbidden ────────────────────────────────────────────────────────
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ENROLLMENT_ACCESS_DENIED", err.Error())
	case errors.Is(err, ErrNotLearner):
		return response.Forbidden(c, "NOT_LEARNER", err.Error())
	case errors.Is(err, ErrGradeDenied):
		return response.Forbidden(c, "GRADE_DENIED", err.Error())
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	// ── Conflict ─────────────────────────────────────────────────────────
	case errors.Is(err, ErrTitleTaken):
		return response.Conflict(c, "COURSE_TITLE_TAKEN", err.Error())
	case errors.Is(err, ErrVersionNotEditable):
		return response.Conflict(c, "VERSION_NOT_EDITABLE", err.Error())
	case errors.Is(err, ErrVersionNotPublished):
		return response.Conflict(c, "VERSION_NOT_PUBLISHED", err.Error())
	case errors.Is(err, ErrDraftExists):
		return response.Conflict(c, "DRAFT_VERSION_EXISTS", err.Error())
	case errors.Is(err, ErrVersionHasNoLessons):
		return response.Conflict(c, "VERSION_HAS_NO_LESSONS", err.Error())
	case errors.Is(err, ErrVersionInUse):
		return response.Conflict(c, "VERSION_IN_USE", err.Error())
	case errors.Is(err, ErrCourseInactive):
		return response.Conflict(c, "COURSE_INACTIVE", err.Error())
	case errors.Is(err, ErrAlreadyEnrolled):
		return response.Conflict(c, "ALREADY_ENROLLED", err.Error())
	case errors.Is(err, ErrEnrollmentClosed):
		return response.Conflict(c, "ENROLLMENT_CLOSED", err.Error())
	case errors.Is(err, ErrAttemptsExhausted):
		return response.Conflict(c, "ATTEMPTS_EXHAUSTED", err.Error())
	case errors.Is(err, ErrAttemptSubmitted):
		return response.Conflict(c, "ATTEMPT_ALREADY_SUBMITTED", err.Error())

	// ── Bad request ──────────────────────────────────────────────────────
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", err.Error())
	case errors.Is(err, ErrInvalidLessonType):
		return response.BadRequest(c, "INVALID_LESSON_TYPE", err.Error())
	case errors.Is(err, ErrInvalidStatus):
		return response.BadRequest(c, "INVALID_STATUS", err.Error())
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", err.Error())
	case errors.Is(err, ErrPassMarkRange):
		return response.BadRequest(c, "INVALID_PASS_MARK", err.Error())
	case errors.Is(err, ErrThresholdRange):
		return response.BadRequest(c, "INVALID_PASS_THRESHOLD", err.Error())
	case errors.Is(err, ErrNotAQuiz):
		return response.BadRequest(c, "NOT_A_QUIZ", err.Error())
	case errors.Is(err, ErrQuizNotConfigured):
		return response.BadRequest(c, "QUIZ_NOT_CONFIGURED", err.Error())
	case errors.Is(err, ErrNoAnswerKey):
		return response.BadRequest(c, "INVALID_ANSWER_KEY", err.Error())
	case errors.Is(err, ErrLessonNotInCourse):
		return response.BadRequest(c, "LESSON_NOT_IN_COURSE", err.Error())
	}

	log.Error("learning error", "error", err)
	return response.InternalServerError(c)
}

// ── Courses ──────────────────────────────────────────────────────────────────

// ListCourses godoc
//
//	@Summary	List the course catalogue
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId	path		string	true	"Organization ID"
//	@Success	200		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/courses [get]
func (h *Handler) ListCourses(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListCourses(c.Context(), orgID, CourseListFilter{
		Category: c.Query("category"),
		IsActive: boolQuery(c, "is_active"),
		Search:   c.Query("search"),
		Limit:    atoiOr(c.Query("limit"), 0),
		Offset:   atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Courses retrieved")
}

// GetCourse godoc
//
//	@Summary	Get a course with its versions
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		courseId	path		string	true	"Course ID"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/courses/{courseId} [get]
func (h *Handler) GetCourse(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.GetCourse(c.Context(), orgID, c.Params("courseId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"course": out}, "Course retrieved")
}

// CreateCourse godoc
//
//	@Summary	Create a course
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId	path		string				true	"Organization ID"
//	@Param		body	body		CreateCourseRequest	true	"Course"
//	@Success	201		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/courses [post]
func (h *Handler) CreateCourse(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateCourseRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateCourse(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"course": out}, "Course created")
}

// UpdateCourse godoc
//
//	@Summary	Update a course
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string				true	"Organization ID"
//	@Param		courseId	path		string				true	"Course ID"
//	@Param		body		body		UpdateCourseRequest	true	"Changes"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/courses/{courseId} [patch]
func (h *Handler) UpdateCourse(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateCourseRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateCourse(c.Context(), orgID, c.Params("courseId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"course": out}, "Course updated")
}

// DeleteCourse godoc
//
//	@Summary	Delete a course with no enrollments
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path	string	true	"Organization ID"
//	@Param		courseId	path	string	true	"Course ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/learning/courses/{courseId} [delete]
func (h *Handler) DeleteCourse(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteCourse(c.Context(), orgID, c.Params("courseId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ── Versions ─────────────────────────────────────────────────────────────────

// ListVersions godoc
//
//	@Summary	List a course's versions
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		courseId	path		string	true	"Course ID"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/courses/{courseId}/versions [get]
func (h *Handler) ListVersions(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.ListVersions(c.Context(), orgID, c.Params("courseId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"versions": out}, "Versions retrieved")
}

// CreateVersion godoc
//
//	@Summary		Open a new draft version
//	@Description	Content is copied from copy_from_version_id when given — the normal path, since a new version is usually an edit of the last.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			courseId	path		string					true	"Course ID"
//	@Param			body		body		CreateVersionRequest	true	"Version"
//	@Success		201			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/courses/{courseId}/versions [post]
func (h *Handler) CreateVersion(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateVersionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateVersion(c.Context(), orgID, c.Params("courseId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"version": out}, "Draft version created")
}

// GetVersion godoc
//
//	@Summary	Get a version with its modules and lessons
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		versionId	path		string	true	"Version ID"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/versions/{versionId} [get]
func (h *Handler) GetVersion(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.GetVersion(c.Context(), orgID, c.Params("versionId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"version": out}, "Version retrieved")
}

// UpdateVersion godoc
//
//	@Summary		Update a DRAFT version
//	@Description	Published versions are immutable — publish a new version instead.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			versionId	path		string					true	"Version ID"
//	@Param			body		body		UpdateVersionRequest	true	"Changes"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/versions/{versionId} [patch]
func (h *Handler) UpdateVersion(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateVersionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateVersion(c.Context(), orgID, c.Params("versionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"version": out}, "Version updated")
}

// PublishVersion godoc
//
//	@Summary	Publish a draft version, making it enrollable and immutable
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		versionId	path		string	true	"Version ID"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/versions/{versionId}/publish [post]
func (h *Handler) PublishVersion(c fiber.Ctx) error {
	userID, _ := middleware.UserIDFromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.PublishVersion(c.Context(), orgID, c.Params("versionId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"version": out}, "Version published")
}

// ArchiveVersion godoc
//
//	@Summary		Archive a published version
//	@Description	Stops new enrollments. Existing enrollments stay pinned to it.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			versionId	path		string	true	"Version ID"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/versions/{versionId}/archive [post]
func (h *Handler) ArchiveVersion(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	out, err := h.service.ArchiveVersion(c.Context(), orgID, c.Params("versionId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"version": out}, "Version archived")
}

// DeleteVersion godoc
//
//	@Summary	Delete a version with no enrollments
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path	string	true	"Organization ID"
//	@Param		versionId	path	string	true	"Version ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/learning/versions/{versionId} [delete]
func (h *Handler) DeleteVersion(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteVersion(c.Context(), orgID, c.Params("versionId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// CreateModule godoc
//
//	@Summary	Add a module to a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string				true	"Organization ID"
//	@Param		versionId	path		string				true	"Version ID"
//	@Param		body		body		CreateModuleRequest	true	"Module"
//	@Success	201			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/versions/{versionId}/modules [post]
func (h *Handler) CreateModule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateModule(c.Context(), orgID, c.Params("versionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"module": out}, "Module created")
}

// UpdateModule godoc
//
//	@Summary	Update a module on a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string				true	"Organization ID"
//	@Param		moduleId	path		string				true	"Module ID"
//	@Param		body		body		UpdateModuleRequest	true	"Changes"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/modules/{moduleId} [patch]
func (h *Handler) UpdateModule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateModuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateModule(c.Context(), orgID, c.Params("moduleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"module": out}, "Module updated")
}

// DeleteModule godoc
//
//	@Summary	Delete a module from a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path	string	true	"Organization ID"
//	@Param		moduleId	path	string	true	"Module ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/learning/modules/{moduleId} [delete]
func (h *Handler) DeleteModule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteModule(c.Context(), orgID, c.Params("moduleId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// CreateLesson godoc
//
//	@Summary	Add a lesson to a module on a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string				true	"Organization ID"
//	@Param		moduleId	path		string				true	"Module ID"
//	@Param		body		body		CreateLessonRequest	true	"Lesson"
//	@Success	201			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/modules/{moduleId}/lessons [post]
func (h *Handler) CreateLesson(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLessonRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateLesson(c.Context(), orgID, c.Params("moduleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"lesson": out}, "Lesson created")
}

// UpdateLesson godoc
//
//	@Summary	Update a lesson on a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string				true	"Organization ID"
//	@Param		lessonId	path		string				true	"Lesson ID"
//	@Param		body		body		UpdateLessonRequest	true	"Changes"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/lessons/{lessonId} [patch]
func (h *Handler) UpdateLesson(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateLessonRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateLesson(c.Context(), orgID, c.Params("lessonId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"lesson": out}, "Lesson updated")
}

// DeleteLesson godoc
//
//	@Summary	Delete a lesson from a draft version
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path	string	true	"Organization ID"
//	@Param		lessonId	path	string	true	"Lesson ID"
//	@Success	204
//	@Router		/organizations/{orgId}/hrm/learning/lessons/{lessonId} [delete]
func (h *Handler) DeleteLesson(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteLesson(c.Context(), orgID, c.Params("lessonId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// SetAnswerKey godoc
//
//	@Summary		Set the correct answer for one quiz question
//	@Description	Authoring-side only. The key is stored in hrm_quiz_answer_keys and is never served to a learner.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			lessonId	path		string				true	"Quiz lesson ID"
//	@Param			body		body		SetAnswerKeyRequest	true	"Answer key"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/learning/lessons/{lessonId}/answer-keys [post]
func (h *Handler) SetAnswerKey(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req SetAnswerKeyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if err := h.service.SetAnswerKey(c.Context(), orgID, c.Params("lessonId"), req); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// GetAnswerKeys godoc
//
//	@Summary		Read a quiz's answer keys
//	@Description	The ONE endpoint that returns correct answers. Requires hrm.enrollments.grade, which 'manager' deliberately does not hold.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			lessonId	path		string	true	"Quiz lesson ID"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/lessons/{lessonId}/answer-keys [get]
func (h *Handler) GetAnswerKeys(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	keys, err := h.service.GetAnswerKeys(c.Context(), orgID, c.Params("lessonId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"answer_keys": keys}, "Answer keys retrieved")
}

// ── Enrollments ──────────────────────────────────────────────────────────────

// ListEnrollments godoc
//
//	@Summary	List enrollments, filtered by the caller's scope tier
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId		path		string	true	"Organization ID"
//	@Param		employee_id	query		string	false	"Filter by employee"
//	@Param		course_id	query		string	false	"Filter by course"
//	@Param		status		query		string	false	"assigned | in_progress | completed | failed | cancelled"
//	@Param		overdue		query		bool	false	"Only past-due, still-open enrollments"
//	@Success	200			{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/enrollments [get]
func (h *Handler) ListEnrollments(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	res, err := h.service.ListEnrollments(c.Context(), orgID, caller, EnrollmentListFilter{
		EmployeeID: c.Query("employee_id"),
		CourseID:   c.Query("course_id"),
		Status:     c.Query("status"),
		Overdue:    c.Query("overdue") == "true",
		Limit:      atoiOr(c.Query("limit"), 0),
		Offset:     atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Enrollments retrieved")
}

// GetEnrollment godoc
//
//	@Summary		Get an enrollment with its PINNED version and computed progress
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			enrollmentId	path		string	true	"Enrollment ID"
//	@Success		200				{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/enrollments/{enrollmentId} [get]
func (h *Handler) GetEnrollment(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	out, err := h.service.GetEnrollment(c.Context(), orgID, c.Params("enrollmentId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"enrollment": out}, "Enrollment retrieved")
}

// Enroll godoc
//
//	@Summary	Assign an employee to a course
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId	path		string			true	"Organization ID"
//	@Param		body	body		EnrollRequest	true	"Enrollment"
//	@Success	201		{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/enrollments [post]
func (h *Handler) Enroll(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req EnrollRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.Enroll(c.Context(), orgID, caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"enrollment": out}, "Enrollment created")
}

// SelfEnroll godoc
//
//	@Summary		Enrol yourself on a course
//	@Description	Uses the CALLER's own employee record; the body carries no employee_id.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		SelfEnrollRequest	true	"Course"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/enrollments/self [post]
func (h *Handler) SelfEnroll(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req SelfEnrollRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.SelfEnroll(c.Context(), orgID, caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"enrollment": out}, "Enrolled")
}

// UpdateEnrollment godoc
//
//	@Summary	Update an enrollment's due date
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId			path		string					true	"Organization ID"
//	@Param		enrollmentId	path		string					true	"Enrollment ID"
//	@Param		body			body		UpdateEnrollmentRequest	true	"Changes"
//	@Success	200				{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/enrollments/{enrollmentId} [patch]
func (h *Handler) UpdateEnrollment(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req UpdateEnrollmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.UpdateEnrollment(c.Context(), orgID, c.Params("enrollmentId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"enrollment": out}, "Enrollment updated")
}

// CancelEnrollment godoc
//
//	@Summary	Cancel an enrollment
//	@Tags		HRM - Learning
//	@Security	BearerAuth
//	@Param		orgId			path		string	true	"Organization ID"
//	@Param		enrollmentId	path		string	true	"Enrollment ID"
//	@Success	200				{object}	response.Response
//	@Router		/organizations/{orgId}/hrm/learning/enrollments/{enrollmentId}/cancel [post]
func (h *Handler) CancelEnrollment(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	out, err := h.service.CancelEnrollment(c.Context(), orgID, c.Params("enrollmentId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"enrollment": out}, "Enrollment cancelled")
}

// MarkLesson godoc
//
//	@Summary		Record progress on a non-quiz lesson
//	@Description	A quiz lesson is completed by PASSING an attempt, never by asserting completion.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId			path		string				true	"Organization ID"
//	@Param			enrollmentId	path		string				true	"Enrollment ID"
//	@Param			lessonId		path		string				true	"Lesson ID"
//	@Param			body			body		MarkLessonRequest	true	"Progress"
//	@Success		200				{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/enrollments/{enrollmentId}/lessons/{lessonId} [post]
func (h *Handler) MarkLesson(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req MarkLessonRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.MarkLesson(c.Context(), orgID,
		c.Params("enrollmentId"), c.Params("lessonId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"enrollment": out}, "Progress recorded")
}

// StartAttempt godoc
//
//	@Summary		Start or resume a quiz attempt
//	@Description	Returns the questions WITHOUT correct answers. Resumes an open attempt rather than burning a retry.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			enrollmentId	path		string	true	"Enrollment ID"
//	@Param			lessonId		path		string	true	"Quiz lesson ID"
//	@Success		200				{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/enrollments/{enrollmentId}/lessons/{lessonId}/attempts [post]
func (h *Handler) StartAttempt(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	out, err := h.service.StartAttempt(c.Context(), orgID,
		c.Params("enrollmentId"), c.Params("lessonId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"attempt": out}, "Attempt ready")
}

// SubmitAttempt godoc
//
//	@Summary		Submit and grade a quiz attempt
//	@Description	Grades against the answer key server-side and FREEZES the result; the grade is never re-derived.
//	@Tags			HRM - Learning
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			attemptId	path		string					true	"Attempt ID"
//	@Param			body		body		SubmitAttemptRequest	true	"Answers"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/learning/attempts/{attemptId}/submit [post]
func (h *Handler) SubmitAttempt(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req SubmitAttemptRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.SubmitAttempt(c.Context(), orgID, c.Params("attemptId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"attempt": out}, "Attempt graded")
}
