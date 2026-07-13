// backend/internal/hrm/salary/repository.go
package salary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for the salary engine.
// TENANT ISOLATION: every query includes org_id. FindByRef returns nil, nil for
// not-found OR wrong-org — callers cannot distinguish the two.
type Repository interface {
	// Components
	FindAllComponents(ctx context.Context, orgID string, activeOnly bool) ([]*SalaryComponent, error)
	CountComponents(ctx context.Context, orgID string, activeOnly bool) (int, error)
	FindComponentByRef(ctx context.Context, orgID, ref string) (*SalaryComponent, error)
	CreateComponent(ctx context.Context, c *SalaryComponent, slabJSON []byte) error
	UpdateComponent(ctx context.Context, c *SalaryComponent, slabJSON []byte) error
	DeleteComponent(ctx context.Context, orgID, ref string) error
	ComponentNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)

	// Structures
	FindAllStructures(ctx context.Context, orgID string, activeOnly bool) ([]*SalaryStructure, error)
	CountStructures(ctx context.Context, orgID string, activeOnly bool) (int, error)
	FindStructureByRef(ctx context.Context, orgID, ref string) (*SalaryStructure, error)
	CreateStructure(ctx context.Context, s *SalaryStructure) error
	UpdateStructure(ctx context.Context, s *SalaryStructure) error
	DeleteStructure(ctx context.Context, orgID, ref string) error
	StructureNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)
	AddComponentToStructure(ctx context.Context, structureID, componentID string, overrideValue *float64, displayOrder int) error
	RemoveComponentFromStructure(ctx context.Context, structureID, componentID string) error
	FindStructureComponents(ctx context.Context, structureID string) ([]*StructureComponent, error)

	// Employee salary records
	FindSalaryHistory(ctx context.Context, orgID, employeeID string) ([]*EmployeeSalaryRecord, error)
	FindActiveSalaryRecord(ctx context.Context, orgID, employeeID string) (*EmployeeSalaryRecord, error)
	CreateSalaryRecord(ctx context.Context, r *EmployeeSalaryRecord) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ─────────────────────────────────────────────────────────────
// Component queries
// ─────────────────────────────────────────────────────────────

const compSelect = `
	id, public_id, org_id, name, description,
	component_type, calc_method, fixed_value,
	formula_expression, formula_variables, slab_config,
	is_taxable, display_order, is_active,
	created_by, created_at, updated_at`

func scanComponent(row pgx.Row) (*SalaryComponent, error) {
	c := &SalaryComponent{}
	var slabRaw []byte
	err := row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description,
		&c.ComponentType, &c.CalcMethod, &c.FixedValue,
		&c.FormulaExpression, &c.FormulaVariables, &slabRaw,
		&c.IsTaxable, &c.DisplayOrder, &c.IsActive,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(slabRaw) > 0 {
		c.SlabConfig = &SlabConfig{}
		if err := json.Unmarshal(slabRaw, c.SlabConfig); err != nil {
			return nil, fmt.Errorf("salary: unmarshal slab_config: %w", err)
		}
	}
	return c, nil
}

func (r *repoImpl) FindAllComponents(ctx context.Context, orgID string, activeOnly bool) ([]*SalaryComponent, error) {
	q := `SELECT ` + compSelect + ` FROM hrm_salary_components WHERE org_id = $1`
	args := []any{orgID}
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	q += ` ORDER BY display_order, name`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("salary: FindAllComponents: %w", err)
	}
	defer rows.Close()

	list := make([]*SalaryComponent, 0)
	for rows.Next() {
		c, err := scanComponent(rows)
		if err != nil {
			return nil, fmt.Errorf("salary: FindAllComponents scan: %w", err)
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountComponents(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	q := `SELECT COUNT(*) FROM hrm_salary_components WHERE org_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	var n int
	if err := r.db.QueryRow(ctx, q, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("salary: CountComponents: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindComponentByRef(ctx context.Context, orgID, ref string) (*SalaryComponent, error) {
	q := `SELECT ` + compSelect + ` FROM hrm_salary_components WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	c, err := scanComponent(r.db.QueryRow(ctx, q, orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("salary: FindComponentByRef: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CreateComponent(ctx context.Context, c *SalaryComponent, slabJSON []byte) error {
	q := `INSERT INTO hrm_salary_components
		(org_id, name, description, component_type, calc_method, fixed_value,
		 formula_expression, formula_variables, slab_config,
		 is_taxable, display_order, is_active, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, public_id, created_at, updated_at`
	return r.db.QueryRow(ctx, q,
		c.OrgID, c.Name, c.Description, c.ComponentType, c.CalcMethod, c.FixedValue,
		c.FormulaExpression, c.FormulaVariables, slabJSON,
		c.IsTaxable, c.DisplayOrder, c.IsActive, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateComponent(ctx context.Context, c *SalaryComponent, slabJSON []byte) error {
	q := `UPDATE hrm_salary_components SET
		name=$1, description=$2, component_type=$3, calc_method=$4, fixed_value=$5,
		formula_expression=$6, formula_variables=$7, slab_config=$8,
		is_taxable=$9, display_order=$10, is_active=$11, updated_at=NOW()
		WHERE id=$12 AND org_id=$13
		RETURNING updated_at`
	return r.db.QueryRow(ctx, q,
		c.Name, c.Description, c.ComponentType, c.CalcMethod, c.FixedValue,
		c.FormulaExpression, c.FormulaVariables, slabJSON,
		c.IsTaxable, c.DisplayOrder, c.IsActive, c.ID, c.OrgID,
	).Scan(&c.UpdatedAt)
}

func (r *repoImpl) DeleteComponent(ctx context.Context, orgID, ref string) error {
	q := `DELETE FROM hrm_salary_components WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	cmd, err := r.db.Exec(ctx, q, orgID, ref)
	if err != nil {
		return fmt.Errorf("salary: DeleteComponent: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrComponentNotFound
	}
	return nil
}

func (r *repoImpl) ComponentNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	q := `SELECT EXISTS(SELECT 1 FROM hrm_salary_components
		WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND is_active=TRUE AND id::text != $3)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, orgID, name, excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("salary: ComponentNameExists: %w", err)
	}
	return exists, nil
}

// ─────────────────────────────────────────────────────────────
// Structure queries
// ─────────────────────────────────────────────────────────────

const structSelect = `id, public_id, org_id, name, description, grade_label, is_active, created_by, created_at, updated_at`

func scanStructure(row pgx.Row) (*SalaryStructure, error) {
	s := &SalaryStructure{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.Name, &s.Description, &s.GradeLabel, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) FindAllStructures(ctx context.Context, orgID string, activeOnly bool) ([]*SalaryStructure, error) {
	q := `SELECT ` + structSelect + ` FROM hrm_salary_structures WHERE org_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	q += ` ORDER BY name`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("salary: FindAllStructures: %w", err)
	}
	defer rows.Close()

	list := make([]*SalaryStructure, 0)
	for rows.Next() {
		s, err := scanStructure(rows)
		if err != nil {
			return nil, fmt.Errorf("salary: FindAllStructures scan: %w", err)
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountStructures(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	q := `SELECT COUNT(*) FROM hrm_salary_structures WHERE org_id = $1`
	if activeOnly {
		q += ` AND is_active = TRUE`
	}
	var n int
	if err := r.db.QueryRow(ctx, q, orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("salary: CountStructures: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindStructureByRef(ctx context.Context, orgID, ref string) (*SalaryStructure, error) {
	q := `SELECT ` + structSelect + ` FROM hrm_salary_structures WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	s, err := scanStructure(r.db.QueryRow(ctx, q, orgID, ref))
	if err != nil {
		return nil, fmt.Errorf("salary: FindStructureByRef: %w", err)
	}
	return s, nil
}

func (r *repoImpl) CreateStructure(ctx context.Context, s *SalaryStructure) error {
	q := `INSERT INTO hrm_salary_structures (org_id,name,description,grade_label,is_active,created_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`
	return r.db.QueryRow(ctx, q, s.OrgID, s.Name, s.Description, s.GradeLabel, s.IsActive, s.CreatedBy).
		Scan(&s.ID, &s.PublicID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) UpdateStructure(ctx context.Context, s *SalaryStructure) error {
	q := `UPDATE hrm_salary_structures SET name=$1, description=$2, grade_label=$3, is_active=$4, updated_at=NOW()
		WHERE id=$5 AND org_id=$6 RETURNING updated_at`
	return r.db.QueryRow(ctx, q, s.Name, s.Description, s.GradeLabel, s.IsActive, s.ID, s.OrgID).Scan(&s.UpdatedAt)
}

func (r *repoImpl) DeleteStructure(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_salary_structures WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil {
		return fmt.Errorf("salary: DeleteStructure: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrStructureNotFound
	}
	return nil
}

func (r *repoImpl) StructureNameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_salary_structures WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND is_active=TRUE AND id::text!=$3)`,
		orgID, name, excludeID).Scan(&exists)
	return exists, err
}

func (r *repoImpl) AddComponentToStructure(ctx context.Context, structureID, componentID string, overrideValue *float64, displayOrder int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO hrm_salary_structure_components (structure_id, component_id, override_value, display_order)
		VALUES ($1,$2,$3,$4)`,
		structureID, componentID, overrideValue, displayOrder)
	if err != nil {
		if err.Error() != "" && contains(err.Error(), "uq_hrm_ssc") {
			return ErrComponentInStructure
		}
		return fmt.Errorf("salary: AddComponentToStructure: %w", err)
	}
	return nil
}

func (r *repoImpl) RemoveComponentFromStructure(ctx context.Context, structureID, componentID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_salary_structure_components WHERE structure_id=$1 AND component_id=$2`,
		structureID, componentID)
	if err != nil {
		return fmt.Errorf("salary: RemoveComponentFromStructure: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrComponentNotInStructure
	}
	return nil
}

func (r *repoImpl) FindStructureComponents(ctx context.Context, structureID string) ([]*StructureComponent, error) {
	q := `SELECT ssc.component_id, ssc.override_value, ssc.display_order,
		sc.id, sc.public_id, sc.org_id, sc.name, sc.description,
		sc.component_type, sc.calc_method, sc.fixed_value,
		sc.formula_expression, sc.formula_variables, sc.slab_config,
		sc.is_taxable, sc.display_order AS comp_order, sc.is_active,
		sc.created_by, sc.created_at, sc.updated_at
		FROM hrm_salary_structure_components ssc
		JOIN hrm_salary_components sc ON sc.id = ssc.component_id
		WHERE ssc.structure_id = $1
		ORDER BY ssc.display_order, sc.name`

	rows, err := r.db.Query(ctx, q, structureID)
	if err != nil {
		return nil, fmt.Errorf("salary: FindStructureComponents: %w", err)
	}
	defer rows.Close()

	list := make([]*StructureComponent, 0)
	for rows.Next() {
		sc := &StructureComponent{Component: &SalaryComponent{}}
		var slabRaw []byte
		if err := rows.Scan(
			&sc.ComponentID, &sc.OverrideValue, &sc.DisplayOrder,
			&sc.Component.ID, &sc.Component.PublicID, &sc.Component.OrgID,
			&sc.Component.Name, &sc.Component.Description,
			&sc.Component.ComponentType, &sc.Component.CalcMethod, &sc.Component.FixedValue,
			&sc.Component.FormulaExpression, &sc.Component.FormulaVariables, &slabRaw,
			&sc.Component.IsTaxable, &sc.Component.DisplayOrder, &sc.Component.IsActive,
			&sc.Component.CreatedBy, &sc.Component.CreatedAt, &sc.Component.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("salary: FindStructureComponents scan: %w", err)
		}
		if len(slabRaw) > 0 {
			sc.Component.SlabConfig = &SlabConfig{}
			_ = json.Unmarshal(slabRaw, sc.Component.SlabConfig)
		}
		list = append(list, sc)
	}
	return list, rows.Err()
}

// ─────────────────────────────────────────────────────────────
// Employee salary record queries
// ─────────────────────────────────────────────────────────────

func (r *repoImpl) FindSalaryHistory(ctx context.Context, orgID, employeeID string) ([]*EmployeeSalaryRecord, error) {
	q := `SELECT id, public_id, org_id, employee_id, structure_id, basic_pay,
		to_char(effective_date,'YYYY-MM-DD'), change_reason, change_notes, created_by, created_at
		FROM hrm_employee_salary_records
		WHERE org_id=$1 AND employee_id=$2
		ORDER BY effective_date DESC`
	rows, err := r.db.Query(ctx, q, orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("salary: FindSalaryHistory: %w", err)
	}
	defer rows.Close()

	list := make([]*EmployeeSalaryRecord, 0)
	for rows.Next() {
		rec := &EmployeeSalaryRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.PublicID, &rec.OrgID, &rec.EmployeeID, &rec.StructureID,
			&rec.BasicPay, &rec.EffectiveDate, &rec.ChangeReason, &rec.ChangeNotes,
			&rec.CreatedBy, &rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("salary: FindSalaryHistory scan: %w", err)
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindActiveSalaryRecord(ctx context.Context, orgID, employeeID string) (*EmployeeSalaryRecord, error) {
	q := `SELECT id, public_id, org_id, employee_id, structure_id, basic_pay,
		to_char(effective_date,'YYYY-MM-DD'), change_reason, change_notes, created_by, created_at
		FROM hrm_employee_salary_records
		WHERE org_id=$1 AND employee_id=$2 AND effective_date <= CURRENT_DATE
		ORDER BY effective_date DESC LIMIT 1`
	rec := &EmployeeSalaryRecord{}
	err := r.db.QueryRow(ctx, q, orgID, employeeID).Scan(
		&rec.ID, &rec.PublicID, &rec.OrgID, &rec.EmployeeID, &rec.StructureID,
		&rec.BasicPay, &rec.EffectiveDate, &rec.ChangeReason, &rec.ChangeNotes,
		&rec.CreatedBy, &rec.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("salary: FindActiveSalaryRecord: %w", err)
	}
	return rec, nil
}

func (r *repoImpl) CreateSalaryRecord(ctx context.Context, rec *EmployeeSalaryRecord) error {
	q := `INSERT INTO hrm_employee_salary_records
		(org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, change_notes, created_by)
		VALUES ($1,$2,$3,$4,$5::date,$6,$7,$8)
		RETURNING id, public_id, created_at`
	return r.db.QueryRow(ctx, q,
		rec.OrgID, rec.EmployeeID, rec.StructureID, rec.BasicPay,
		rec.EffectiveDate, rec.ChangeReason, rec.ChangeNotes, rec.CreatedBy,
	).Scan(&rec.ID, &rec.PublicID, &rec.CreatedAt)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
