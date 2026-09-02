// backend/internal/platform/forms/repository.go
package forms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the data access interface for the form engine.
//
// TENANT ISOLATION: every method takes orgID. Sections, questions and
// responses have no org_id column of their own — every query reaches them by
// JOINing through their parent (template or instance) and filtering the
// parent's org_id, the platform_checklist_* and hrm_approval_decisions
// precedent.
//
// InstantiateWithResponses owns its transaction inside the repository rather
// than accepting a caller-supplied pgx.Tx: no platform service holds a
// *pgxpool.Pool, and instantiation is a single logical write from the
// consumer's point of view.
type Repository interface {
	// Templates
	FindTemplates(ctx context.Context, orgID string, formType *FormType) ([]*Template, error)
	FindTemplateByRef(ctx context.Context, orgID, ref string) (*Template, error)
	FindDefaultTemplate(ctx context.Context, orgID string, formType FormType) (*Template, error)
	CreateTemplate(ctx context.Context, t *Template) error
	UpdateTemplate(ctx context.Context, t *Template) error
	// SetTemplateDefault atomically clears any existing default for the same
	// (org, form_type) before setting this one — the guard the partial unique
	// index requires and crm_pipelines never got.
	SetTemplateDefault(ctx context.Context, orgID, templateID string, formType FormType) error
	DeleteTemplate(ctx context.Context, orgID, templateID string) error
	CountInstancesForTemplate(ctx context.Context, templateID string) (int, error)

	// Sections + questions
	FindSections(ctx context.Context, orgID, templateID string) ([]*Section, error)
	FindSectionByRef(ctx context.Context, orgID, ref string) (*Section, error)
	CreateSection(ctx context.Context, s *Section) error
	UpdateSection(ctx context.Context, orgID string, s *Section) error
	DeleteSection(ctx context.Context, orgID, sectionID string) error

	FindQuestions(ctx context.Context, orgID, templateID string) ([]*Question, error)
	FindQuestionByRef(ctx context.Context, orgID, ref string) (*Question, error)
	CreateQuestion(ctx context.Context, q *Question) error
	UpdateQuestion(ctx context.Context, orgID string, q *Question) error
	DeleteQuestion(ctx context.Context, orgID, questionID string) error

	// Instances
	FindInstances(ctx context.Context, orgID string, filter InstanceListFilter) ([]*Instance, error)
	CountInstances(ctx context.Context, orgID string, filter InstanceListFilter) (int, error)
	FindInstanceByRef(ctx context.Context, orgID, ref string) (*Instance, error)
	// InstantiateWithResponses inserts the instance AND one response row per
	// question in one transaction. A partial instantiation would leave a form
	// missing questions with no way to tell which.
	InstantiateWithResponses(ctx context.Context, inst *Instance, responses []*Response) error
	SetInstanceStatus(ctx context.Context, orgID, instanceID string, status InstanceStatus, submitted bool) (*Instance, error)

	// Responses
	FindResponses(ctx context.Context, instanceID string) ([]*Response, error)
	FindResponseByRef(ctx context.Context, instanceID, ref string) (*Response, error)
	// SaveAnswers writes several answers in one transaction, so a partial
	// save cannot leave half a page of a long form persisted.
	SaveAnswers(ctx context.Context, instanceID string, answers []*Response) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Templates ────────────────────────────────────────────────────────────────

const templateCols = `id, public_id, org_id, name, description, form_type, is_default, is_active,
	created_by, created_at, updated_at`

func scanTemplate(row interface{ Scan(...any) error }, t *Template) error {
	return row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.Name, &t.Description, &t.FormType,
		&t.IsDefault, &t.IsActive, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) FindTemplates(ctx context.Context, orgID string, formType *FormType) ([]*Template, error) {
	q := `SELECT ` + templateCols + ` FROM platform_form_templates WHERE org_id = $1`
	args := []any{orgID}
	if formType != nil {
		args = append(args, string(*formType))
		q += fmt.Sprintf(" AND form_type = $%d", len(args))
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("forms: FindTemplates: %w", err)
	}
	defer rows.Close()
	list := make([]*Template, 0)
	for rows.Next() {
		t := &Template{}
		if err := scanTemplate(rows, t); err != nil {
			return nil, fmt.Errorf("forms: FindTemplates: scan: %w", err)
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindTemplateByRef(ctx context.Context, orgID, ref string) (*Template, error) {
	q := `SELECT ` + templateCols + ` FROM platform_form_templates
	      WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	t := &Template{}
	err := scanTemplate(r.db.QueryRow(ctx, q, orgID, ref), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindTemplateByRef: %w", err)
	}
	return t, nil
}

func (r *repoImpl) FindDefaultTemplate(ctx context.Context, orgID string, formType FormType) (*Template, error) {
	q := `SELECT ` + templateCols + ` FROM platform_form_templates
	      WHERE org_id = $1 AND form_type = $2 AND is_default AND is_active`
	t := &Template{}
	err := scanTemplate(r.db.QueryRow(ctx, q, orgID, string(formType)), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindDefaultTemplate: %w", err)
	}
	return t, nil
}

func (r *repoImpl) CreateTemplate(ctx context.Context, t *Template) error {
	// A template created as the default must clear its sibling atomically,
	// or the partial unique index raises a bare 23505.
	if t.IsDefault {
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("forms: CreateTemplate: begin: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx,
			`UPDATE platform_form_templates SET is_default = FALSE, updated_at = NOW()
			 WHERE org_id = $1 AND form_type = $2 AND is_default`,
			t.OrgID, string(t.FormType),
		); err != nil {
			return fmt.Errorf("forms: CreateTemplate: clear default: %w", err)
		}
		if err := tx.QueryRow(ctx, insertTemplateSQL,
			t.OrgID, t.Name, t.Description, string(t.FormType), t.IsDefault, t.CreatedBy,
		).Scan(&t.ID, &t.PublicID, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return fmt.Errorf("forms: CreateTemplate: insert: %w", err)
		}
		return tx.Commit(ctx)
	}

	return r.db.QueryRow(ctx, insertTemplateSQL,
		t.OrgID, t.Name, t.Description, string(t.FormType), t.IsDefault, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
}

const insertTemplateSQL = `
	INSERT INTO platform_form_templates (org_id, name, description, form_type, is_default, created_by)
	VALUES ($1,$2,$3,$4,$5,$6)
	RETURNING id, public_id, is_active, created_at, updated_at`

// UpdateTemplate never writes is_default = TRUE — promotion goes through
// SetTemplateDefault, which clears the sibling atomically. Writing FALSE here
// is safe because it can never conflict.
func (r *repoImpl) UpdateTemplate(ctx context.Context, t *Template) error {
	err := r.db.QueryRow(ctx,
		`UPDATE platform_form_templates SET name=$1, description=$2, is_default=$3, is_active=$4, updated_at=NOW()
		 WHERE id=$5 AND org_id=$6 RETURNING updated_at`,
		t.Name, t.Description, t.IsDefault, t.IsActive, t.ID, t.OrgID,
	).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTemplateNotFound
	}
	return err
}

func (r *repoImpl) SetTemplateDefault(ctx context.Context, orgID, templateID string, formType FormType) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("forms: SetTemplateDefault: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE platform_form_templates SET is_default = FALSE, updated_at = NOW()
		 WHERE org_id = $1 AND form_type = $2 AND is_default`,
		orgID, string(formType),
	); err != nil {
		return fmt.Errorf("forms: SetTemplateDefault: clear: %w", err)
	}
	cmd, err := tx.Exec(ctx,
		`UPDATE platform_form_templates SET is_default = TRUE, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, templateID,
	)
	if err != nil {
		return fmt.Errorf("forms: SetTemplateDefault: set: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM platform_form_templates WHERE org_id=$1 AND id=$2`, orgID, templateID)
	if err != nil {
		return fmt.Errorf("forms: DeleteTemplate: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *repoImpl) CountInstancesForTemplate(ctx context.Context, templateID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_form_instances WHERE template_id = $1`, templateID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("forms: CountInstancesForTemplate: %w", err)
	}
	return count, nil
}

// ── Sections ─────────────────────────────────────────────────────────────────

// Aliased because sections and templates share id/public_id/created_at.
const sectionCols = `s.id, s.public_id, s.template_id, s.title, s.description, s.display_order,
	s.created_at, s.updated_at`

func scanSection(row interface{ Scan(...any) error }, s *Section) error {
	return row.Scan(&s.ID, &s.PublicID, &s.TemplateID, &s.Title, &s.Description,
		&s.DisplayOrder, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) FindSections(ctx context.Context, orgID, templateID string) ([]*Section, error) {
	const q = `SELECT ` + sectionCols + `
		FROM platform_form_sections s
		JOIN platform_form_templates t ON t.id = s.template_id
		WHERE t.org_id = $1 AND s.template_id = $2
		ORDER BY s.display_order ASC, s.created_at ASC`
	rows, err := r.db.Query(ctx, q, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("forms: FindSections: %w", err)
	}
	defer rows.Close()
	list := make([]*Section, 0)
	for rows.Next() {
		s := &Section{}
		if err := scanSection(rows, s); err != nil {
			return nil, fmt.Errorf("forms: FindSections: scan: %w", err)
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindSectionByRef(ctx context.Context, orgID, ref string) (*Section, error) {
	const q = `SELECT ` + sectionCols + `
		FROM platform_form_sections s
		JOIN platform_form_templates t ON t.id = s.template_id
		WHERE t.org_id = $1 AND (s.id::text = $2 OR s.public_id = $2)`
	s := &Section{}
	err := scanSection(r.db.QueryRow(ctx, q, orgID, ref), s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindSectionByRef: %w", err)
	}
	return s, nil
}

func (r *repoImpl) CreateSection(ctx context.Context, s *Section) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO platform_form_sections (template_id, title, description, display_order)
		 VALUES ($1,$2,$3,$4) RETURNING id, public_id, created_at, updated_at`,
		s.TemplateID, s.Title, s.Description, s.DisplayOrder,
	).Scan(&s.ID, &s.PublicID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) UpdateSection(ctx context.Context, orgID string, s *Section) error {
	err := r.db.QueryRow(ctx,
		`UPDATE platform_form_sections SET title=$1, description=$2, display_order=$3, updated_at=NOW()
		 WHERE id=$4 AND template_id IN (SELECT id FROM platform_form_templates WHERE org_id=$5)
		 RETURNING updated_at`,
		s.Title, s.Description, s.DisplayOrder, s.ID, orgID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSectionNotFound
	}
	return err
}

func (r *repoImpl) DeleteSection(ctx context.Context, orgID, sectionID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM platform_form_sections
		 WHERE id=$1 AND template_id IN (SELECT id FROM platform_form_templates WHERE org_id=$2)`,
		sectionID, orgID,
	)
	if err != nil {
		return fmt.Errorf("forms: DeleteSection: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrSectionNotFound
	}
	return nil
}

// ── Questions ────────────────────────────────────────────────────────────────

const questionCols = `q.id, q.public_id, q.section_id, q.question_text, q.help_text, q.question_type,
	q.is_required, q.display_order, q.scale_min, q.scale_max, q.options, q.weight,
	q.created_at, q.updated_at`

func scanQuestion(row interface{ Scan(...any) error }, q *Question) error {
	if err := row.Scan(&q.ID, &q.PublicID, &q.SectionID, &q.QuestionText, &q.HelpText, &q.QuestionType,
		&q.IsRequired, &q.DisplayOrder, &q.ScaleMin, &q.ScaleMax, &q.OptionsRaw, &q.Weight,
		&q.CreatedAt, &q.UpdatedAt); err != nil {
		return err
	}
	return q.ParseOptions()
}

// FindQuestions returns every question in a template, ordered by section then
// question — the order a form renders in.
func (r *repoImpl) FindQuestions(ctx context.Context, orgID, templateID string) ([]*Question, error) {
	const q = `SELECT ` + questionCols + `
		FROM platform_form_questions q
		JOIN platform_form_sections s ON s.id = q.section_id
		JOIN platform_form_templates t ON t.id = s.template_id
		WHERE t.org_id = $1 AND s.template_id = $2
		ORDER BY s.display_order ASC, q.display_order ASC, q.created_at ASC`
	rows, err := r.db.Query(ctx, q, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("forms: FindQuestions: %w", err)
	}
	defer rows.Close()
	list := make([]*Question, 0)
	for rows.Next() {
		qq := &Question{}
		if err := scanQuestion(rows, qq); err != nil {
			return nil, fmt.Errorf("forms: FindQuestions: scan: %w", err)
		}
		list = append(list, qq)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindQuestionByRef(ctx context.Context, orgID, ref string) (*Question, error) {
	const query = `SELECT ` + questionCols + `
		FROM platform_form_questions q
		JOIN platform_form_sections s ON s.id = q.section_id
		JOIN platform_form_templates t ON t.id = s.template_id
		WHERE t.org_id = $1 AND (q.id::text = $2 OR q.public_id = $2)`
	qq := &Question{}
	err := scanQuestion(r.db.QueryRow(ctx, query, orgID, ref), qq)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindQuestionByRef: %w", err)
	}
	return qq, nil
}

func (r *repoImpl) CreateQuestion(ctx context.Context, q *Question) error {
	raw, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("forms: CreateQuestion: marshal options: %w", err)
	}
	q.OptionsRaw = raw
	return r.db.QueryRow(ctx,
		`INSERT INTO platform_form_questions
		    (section_id, question_text, help_text, question_type, is_required, display_order,
		     scale_min, scale_max, options, weight)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, created_at, updated_at`,
		q.SectionID, q.QuestionText, q.HelpText, string(q.QuestionType), q.IsRequired, q.DisplayOrder,
		q.ScaleMin, q.ScaleMax, raw, q.Weight,
	).Scan(&q.ID, &q.PublicID, &q.CreatedAt, &q.UpdatedAt)
}

func (r *repoImpl) UpdateQuestion(ctx context.Context, orgID string, q *Question) error {
	raw, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("forms: UpdateQuestion: marshal options: %w", err)
	}
	q.OptionsRaw = raw
	err = r.db.QueryRow(ctx,
		`UPDATE platform_form_questions SET
		    question_text=$1, help_text=$2, question_type=$3, is_required=$4, display_order=$5,
		    scale_min=$6, scale_max=$7, options=$8, weight=$9, updated_at=NOW()
		 WHERE id=$10 AND section_id IN (
		     SELECT s.id FROM platform_form_sections s
		     JOIN platform_form_templates t ON t.id = s.template_id
		     WHERE t.org_id = $11)
		 RETURNING updated_at`,
		q.QuestionText, q.HelpText, string(q.QuestionType), q.IsRequired, q.DisplayOrder,
		q.ScaleMin, q.ScaleMax, raw, q.Weight, q.ID, orgID,
	).Scan(&q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrQuestionNotFound
	}
	return err
}

func (r *repoImpl) DeleteQuestion(ctx context.Context, orgID, questionID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM platform_form_questions
		 WHERE id=$1 AND section_id IN (
		     SELECT s.id FROM platform_form_sections s
		     JOIN platform_form_templates t ON t.id = s.template_id
		     WHERE t.org_id = $2)`,
		questionID, orgID,
	)
	if err != nil {
		return fmt.Errorf("forms: DeleteQuestion: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrQuestionNotFound
	}
	return nil
}

// ── Instances ────────────────────────────────────────────────────────────────

const instanceCols = `id, public_id, org_id, template_id, template_name, form_type,
	subject_type, subject_id, subject_label, respondent_user_id, respondent_role,
	status, submitted_at, created_by, created_at, updated_at`

func scanInstance(row interface{ Scan(...any) error }, i *Instance) error {
	return row.Scan(&i.ID, &i.PublicID, &i.OrgID, &i.TemplateID, &i.TemplateName, &i.FormType,
		&i.SubjectType, &i.SubjectID, &i.SubjectLabel, &i.RespondentUserID, &i.RespondentRole,
		&i.Status, &i.SubmittedAt, &i.CreatedBy, &i.CreatedAt, &i.UpdatedAt)
}

func buildInstancesWhere(orgID string, f InstanceListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	add := func(col, val string) {
		if val == "" {
			return
		}
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	add("form_type", f.FormType)
	add("subject_type", f.SubjectType)
	add("subject_id", f.SubjectID)
	add("respondent_user_id", f.RespondentUserID)
	add("status", f.Status)
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindInstances(ctx context.Context, orgID string, filter InstanceListFilter) ([]*Instance, error) {
	where, args := buildInstancesWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM platform_form_instances WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		instanceCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("forms: FindInstances: %w", err)
	}
	defer rows.Close()
	list := make([]*Instance, 0)
	for rows.Next() {
		i := &Instance{}
		if err := scanInstance(rows, i); err != nil {
			return nil, fmt.Errorf("forms: FindInstances: scan: %w", err)
		}
		list = append(list, i)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountInstances(ctx context.Context, orgID string, filter InstanceListFilter) (int, error) {
	where, args := buildInstancesWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM platform_form_instances WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("forms: CountInstances: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindInstanceByRef(ctx context.Context, orgID, ref string) (*Instance, error) {
	q := `SELECT ` + instanceCols + ` FROM platform_form_instances
	      WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	i := &Instance{}
	err := scanInstance(r.db.QueryRow(ctx, q, orgID, ref), i)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindInstanceByRef: %w", err)
	}
	return i, nil
}

func (r *repoImpl) InstantiateWithResponses(ctx context.Context, inst *Instance, responses []*Response) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("forms: InstantiateWithResponses: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx,
		`INSERT INTO platform_form_instances
		    (org_id, template_id, template_name, form_type, subject_type, subject_id, subject_label,
		     respondent_user_id, respondent_role, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, status, created_at, updated_at`,
		inst.OrgID, inst.TemplateID, inst.TemplateName, string(inst.FormType),
		string(inst.SubjectType), inst.SubjectID, inst.SubjectLabel,
		inst.RespondentUserID, inst.RespondentRole, inst.CreatedBy,
	).Scan(&inst.ID, &inst.PublicID, &inst.Status, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return fmt.Errorf("forms: InstantiateWithResponses: insert instance: %w", err)
	}

	for _, resp := range responses {
		resp.InstanceID = inst.ID
		raw, err := json.Marshal(resp.Options)
		if err != nil {
			return fmt.Errorf("forms: InstantiateWithResponses: marshal options: %w", err)
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO platform_form_responses
			    (instance_id, question_id, section_title, question_text, question_type, is_required,
			     display_order, scale_min, scale_max, options, weight)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			 RETURNING id, public_id, created_at, updated_at`,
			resp.InstanceID, resp.QuestionID, resp.SectionTitle, resp.QuestionText,
			string(resp.QuestionType), resp.IsRequired, resp.DisplayOrder,
			resp.ScaleMin, resp.ScaleMax, raw, resp.Weight,
		).Scan(&resp.ID, &resp.PublicID, &resp.CreatedAt, &resp.UpdatedAt); err != nil {
			return fmt.Errorf("forms: InstantiateWithResponses: insert response: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *repoImpl) SetInstanceStatus(ctx context.Context, orgID, instanceID string, status InstanceStatus, submitted bool) (*Instance, error) {
	i := &Instance{}
	err := scanInstance(r.db.QueryRow(ctx,
		`UPDATE platform_form_instances SET
		    status = $1,
		    submitted_at = CASE WHEN $2 THEN COALESCE(submitted_at, NOW()) ELSE submitted_at END,
		    updated_at = NOW()
		 WHERE id = $3 AND org_id = $4
		 RETURNING `+instanceCols,
		string(status), submitted, instanceID, orgID,
	), i)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInstanceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("forms: SetInstanceStatus: %w", err)
	}
	return i, nil
}

// ── Responses ────────────────────────────────────────────────────────────────

const responseCols = `id, public_id, instance_id, question_id, section_title, question_text,
	question_type, is_required, display_order, scale_min, scale_max, options, weight,
	answer_text, answer_number, answer_boolean, answer_date, answer_options,
	answered_at, created_at, updated_at`

func scanResponse(row interface{ Scan(...any) error }, r *Response) error {
	if err := row.Scan(&r.ID, &r.PublicID, &r.InstanceID, &r.QuestionID, &r.SectionTitle, &r.QuestionText,
		&r.QuestionType, &r.IsRequired, &r.DisplayOrder, &r.ScaleMin, &r.ScaleMax, &r.OptionsRaw, &r.Weight,
		&r.AnswerText, &r.AnswerNumber, &r.AnswerBoolean, &r.AnswerDate, &r.AnswerOptions,
		&r.AnsweredAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return err
	}
	return r.ParseOptions()
}

func (r *repoImpl) FindResponses(ctx context.Context, instanceID string) ([]*Response, error) {
	q := `SELECT ` + responseCols + ` FROM platform_form_responses
	      WHERE instance_id = $1 ORDER BY display_order ASC, created_at ASC`
	rows, err := r.db.Query(ctx, q, instanceID)
	if err != nil {
		return nil, fmt.Errorf("forms: FindResponses: %w", err)
	}
	defer rows.Close()
	list := make([]*Response, 0)
	for rows.Next() {
		resp := &Response{}
		if err := scanResponse(rows, resp); err != nil {
			return nil, fmt.Errorf("forms: FindResponses: scan: %w", err)
		}
		list = append(list, resp)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindResponseByRef(ctx context.Context, instanceID, ref string) (*Response, error) {
	q := `SELECT ` + responseCols + ` FROM platform_form_responses
	      WHERE instance_id = $1 AND (id::text = $2 OR public_id = $2)`
	resp := &Response{}
	err := scanResponse(r.db.QueryRow(ctx, q, instanceID, ref), resp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forms: FindResponseByRef: %w", err)
	}
	return resp, nil
}

func (r *repoImpl) SaveAnswers(ctx context.Context, instanceID string, answers []*Response) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("forms: SaveAnswers: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, a := range answers {
		cmd, err := tx.Exec(ctx,
			`UPDATE platform_form_responses SET
			    answer_text = $1, answer_number = $2, answer_boolean = $3,
			    answer_date = $4, answer_options = $5,
			    answered_at = NOW(), updated_at = NOW()
			 WHERE id = $6 AND instance_id = $7`,
			a.AnswerText, a.AnswerNumber, a.AnswerBoolean, a.AnswerDate, a.AnswerOptions,
			a.ID, instanceID,
		)
		if err != nil {
			return fmt.Errorf("forms: SaveAnswers: %w", err)
		}
		if cmd.RowsAffected() == 0 {
			return ErrResponseNotFound
		}
	}
	return tx.Commit(ctx)
}
