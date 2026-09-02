// backend/internal/platform/forms/model.go
package forms

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Package forms is the configurable form primitive: sections → typed
// questions → typed responses → scoring. Definitions are snapshotted onto
// each instance so historical records render exactly as authored.
//
// Like every internal/platform package, this one imports nothing from
// internal/ except internal/middleware (in handler.go). Anything it needs
// from authz is declared here as a narrow interface and satisfied
// structurally — see AccessDirectory.

// AccessDirectory is the minimal slice of authz.Service this package needs.
// Declared locally so this package gains no platform → authz import edge;
// authz.Service satisfies it structurally.
//
// The parameter ORDER is load-bearing: satisfaction is structural, not
// declared, so it must match authz.Service.Can exactly. The
// checklists.AccessDirectory precedent.
type AccessDirectory interface {
	Can(ctx context.Context, userID, orgID, resource, action string) (bool, error)
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// FormType discriminates one engine across consumers rather than forking a
// table per consumer — the checklist_type precedent. All known consumers are
// enumerated now; only 'appraisal' has one in this phase.
type FormType string

const (
	FormTypeAppraisal     FormType = "appraisal"
	FormTypeFeedback360   FormType = "feedback_360"
	FormTypeSurvey        FormType = "survey"
	FormTypeAssessment    FormType = "assessment"
	FormTypeExitInterview FormType = "exit_interview"
	FormTypeCustom        FormType = "custom"
)

func (f FormType) IsValid() bool {
	switch f {
	case FormTypeAppraisal, FormTypeFeedback360, FormTypeSurvey,
		FormTypeAssessment, FormTypeExitInterview, FormTypeCustom:
		return true
	}
	return false
}

// QuestionType selects which typed answer column a response writes to.
type QuestionType string

const (
	QuestionText         QuestionType = "text"
	QuestionTextarea     QuestionType = "textarea"
	QuestionNumber       QuestionType = "number"
	QuestionScale        QuestionType = "scale"
	QuestionSingleSelect QuestionType = "single_select"
	QuestionMultiSelect  QuestionType = "multi_select"
	QuestionBoolean      QuestionType = "boolean"
	QuestionDate         QuestionType = "date"
)

func (q QuestionType) IsValid() bool {
	switch q {
	case QuestionText, QuestionTextarea, QuestionNumber, QuestionScale,
		QuestionSingleSelect, QuestionMultiSelect, QuestionBoolean, QuestionDate:
		return true
	}
	return false
}

// IsScorable reports whether this type can contribute to a form's score.
// Only numeric answers can: a free-text answer has no defensible numeric
// interpretation, and inventing one would silently skew every aggregate.
func (q QuestionType) IsScorable() bool {
	return q == QuestionScale || q == QuestionNumber
}

type InstanceStatus string

const (
	InstanceDraft     InstanceStatus = "draft"
	InstanceSubmitted InstanceStatus = "submitted"
	InstanceCancelled InstanceStatus = "cancelled"
)

// SubjectType is who a form is ABOUT. Widen the CHECK in a migration, not
// this shape, when a new consumer arrives.
type SubjectType string

const (
	SubjectEmployee  SubjectType = "employee"
	SubjectCandidate SubjectType = "candidate"
)

func (s SubjectType) IsValid() bool {
	switch s {
	case SubjectEmployee, SubjectCandidate:
		return true
	}
	return false
}

// ── Definition ───────────────────────────────────────────────────────────────

type Template struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	FormType    FormType  `db:"form_type"   json:"form_type"`
	IsDefault   bool      `db:"is_default"  json:"is_default"`
	IsActive    bool      `db:"is_active"   json:"is_active"`
	CreatedBy   string    `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type Section struct {
	ID           string    `db:"id"            json:"id"`
	PublicID     string    `db:"public_id"     json:"public_id"`
	TemplateID   string    `db:"template_id"   json:"template_id"`
	Title        string    `db:"title"         json:"title"`
	Description  *string   `db:"description"   json:"description,omitempty"`
	DisplayOrder int       `db:"display_order" json:"display_order"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`

	Questions []*Question `db:"-" json:"questions,omitempty"`
}

// Option is one choice on a select question.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Question struct {
	ID           string       `db:"id"            json:"id"`
	PublicID     string       `db:"public_id"     json:"public_id"`
	SectionID    string       `db:"section_id"    json:"section_id"`
	QuestionText string       `db:"question_text" json:"question_text"`
	HelpText     *string      `db:"help_text"     json:"help_text,omitempty"`
	QuestionType QuestionType `db:"question_type" json:"question_type"`
	IsRequired   bool         `db:"is_required"   json:"is_required"`
	DisplayOrder int          `db:"display_order" json:"display_order"`
	ScaleMin     *int         `db:"scale_min"     json:"scale_min,omitempty"`
	ScaleMax     *int         `db:"scale_max"     json:"scale_max,omitempty"`
	// OptionsRaw is the JSONB column; Options is the parsed view. Raw is
	// json:"-" so the API never emits the byte slice.
	OptionsRaw []byte           `db:"options" json:"-"`
	Options    []Option         `db:"-"       json:"options"`
	Weight     *decimal.Decimal `db:"weight"  json:"weight,omitempty"`
	CreatedAt  time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time        `db:"updated_at" json:"updated_at"`
}

// ParseOptions decodes OptionsRaw into Options. An empty column yields an
// empty slice rather than nil, so JSON renders [] not null.
func (q *Question) ParseOptions() error {
	if len(q.OptionsRaw) == 0 {
		q.Options = []Option{}
		return nil
	}
	return json.Unmarshal(q.OptionsRaw, &q.Options)
}

// TemplateWithSections is the authored definition, fully hydrated.
type TemplateWithSections struct {
	*Template
	Sections []*Section `json:"sections"`
}

// ── Instance ─────────────────────────────────────────────────────────────────

// SubjectContext is what a consumer passes to instantiate a form. The engine
// never dereferences these values — it stores them. Resolving them is the
// consumer's job, which is exactly what keeps this package free of hrm_*.
type SubjectContext struct {
	SubjectType      SubjectType
	SubjectID        string
	SubjectLabel     string
	RespondentUserID *string
	RespondentRole   string
	CreatedBy        string
}

type Instance struct {
	ID               string         `db:"id"                 json:"id"`
	PublicID         string         `db:"public_id"          json:"public_id"`
	OrgID            string         `db:"org_id"             json:"org_id"`
	TemplateID       *string        `db:"template_id"        json:"template_id,omitempty"`
	TemplateName     string         `db:"template_name"      json:"template_name"`
	FormType         FormType       `db:"form_type"          json:"form_type"`
	SubjectType      SubjectType    `db:"subject_type"       json:"subject_type"`
	SubjectID        string         `db:"subject_id"         json:"subject_id"`
	SubjectLabel     string         `db:"subject_label"      json:"subject_label"`
	RespondentUserID *string        `db:"respondent_user_id" json:"respondent_user_id,omitempty"`
	RespondentRole   *string        `db:"respondent_role"    json:"respondent_role,omitempty"`
	Status           InstanceStatus `db:"status"             json:"status"`
	SubmittedAt      *time.Time     `db:"submitted_at"       json:"submitted_at,omitempty"`
	CreatedBy        string         `db:"created_by"         json:"created_by"`
	CreatedAt        time.Time      `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"         json:"updated_at"`
}

// Response is one question's snapshot plus its typed answer. A row exists
// for every question from instantiation; answering is an UPDATE.
type Response struct {
	ID         string  `db:"id"          json:"id"`
	PublicID   string  `db:"public_id"   json:"public_id"`
	InstanceID string  `db:"instance_id" json:"instance_id"`
	QuestionID *string `db:"question_id" json:"question_id,omitempty"`

	// Frozen question snapshot.
	SectionTitle string           `db:"section_title" json:"section_title"`
	QuestionText string           `db:"question_text" json:"question_text"`
	QuestionType QuestionType     `db:"question_type" json:"question_type"`
	IsRequired   bool             `db:"is_required"   json:"is_required"`
	DisplayOrder int              `db:"display_order" json:"display_order"`
	ScaleMin     *int             `db:"scale_min"     json:"scale_min,omitempty"`
	ScaleMax     *int             `db:"scale_max"     json:"scale_max,omitempty"`
	OptionsRaw   []byte           `db:"options"       json:"-"`
	Options      []Option         `db:"-"             json:"options"`
	Weight       *decimal.Decimal `db:"weight"        json:"weight,omitempty"`

	// Typed answer — exactly one is populated, selected by QuestionType.
	AnswerText    *string          `db:"answer_text"    json:"answer_text,omitempty"`
	AnswerNumber  *decimal.Decimal `db:"answer_number"  json:"answer_number,omitempty"`
	AnswerBoolean *bool            `db:"answer_boolean" json:"answer_boolean,omitempty"`
	AnswerDate    *time.Time       `db:"answer_date"    json:"answer_date,omitempty"`
	AnswerOptions []string         `db:"answer_options" json:"answer_options,omitempty"`

	AnsweredAt *time.Time `db:"answered_at" json:"answered_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"  json:"updated_at"`
}

func (r *Response) ParseOptions() error {
	if len(r.OptionsRaw) == 0 {
		r.Options = []Option{}
		return nil
	}
	return json.Unmarshal(r.OptionsRaw, &r.Options)
}

// IsAnswered reports whether any typed answer column carries a value.
func (r *Response) IsAnswered() bool {
	return r.AnswerText != nil || r.AnswerNumber != nil || r.AnswerBoolean != nil ||
		r.AnswerDate != nil || len(r.AnswerOptions) > 0
}

// Score is a form instance's weighted numeric result. Always computed from
// responses, never stored — the platform_checklist_instances rule ("progress
// is ALWAYS computed ... no denormalized counter to drift").
type Score struct {
	// Earned is the weighted sum of normalised answers, 0-100.
	Percent decimal.Decimal `json:"percent"`
	// ScoredCount is how many questions contributed. Zero means the form has
	// no scorable answered questions, and Percent is zero rather than
	// meaningless.
	ScoredCount int `json:"scored_count"`
	// MaxWeight is the total weight of contributing questions, exposed so a
	// consumer can tell "scored 0 of 40" from "nothing scorable at all".
	TotalWeight decimal.Decimal `json:"total_weight"`
}

// InstanceWithResponses is an instance plus its questions/answers and score.
type InstanceWithResponses struct {
	*Instance
	Responses []*Response `json:"responses"`
	Score     Score       `json:"score"`
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateTemplateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	FormType    string  `json:"form_type"`
	IsDefault   bool    `json:"is_default"`
}

type UpdateTemplateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *bool   `json:"is_default"`
	IsActive    *bool   `json:"is_active"`
}

type CreateSectionRequest struct {
	Title        string  `json:"title"`
	Description  *string `json:"description"`
	DisplayOrder *int    `json:"display_order"`
}

type UpdateSectionRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	DisplayOrder *int    `json:"display_order"`
}

type CreateQuestionRequest struct {
	QuestionText string           `json:"question_text"`
	HelpText     *string          `json:"help_text"`
	QuestionType string           `json:"question_type"`
	IsRequired   bool             `json:"is_required"`
	DisplayOrder *int             `json:"display_order"`
	ScaleMin     *int             `json:"scale_min"`
	ScaleMax     *int             `json:"scale_max"`
	Options      []Option         `json:"options"`
	Weight       *decimal.Decimal `json:"weight"`
}

type UpdateQuestionRequest struct {
	QuestionText *string          `json:"question_text"`
	HelpText     *string          `json:"help_text"`
	QuestionType *string          `json:"question_type"`
	IsRequired   *bool            `json:"is_required"`
	DisplayOrder *int             `json:"display_order"`
	ScaleMin     *int             `json:"scale_min"`
	ScaleMax     *int             `json:"scale_max"`
	Options      []Option         `json:"options"`
	Weight       *decimal.Decimal `json:"weight"`
}

// AnswerRequest is one question's answer. Only the field matching the
// question's type is read; the rest are ignored.
type AnswerRequest struct {
	ResponseID    string           `json:"response_id"`
	AnswerText    *string          `json:"answer_text"`
	AnswerNumber  *decimal.Decimal `json:"answer_number"`
	AnswerBoolean *bool            `json:"answer_boolean"`
	AnswerDate    *string          `json:"answer_date"`
	AnswerOptions []string         `json:"answer_options"`
}

// SaveAnswersRequest is a partial save: only the listed responses change, so
// a long form can be filled in over several sittings.
type SaveAnswersRequest struct {
	Answers []AnswerRequest `json:"answers"`
}

type InstanceListFilter struct {
	FormType         string
	SubjectType      string
	SubjectID        string
	RespondentUserID string
	Status           string
	Limit            int
	Offset           int
}

func (f *InstanceListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type InstanceListResponse struct {
	Instances []*Instance `json:"instances"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

// ── Sentinel errors ──────────────────────────────────────────────────────────

var (
	ErrTemplateNotFound = errors.New("form template not found")
	ErrSectionNotFound  = errors.New("form section not found")
	ErrQuestionNotFound = errors.New("form question not found")
	ErrInstanceNotFound = errors.New("form instance not found")
	ErrResponseNotFound = errors.New("form response not found")

	ErrNameRequired         = errors.New("name is required")
	ErrTitleRequired        = errors.New("title is required")
	ErrQuestionTextRequired = errors.New("question_text is required")
	ErrInvalidFormType      = errors.New("form_type must be one of: appraisal, feedback_360, survey, assessment, exit_interview, custom")
	ErrInvalidQuestionType  = errors.New("question_type must be one of: text, textarea, number, scale, single_select, multi_select, boolean, date")
	ErrInvalidSubjectType   = errors.New("subject_type must be one of: employee, candidate")
	ErrInvalidScaleBounds   = errors.New("scale_max must be greater than scale_min, and both are required for a scale question")
	ErrOptionsRequired      = errors.New("a select question requires at least one option")
	ErrInvalidWeight        = errors.New("weight must be zero or greater")
	ErrInvalidDate          = errors.New("answer_date must be a valid date in YYYY-MM-DD format")

	// ErrTemplateHasInstances blocks destructive edits to a template that
	// live instances were built from. Instances snapshot their definition, so
	// they would survive — but silently diverging from the template a user is
	// still editing is worse than refusing.
	ErrTemplateHasInstances = errors.New("this template has form instances and cannot be deleted — deactivate it instead")
	ErrInstanceSubmitted    = errors.New("this form has been submitted and can no longer be edited")
	ErrInstanceCancelled    = errors.New("this form has been cancelled")
	ErrNotRespondent        = errors.New("only the assigned respondent may answer this form")
	ErrAnswerTypeMismatch   = errors.New("the supplied answer does not match the question's type")
	ErrAnswerOutOfRange     = errors.New("the answer falls outside the question's scale bounds")
	ErrOptionNotAllowed     = errors.New("the selected option is not one of the question's options")
	ErrRequiredUnanswered   = errors.New("every required question must be answered before submitting")
	ErrTemplateEmpty        = errors.New("a form template needs at least one question before it can be instantiated")
)
