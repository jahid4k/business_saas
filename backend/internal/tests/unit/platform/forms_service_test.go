// backend/internal/tests/unit/platform/forms_service_test.go
// Form engine rules, against a hand-written stub repository. Black-box
// against the exported Service, the checklists_service_test.go precedent.
package platform

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

func fdec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}

func iptr(i int) *int                         { return &i }
func sptr(s string) *string                   { return &s }
func bptr(b bool) *bool                       { return &b }
func dptr(d decimal.Decimal) *decimal.Decimal { return &d }

// ── Stub repository ──────────────────────────────────────────────────────────

type stubFormRepo struct {
	seq int

	templates map[string]*forms.Template
	sections  map[string]*forms.Section
	questions map[string]*forms.Question
	instances map[string]*forms.Instance
	responses map[string][]*forms.Response
}

func newStubFormRepo() *stubFormRepo {
	return &stubFormRepo{
		templates: map[string]*forms.Template{},
		sections:  map[string]*forms.Section{},
		questions: map[string]*forms.Question{},
		instances: map[string]*forms.Instance{},
		responses: map[string][]*forms.Response{},
	}
}

func (r *stubFormRepo) next(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func fmatch(id, publicID, ref string) bool { return id == ref || publicID == ref }

func (r *stubFormRepo) FindTemplates(_ context.Context, orgID string, ft *forms.FormType) ([]*forms.Template, error) {
	out := make([]*forms.Template, 0)
	for _, t := range r.templates {
		if t.OrgID != orgID {
			continue
		}
		if ft != nil && t.FormType != *ft {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *stubFormRepo) FindTemplateByRef(_ context.Context, orgID, ref string) (*forms.Template, error) {
	for _, t := range r.templates {
		if t.OrgID == orgID && fmatch(t.ID, t.PublicID, ref) {
			return t, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) FindDefaultTemplate(_ context.Context, orgID string, ft forms.FormType) (*forms.Template, error) {
	for _, t := range r.templates {
		if t.OrgID == orgID && t.FormType == ft && t.IsDefault && t.IsActive {
			return t, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) CreateTemplate(_ context.Context, t *forms.Template) error {
	if t.IsDefault {
		for _, e := range r.templates {
			if e.OrgID == t.OrgID && e.FormType == t.FormType {
				e.IsDefault = false
			}
		}
	}
	t.ID = r.next("fmt")
	t.PublicID = "pub_" + t.ID
	t.IsActive = true
	t.CreatedAt, t.UpdatedAt = time.Now(), time.Now()
	r.templates[t.ID] = t
	return nil
}

func (r *stubFormRepo) UpdateTemplate(_ context.Context, t *forms.Template) error {
	if _, ok := r.templates[t.ID]; !ok {
		return forms.ErrTemplateNotFound
	}
	r.templates[t.ID] = t
	return nil
}

func (r *stubFormRepo) SetTemplateDefault(_ context.Context, orgID, templateID string, ft forms.FormType) error {
	target, ok := r.templates[templateID]
	if !ok {
		return forms.ErrTemplateNotFound
	}
	for _, e := range r.templates {
		if e.OrgID == orgID && e.FormType == ft {
			e.IsDefault = false
		}
	}
	target.IsDefault = true
	return nil
}

func (r *stubFormRepo) DeleteTemplate(_ context.Context, _, templateID string) error {
	if _, ok := r.templates[templateID]; !ok {
		return forms.ErrTemplateNotFound
	}
	delete(r.templates, templateID)
	return nil
}

func (r *stubFormRepo) CountInstancesForTemplate(_ context.Context, templateID string) (int, error) {
	n := 0
	for _, i := range r.instances {
		if i.TemplateID != nil && *i.TemplateID == templateID {
			n++
		}
	}
	return n, nil
}

func (r *stubFormRepo) FindSections(_ context.Context, _, templateID string) ([]*forms.Section, error) {
	out := make([]*forms.Section, 0)
	for _, s := range r.sections {
		if s.TemplateID == templateID {
			out = append(out, s)
		}
	}
	// Stable order by DisplayOrder, mirroring the real ORDER BY.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (r *stubFormRepo) FindSectionByRef(_ context.Context, _, ref string) (*forms.Section, error) {
	for _, s := range r.sections {
		if fmatch(s.ID, s.PublicID, ref) {
			return s, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) CreateSection(_ context.Context, s *forms.Section) error {
	s.ID = r.next("fsec")
	s.PublicID = "pub_" + s.ID
	s.CreatedAt, s.UpdatedAt = time.Now(), time.Now()
	r.sections[s.ID] = s
	return nil
}

func (r *stubFormRepo) UpdateSection(_ context.Context, _ string, s *forms.Section) error {
	if _, ok := r.sections[s.ID]; !ok {
		return forms.ErrSectionNotFound
	}
	r.sections[s.ID] = s
	return nil
}

func (r *stubFormRepo) DeleteSection(_ context.Context, _, sectionID string) error {
	if _, ok := r.sections[sectionID]; !ok {
		return forms.ErrSectionNotFound
	}
	delete(r.sections, sectionID)
	return nil
}

func (r *stubFormRepo) FindQuestions(_ context.Context, _, templateID string) ([]*forms.Question, error) {
	out := make([]*forms.Question, 0)
	for _, q := range r.questions {
		sec, ok := r.sections[q.SectionID]
		if !ok || sec.TemplateID != templateID {
			continue
		}
		out = append(out, q)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (r *stubFormRepo) FindQuestionByRef(_ context.Context, _, ref string) (*forms.Question, error) {
	for _, q := range r.questions {
		if fmatch(q.ID, q.PublicID, ref) {
			return q, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) CreateQuestion(_ context.Context, q *forms.Question) error {
	q.ID = r.next("fqst")
	q.PublicID = "pub_" + q.ID
	q.CreatedAt, q.UpdatedAt = time.Now(), time.Now()
	r.questions[q.ID] = q
	return nil
}

func (r *stubFormRepo) UpdateQuestion(_ context.Context, _ string, q *forms.Question) error {
	if _, ok := r.questions[q.ID]; !ok {
		return forms.ErrQuestionNotFound
	}
	r.questions[q.ID] = q
	return nil
}

func (r *stubFormRepo) DeleteQuestion(_ context.Context, _, questionID string) error {
	if _, ok := r.questions[questionID]; !ok {
		return forms.ErrQuestionNotFound
	}
	delete(r.questions, questionID)
	return nil
}

func (r *stubFormRepo) FindInstances(_ context.Context, orgID string, f forms.InstanceListFilter) ([]*forms.Instance, error) {
	out := make([]*forms.Instance, 0)
	for _, i := range r.instances {
		if i.OrgID != orgID {
			continue
		}
		if f.RespondentUserID != "" && (i.RespondentUserID == nil || *i.RespondentUserID != f.RespondentUserID) {
			continue
		}
		if f.Status != "" && string(i.Status) != f.Status {
			continue
		}
		out = append(out, i)
	}
	return out, nil
}

func (r *stubFormRepo) CountInstances(ctx context.Context, orgID string, f forms.InstanceListFilter) (int, error) {
	out, _ := r.FindInstances(ctx, orgID, f)
	return len(out), nil
}

func (r *stubFormRepo) FindInstanceByRef(_ context.Context, orgID, ref string) (*forms.Instance, error) {
	for _, i := range r.instances {
		if i.OrgID == orgID && fmatch(i.ID, i.PublicID, ref) {
			return i, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) InstantiateWithResponses(_ context.Context, inst *forms.Instance, responses []*forms.Response) error {
	inst.ID = r.next("fins")
	inst.PublicID = "pub_" + inst.ID
	inst.Status = forms.InstanceDraft
	inst.CreatedAt, inst.UpdatedAt = time.Now(), time.Now()
	r.instances[inst.ID] = inst
	for _, resp := range responses {
		resp.InstanceID = inst.ID
		resp.ID = r.next("frsp")
		resp.PublicID = "pub_" + resp.ID
		resp.CreatedAt, resp.UpdatedAt = time.Now(), time.Now()
	}
	r.responses[inst.ID] = responses
	return nil
}

func (r *stubFormRepo) SetInstanceStatus(_ context.Context, orgID, instanceID string, status forms.InstanceStatus, submitted bool) (*forms.Instance, error) {
	i, ok := r.instances[instanceID]
	if !ok || i.OrgID != orgID {
		return nil, forms.ErrInstanceNotFound
	}
	i.Status = status
	if submitted && i.SubmittedAt == nil {
		now := time.Now()
		i.SubmittedAt = &now
	}
	return i, nil
}

func (r *stubFormRepo) FindResponses(_ context.Context, instanceID string) ([]*forms.Response, error) {
	return r.responses[instanceID], nil
}

func (r *stubFormRepo) FindResponseByRef(_ context.Context, instanceID, ref string) (*forms.Response, error) {
	for _, resp := range r.responses[instanceID] {
		if fmatch(resp.ID, resp.PublicID, ref) {
			return resp, nil
		}
	}
	return nil, nil
}

func (r *stubFormRepo) SaveAnswers(_ context.Context, instanceID string, answers []*forms.Response) error {
	for _, a := range answers {
		found := false
		for _, existing := range r.responses[instanceID] {
			if existing.ID == a.ID {
				found = true
				break
			}
		}
		if !found {
			return forms.ErrResponseNotFound
		}
		now := time.Now()
		a.AnsweredAt = &now
	}
	return nil
}

var _ forms.Repository = (*stubFormRepo)(nil)

// ── Stub AccessDirectory ─────────────────────────────────────────────────────

type stubFormDirectory struct{ canManage bool }

func (d *stubFormDirectory) Can(_ context.Context, _, _, _, _ string) (bool, error) {
	return d.canManage, nil
}

var _ forms.AccessDirectory = (*stubFormDirectory)(nil)

// ── Helpers ──────────────────────────────────────────────────────────────────

const (
	formOrg      = "org_forms"
	respondentID = "user_respondent"
	outsiderID   = "user_outsider"
)

func newFormSvc(canManage bool) (forms.Service, *stubFormRepo) {
	repo := newStubFormRepo()
	return forms.NewService(repo, &stubFormDirectory{canManage: canManage}), repo
}

// seedTemplate builds a template with one section and the given questions.
func seedTemplate(t *testing.T, svc forms.Service, questions ...forms.CreateQuestionRequest) *forms.Template {
	t.Helper()
	ctx := context.Background()
	tmpl, err := svc.CreateTemplate(ctx, formOrg, "user_admin", forms.CreateTemplateRequest{
		Name: "Annual review", FormType: string(forms.FormTypeAppraisal), IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	sec, err := svc.CreateSection(ctx, formOrg, tmpl.ID, forms.CreateSectionRequest{Title: "Performance"})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	for i, q := range questions {
		q.DisplayOrder = iptr(i)
		if _, err := svc.CreateQuestion(ctx, formOrg, sec.ID, q); err != nil {
			t.Fatalf("create question %d: %v", i, err)
		}
	}
	return tmpl
}

func scaleQuestion(text string, min, max int, weight string, required bool) forms.CreateQuestionRequest {
	return forms.CreateQuestionRequest{
		QuestionText: text, QuestionType: string(forms.QuestionScale),
		ScaleMin: iptr(min), ScaleMax: iptr(max), Weight: dptr(fdec(weight)), IsRequired: required,
	}
}

func seedInstance(t *testing.T, svc forms.Service, tmpl *forms.Template) *forms.InstanceWithResponses {
	t.Helper()
	respondent := respondentID
	inst, err := svc.Instantiate(context.Background(), formOrg, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "emp_1", SubjectLabel: "Alex",
		RespondentUserID: &respondent, RespondentRole: "self", CreatedBy: "user_admin",
	})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return inst
}

// ============================================================
// Snapshotting
// ============================================================

// TestInstantiate_CreatesARowPerQuestionAndSnapshotsDefinition is the
// property that makes a form render as authored forever: the snapshot is
// complete at instantiation, not assembled lazily from a template that may
// since have changed.
func TestInstantiate_CreatesARowPerQuestionAndSnapshotsDefinition(t *testing.T) {
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Delivery", 1, 5, "60", true),
		forms.CreateQuestionRequest{QuestionText: "Comments", QuestionType: string(forms.QuestionTextarea)},
	)
	inst := seedInstance(t, svc, tmpl)

	if len(inst.Responses) != 2 {
		t.Fatalf("expected one response row per question, got %d", len(inst.Responses))
	}
	if inst.TemplateName != "Annual review" {
		t.Errorf("expected the template name snapshotted onto the instance, got %q", inst.TemplateName)
	}
	first := inst.Responses[0]
	if first.QuestionText != "Delivery" || first.SectionTitle != "Performance" {
		t.Errorf("expected the question and section text snapshotted, got %q / %q", first.QuestionText, first.SectionTitle)
	}
	if first.IsAnswered() {
		t.Error("expected a freshly instantiated response to be unanswered")
	}
}

// TestInstantiate_SurvivesTemplateEdits proves the snapshot is a copy, not a
// reference — editing the template after instantiation must not rewrite a
// live form.
func TestInstantiate_SurvivesTemplateEdits(t *testing.T) {
	svc, repo := newFormSvc(false)
	tmpl := seedTemplate(t, svc, scaleQuestion("Original wording", 1, 5, "100", false))
	inst := seedInstance(t, svc, tmpl)

	// Rewrite the question on the template.
	var questionID string
	for id := range repo.questions {
		questionID = id
	}
	if _, err := svc.UpdateQuestion(context.Background(), formOrg, questionID, forms.UpdateQuestionRequest{
		QuestionText: sptr("Completely different wording"),
	}); err != nil {
		t.Fatalf("update question: %v", err)
	}

	fresh, err := svc.GetInstance(context.Background(), formOrg, inst.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if fresh.Responses[0].QuestionText != "Original wording" {
		t.Errorf("expected the instance to keep its snapshot after a template edit, got %q", fresh.Responses[0].QuestionText)
	}
}

func TestInstantiate_RejectsEmptyTemplate(t *testing.T) {
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc) // no questions
	respondent := respondentID
	_, err := svc.Instantiate(context.Background(), formOrg, tmpl.ID, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "emp_1", SubjectLabel: "Alex",
		RespondentUserID: &respondent, CreatedBy: "user_admin",
	})
	if !errors.Is(err, forms.ErrTemplateEmpty) {
		t.Fatalf("expected ErrTemplateEmpty, got %v", err)
	}
}

// TestInstantiateDefault_NoTemplateIsNotAnError pins the contract consumers
// depend on: "no form configured" is a non-event they can ignore.
func TestInstantiateDefault_NoTemplateIsNotAnError(t *testing.T) {
	svc, _ := newFormSvc(false)
	inst, err := svc.InstantiateDefault(context.Background(), formOrg, forms.FormTypeSurvey, forms.SubjectContext{
		SubjectType: forms.SubjectEmployee, SubjectID: "emp_1", SubjectLabel: "Alex", CreatedBy: "user_admin",
	})
	if err != nil {
		t.Fatalf("expected no error when no default template exists, got %v", err)
	}
	if inst != nil {
		t.Errorf("expected a nil instance when no default template exists, got %+v", inst)
	}
}

// ============================================================
// Answer typing
// ============================================================

func TestSaveAnswers_RejectsTypeMismatchAndOutOfRange(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Delivery", 1, 5, "100", false),
		forms.CreateQuestionRequest{
			QuestionText: "Region", QuestionType: string(forms.QuestionSingleSelect),
			Options: []forms.Option{{Value: "emea", Label: "EMEA"}, {Value: "apac", Label: "APAC"}},
		},
	)
	inst := seedInstance(t, svc, tmpl)
	scaleResp, selectResp := inst.Responses[0], inst.Responses[1]

	// Text supplied for a scale question.
	_, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: scaleResp.ID, AnswerText: sptr("great")}},
	})
	if !errors.Is(err, forms.ErrAnswerTypeMismatch) {
		t.Fatalf("expected ErrAnswerTypeMismatch, got %v", err)
	}

	// Above the declared scale maximum.
	_, err = svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: scaleResp.ID, AnswerNumber: dptr(fdec("9"))}},
	})
	if !errors.Is(err, forms.ErrAnswerOutOfRange) {
		t.Fatalf("expected ErrAnswerOutOfRange, got %v", err)
	}

	// An option the question never offered.
	_, err = svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: selectResp.ID, AnswerOptions: []string{"antarctica"}}},
	})
	if !errors.Is(err, forms.ErrOptionNotAllowed) {
		t.Fatalf("expected ErrOptionNotAllowed, got %v", err)
	}

	// A valid answer to each.
	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: scaleResp.ID, AnswerNumber: dptr(fdec("4"))},
			{ResponseID: selectResp.ID, AnswerOptions: []string{"emea"}},
		},
	}); err != nil {
		t.Fatalf("expected valid answers to be accepted, got %v", err)
	}
}

// TestSaveAnswers_ValidatesWholeBatchBeforeWriting proves one bad answer
// cannot leave the rest of a batch persisted.
func TestSaveAnswers_ValidatesWholeBatchBeforeWriting(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Good", 1, 5, "50", false),
		scaleQuestion("Bad", 1, 5, "50", false),
	)
	inst := seedInstance(t, svc, tmpl)

	_, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("4"))},
			{ResponseID: inst.Responses[1].ID, AnswerNumber: dptr(fdec("99"))}, // out of range
		},
	})
	if !errors.Is(err, forms.ErrAnswerOutOfRange) {
		t.Fatalf("expected ErrAnswerOutOfRange, got %v", err)
	}

	fresh, err := svc.GetInstance(ctx, formOrg, inst.ID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if fresh.Responses[0].AnsweredAt != nil {
		t.Error("expected the valid answer in a rejected batch to remain unwritten")
	}
}

// TestSaveAnswers_SwitchingAnswerClearsOtherColumns guards against a stale
// value lingering in a column the question no longer uses.
func TestSaveAnswers_SwitchingAnswerClearsOtherColumns(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc, forms.CreateQuestionRequest{
		QuestionText: "Notes", QuestionType: string(forms.QuestionTextarea),
	})
	inst := seedInstance(t, svc, tmpl)

	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{
			ResponseID: inst.Responses[0].ID,
			AnswerText: sptr("first"),
			// Deliberately supplying a number too — it must be ignored and
			// left clear, because the question is a textarea.
			AnswerNumber: dptr(fdec("7")),
		}},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if inst.Responses[0].AnswerNumber != nil {
		t.Error("expected answer_number to stay clear for a textarea question")
	}
	if inst.Responses[0].AnswerText == nil || *inst.Responses[0].AnswerText != "first" {
		t.Error("expected answer_text to carry the value")
	}
}

// ============================================================
// Submission + immutability
// ============================================================

func TestSubmit_RequiresEveryRequiredQuestionAnswered(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Required", 1, 5, "100", true),
		forms.CreateQuestionRequest{QuestionText: "Optional", QuestionType: string(forms.QuestionText)},
	)
	inst := seedInstance(t, svc, tmpl)

	if _, err := svc.SubmitInstance(ctx, formOrg, inst.ID, respondentID); !errors.Is(err, forms.ErrRequiredUnanswered) {
		t.Fatalf("expected ErrRequiredUnanswered, got %v", err)
	}

	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("5"))}},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := svc.SubmitInstance(ctx, formOrg, inst.ID, respondentID); err != nil {
		t.Fatalf("expected submit to succeed once required answers exist, got %v", err)
	}
}

// TestSubmit_IsImmutable is the finalized-payslip rule applied to forms.
func TestSubmit_IsImmutable(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc, scaleQuestion("Delivery", 1, 5, "100", false))
	inst := seedInstance(t, svc, tmpl)

	if _, err := svc.SubmitInstance(ctx, formOrg, inst.ID, respondentID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("3"))}},
	})
	if !errors.Is(err, forms.ErrInstanceSubmitted) {
		t.Fatalf("expected ErrInstanceSubmitted on edit-after-submit, got %v", err)
	}
	if _, err := svc.SubmitInstance(ctx, formOrg, inst.ID, respondentID); !errors.Is(err, forms.ErrInstanceSubmitted) {
		t.Fatalf("expected a second submit to be refused, got %v", err)
	}
}

// TestSaveAnswers_OnlyRespondentOrManagerMayWrite pins the narrowing the
// route gate cannot express.
func TestSaveAnswers_OnlyRespondentOrManagerMayWrite(t *testing.T) {
	ctx := context.Background()

	t.Run("outsider without manage is refused", func(t *testing.T) {
		svc, _ := newFormSvc(false)
		tmpl := seedTemplate(t, svc, scaleQuestion("Delivery", 1, 5, "100", false))
		inst := seedInstance(t, svc, tmpl)
		_, err := svc.SaveAnswers(ctx, formOrg, inst.ID, outsiderID, forms.SaveAnswersRequest{
			Answers: []forms.AnswerRequest{{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("3"))}},
		})
		if !errors.Is(err, forms.ErrNotRespondent) {
			t.Fatalf("expected ErrNotRespondent, got %v", err)
		}
	})

	t.Run("outsider holding manage may write", func(t *testing.T) {
		svc, _ := newFormSvc(true) // directory grants manage
		tmpl := seedTemplate(t, svc, scaleQuestion("Delivery", 1, 5, "100", false))
		inst := seedInstance(t, svc, tmpl)
		if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, outsiderID, forms.SaveAnswersRequest{
			Answers: []forms.AnswerRequest{{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("3"))}},
		}); err != nil {
			t.Fatalf("expected a manage-holder to write on someone's behalf, got %v", err)
		}
	})
}

// ============================================================
// Scoring
// ============================================================

// TestScore_NormalisesAcrossDifferentScales is the property that lets a 1-10
// question and a 1-5 question coexist without the wider scale dominating.
func TestScore_NormalisesAcrossDifferentScales(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Out of five", 1, 5, "50", false),
		scaleQuestion("Out of ten", 1, 10, "50", false),
	)
	inst := seedInstance(t, svc, tmpl)

	// Both answered at their maximum → 100%, despite different raw numbers.
	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("5"))},
			{ResponseID: inst.Responses[1].ID, AnswerNumber: dptr(fdec("10"))},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	score, err := svc.ScoreInstance(ctx, formOrg, inst.ID)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if !score.Percent.Equal(fdec("100")) {
		t.Errorf("expected 100%% when both scales are maxed, got %s", score.Percent)
	}
	if score.ScoredCount != 2 {
		t.Errorf("expected 2 scored questions, got %d", score.ScoredCount)
	}
}

func TestScore_WeightsAreRespected(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Heavy", 1, 5, "75", false),
		scaleQuestion("Light", 1, 5, "25", false),
	)
	inst := seedInstance(t, svc, tmpl)

	// Heavy at max (1.0), light at min (0.0) → 75%.
	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("5"))},
			{ResponseID: inst.Responses[1].ID, AnswerNumber: dptr(fdec("1"))},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	score, _ := svc.ScoreInstance(ctx, formOrg, inst.ID)
	if !score.Percent.Equal(fdec("75")) {
		t.Errorf("expected 75%% with a 75/25 weighting, got %s", score.Percent)
	}
}

// TestScore_UnscorableQuestionsDoNotDilute pins the decision that free-text
// and unweighted questions contribute nothing rather than counting as zero.
func TestScore_UnscorableQuestionsDoNotDilute(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc,
		scaleQuestion("Scored", 1, 5, "100", false),
		forms.CreateQuestionRequest{QuestionText: "Free text", QuestionType: string(forms.QuestionTextarea)},
		// A scale question with no weight: deliberately excluded.
		forms.CreateQuestionRequest{
			QuestionText: "Unweighted", QuestionType: string(forms.QuestionScale),
			ScaleMin: iptr(1), ScaleMax: iptr(5),
		},
	)
	inst := seedInstance(t, svc, tmpl)

	if _, err := svc.SaveAnswers(ctx, formOrg, inst.ID, respondentID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: inst.Responses[0].ID, AnswerNumber: dptr(fdec("5"))},
			{ResponseID: inst.Responses[1].ID, AnswerText: sptr("some prose")},
			{ResponseID: inst.Responses[2].ID, AnswerNumber: dptr(fdec("1"))},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	score, _ := svc.ScoreInstance(ctx, formOrg, inst.ID)
	if !score.Percent.Equal(fdec("100")) {
		t.Errorf("expected only the weighted question to score (100%%), got %s", score.Percent)
	}
	if score.ScoredCount != 1 {
		t.Errorf("expected exactly 1 contributing question, got %d", score.ScoredCount)
	}
}

// TestScore_NothingScorableIsZeroNotUndefined guards the divide-by-zero the
// weighted average would otherwise hit.
func TestScore_NothingScorableIsZeroNotUndefined(t *testing.T) {
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc, forms.CreateQuestionRequest{
		QuestionText: "Free text only", QuestionType: string(forms.QuestionTextarea),
	})
	inst := seedInstance(t, svc, tmpl)

	if !inst.Score.Percent.Equal(decimal.Zero) || inst.Score.ScoredCount != 0 {
		t.Errorf("expected a zero score with zero contributors, got %+v", inst.Score)
	}
}

// ============================================================
// Template authoring validation
// ============================================================

func TestCreateQuestion_ValidatesPerTypeShape(t *testing.T) {
	ctx := context.Background()
	svc, repo := newFormSvc(false)
	tmpl := seedTemplate(t, svc)
	var sectionID string
	for id, s := range repo.sections {
		if s.TemplateID == tmpl.ID {
			sectionID = id
		}
	}

	// A scale question with no bounds.
	_, err := svc.CreateQuestion(ctx, formOrg, sectionID, forms.CreateQuestionRequest{
		QuestionText: "Unbounded", QuestionType: string(forms.QuestionScale),
	})
	if !errors.Is(err, forms.ErrInvalidScaleBounds) {
		t.Fatalf("expected ErrInvalidScaleBounds, got %v", err)
	}

	// Inverted bounds.
	_, err = svc.CreateQuestion(ctx, formOrg, sectionID, forms.CreateQuestionRequest{
		QuestionText: "Inverted", QuestionType: string(forms.QuestionScale),
		ScaleMin: iptr(5), ScaleMax: iptr(1),
	})
	if !errors.Is(err, forms.ErrInvalidScaleBounds) {
		t.Fatalf("expected ErrInvalidScaleBounds for inverted bounds, got %v", err)
	}

	// A select question with no options.
	_, err = svc.CreateQuestion(ctx, formOrg, sectionID, forms.CreateQuestionRequest{
		QuestionText: "Choose", QuestionType: string(forms.QuestionSingleSelect),
	})
	if !errors.Is(err, forms.ErrOptionsRequired) {
		t.Fatalf("expected ErrOptionsRequired, got %v", err)
	}
}

// TestUpdateQuestion_ValidatesTheMergedShape catches the case where changing
// a question's TYPE leaves it missing fields the new type requires.
func TestUpdateQuestion_ValidatesTheMergedShape(t *testing.T) {
	ctx := context.Background()
	svc, repo := newFormSvc(false)
	seedTemplate(t, svc, forms.CreateQuestionRequest{
		QuestionText: "Plain text", QuestionType: string(forms.QuestionText),
	})
	var questionID string
	for id := range repo.questions {
		questionID = id
	}

	// Switching a text question to a scale, without supplying bounds.
	_, err := svc.UpdateQuestion(ctx, formOrg, questionID, forms.UpdateQuestionRequest{
		QuestionType: sptr(string(forms.QuestionScale)),
	})
	if !errors.Is(err, forms.ErrInvalidScaleBounds) {
		t.Fatalf("expected the merged shape to be validated, got %v", err)
	}
}

func TestDeleteTemplate_BlockedOnceInstancesExist(t *testing.T) {
	ctx := context.Background()
	svc, _ := newFormSvc(false)
	tmpl := seedTemplate(t, svc, scaleQuestion("Delivery", 1, 5, "100", false))
	seedInstance(t, svc, tmpl)

	if err := svc.DeleteTemplate(ctx, formOrg, tmpl.ID); !errors.Is(err, forms.ErrTemplateHasInstances) {
		t.Fatalf("expected ErrTemplateHasInstances, got %v", err)
	}
}

func TestCreateTemplate_RejectsUnknownFormType(t *testing.T) {
	svc, _ := newFormSvc(false)
	_, err := svc.CreateTemplate(context.Background(), formOrg, "user_admin", forms.CreateTemplateRequest{
		Name: "Nope", FormType: "not_a_real_type",
	})
	if !errors.Is(err, forms.ErrInvalidFormType) {
		t.Fatalf("expected ErrInvalidFormType, got %v", err)
	}
}
