// backend/internal/platform/checklists/repository.go
package checklists

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for the checklist engine.
//
// TENANT ISOLATION: every method takes orgID. Template items and instance
// items have no org_id column of their own — every query reaches them by
// JOINing through their parent (template or instance) and filtering the
// parent's org_id, the hrm_approval_decisions precedent.
//
// CreateTemplateWithItems and InsertInstanceWithItems own their transaction
// inside the repo (unlike HRM's Apply()-style pgx.Tx-bypass pattern) — no
// platform service holds a *pgxpool.Pool today, and there is no cross-package
// write here needing service-level coordination.
type Repository interface {
	// Templates
	FindTemplates(ctx context.Context, orgID string, checklistType *ChecklistType) ([]*Template, error)
	FindTemplateByID(ctx context.Context, orgID, templateID string) (*Template, error)
	FindDefaultTemplate(ctx context.Context, orgID string, checklistType ChecklistType) (*Template, error)
	// CreateTemplateWithItems inserts a template and its items atomically.
	// If req.IsDefault, it clears any existing default for the same
	// org+checklist_type first, inside the same transaction — the approvals
	// engine's repository does NOT do this and a second is_default raises a
	// raw 23505; this repo must not repeat that.
	CreateTemplateWithItems(ctx context.Context, t *Template, items []*TemplateItem) error
	UpdateTemplate(ctx context.Context, t *Template) error
	// DeleteTemplate hard-deletes a template. Its items cascade; any live
	// instances keep their frozen template_name/checklist_type snapshot and
	// have template_id SET NULL — deleting a template must not kill a live
	// checklist.
	DeleteTemplate(ctx context.Context, orgID, templateID string) error
	// SetTemplateDefault clears any existing default for org+checklist_type
	// then sets templateID as the new default, atomically.
	SetTemplateDefault(ctx context.Context, orgID, templateID string, checklistType ChecklistType) error

	// Template items
	FindTemplateItems(ctx context.Context, orgID, templateID string) ([]*TemplateItem, error)
	FindActiveTemplateItemCount(ctx context.Context, orgID, templateID string) (int, error)
	FindTemplateItemByID(ctx context.Context, orgID, templateID, itemID string) (*TemplateItem, error)
	CreateTemplateItem(ctx context.Context, item *TemplateItem) error
	UpdateTemplateItem(ctx context.Context, item *TemplateItem) error
	DeleteTemplateItem(ctx context.Context, orgID, templateID, itemID string) error

	// Instances
	InsertInstanceWithItems(ctx context.Context, inst *Instance, items []*InstanceItem) error
	FindInstances(ctx context.Context, orgID string, f InstanceFilter) ([]*Instance, error)
	CountInstances(ctx context.Context, orgID string, f InstanceFilter) (int, error)
	FindInstanceByID(ctx context.Context, orgID, instanceID string) (*Instance, error)
	UpdateInstanceStatus(ctx context.Context, orgID, instanceID string, status InstanceStatus, cancelReason *string) error

	// Instance items
	FindInstanceItems(ctx context.Context, orgID, instanceID string) ([]*InstanceItem, error)
	FindInstanceItemByID(ctx context.Context, orgID, itemID string) (*InstanceItem, error)
	FindInstanceItemsByAssignee(ctx context.Context, orgID, userID string, status *ItemStatus) ([]*InstanceItem, error)
	UpdateInstanceItem(ctx context.Context, item *InstanceItem) error
	GetProgress(ctx context.Context, orgID, instanceID string) (*Progress, error)
	GetProgressBatch(ctx context.Context, orgID string, instanceIDs []string) (map[string]*Progress, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ============================================================
// Templates
// ============================================================

const templateCols = `
	id, public_id, org_id, name, description, checklist_type,
	is_default, is_active, created_by, created_at, updated_at`

func scanTemplate(row interface{ Scan(...any) error }, t *Template) error {
	return row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.Name, &t.Description, &t.ChecklistType,
		&t.IsDefault, &t.IsActive, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
}

func (r *repoImpl) FindTemplates(ctx context.Context, orgID string, checklistType *ChecklistType) ([]*Template, error) {
	q := `SELECT ` + templateCols + ` FROM platform_checklist_templates WHERE org_id = $1`
	args := []any{orgID}
	if checklistType != nil {
		q += ` AND checklist_type = $2`
		args = append(args, *checklistType)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("checklists: FindTemplates: %w", err)
	}
	defer rows.Close()

	var out []*Template
	for rows.Next() {
		t := &Template{}
		if err := scanTemplate(rows, t); err != nil {
			return nil, fmt.Errorf("checklists: FindTemplates: scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindTemplateByID(ctx context.Context, orgID, templateID string) (*Template, error) {
	q := `SELECT ` + templateCols + `
		FROM platform_checklist_templates
		WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	t := &Template{}
	err := scanTemplate(r.db.QueryRow(ctx, q, orgID, templateID), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: FindTemplateByID: %w", err)
	}
	return t, nil
}

func (r *repoImpl) FindDefaultTemplate(ctx context.Context, orgID string, checklistType ChecklistType) (*Template, error) {
	q := `SELECT ` + templateCols + `
		FROM platform_checklist_templates
		WHERE org_id = $1 AND checklist_type = $2 AND is_default = TRUE AND is_active = TRUE`
	t := &Template{}
	err := scanTemplate(r.db.QueryRow(ctx, q, orgID, checklistType), t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: FindDefaultTemplate: %w", err)
	}
	return t, nil
}

const insertTemplateSQL = `
	INSERT INTO platform_checklist_templates
	    (org_id, name, description, checklist_type, is_default, is_active, created_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7)
	RETURNING id, public_id, created_at, updated_at`

const insertTemplateItemSQL = `
	INSERT INTO platform_checklist_template_items
	    (template_id, title, description, owner_type, owner_role, owner_user_id,
	     due_offset_days, is_blocking, requires_attachment, display_order, is_active)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	RETURNING id, public_id, created_at, updated_at`

func (r *repoImpl) CreateTemplateWithItems(ctx context.Context, t *Template, items []*TemplateItem) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("checklists: CreateTemplateWithItems: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if t.IsDefault {
		if _, err := tx.Exec(ctx,
			`UPDATE platform_checklist_templates SET is_default = FALSE, updated_at = NOW()
			 WHERE org_id = $1 AND checklist_type = $2 AND is_default = TRUE`,
			t.OrgID, t.ChecklistType,
		); err != nil {
			return fmt.Errorf("checklists: CreateTemplateWithItems: clear default: %w", err)
		}
	}

	if err := tx.QueryRow(ctx, insertTemplateSQL,
		t.OrgID, t.Name, t.Description, t.ChecklistType, t.IsDefault, t.IsActive, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return fmt.Errorf("checklists: CreateTemplateWithItems: insert template: %w", err)
	}

	for _, item := range items {
		item.TemplateID = t.ID
		if err := tx.QueryRow(ctx, insertTemplateItemSQL,
			item.TemplateID, item.Title, item.Description, item.OwnerType, item.OwnerRole, item.OwnerUserID,
			item.DueOffsetDays, item.IsBlocking, item.RequiresAttachment, item.DisplayOrder, item.IsActive,
		).Scan(&item.ID, &item.PublicID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return fmt.Errorf("checklists: CreateTemplateWithItems: insert item %q: %w", item.Title, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("checklists: CreateTemplateWithItems: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateTemplate(ctx context.Context, t *Template) error {
	const q = `
		UPDATE platform_checklist_templates
		SET name = $1, description = $2, is_default = $3, is_active = $4, updated_at = NOW()
		WHERE org_id = $5 AND id = $6
		RETURNING updated_at`
	err := r.db.QueryRow(ctx, q, t.Name, t.Description, t.IsDefault, t.IsActive, t.OrgID, t.ID).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTemplateNotFound
	}
	if err != nil {
		return fmt.Errorf("checklists: UpdateTemplate: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteTemplate(ctx context.Context, orgID, templateID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM platform_checklist_templates WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`,
		orgID, templateID,
	)
	if err != nil {
		return fmt.Errorf("checklists: DeleteTemplate: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *repoImpl) SetTemplateDefault(ctx context.Context, orgID, templateID string, checklistType ChecklistType) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("checklists: SetTemplateDefault: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE platform_checklist_templates SET is_default = FALSE, updated_at = NOW()
		 WHERE org_id = $1 AND checklist_type = $2 AND is_default = TRUE`,
		orgID, checklistType,
	); err != nil {
		return fmt.Errorf("checklists: SetTemplateDefault: clear: %w", err)
	}

	cmd, err := tx.Exec(ctx,
		`UPDATE platform_checklist_templates SET is_default = TRUE, updated_at = NOW() WHERE org_id = $1 AND id = $2`,
		orgID, templateID,
	)
	if err != nil {
		return fmt.Errorf("checklists: SetTemplateDefault: set: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("checklists: SetTemplateDefault: commit: %w", err)
	}
	return nil
}

// ============================================================
// Template items
// ============================================================

// templateItemCols is pre-qualified with the "ti" alias — both call sites
// JOIN platform_checklist_template_items to platform_checklist_templates (to
// filter by the parent's org_id), and both tables have id/public_id/
// created_at/updated_at columns, so an unqualified SELECT list is ambiguous.
const templateItemCols = `
	ti.id, ti.public_id, ti.template_id, ti.title, ti.description, ti.owner_type, ti.owner_role, ti.owner_user_id,
	ti.due_offset_days, ti.is_blocking, ti.requires_attachment, ti.display_order, ti.is_active, ti.created_at, ti.updated_at`

func scanTemplateItem(row interface{ Scan(...any) error }, it *TemplateItem) error {
	return row.Scan(
		&it.ID, &it.PublicID, &it.TemplateID, &it.Title, &it.Description, &it.OwnerType, &it.OwnerRole, &it.OwnerUserID,
		&it.DueOffsetDays, &it.IsBlocking, &it.RequiresAttachment, &it.DisplayOrder, &it.IsActive, &it.CreatedAt, &it.UpdatedAt,
	)
}

func (r *repoImpl) FindTemplateItems(ctx context.Context, orgID, templateID string) ([]*TemplateItem, error) {
	q := `SELECT ` + templateItemCols + `
		FROM platform_checklist_template_items ti
		JOIN platform_checklist_templates t ON t.id = ti.template_id
		WHERE t.org_id = $1 AND (t.id::text = $2 OR t.public_id = $2)
		ORDER BY ti.display_order, ti.created_at`

	rows, err := r.db.Query(ctx, q, orgID, templateID)
	if err != nil {
		return nil, fmt.Errorf("checklists: FindTemplateItems: %w", err)
	}
	defer rows.Close()

	var out []*TemplateItem
	for rows.Next() {
		it := &TemplateItem{}
		if err := scanTemplateItem(rows, it); err != nil {
			return nil, fmt.Errorf("checklists: FindTemplateItems: scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindActiveTemplateItemCount(ctx context.Context, orgID, templateID string) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM platform_checklist_template_items ti
		JOIN platform_checklist_templates t ON t.id = ti.template_id
		WHERE t.org_id = $1 AND (t.id::text = $2 OR t.public_id = $2) AND ti.is_active = TRUE`
	var n int
	if err := r.db.QueryRow(ctx, q, orgID, templateID).Scan(&n); err != nil {
		return 0, fmt.Errorf("checklists: FindActiveTemplateItemCount: %w", err)
	}
	return n, nil
}

func (r *repoImpl) FindTemplateItemByID(ctx context.Context, orgID, templateID, itemID string) (*TemplateItem, error) {
	q := `SELECT ` + templateItemCols + `
		FROM platform_checklist_template_items ti
		JOIN platform_checklist_templates t ON t.id = ti.template_id
		WHERE t.org_id = $1 AND (t.id::text = $2 OR t.public_id = $2) AND (ti.id::text = $3 OR ti.public_id = $3)`
	it := &TemplateItem{}
	err := scanTemplateItem(r.db.QueryRow(ctx, q, orgID, templateID, itemID), it)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: FindTemplateItemByID: %w", err)
	}
	return it, nil
}

func (r *repoImpl) CreateTemplateItem(ctx context.Context, item *TemplateItem) error {
	return r.db.QueryRow(ctx, insertTemplateItemSQL,
		item.TemplateID, item.Title, item.Description, item.OwnerType, item.OwnerRole, item.OwnerUserID,
		item.DueOffsetDays, item.IsBlocking, item.RequiresAttachment, item.DisplayOrder, item.IsActive,
	).Scan(&item.ID, &item.PublicID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *repoImpl) UpdateTemplateItem(ctx context.Context, item *TemplateItem) error {
	const q = `
		UPDATE platform_checklist_template_items
		SET title = $1, description = $2, owner_type = $3, owner_role = $4, owner_user_id = $5,
		    due_offset_days = $6, is_blocking = $7, requires_attachment = $8, display_order = $9,
		    is_active = $10, updated_at = NOW()
		WHERE id = $11
		RETURNING updated_at`
	err := r.db.QueryRow(ctx, q,
		item.Title, item.Description, item.OwnerType, item.OwnerRole, item.OwnerUserID,
		item.DueOffsetDays, item.IsBlocking, item.RequiresAttachment, item.DisplayOrder,
		item.IsActive, item.ID,
	).Scan(&item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTemplateItemNotFound
	}
	if err != nil {
		return fmt.Errorf("checklists: UpdateTemplateItem: %w", err)
	}
	return nil
}

func (r *repoImpl) DeleteTemplateItem(ctx context.Context, orgID, templateID, itemID string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM platform_checklist_template_items ti
		 USING platform_checklist_templates t
		 WHERE ti.template_id = t.id
		   AND t.org_id = $1 AND (t.id::text = $2 OR t.public_id = $2)
		   AND (ti.id::text = $3 OR ti.public_id = $3)`,
		orgID, templateID, itemID,
	)
	if err != nil {
		return fmt.Errorf("checklists: DeleteTemplateItem: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrTemplateItemNotFound
	}
	return nil
}

// ============================================================
// Instances
// ============================================================

const instanceCols = `
	id, public_id, org_id, template_id, template_name, checklist_type,
	subject_type, subject_id, subject_label, subject_user_id, anchor_date,
	status, completed_at, cancelled_at, cancel_reason, created_by, created_at, updated_at`

func scanInstance(row interface{ Scan(...any) error }, inst *Instance) error {
	return row.Scan(
		&inst.ID, &inst.PublicID, &inst.OrgID, &inst.TemplateID, &inst.TemplateName, &inst.ChecklistType,
		&inst.SubjectType, &inst.SubjectID, &inst.SubjectLabel, &inst.SubjectUserID, &inst.AnchorDate,
		&inst.Status, &inst.CompletedAt, &inst.CancelledAt, &inst.CancelReason, &inst.CreatedBy, &inst.CreatedAt, &inst.UpdatedAt,
	)
}

const insertInstanceSQL = `
	INSERT INTO platform_checklist_instances
	    (org_id, template_id, template_name, checklist_type, subject_type, subject_id,
	     subject_label, subject_user_id, anchor_date, status, created_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	RETURNING id, public_id, created_at, updated_at`

const insertInstanceItemSQL = `
	INSERT INTO platform_checklist_instance_items
	    (instance_id, template_item_id, title, description, owner_type, owner_role,
	     is_blocking, requires_attachment, display_order, due_offset_days, assignee_user_id, due_date, status)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	RETURNING id, public_id, created_at, updated_at`

// InsertInstanceWithItems inserts an instance and all of its items in one
// transaction. If any item insert fails, the whole instance is rolled back —
// a checklist that partially exists is worse than one that doesn't exist yet
// (the caller's InstantiateDefault/Instantiate is the retry path).
func (r *repoImpl) InsertInstanceWithItems(ctx context.Context, inst *Instance, items []*InstanceItem) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("checklists: InsertInstanceWithItems: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, insertInstanceSQL,
		inst.OrgID, inst.TemplateID, inst.TemplateName, inst.ChecklistType, inst.SubjectType, inst.SubjectID,
		inst.SubjectLabel, inst.SubjectUserID, inst.AnchorDate, inst.Status, inst.CreatedBy,
	).Scan(&inst.ID, &inst.PublicID, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return fmt.Errorf("checklists: InsertInstanceWithItems: insert instance: %w", err)
	}

	for _, item := range items {
		item.InstanceID = inst.ID
		if err := tx.QueryRow(ctx, insertInstanceItemSQL,
			item.InstanceID, item.TemplateItemID, item.Title, item.Description, item.OwnerType, item.OwnerRole,
			item.IsBlocking, item.RequiresAttachment, item.DisplayOrder, item.DueOffsetDays,
			item.AssigneeUserID, item.DueDate, item.Status,
		).Scan(&item.ID, &item.PublicID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return fmt.Errorf("checklists: InsertInstanceWithItems: insert item %q: %w", item.Title, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("checklists: InsertInstanceWithItems: commit: %w", err)
	}
	return nil
}

func (r *repoImpl) FindInstances(ctx context.Context, orgID string, f InstanceFilter) ([]*Instance, error) {
	q := `SELECT ` + instanceCols + ` FROM platform_checklist_instances WHERE org_id = $1`
	args := []any{orgID}
	q, args = appendInstanceFilter(q, args, f)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("checklists: FindInstances: %w", err)
	}
	defer rows.Close()

	var out []*Instance
	for rows.Next() {
		inst := &Instance{}
		if err := scanInstance(rows, inst); err != nil {
			return nil, fmt.Errorf("checklists: FindInstances: scan: %w", err)
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (r *repoImpl) CountInstances(ctx context.Context, orgID string, f InstanceFilter) (int, error) {
	q := `SELECT COUNT(*) FROM platform_checklist_instances WHERE org_id = $1`
	args := []any{orgID}
	q, args = appendInstanceFilter(q, args, f)

	var n int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("checklists: CountInstances: %w", err)
	}
	return n, nil
}

// appendInstanceFilter appends null-guarded predicates for InstanceFilter.
// Plain fixed-SQL filter structs, not a buildXWhere helper — platform has
// zero buildXWhere usages today. This shape can defeat index selection at
// scale; at tens of instances per org that's a non-issue for a first version.
func appendInstanceFilter(q string, args []any, f InstanceFilter) (string, []any) {
	if f.SubjectType != nil {
		args = append(args, *f.SubjectType)
		q += fmt.Sprintf(` AND subject_type = $%d`, len(args))
	}
	if f.SubjectID != nil {
		args = append(args, *f.SubjectID)
		q += fmt.Sprintf(` AND subject_id::text = $%d`, len(args))
	}
	if f.Status != nil {
		args = append(args, *f.Status)
		q += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	return q, args
}

func (r *repoImpl) FindInstanceByID(ctx context.Context, orgID, instanceID string) (*Instance, error) {
	q := `SELECT ` + instanceCols + `
		FROM platform_checklist_instances
		WHERE org_id = $1 AND (id::text = $2 OR public_id = $2)`
	inst := &Instance{}
	err := scanInstance(r.db.QueryRow(ctx, q, orgID, instanceID), inst)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: FindInstanceByID: %w", err)
	}
	return inst, nil
}

func (r *repoImpl) UpdateInstanceStatus(ctx context.Context, orgID, instanceID string, status InstanceStatus, cancelReason *string) error {
	const q = `
		UPDATE platform_checklist_instances
		SET status = $1,
		    completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE completed_at END,
		    cancelled_at = CASE WHEN $1 = 'cancelled' THEN NOW() ELSE cancelled_at END,
		    cancel_reason = CASE WHEN $1 = 'cancelled' THEN $2 ELSE cancel_reason END,
		    updated_at = NOW()
		WHERE org_id = $3 AND id = $4`
	cmd, err := r.db.Exec(ctx, q, status, cancelReason, orgID, instanceID)
	if err != nil {
		return fmt.Errorf("checklists: UpdateInstanceStatus: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

// ============================================================
// Instance items
// ============================================================

// instanceItemCols is pre-qualified with the "ii" alias — every call site
// reaches platform_checklist_instance_items via a JOIN through its parent
// instance (to filter by org_id, which this table has no column for), so
// there is no unaliased use of this column list.
const instanceItemCols = `
	ii.id, ii.public_id, ii.instance_id, ii.template_item_id, ii.title, ii.description, ii.owner_type, ii.owner_role,
	ii.is_blocking, ii.requires_attachment, ii.display_order, ii.due_offset_days, ii.assignee_user_id, ii.due_date,
	ii.status, ii.completed_by, ii.completed_at, ii.completion_note, ii.attachment_url, ii.skip_reason, ii.created_at, ii.updated_at`

func scanInstanceItem(row interface{ Scan(...any) error }, it *InstanceItem) error {
	return row.Scan(
		&it.ID, &it.PublicID, &it.InstanceID, &it.TemplateItemID, &it.Title, &it.Description, &it.OwnerType, &it.OwnerRole,
		&it.IsBlocking, &it.RequiresAttachment, &it.DisplayOrder, &it.DueOffsetDays, &it.AssigneeUserID, &it.DueDate,
		&it.Status, &it.CompletedBy, &it.CompletedAt, &it.CompletionNote, &it.AttachmentURL, &it.SkipReason, &it.CreatedAt, &it.UpdatedAt,
	)
}

func (r *repoImpl) FindInstanceItems(ctx context.Context, orgID, instanceID string) ([]*InstanceItem, error) {
	q := `SELECT ` + instanceItemCols + `
		FROM platform_checklist_instance_items ii
		JOIN platform_checklist_instances i ON i.id = ii.instance_id
		WHERE i.org_id = $1 AND (i.id::text = $2 OR i.public_id = $2)
		ORDER BY ii.display_order, ii.created_at`

	rows, err := r.db.Query(ctx, q, orgID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("checklists: FindInstanceItems: %w", err)
	}
	defer rows.Close()

	var out []*InstanceItem
	for rows.Next() {
		it := &InstanceItem{}
		if err := scanInstanceItem(rows, it); err != nil {
			return nil, fmt.Errorf("checklists: FindInstanceItems: scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindInstanceItemByID(ctx context.Context, orgID, itemID string) (*InstanceItem, error) {
	q := `SELECT ` + instanceItemCols + `
		FROM platform_checklist_instance_items ii
		JOIN platform_checklist_instances i ON i.id = ii.instance_id
		WHERE i.org_id = $1 AND (ii.id::text = $2 OR ii.public_id = $2)`
	it := &InstanceItem{}
	err := scanInstanceItem(r.db.QueryRow(ctx, q, orgID, itemID), it)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: FindInstanceItemByID: %w", err)
	}
	return it, nil
}

func (r *repoImpl) FindInstanceItemsByAssignee(ctx context.Context, orgID, userID string, status *ItemStatus) ([]*InstanceItem, error) {
	q := `SELECT ` + instanceItemCols + `
		FROM platform_checklist_instance_items ii
		JOIN platform_checklist_instances i ON i.id = ii.instance_id
		WHERE i.org_id = $1 AND ii.assignee_user_id = $2`
	args := []any{orgID, userID}
	if status != nil {
		args = append(args, *status)
		q += fmt.Sprintf(` AND ii.status = $%d`, len(args))
	}
	q += ` ORDER BY ii.due_date NULLS LAST, ii.created_at`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("checklists: FindInstanceItemsByAssignee: %w", err)
	}
	defer rows.Close()

	var out []*InstanceItem
	for rows.Next() {
		it := &InstanceItem{}
		if err := scanInstanceItem(rows, it); err != nil {
			return nil, fmt.Errorf("checklists: FindInstanceItemsByAssignee: scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *repoImpl) UpdateInstanceItem(ctx context.Context, item *InstanceItem) error {
	const q = `
		UPDATE platform_checklist_instance_items
		SET status = $1, completed_by = $2, completed_at = $3, completion_note = $4,
		    attachment_url = $5, skip_reason = $6, updated_at = NOW()
		WHERE id = $7
		RETURNING updated_at`
	err := r.db.QueryRow(ctx, q,
		item.Status, item.CompletedBy, item.CompletedAt, item.CompletionNote,
		item.AttachmentURL, item.SkipReason, item.ID,
	).Scan(&item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstanceItemNotFound
	}
	if err != nil {
		return fmt.Errorf("checklists: UpdateInstanceItem: %w", err)
	}
	return nil
}

const progressSelect = `
	SELECT
	    i.id,
	    COUNT(ii.id) AS total,
	    COUNT(ii.id) FILTER (WHERE ii.status = 'completed') AS completed,
	    COUNT(ii.id) FILTER (WHERE ii.status = 'skipped') AS skipped,
	    COUNT(ii.id) FILTER (WHERE ii.status = 'pending') AS pending,
	    COUNT(ii.id) FILTER (WHERE ii.status = 'pending' AND ii.is_blocking) AS blocking_open
	FROM platform_checklist_instances i
	LEFT JOIN platform_checklist_instance_items ii ON ii.instance_id = i.id
	WHERE i.org_id = $1`

func scanProgress(row interface{ Scan(...any) error }) (*Progress, error) {
	p := &Progress{}
	var instID string
	if err := row.Scan(&instID, &p.TotalItems, &p.CompletedItems, &p.SkippedItems, &p.PendingItems, &p.BlockingOpen); err != nil {
		return nil, err
	}
	p.InstanceID = instID
	if p.TotalItems > 0 {
		p.PercentDone = ((p.CompletedItems + p.SkippedItems) * 100) / p.TotalItems
	}
	return p, nil
}

func (r *repoImpl) GetProgress(ctx context.Context, orgID, instanceID string) (*Progress, error) {
	q := progressSelect + ` AND (i.id::text = $2 OR i.public_id = $2) GROUP BY i.id`
	p, err := scanProgress(r.db.QueryRow(ctx, q, orgID, instanceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checklists: GetProgress: %w", err)
	}
	return p, nil
}

// GetProgressBatch computes progress for many instances in one query — used
// by the list endpoint to avoid N+1.
func (r *repoImpl) GetProgressBatch(ctx context.Context, orgID string, instanceIDs []string) (map[string]*Progress, error) {
	out := map[string]*Progress{}
	if len(instanceIDs) == 0 {
		return out, nil
	}
	q := progressSelect + ` AND i.id::text = ANY($2::text[]) GROUP BY i.id`
	rows, err := r.db.Query(ctx, q, orgID, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("checklists: GetProgressBatch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		p, err := scanProgress(rows)
		if err != nil {
			return nil, fmt.Errorf("checklists: GetProgressBatch: scan: %w", err)
		}
		out[p.InstanceID] = p
	}
	return out, rows.Err()
}
