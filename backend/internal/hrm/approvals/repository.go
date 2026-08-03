// backend/internal/hrm/approvals/repository.go
package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for the approval engine.
type Repository interface {
	// Templates
	FindAllTemplates(ctx context.Context, orgID string, actionType string) ([]*ApprovalTemplate, error)
	FindTemplateByRef(ctx context.Context, orgID, ref string) (*ApprovalTemplate, error)
	FindDefaultTemplate(ctx context.Context, orgID string, actionType ActionType) (*ApprovalTemplate, error)
	CreateTemplate(ctx context.Context, t *ApprovalTemplate, levels []*ApprovalTemplateLevel) error
	UpdateTemplate(ctx context.Context, t *ApprovalTemplate) error
	DeleteTemplate(ctx context.Context, orgID, ref string) error
	FindTemplateLevels(ctx context.Context, templateID string) ([]*ApprovalTemplateLevel, error)

	// Instances
	FindAllInstances(ctx context.Context, orgID string, limit, offset int, status string, requesterID string) ([]*ApprovalInstance, int, error)
	FindInstanceByRef(ctx context.Context, orgID, ref string) (*ApprovalInstance, error)
	FindInstanceByEntity(ctx context.Context, entityType, entityID string) (*ApprovalInstance, error)
	CreateInstance(ctx context.Context, inst *ApprovalInstance) error
	UpdateInstance(ctx context.Context, inst *ApprovalInstance) error
	CreateDecision(ctx context.Context, d *ApprovalDecision) error
	FindDecisions(ctx context.Context, instanceID string) ([]*ApprovalDecision, error)
}

type repoImpl struct { db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const tmplSelect = `id, public_id, org_id, name, description, action_type, condition_expression, is_default, is_active, created_by, created_at, updated_at`

func scanTemplate(row pgx.Row) (*ApprovalTemplate, error) {
	t := &ApprovalTemplate{}
	err := row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.Name, &t.Description, &t.ActionType,
		&t.ConditionExpression, &t.IsDefault, &t.IsActive, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return t, nil
}

func (r *repoImpl) FindAllTemplates(ctx context.Context, orgID, actionType string) ([]*ApprovalTemplate, error) {
	q := `SELECT ` + tmplSelect + ` FROM hrm_approval_templates WHERE org_id=$1 AND is_active=TRUE`
	args := []any{orgID}
	if actionType != "" {
		args = append(args, actionType)
		q += fmt.Sprintf(` AND action_type=$%d`, len(args))
	}
	q += ` ORDER BY action_type, name`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("approvals: FindAllTemplates: %w", err) }
	defer rows.Close()
	list := make([]*ApprovalTemplate, 0)
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil { return nil, err }
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindTemplateByRef(ctx context.Context, orgID, ref string) (*ApprovalTemplate, error) {
	q := `SELECT ` + tmplSelect + ` FROM hrm_approval_templates WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	return scanTemplate(r.db.QueryRow(ctx, q, orgID, ref))
}

func (r *repoImpl) FindDefaultTemplate(ctx context.Context, orgID string, actionType ActionType) (*ApprovalTemplate, error) {
	q := `SELECT ` + tmplSelect + ` FROM hrm_approval_templates WHERE org_id=$1 AND action_type=$2 AND is_default=TRUE AND is_active=TRUE LIMIT 1`
	return scanTemplate(r.db.QueryRow(ctx, q, orgID, actionType))
}

func (r *repoImpl) CreateTemplate(ctx context.Context, t *ApprovalTemplate, levels []*ApprovalTemplateLevel) error {
	tx, err := r.db.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO hrm_approval_templates (org_id,name,description,action_type,condition_expression,is_default,is_active,created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
		t.OrgID, t.Name, t.Description, t.ActionType, t.ConditionExpression, t.IsDefault, true, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil { return fmt.Errorf("approvals: CreateTemplate: %w", err) }

	for _, lv := range levels {
		lv.TemplateID = t.ID
		if _, err := tx.Exec(ctx,
			`INSERT INTO hrm_approval_template_levels (template_id,level,approver_type,approver_role,approver_user_id,sla_hours,on_sla_breach)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			lv.TemplateID, lv.Level, lv.ApproverType, lv.ApproverRole, lv.ApproverUserID, lv.SLAHours, lv.OnSLABreach,
		); err != nil { return fmt.Errorf("approvals: CreateTemplate level: %w", err) }
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) UpdateTemplate(ctx context.Context, t *ApprovalTemplate) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_approval_templates SET name=$1, description=$2, condition_expression=$3, is_default=$4, is_active=$5, updated_at=NOW()
		WHERE id=$6 AND org_id=$7 RETURNING updated_at`,
		t.Name, t.Description, t.ConditionExpression, t.IsDefault, t.IsActive, t.ID, t.OrgID,
	).Scan(&t.UpdatedAt)
}

func (r *repoImpl) DeleteTemplate(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_approval_templates WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil { return fmt.Errorf("approvals: DeleteTemplate: %w", err) }
	if cmd.RowsAffected() == 0 { return ErrTemplateNotFound }
	return nil
}

func (r *repoImpl) FindTemplateLevels(ctx context.Context, templateID string) ([]*ApprovalTemplateLevel, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, template_id, level, approver_type, approver_role, approver_user_id, sla_hours, on_sla_breach
		FROM hrm_approval_template_levels WHERE template_id=$1 ORDER BY level`,
		templateID)
	if err != nil { return nil, fmt.Errorf("approvals: FindTemplateLevels: %w", err) }
	defer rows.Close()
	list := make([]*ApprovalTemplateLevel, 0)
	for rows.Next() {
		lv := &ApprovalTemplateLevel{}
		if err := rows.Scan(&lv.ID, &lv.TemplateID, &lv.Level, &lv.ApproverType, &lv.ApproverRole, &lv.ApproverUserID, &lv.SLAHours, &lv.OnSLABreach); err != nil {
			return nil, err
		}
		list = append(list, lv)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindAllInstances(ctx context.Context, orgID string, limit, offset int, status string, requesterID string) ([]*ApprovalInstance, int, error) {
	where := "WHERE org_id=$1"
	args := []any{orgID}
	
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND overall_status=$%d", len(args))
	}
	if requesterID != "" {
		args = append(args, requesterID)
		where += fmt.Sprintf(" AND requested_by=$%d", len(args))
	}
	
	var total int
	err := r.db.QueryRow(ctx, "SELECT count(*) FROM hrm_approval_instances "+where, args...).Scan(&total)
	if err != nil { return nil, 0, err }
	
	q := `SELECT id, public_id, org_id, template_id, entity_type, entity_id, instance_snapshot,
		current_level, overall_status, requested_by, created_at, updated_at, completed_at
		FROM hrm_approval_instances ` + where + ` ORDER BY created_at DESC LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)
	
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()
	
	list := make([]*ApprovalInstance, 0)
	for rows.Next() {
		inst := &ApprovalInstance{}
		err := rows.Scan(&inst.ID, &inst.PublicID, &inst.OrgID, &inst.TemplateID, &inst.EntityType, &inst.EntityID,
			&inst.InstanceSnapshot, &inst.CurrentLevel, &inst.OverallStatus, &inst.RequestedBy,
			&inst.CreatedAt, &inst.UpdatedAt, &inst.CompletedAt)
		if err != nil { return nil, 0, err }
		_ = inst.ParseSnapshot()
		list = append(list, inst)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindInstanceByRef(ctx context.Context, orgID, ref string) (*ApprovalInstance, error) {
	inst := &ApprovalInstance{}
	err := r.db.QueryRow(ctx,
		`SELECT id, public_id, org_id, template_id, entity_type, entity_id, instance_snapshot,
		current_level, overall_status, requested_by, created_at, updated_at, completed_at
		FROM hrm_approval_instances WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref,
	).Scan(&inst.ID, &inst.PublicID, &inst.OrgID, &inst.TemplateID, &inst.EntityType, &inst.EntityID,
		&inst.InstanceSnapshot, &inst.CurrentLevel, &inst.OverallStatus, &inst.RequestedBy,
		&inst.CreatedAt, &inst.UpdatedAt, &inst.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, fmt.Errorf("approvals: FindInstanceByRef: %w", err) }
	_ = inst.ParseSnapshot()
	return inst, nil
}

func (r *repoImpl) FindInstanceByEntity(ctx context.Context, entityType, entityID string) (*ApprovalInstance, error) {
	inst := &ApprovalInstance{}
	err := r.db.QueryRow(ctx,
		`SELECT id, public_id, org_id, template_id, entity_type, entity_id, instance_snapshot,
		current_level, overall_status, requested_by, created_at, updated_at, completed_at
		FROM hrm_approval_instances WHERE entity_type=$1 AND entity_id=$2 AND overall_status='pending'`,
		entityType, entityID,
	).Scan(&inst.ID, &inst.PublicID, &inst.OrgID, &inst.TemplateID, &inst.EntityType, &inst.EntityID,
		&inst.InstanceSnapshot, &inst.CurrentLevel, &inst.OverallStatus, &inst.RequestedBy,
		&inst.CreatedAt, &inst.UpdatedAt, &inst.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, fmt.Errorf("approvals: FindInstanceByEntity: %w", err) }
	_ = inst.ParseSnapshot()
	return inst, nil
}

func (r *repoImpl) CreateInstance(ctx context.Context, inst *ApprovalInstance) error {
	snapshotJSON, err := json.Marshal(inst.Snapshot)
	if err != nil { return fmt.Errorf("approvals: CreateInstance marshal: %w", err) }
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_approval_instances (org_id, template_id, entity_type, entity_id, instance_snapshot, current_level, overall_status, requested_by)
		VALUES ($1,$2,$3,$4,$5,$6,'pending',$7) RETURNING id, public_id, created_at, updated_at`,
		inst.OrgID, inst.TemplateID, inst.EntityType, inst.EntityID, snapshotJSON, inst.CurrentLevel, inst.RequestedBy,
	).Scan(&inst.ID, &inst.PublicID, &inst.CreatedAt, &inst.UpdatedAt)
}

func (r *repoImpl) UpdateInstance(ctx context.Context, inst *ApprovalInstance) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_approval_instances SET current_level=$1, overall_status=$2, completed_at=$3, updated_at=NOW()
		WHERE id=$4 RETURNING updated_at`,
		inst.CurrentLevel, inst.OverallStatus, inst.CompletedAt, inst.ID,
	).Scan(&inst.UpdatedAt)
}

func (r *repoImpl) CreateDecision(ctx context.Context, d *ApprovalDecision) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_approval_decisions (instance_id, level, approver_id, action, note)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, decided_at`,
		d.InstanceID, d.Level, d.ApproverID, d.Action, d.Note,
	).Scan(&d.ID, &d.DecidedAt)
}

func (r *repoImpl) FindDecisions(ctx context.Context, instanceID string) ([]*ApprovalDecision, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, instance_id, level, approver_id, action, note, decided_at
		FROM hrm_approval_decisions WHERE instance_id=$1 ORDER BY level`,
		instanceID)
	if err != nil { return nil, err }
	defer rows.Close()
	list := make([]*ApprovalDecision, 0)
	for rows.Next() {
		d := &ApprovalDecision{}
		if err := rows.Scan(&d.ID, &d.InstanceID, &d.Level, &d.ApproverID, &d.Action, &d.Note, &d.DecidedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}
