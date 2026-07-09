// backend/internal/hrm/contracts/repository.go
package contracts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID string) ([]*EmployeeContract, error)
	FindActive(ctx context.Context, orgID, employeeID string) (*EmployeeContract, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeContract, error)
	Create(ctx context.Context, c *EmployeeContract) error
	Update(ctx context.Context, c *EmployeeContract) error
	Deactivate(ctx context.Context, orgID, employeeID, ref string) error
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const cSel = `id, public_id, org_id, employee_id, contract_type,
	to_char(start_date,'YYYY-MM-DD'), to_char(end_date,'YYYY-MM-DD'), to_char(probation_end_date,'YYYY-MM-DD'),
	notice_period_days, salary_structure_id, work_hours_per_week,
	document_id, is_active, notes, created_by, created_at, updated_at`

func sc(row pgx.Row) (*EmployeeContract, error) {
	c := &EmployeeContract{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.EmployeeID, &c.ContractType,
		&c.StartDate, &c.EndDate, &c.ProbationEndDate,
		&c.NoticePeriodDays, &c.SalaryStructureID, &c.WorkHoursPerWeek,
		&c.DocumentID, &c.IsActive, &c.Notes, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return c, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID string) ([]*EmployeeContract, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+cSel+` FROM hrm_employee_contracts WHERE org_id=$1 AND employee_id=$2 ORDER BY start_date DESC`,
		orgID, employeeID)
	if err != nil { return nil, fmt.Errorf("contracts: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*EmployeeContract, 0)
	for rows.Next() { c, err := sc(rows); if err != nil { return nil, err }; list = append(list, c) }
	return list, rows.Err()
}

func (r *repoImpl) FindActive(ctx context.Context, orgID, employeeID string) (*EmployeeContract, error) {
	return sc(r.db.QueryRow(ctx,
		`SELECT `+cSel+` FROM hrm_employee_contracts WHERE org_id=$1 AND employee_id=$2 AND is_active=TRUE`,
		orgID, employeeID))
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*EmployeeContract, error) {
	return sc(r.db.QueryRow(ctx,
		`SELECT `+cSel+` FROM hrm_employee_contracts WHERE org_id=$1 AND employee_id=$2 AND (id::text=$3 OR public_id=$3)`,
		orgID, employeeID, ref))
}

func (r *repoImpl) Create(ctx context.Context, c *EmployeeContract) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_contracts (org_id,employee_id,contract_type,start_date,end_date,probation_end_date,
		notice_period_days,salary_structure_id,work_hours_per_week,document_id,is_active,notes,created_by)
		VALUES ($1,$2,$3,$4::date,$5::date,$6::date,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.EmployeeID, c.ContractType, c.StartDate, c.EndDate, c.ProbationEndDate,
		c.NoticePeriodDays, c.SalaryStructureID, c.WorkHoursPerWeek, c.DocumentID, c.IsActive, c.Notes, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, c *EmployeeContract) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_employee_contracts SET end_date=$1::date,probation_end_date=$2::date,
		notice_period_days=$3,salary_structure_id=$4,work_hours_per_week=$5,document_id=$6,notes=$7,updated_at=NOW()
		WHERE id=$8 AND org_id=$9 RETURNING updated_at`,
		c.EndDate, c.ProbationEndDate, c.NoticePeriodDays, c.SalaryStructureID,
		c.WorkHoursPerWeek, c.DocumentID, c.Notes, c.ID, c.OrgID).Scan(&c.UpdatedAt)
}

func (r *repoImpl) Deactivate(ctx context.Context, orgID, employeeID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE hrm_employee_contracts SET is_active=FALSE, updated_at=NOW()
		WHERE org_id=$1 AND employee_id=$2 AND (id::text=$3 OR public_id=$3)`,
		orgID, employeeID, ref)
	if err != nil { return fmt.Errorf("contracts: Deactivate: %w", err) }
	if cmd.RowsAffected() == 0 { return ErrContractNotFound }
	return nil
}
