// backend/internal/hrm/learning/quiz_model.go
package learning

import (
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

// QuizAttempt is one graded run at a quiz lesson.
//
// Score, PointsEarned, PointsPossible, Passed and PassMarkSnapshot are all
// FROZEN at grading time and never re-derived. The reason is concrete rather
// than stylistic: AnswerKey is keyed on platform_form_questions.id, and
// platform_form_responses.question_id is ON DELETE SET NULL — documented in
// migration 00084 as "provenance only, never joined for display", because the
// question snapshot lives on the response row. So the moment a question is
// deleted the key is unreachable, and a re-grade would silently score zero.
//
// Same publish-immutability reasoning as hrm_appraisals snapshotting its
// scores: a record whose numbers are recomputed from mutable sources is not
// immutable.
type QuizAttempt struct {
	ID            string `db:"id"            json:"id"`
	PublicID      string `db:"public_id"     json:"public_id"`
	OrgID         string `db:"org_id"        json:"org_id"`
	EnrollmentID  string `db:"enrollment_id" json:"enrollment_id"`
	LessonID      string `db:"lesson_id"     json:"lesson_id"`
	AttemptNumber int    `db:"attempt_number" json:"attempt_number"`

	// The learner's answers live in the form engine. Exposed to the learner
	// because it is THEIR instance — the same reasoning that lets
	// feedback.MyRequest carry one.
	FormInstanceID *string `db:"form_instance_id" json:"form_instance_id,omitempty"`

	Score            *decimal.Decimal `db:"score"              json:"score,omitempty"`
	PointsEarned     *decimal.Decimal `db:"points_earned"      json:"points_earned,omitempty"`
	PointsPossible   *decimal.Decimal `db:"points_possible"    json:"points_possible,omitempty"`
	Passed           *bool            `db:"passed"             json:"passed,omitempty"`
	PassMarkSnapshot *decimal.Decimal `db:"pass_mark_snapshot" json:"pass_mark_snapshot,omitempty"`

	StartedAt   time.Time  `db:"started_at"   json:"started_at"`
	SubmittedAt *time.Time `db:"submitted_at" json:"submitted_at,omitempty"`
	GradedAt    *time.Time `db:"graded_at"    json:"graded_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`
}

// IsGraded reports whether this attempt has a frozen result.
func (a *QuizAttempt) IsGraded() bool { return a.GradedAt != nil }

// AnswerKey is the correct answer for one question, owned by HRM so
// platform/forms carries no assessment semantics.
//
// ⚠ This type must NEVER be serialised to a learner. It carries no json tags
// that would make that convenient, and no learner-facing response type embeds
// it. The protection is that the attempt read path does not query the key
// table at all — there is no field to forget to strip, which is the same
// structural approach internal/hrm/feedback uses for 360 anonymity.
type AnswerKey struct {
	ID         string `db:"id"`
	PublicID   string `db:"public_id"`
	OrgID      string `db:"org_id"`
	QuestionID string `db:"question_id"`

	// Exactly one is populated, selected by the question's type.
	CorrectText    *string          `db:"correct_text"`
	CorrectNumber  *decimal.Decimal `db:"correct_number"`
	CorrectBoolean *bool            `db:"correct_boolean"`
	CorrectOptions []string         `db:"correct_options"`

	Points        decimal.Decimal `db:"points"`
	PartialCredit bool            `db:"partial_credit"`
	Explanation   *string         `db:"explanation"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// QuestionForAttempt is what a LEARNER sees while sitting a quiz. The build
// plan names this type specifically.
//
// It is a distinct type, not an AnswerKey or a forms.Response with fields
// blanked — the performance.GoalRef precedent applied to a stronger
// requirement. A separate type means no column added to hrm_quiz_answer_keys
// in a later phase can leak through this path by being picked up in a shared
// struct. There is deliberately no Correct* field, and no Explanation:
// explanations often restate the answer.
type QuestionForAttempt struct {
	ResponseID   string         `json:"response_id"`
	QuestionText string         `json:"question_text"`
	QuestionType string         `json:"question_type"`
	IsRequired   bool           `json:"is_required"`
	DisplayOrder int            `json:"display_order"`
	ScaleMin     *int           `json:"scale_min,omitempty"`
	ScaleMax     *int           `json:"scale_max,omitempty"`
	Options      []forms.Option `json:"options,omitempty"`

	// The learner's OWN answer so far, so a part-finished attempt renders.
	AnswerText    *string          `json:"answer_text,omitempty"`
	AnswerNumber  *decimal.Decimal `json:"answer_number,omitempty"`
	AnswerBoolean *bool            `json:"answer_boolean,omitempty"`
	AnswerOptions []string         `json:"answer_options,omitempty"`
}

// AttemptDetail is the learner-facing view of an attempt in progress or
// finished. Questions carries no correct answers, by construction.
type AttemptDetail struct {
	*QuizAttempt
	Questions []QuestionForAttempt `json:"questions"`
	// AttemptsRemaining is nil when the lesson allows unlimited retries.
	AttemptsRemaining *int `json:"attempts_remaining,omitempty"`
}

// ── Grading ──────────────────────────────────────────────────────────────────

// GradeResult is the frozen outcome of grading one attempt.
type GradeResult struct {
	PointsEarned   decimal.Decimal
	PointsPossible decimal.Decimal
	// ScorePercent is 0 when PointsPossible is zero — a quiz with no scorable
	// questions scores zero rather than dividing by zero or reporting 100.
	ScorePercent decimal.Decimal
	Passed       bool
}

// Grade compares a learner's submitted responses against the answer keys and
// produces the frozen result stored on the attempt.
//
// This is deliberately NOT forms.ScoreInstance. The engine's score normalises
// each answer 0-1 against its own scale and weights it — meaningful for an
// appraisal ("how highly did you rate this"), meaningless for a quiz, where
// the only question is whether the answer matches the key.
//
// Rules, each of which has a test:
//   - A question with no key does not score and does not inflate the
//     denominator. That is what lets an ungraded free-text reflection sit in
//     the same form as scored questions — the platform_form_questions.weight
//     IS NULL precedent.
//   - multi_select is all-or-nothing unless the key sets PartialCredit, in
//     which case credit is (correct selected − incorrect selected) / expected,
//     floored at zero. Without the floor, guessing every option would score
//     positively.
//   - Option comparison is order-insensitive and case-insensitive: option
//     ORDER is a rendering choice, not an answer.
//   - Text comparison is trimmed and case-insensitive. Exact-match free text
//     is a blunt instrument, which is why the service warns rather than
//     forbids; graders can override.
func Grade(responses []*forms.Response, keys map[string]*AnswerKey) GradeResult {
	earned, possible := decimal.Zero, decimal.Zero

	for _, r := range responses {
		if r == nil || r.QuestionID == nil {
			continue
		}
		key, ok := keys[*r.QuestionID]
		if !ok || key == nil {
			// Unkeyed question: contributes to neither side.
			continue
		}
		possible = possible.Add(key.Points)
		earned = earned.Add(key.Points.Mul(creditFor(r, key)))
	}

	res := GradeResult{PointsEarned: earned, PointsPossible: possible}
	if possible.IsZero() {
		res.ScorePercent = decimal.Zero
		return res
	}
	res.ScorePercent = earned.Div(possible).Mul(decimal.NewFromInt(100)).Round(2)
	return res
}

// creditFor returns the fraction of a question's points earned, in [0,1].
func creditFor(r *forms.Response, key *AnswerKey) decimal.Decimal {
	switch r.QuestionType {
	case forms.QuestionBoolean:
		if r.AnswerBoolean != nil && key.CorrectBoolean != nil &&
			*r.AnswerBoolean == *key.CorrectBoolean {
			return decimal.NewFromInt(1)
		}

	case forms.QuestionNumber, forms.QuestionScale:
		if r.AnswerNumber != nil && key.CorrectNumber != nil &&
			r.AnswerNumber.Equal(*key.CorrectNumber) {
			return decimal.NewFromInt(1)
		}

	case forms.QuestionText, forms.QuestionTextarea:
		if r.AnswerText != nil && key.CorrectText != nil &&
			strings.EqualFold(strings.TrimSpace(*r.AnswerText), strings.TrimSpace(*key.CorrectText)) {
			return decimal.NewFromInt(1)
		}

	case forms.QuestionSingleSelect:
		if len(r.AnswerOptions) == 1 && len(key.CorrectOptions) == 1 &&
			strings.EqualFold(r.AnswerOptions[0], key.CorrectOptions[0]) {
			return decimal.NewFromInt(1)
		}

	case forms.QuestionMultiSelect:
		return multiSelectCredit(r.AnswerOptions, key.CorrectOptions, key.PartialCredit)
	}

	return decimal.Zero
}

// multiSelectCredit scores a multi-select answer.
//
// All-or-nothing by default. With partial credit, the score is
// (hits − misses) / expected floored at zero, so selecting everything scores
// zero rather than full marks — the failure mode a naive "count the hits"
// implementation ships.
func multiSelectCredit(given, correct []string, partial bool) decimal.Decimal {
	if len(correct) == 0 {
		return decimal.Zero
	}
	want := normaliseOptions(correct)
	got := normaliseOptions(given)

	hits, wrong := 0, 0
	for _, g := range got {
		if containsOption(want, g) {
			hits++
		} else {
			wrong++
		}
	}

	if !partial {
		if hits == len(want) && wrong == 0 {
			return decimal.NewFromInt(1)
		}
		return decimal.Zero
	}

	net := hits - wrong
	if net <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromInt(int64(net)).Div(decimal.NewFromInt(int64(len(want)))).Round(4)
}

// normaliseOptions lowercases, trims and sorts, so option ORDER never affects
// a result — order is a rendering choice, not an answer.
func normaliseOptions(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		v := strings.ToLower(strings.TrimSpace(s))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func containsOption(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// ── Requests ─────────────────────────────────────────────────────────────────

// SetAnswerKeyRequest writes the key for one question. Authoring-side only,
// gated on hrm.courses.manage.
type SetAnswerKeyRequest struct {
	QuestionID     string           `json:"question_id"`
	CorrectText    *string          `json:"correct_text"`
	CorrectNumber  *decimal.Decimal `json:"correct_number"`
	CorrectBoolean *bool            `json:"correct_boolean"`
	CorrectOptions []string         `json:"correct_options"`
	Points         *decimal.Decimal `json:"points"`
	PartialCredit  *bool            `json:"partial_credit"`
	Explanation    *string          `json:"explanation"`
}

// SubmitAttemptRequest carries the learner's answers. Response ids come from
// the QuestionForAttempt list they were served.
type SubmitAttemptRequest struct {
	Answers []forms.AnswerRequest `json:"answers"`
}
