// backend/internal/hrm/leave/repository.go
package leave

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for HRM leave types and leave requests.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
type Repository interface {
	// Leave Types
	FindAllLeaveTypes(ctx context.Context, orgID string, activeOnly bool) ([]*LeaveType, error)
	FindLeaveTypeByRef(ctx context.Context, orgID, ref string) (*LeaveType, error)
	CreateLeaveType(ctx context.Context, lt *LeaveType) error
	UpdateLeaveType(ctx context.Context, lt *LeaveType) error
	DeleteLeaveType(ctx context.Context, orgID, ref string) error
	LeaveTypeExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error)

	// Leave Requests
	FindAllRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) ([]*LeaveRequest, error)
	CountRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) (int, error)
	FindRequestByRef(ctx context.Context, orgID, ref string) (*LeaveRequest, error)
	CreateRequest(ctx context.Context, r *LeaveRequest) error
	UpdateRequest(ctx context.Context, r *LeaveRequest) error
	DeleteRequest(ctx context.Context, orgID, ref string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ─────────────────────────────────────────────────────────
// Leave Type repo
// ─────────────────────────────────────────────────────────

const ltSelect = `id, public_id, org_id, name, description, max_days_per_year, is_paid, requires_approval, is_active, created_by, created_at, updated_at`

func scanLeaveType(row pgx.Row) (*LeaveType, error) {
	lt := &LeaveType{}
	err := row.Scan(
		&lt.ID, &lt.PublicID, &lt.OrgID, &lt.Name, &lt.Description,
		&lt.MaxDaysPerYear, &lt.IsPaid, &lt.RequiresApproval, &lt.IsActive,
		&lt.CreatedBy, &lt.CreatedAt, &lt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return lt, nil
}

func (r *repoImpl) FindAllLeaveTypes(ctx context.Context, orgID string, activeOnly bool) ([]*LeaveType, error) {
	where := "org_id = $1"
	args := []any{orgID}
	if activeOnly {
		where += " AND is_active = TRUE"
	}
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM hrm_leave_types WHERE %s ORDER BY name ASC`, ltSelect, where), args...)
	if err != nil {
		return nil, fmt.Errorf("leave: FindAllLeaveTypes: %w", err)
	}
	defer rows.Close()

	var list []*LeaveType
	for rows.Next() {
		lt, err := scanLeaveType(rows)
		if err != nil {
			return nil, fmt.Errorf("leave: FindAllLeaveTypes: scan: %w", err)
		}
		list = append(list, lt)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindLeaveTypeByRef(ctx context.Context, orgID, ref string) (*LeaveType, error) {
	q := `SELECT ` + ltSelect + ` FROM hrm_leave_types WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	lt, err := scanLeaveType(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("leave: FindLeaveTypeByRef: %w", err)
	}
	return lt, nil
}

func (r *repoImpl) CreateLeaveType(ctx context.Context, lt *LeaveType) error {
	const q = `
		INSERT INTO hrm_leave_types
		    (org_id, name, description, max_days_per_year, is_paid, requires_approval, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + ltSelect

	created, err := scanLeaveType(r.db.QueryRow(ctx, q,
		lt.OrgID, lt.Name, lt.Description, lt.MaxDaysPerYear,
		lt.IsPaid, lt.RequiresApproval, lt.IsActive, lt.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("leave: CreateLeaveType: %w", err)
	}
	*lt = *created
	return nil
}

func (r *repoImpl) UpdateLeaveType(ctx context.Context, lt *LeaveType) error {
	const q = `
		UPDATE hrm_leave_types
		SET name = $1, description = $2, max_days_per_year = $3,
		    is_paid = $4, requires_approval = $5, is_active = $6, updated_at = NOW()
		WHERE id = $7 AND org_id = $8
		RETURNING ` + ltSelect

	updated, err := scanLeaveType(r.db.QueryRow(ctx, q,
		lt.Name, lt.Description, lt.MaxDaysPerYear,
		lt.IsPaid, lt.RequiresApproval, lt.IsActive, lt.ID, lt.OrgID,
	))
	if err != nil {
		return fmt.Errorf("leave: UpdateLeaveType: %w", err)
	}
	if updated == nil {
		return ErrLeaveTypeNotFound
	}
	*lt = *updated
	return nil
}

func (r *repoImpl) DeleteLeaveType(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_leave_types WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`,
		orgID, strings.TrimSpace(ref),
	)
	if err != nil {
		return fmt.Errorf("leave: DeleteLeaveType: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLeaveTypeNotFound
	}
	return nil
}

func (r *repoImpl) LeaveTypeExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_leave_types
		WHERE org_id = $1 AND LOWER(name) = LOWER($2) AND is_active = TRUE AND id::TEXT != $3
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(name), excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("leave: LeaveTypeExistsByName: %w", err)
	}
	return exists, nil
}

// ─────────────────────────────────────────────────────────
// Leave Request repo
// ─────────────────────────────────────────────────────────

const lrSelect = `id, public_id, org_id, employee_id, leave_type_id, start_date, end_date, total_days, reason, status, reviewed_by, reviewed_at, review_note, created_by, created_at, updated_at`

func scanLeaveRequest(row pgx.Row) (*LeaveRequest, error) {
	lr := &LeaveRequest{}
	err := row.Scan(
		&lr.ID, &lr.PublicID, &lr.OrgID, &lr.EmployeeID, &lr.LeaveTypeID,
		&lr.StartDate, &lr.EndDate, &lr.TotalDays, &lr.Reason, &lr.Status,
		&lr.ReviewedBy, &lr.ReviewedAt, &lr.ReviewNote,
		&lr.CreatedBy, &lr.CreatedAt, &lr.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return lr, nil
}

func buildRequestWhere(orgID string, filter LeaveRequestFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}

	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id::TEXT = $%d", len(args)))
	}
	if filter.LeaveTypeID != "" {
		args = append(args, filter.LeaveTypeID)
		clauses = append(clauses, fmt.Sprintf("leave_type_id::TEXT = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAllRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) ([]*LeaveRequest, error) {
	where, args := buildRequestWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`
		SELECT %s FROM hrm_leave_requests
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`,
		lrSelect, where, len(args)-1, len(args),
	)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("leave: FindAllRequests: %w", err)
	}
	defer rows.Close()

	var list []*LeaveRequest
	for rows.Next() {
		lr, err := scanLeaveRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("leave: FindAllRequests: scan: %w", err)
		}
		list = append(list, lr)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountRequests(ctx context.Context, orgID string, filter LeaveRequestFilter) (int, error) {
	where, args := buildRequestWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_leave_requests WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("leave: CountRequests: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindRequestByRef(ctx context.Context, orgID, ref string) (*LeaveRequest, error) {
	q := `SELECT ` + lrSelect + `
		FROM hrm_leave_requests
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	lr, err := scanLeaveRequest(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("leave: FindRequestByRef: %w", err)
	}
	return lr, nil
}

func (r *repoImpl) CreateRequest(ctx context.Context, lr *LeaveRequest) error {
	const q = `
		INSERT INTO hrm_leave_requests
		    (org_id, employee_id, leave_type_id, start_date, end_date, total_days, reason, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + lrSelect

	created, err := scanLeaveRequest(r.db.QueryRow(ctx, q,
		lr.OrgID, lr.EmployeeID, lr.LeaveTypeID,
		lr.StartDate, lr.EndDate, lr.TotalDays,
		lr.Reason, lr.Status, lr.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("leave: CreateRequest: %w", err)
	}
	*lr = *created
	return nil
}

func (r *repoImpl) UpdateRequest(ctx context.Context, lr *LeaveRequest) error {
	const q = `
		UPDATE hrm_leave_requests
		SET status = $1, reviewed_by = $2, reviewed_at = $3, review_note = $4, updated_at = NOW()
		WHERE id = $5 AND org_id = $6
		RETURNING ` + lrSelect

	updated, err := scanLeaveRequest(r.db.QueryRow(ctx, q,
		lr.Status, lr.ReviewedBy, lr.ReviewedAt, lr.ReviewNote, lr.ID, lr.OrgID,
	))
	if err != nil {
		return fmt.Errorf("leave: UpdateRequest: %w", err)
	}
	if updated == nil {
		return ErrLeaveRequestNotFound
	}
	*lr = *updated
	return nil
}

func (r *repoImpl) DeleteRequest(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_leave_requests WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`,
		orgID, strings.TrimSpace(ref),
	)
	if err != nil {
		return fmt.Errorf("leave: DeleteRequest: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLeaveRequestNotFound
	}
	return nil
}
