// backend/internal/hrm/learning/quiz_repository.go
package learning

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AttemptRepository interface {
	FindAttempts(ctx context.Context, enrollmentID string) ([]*QuizAttempt, error)
	FindAttemptByRef(ctx context.Context, orgID, ref string) (*QuizAttempt, error)
	FindOpenAttempt(ctx context.Context, enrollmentID, lessonID string) (*QuizAttempt, error)
	CountAttempts(ctx context.Context, enrollmentID, lessonID string) (int, error)
	HasPassedAttempt(ctx context.Context, enrollmentID, lessonID string) (bool, error)
	CreateAttempt(ctx context.Context, a *QuizAttempt) error
	// GradeAttempt freezes the result. Called once, at submit.
	GradeAttempt(ctx context.Context, orgID string, a *QuizAttempt) error
}

// AnswerKeyRepository is deliberately its own interface.
//
// ⚠ Exactly two callers are legitimate: the AUTHORING path (writing a key,
// gated on hrm.courses.manage) and GRADING (reading keys server-side at
// submit). No learner-facing read may call these methods — that separation is
// the mechanism protecting the answers, not a convention. See migration
// 00092's header, and the equivalent split in internal/hrm/feedback for 360
// anonymity.
type AnswerKeyRepository interface {
	// FindAnswerKeysForTemplate returns every key for a template's questions,
	// mapped by question id, ready for Grade. Server-side only.
	FindAnswerKeysForTemplate(ctx context.Context, orgID, templateID string) (map[string]*AnswerKey, error)
	FindAnswerKeyByQuestion(ctx context.Context, orgID, questionID string) (*AnswerKey, error)
	UpsertAnswerKey(ctx context.Context, k *AnswerKey) error
	DeleteAnswerKey(ctx context.Context, orgID, questionID string) error
}

// ── Attempts ─────────────────────────────────────────────────────────────────

const attemptCols = `id, public_id, org_id, enrollment_id, lesson_id, attempt_number,
	form_instance_id, score, points_earned, points_possible, passed, pass_mark_snapshot,
	started_at, submitted_at, graded_at, created_at, updated_at`

func scanAttempt(row pgx.Row) (*QuizAttempt, error) {
	a := &QuizAttempt{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.EnrollmentID, &a.LessonID, &a.AttemptNumber,
		&a.FormInstanceID, &a.Score, &a.PointsEarned, &a.PointsPossible, &a.Passed,
		&a.PassMarkSnapshot, &a.StartedAt, &a.SubmittedAt, &a.GradedAt, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *repoImpl) FindAttempts(ctx context.Context, enrollmentID string) ([]*QuizAttempt, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_quiz_attempts WHERE enrollment_id=$1
			ORDER BY lesson_id, attempt_number`, attemptCols), enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindAttempts: %w", err)
	}
	defer rows.Close()

	out := make([]*QuizAttempt, 0)
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindAttempts: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindAttemptByRef(ctx context.Context, orgID, ref string) (*QuizAttempt, error) {
	a, err := scanAttempt(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_quiz_attempts
			WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, attemptCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("learning: FindAttemptByRef: %w", err)
	}
	return a, nil
}

// FindOpenAttempt returns the learner's ungraded attempt at this lesson, if
// any. Resuming an open attempt rather than starting a new one is what stops
// a page refresh burning a retry.
func (r *repoImpl) FindOpenAttempt(ctx context.Context, enrollmentID, lessonID string) (*QuizAttempt, error) {
	a, err := scanAttempt(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_quiz_attempts
			WHERE enrollment_id=$1 AND lesson_id=$2 AND graded_at IS NULL
			ORDER BY attempt_number DESC LIMIT 1`, attemptCols), enrollmentID, lessonID))
	if err != nil {
		return nil, fmt.Errorf("learning: FindOpenAttempt: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CountAttempts(ctx context.Context, enrollmentID, lessonID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_quiz_attempts WHERE enrollment_id=$1 AND lesson_id=$2`,
		enrollmentID, lessonID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("learning: CountAttempts: %w", err)
	}
	return n, nil
}

func (r *repoImpl) HasPassedAttempt(ctx context.Context, enrollmentID, lessonID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_quiz_attempts
		  WHERE enrollment_id=$1 AND lesson_id=$2 AND passed = TRUE)`,
		enrollmentID, lessonID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("learning: HasPassedAttempt: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CreateAttempt(ctx context.Context, a *QuizAttempt) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_quiz_attempts
		    (org_id, enrollment_id, lesson_id, attempt_number, form_instance_id, pass_mark_snapshot)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, public_id, started_at, created_at, updated_at`,
		a.OrgID, a.EnrollmentID, a.LessonID, a.AttemptNumber, a.FormInstanceID, a.PassMarkSnapshot,
	).Scan(&a.ID, &a.PublicID, &a.StartedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: CreateAttempt: %w", err)
	}
	return nil
}

// GradeAttempt freezes score, points and pass/fail together with graded_at.
// One statement, so an attempt can never be observed graded-but-unscored.
func (r *repoImpl) GradeAttempt(ctx context.Context, orgID string, a *QuizAttempt) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_quiz_attempts SET
		    score=$1, points_earned=$2, points_possible=$3, passed=$4,
		    submitted_at=COALESCE(submitted_at, NOW()), graded_at=NOW(), updated_at=NOW()
		 WHERE org_id=$5 AND id=$6
		 RETURNING submitted_at, graded_at, updated_at`,
		a.Score, a.PointsEarned, a.PointsPossible, a.Passed, orgID, a.ID,
	).Scan(&a.SubmittedAt, &a.GradedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAttemptNotFound
	}
	if err != nil {
		return fmt.Errorf("learning: GradeAttempt: %w", err)
	}
	return nil
}

// ── Answer keys ──────────────────────────────────────────────────────────────

const answerKeyCols = `id, public_id, org_id, question_id, correct_text, correct_number,
	correct_boolean, correct_options, points, partial_credit, explanation, created_at, updated_at`

func scanAnswerKey(row pgx.Row) (*AnswerKey, error) {
	k := &AnswerKey{}
	err := row.Scan(&k.ID, &k.PublicID, &k.OrgID, &k.QuestionID, &k.CorrectText, &k.CorrectNumber,
		&k.CorrectBoolean, &k.CorrectOptions, &k.Points, &k.PartialCredit, &k.Explanation,
		&k.CreatedAt, &k.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// FindAnswerKeysForTemplate walks template → sections → questions to collect
// every key. SERVER-SIDE ONLY: the result is consumed by Grade and never
// reaches a response body.
func (r *repoImpl) FindAnswerKeysForTemplate(ctx context.Context, orgID, templateID string) (map[string]*AnswerKey, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_quiz_answer_keys k
			WHERE k.org_id = $1
			  AND k.question_id IN (
			      SELECT q.id FROM platform_form_questions q
			        JOIN platform_form_sections s ON s.id = q.section_id
			       WHERE s.template_id = $2)`, prefixCols("k", answerKeyCols)),
		orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("learning: FindAnswerKeysForTemplate: %w", err)
	}
	defer rows.Close()

	out := map[string]*AnswerKey{}
	for rows.Next() {
		k, err := scanAnswerKey(rows)
		if err != nil {
			return nil, fmt.Errorf("learning: FindAnswerKeysForTemplate: scan: %w", err)
		}
		out[k.QuestionID] = k
	}
	return out, rows.Err()
}

func (r *repoImpl) FindAnswerKeyByQuestion(ctx context.Context, orgID, questionID string) (*AnswerKey, error) {
	k, err := scanAnswerKey(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_quiz_answer_keys WHERE org_id=$1 AND question_id=$2`,
			answerKeyCols), orgID, questionID))
	if err != nil {
		return nil, fmt.Errorf("learning: FindAnswerKeyByQuestion: %w", err)
	}
	return k, nil
}

func (r *repoImpl) UpsertAnswerKey(ctx context.Context, k *AnswerKey) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_quiz_answer_keys
		    (org_id, question_id, correct_text, correct_number, correct_boolean,
		     correct_options, points, partial_credit, explanation)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (question_id) DO UPDATE SET
		    correct_text=EXCLUDED.correct_text,
		    correct_number=EXCLUDED.correct_number,
		    correct_boolean=EXCLUDED.correct_boolean,
		    correct_options=EXCLUDED.correct_options,
		    points=EXCLUDED.points,
		    partial_credit=EXCLUDED.partial_credit,
		    explanation=EXCLUDED.explanation,
		    updated_at=NOW()
		 RETURNING id, public_id, created_at, updated_at`,
		k.OrgID, k.QuestionID, k.CorrectText, k.CorrectNumber, k.CorrectBoolean,
		k.CorrectOptions, k.Points, k.PartialCredit, k.Explanation,
	).Scan(&k.ID, &k.PublicID, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return fmt.Errorf("learning: UpsertAnswerKey: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteAnswerKey(ctx context.Context, orgID, questionID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM hrm_quiz_answer_keys WHERE org_id=$1 AND question_id=$2`, orgID, questionID)
	if err != nil {
		return fmt.Errorf("learning: DeleteAnswerKey: %w", err)
	}
	return nil
}
