// backend/internal/hrm/promotions/repository.go
package promotions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for employee promotions.
// TENANT ISOLATION: every query includes org_id.
type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Promotion, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error)
	Create(ctx context.Context, p *Promotion) error
	Update(ctx context.Context, p *Promotion) error
	UpdateStatus(ctx context.Context, id string, status PromotionStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id,
	from_position_id, from_department_id, from_salary_structure_id, from_basic_pay,
	to_position_id, to_department_id, to_salary_structure_id, new_basic_pay,
	to_char(effective_date,'YYYY-MM-DD'), reason, notes,
	approval_instance_id, document_id, status,
	applied_at, applied_by, created_by, created_at, updated_at`

func scanPromo(row pgx.Row) (*Promotion, error) {
	p := &Promotion{}
	err := row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.EmployeeID,
		&p.FromPositionID, &p.FromDepartmentID, &p.FromSalaryStructureID, &p.FromBasicPay,
		&p.ToPositionID, &p.ToDepartmentID, &p.ToSalaryStructureID, &p.NewBasicPay,
		&p.EffectiveDate, &p.Reason, &p.Notes,
		&p.ApprovalInstanceID, &p.DocumentID, &p.Status,
		&p.AppliedAt, &p.AppliedBy, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return p, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, status string) ([]*Promotion, error) {
	q := `SELECT ` + sel + ` FROM hrm_promotions WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("promotions: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Promotion, 0)
	for rows.Next() {
		p, err := scanPromo(rows)
		if err != nil { return nil, err }
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Promotion, error) {
	q := `SELECT ` + sel + ` FROM hrm_promotions WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	return scanPromo(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) Create(ctx context.Context, p *Promotion) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_promotions
		(org_id, employee_id,
		 from_position_id, from_department_id, from_salary_structure_id, from_basic_pay,
		 to_position_id, to_department_id, to_salary_structure_id, new_basic_pay,
		 effective_date, reason, notes, approval_instance_id, document_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::date,$12,$13,$14,$15,$16,$17)
		RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.EmployeeID,
		p.FromPositionID, p.FromDepartmentID, p.FromSalaryStructureID, p.FromBasicPay,
		p.ToPositionID, p.ToDepartmentID, p.ToSalaryStructureID, p.NewBasicPay,
		p.EffectiveDate, p.Reason, p.Notes, p.ApprovalInstanceID, p.DocumentID, p.Status, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, p *Promotion) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_promotions SET
		to_position_id=$1, to_department_id=$2, to_salary_structure_id=$3, new_basic_pay=$4,
		effective_date=$5::date, reason=$6, notes=$7, document_id=$8, updated_at=NOW()
		WHERE id=$9 AND org_id=$10 RETURNING updated_at`,
		p.ToPositionID, p.ToDepartmentID, p.ToSalaryStructureID, p.NewBasicPay,
		p.EffectiveDate, p.Reason, p.Notes, p.DocumentID, p.ID, p.OrgID,
	).Scan(&p.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status PromotionStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_promotions SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}
