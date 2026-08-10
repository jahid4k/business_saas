// backend/internal/hrm/learning/quiz_service.go
package learning

import (
	"context"
	"fmt"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

// StartAttempt opens (or resumes) a learner's attempt at a quiz lesson.
//
// Resuming an existing ungraded attempt rather than opening a new one is what
// stops a page refresh burning a retry.
func (s *serviceImpl) StartAttempt(ctx context.Context, orgID, enrollmentRef, lessonRef string, caller Caller) (*AttemptDetail, error) {
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
	if l.LessonType != LessonQuiz {
		return nil, ErrNotAQuiz
	}
	if l.FormTemplateID == nil {
		return nil, ErrQuizNotConfigured
	}

	// Resume rather than restart.
	if open, err := s.repo.FindOpenAttempt(ctx, e.ID, l.ID); err != nil {
		return nil, err
	} else if open != nil {
		return s.hydrateAttempt(ctx, orgID, open, l)
	}

	// A passed quiz is done; re-sitting it could only lower the record.
	passed, err := s.repo.HasPassedAttempt(ctx, e.ID, l.ID)
	if err != nil {
		return nil, err
	}
	if passed {
		return nil, ErrAttemptsExhausted
	}

	used, err := s.repo.CountAttempts(ctx, e.ID, l.ID)
	if err != nil {
		return nil, err
	}
	if l.MaxAttempts != nil && used >= *l.MaxAttempts {
		return nil, ErrAttemptsExhausted
	}

	if err := s.repo.MarkEnrollmentStarted(ctx, orgID, e.ID); err != nil {
		return nil, err
	}

	a := &QuizAttempt{
		OrgID: orgID, EnrollmentID: e.ID, LessonID: l.ID,
		AttemptNumber: used + 1,
		// Frozen from the lesson now, so a later pass_mark change cannot flip
		// this attempt's result from fail to pass.
		PassMarkSnapshot: l.PassMark,
	}

	if s.forms != nil {
		emp, err := s.repo.FindEmployeeRef(ctx, orgID, e.EmployeeID)
		if err != nil {
			return nil, err
		}
		subject := forms.SubjectContext{
			SubjectType: forms.SubjectEmployee,
			SubjectID:   e.EmployeeID,
			// The learner is both subject and respondent here — unlike an
			// appraisal, where they differ. Setting both explicitly rather
			// than letting one default keeps that a stated fact.
			RespondentRole: "learner",
			CreatedBy:      caller.UserID,
		}
		if emp != nil {
			subject.SubjectLabel = emp.DisplayName
			subject.RespondentUserID = emp.UserID
		}
		inst, err := s.forms.Instantiate(ctx, orgID, *l.FormTemplateID, subject)
		if err != nil {
			return nil, fmt.Errorf("learning: instantiate quiz: %w", err)
		}
		a.FormInstanceID = &inst.ID
	}

	if err := s.repo.CreateAttempt(ctx, a); err != nil {
		return nil, err
	}
	return s.hydrateAttempt(ctx, orgID, a, l)
}

// SubmitAttempt saves the learner's answers, grades them against the key, and
// FREEZES the result.
//
// The answer key is read here and only here on the learner path — server-side,
// never returned. Grading happens once, while the questions still exist:
// platform_form_responses.question_id is ON DELETE SET NULL, so a later
// re-grade could find no key and silently score zero.
func (s *serviceImpl) SubmitAttempt(ctx context.Context, orgID, attemptRef string, caller Caller, req SubmitAttemptRequest) (*AttemptDetail, error) {
	a, err := s.repo.FindAttemptByRef(ctx, orgID, attemptRef)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAttemptNotFound
	}
	if a.IsGraded() {
		return nil, ErrAttemptSubmitted
	}

	e, err := s.loadOwnEnrollment(ctx, orgID, a.EnrollmentID, caller)
	if err != nil {
		return nil, err
	}
	l, err := s.loadLesson(ctx, orgID, a.LessonID)
	if err != nil {
		return nil, err
	}
	if l.FormTemplateID == nil {
		return nil, ErrQuizNotConfigured
	}
	if a.FormInstanceID == nil || s.forms == nil {
		return nil, ErrQuizNotConfigured
	}

	// Persist the answers through the engine, then submit so the instance is
	// closed to further edits — the learner must not be able to change an
	// answer after seeing the grade.
	if len(req.Answers) > 0 {
		if _, err := s.forms.SaveAnswers(ctx, orgID, *a.FormInstanceID, caller.UserID,
			forms.SaveAnswersRequest{Answers: req.Answers}); err != nil {
			return nil, fmt.Errorf("learning: save quiz answers: %w", err)
		}
	}
	inst, err := s.forms.SubmitInstance(ctx, orgID, *a.FormInstanceID, caller.UserID)
	if err != nil {
		return nil, fmt.Errorf("learning: submit quiz: %w", err)
	}

	keys, err := s.repo.FindAnswerKeysForTemplate(ctx, orgID, *l.FormTemplateID)
	if err != nil {
		return nil, err
	}

	result := Grade(inst.Responses, keys)
	score := result.ScorePercent
	earned := result.PointsEarned
	possible := result.PointsPossible

	// Pass mark comes from the SNAPSHOT taken when the attempt opened, not
	// from the lesson as it stands now.
	passMark := a.PassMarkSnapshot
	if passMark == nil {
		passMark = l.PassMark
	}
	passed := true
	if passMark != nil {
		passed = score.GreaterThanOrEqual(*passMark)
	}

	a.Score = &score
	a.PointsEarned = &earned
	a.PointsPossible = &possible
	a.Passed = &passed
	if err := s.repo.GradeAttempt(ctx, orgID, a); err != nil {
		return nil, err
	}

	// A passed quiz completes its lesson. A failed one leaves the lesson
	// in_progress so the learner can retry if attempts remain.
	progressStatus := ProgressInProgress
	if passed {
		progressStatus = ProgressCompleted
	}
	if _, err := s.repo.UpsertProgress(ctx, e.ID, l.ID, progressStatus); err != nil {
		return nil, err
	}
	if passed {
		if err := s.maybeComplete(ctx, orgID, e); err != nil {
			return nil, err
		}
	}

	return s.hydrateAttempt(ctx, orgID, a, l)
}

// hydrateAttempt builds the LEARNER-FACING view.
//
// ⚠ It reads the form instance and maps each response to a QuestionForAttempt,
// which has no correct-answer field. It does NOT call
// FindAnswerKeysForTemplate. That is the protection: there is no key in scope
// to forget to strip.
func (s *serviceImpl) hydrateAttempt(ctx context.Context, orgID string, a *QuizAttempt, l *Lesson) (*AttemptDetail, error) {
	detail := &AttemptDetail{QuizAttempt: a, Questions: []QuestionForAttempt{}}

	if l != nil && l.MaxAttempts != nil {
		used, err := s.repo.CountAttempts(ctx, a.EnrollmentID, a.LessonID)
		if err != nil {
			return nil, err
		}
		remaining := *l.MaxAttempts - used
		if remaining < 0 {
			remaining = 0
		}
		detail.AttemptsRemaining = &remaining
	}

	if a.FormInstanceID == nil || s.forms == nil {
		return detail, nil
	}
	inst, err := s.forms.GetInstance(ctx, orgID, *a.FormInstanceID)
	if err != nil {
		return nil, fmt.Errorf("learning: read quiz instance: %w", err)
	}
	detail.Questions = toQuestionsForAttempt(inst)
	return detail, nil
}

// toQuestionsForAttempt maps engine responses to the learner-facing DTO,
// FIELD BY FIELD rather than by embedding.
//
// The field-by-field copy is deliberate: a column added to
// platform_form_responses in a later phase cannot silently appear on this
// path. The cost is one line per new answer type; the alternative is a leak
// nobody notices. Same reasoning as feedback.stripAnswers.
func toQuestionsForAttempt(inst *forms.InstanceWithResponses) []QuestionForAttempt {
	if inst == nil {
		return []QuestionForAttempt{}
	}
	out := make([]QuestionForAttempt, 0, len(inst.Responses))
	for _, r := range inst.Responses {
		if r == nil {
			continue
		}
		out = append(out, QuestionForAttempt{
			ResponseID:   r.ID,
			QuestionText: r.QuestionText,
			QuestionType: string(r.QuestionType),
			IsRequired:   r.IsRequired,
			DisplayOrder: r.DisplayOrder,
			ScaleMin:     r.ScaleMin,
			ScaleMax:     r.ScaleMax,
			Options:      r.Options,

			AnswerText:    r.AnswerText,
			AnswerNumber:  r.AnswerNumber,
			AnswerBoolean: r.AnswerBoolean,
			AnswerOptions: r.AnswerOptions,
		})
	}
	return out
}
