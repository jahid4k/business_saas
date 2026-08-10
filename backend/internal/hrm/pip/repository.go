// backend/internal/hrm/pip/repository.go
package pip

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

type Repository interface {
	Find(ctx context.Context, orgID string, f ListFilter) ([]*PIP, error)
	Count(ctx context.Context, orgID string, f ListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*PIP, error)
	HasOpenPlan(ctx context.Context, orgID, employeeID string) (bool, error)
	Create(ctx context.Context, p *PIP) error
	Update(ctx context.Context, p *PIP) error
	SetStatus(ctx context.Context, orgID, id string, status Status) (*PIP, error)

	FindCheckins(ctx context.Context, pipID string) ([]*Checkin, error)
	// CreateCheckin appends one entry. Plain insert: nothing else changes.
	CreateCheckin(ctx context.Context, ch *Checkin) error

	// ExtendWithCheckin moves the end date AND appends the extension entry in
	// one transaction, so an extension can never exist without its written
	// reason, nor a reason without the extension.
	ExtendWithCheckin(ctx context.Context, orgID string, p *PIP, ch *Checkin) error
	// CloseWithCheckin records the outcome AND appends the closure entry
	// transactionally, for the same reason.
	CloseWithCheckin(ctx context.Context, orgID string, p *PIP, ch *Checkin) error
	// LinkTermination attaches the draft termination created by the failed-PIP
	// handoff. Separate from CloseWithCheckin deliberately — see the service.
	LinkTermination(ctx context.Context, orgID, id, terminationID string) error

	FindEmployeeRef(ctx context.Context, orgID, employeeRef string) (*EmployeeRef, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const cols = `id, public_id, org_id, employee_id, manager_employee_id, title, concerns,
	success_criteria, support_provided, start_date, end_date, original_end_date,
	status, outcome, termination_id, closed_at, closed_by, created_by, created_at, updated_at`

func scanPIP(row pgx.Row) (*PIP, error) {
	p := &PIP{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.EmployeeID, &p.ManagerEmployeeID,
		&p.Title, &p.Concerns, &p.SuccessCriteria, &p.SupportProvided,
		&p.StartDate, &p.EndDate, &p.OriginalEndDate, &p.Status, &p.Outcome,
		&p.TerminationID, &p.ClosedAt, &p.ClosedBy, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func buildWhere(orgID string, f ListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.EmployeeID != "" {
		args = append(args, f.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Outcome != "" {
		args = append(args, f.Outcome)
		clauses = append(clauses, fmt.Sprintf("outcome = $%d", len(args)))
	}
	// Scope predicate last so its placeholder offset accounts for every
	// filter above it. This is the control that stops a peer reading a
	// colleague's improvement plan.
	if f.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(f.Scope, "employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) Find(ctx context.Context, orgID string, f ListFilter) ([]*PIP, error) {
	where, args := buildWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_pips WHERE %s
		ORDER BY start_date DESC, created_at DESC LIMIT $%d OFFSET $%d`,
		cols, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("pip: Find: %w", err)
	}
	defer rows.Close()

	out := make([]*PIP, 0)
	for rows.Next() {
		p, err := scanPIP(rows)
		if err != nil {
			return nil, fmt.Errorf("pip: Find: scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID string, f ListFilter) (int, error) {
	where, args := buildWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_pips WHERE %s`, where), args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("pip: Count: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*PIP, error) {
	p, err := scanPIP(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_pips
			WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`, cols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("pip: FindByRef: %w", err)
	}
	return p, nil
}

// HasOpenPlan mirrors the partial unique index uq_hrm_pip_employee_open. The
// index is the guarantee; this is the friendly message, and the two must list
// the same statuses — Status.IsOpen is the shared definition.
func (r *repoImpl) HasOpenPlan(ctx context.Context, orgID, employeeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_pips
		  WHERE org_id = $1 AND employee_id = $2
		    AND status IN ('draft', 'active', 'extended'))`,
		orgID, employeeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("pip: HasOpenPlan: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) Create(ctx context.Context, p *PIP) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_pips
		    (org_id, employee_id, manager_employee_id, title, concerns, success_criteria,
		     support_provided, start_date, end_date, original_end_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10)
		 RETURNING id, public_id, status, original_end_date, created_at, updated_at`,
		p.OrgID, p.EmployeeID, p.ManagerEmployeeID, p.Title, p.Concerns, p.SuccessCriteria,
		p.SupportProvided, p.StartDate, p.EndDate, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.Status, &p.OriginalEndDate, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("pip: Create: %w", err)
	}
	return nil
}

// Update writes the editable definition fields only. end_date is absent
// deliberately: it moves through ExtendWithCheckin, which forces a written
// reason. A silent end-date edit is the failure mode this instrument has.
func (r *repoImpl) Update(ctx context.Context, p *PIP) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_pips SET
		    title = $1, concerns = $2, success_criteria = $3, support_provided = $4,
		    start_date = $5, updated_at = NOW()
		 WHERE org_id = $6 AND id = $7
		 RETURNING updated_at`,
		p.Title, p.Concerns, p.SuccessCriteria, p.SupportProvided, p.StartDate, p.OrgID, p.ID,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("pip: Update: %w", err)
	}
	return nil
}

func (r *repoImpl) SetStatus(ctx context.Context, orgID, id string, status Status) (*PIP, error) {
	p, err := scanPIP(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_pips SET status = $1, updated_at = NOW()
		 WHERE org_id = $2 AND id = $3 RETURNING %s`, cols), status, orgID, id))
	if err != nil {
		return nil, fmt.Errorf("pip: SetStatus: %w", err)
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// ── Check-ins ────────────────────────────────────────────────────────────────

const checkinCols = `id, public_id, pip_id, entry_type, progress, note,
	previous_end_date, new_end_date, checked_in_by, checked_in_at, created_at`

func scanCheckin(row pgx.Row) (*Checkin, error) {
	ch := &Checkin{}
	err := row.Scan(&ch.ID, &ch.PublicID, &ch.PIPID, &ch.EntryType, &ch.Progress, &ch.Note,
		&ch.PreviousEndDate, &ch.NewEndDate, &ch.CheckedInBy, &ch.CheckedInAt, &ch.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (r *repoImpl) FindCheckins(ctx context.Context, pipID string) ([]*Checkin, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_pip_checkins WHERE pip_id = $1
			ORDER BY checked_in_at ASC`, checkinCols), pipID)
	if err != nil {
		return nil, fmt.Errorf("pip: FindCheckins: %w", err)
	}
	defer rows.Close()

	out := make([]*Checkin, 0)
	for rows.Next() {
		ch, err := scanCheckin(rows)
		if err != nil {
			return nil, fmt.Errorf("pip: FindCheckins: scan: %w", err)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// insertCheckin appends one entry inside an existing transaction. Every
// extension and closure goes through it, which is what guarantees the history
// has no holes.
func insertCheckin(ctx context.Context, tx pgx.Tx, ch *Checkin) error {
	return tx.QueryRow(ctx,
		`INSERT INTO hrm_pip_checkins
		    (pip_id, entry_type, progress, note, previous_end_date, new_end_date, checked_in_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, public_id, checked_in_at, created_at`,
		ch.PIPID, ch.EntryType, ch.Progress, ch.Note, ch.PreviousEndDate, ch.NewEndDate, ch.CheckedInBy,
	).Scan(&ch.ID, &ch.PublicID, &ch.CheckedInAt, &ch.CreatedAt)
}

func (r *repoImpl) CreateCheckin(ctx context.Context, ch *Checkin) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_pip_checkins
		    (pip_id, entry_type, progress, note, checked_in_by)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, public_id, checked_in_at, created_at`,
		ch.PIPID, ch.EntryType, ch.Progress, ch.Note, ch.CheckedInBy,
	).Scan(&ch.ID, &ch.PublicID, &ch.CheckedInAt, &ch.CreatedAt)
	if err != nil {
		return fmt.Errorf("pip: CreateCheckin: %w", err)
	}
	return nil
}

func (r *repoImpl) ExtendWithCheckin(ctx context.Context, orgID string, p *PIP, ch *Checkin) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pip: ExtendWithCheckin: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx,
		`UPDATE hrm_pips SET end_date = $1, status = 'extended', updated_at = NOW()
		 WHERE org_id = $2 AND id = $3
		 RETURNING end_date, status, updated_at`,
		p.EndDate, orgID, p.ID,
	).Scan(&p.EndDate, &p.Status, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("pip: ExtendWithCheckin: update: %w", err)
	}

	if err := insertCheckin(ctx, tx, ch); err != nil {
		return fmt.Errorf("pip: ExtendWithCheckin: checkin: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pip: ExtendWithCheckin: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) CloseWithCheckin(ctx context.Context, orgID string, p *PIP, ch *Checkin) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pip: CloseWithCheckin: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx,
		`UPDATE hrm_pips SET
		    status = 'closed', outcome = $1, closed_at = NOW(), closed_by = $2, updated_at = NOW()
		 WHERE org_id = $3 AND id = $4
		 RETURNING status, closed_at, updated_at`,
		p.Outcome, p.ClosedBy, orgID, p.ID,
	).Scan(&p.Status, &p.ClosedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("pip: CloseWithCheckin: update: %w", err)
	}

	if err := insertCheckin(ctx, tx, ch); err != nil {
		return fmt.Errorf("pip: CloseWithCheckin: checkin: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pip: CloseWithCheckin: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) LinkTermination(ctx context.Context, orgID, id, terminationID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_pips SET termination_id = $1, updated_at = NOW()
		 WHERE org_id = $2 AND id = $3`, terminationID, orgID, id)
	if err != nil {
		return fmt.Errorf("pip: LinkTermination: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Employee resolution ──────────────────────────────────────────────────────

// FindEmployeeRef owns its query rather than importing internal/hrm/employees
// — the onboarding precedent, which keeps the dependency graph free of an
// employees ↔ pip edge.
func (r *repoImpl) FindEmployeeRef(ctx context.Context, orgID, employeeRef string) (*EmployeeRef, error) {
	e := &EmployeeRef{}
	err := r.db.QueryRow(ctx,
		`SELECT id,
		        TRIM(COALESCE(first_name,'') || ' ' || COALESCE(last_name,'')) AS display_name,
		        manager_id
		   FROM hrm_employees
		  WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`,
		orgID, employeeRef).Scan(&e.EmployeeID, &e.DisplayName, &e.ManagerEmployeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pip: FindEmployeeRef: %w", err)
	}
	return e, nil
}

// dateOnly strips the time component so day arithmetic is not thrown off by
// the timestamps Postgres returns for DATE columns in some drivers.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
