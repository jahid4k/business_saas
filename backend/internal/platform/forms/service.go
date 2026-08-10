// backend/internal/platform/forms/service.go
package forms

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const dateLayout = "2006-01-02"

// Service defines business logic for the form engine.
type Service interface {
	// Templates
	ListTemplates(ctx context.Context, orgID string, formType *FormType) ([]*Template, error)
	GetTemplate(ctx context.Context, orgID, ref string) (*TemplateWithSections, error)
	CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*Template, error)
	UpdateTemplate(ctx context.Context, orgID, ref string, req UpdateTemplateRequest) (*Template, error)
	DeleteTemplate(ctx context.Context, orgID, ref string) error

	// Sections
	CreateSection(ctx context.Context, orgID, templateRef string, req CreateSectionRequest) (*Section, error)
	UpdateSection(ctx context.Context, orgID, ref string, req UpdateSectionRequest) (*Section, error)
	DeleteSection(ctx context.Context, orgID, ref string) error

	// Questions
	CreateQuestion(ctx context.Context, orgID, sectionRef string, req CreateQuestionRequest) (*Question, error)
	UpdateQuestion(ctx context.Context, orgID, ref string, req UpdateQuestionRequest) (*Question, error)
	DeleteQuestion(ctx context.Context, orgID, ref string) error

	// Instantiation — deliberately NOT exposed over a generic HTTP route.
	// A generic route would have to trust a client-supplied subject_id and
	// respondent_user_id, which is an impersonation vector; a form response
	// is attributable evidence about a person. Consumers call this from
	// their own endpoints, having resolved the subject from their own domain.
	// The checklists.Instantiate precedent.
	Instantiate(ctx context.Context, orgID, templateRef string, subj SubjectContext) (*InstanceWithResponses, error)
	// InstantiateDefault returns (nil, nil) — not an error — when the org has
	// no default template for formType, so a consumer can treat "no form
	// configured" as a non-event.
	InstantiateDefault(ctx context.Context, orgID string, formType FormType, subj SubjectContext) (*InstanceWithResponses, error)

	// Instances
	ListInstances(ctx context.Context, orgID string, filter InstanceListFilter) (*InstanceListResponse, error)
	GetInstance(ctx context.Context, orgID, ref string) (*InstanceWithResponses, error)
	SaveAnswers(ctx context.Context, orgID, ref, callerUserID string, req SaveAnswersRequest) (*InstanceWithResponses, error)
	SubmitInstance(ctx context.Context, orgID, ref, callerUserID string) (*InstanceWithResponses, error)
	CancelInstance(ctx context.Context, orgID, ref string) (*Instance, error)

	// ScoreInstance is exposed so consumers (appraisals) can read a form's
	// weighted result without re-implementing the arithmetic.
	ScoreInstance(ctx context.Context, orgID, ref string) (Score, error)
}

type serviceImpl struct {
	repo      Repository
	directory AccessDirectory
}

func NewService(repo Repository, directory AccessDirectory) Service {
	return &serviceImpl{repo: repo, directory: directory}
}

// ── Templates ────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListTemplates(ctx context.Context, orgID string, formType *FormType) ([]*Template, error) {
	list, err := s.repo.FindTemplates(ctx, orgID, formType)
	if err != nil {
		return nil, fmt.Errorf("forms: ListTemplates: %w", err)
	}
	if list == nil {
		list = []*Template{}
	}
	return list, nil
}

func (s *serviceImpl) GetTemplate(ctx context.Context, orgID, ref string) (*TemplateWithSections, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("forms: GetTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	sections, questions, err := s.loadDefinition(ctx, orgID, t.ID)
	if err != nil {
		return nil, err
	}
	bySection := map[string][]*Question{}
	for _, q := range questions {
		bySection[q.SectionID] = append(bySection[q.SectionID], q)
	}
	for _, sec := range sections {
		sec.Questions = bySection[sec.ID]
		if sec.Questions == nil {
			sec.Questions = []*Question{}
		}
	}
	return &TemplateWithSections{Template: t, Sections: sections}, nil
}

// loadDefinition fetches a template's sections and questions in two queries
// rather than one per section — the N+1 this engine would otherwise hit on
// every render.
func (s *serviceImpl) loadDefinition(ctx context.Context, orgID, templateID string) ([]*Section, []*Question, error) {
	sections, err := s.repo.FindSections(ctx, orgID, templateID)
	if err != nil {
		return nil, nil, fmt.Errorf("forms: load definition: %w", err)
	}
	questions, err := s.repo.FindQuestions(ctx, orgID, templateID)
	if err != nil {
		return nil, nil, fmt.Errorf("forms: load definition: %w", err)
	}
	if sections == nil {
		sections = []*Section{}
	}
	if questions == nil {
		questions = []*Question{}
	}
	return sections, questions, nil
}

func (s *serviceImpl) CreateTemplate(ctx context.Context, orgID, userID string, req CreateTemplateRequest) (*Template, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	ft := FormType(strings.TrimSpace(req.FormType))
	if !ft.IsValid() {
		return nil, ErrInvalidFormType
	}

	t := &Template{
		OrgID: orgID, Name: name, Description: nilIfBlank(req.Description),
		FormType: ft, IsDefault: req.IsDefault, CreatedBy: userID,
	}
	if err := s.repo.CreateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("forms: CreateTemplate: %w", err)
	}
	return t, nil
}

func (s *serviceImpl) UpdateTemplate(ctx context.Context, orgID, ref string, req UpdateTemplateRequest) (*Template, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("forms: UpdateTemplate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		t.Name = name
	}
	if req.Description != nil {
		t.Description = nilIfBlank(req.Description)
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}

	// Promotion to default goes through the atomic clear-then-set path, never
	// a plain UPDATE, because the partial unique index would otherwise 23505.
	promoting := req.IsDefault != nil && *req.IsDefault && !t.IsDefault
	if req.IsDefault != nil && !*req.IsDefault {
		t.IsDefault = false
	}

	if err := s.repo.UpdateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("forms: UpdateTemplate: %w", err)
	}
	if promoting {
		if err := s.repo.SetTemplateDefault(ctx, orgID, t.ID, t.FormType); err != nil {
			return nil, fmt.Errorf("forms: UpdateTemplate: promote default: %w", err)
		}
		t.IsDefault = true
	}
	return t, nil
}

func (s *serviceImpl) DeleteTemplate(ctx context.Context, orgID, ref string) error {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("forms: DeleteTemplate: %w", err)
	}
	if t == nil {
		return ErrTemplateNotFound
	}

	// Instances snapshot their definition, so they would survive the delete
	// intact — but a template a user is still editing silently diverging from
	// live forms is worse than refusing. Deactivate instead.
	count, err := s.repo.CountInstancesForTemplate(ctx, t.ID)
	if err != nil {
		return fmt.Errorf("forms: DeleteTemplate: count instances: %w", err)
	}
	if count > 0 {
		return ErrTemplateHasInstances
	}

	if err := s.repo.DeleteTemplate(ctx, orgID, t.ID); err != nil {
		return fmt.Errorf("forms: DeleteTemplate: %w", err)
	}
	return nil
}

// ── Sections ─────────────────────────────────────────────────────────────────

func (s *serviceImpl) CreateSection(ctx context.Context, orgID, templateRef string, req CreateSectionRequest) (*Section, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, templateRef)
	if err != nil {
		return nil, fmt.Errorf("forms: CreateSection: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	sec := &Section{TemplateID: t.ID, Title: title, Description: nilIfBlank(req.Description)}
	if req.DisplayOrder != nil {
		sec.DisplayOrder = *req.DisplayOrder
	}
	if err := s.repo.CreateSection(ctx, sec); err != nil {
		return nil, fmt.Errorf("forms: CreateSection: %w", err)
	}
	return sec, nil
}

func (s *serviceImpl) UpdateSection(ctx context.Context, orgID, ref string, req UpdateSectionRequest) (*Section, error) {
	sec, err := s.repo.FindSectionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("forms: UpdateSection: %w", err)
	}
	if sec == nil {
		return nil, ErrSectionNotFound
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		sec.Title = title
	}
	if req.Description != nil {
		sec.Description = nilIfBlank(req.Description)
	}
	if req.DisplayOrder != nil {
		sec.DisplayOrder = *req.DisplayOrder
	}
	if err := s.repo.UpdateSection(ctx, orgID, sec); err != nil {
		return nil, fmt.Errorf("forms: UpdateSection: %w", err)
	}
	return sec, nil
}

func (s *serviceImpl) DeleteSection(ctx context.Context, orgID, ref string) error {
	sec, err := s.repo.FindSectionByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("forms: DeleteSection: %w", err)
	}
	if sec == nil {
		return ErrSectionNotFound
	}
	if err := s.repo.DeleteSection(ctx, orgID, sec.ID); err != nil {
		return fmt.Errorf("forms: DeleteSection: %w", err)
	}
	return nil
}

// ── Questions ────────────────────────────────────────────────────────────────

// validateQuestionShape enforces the per-type rules the schema deliberately
// does not: a CHECK pairing question_type with scale_min/options would fire
// on any UPDATE that changes the type, so the service owns them instead (the
// migration 00076 CHECK-versus-UPDATE reasoning).
func validateQuestionShape(q *Question) error {
	if strings.TrimSpace(q.QuestionText) == "" {
		return ErrQuestionTextRequired
	}
	if !q.QuestionType.IsValid() {
		return ErrInvalidQuestionType
	}
	if q.Weight != nil && q.Weight.IsNegative() {
		return ErrInvalidWeight
	}

	switch q.QuestionType {
	case QuestionScale:
		if q.ScaleMin == nil || q.ScaleMax == nil || *q.ScaleMax <= *q.ScaleMin {
			return ErrInvalidScaleBounds
		}
	case QuestionSingleSelect, QuestionMultiSelect:
		if len(q.Options) == 0 {
			return ErrOptionsRequired
		}
	}
	return nil
}

func (s *serviceImpl) CreateQuestion(ctx context.Context, orgID, sectionRef string, req CreateQuestionRequest) (*Question, error) {
	sec, err := s.repo.FindSectionByRef(ctx, orgID, sectionRef)
	if err != nil {
		return nil, fmt.Errorf("forms: CreateQuestion: %w", err)
	}
	if sec == nil {
		return nil, ErrSectionNotFound
	}

	q := &Question{
		SectionID:    sec.ID,
		QuestionText: strings.TrimSpace(req.QuestionText),
		HelpText:     nilIfBlank(req.HelpText),
		QuestionType: QuestionType(strings.TrimSpace(req.QuestionType)),
		IsRequired:   req.IsRequired,
		ScaleMin:     req.ScaleMin,
		ScaleMax:     req.ScaleMax,
		Options:      req.Options,
		Weight:       req.Weight,
	}
	if q.Options == nil {
		q.Options = []Option{}
	}
	if req.DisplayOrder != nil {
		q.DisplayOrder = *req.DisplayOrder
	}
	if err := validateQuestionShape(q); err != nil {
		return nil, err
	}
	if err := s.repo.CreateQuestion(ctx, q); err != nil {
		return nil, fmt.Errorf("forms: CreateQuestion: %w", err)
	}
	return q, nil
}

func (s *serviceImpl) UpdateQuestion(ctx context.Context, orgID, ref string, req UpdateQuestionRequest) (*Question, error) {
	q, err := s.repo.FindQuestionByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("forms: UpdateQuestion: %w", err)
	}
	if q == nil {
		return nil, ErrQuestionNotFound
	}

	if req.QuestionText != nil {
		q.QuestionText = strings.TrimSpace(*req.QuestionText)
	}
	if req.HelpText != nil {
		q.HelpText = nilIfBlank(req.HelpText)
	}
	if req.QuestionType != nil {
		q.QuestionType = QuestionType(strings.TrimSpace(*req.QuestionType))
	}
	if req.IsRequired != nil {
		q.IsRequired = *req.IsRequired
	}
	if req.DisplayOrder != nil {
		q.DisplayOrder = *req.DisplayOrder
	}
	if req.ScaleMin != nil {
		q.ScaleMin = req.ScaleMin
	}
	if req.ScaleMax != nil {
		q.ScaleMax = req.ScaleMax
	}
	if req.Options != nil {
		q.Options = req.Options
	}
	if req.Weight != nil {
		q.Weight = req.Weight
	}

	// Validated against the MERGED question, so a type change is checked
	// against the fields it now needs rather than the ones it arrived with.
	if err := validateQuestionShape(q); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateQuestion(ctx, orgID, q); err != nil {
		return nil, fmt.Errorf("forms: UpdateQuestion: %w", err)
	}
	return q, nil
}

func (s *serviceImpl) DeleteQuestion(ctx context.Context, orgID, ref string) error {
	q, err := s.repo.FindQuestionByRef(ctx, orgID, ref)
	if err != nil {
		return fmt.Errorf("forms: DeleteQuestion: %w", err)
	}
	if q == nil {
		return ErrQuestionNotFound
	}
	if err := s.repo.DeleteQuestion(ctx, orgID, q.ID); err != nil {
		return fmt.Errorf("forms: DeleteQuestion: %w", err)
	}
	return nil
}

// ── Instantiation ────────────────────────────────────────────────────────────

func (s *serviceImpl) Instantiate(ctx context.Context, orgID, templateRef string, subj SubjectContext) (*InstanceWithResponses, error) {
	t, err := s.repo.FindTemplateByRef(ctx, orgID, templateRef)
	if err != nil {
		return nil, fmt.Errorf("forms: Instantiate: %w", err)
	}
	if t == nil {
		return nil, ErrTemplateNotFound
	}
	return s.instantiate(ctx, orgID, t, subj)
}

func (s *serviceImpl) InstantiateDefault(ctx context.Context, orgID string, formType FormType, subj SubjectContext) (*InstanceWithResponses, error) {
	t, err := s.repo.FindDefaultTemplate(ctx, orgID, formType)
	if err != nil {
		return nil, fmt.Errorf("forms: InstantiateDefault: %w", err)
	}
	// No default configured is a non-event, not an error — the caller decides
	// whether that matters. The checklists.InstantiateDefault contract.
	if t == nil {
		return nil, nil
	}
	return s.instantiate(ctx, orgID, t, subj)
}

func (s *serviceImpl) instantiate(ctx context.Context, orgID string, t *Template, subj SubjectContext) (*InstanceWithResponses, error) {
	if !subj.SubjectType.IsValid() {
		return nil, ErrInvalidSubjectType
	}

	sections, questions, err := s.loadDefinition(ctx, orgID, t.ID)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return nil, ErrTemplateEmpty
	}
	sectionTitle := map[string]string{}
	for _, sec := range sections {
		sectionTitle[sec.ID] = sec.Title
	}

	inst := &Instance{
		OrgID: orgID, TemplateID: &t.ID,
		// Snapshot: a renamed or deleted template must not change how a live
		// instance renders.
		TemplateName: t.Name, FormType: t.FormType,
		SubjectType: subj.SubjectType, SubjectID: subj.SubjectID, SubjectLabel: subj.SubjectLabel,
		RespondentUserID: subj.RespondentUserID, CreatedBy: subj.CreatedBy,
	}
	if role := strings.TrimSpace(subj.RespondentRole); role != "" {
		inst.RespondentRole = &role
	}

	// One response row per question, created now with a null answer. This is
	// what makes the snapshot complete: the form renders as authored even if
	// the template is later edited, and unanswered questions are visible
	// rather than merely absent.
	responses := make([]*Response, 0, len(questions))
	for i, q := range questions {
		qid := q.ID
		responses = append(responses, &Response{
			QuestionID:   &qid,
			SectionTitle: sectionTitle[q.SectionID],
			QuestionText: q.QuestionText,
			QuestionType: q.QuestionType,
			IsRequired:   q.IsRequired,
			// Renumbered densely across the whole form so the render order is
			// stable even if sections and questions share ordinal values.
			DisplayOrder: i,
			ScaleMin:     q.ScaleMin,
			ScaleMax:     q.ScaleMax,
			Options:      q.Options,
			Weight:       q.Weight,
		})
	}

	if err := s.repo.InstantiateWithResponses(ctx, inst, responses); err != nil {
		return nil, fmt.Errorf("forms: Instantiate: %w", err)
	}
	return &InstanceWithResponses{Instance: inst, Responses: responses, Score: computeScore(responses)}, nil
}

// ── Instances ────────────────────────────────────────────────────────────────

func (s *serviceImpl) ListInstances(ctx context.Context, orgID string, filter InstanceListFilter) (*InstanceListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindInstances(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("forms: ListInstances: %w", err)
	}
	if list == nil {
		list = []*Instance{}
	}
	total, err := s.repo.CountInstances(ctx, orgID, filter)
	if err != nil {
		return nil, fmt.Errorf("forms: ListInstances: count: %w", err)
	}
	return &InstanceListResponse{Instances: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *serviceImpl) GetInstance(ctx context.Context, orgID, ref string) (*InstanceWithResponses, error) {
	inst, responses, err := s.loadInstance(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	return &InstanceWithResponses{Instance: inst, Responses: responses, Score: computeScore(responses)}, nil
}

func (s *serviceImpl) loadInstance(ctx context.Context, orgID, ref string) (*Instance, []*Response, error) {
	inst, err := s.repo.FindInstanceByRef(ctx, orgID, ref)
	if err != nil {
		return nil, nil, fmt.Errorf("forms: load instance: %w", err)
	}
	if inst == nil {
		return nil, nil, ErrInstanceNotFound
	}
	responses, err := s.repo.FindResponses(ctx, inst.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("forms: load instance: %w", err)
	}
	if responses == nil {
		responses = []*Response{}
	}
	return inst, responses, nil
}

// authorizeRespond narrows the broadly-granted platform.forms.respond
// permission to the instance's actual respondent. The route gate cannot
// express "is this YOUR form", so it does not try — the
// platform.checklists.complete precedent.
//
// A caller holding platform.forms.manage may also write, which is what lets
// an admin correct a form on someone's behalf.
func (s *serviceImpl) authorizeRespond(ctx context.Context, orgID, callerUserID string, inst *Instance) error {
	if inst.RespondentUserID != nil && *inst.RespondentUserID == callerUserID {
		return nil
	}
	ok, err := s.directory.Can(ctx, callerUserID, orgID, "platform.forms", "manage")
	if err != nil {
		return fmt.Errorf("forms: authorize respond: %w", err)
	}
	if !ok {
		return ErrNotRespondent
	}
	return nil
}

// applyAnswer validates one answer against its question snapshot and writes
// it into the typed column that question's type selects. Every other answer
// column is cleared, so switching an answer can never leave a stale value in
// a column the question no longer uses.
func applyAnswer(resp *Response, a AnswerRequest) error {
	resp.AnswerText, resp.AnswerNumber = nil, nil
	resp.AnswerBoolean, resp.AnswerDate, resp.AnswerOptions = nil, nil, nil

	switch resp.QuestionType {
	case QuestionText, QuestionTextarea:
		if a.AnswerText == nil {
			return ErrAnswerTypeMismatch
		}
		resp.AnswerText = a.AnswerText

	case QuestionNumber:
		if a.AnswerNumber == nil {
			return ErrAnswerTypeMismatch
		}
		resp.AnswerNumber = a.AnswerNumber

	case QuestionScale:
		if a.AnswerNumber == nil {
			return ErrAnswerTypeMismatch
		}
		if resp.ScaleMin != nil && a.AnswerNumber.LessThan(decimal.NewFromInt(int64(*resp.ScaleMin))) {
			return ErrAnswerOutOfRange
		}
		if resp.ScaleMax != nil && a.AnswerNumber.GreaterThan(decimal.NewFromInt(int64(*resp.ScaleMax))) {
			return ErrAnswerOutOfRange
		}
		resp.AnswerNumber = a.AnswerNumber

	case QuestionBoolean:
		if a.AnswerBoolean == nil {
			return ErrAnswerTypeMismatch
		}
		resp.AnswerBoolean = a.AnswerBoolean

	case QuestionDate:
		if a.AnswerDate == nil || strings.TrimSpace(*a.AnswerDate) == "" {
			return ErrAnswerTypeMismatch
		}
		d, err := time.Parse(dateLayout, strings.TrimSpace(*a.AnswerDate))
		if err != nil {
			return ErrInvalidDate
		}
		resp.AnswerDate = &d

	case QuestionSingleSelect:
		if len(a.AnswerOptions) != 1 {
			return ErrAnswerTypeMismatch
		}
		if !optionAllowed(resp.Options, a.AnswerOptions[0]) {
			return ErrOptionNotAllowed
		}
		resp.AnswerOptions = a.AnswerOptions

	case QuestionMultiSelect:
		if len(a.AnswerOptions) == 0 {
			return ErrAnswerTypeMismatch
		}
		for _, v := range a.AnswerOptions {
			if !optionAllowed(resp.Options, v) {
				return ErrOptionNotAllowed
			}
		}
		resp.AnswerOptions = a.AnswerOptions

	default:
		return ErrInvalidQuestionType
	}
	return nil
}

func optionAllowed(options []Option, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

func (s *serviceImpl) SaveAnswers(ctx context.Context, orgID, ref, callerUserID string, req SaveAnswersRequest) (*InstanceWithResponses, error) {
	inst, responses, err := s.loadInstance(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if err := s.assertEditable(inst); err != nil {
		return nil, err
	}
	if err := s.authorizeRespond(ctx, orgID, callerUserID, inst); err != nil {
		return nil, err
	}

	byRef := map[string]*Response{}
	for _, r := range responses {
		byRef[r.ID] = r
		byRef[r.PublicID] = r
	}

	// Every answer is validated BEFORE any is written, so one bad answer in a
	// batch cannot leave the rest persisted.
	toSave := make([]*Response, 0, len(req.Answers))
	for _, a := range req.Answers {
		resp, ok := byRef[strings.TrimSpace(a.ResponseID)]
		if !ok {
			return nil, ErrResponseNotFound
		}
		if err := applyAnswer(resp, a); err != nil {
			return nil, err
		}
		toSave = append(toSave, resp)
	}

	if err := s.repo.SaveAnswers(ctx, inst.ID, toSave); err != nil {
		return nil, err
	}
	return &InstanceWithResponses{Instance: inst, Responses: responses, Score: computeScore(responses)}, nil
}

// assertEditable is the immutability rule: a submitted form can never be
// edited again, the hrm_interview_scorecards and finalized-payslip precedent.
func (s *serviceImpl) assertEditable(inst *Instance) error {
	if inst.Status == InstanceCancelled {
		return ErrInstanceCancelled
	}
	if inst.Status == InstanceSubmitted || inst.SubmittedAt != nil {
		return ErrInstanceSubmitted
	}
	return nil
}

func (s *serviceImpl) SubmitInstance(ctx context.Context, orgID, ref, callerUserID string) (*InstanceWithResponses, error) {
	inst, responses, err := s.loadInstance(ctx, orgID, ref)
	if err != nil {
		return nil, err
	}
	if err := s.assertEditable(inst); err != nil {
		return nil, err
	}
	if err := s.authorizeRespond(ctx, orgID, callerUserID, inst); err != nil {
		return nil, err
	}

	// Required questions are enforced at submit, not at save — a long form
	// must be fillable across several sittings.
	for _, r := range responses {
		if r.IsRequired && !r.IsAnswered() {
			return nil, ErrRequiredUnanswered
		}
	}

	updated, err := s.repo.SetInstanceStatus(ctx, orgID, inst.ID, InstanceSubmitted, true)
	if err != nil {
		return nil, fmt.Errorf("forms: SubmitInstance: %w", err)
	}
	return &InstanceWithResponses{Instance: updated, Responses: responses, Score: computeScore(responses)}, nil
}

func (s *serviceImpl) CancelInstance(ctx context.Context, orgID, ref string) (*Instance, error) {
	inst, err := s.repo.FindInstanceByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("forms: CancelInstance: %w", err)
	}
	if inst == nil {
		return nil, ErrInstanceNotFound
	}
	if inst.Status == InstanceSubmitted {
		return nil, ErrInstanceSubmitted
	}
	updated, err := s.repo.SetInstanceStatus(ctx, orgID, inst.ID, InstanceCancelled, false)
	if err != nil {
		return nil, fmt.Errorf("forms: CancelInstance: %w", err)
	}
	return updated, nil
}

func (s *serviceImpl) ScoreInstance(ctx context.Context, orgID, ref string) (Score, error) {
	_, responses, err := s.loadInstance(ctx, orgID, ref)
	if err != nil {
		return Score{}, err
	}
	return computeScore(responses), nil
}

// computeScore is the weighted result of a form's numeric answers, always
// computed and never stored — the platform_checklist_instances rule.
//
// Each contributing answer is normalised to 0-1 against its own scale before
// weighting, so a 1-10 question and a 1-5 question can sit in the same form
// without the wider scale silently dominating. A question with no weight, no
// answer, or a non-numeric type contributes nothing.
func computeScore(responses []*Response) Score {
	weighted := decimal.Zero
	totalWeight := decimal.Zero
	count := 0

	for _, r := range responses {
		if r.Weight == nil || r.Weight.IsZero() || r.AnswerNumber == nil || !r.QuestionType.IsScorable() {
			continue
		}
		norm, ok := normalise(r)
		if !ok {
			continue
		}
		weighted = weighted.Add(norm.Mul(*r.Weight))
		totalWeight = totalWeight.Add(*r.Weight)
		count++
	}

	if totalWeight.IsZero() {
		return Score{Percent: decimal.Zero, ScoredCount: 0, TotalWeight: decimal.Zero}
	}
	pct := weighted.Div(totalWeight).Mul(decimal.NewFromInt(100)).Round(2)
	return Score{Percent: pct, ScoredCount: count, TotalWeight: totalWeight}
}

// normalise maps an answer onto 0-1 within its question's own bounds. A
// 'number' question has no declared bounds, so it cannot be normalised and is
// excluded rather than guessed at.
func normalise(r *Response) (decimal.Decimal, bool) {
	if r.QuestionType != QuestionScale || r.ScaleMin == nil || r.ScaleMax == nil {
		return decimal.Zero, false
	}
	min := decimal.NewFromInt(int64(*r.ScaleMin))
	max := decimal.NewFromInt(int64(*r.ScaleMax))
	span := max.Sub(min)
	if span.IsZero() {
		return decimal.Zero, false
	}
	n := r.AnswerNumber.Sub(min).Div(span)
	if n.IsNegative() {
		return decimal.Zero, true
	}
	if n.GreaterThan(decimal.NewFromInt(1)) {
		return decimal.NewFromInt(1), true
	}
	return n, true
}

func nilIfBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	return &trimmed
}
