// backend/internal/hrm/certifications/repository.go
package certifications

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
	// ── Catalogue (org-level, never scope-filtered) ─────────────────────
	FindCertifications(ctx context.Context, orgID string, f CertificationListFilter) ([]*Certification, error)
	CountCertifications(ctx context.Context, orgID string, f CertificationListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Certification, error)
	NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
	Create(ctx context.Context, c *Certification) error
	Update(ctx context.Context, c *Certification) error
	Delete(ctx context.Context, orgID, id string) error
	CertificationInUse(ctx context.Context, certificationID string) (bool, error)

	// ── Employee credentials (carry employee_id, SCOPE-FILTERED) ────────
	FindEmployeeCertifications(ctx context.Context, orgID string, f EmployeeCertificationListFilter) ([]*EmployeeCertification, error)
	CountEmployeeCertifications(ctx context.Context, orgID string, f EmployeeCertificationListFilter) (int, error)
	FindEmployeeCertByRef(ctx context.Context, orgID, ref string) (*EmployeeCertification, error)
	HasLiveCredential(ctx context.Context, orgID, employeeID, certificationID string) (bool, error)
	Issue(ctx context.Context, ec *EmployeeCertification) error
	UpdateEmployeeCert(ctx context.Context, ec *EmployeeCertification) error
	SetStatus(ctx context.Context, orgID, id string, status Status) (*EmployeeCertification, error)

	// ── The expiry sweep ────────────────────────────────────────────────
	// MarkExpiring flips active credentials lapsing within `days` to
	// 'expiring' and stamps expiry_notified_at, so the same credential is not
	// re-notified every night. Returns rows affected.
	MarkExpiring(ctx context.Context, days int) (int, error)
	// MarkExpired flips anything past its date to 'expired'. Runs AFTER
	// MarkExpiring so a credential that lapsed today lands in the right state.
	MarkExpired(ctx context.Context) (int, error)

	EmployeeExists(ctx context.Context, orgID, employeeRef string) (string, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Catalogue ────────────────────────────────────────────────────────────────

const certCols = `id, public_id, org_id, name, description, issuing_body, validity_months,
	course_id, skill_id, is_active, created_by, created_at, updated_at`

func scanCert(row pgx.Row) (*Certification, error) {
	c := &Certification{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.IssuingBody,
		&c.ValidityMonths, &c.CourseID, &c.SkillID, &c.IsActive, &c.CreatedBy,
		&c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func buildCertWhere(orgID string, f CertificationListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.IsActive != nil {
		args = append(args, *f.IsActive)
		clauses = append(clauses, fmt.Sprintf("is_active = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		clauses = append(clauses, fmt.Sprintf("name ILIKE $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindCertifications(ctx context.Context, orgID string, f CertificationListFilter) ([]*Certification, error) {
	where, args := buildCertWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_certifications WHERE %s ORDER BY name LIMIT $%d OFFSET $%d`,
			certCols, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("certifications: FindCertifications: %w", err)
	}
	defer rows.Close()

	out := make([]*Certification, 0)
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, fmt.Errorf("certifications: FindCertifications: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountCertifications(ctx context.Context, orgID string, f CertificationListFilter) (int, error) {
	where, args := buildCertWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_certifications WHERE %s`, where), args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("certifications: CountCertifications: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Certification, error) {
	c, err := scanCert(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_certifications WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
			certCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("certifications: FindByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_certifications
		  WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND ($3='' OR id::text<>$3))`,
		orgID, name, excludeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("certifications: NameExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) Create(ctx context.Context, c *Certification) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_certifications
		    (org_id, name, description, issuing_body, validity_months, course_id, skill_id, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, public_id, is_active, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.IssuingBody, c.ValidityMonths, c.CourseID, c.SkillID, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("certifications: Create: %w", err)
	}
	return nil
}

func (r *repoImpl) Update(ctx context.Context, c *Certification) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_certifications SET name=$1, description=$2, issuing_body=$3,
		    validity_months=$4, course_id=$5, skill_id=$6, is_active=$7, updated_at=NOW()
		 WHERE org_id=$8 AND id=$9 RETURNING updated_at`,
		c.Name, c.Description, c.IssuingBody, c.ValidityMonths, c.CourseID, c.SkillID,
		c.IsActive, c.OrgID, c.ID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("certifications: Update: %w", err)
	}
	return nil
}

func (r *repoImpl) Delete(ctx context.Context, orgID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_certifications WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("certifications: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CertificationInUse mirrors the RESTRICT FK on hrm_employee_certifications,
// turning a raw 23503 into a usable message.
func (r *repoImpl) CertificationInUse(ctx context.Context, certificationID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employee_certifications WHERE certification_id=$1)`,
		certificationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("certifications: CertificationInUse: %w", err)
	}
	return exists, nil
}

// ── Employee credentials ─────────────────────────────────────────────────────

const empCertCols = `id, public_id, org_id, employee_id, certification_id, enrollment_id,
	credential_id, issued_on, expires_at, status, expiry_notified_at, notes, issued_by,
	created_at, updated_at`

func scanEmpCert(row pgx.Row) (*EmployeeCertification, error) {
	ec := &EmployeeCertification{}
	err := row.Scan(&ec.ID, &ec.PublicID, &ec.OrgID, &ec.EmployeeID, &ec.CertificationID,
		&ec.EnrollmentID, &ec.CredentialID, &ec.IssuedOn, &ec.ExpiresAt, &ec.Status,
		&ec.ExpiryNotifiedAt, &ec.Notes, &ec.IssuedBy, &ec.CreatedAt, &ec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ec, nil
}

func buildEmpCertWhere(orgID string, f EmployeeCertificationListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.EmployeeID != "" {
		args = append(args, f.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if f.CertificationID != "" {
		args = append(args, f.CertificationID)
		clauses = append(clauses, fmt.Sprintf("certification_id = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.ExpiringWithinDays > 0 {
		args = append(args, f.ExpiringWithinDays)
		clauses = append(clauses, fmt.Sprintf(
			"expires_at IS NOT NULL AND expires_at <= CURRENT_DATE + make_interval(days => $%d) "+
				"AND status IN ('active','expiring')", len(args)))
	}
	// Scope predicate last so its placeholder offset accounts for every filter
	// above it.
	if f.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(f.Scope, "employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindEmployeeCertifications(ctx context.Context, orgID string, f EmployeeCertificationListFilter) ([]*EmployeeCertification, error) {
	where, args := buildEmpCertWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	// No table alias: the scope predicate embeds its own org_id inside a
	// FROM hrm_employees subquery, so the WHERE clause must stay unqualified.
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s,
			(SELECT name FROM hrm_certifications c WHERE c.id = hrm_employee_certifications.certification_id)
			FROM hrm_employee_certifications WHERE %s
			ORDER BY expires_at NULLS LAST, created_at DESC LIMIT $%d OFFSET $%d`,
			empCertCols, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("certifications: FindEmployeeCertifications: %w", err)
	}
	defer rows.Close()

	out := make([]*EmployeeCertification, 0)
	for rows.Next() {
		ec := &EmployeeCertification{}
		if err := rows.Scan(&ec.ID, &ec.PublicID, &ec.OrgID, &ec.EmployeeID, &ec.CertificationID,
			&ec.EnrollmentID, &ec.CredentialID, &ec.IssuedOn, &ec.ExpiresAt, &ec.Status,
			&ec.ExpiryNotifiedAt, &ec.Notes, &ec.IssuedBy, &ec.CreatedAt, &ec.UpdatedAt,
			&ec.CertificationName); err != nil {
			return nil, fmt.Errorf("certifications: FindEmployeeCertifications: scan: %w", err)
		}
		out = append(out, ec)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountEmployeeCertifications(ctx context.Context, orgID string, f EmployeeCertificationListFilter) (int, error) {
	where, args := buildEmpCertWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_employee_certifications WHERE %s`, where),
		args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("certifications: CountEmployeeCertifications: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindEmployeeCertByRef(ctx context.Context, orgID, ref string) (*EmployeeCertification, error) {
	ec, err := scanEmpCert(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_employee_certifications
			WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, empCertCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("certifications: FindEmployeeCertByRef: %w", err)
	}
	return ec, nil
}

func (r *repoImpl) HasLiveCredential(ctx context.Context, orgID, employeeID, certificationID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employee_certifications
		  WHERE org_id=$1 AND employee_id=$2 AND certification_id=$3
		    AND status IN ('active','expiring'))`,
		orgID, employeeID, certificationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("certifications: HasLiveCredential: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) Issue(ctx context.Context, ec *EmployeeCertification) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_certifications
		    (org_id, employee_id, certification_id, enrollment_id, credential_id,
		     issued_on, expires_at, notes, issued_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, status, created_at, updated_at`,
		ec.OrgID, ec.EmployeeID, ec.CertificationID, ec.EnrollmentID, ec.CredentialID,
		ec.IssuedOn, ec.ExpiresAt, ec.Notes, ec.IssuedBy,
	).Scan(&ec.ID, &ec.PublicID, &ec.Status, &ec.CreatedAt, &ec.UpdatedAt)
	if err != nil {
		return fmt.Errorf("certifications: Issue: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateEmployeeCert(ctx context.Context, ec *EmployeeCertification) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_employee_certifications SET credential_id=$1, expires_at=$2, notes=$3,
		    updated_at=NOW()
		 WHERE org_id=$4 AND id=$5 RETURNING updated_at`,
		ec.CredentialID, ec.ExpiresAt, ec.Notes, ec.OrgID, ec.ID,
	).Scan(&ec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEmployeeCertNotFound
	}
	if err != nil {
		return fmt.Errorf("certifications: UpdateEmployeeCert: %w", err)
	}
	return nil
}

func (r *repoImpl) SetStatus(ctx context.Context, orgID, id string, status Status) (*EmployeeCertification, error) {
	ec, err := scanEmpCert(r.db.QueryRow(ctx,
		fmt.Sprintf(`UPDATE hrm_employee_certifications SET status=$1, updated_at=NOW()
		 WHERE org_id=$2 AND id=$3 RETURNING %s`, empCertCols), status, orgID, id))
	if err != nil {
		return nil, fmt.Errorf("certifications: SetStatus: %w", err)
	}
	if ec == nil {
		return nil, ErrEmployeeCertNotFound
	}
	return ec, nil
}

// ── The expiry sweep ─────────────────────────────────────────────────────────

// MarkExpiring flips ACTIVE credentials lapsing within `days` to 'expiring'.
//
// The expiry_notified_at guard is what stops the job re-notifying the same
// credential every night for a month: only rows not yet stamped are touched.
// Cross-org by design — the scheduler runs once for the whole instance, the
// same shape as the leave accrual and absence sweep jobs.
func (r *repoImpl) MarkExpiring(ctx context.Context, days int) (int, error) {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_certifications SET
		    status = 'expiring', expiry_notified_at = NOW(), updated_at = NOW()
		 WHERE status = 'active'
		   AND expires_at IS NOT NULL
		   AND expires_at > CURRENT_DATE
		   AND expires_at <= CURRENT_DATE + make_interval(days => $1)
		   AND expiry_notified_at IS NULL`, days)
	if err != nil {
		return 0, fmt.Errorf("certifications: MarkExpiring: %w", err)
	}
	return int(cmd.RowsAffected()), nil
}

// MarkExpired flips anything past its date to 'expired'.
//
// The boundary is strict: expires_at < CURRENT_DATE, so a credential expiring
// TODAY is still valid today. Using <= would cut somebody off a day early,
// which for a safety certification is a real operational error.
func (r *repoImpl) MarkExpired(ctx context.Context) (int, error) {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_certifications SET status = 'expired', updated_at = NOW()
		 WHERE status IN ('active','expiring')
		   AND expires_at IS NOT NULL
		   AND expires_at < CURRENT_DATE`)
	if err != nil {
		return 0, fmt.Errorf("certifications: MarkExpired: %w", err)
	}
	return int(cmd.RowsAffected()), nil
}

func (r *repoImpl) EmployeeExists(ctx context.Context, orgID, employeeRef string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, employeeRef).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("certifications: EmployeeExists: %w", err)
	}
	return id, nil
}
