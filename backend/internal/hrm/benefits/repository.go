// backend/internal/hrm/benefits/repository.go
package benefits

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

type Repository interface {
	ListPlans(ctx context.Context, orgID string) ([]*Plan, error)
	FindPlanByRef(ctx context.Context, orgID, ref string) (*Plan, error)
	CreatePlan(ctx context.Context, p *Plan) error

	CreateTier(ctx context.Context, planID string, t *Tier) error
	ListTiersByPlan(ctx context.Context, planID string) ([]*Tier, error)
	FindTierByRef(ctx context.Context, ref string) (*Tier, error)

	ListEnrollments(ctx context.Context, orgID string, filter ListFilter) ([]*Enrollment, int, error)
	FindEnrollmentByRef(ctx context.Context, orgID, ref string) (*Enrollment, error)
	FindActiveEnrollment(ctx context.Context, employeeID, planID string) (*Enrollment, error)
	CreateEnrollment(ctx context.Context, e *Enrollment) error
	UpdateEnrollmentStatus(ctx context.Context, id string, status EnrollmentStatus) error
	// ActivatePendingDue flips every 'pending' enrollment whose
	// effective_date has arrived to 'active', instance-wide (the
	// leave/absence-sweep shape) — returns how many it touched.
	ActivatePendingDue(ctx context.Context) (int, error)
	// ActiveEnrollmentsForEmployee returns an employee's currently active
	// enrollments — the source computePayslips' benefits stage reads.
	ActiveEnrollmentsForEmployee(ctx context.Context, orgID, employeeID string) ([]*Enrollment, error)

	CreateDependent(ctx context.Context, d *Dependent) error
	ListDependentsByEmployee(ctx context.Context, orgID, employeeID string) ([]*Dependent, error)
	FindDependentByRef(ctx context.Context, orgID, ref string) (*Dependent, error)
	VerifyDependent(ctx context.Context, id, verifiedBy string) error

	// FindEmployeeIDByUserID resolves a caller's OWN hrm_employees.id, for
	// EnrollSelf — the compensation.Repository precedent
	// (internal/hrm/compensation/revisions_repository.go).
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const planSel = `id, public_id, org_id, name, plan_type, description, is_active, created_by, created_at, updated_at`

func scanPlan(row pgx.Row) (*Plan, error) {
	p := &Plan{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.Name, &p.PlanType, &p.Description, &p.IsActive,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) ListPlans(ctx context.Context, orgID string) ([]*Plan, error) {
	rows, err := r.db.Query(ctx, `SELECT `+planSel+` FROM hrm_benefit_plans WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("benefits: ListPlans: %w", err)
	}
	defer rows.Close()
	list := make([]*Plan, 0)
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindPlanByRef(ctx context.Context, orgID, ref string) (*Plan, error) {
	return scanPlan(r.db.QueryRow(ctx,
		`SELECT `+planSel+` FROM hrm_benefit_plans WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreatePlan(ctx context.Context, p *Plan) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_benefit_plans (org_id, name, plan_type, description, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.Name, p.PlanType, p.Description, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

const tierSel = `id, public_id, plan_id, tier_name, employee_cost, employer_cost, is_active, created_by, created_at, updated_at`

func scanTier(row pgx.Row) (*Tier, error) {
	t := &Tier{}
	err := row.Scan(&t.ID, &t.PublicID, &t.PlanID, &t.TierName, &t.EmployeeCost, &t.EmployerCost, &t.IsActive,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *repoImpl) CreateTier(ctx context.Context, planID string, t *Tier) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_benefit_tiers (plan_id, tier_name, employee_cost, employer_cost, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, created_at, updated_at`,
		planID, t.TierName, t.EmployeeCost, t.EmployerCost, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) ListTiersByPlan(ctx context.Context, planID string) ([]*Tier, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+tierSel+` FROM hrm_benefit_tiers WHERE plan_id=$1::uuid ORDER BY tier_name`, planID)
	if err != nil {
		return nil, fmt.Errorf("benefits: ListTiersByPlan: %w", err)
	}
	defer rows.Close()
	list := make([]*Tier, 0)
	for rows.Next() {
		t, err := scanTier(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindTierByRef(ctx context.Context, ref string) (*Tier, error) {
	return scanTier(r.db.QueryRow(ctx,
		`SELECT `+tierSel+` FROM hrm_benefit_tiers WHERE id::text=$1 OR public_id=$1`, ref))
}

const enrollmentSel = `id, public_id, org_id, employee_id, plan_id, tier_id, enrollment_window_type, status,
	effective_date, end_date, employee_cost_snapshot, employer_cost_snapshot,
	created_by, created_at, updated_at`

func scanEnrollment(row pgx.Row) (*Enrollment, error) {
	e := &Enrollment{}
	err := row.Scan(&e.ID, &e.PublicID, &e.OrgID, &e.EmployeeID, &e.PlanID, &e.TierID,
		&e.EnrollmentWindowType, &e.Status, &e.EffectiveDate, &e.EndDate,
		&e.EmployeeCostSnapshot, &e.EmployerCostSnapshot,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *repoImpl) ListEnrollments(ctx context.Context, orgID string, filter ListFilter) ([]*Enrollment, int, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_benefit_enrollments WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("benefits: ListEnrollments: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_benefit_enrollments WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		enrollmentSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("benefits: ListEnrollments: %w", err)
	}
	defer rows.Close()
	list := make([]*Enrollment, 0)
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, e)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindEnrollmentByRef(ctx context.Context, orgID, ref string) (*Enrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx,
		`SELECT `+enrollmentSel+` FROM hrm_benefit_enrollments WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindActiveEnrollment(ctx context.Context, employeeID, planID string) (*Enrollment, error) {
	return scanEnrollment(r.db.QueryRow(ctx,
		`SELECT `+enrollmentSel+` FROM hrm_benefit_enrollments
		  WHERE employee_id=$1::uuid AND plan_id=$2::uuid AND status IN ('pending','active')`,
		employeeID, planID))
}

func (r *repoImpl) CreateEnrollment(ctx context.Context, e *Enrollment) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_benefit_enrollments
		    (org_id, employee_id, plan_id, tier_id, enrollment_window_type, status, effective_date,
		     employee_cost_snapshot, employer_cost_snapshot, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, public_id, created_at, updated_at`,
		e.OrgID, e.EmployeeID, e.PlanID, e.TierID, e.EnrollmentWindowType, e.Status, e.EffectiveDate,
		e.EmployeeCostSnapshot, e.EmployerCostSnapshot, e.CreatedBy,
	).Scan(&e.ID, &e.PublicID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil && strings.Contains(err.Error(), "uq_hrm_bfe_employee_plan_active") {
		return ErrAlreadyEnrolled
	}
	return err
}

func (r *repoImpl) UpdateEnrollmentStatus(ctx context.Context, id string, status EnrollmentStatus) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_benefit_enrollments SET status=$2, updated_at=NOW() WHERE id=$1::uuid`, id, status)
	if err != nil {
		return fmt.Errorf("benefits: UpdateEnrollmentStatus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrEnrollmentNotFound
	}
	return nil
}

func (r *repoImpl) ActivatePendingDue(ctx context.Context) (int, error) {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_benefit_enrollments
		    SET status='active', updated_at=NOW()
		  WHERE status='pending' AND effective_date <= CURRENT_DATE`)
	if err != nil {
		return 0, fmt.Errorf("benefits: ActivatePendingDue: %w", err)
	}
	return int(ct.RowsAffected()), nil
}

func (r *repoImpl) ActiveEnrollmentsForEmployee(ctx context.Context, orgID, employeeID string) ([]*Enrollment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+enrollmentSel+` FROM hrm_benefit_enrollments
		  WHERE org_id=$1 AND employee_id=$2 AND status='active'`,
		orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("benefits: ActiveEnrollmentsForEmployee: %w", err)
	}
	defer rows.Close()
	list := make([]*Enrollment, 0)
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

const dependentSel = `id, public_id, org_id, employee_id, enrollment_id, full_name, relationship,
	date_of_birth, is_verified, verified_by, verified_at, created_by, created_at, updated_at`

func scanDependent(row pgx.Row) (*Dependent, error) {
	d := &Dependent{}
	err := row.Scan(&d.ID, &d.PublicID, &d.OrgID, &d.EmployeeID, &d.EnrollmentID, &d.FullName, &d.Relationship,
		&d.DateOfBirth, &d.IsVerified, &d.VerifiedBy, &d.VerifiedAt, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *repoImpl) CreateDependent(ctx context.Context, d *Dependent) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_dependents (org_id, employee_id, enrollment_id, full_name, relationship, date_of_birth, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
		d.OrgID, d.EmployeeID, d.EnrollmentID, d.FullName, d.Relationship, d.DateOfBirth, d.CreatedBy,
	).Scan(&d.ID, &d.PublicID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *repoImpl) ListDependentsByEmployee(ctx context.Context, orgID, employeeID string) ([]*Dependent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+dependentSel+` FROM hrm_dependents WHERE org_id=$1 AND employee_id=$2 ORDER BY created_at`,
		orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("benefits: ListDependentsByEmployee: %w", err)
	}
	defer rows.Close()
	list := make([]*Dependent, 0)
	for rows.Next() {
		d, err := scanDependent(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindDependentByRef(ctx context.Context, orgID, ref string) (*Dependent, error) {
	return scanDependent(r.db.QueryRow(ctx,
		`SELECT `+dependentSel+` FROM hrm_dependents WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) VerifyDependent(ctx context.Context, id, verifiedBy string) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_dependents SET is_verified=TRUE, verified_by=$2, verified_at=NOW(), updated_at=NOW() WHERE id=$1::uuid`,
		id, verifiedBy)
	if err != nil {
		return fmt.Errorf("benefits: VerifyDependent: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrDependentNotFound
	}
	return nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM hrm_employees WHERE org_id=$1 AND user_id=$2 LIMIT 1`,
		orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("benefits: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}
