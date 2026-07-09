// backend/internal/hrm/reports/repository.go
package reports

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles read-only aggregate queries for HRM reports.
// All queries use a single correlated-subquery row or a GROUP BY scan;
// no write operations ever occur in this package.
type Repository interface {
	GetSummary(ctx context.Context, orgID string) (*HRMSummary, error)
	GetHeadcountByDepartment(ctx context.Context, orgID string) ([]*HeadcountByDepartment, error)
	GetLeaveSummary(ctx context.Context, orgID string) ([]*LeaveSummaryByType, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new HRM reports repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// GetSummary returns high-level HRM KPIs in a single round-trip.
// Each sub-select runs independently inside Postgres and is folded into
// one output row — semantically equivalent to seven separate queries but
// without the network overhead.
func (r *repoImpl) GetSummary(ctx context.Context, orgID string) (*HRMSummary, error) {
	const q = `
		SELECT
		    (SELECT COUNT(*) FROM hrm_employees    WHERE org_id = $1),
		    (SELECT COUNT(*) FROM hrm_employees    WHERE org_id = $1 AND status = 'active'),
		    (SELECT COUNT(*) FROM hrm_employees    WHERE org_id = $1 AND status = 'on_leave'),
		    (SELECT COUNT(*) FROM hrm_employees    WHERE org_id = $1 AND status = 'terminated'),
		    (SELECT COUNT(*) FROM hrm_departments  WHERE org_id = $1 AND is_active = TRUE),
		    (SELECT COUNT(*) FROM hrm_positions    WHERE org_id = $1 AND is_active = TRUE),
		    (SELECT COUNT(*) FROM hrm_leave_requests WHERE org_id = $1 AND status = 'pending'),
		    (SELECT COUNT(*) FROM hrm_leave_requests
		        WHERE org_id = $1
		          AND status = 'approved'
		          AND start_date <= CURRENT_DATE
		          AND end_date   >= CURRENT_DATE)`

	s := &HRMSummary{}
	err := r.db.QueryRow(ctx, q, orgID).Scan(
		&s.TotalEmployees,
		&s.ActiveEmployees,
		&s.OnLeaveEmployees,
		&s.TerminatedEmployees,
		&s.TotalDepartments,
		&s.TotalPositions,
		&s.PendingLeaveRequests,
		&s.ApprovedLeaveToday,
	)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetSummary: %w", err)
	}
	return s, nil
}

// GetHeadcountByDepartment returns active employee counts per department,
// ordered by headcount descending. Departments with zero active employees
// are excluded.
func (r *repoImpl) GetHeadcountByDepartment(ctx context.Context, orgID string) ([]*HeadcountByDepartment, error) {
	const q = `
		SELECT
		    d.public_id   AS department_id,
		    d.name        AS department_name,
		    COUNT(e.id)   AS headcount
		FROM hrm_departments d
		JOIN hrm_employees e
		    ON e.department_id = d.id
		   AND e.org_id        = $1
		   AND e.status        = 'active'
		WHERE d.org_id    = $1
		  AND d.is_active = TRUE
		GROUP BY d.id, d.public_id, d.name
		ORDER BY headcount DESC, d.name ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetHeadcountByDepartment: %w", err)
	}
	defer rows.Close()

	result := make([]*HeadcountByDepartment, 0)
	for rows.Next() {
		h := &HeadcountByDepartment{}
		if err := rows.Scan(&h.DepartmentID, &h.DepartmentName, &h.Headcount); err != nil {
			return nil, fmt.Errorf("hrm reports: GetHeadcountByDepartment scan: %w", err)
		}
		result = append(result, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hrm reports: GetHeadcountByDepartment rows: %w", err)
	}
	return result, nil
}

// GetLeaveSummary returns leave request statistics per leave type,
// including total requests, counts by status, and total approved days.
func (r *repoImpl) GetLeaveSummary(ctx context.Context, orgID string) ([]*LeaveSummaryByType, error) {
	const q = `
		SELECT
		    lt.public_id                                                  AS leave_type_id,
		    lt.name                                                       AS leave_type_name,
		    COUNT(lr.id)                                                  AS total_requests,
		    COUNT(lr.id) FILTER (WHERE lr.status = 'approved')           AS approved,
		    COUNT(lr.id) FILTER (WHERE lr.status = 'pending')            AS pending,
		    COUNT(lr.id) FILTER (WHERE lr.status = 'rejected')           AS rejected,
		    COALESCE(SUM(lr.total_days) FILTER (WHERE lr.status = 'approved'), 0) AS total_days
		FROM hrm_leave_types lt
		LEFT JOIN hrm_leave_requests lr
		    ON lr.leave_type_id = lt.id
		   AND lr.org_id        = $1
		WHERE lt.org_id = $1
		GROUP BY lt.id, lt.public_id, lt.name
		ORDER BY total_requests DESC, lt.name ASC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("hrm reports: GetLeaveSummary: %w", err)
	}
	defer rows.Close()

	result := make([]*LeaveSummaryByType, 0)
	for rows.Next() {
		s := &LeaveSummaryByType{}
		if err := rows.Scan(
			&s.LeaveTypeID, &s.LeaveTypeName,
			&s.TotalRequests, &s.Approved, &s.Pending, &s.Rejected,
			&s.TotalDays,
		); err != nil {
			return nil, fmt.Errorf("hrm reports: GetLeaveSummary scan: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hrm reports: GetLeaveSummary rows: %w", err)
	}
	return result, nil
}
