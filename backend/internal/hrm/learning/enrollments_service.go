// backend/internal/hrm/learning/enrollments_service.go
package learning

import (
	"context"
	"fmt"
	"strings"
)

// ── Reads ────────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListEnrollments(ctx context.Context, orgID string, caller Caller, f EnrollmentListFilter) (*EnrollmentListResponse, error) {
	f.Normalise()
	f.Scope = caller.Tier
	f.CallerUserID = caller.UserID

	list, err := s.repo.FindEnrollments(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountEnrollments(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	return &EnrollmentListResponse{Enrollments: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *serviceImpl) GetEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*EnrollmentDetail, error) {
	e, err := s.loadEnrollment(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	return s.hydrateEnrollment(ctx, orgID, e)
}

// loadEnrollment fetches org-scoped, then narrows by the caller's scope tier
// against the enrollment's own learner. The list path filters in SQL; this is
// the fetch-by-id half of the same control.
func (s *serviceImpl) loadEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*Enrollment, error) {
	e, err := s.repo.FindEnrollmentByRef(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, ErrEnrollmentNotFound
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, e.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("learning: authorize: %w", err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}
	return e, nil
}

// hydrateEnrollment attaches the PINNED version's content and the computed
// completion figures.
//
// Completion is computed on every read from lesson progress, never stored —
// migration 00076's rule. The denominator counts REQUIRED lessons only, and
// so does the numerator, which is what stops an optional lesson pushing a
// course past 100%.
func (s *serviceImpl) hydrateEnrollment(ctx context.Context, orgID string, e *Enrollment) (*EnrollmentDetail, error) {
	detail := &EnrollmentDetail{Enrollment: e}

	// The pinned version, not the course's current one. This is the whole
	// point of version_id.
	v, err := s.repo.FindVersionByRef(ctx, orgID, e.VersionID)
	if err != nil {
		return nil, err
	}
	if v != nil {
		if err := s.hydrateVersion(ctx, v); err != nil {
			return nil, err
		}
		detail.Version = v
	}

	progress, err := s.repo.FindProgress(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = []*LessonProgress{}
	}
	detail.Progress = progress

	attempts, err := s.repo.FindAttempts(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	detail.Attempts = attempts

	required, err := s.repo.CountRequiredLessons(ctx, e.VersionID)
	if err != nil {
		return nil, err
	}
	completed, err := s.repo.CountCompletedRequired(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	detail.RequiredLessons = required
	detail.CompletedLessons = completed
	detail.CompletionPercent = CompletionPercent(completed, required)

	return detail, nil
}

// ── Writes ───────────────────────────────────────────────────────────────────

// authorizeAssign narrows an assignment. hrm.enrollments.manage is unscoped at
// the route, so this record check is the only thing stopping a view_team
// manager assigning training to somebody outside their reporting line.
func (s *serviceImpl) authorizeAssign(ctx context.Context, orgID, employeeID string, caller Caller) error {
	if !caller.CanManage {
		return ErrAccessDenied
	}
	ok, err := s.records.AuthorizeRecordAccess(ctx, caller.Tier, orgID, caller.UserID, employeeID)
	if err != nil {
		return fmt.Errorf("learning: authorize assign: %w", err)
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

func (s *serviceImpl) Enroll(ctx context.Context, orgID string, caller Caller, req EnrollRequest) (*EnrollmentDetail, error) {
	emp, err := s.repo.FindEmployeeRef(ctx, orgID, strings.TrimSpace(req.EmployeeID))
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}
	if err := s.authorizeAssign(ctx, orgID, emp.EmployeeID, caller); err != nil {
		return nil, err
	}
	return s.enroll(ctx, orgID, caller, emp.EmployeeID, req.CourseID, req.VersionID, req.DueDate, AssignedManual)
}

// SelfEnroll uses the CALLER's own employee id, never one from the request
// body. That is what stops hrm.enrollments.enroll_self — which reaches every
// member — being an assignment permission in disguise.
func (s *serviceImpl) SelfEnroll(ctx context.Context, orgID string, caller Caller, req SelfEnrollRequest) (*EnrollmentDetail, error) {
	employeeID := caller.EmployeeID
	if employeeID == "" {
		resolved, err := s.repo.FindEmployeeIDByUserID(ctx, orgID, caller.UserID)
		if err != nil {
			return nil, err
		}
		if resolved == "" {
			return nil, ErrEmployeeNotFound
		}
		employeeID = resolved
	}
	return s.enroll(ctx, orgID, caller, employeeID, req.CourseID, nil, req.DueDate, AssignedSelf)
}

// enroll is the shared path. It resolves the version to pin and refuses to
// enrol against anything but a published one.
func (s *serviceImpl) enroll(ctx context.Context, orgID string, caller Caller, employeeID, courseRef string, versionRef, dueDate *string, via AssignedVia) (*EnrollmentDetail, error) {
	c, err := s.loadCourse(ctx, orgID, strings.TrimSpace(courseRef))
	if err != nil {
		return nil, err
	}
	if !c.IsActive {
		return nil, ErrCourseInactive
	}

	// Resolve the version to PIN. Naming one explicitly supports deliberately
	// re-assigning somebody onto an older version; omitting it takes the
	// current published one, which is the normal path.
	var v *CourseVersion
	if versionRef != nil && strings.TrimSpace(*versionRef) != "" {
		v, err = s.loadVersion(ctx, orgID, strings.TrimSpace(*versionRef))
		if err != nil {
			return nil, err
		}
		if v.CourseID != c.ID {
			return nil, ErrVersionNotFound
		}
	} else {
		v, err = s.repo.FindPublishedVersion(ctx, orgID, c.ID)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, ErrVersionNotPublished
		}
	}
	// A draft has no business being enrolled against: its content is still
	// changing, which is exactly what the pin exists to prevent.
	if v.Status != VersionPublished {
		return nil, ErrVersionNotPublished
	}

	// uq_hrm_enr_employee_course_live is the guarantee; this is the message.
	live, err := s.repo.HasLiveEnrollment(ctx, orgID, employeeID, c.ID)
	if err != nil {
		return nil, err
	}
	if live {
		return nil, ErrAlreadyEnrolled
	}

	due, err := parseDate(dueDate)
	if err != nil {
		return nil, err
	}

	e := &Enrollment{
		OrgID: orgID, EmployeeID: employeeID, CourseID: c.ID, VersionID: v.ID,
		AssignedVia: via, DueDate: due,
	}
	if caller.UserID != "" {
		e.AssignedBy = &caller.UserID
	}
	if err := s.repo.CreateEnrollment(ctx, e); err != nil {
		return nil, err
	}
	return s.hydrateEnrollment(ctx, orgID, e)
}

func (s *serviceImpl) UpdateEnrollment(ctx context.Context, orgID, ref string, caller Caller, req UpdateEnrollmentRequest) (*EnrollmentDetail, error) {
	e, err := s.loadEnrollment(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAssign(ctx, orgID, e.EmployeeID, caller); err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrEnrollmentClosed
	}

	if req.DueDate != nil {
		due, err := parseDate(req.DueDate)
		if err != nil {
			return nil, err
		}
		e.DueDate = due
	}
	if err := s.repo.UpdateEnrollment(ctx, e); err != nil {
		return nil, err
	}
	return s.hydrateEnrollment(ctx, orgID, e)
}

func (s *serviceImpl) CancelEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*EnrollmentDetail, error) {
	e, err := s.loadEnrollment(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAssign(ctx, orgID, e.EmployeeID, caller); err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrEnrollmentClosed
	}
	out, err := s.repo.SetEnrollmentStatus(ctx, orgID, e.ID, EnrollmentCancelled)
	if err != nil {
		return nil, err
	}
	return s.hydrateEnrollment(ctx, orgID, out)
}

// ── Lesson progress ──────────────────────────────────────────────────────────

// MarkLesson records progress on a NON-QUIZ lesson.
//
// A quiz lesson is completed by passing an attempt, never by asserting
// completion — otherwise the assessment is optional, which is the same as
// absent.
func (s *serviceImpl) MarkLesson(ctx context.Context, orgID, enrollmentRef, lessonRef string, caller Caller, req MarkLessonRequest) (*EnrollmentDetail, error) {
	e, err := s.loadOwnEnrollment(ctx, orgID, enrollmentRef, caller)
	if err != nil {
		return nil, err
	}
	if e.Status.IsTerminal() {
		return nil, ErrEnrollmentClosed
	}

	l, err := s.lessonInEnrollment(ctx, orgID, e, lessonRef)
	if err != nil {
		return nil, err
	}
	if l.LessonType == LessonQuiz {
		return nil, fmt.Errorf("%w: a quiz is completed by passing an attempt", ErrNotAQuiz)
	}

	status := ProgressStatus(strings.TrimSpace(req.Status))
	if !status.IsValid() {
		return nil, ErrInvalidStatus
	}

	if err := s.repo.MarkEnrollmentStarted(ctx, orgID, e.ID); err != nil {
		return nil, err
	}
	if _, err := s.repo.UpsertProgress(ctx, e.ID, l.ID, status); err != nil {
		return nil, err
	}
	if err := s.maybeComplete(ctx, orgID, e); err != nil {
		return nil, err
	}

	fresh, err := s.repo.FindEnrollmentByRef(ctx, orgID, e.ID)
	if err != nil {
		return nil, err
	}
	if fresh == nil {
		return nil, ErrEnrollmentNotFound
	}
	return s.hydrateEnrollment(ctx, orgID, fresh)
}

// loadOwnEnrollment narrows to the enrollment's own learner. Assigning
// managers reach enrollments through the scope tier; PROGRESSING through one
// is the learner's act alone, and hrm.enrollments.attempt reaches every
// member, so the route gate cannot express "is this YOUR enrollment".
//
// A caller holding manage may also act, which is what lets HR record an
// offline completion — but it is a deliberate branch, not an accident.
func (s *serviceImpl) loadOwnEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*Enrollment, error) {
	e, err := s.loadEnrollment(ctx, orgID, ref, caller)
	if err != nil {
		return nil, err
	}
	if caller.CanManage {
		return e, nil
	}
	if caller.EmployeeID == "" || caller.EmployeeID != e.EmployeeID {
		return nil, ErrNotLearner
	}
	return e, nil
}

// lessonInEnrollment resolves a lesson AND proves it belongs to the
// enrollment's pinned version. Without that second half, a learner could mark
// a lesson from an entirely different course complete and move their own
// progress bar.
func (s *serviceImpl) lessonInEnrollment(ctx context.Context, orgID string, e *Enrollment, lessonRef string) (*Lesson, error) {
	l, err := s.loadLesson(ctx, orgID, strings.TrimSpace(lessonRef))
	if err != nil {
		return nil, err
	}
	lessons, err := s.repo.FindLessons(ctx, e.VersionID)
	if err != nil {
		return nil, err
	}
	for _, cand := range lessons {
		if cand.ID == l.ID {
			return l, nil
		}
	}
	return nil, ErrLessonNotInCourse
}

// maybeComplete flips the enrollment to completed once every required lesson
// is done. Computed from the same counts the detail view reports, so the two
// can never disagree.
func (s *serviceImpl) maybeComplete(ctx context.Context, orgID string, e *Enrollment) error {
	required, err := s.repo.CountRequiredLessons(ctx, e.VersionID)
	if err != nil {
		return err
	}
	if required == 0 {
		// An empty version does not auto-complete; that would mark every
		// learner done on a course with nothing in it.
		return nil
	}
	completed, err := s.repo.CountCompletedRequired(ctx, e.ID)
	if err != nil {
		return err
	}

	v, err := s.repo.FindVersionByRef(ctx, orgID, e.VersionID)
	if err != nil {
		return err
	}
	threshold := CompletionPercent(completed, required)
	if v != nil && threshold.LessThan(v.PassThreshold) {
		return nil
	}
	return s.repo.MarkEnrollmentCompleted(ctx, orgID, e.ID)
}
