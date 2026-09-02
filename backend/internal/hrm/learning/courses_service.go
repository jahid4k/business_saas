// backend/internal/hrm/learning/courses_service.go
package learning

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// ── Courses ──────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListCourses(ctx context.Context, orgID string, f CourseListFilter) (*CourseListResponse, error) {
	f.Normalise()
	courses, err := s.repo.FindCourses(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountCourses(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &CourseListResponse{Courses: courses, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

// GetCourse hydrates the version list, which is what a catalogue page needs to
// show "v3, published" without a second round trip.
func (s *serviceImpl) GetCourse(ctx context.Context, orgID, ref string) (*Course, error) {
	c, err := s.loadCourse(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.FindVersions(ctx, orgID, c.ID)
	if err != nil {
		return nil, err
	}
	c.Versions = versions
	return c, nil
}

func (s *serviceImpl) loadCourse(ctx context.Context, orgID, ref string) (*Course, error) {
	c, err := s.repo.FindCourseByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrCourseNotFound
	}
	return c, nil
}

func (s *serviceImpl) CreateCourse(ctx context.Context, orgID, createdBy string, req CreateCourseRequest) (*Course, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	taken, err := s.repo.CourseTitleExists(ctx, orgID, title, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrTitleTaken
	}

	c := &Course{
		OrgID: orgID, Title: title,
		Description: nilIfBlank(req.Description), Category: nilIfBlank(req.Category),
		CreatedBy: createdBy,
	}
	if err := s.repo.CreateCourse(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) UpdateCourse(ctx context.Context, orgID, ref string, req UpdateCourseRequest) (*Course, error) {
	c, err := s.loadCourse(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		taken, err := s.repo.CourseTitleExists(ctx, orgID, title, c.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrTitleTaken
		}
		c.Title = title
	}
	if req.Description != nil {
		c.Description = nilIfBlank(req.Description)
	}
	if req.Category != nil {
		c.Category = nilIfBlank(req.Category)
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}

	// Note what is absent: nothing here touches a published version's content.
	// Renaming the course does not rewrite title_snapshot on versions already
	// published — those record what the course was called at the time.
	if err := s.repo.UpdateCourse(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *serviceImpl) DeleteCourse(ctx context.Context, orgID, ref string) error {
	c, err := s.loadCourse(ctx, orgID, ref)
	if err != nil {
		return err
	}
	// hrm_enrollments.course_id is ON DELETE RESTRICT, so Postgres refuses
	// this anyway; checking first turns a raw 23503 into a usable message.
	versions, err := s.repo.FindVersions(ctx, orgID, c.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		inUse, err := s.repo.VersionHasEnrollments(ctx, v.ID)
		if err != nil {
			return err
		}
		if inUse {
			return ErrVersionInUse
		}
	}
	return s.repo.DeleteCourse(ctx, orgID, c.ID)
}

// ── Versions ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListVersions(ctx context.Context, orgID, courseRef string) ([]*CourseVersion, error) {
	c, err := s.loadCourse(ctx, orgID, courseRef)
	if err != nil {
		return nil, err
	}
	return s.repo.FindVersions(ctx, orgID, c.ID)
}

// GetVersion hydrates modules and their lessons — the authoring view, and
// also what an enrolled learner reads through their pinned version.
func (s *serviceImpl) GetVersion(ctx context.Context, orgID, ref string) (*CourseVersion, error) {
	v, err := s.loadVersion(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if err := s.hydrateVersion(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *serviceImpl) loadVersion(ctx context.Context, orgID, ref string) (*CourseVersion, error) {
	v, err := s.repo.FindVersionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrVersionNotFound
	}
	return v, nil
}

// hydrateVersion attaches modules with their lessons nested underneath.
func (s *serviceImpl) hydrateVersion(ctx context.Context, v *CourseVersion) error {
	modules, err := s.repo.FindModules(ctx, v.ID)
	if err != nil {
		return err
	}
	lessons, err := s.repo.FindLessons(ctx, v.ID)
	if err != nil {
		return err
	}
	byModule := map[string][]*Lesson{}
	for _, l := range lessons {
		byModule[l.ModuleID] = append(byModule[l.ModuleID], l)
	}
	for _, m := range modules {
		m.Lessons = byModule[m.ID]
		if m.Lessons == nil {
			m.Lessons = []*Lesson{}
		}
	}
	v.Modules = modules
	return nil
}

// CreateVersion opens a new draft, optionally copying content from an existing
// version — the normal path, since a new version is usually an edit of the
// last one rather than a blank page.
func (s *serviceImpl) CreateVersion(ctx context.Context, orgID, courseRef, createdBy string, req CreateVersionRequest) (*CourseVersion, error) {
	c, err := s.loadCourse(ctx, orgID, courseRef)
	if err != nil {
		return nil, err
	}

	// uq_hrm_crsv_one_draft enforces this too; checking first names the
	// problem instead of surfacing a 23505. Two concurrent drafts would make
	// "the next version" ambiguous and let two authors overwrite each other.
	existing, err := s.repo.FindVersions(ctx, orgID, c.ID)
	if err != nil {
		return nil, err
	}
	for _, v := range existing {
		if v.Status == VersionDraft {
			return nil, ErrDraftExists
		}
	}

	threshold := decimal.NewFromInt(100)
	if req.PassThreshold != nil {
		if req.PassThreshold.LessThanOrEqual(decimal.Zero) ||
			req.PassThreshold.GreaterThan(decimal.NewFromInt(100)) {
			return nil, ErrThresholdRange
		}
		threshold = *req.PassThreshold
	}

	number, err := s.repo.NextVersionNumber(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	v := &CourseVersion{
		OrgID: orgID, CourseID: c.ID, VersionNumber: number,
		// Snapshotted now so an archived version still renders under the title
		// it was published as, even after the course is renamed.
		TitleSnapshot: c.Title,
		ChangeNote:    nilIfBlank(req.ChangeNote),
		PassThreshold: threshold,
		CreatedBy:     createdBy,
	}
	if err := s.repo.CreateVersion(ctx, v); err != nil {
		return nil, err
	}

	if req.CopyFromVersionID != nil && strings.TrimSpace(*req.CopyFromVersionID) != "" {
		src, err := s.loadVersion(ctx, orgID, strings.TrimSpace(*req.CopyFromVersionID))
		if err != nil {
			return nil, err
		}
		if src.CourseID != c.ID {
			return nil, ErrVersionNotFound
		}
		if err := s.repo.CopyVersionContent(ctx, src.ID, v.ID); err != nil {
			return nil, err
		}
	}

	if err := s.hydrateVersion(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *serviceImpl) UpdateVersion(ctx context.Context, orgID, ref string, req UpdateVersionRequest) (*CourseVersion, error) {
	v, err := s.loadVersion(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	// The version-pinning guard. Editing a published version would change what
	// an already-enrolled learner is being assessed on.
	if !v.Status.IsEditable() {
		return nil, ErrVersionNotEditable
	}

	if req.ChangeNote != nil {
		v.ChangeNote = nilIfBlank(req.ChangeNote)
	}
	if req.PassThreshold != nil {
		if req.PassThreshold.LessThanOrEqual(decimal.Zero) ||
			req.PassThreshold.GreaterThan(decimal.NewFromInt(100)) {
			return nil, ErrThresholdRange
		}
		v.PassThreshold = *req.PassThreshold
	}
	if err := s.repo.UpdateVersion(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// PublishVersion makes a draft enrollable and freezes its content.
//
// Publishing does NOT archive the previous version: existing enrollments stay
// pinned to it and must keep resolving their content. Archiving is a separate,
// deliberate act.
func (s *serviceImpl) PublishVersion(ctx context.Context, orgID, ref, actorID string) (*CourseVersion, error) {
	v, err := s.loadVersion(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if v.Status != VersionDraft {
		return nil, ErrVersionNotEditable
	}

	// An empty version would complete instantly for every learner assigned it,
	// since completion is computed over required lessons.
	lessons, err := s.repo.FindLessons(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	if len(lessons) == 0 {
		return nil, ErrVersionHasNoLessons
	}

	out, err := s.repo.SetVersionStatus(ctx, orgID, v.ID, VersionPublished, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.hydrateVersion(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *serviceImpl) ArchiveVersion(ctx context.Context, orgID, ref string) (*CourseVersion, error) {
	v, err := s.loadVersion(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if v.Status != VersionPublished {
		return nil, ErrVersionNotPublished
	}
	// Archiving stops NEW enrollments. It deliberately does not touch existing
	// ones — they stay pinned here, which is the whole point of the pin.
	return s.repo.SetVersionStatus(ctx, orgID, v.ID, VersionArchived, "")
}

func (s *serviceImpl) DeleteVersion(ctx context.Context, orgID, ref string) error {
	v, err := s.loadVersion(ctx, orgID, ref)
	if err != nil {
		return err
	}
	inUse, err := s.repo.VersionHasEnrollments(ctx, v.ID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrVersionInUse
	}
	return s.repo.DeleteVersion(ctx, orgID, v.ID)
}

// ── Modules and lessons ──────────────────────────────────────────────────────

// assertEditableVersion is the single gate every content write passes through.
// One helper rather than the check repeated six times, because the one place
// it gets forgotten is the one that corrupts a published version.
func (s *serviceImpl) assertEditableVersion(ctx context.Context, orgID, versionID string) (*CourseVersion, error) {
	v, err := s.repo.FindVersionByRef(ctx, orgID, versionID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrVersionNotFound
	}
	if !v.Status.IsEditable() {
		return nil, ErrVersionNotEditable
	}
	return v, nil
}

func (s *serviceImpl) CreateModule(ctx context.Context, orgID, versionRef string, req CreateModuleRequest) (*Module, error) {
	v, err := s.assertEditableVersion(ctx, orgID, versionRef)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}

	m := &Module{
		VersionID: v.ID, Title: title, Description: nilIfBlank(req.Description),
		DisplayOrder: intOr(req.DisplayOrder, 0),
	}
	if err := s.repo.CreateModule(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *serviceImpl) UpdateModule(ctx context.Context, orgID, ref string, req UpdateModuleRequest) (*Module, error) {
	m, err := s.repo.FindModuleByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrModuleNotFound
	}
	if _, err := s.assertEditableVersion(ctx, orgID, m.VersionID); err != nil {
		return nil, err
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		m.Title = title
	}
	if req.Description != nil {
		m.Description = nilIfBlank(req.Description)
	}
	if req.DisplayOrder != nil {
		m.DisplayOrder = *req.DisplayOrder
	}
	if err := s.repo.UpdateModule(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *serviceImpl) DeleteModule(ctx context.Context, orgID, ref string) error {
	m, err := s.repo.FindModuleByRef(ctx, orgID, ref)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrModuleNotFound
	}
	if _, err := s.assertEditableVersion(ctx, orgID, m.VersionID); err != nil {
		return err
	}
	return s.repo.DeleteModule(ctx, m.ID)
}

func (s *serviceImpl) CreateLesson(ctx context.Context, orgID, moduleRef string, req CreateLessonRequest) (*Lesson, error) {
	m, err := s.repo.FindModuleByRef(ctx, orgID, moduleRef)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrModuleNotFound
	}
	if _, err := s.assertEditableVersion(ctx, orgID, m.VersionID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	lessonType := LessonType(strings.TrimSpace(req.LessonType))
	if !lessonType.IsValid() {
		return nil, ErrInvalidLessonType
	}
	if req.PassMark != nil && (req.PassMark.IsNegative() || req.PassMark.GreaterThan(decimal.NewFromInt(100))) {
		return nil, ErrPassMarkRange
	}
	// A quiz with no template has no questions to serve. The schema
	// deliberately carries no CHECK for this — a CHECK pairing lesson_type
	// with form_template_id would break DELETE on platform_form_templates,
	// since ON DELETE SET NULL is an UPDATE and Postgres re-evaluates CHECKs
	// on UPDATE (the 00076 trap). So the rule lives here.
	if lessonType == LessonQuiz && nilIfBlank(req.FormTemplateID) == nil {
		return nil, ErrQuizNotConfigured
	}

	l := &Lesson{
		ModuleID: m.ID, Title: title, LessonType: lessonType,
		ContentURL: nilIfBlank(req.ContentURL), ContentText: nilIfBlank(req.ContentText),
		FormTemplateID: nilIfBlank(req.FormTemplateID),
		PassMark:       req.PassMark, MaxAttempts: req.MaxAttempts,
		IsRequired:   boolOr(req.IsRequired, true),
		DisplayOrder: intOr(req.DisplayOrder, 0),
	}
	if err := s.repo.CreateLesson(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

func (s *serviceImpl) UpdateLesson(ctx context.Context, orgID, ref string, req UpdateLessonRequest) (*Lesson, error) {
	l, err := s.loadLesson(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	m, err := s.repo.FindModuleByRef(ctx, orgID, l.ModuleID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrModuleNotFound
	}
	if _, err := s.assertEditableVersion(ctx, orgID, m.VersionID); err != nil {
		return nil, err
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		l.Title = title
	}
	if req.ContentURL != nil {
		l.ContentURL = nilIfBlank(req.ContentURL)
	}
	if req.ContentText != nil {
		l.ContentText = nilIfBlank(req.ContentText)
	}
	if req.FormTemplateID != nil {
		l.FormTemplateID = nilIfBlank(req.FormTemplateID)
	}
	if req.PassMark != nil {
		if req.PassMark.IsNegative() || req.PassMark.GreaterThan(decimal.NewFromInt(100)) {
			return nil, ErrPassMarkRange
		}
		l.PassMark = req.PassMark
	}
	if req.MaxAttempts != nil {
		l.MaxAttempts = req.MaxAttempts
	}
	if req.IsRequired != nil {
		l.IsRequired = *req.IsRequired
	}
	if req.DisplayOrder != nil {
		l.DisplayOrder = *req.DisplayOrder
	}
	if l.LessonType == LessonQuiz && l.FormTemplateID == nil {
		return nil, ErrQuizNotConfigured
	}

	if err := s.repo.UpdateLesson(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

func (s *serviceImpl) DeleteLesson(ctx context.Context, orgID, ref string) error {
	l, err := s.loadLesson(ctx, orgID, ref)
	if err != nil {
		return err
	}
	m, err := s.repo.FindModuleByRef(ctx, orgID, l.ModuleID)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrModuleNotFound
	}
	if _, err := s.assertEditableVersion(ctx, orgID, m.VersionID); err != nil {
		return err
	}
	return s.repo.DeleteLesson(ctx, l.ID)
}

func (s *serviceImpl) loadLesson(ctx context.Context, orgID, ref string) (*Lesson, error) {
	l, err := s.repo.FindLessonByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, ErrLessonNotFound
	}
	return l, nil
}

// ── Answer keys ──────────────────────────────────────────────────────────────

// SetAnswerKey writes the key for one question on a quiz lesson's template.
// Authoring-side, gated at the route on hrm.courses.manage.
func (s *serviceImpl) SetAnswerKey(ctx context.Context, orgID, lessonRef string, req SetAnswerKeyRequest) error {
	l, err := s.loadLesson(ctx, orgID, lessonRef)
	if err != nil {
		return err
	}
	if l.LessonType != LessonQuiz {
		return ErrNotAQuiz
	}
	if l.FormTemplateID == nil {
		return ErrQuizNotConfigured
	}
	if strings.TrimSpace(req.QuestionID) == "" {
		return fmt.Errorf("%w: question_id is required", ErrNoAnswerKey)
	}

	points := decimal.NewFromInt(1)
	if req.Points != nil {
		if req.Points.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("%w: points must be greater than zero", ErrNoAnswerKey)
		}
		points = *req.Points
	}

	k := &AnswerKey{
		OrgID: orgID, QuestionID: strings.TrimSpace(req.QuestionID),
		CorrectText: nilIfBlank(req.CorrectText), CorrectNumber: req.CorrectNumber,
		CorrectBoolean: req.CorrectBoolean, CorrectOptions: req.CorrectOptions,
		Points: points, PartialCredit: boolOr(req.PartialCredit, false),
		Explanation: nilIfBlank(req.Explanation),
	}
	return s.repo.UpsertAnswerKey(ctx, k)
}

// GetAnswerKeys is the ONE read path in this package that returns correct
// answers, and it requires hrm.enrollments.grade — a permission 'manager'
// deliberately does not hold, because a manager who could read the key for
// their report's quiz has defeated the assessment.
//
// The check is here as well as at the route so the service cannot be reached
// another way.
func (s *serviceImpl) GetAnswerKeys(ctx context.Context, orgID, lessonRef string, caller Caller) (map[string]*AnswerKey, error) {
	if !caller.CanGrade {
		return nil, ErrGradeDenied
	}
	l, err := s.loadLesson(ctx, orgID, lessonRef)
	if err != nil {
		return nil, err
	}
	if l.LessonType != LessonQuiz {
		return nil, ErrNotAQuiz
	}
	if l.FormTemplateID == nil {
		return nil, ErrQuizNotConfigured
	}
	return s.repo.FindAnswerKeysForTemplate(ctx, orgID, *l.FormTemplateID)
}
