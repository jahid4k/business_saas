// backend/internal/hrm/feedback/repository.go
package feedback

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// Repository is where the anonymity contract is MECHANICALLY enforced rather
// than merely intended.
//
// The identity-bearing and content-bearing reads are separate methods
// returning separate types, and neither query selects the other's columns:
//
//	FindRequestSummaries  → SELECT respondent_name, status, ...   (no answers)
//	FindSubmittedForSubject → SELECT form_instance_id, relationship (no identity)
//
// The second returns form instance ids because the SERVICE must fetch answers
// with them; those ids never reach a handler response. Keeping the identity
// columns out of that query entirely means a future caller cannot accidentally
// serialise a respondent by widening a struct — there is nothing to widen it
// with.
//
// This is the direct answer to hrm_complaints.is_anonymous, which stores a
// promise no query honours.
type Repository interface {
	// ── Cycles ──────────────────────────────────────────────────────────
	FindCycles(ctx context.Context, orgID string, f CycleListFilter) ([]*Cycle, error)
	CountCycles(ctx context.Context, orgID string, f CycleListFilter) (int, error)
	FindCycleByRef(ctx context.Context, orgID, ref string) (*Cycle, error)
	CycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
	CreateCycle(ctx context.Context, c *Cycle) error
	UpdateCycle(ctx context.Context, c *Cycle) error
	SetCycleStatus(ctx context.Context, orgID, id string, status CycleStatus) (*Cycle, error)

	// ── Requests: coordination path (identity, no content) ──────────────
	FindRequestSummaries(ctx context.Context, orgID string, f RequestListFilter) ([]*RequestSummary, error)
	CountRequests(ctx context.Context, orgID string, f RequestListFilter) (int, error)
	FindRequestByRef(ctx context.Context, orgID, ref string) (*Request, error)
	CreateRequests(ctx context.Context, reqs []*Request) error
	SetRequestSubmitted(ctx context.Context, orgID, id string) (*Request, error)
	SetRequestDeclined(ctx context.Context, orgID, id string, reason *string) (*Request, error)
	RequestExists(ctx context.Context, cycleID, subjectEmployeeID string, respondentEmployeeID, email *string) (bool, error)

	// ── Requests: content path (no identity) ────────────────────────────
	// FindSubmittedForSubject returns ONLY (relationship, form_instance_id)
	// for submitted responses. It deliberately selects no column that names
	// a respondent, so the aggregate cannot leak identity by omission.
	FindSubmittedForSubject(ctx context.Context, orgID, cycleID, subjectEmployeeID string) ([]*SubmittedRef, error)

	// ── Respondent's own inbox ──────────────────────────────────────────
	FindRequestsForRespondent(ctx context.Context, orgID, userID string) ([]*MyRequest, error)

	// ── Employee resolution ─────────────────────────────────────────────
	FindEmployeeSubject(ctx context.Context, orgID, employeeRef string) (*EmployeeSubject, error)
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

// SubmittedRef is the content path's carrier: which relationship group a
// response belongs to, and where its answers live. The service converts it to
// AnonymousResponse after fetching and stripping; it is never returned from a
// handler.
//
// FormInstanceID carries `json:"-"` deliberately. It is the one field in this
// package capable of defeating the anonymity contract — platform_form_
// instances stores respondent_user_id, so an instance id plus
// GET /forms/instances/:id names the respondent. Tagging it here means that
// even if a future handler returns a SubmittedRef by mistake, the id does not
// cross the wire. The type is exported only so tests can implement Repository.
type SubmittedRef struct {
	Relationship   Relationship `json:"relationship"`
	FormInstanceID *string      `json:"-"`
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Cycles ───────────────────────────────────────────────────────────────────

const cycleCols = `id, public_id, org_id, name, description, period_start, period_end,
	form_template_id, min_responses, status, closed_at, created_by, created_at, updated_at`

func scanCycle(row pgx.Row) (*Cycle, error) {
	c := &Cycle{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description,
		&c.PeriodStart, &c.PeriodEnd, &c.FormTemplateID, &c.MinResponses,
		&c.Status, &c.ClosedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func buildCycleWhere(orgID string, f CycleListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindCycles(ctx context.Context, orgID string, f CycleListFilter) ([]*Cycle, error) {
	where, args := buildCycleWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_feedback_cycles WHERE %s
		ORDER BY period_start DESC, created_at DESC LIMIT $%d OFFSET $%d`,
		cycleCols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("feedback: FindCycles: %w", err)
	}
	defer rows.Close()

	out := make([]*Cycle, 0)
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, fmt.Errorf("feedback: FindCycles: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountCycles(ctx context.Context, orgID string, f CycleListFilter) (int, error) {
	where, args := buildCycleWhere(orgID, f)
	var n int
	err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_feedback_cycles WHERE %s`, where), args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("feedback: CountCycles: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindCycleByRef(ctx context.Context, orgID, ref string) (*Cycle, error) {
	c, err := scanCycle(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_feedback_cycles
			WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`, cycleCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("feedback: FindCycleByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CycleNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_feedback_cycles
		  WHERE org_id = $1 AND LOWER(name) = LOWER($2) AND ($3 = '' OR id::text <> $3))`,
		orgID, name, excludeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("feedback: CycleNameExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CreateCycle(ctx context.Context, c *Cycle) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_feedback_cycles
		    (org_id, name, description, period_start, period_end,
		     form_template_id, min_responses, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, public_id, status, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.PeriodStart, c.PeriodEnd,
		c.FormTemplateID, c.MinResponses, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("feedback: CreateCycle: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateCycle(ctx context.Context, c *Cycle) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_feedback_cycles SET
		    name = $1, description = $2, period_start = $3, period_end = $4,
		    min_responses = $5, updated_at = NOW()
		 WHERE org_id = $6 AND id = $7
		 RETURNING updated_at`,
		c.Name, c.Description, c.PeriodStart, c.PeriodEnd, c.MinResponses, c.OrgID, c.ID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCycleNotFound
	}
	if err != nil {
		return fmt.Errorf("feedback: UpdateCycle: %w", err)
	}
	return nil
}

func (r *repoImpl) SetCycleStatus(ctx context.Context, orgID, id string, status CycleStatus) (*Cycle, error) {
	c, err := scanCycle(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_feedback_cycles SET
		    status = $1,
		    closed_at = CASE WHEN $1 = 'closed' THEN NOW() ELSE closed_at END,
		    updated_at = NOW()
		 WHERE org_id = $2 AND id = $3
		 RETURNING %s`, cycleCols), status, orgID, id))
	if err != nil {
		return nil, fmt.Errorf("feedback: SetCycleStatus: %w", err)
	}
	if c == nil {
		return nil, ErrCycleNotFound
	}
	return c, nil
}

// ── Requests: the COORDINATION path ──────────────────────────────────────────
//
// Carries identity. Selects no answer column, and has no parameter that could
// ask for one.

func buildRequestWhere(orgID string, f RequestListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.CycleID != "" {
		args = append(args, f.CycleID)
		clauses = append(clauses, fmt.Sprintf("cycle_id = $%d", len(args)))
	}
	if f.SubjectEmployeeID != "" {
		args = append(args, f.SubjectEmployeeID)
		clauses = append(clauses, fmt.Sprintf("subject_employee_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	// Scope predicate last so its placeholder offset accounts for every
	// filter above it. Note the column is subject_employee_id: the tier
	// governs WHOSE FEEDBACK you may coordinate, never whose responses you
	// may read.
	if f.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(f.Scope, "subject_employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindRequestSummaries(ctx context.Context, orgID string, f RequestListFilter) ([]*RequestSummary, error) {
	where, args := buildRequestWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT id, public_id, respondent_name, relationship, status, submitted_at
		FROM hrm_feedback_requests WHERE %s
		ORDER BY relationship, respondent_name LIMIT $%d OFFSET $%d`,
		where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("feedback: FindRequestSummaries: %w", err)
	}
	defer rows.Close()

	out := make([]*RequestSummary, 0)
	for rows.Next() {
		s := &RequestSummary{}
		if err := rows.Scan(&s.ID, &s.PublicID, &s.RespondentName, &s.Relationship,
			&s.Status, &s.SubmittedAt); err != nil {
			return nil, fmt.Errorf("feedback: FindRequestSummaries: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountRequests(ctx context.Context, orgID string, f RequestListFilter) (int, error) {
	where, args := buildRequestWhere(orgID, f)
	var n int
	err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_feedback_requests WHERE %s`, where), args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("feedback: CountRequests: %w", err)
	}
	return n, nil
}

const requestCols = `id, public_id, org_id, cycle_id, subject_employee_id,
	respondent_employee_id, respondent_user_id, respondent_name, respondent_email,
	relationship, form_instance_id, status, submitted_at, declined_at, decline_reason,
	requested_by, created_at, updated_at`

func scanRequest(row pgx.Row) (*Request, error) {
	q := &Request{}
	err := row.Scan(&q.ID, &q.PublicID, &q.OrgID, &q.CycleID, &q.SubjectEmployeeID,
		&q.RespondentEmployeeID, &q.RespondentUserID, &q.RespondentName, &q.RespondentEmail,
		&q.Relationship, &q.FormInstanceID, &q.Status, &q.SubmittedAt, &q.DeclinedAt,
		&q.DeclineReason, &q.RequestedBy, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (r *repoImpl) FindRequestByRef(ctx context.Context, orgID, ref string) (*Request, error) {
	q, err := scanRequest(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_feedback_requests
			WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`, requestCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("feedback: FindRequestByRef: %w", err)
	}
	return q, nil
}

// CreateRequests inserts a batch in one transaction. All-or-nothing because a
// half-created 360 is a coordination problem the caller cannot see: some
// respondents notified, others not, with no record of which.
func (r *repoImpl) CreateRequests(ctx context.Context, reqs []*Request) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("feedback: CreateRequests: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, q := range reqs {
		err := tx.QueryRow(ctx,
			`INSERT INTO hrm_feedback_requests
			    (org_id, cycle_id, subject_employee_id, respondent_employee_id,
			     respondent_user_id, respondent_name, respondent_email, relationship,
			     form_instance_id, requested_by)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			 RETURNING id, public_id, status, created_at, updated_at`,
			q.OrgID, q.CycleID, q.SubjectEmployeeID, q.RespondentEmployeeID,
			q.RespondentUserID, q.RespondentName, q.RespondentEmail, q.Relationship,
			q.FormInstanceID, q.RequestedBy,
		).Scan(&q.ID, &q.PublicID, &q.Status, &q.CreatedAt, &q.UpdatedAt)
		if err != nil {
			return fmt.Errorf("feedback: CreateRequests: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("feedback: CreateRequests: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) SetRequestSubmitted(ctx context.Context, orgID, id string) (*Request, error) {
	q, err := scanRequest(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_feedback_requests SET
		    status = 'submitted', submitted_at = NOW(), updated_at = NOW()
		 WHERE org_id = $1 AND id = $2 RETURNING %s`, requestCols), orgID, id))
	if err != nil {
		return nil, fmt.Errorf("feedback: SetRequestSubmitted: %w", err)
	}
	if q == nil {
		return nil, ErrRequestNotFound
	}
	return q, nil
}

func (r *repoImpl) SetRequestDeclined(ctx context.Context, orgID, id string, reason *string) (*Request, error) {
	q, err := scanRequest(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_feedback_requests SET
		    status = 'declined', declined_at = NOW(), decline_reason = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2 RETURNING %s`, requestCols), orgID, id, reason))
	if err != nil {
		return nil, fmt.Errorf("feedback: SetRequestDeclined: %w", err)
	}
	if q == nil {
		return nil, ErrRequestNotFound
	}
	return q, nil
}

func (r *repoImpl) RequestExists(ctx context.Context, cycleID, subjectEmployeeID string, respondentEmployeeID, email *string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM hrm_feedback_requests
		    WHERE cycle_id = $1 AND subject_employee_id = $2
		      AND ( ($3::uuid IS NOT NULL AND respondent_employee_id = $3::uuid)
		         OR ($3::uuid IS NULL AND respondent_employee_id IS NULL
		             AND LOWER(respondent_email) = LOWER($4)) ))`,
		cycleID, subjectEmployeeID, respondentEmployeeID, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("feedback: RequestExists: %w", err)
	}
	return exists, nil
}

// ── Requests: the CONTENT path ───────────────────────────────────────────────

// FindSubmittedForSubject is the anonymity boundary.
//
// It selects exactly two columns — relationship and form_instance_id — and no
// third. There is no respondent id, no name, no email, no submitted_at
// timestamp that could be correlated against a coordination listing to
// re-identify someone. Widening this SELECT is the change that breaks the
// module's central promise, which is why the returned type is unexported and
// carries nothing else either.
//
// Ordering is by relationship then form_instance_id, deliberately NOT by
// submitted_at: response order is a correlation channel against the
// coordination view, where submission times are visible.
func (r *repoImpl) FindSubmittedForSubject(ctx context.Context, orgID, cycleID, subjectEmployeeID string) ([]*SubmittedRef, error) {
	rows, err := r.db.Query(ctx,
		`SELECT relationship, form_instance_id
		   FROM hrm_feedback_requests
		  WHERE org_id = $1 AND cycle_id = $2 AND subject_employee_id = $3
		    AND status = 'submitted'
		  ORDER BY relationship, form_instance_id`,
		orgID, cycleID, subjectEmployeeID)
	if err != nil {
		return nil, fmt.Errorf("feedback: FindSubmittedForSubject: %w", err)
	}
	defer rows.Close()

	out := make([]*SubmittedRef, 0)
	for rows.Next() {
		s := &SubmittedRef{}
		if err := rows.Scan(&s.Relationship, &s.FormInstanceID); err != nil {
			return nil, fmt.Errorf("feedback: FindSubmittedForSubject: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── Respondent's own inbox ───────────────────────────────────────────────────

// FindRequestsForRespondent returns the caller's OWN asks, including the form
// instance id they are meant to fill in. Not a leak: it is their row, and the
// WHERE clause is respondent_user_id = the caller.
func (r *repoImpl) FindRequestsForRespondent(ctx context.Context, orgID, userID string) ([]*MyRequest, error) {
	rows, err := r.db.Query(ctx,
		`SELECT fr.id, fr.public_id, fr.cycle_id, fc.name, fc.period_end,
		        TRIM(COALESCE(e.first_name,'') || ' ' || COALESCE(e.last_name,'')) AS subject_name,
		        fr.subject_employee_id, fr.relationship, fr.status,
		        fr.form_instance_id, fr.submitted_at
		   FROM hrm_feedback_requests fr
		   JOIN hrm_feedback_cycles fc ON fc.id = fr.cycle_id
		   JOIN hrm_employees e        ON e.id = fr.subject_employee_id
		  WHERE fr.org_id = $1 AND fr.respondent_user_id = $2
		    AND fr.status IN ('pending', 'submitted')
		  ORDER BY fc.period_end ASC, subject_name ASC`,
		orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("feedback: FindRequestsForRespondent: %w", err)
	}
	defer rows.Close()

	out := make([]*MyRequest, 0)
	for rows.Next() {
		m := &MyRequest{}
		if err := rows.Scan(&m.ID, &m.PublicID, &m.CycleID, &m.CycleName, &m.PeriodEnd,
			&m.SubjectName, &m.SubjectEmployeeID, &m.Relationship, &m.Status,
			&m.FormInstanceID, &m.SubmittedAt); err != nil {
			return nil, fmt.Errorf("feedback: FindRequestsForRespondent: scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── Employee resolution ──────────────────────────────────────────────────────

// FindEmployeeSubject owns its query rather than importing internal/hrm/
// employees — the onboarding precedent, which keeps the dependency graph free
// of an employees ↔ feedback edge.
func (r *repoImpl) FindEmployeeSubject(ctx context.Context, orgID, employeeRef string) (*EmployeeSubject, error) {
	s := &EmployeeSubject{}
	err := r.db.QueryRow(ctx,
		`SELECT id,
		        TRIM(COALESCE(first_name,'') || ' ' || COALESCE(last_name,'')) AS display_name,
		        email, user_id
		   FROM hrm_employees
		  WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`,
		orgID, employeeRef).Scan(&s.EmployeeID, &s.DisplayName, &s.Email, &s.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feedback: FindEmployeeSubject: %w", err)
	}
	return s, nil
}

// FindEmployeeIDByUserID resolves the caller's own hrm_employees.id. Returns
// "" and no error when the caller has no employee row — a valid state for a
// non-employee admin acting on the org, not a failure.
func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE org_id = $1 AND user_id = $2`, orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("feedback: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}
