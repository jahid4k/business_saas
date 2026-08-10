// backend/internal/tests/unit/hrm/learning/grading_test.go
// Phase 6A quiz grading. Written before the dependent layers so the formula is
// locked first — the same order Phase 5A used for its progress formula.
//
// Grading is deliberately NOT forms.ScoreInstance: the engine normalises each
// answer 0-1 against its own scale and weights it, which answers "how highly
// did you rate this", not "did you get it right".
package learning_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/learning"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func decPtr(s string) *decimal.Decimal {
	d := dec(s)
	return &d
}

// resp builds one answered response. qid doubles as the question id the key
// map is keyed on.
func resp(qid string, qType forms.QuestionType) *forms.Response {
	id := qid
	return &forms.Response{ID: "r_" + qid, QuestionID: &id, QuestionType: qType}
}

// ============================================================
// Per-type correctness
// ============================================================

func TestGrade_EachQuestionType(t *testing.T) {
	cases := []struct {
		name string
		r    *forms.Response
		key  *learning.AnswerKey
		want string // expected score percent
	}{
		{
			name: "boolean correct",
			r:    withBool(resp("q1", forms.QuestionBoolean), true),
			key:  &learning.AnswerKey{CorrectBoolean: boolPtr(true), Points: dec("1")},
			want: "100",
		},
		{
			name: "boolean wrong",
			r:    withBool(resp("q1", forms.QuestionBoolean), false),
			key:  &learning.AnswerKey{CorrectBoolean: boolPtr(true), Points: dec("1")},
			want: "0",
		},
		{
			name: "number correct",
			r:    withNumber(resp("q1", forms.QuestionNumber), "42"),
			key:  &learning.AnswerKey{CorrectNumber: decPtr("42"), Points: dec("1")},
			want: "100",
		},
		{
			// 42.0 and 42 are the same number; a string comparison would fail
			// this and is the obvious wrong implementation.
			name: "number equal despite different scale",
			r:    withNumber(resp("q1", forms.QuestionNumber), "42.00"),
			key:  &learning.AnswerKey{CorrectNumber: decPtr("42"), Points: dec("1")},
			want: "100",
		},
		{
			name: "scale correct",
			r:    withNumber(resp("q1", forms.QuestionScale), "4"),
			key:  &learning.AnswerKey{CorrectNumber: decPtr("4"), Points: dec("1")},
			want: "100",
		},
		{
			name: "text correct ignoring case and surrounding space",
			r:    withText(resp("q1", forms.QuestionText), "  Mitochondria "),
			key:  &learning.AnswerKey{CorrectText: strPtr("mitochondria"), Points: dec("1")},
			want: "100",
		},
		{
			name: "textarea wrong",
			r:    withText(resp("q1", forms.QuestionTextarea), "chloroplast"),
			key:  &learning.AnswerKey{CorrectText: strPtr("mitochondria"), Points: dec("1")},
			want: "0",
		},
		{
			name: "single select correct",
			r:    withOptions(resp("q1", forms.QuestionSingleSelect), "emea"),
			key:  &learning.AnswerKey{CorrectOptions: []string{"EMEA"}, Points: dec("1")},
			want: "100",
		},
		{
			name: "single select wrong",
			r:    withOptions(resp("q1", forms.QuestionSingleSelect), "apac"),
			key:  &learning.AnswerKey{CorrectOptions: []string{"emea"}, Points: dec("1")},
			want: "0",
		},
		{
			name: "unanswered scores zero, never a nil panic",
			r:    resp("q1", forms.QuestionBoolean),
			key:  &learning.AnswerKey{CorrectBoolean: boolPtr(true), Points: dec("1")},
			want: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := learning.Grade(
				[]*forms.Response{tc.r},
				map[string]*learning.AnswerKey{"q1": tc.key},
			)
			if !got.ScorePercent.Equal(dec(tc.want)) {
				t.Errorf("score = %s, want %s", got.ScorePercent, tc.want)
			}
		})
	}
}

// ============================================================
// multi_select — where the naive implementation goes wrong
// ============================================================

// TestGrade_MultiSelect_AllOrNothing is the default: partial selections and
// over-selections both score zero.
func TestGrade_MultiSelect_AllOrNothing(t *testing.T) {
	key := &learning.AnswerKey{CorrectOptions: []string{"go", "sql"}, Points: dec("1")}

	cases := []struct {
		name  string
		given []string
		want  string
	}{
		{"exact match", []string{"go", "sql"}, "100"},
		{"order does not matter", []string{"sql", "go"}, "100"},
		{"case does not matter", []string{"GO", "Sql"}, "100"},
		{"missing one", []string{"go"}, "0"},
		{"one extra", []string{"go", "sql", "rust"}, "0"},
		{"all wrong", []string{"rust"}, "0"},
		{"nothing selected", nil, "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withOptions(resp("q1", forms.QuestionMultiSelect), tc.given...)
			got := learning.Grade([]*forms.Response{r}, map[string]*learning.AnswerKey{"q1": key})
			if !got.ScorePercent.Equal(dec(tc.want)) {
				t.Errorf("score = %s, want %s", got.ScorePercent, tc.want)
			}
		})
	}
}

// TestGrade_MultiSelect_PartialCreditPenalisesGuessing is the headline
// grading test. A "count the hits" implementation gives full marks for
// selecting every option, which makes the question worthless. Credit is
// (hits − misses) / expected, floored at zero.
func TestGrade_MultiSelect_PartialCreditPenalisesGuessing(t *testing.T) {
	key := &learning.AnswerKey{
		CorrectOptions: []string{"go", "sql"}, Points: dec("1"), PartialCredit: true,
	}

	cases := []struct {
		name  string
		given []string
		want  string
	}{
		{"both correct", []string{"go", "sql"}, "100"},
		{"one of two", []string{"go"}, "50"},
		{"one right one wrong nets to zero", []string{"go", "rust"}, "0"},
		{"selecting everything must NOT score full marks", []string{"go", "sql", "rust", "python"}, "0"},
		{"only wrong answers", []string{"rust", "python"}, "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withOptions(resp("q1", forms.QuestionMultiSelect), tc.given...)
			got := learning.Grade([]*forms.Response{r}, map[string]*learning.AnswerKey{"q1": key})
			if !got.ScorePercent.Equal(dec(tc.want)) {
				t.Errorf("given %v: score = %s, want %s", tc.given, got.ScorePercent, tc.want)
			}
		})
	}
}

// ============================================================
// Weighting and the unkeyed-question rule
// ============================================================

// TestGrade_UnkeyedQuestionsDoNotScoreOrDilute pins the rule that lets an
// ungraded reflection question sit in the same form as scored ones — the
// platform_form_questions.weight IS NULL precedent.
func TestGrade_UnkeyedQuestionsDoNotScoreOrDilute(t *testing.T) {
	responses := []*forms.Response{
		withBool(resp("q1", forms.QuestionBoolean), true),
		withText(resp("q2", forms.QuestionTextarea), "Some reflective prose"),
	}
	keys := map[string]*learning.AnswerKey{
		"q1": {CorrectBoolean: boolPtr(true), Points: dec("1")},
		// q2 has no key at all.
	}

	got := learning.Grade(responses, keys)
	if !got.ScorePercent.Equal(dec("100")) {
		t.Errorf("score = %s, want 100 — an unkeyed question must not dilute the total", got.ScorePercent)
	}
	if !got.PointsPossible.Equal(dec("1")) {
		t.Errorf("points_possible = %s, want 1 — the unkeyed question must not enter the denominator", got.PointsPossible)
	}
}

func TestGrade_RespectsPerQuestionPoints(t *testing.T) {
	responses := []*forms.Response{
		withBool(resp("q1", forms.QuestionBoolean), true),  // correct, worth 3
		withBool(resp("q2", forms.QuestionBoolean), false), // wrong, worth 1
	}
	keys := map[string]*learning.AnswerKey{
		"q1": {CorrectBoolean: boolPtr(true), Points: dec("3")},
		"q2": {CorrectBoolean: boolPtr(true), Points: dec("1")},
	}

	got := learning.Grade(responses, keys)
	if !got.PointsEarned.Equal(dec("3")) {
		t.Errorf("points_earned = %s, want 3", got.PointsEarned)
	}
	if !got.PointsPossible.Equal(dec("4")) {
		t.Errorf("points_possible = %s, want 4", got.PointsPossible)
	}
	if !got.ScorePercent.Equal(dec("75")) {
		t.Errorf("score = %s, want 75", got.ScorePercent)
	}
}

// TestGrade_NoScorableQuestions is the degenerate case. A quiz with no keys
// must score zero rather than divide by zero or report a vacuous 100.
func TestGrade_NoScorableQuestions(t *testing.T) {
	responses := []*forms.Response{withText(resp("q1", forms.QuestionTextarea), "prose")}

	got := learning.Grade(responses, map[string]*learning.AnswerKey{})
	if !got.ScorePercent.Equal(decimal.Zero) {
		t.Errorf("score = %s, want 0", got.ScorePercent)
	}
	if !got.PointsPossible.IsZero() {
		t.Errorf("points_possible = %s, want 0", got.PointsPossible)
	}
}

// TestGrade_IgnoresResponsesWithNoQuestionID pins behaviour against the real
// schema: platform_form_responses.question_id is ON DELETE SET NULL, so a
// response whose question was deleted arrives with a nil id. It must be
// skipped, not panic.
func TestGrade_IgnoresResponsesWithNoQuestionID(t *testing.T) {
	orphan := &forms.Response{ID: "r_orphan", QuestionType: forms.QuestionBoolean}
	orphan.AnswerBoolean = boolPtr(true)

	responses := []*forms.Response{
		withBool(resp("q1", forms.QuestionBoolean), true),
		orphan,
		nil, // defensive: a nil entry must not panic either
	}
	keys := map[string]*learning.AnswerKey{"q1": {CorrectBoolean: boolPtr(true), Points: dec("1")}}

	got := learning.Grade(responses, keys)
	if !got.ScorePercent.Equal(dec("100")) {
		t.Errorf("score = %s, want 100", got.ScorePercent)
	}
	if !got.PointsPossible.Equal(dec("1")) {
		t.Errorf("points_possible = %s, want 1", got.PointsPossible)
	}
}

// TestGrade_RoundsToTwoPlaces keeps a stored NUMERIC(5,2) from silently
// truncating something the caller was told was different.
func TestGrade_RoundsToTwoPlaces(t *testing.T) {
	responses := []*forms.Response{
		withBool(resp("q1", forms.QuestionBoolean), true),
		withBool(resp("q2", forms.QuestionBoolean), true),
		withBool(resp("q3", forms.QuestionBoolean), false),
	}
	keys := map[string]*learning.AnswerKey{
		"q1": {CorrectBoolean: boolPtr(true), Points: dec("1")},
		"q2": {CorrectBoolean: boolPtr(true), Points: dec("1")},
		"q3": {CorrectBoolean: boolPtr(true), Points: dec("1")},
	}

	got := learning.Grade(responses, keys)
	if !got.ScorePercent.Equal(dec("66.67")) {
		t.Errorf("score = %s, want 66.67", got.ScorePercent)
	}
}

// ============================================================
// The DTO shape a learner receives
// ============================================================

// TestQuestionForAttempt_CarriesNoCorrectAnswer asserts the TYPE, not a
// value. The build plan names this DTO specifically. If someone later
// "simplifies" it into an AnswerKey or adds a correct/explanation field, this
// fails loudly rather than silently handing learners the answers.
func TestQuestionForAttempt_CarriesNoCorrectAnswer(t *testing.T) {
	forbidden := []string{"correct", "answerkey", "key", "explanation", "points", "partialcredit"}
	assertNoForbiddenFields(t, reflectTypeOf(learning.QuestionForAttempt{}), forbidden)
}

// TestAttemptDetail_ExposesNoAnswerKey covers the wrapper too — embedding an
// AnswerKey on the detail would defeat the DTO.
func TestAttemptDetail_ExposesNoAnswerKey(t *testing.T) {
	forbidden := []string{"correct", "answerkey", "explanation"}
	assertNoForbiddenFields(t, reflectTypeOf(learning.AttemptDetail{}), forbidden)
}
