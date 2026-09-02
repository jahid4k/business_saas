// backend/internal/hrm/warnings/repository.go
package warnings

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

// Repository defines data access for employee warning records.
type Repository interface {
	FindAll(ctx context.Context, orgID string, filter WarningListFilter) ([]*EmployeeWarning, error)
	Count(ctx context.Context, orgID string, filter WarningListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error)
	CountActiveByTypeAndEmployee(ctx context.Context, orgID, employeeID, warningTypeID string, withinDays int) (int, error)
	Create(ctx context.Context, w *EmployeeWarning) error
	Update(ctx context.Context, w *EmployeeWarning) error
	UpdateStatus(ctx context.Context, id string, status WarningStatus) error
	SetApprovalInstance(ctx context.Context, id, instanceID string, status WarningStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id,
	warning_type_id, warning_type_name, severity_level,
	title, description, to_char(incident_date,'YYYY-MM-DD'),
	issued_by, witness_ids,
	approval_instance_id, document_id,
	can_employee_respond, response_window_days, to_char(response_deadline,'YYYY-MM-DD'),
	employee_response, employee_responded_at,
	appeal_reason, appeal_submitted_at, appeal_resolution, appeal_resolved_at,
	to_char(expires_at,'YYYY-MM-DD'), is_active,
	issued_at, status, created_by, created_at, updated_at`

func scanW(row pgx.Row) (*EmployeeWarning, error) {
	w := &EmployeeWarning{}
	err := row.Scan(
		&w.ID, &w.PublicID, &w.OrgID, &w.EmployeeID,
		&w.WarningTypeID, &w.WarningTypeName, &w.SeverityLevel,
		&w.Title, &w.Description, &w.IncidentDate,
		&w.IssuedBy, &w.WitnessIDs,
		&w.ApprovalInstanceID, &w.DocumentID,
		&w.CanEmployeeRespond, &w.ResponseWindowDays, &w.ResponseDeadline,
		&w.EmployeeResponse, &w.EmployeeRespondedAt,
		&w.AppealReason, &w.AppealSubmittedAt, &w.AppealResolution, &w.AppealResolvedAt,
		&w.ExpiresAt, &w.IsActive,
		&w.IssuedAt, &w.Status, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return w, nil
}

func buildWarningsWhere(orgID string, filter WarningListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.ActiveOnly {
		clauses = append(clauses, "is_active = TRUE")
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, filter WarningListFilter) ([]*EmployeeWarning, error) {
	where, args := buildWarningsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_employee_warnings WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		sel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("warnings: FindAll: %w", err)
	}
	defer rows.Close()
	list := make([]*EmployeeWarning, 0)
	for rows.Next() {
		w, err := scanW(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID string, filter WarningListFilter) (int, error) {
	where, args := buildWarningsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_employee_warnings WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("warnings: Count: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeWarning, error) {
	q := `SELECT ` + sel + ` FROM hrm_employee_warnings WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" {
		args = append(args, employeeID)
		q += fmt.Sprintf(` AND employee_id=$%d`, len(args))
	}
	return scanW(r.db.QueryRow(ctx, q, args...))
}

// CountActiveByTypeAndEmployee returns the count of active (is_active=TRUE, status=issued/acknowledged/appealed)
// warnings of a specific type for an employee. Used for escalation rule evaluation.
// withinDays=0 counts all-time; >0 counts only within the last N days.
func (r *repoImpl) CountActiveByTypeAndEmployee(ctx context.Context, orgID, employeeID, warningTypeID string, withinDays int) (int, error) {
	q := `SELECT COUNT(*) FROM hrm_employee_warnings
		WHERE org_id=$1 AND employee_id=$2 AND warning_type_id=$3
		AND is_active=TRUE AND status IN ('issued','acknowledged','appealed')`
	args := []any{orgID, employeeID, warningTypeID}
	if withinDays > 0 {
		args = append(args, withinDays)
		q += fmt.Sprintf(` AND issued_at >= NOW() - INTERVAL '%d days'`, withinDays)
	}
	var count int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("warnings: CountActive: %w", err)
	}
	return count, nil
}

func (r *repoImpl) Create(ctx context.Context, w *EmployeeWarning) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_warnings
		(org_id, employee_id, warning_type_id, warning_type_name, severity_level,
		 title, description, incident_date, issued_by, witness_ids,
		 can_employee_respond, response_window_days, document_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, public_id, created_at, updated_at`,
		w.OrgID, w.EmployeeID, w.WarningTypeID, w.WarningTypeName, w.SeverityLevel,
		w.Title, w.Description, w.IncidentDate, w.IssuedBy, w.WitnessIDs,
		w.CanEmployeeRespond, w.ResponseWindowDays, w.DocumentID, w.Status, w.CreatedBy,
	).Scan(&w.ID, &w.PublicID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, w *EmployeeWarning) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_employee_warnings SET
		title=$1, description=$2, incident_date=$3::date, witness_ids=$4, document_id=$5,
		response_deadline=$6::date, employee_response=$7, employee_responded_at=$8,
		appeal_reason=$9, appeal_submitted_at=$10, appeal_resolution=$11, appeal_resolved_at=$12,
		issued_at=$13, expires_at=$14::date, is_active=$15, updated_at=NOW()
		WHERE id=$16 AND org_id=$17 RETURNING updated_at`,
		w.Title, w.Description, w.IncidentDate, w.WitnessIDs, w.DocumentID,
		w.ResponseDeadline, w.EmployeeResponse, w.EmployeeRespondedAt,
		w.AppealReason, w.AppealSubmittedAt, w.AppealResolution, w.AppealResolvedAt,
		w.IssuedAt, w.ExpiresAt, w.IsActive, w.ID, w.OrgID,
	).Scan(&w.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status WarningStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_employee_warnings SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *repoImpl) SetApprovalInstance(ctx context.Context, id, instanceID string, status WarningStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_warnings SET approval_instance_id=$1, status=$2, updated_at=NOW() WHERE id=$3`,
		instanceID, status, id,
	)
	return err
}
