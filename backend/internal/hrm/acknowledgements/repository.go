// backend/internal/hrm/acknowledgements/repository.go
package acknowledgements

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID, employeeID, ackType, status string) ([]*Acknowledgement, error)
	FindByEntity(ctx context.Context, orgID, ackType, ackID string) ([]*Acknowledgement, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Acknowledgement, error)
	Create(ctx context.Context, a *Acknowledgement) error
	UpdateStatus(ctx context.Context, id string, status AckStatus) error
	Acknowledge(ctx context.Context, id string, notes, signature *string) error
	Decline(ctx context.Context, id, reason string) error
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id,
	acknowledgeable_type, acknowledgeable_id::text, entity_title,
	notes, signature_required, signed_at, signature_data,
	status, acknowledged_at, declined_at, decline_reason,
	to_char(expires_at,'YYYY-MM-DD'),
	requested_by, requested_at, reminder_sent_at,
	created_at, updated_at`

func scanAck(row pgx.Row) (*Acknowledgement, error) {
	a := &Acknowledgement{}
	err := row.Scan(
		&a.ID, &a.PublicID, &a.OrgID, &a.EmployeeID,
		&a.AcknowledgeableType, &a.AcknowledgeableID, &a.EntityTitle,
		&a.Notes, &a.SignatureRequired, &a.SignedAt, &a.SignatureData,
		&a.Status, &a.AcknowledgedAt, &a.DeclinedAt, &a.DeclineReason,
		&a.ExpiresAt,
		&a.RequestedBy, &a.RequestedAt, &a.ReminderSentAt,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return a, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, employeeID, ackType, status string) ([]*Acknowledgement, error) {
	q := `SELECT ` + sel + ` FROM hrm_acknowledgements WHERE org_id=$1`
	args := []any{orgID}
	if employeeID != "" { args = append(args, employeeID); q += fmt.Sprintf(` AND employee_id=$%d`, len(args)) }
	if ackType != "" { args = append(args, ackType); q += fmt.Sprintf(` AND acknowledgeable_type=$%d`, len(args)) }
	if status != "" { args = append(args, status); q += fmt.Sprintf(` AND status=$%d`, len(args)) }
	q += ` ORDER BY requested_at DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("acknowledgements: FindAll: %w", err) }
	defer rows.Close()
	list := make([]*Acknowledgement, 0)
	for rows.Next() { a, err := scanAck(rows); if err != nil { return nil, err }; list = append(list, a) }
	return list, rows.Err()
}

func (r *repoImpl) FindByEntity(ctx context.Context, orgID, ackType, ackID string) ([]*Acknowledgement, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+sel+` FROM hrm_acknowledgements WHERE org_id=$1 AND acknowledgeable_type=$2 AND acknowledgeable_id=$3::uuid ORDER BY requested_at DESC`,
		orgID, ackType, ackID)
	if err != nil { return nil, fmt.Errorf("acknowledgements: FindByEntity: %w", err) }
	defer rows.Close()
	list := make([]*Acknowledgement, 0)
	for rows.Next() { a, err := scanAck(rows); if err != nil { return nil, err }; list = append(list, a) }
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Acknowledgement, error) {
	return scanAck(r.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_acknowledgements WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) Create(ctx context.Context, a *Acknowledgement) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_acknowledgements
		(org_id, employee_id, acknowledgeable_type, acknowledgeable_id,
		 entity_title, signature_required, expires_at, status, requested_by)
		VALUES ($1,$2,$3,$4::uuid,$5,$6,$7::date,$8,$9)
		RETURNING id, public_id, requested_at, created_at, updated_at`,
		a.OrgID, a.EmployeeID, a.AcknowledgeableType, a.AcknowledgeableID,
		a.EntityTitle, a.SignatureRequired, a.ExpiresAt, a.Status, a.RequestedBy,
	).Scan(&a.ID, &a.PublicID, &a.RequestedAt, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status AckStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_acknowledgements SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *repoImpl) Acknowledge(ctx context.Context, id string, notes, signature *string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_acknowledgements SET status='acknowledged', acknowledged_at=NOW(),
		notes=$1, signature_data=$2, signed_at=CASE WHEN $2 IS NOT NULL THEN NOW() END, updated_at=NOW()
		WHERE id=$3`,
		notes, signature, id)
	return err
}

func (r *repoImpl) Decline(ctx context.Context, id, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_acknowledgements SET status='declined', declined_at=NOW(), decline_reason=$1, updated_at=NOW() WHERE id=$2`,
		reason, id)
	return err
}
