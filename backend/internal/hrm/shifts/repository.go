// backend/internal/hrm/shifts/repository.go
package shifts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAll(ctx context.Context, orgID string, activeOnly bool) ([]*Shift, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Shift, error)
	Create(ctx context.Context, s *Shift) error
	Update(ctx context.Context, s *Shift) error
	Delete(ctx context.Context, orgID, ref string) error
	NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error)

	FindAssignments(ctx context.Context, orgID string, assigneeType, assigneeID string) ([]*WorkScheduleAssignment, error)
	FindActiveAssignment(ctx context.Context, assigneeType, assigneeID string) (*WorkScheduleAssignment, error)
	CreateAssignment(ctx context.Context, a *WorkScheduleAssignment) error
	DeleteAssignment(ctx context.Context, orgID, ref string) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const shiftSel = `id, public_id, org_id, name, description, shift_type,
	start_time::text, end_time::text, core_start_time::text, core_end_time::text,
	weekly_hours_target, break_minutes, working_days,
	track_overtime, overtime_threshold_hours, track_breaks,
	is_default, is_active, created_by, created_at, updated_at`

func scanShift(row pgx.Row) (*Shift, error) {
	s := &Shift{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.Name, &s.Description, &s.ShiftType,
		&s.StartTime, &s.EndTime, &s.CoreStartTime, &s.CoreEndTime,
		&s.WeeklyHoursTarget, &s.BreakMinutes, &s.WorkingDays,
		&s.TrackOvertime, &s.OvertimeThresholdHours, &s.TrackBreaks,
		&s.IsDefault, &s.IsActive, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, activeOnly bool) ([]*Shift, error) {
	q := `SELECT ` + shiftSel + ` FROM hrm_shifts WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active=TRUE`
	}
	q += ` ORDER BY is_default DESC, name`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("shifts: FindAll: %w", err)
	}
	defer rows.Close()
	list := make([]*Shift, 0)
	for rows.Next() {
		s, err := scanShift(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Shift, error) {
	return scanShift(r.db.QueryRow(ctx,
		`SELECT `+shiftSel+` FROM hrm_shifts WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) Create(ctx context.Context, s *Shift) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_shifts (org_id,name,description,shift_type,start_time,end_time,
		core_start_time,core_end_time,weekly_hours_target,break_minutes,working_days,
		track_overtime,overtime_threshold_hours,track_breaks,is_default,is_active,created_by)
		VALUES ($1,$2,$3,$4,$5::time,$6::time,$7::time,$8::time,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, public_id, created_at, updated_at`,
		s.OrgID, s.Name, s.Description, s.ShiftType,
		s.StartTime, s.EndTime, s.CoreStartTime, s.CoreEndTime,
		s.WeeklyHoursTarget, s.BreakMinutes, s.WorkingDays,
		s.TrackOvertime, s.OvertimeThresholdHours, s.TrackBreaks,
		s.IsDefault, s.IsActive, s.CreatedBy,
	).Scan(&s.ID, &s.PublicID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, s *Shift) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_shifts SET name=$1,description=$2,start_time=$3::time,end_time=$4::time,
		core_start_time=$5::time,core_end_time=$6::time,weekly_hours_target=$7,
		break_minutes=$8,working_days=$9,track_overtime=$10,overtime_threshold_hours=$11,
		track_breaks=$12,is_default=$13,is_active=$14,updated_at=NOW()
		WHERE id=$15 AND org_id=$16 RETURNING updated_at`,
		s.Name, s.Description, s.StartTime, s.EndTime, s.CoreStartTime, s.CoreEndTime,
		s.WeeklyHoursTarget, s.BreakMinutes, s.WorkingDays, s.TrackOvertime,
		s.OvertimeThresholdHours, s.TrackBreaks, s.IsDefault, s.IsActive, s.ID, s.OrgID,
	).Scan(&s.UpdatedAt)
}

func (r *repoImpl) Delete(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_shifts WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil {
		return fmt.Errorf("shifts: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrShiftNotFound
	}
	return nil
}

func (r *repoImpl) NameExists(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	var e bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_shifts WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND is_active=TRUE AND id::text!=$3)`,
		orgID, name, excludeID).Scan(&e)
	return e, err
}

func (r *repoImpl) FindAssignments(ctx context.Context, orgID, assigneeType, assigneeID string) ([]*WorkScheduleAssignment, error) {
	q := `SELECT id,public_id,org_id,shift_id,assignee_type,assignee_id,
		to_char(effective_date,'YYYY-MM-DD'),to_char(end_date,'YYYY-MM-DD'),created_by,created_at
		FROM hrm_work_schedule_assignments WHERE org_id=$1`
	args := []any{orgID}
	if assigneeType != "" {
		args = append(args, assigneeType)
		q += fmt.Sprintf(` AND assignee_type=$%d`, len(args))
	}
	if assigneeID != "" {
		args = append(args, assigneeID)
		q += fmt.Sprintf(` AND assignee_id=$%d`, len(args))
	}
	q += ` ORDER BY effective_date DESC`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("shifts: FindAssignments: %w", err)
	}
	defer rows.Close()
	list := make([]*WorkScheduleAssignment, 0)
	for rows.Next() {
		a := &WorkScheduleAssignment{}
		if err := rows.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.ShiftID, &a.AssigneeType,
			&a.AssigneeID, &a.EffectiveDate, &a.EndDate, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindActiveAssignment(ctx context.Context, assigneeType, assigneeID string) (*WorkScheduleAssignment, error) {
	a := &WorkScheduleAssignment{}
	err := r.db.QueryRow(ctx,
		`SELECT id,public_id,org_id,shift_id,assignee_type,assignee_id,
		to_char(effective_date,'YYYY-MM-DD'),to_char(end_date,'YYYY-MM-DD'),created_by,created_at
		FROM hrm_work_schedule_assignments
		WHERE assignee_type=$1 AND assignee_id=$2 AND effective_date<=CURRENT_DATE
		AND (end_date IS NULL OR end_date>=CURRENT_DATE)
		ORDER BY effective_date DESC LIMIT 1`,
		assigneeType, assigneeID,
	).Scan(&a.ID, &a.PublicID, &a.OrgID, &a.ShiftID, &a.AssigneeType,
		&a.AssigneeID, &a.EffectiveDate, &a.EndDate, &a.CreatedBy, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("shifts: FindActiveAssignment: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CreateAssignment(ctx context.Context, a *WorkScheduleAssignment) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_work_schedule_assignments (org_id,shift_id,assignee_type,assignee_id,effective_date,end_date,created_by)
		VALUES ($1,$2,$3,$4,$5::date,$6::date,$7) RETURNING id, public_id, created_at`,
		a.OrgID, a.ShiftID, a.AssigneeType, a.AssigneeID, a.EffectiveDate, a.EndDate, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt)
}

func (r *repoImpl) DeleteAssignment(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_work_schedule_assignments WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil {
		return fmt.Errorf("shifts: DeleteAssignment: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}
