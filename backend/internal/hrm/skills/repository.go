// backend/internal/hrm/skills/repository.go
package skills

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
	// ── Taxonomy (org-level, no employee_id, never scope-filtered) ──────
	FindSkills(ctx context.Context, orgID string, f SkillListFilter) ([]*Skill, error)
	CountSkills(ctx context.Context, orgID string, f SkillListFilter) (int, error)
	FindSkillByRef(ctx context.Context, orgID, ref string) (*Skill, error)
	SkillNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
	CreateSkill(ctx context.Context, s *Skill) error
	UpdateSkill(ctx context.Context, s *Skill) error
	DeleteSkill(ctx context.Context, orgID, id string) error
	SkillInUse(ctx context.Context, skillID string) (bool, error)

	// ── Employee skills (carry employee_id, SCOPE-FILTERED) ─────────────
	FindEmployeeSkills(ctx context.Context, orgID string, f EmployeeSkillListFilter) ([]*EmployeeSkill, error)
	CountEmployeeSkills(ctx context.Context, orgID string, f EmployeeSkillListFilter) (int, error)
	FindEmployeeSkillByRef(ctx context.Context, orgID, ref string) (*EmployeeSkill, error)
	HasSkill(ctx context.Context, orgID, employeeID, skillID string) (bool, error)
	GrantSkill(ctx context.Context, es *EmployeeSkill) error
	UpdateEmployeeSkill(ctx context.Context, es *EmployeeSkill) error
	RevokeSkill(ctx context.Context, orgID, id string) error

	EmployeeExists(ctx context.Context, orgID, employeeRef string) (string, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Taxonomy ─────────────────────────────────────────────────────────────────

const skillCols = `id, public_id, org_id, name, description, category, is_active,
	created_by, created_at, updated_at`

func scanSkill(row pgx.Row) (*Skill, error) {
	s := &Skill{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.Name, &s.Description, &s.Category,
		&s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func buildSkillWhere(orgID string, f SkillListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.Category != "" {
		args = append(args, f.Category)
		clauses = append(clauses, fmt.Sprintf("category = $%d", len(args)))
	}
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

func (r *repoImpl) FindSkills(ctx context.Context, orgID string, f SkillListFilter) ([]*Skill, error) {
	where, args := buildSkillWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_skills WHERE %s ORDER BY name LIMIT $%d OFFSET $%d`,
			skillCols, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("skills: FindSkills: %w", err)
	}
	defer rows.Close()

	out := make([]*Skill, 0)
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("skills: FindSkills: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountSkills(ctx context.Context, orgID string, f SkillListFilter) (int, error) {
	where, args := buildSkillWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_skills WHERE %s`, where), args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("skills: CountSkills: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindSkillByRef(ctx context.Context, orgID, ref string) (*Skill, error) {
	s, err := scanSkill(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_skills WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
			skillCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("skills: FindSkillByRef: %w", err)
	}
	return s, nil
}

func (r *repoImpl) SkillNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_skills
		  WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND ($3='' OR id::text<>$3))`,
		orgID, name, excludeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("skills: SkillNameExists: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) CreateSkill(ctx context.Context, s *Skill) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_skills (org_id, name, description, category, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, public_id, is_active, created_at, updated_at`,
		s.OrgID, s.Name, s.Description, s.Category, s.CreatedBy,
	).Scan(&s.ID, &s.PublicID, &s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("skills: CreateSkill: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateSkill(ctx context.Context, s *Skill) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_skills SET name=$1, description=$2, category=$3, is_active=$4, updated_at=NOW()
		 WHERE org_id=$5 AND id=$6 RETURNING updated_at`,
		s.Name, s.Description, s.Category, s.IsActive, s.OrgID, s.ID,
	).Scan(&s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSkillNotFound
	}
	if err != nil {
		return fmt.Errorf("skills: UpdateSkill: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteSkill(ctx context.Context, orgID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_skills WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("skills: DeleteSkill: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrSkillNotFound
	}
	return nil
}

// SkillInUse guards the delete. hrm_employee_skills.skill_id is ON DELETE
// CASCADE, so Postgres would happily erase everybody's record of holding the
// skill — checking first turns a silent data loss into a refusal.
func (r *repoImpl) SkillInUse(ctx context.Context, skillID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employee_skills WHERE skill_id=$1)`, skillID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("skills: SkillInUse: %w", err)
	}
	return exists, nil
}

// ── Employee skills ──────────────────────────────────────────────────────────

const empSkillCols = `id, public_id, org_id, employee_id, skill_id, proficiency, source,
	source_enrollment_id, source_certification_id, acquired_on, notes, created_by,
	created_at, updated_at`

func scanEmployeeSkill(row pgx.Row) (*EmployeeSkill, error) {
	es := &EmployeeSkill{}
	err := row.Scan(&es.ID, &es.PublicID, &es.OrgID, &es.EmployeeID, &es.SkillID, &es.Proficiency,
		&es.Source, &es.SourceEnrollmentID, &es.SourceCertificationID, &es.AcquiredOn,
		&es.Notes, &es.CreatedBy, &es.CreatedAt, &es.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return es, nil
}

func buildEmpSkillWhere(orgID string, f EmployeeSkillListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id = $1"}
	if f.EmployeeID != "" {
		args = append(args, f.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if f.SkillID != "" {
		args = append(args, f.SkillID)
		clauses = append(clauses, fmt.Sprintf("skill_id = $%d", len(args)))
	}
	if f.Source != "" {
		args = append(args, f.Source)
		clauses = append(clauses, fmt.Sprintf("source = $%d", len(args)))
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

func (r *repoImpl) FindEmployeeSkills(ctx context.Context, orgID string, f EmployeeSkillListFilter) ([]*EmployeeSkill, error) {
	where, args := buildEmpSkillWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	// No table alias: the scope predicate embeds its own org_id inside a
	// FROM hrm_employees subquery, so the WHERE clause must stay unqualified.
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s,
			(SELECT name FROM hrm_skills s WHERE s.id = hrm_employee_skills.skill_id)
			FROM hrm_employee_skills WHERE %s
			ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
			empSkillCols, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("skills: FindEmployeeSkills: %w", err)
	}
	defer rows.Close()

	out := make([]*EmployeeSkill, 0)
	for rows.Next() {
		es := &EmployeeSkill{}
		if err := rows.Scan(&es.ID, &es.PublicID, &es.OrgID, &es.EmployeeID, &es.SkillID,
			&es.Proficiency, &es.Source, &es.SourceEnrollmentID, &es.SourceCertificationID,
			&es.AcquiredOn, &es.Notes, &es.CreatedBy, &es.CreatedAt, &es.UpdatedAt,
			&es.SkillName); err != nil {
			return nil, fmt.Errorf("skills: FindEmployeeSkills: scan: %w", err)
		}
		out = append(out, es)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountEmployeeSkills(ctx context.Context, orgID string, f EmployeeSkillListFilter) (int, error) {
	where, args := buildEmpSkillWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_employee_skills WHERE %s`, where), args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("skills: CountEmployeeSkills: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindEmployeeSkillByRef(ctx context.Context, orgID, ref string) (*EmployeeSkill, error) {
	es, err := scanEmployeeSkill(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM hrm_employee_skills
			WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, empSkillCols), orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("skills: FindEmployeeSkillByRef: %w", err)
	}
	return es, nil
}

func (r *repoImpl) HasSkill(ctx context.Context, orgID, employeeID, skillID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_employee_skills
		  WHERE org_id=$1 AND employee_id=$2 AND skill_id=$3)`,
		orgID, employeeID, skillID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("skills: HasSkill: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) GrantSkill(ctx context.Context, es *EmployeeSkill) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_skills
		    (org_id, employee_id, skill_id, proficiency, source,
		     source_enrollment_id, source_certification_id, acquired_on, notes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, public_id, created_at, updated_at`,
		es.OrgID, es.EmployeeID, es.SkillID, es.Proficiency, es.Source,
		es.SourceEnrollmentID, es.SourceCertificationID, es.AcquiredOn, es.Notes, es.CreatedBy,
	).Scan(&es.ID, &es.PublicID, &es.CreatedAt, &es.UpdatedAt)
	if err != nil {
		return fmt.Errorf("skills: GrantSkill: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateEmployeeSkill(ctx context.Context, es *EmployeeSkill) error {
	err := r.db.QueryRow(ctx,
		`UPDATE hrm_employee_skills SET proficiency=$1, acquired_on=$2, notes=$3, updated_at=NOW()
		 WHERE org_id=$4 AND id=$5 RETURNING updated_at`,
		es.Proficiency, es.AcquiredOn, es.Notes, es.OrgID, es.ID,
	).Scan(&es.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEmployeeSkillNotFound
	}
	if err != nil {
		return fmt.Errorf("skills: UpdateEmployeeSkill: %w", err)
	}
	return nil
}

func (r *repoImpl) RevokeSkill(ctx context.Context, orgID, id string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_employee_skills WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return fmt.Errorf("skills: RevokeSkill: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrEmployeeSkillNotFound
	}
	return nil
}

// EmployeeExists resolves an employee reference to its id. This package owns
// the query rather than importing internal/hrm/employees — the onboarding /
// feedback / pip precedent.
func (r *repoImpl) EmployeeExists(ctx context.Context, orgID, employeeRef string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM hrm_employees WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, employeeRef).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("skills: EmployeeExists: %w", err)
	}
	return id, nil
}
