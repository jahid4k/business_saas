// backend/internal/tests/integration/forms_test.go
// The form engine against real Postgres — what a stub repository cannot
// prove: that the typed answer columns actually round-trip through their
// distinct SQL types, that instantiation is transactional, that the JSONB
// options column survives a round trip, and that the ON DELETE rules leave
// live instances intact.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

func formDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}

func intPtr(i int) *int        { return &i }
func strPtr2(s string) *string { return &s }

// seedFormTemplate builds a template with one section covering every question
// type, so a single instance exercises all five typed answer columns.
func seedFormTemplate(t *testing.T, env *testEnv, orgID, userID string) *forms.Template {
	t.Helper()
	ctx := context.Background()

	tmpl, err := env.formsSvc.CreateTemplate(ctx, orgID, userID, forms.CreateTemplateRequest{
		Name: "Every type " + uniqueSlug("f"), FormType: string(forms.FormTypeAppraisal), IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	sec, err := env.formsSvc.CreateSection(ctx, orgID, tmpl.ID, forms.CreateSectionRequest{
		Title: "All question types",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}

	weight := formDec("100")
	questions := []forms.CreateQuestionRequest{
		{QuestionText: "Rate delivery", QuestionType: string(forms.QuestionScale),
			ScaleMin: intPtr(1), ScaleMax: intPtr(5), Weight: &weight, IsRequired: true},
		{QuestionText: "Free comment", QuestionType: string(forms.QuestionTextarea)},
		{QuestionText: "Headcount", QuestionType: string(forms.QuestionNumber)},
		{QuestionText: "Promotion ready", QuestionType: string(forms.QuestionBoolean)},
		{QuestionText: "Review date", QuestionType: string(forms.QuestionDate)},
		{QuestionText: "Region", QuestionType: string(forms.QuestionSingleSelect),
			Options: []forms.Option{{Value: "emea", Label: "EMEA"}, {Value: "apac", Label: "APAC"}}},
		{QuestionText: "Skills", QuestionType: string(forms.QuestionMultiSelect),
			Options: []forms.Option{{Value: "go", Label: "Go"}, {Value: "sql", Label: "SQL"}}},
	}
	for i, q := range questions {
		q.DisplayOrder = intPtr(i)
		if _, err := env.formsSvc.CreateQuestion(ctx, orgID, sec.ID, q); err != nil {
			t.Fatalf("create question %d: %v", i, err)
		}
	}
	return tmpl
}

// ============================================================
// Typed answers actually round-trip through their SQL columns
// ============================================================

// TestIntegration_Forms_TypedAnswersRoundTrip is the headline test. A stub
// repository stores Go values in a map and proves nothing about whether
// NUMERIC, BOOLEAN, DATE and TEXT[] survive the driver — this does.
func TestIntegration_Forms_TypedAnswersRoundTrip(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)

	tmpl := seedFormTemplate(t, env, orgID, ownerID)
	respondent := ownerID
	inst, err := env.formsSvc.Instantiate(ctx, orgID, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "00000000-0000-0000-0000-000000000001",
		SubjectLabel: "Alex Doe", RespondentUserID: &respondent, RespondentRole: "self", CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if len(inst.Responses) != 7 {
		t.Fatalf("expected 7 response rows, got %d", len(inst.Responses))
	}

	byText := map[string]*forms.Response{}
	for _, r := range inst.Responses {
		byText[r.QuestionText] = r
	}

	reviewDate := "2030-06-30"
	if _, err := env.formsSvc.SaveAnswers(ctx, orgID, inst.ID, ownerID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: byText["Rate delivery"].ID, AnswerNumber: &[]decimal.Decimal{formDec("4")}[0]},
			{ResponseID: byText["Free comment"].ID, AnswerText: strPtr2("Consistently strong")},
			{ResponseID: byText["Headcount"].ID, AnswerNumber: &[]decimal.Decimal{formDec("12.5")}[0]},
			{ResponseID: byText["Promotion ready"].ID, AnswerBoolean: &[]bool{true}[0]},
			{ResponseID: byText["Review date"].ID, AnswerDate: &reviewDate},
			{ResponseID: byText["Region"].ID, AnswerOptions: []string{"emea"}},
			{ResponseID: byText["Skills"].ID, AnswerOptions: []string{"go", "sql"}},
		},
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}

	// Re-read from the database, not from the in-memory objects.
	fresh, err := env.formsSvc.GetInstance(ctx, orgID, inst.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	got := map[string]*forms.Response{}
	for _, r := range fresh.Responses {
		got[r.QuestionText] = r
	}

	if v := got["Rate delivery"].AnswerNumber; v == nil || !v.Equal(formDec("4")) {
		t.Errorf("scale answer did not round-trip: %v", v)
	}
	if v := got["Free comment"].AnswerText; v == nil || *v != "Consistently strong" {
		t.Errorf("text answer did not round-trip: %v", v)
	}
	// A fractional NUMERIC is the case a float column would quietly corrupt.
	if v := got["Headcount"].AnswerNumber; v == nil || !v.Equal(formDec("12.5")) {
		t.Errorf("numeric answer did not round-trip: %v", v)
	}
	if v := got["Promotion ready"].AnswerBoolean; v == nil || !*v {
		t.Errorf("boolean answer did not round-trip: %v", v)
	}
	if v := got["Review date"].AnswerDate; v == nil || v.Format("2006-01-02") != reviewDate {
		t.Errorf("date answer did not round-trip: %v", v)
	}
	if v := got["Region"].AnswerOptions; len(v) != 1 || v[0] != "emea" {
		t.Errorf("single-select answer did not round-trip: %v", v)
	}
	if v := got["Skills"].AnswerOptions; len(v) != 2 || v[0] != "go" || v[1] != "sql" {
		t.Errorf("multi-select TEXT[] did not round-trip: %v", v)
	}
}

// TestIntegration_Forms_OptionsJSONBRoundTrips proves the one JSONB column
// survives marshal → store → scan → unmarshal.
func TestIntegration_Forms_OptionsJSONBRoundTrips(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	tmpl := seedFormTemplate(t, env, orgID, ownerID)

	full, err := env.formsSvc.GetTemplate(ctx, orgID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	var region *forms.Question
	for _, sec := range full.Sections {
		for _, q := range sec.Questions {
			if q.QuestionText == "Region" {
				region = q
			}
		}
	}
	if region == nil {
		t.Fatal("expected the Region question to be returned")
	}
	if len(region.Options) != 2 || region.Options[0].Value != "emea" || region.Options[0].Label != "EMEA" {
		t.Errorf("options JSONB did not round-trip: %+v", region.Options)
	}
}

// ============================================================
// Instantiation atomicity + snapshot durability
// ============================================================

// TestIntegration_Forms_InstantiationIsAtomic forces the response insert to
// fail and asserts no orphan instance survives. A half-instantiated form
// would be missing questions with no way to tell which.
func TestIntegration_Forms_InstantiationIsAtomic(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	tmpl := seedFormTemplate(t, env, orgID, ownerID)

	var before int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_form_instances WHERE org_id = $1`, orgID).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}

	// A subject_id that is not a UUID fails inside the transaction, after the
	// instance row has already been inserted.
	_, err := env.formsSvc.Instantiate(ctx, orgID, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "not-a-uuid",
		SubjectLabel: "Broken", CreatedBy: ownerID,
	})
	if err == nil {
		t.Fatal("expected instantiation with an invalid subject_id to fail")
	}

	var after int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_form_instances WHERE org_id = $1`, orgID).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Errorf("expected no instance row to survive a failed instantiation, count went %d → %d", before, after)
	}
}

// TestIntegration_Forms_TemplateDeleteLeavesInstanceIntact pins the
// ON DELETE SET NULL on template_id: the snapshot is what renders, so a live
// form must survive its template being deleted.
func TestIntegration_Forms_TemplateDeleteLeavesInstanceIntact(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	tmpl := seedFormTemplate(t, env, orgID, ownerID)

	respondent := ownerID
	inst, err := env.formsSvc.Instantiate(ctx, orgID, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "00000000-0000-0000-0000-000000000001",
		SubjectLabel: "Alex", RespondentUserID: &respondent, CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// The service refuses this, so go around it — the point is the FK.
	if _, err := env.db.Exec(ctx, `DELETE FROM platform_form_templates WHERE id = $1`, tmpl.ID); err != nil {
		t.Fatalf("SCHEMA: deleting a template with live instances must not error, got: %v", err)
	}

	fresh, err := env.formsSvc.GetInstance(ctx, orgID, inst.ID)
	if err != nil {
		t.Fatalf("SCHEMA: the instance must survive its template's deletion, got: %v", err)
	}
	if fresh.TemplateID != nil {
		t.Errorf("expected template_id nulled by ON DELETE SET NULL, got %v", *fresh.TemplateID)
	}
	if fresh.TemplateName == "" || len(fresh.Responses) != 7 {
		t.Errorf("expected the snapshot to survive intact: name=%q responses=%d", fresh.TemplateName, len(fresh.Responses))
	}
	if fresh.Responses[0].QuestionText == "" {
		t.Error("expected the question snapshot to survive the template's deletion")
	}
}

// ============================================================
// Constraints and isolation
// ============================================================

// TestIntegration_Forms_OneDefaultPerFormType proves the partial unique index
// that crm_pipelines never got.
func TestIntegration_Forms_OneDefaultPerFormType(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	seedFormTemplate(t, env, orgID, ownerID) // already default

	// A raw second default must violate uq_pfrm_tpl_default.
	_, err := env.db.Exec(ctx,
		`INSERT INTO platform_form_templates (org_id, name, form_type, is_default, is_active, created_by)
		 VALUES ($1, 'Second default', 'appraisal', TRUE, TRUE, $2)`,
		orgID, ownerID,
	)
	if err == nil {
		t.Error("expected uq_pfrm_tpl_default to reject a second default appraisal template")
	}

	// The service's atomic clear-then-set path must succeed where the raw
	// insert failed.
	second, err := env.formsSvc.CreateTemplate(ctx, orgID, ownerID, forms.CreateTemplateRequest{
		Name: "Second " + uniqueSlug("f"), FormType: string(forms.FormTypeAppraisal), IsDefault: true,
	})
	if err != nil {
		t.Fatalf("expected the service to promote a new default atomically, got %v", err)
	}
	def, err := env.formsSvc.ListTemplates(ctx, orgID, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	defaults := 0
	for _, tm := range def {
		if tm.IsDefault {
			defaults++
			if tm.ID != second.ID {
				t.Errorf("expected the newest template to hold the default, got %s", tm.ID)
			}
		}
	}
	if defaults != 1 {
		t.Errorf("expected exactly one default appraisal template, got %d", defaults)
	}
}

func TestIntegration_Forms_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, _, ownerAID := seedScopeTestOrg(t, env)
	orgBID, _, _ := seedScopeTestOrg(t, env)

	tmpl := seedFormTemplate(t, env, orgAID, ownerAID)
	respondent := ownerAID
	inst, err := env.formsSvc.Instantiate(ctx, orgAID, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "00000000-0000-0000-0000-000000000001",
		SubjectLabel: "Alex", RespondentUserID: &respondent, CreatedBy: ownerAID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if _, err := env.formsSvc.GetTemplate(ctx, orgBID, tmpl.ID); !errors.Is(err, forms.ErrTemplateNotFound) {
		t.Errorf("SECURITY: org B must not read org A's template, got %v", err)
	}
	if _, err := env.formsSvc.GetInstance(ctx, orgBID, inst.ID); !errors.Is(err, forms.ErrInstanceNotFound) {
		t.Errorf("SECURITY: org B must not read org A's form instance, got %v", err)
	}
}

// TestIntegration_Forms_SubmittedInstanceIsImmutableInPractice checks the
// rule survives a real re-read rather than only an in-memory flag.
func TestIntegration_Forms_SubmittedInstanceIsImmutable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	tmpl := seedFormTemplate(t, env, orgID, ownerID)

	respondent := ownerID
	inst, err := env.formsSvc.Instantiate(ctx, orgID, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "00000000-0000-0000-0000-000000000001",
		SubjectLabel: "Alex", RespondentUserID: &respondent, CreatedBy: ownerID,
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// The only required question is the scale one.
	var scaleID string
	for _, r := range inst.Responses {
		if r.QuestionText == "Rate delivery" {
			scaleID = r.ID
		}
	}
	if _, err := env.formsSvc.SaveAnswers(ctx, orgID, inst.ID, ownerID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: scaleID, AnswerNumber: &[]decimal.Decimal{formDec("5")}[0]}},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := env.formsSvc.SubmitInstance(ctx, orgID, inst.ID, ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Re-read, then attempt an edit — proving submitted_at persisted rather
	// than merely being set on the returned struct.
	_, err = env.formsSvc.SaveAnswers(ctx, orgID, inst.ID, ownerID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: scaleID, AnswerNumber: &[]decimal.Decimal{formDec("1")}[0]}},
	})
	if !errors.Is(err, forms.ErrInstanceSubmitted) {
		t.Fatalf("expected ErrInstanceSubmitted after a real re-read, got %v", err)
	}

	fresh, _ := env.formsSvc.GetInstance(ctx, orgID, inst.ID)
	if fresh.SubmittedAt == nil {
		t.Error("expected submitted_at to be persisted")
	}
	if !fresh.Score.Percent.Equal(formDec("100")) {
		t.Errorf("expected a 5-of-5 scale answer to score 100%%, got %s", fresh.Score.Percent)
	}
}
