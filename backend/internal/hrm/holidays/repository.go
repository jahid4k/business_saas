// backend/internal/hrm/holidays/repository.go
package holidays

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	FindAllCalendars(ctx context.Context, orgID string, activeOnly bool) ([]*HolidayCalendar, error)
	FindCalendarByRef(ctx context.Context, orgID, ref string) (*HolidayCalendar, error)
	CreateCalendar(ctx context.Context, c *HolidayCalendar) error
	UpdateCalendar(ctx context.Context, c *HolidayCalendar) error
	DeleteCalendar(ctx context.Context, orgID, ref string) error
	CalendarNameExists(ctx context.Context, orgID string, name string, year int, excludeID string) (bool, error)

	FindHolidays(ctx context.Context, calendarID string) ([]*Holiday, error)
	FindHolidayByRef(ctx context.Context, calendarID, ref string) (*Holiday, error)
	CreateHoliday(ctx context.Context, h *Holiday) error
	UpdateHoliday(ctx context.Context, h *Holiday) error
	DeleteHoliday(ctx context.Context, calendarID, ref string) error

	FindAssignment(ctx context.Context, assigneeType, assigneeID string) (*CalendarAssignment, error)
	UpsertAssignment(ctx context.Context, a *CalendarAssignment) error
	DeleteAssignment(ctx context.Context, orgID, ref string) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const calSel = `id, public_id, org_id, name, description, country_code, year, is_active, created_by, created_at, updated_at`

func scanCal(row pgx.Row) (*HolidayCalendar, error) {
	c := &HolidayCalendar{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.CountryCode, &c.Year, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) FindAllCalendars(ctx context.Context, orgID string, activeOnly bool) ([]*HolidayCalendar, error) {
	q := `SELECT ` + calSel + ` FROM hrm_holiday_calendars WHERE org_id=$1`
	if activeOnly {
		q += ` AND is_active=TRUE`
	}
	q += ` ORDER BY year DESC, name`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("holidays: FindAllCalendars: %w", err)
	}
	defer rows.Close()
	list := make([]*HolidayCalendar, 0)
	for rows.Next() {
		c, err := scanCal(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindCalendarByRef(ctx context.Context, orgID, ref string) (*HolidayCalendar, error) {
	return scanCal(r.db.QueryRow(ctx, `SELECT `+calSel+` FROM hrm_holiday_calendars WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) CreateCalendar(ctx context.Context, c *HolidayCalendar) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_holiday_calendars (org_id,name,description,country_code,year,is_active,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.CountryCode, c.Year, c.IsActive, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateCalendar(ctx context.Context, c *HolidayCalendar) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_holiday_calendars SET name=$1,description=$2,is_active=$3,updated_at=NOW() WHERE id=$4 AND org_id=$5 RETURNING updated_at`,
		c.Name, c.Description, c.IsActive, c.ID, c.OrgID).Scan(&c.UpdatedAt)
}

func (r *repoImpl) DeleteCalendar(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_holiday_calendars WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil {
		return fmt.Errorf("holidays: DeleteCalendar: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCalendarNotFound
	}
	return nil
}

func (r *repoImpl) CalendarNameExists(ctx context.Context, orgID, name string, year int, excludeID string) (bool, error) {
	var e bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM hrm_holiday_calendars WHERE org_id=$1 AND LOWER(name)=LOWER($2) AND year=$3 AND id::text!=$4)`,
		orgID, name, year, excludeID).Scan(&e)
	return e, err
}

const holSel = `id, public_id, calendar_id, name, to_char(date,'YYYY-MM-DD'), holiday_type, is_paid, repeat_yearly, created_at`

func scanHol(row pgx.Row) (*Holiday, error) {
	h := &Holiday{}
	err := row.Scan(&h.ID, &h.PublicID, &h.CalendarID, &h.Name, &h.Date, &h.HolidayType, &h.IsPaid, &h.RepeatYearly, &h.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return h, nil
}

func (r *repoImpl) FindHolidays(ctx context.Context, calendarID string) ([]*Holiday, error) {
	rows, err := r.db.Query(ctx, `SELECT `+holSel+` FROM hrm_holidays WHERE calendar_id=$1 ORDER BY date`, calendarID)
	if err != nil {
		return nil, fmt.Errorf("holidays: FindHolidays: %w", err)
	}
	defer rows.Close()
	list := make([]*Holiday, 0)
	for rows.Next() {
		h, err := scanHol(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindHolidayByRef(ctx context.Context, calendarID, ref string) (*Holiday, error) {
	return scanHol(r.db.QueryRow(ctx, `SELECT `+holSel+` FROM hrm_holidays WHERE calendar_id=$1 AND (id::text=$2 OR public_id=$2)`, calendarID, ref))
}

func (r *repoImpl) CreateHoliday(ctx context.Context, h *Holiday) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_holidays (calendar_id,name,date,holiday_type,is_paid,repeat_yearly)
		VALUES ($1,$2,$3::date,$4,$5,$6) RETURNING id, public_id, created_at`,
		h.CalendarID, h.Name, h.Date, h.HolidayType, h.IsPaid, h.RepeatYearly,
	).Scan(&h.ID, &h.PublicID, &h.CreatedAt)
}

func (r *repoImpl) UpdateHoliday(ctx context.Context, h *Holiday) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_holidays SET name=$1,date=$2::date,holiday_type=$3,is_paid=$4,repeat_yearly=$5 WHERE id=$6`,
		h.Name, h.Date, h.HolidayType, h.IsPaid, h.RepeatYearly, h.ID)
	return err
}

func (r *repoImpl) DeleteHoliday(ctx context.Context, calendarID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_holidays WHERE calendar_id=$1 AND (id::text=$2 OR public_id=$2)`, calendarID, ref)
	if err != nil {
		return fmt.Errorf("holidays: DeleteHoliday: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrHolidayNotFound
	}
	return nil
}

const asgSel = `id, public_id, org_id, calendar_id, assignee_type, assignee_id, to_char(effective_date,'YYYY-MM-DD'), created_by, created_at`

func (r *repoImpl) FindAssignment(ctx context.Context, assigneeType, assigneeID string) (*CalendarAssignment, error) {
	a := &CalendarAssignment{}
	err := r.db.QueryRow(ctx,
		`SELECT `+asgSel+` FROM hrm_calendar_assignments WHERE assignee_type=$1 AND assignee_id=$2`,
		assigneeType, assigneeID).Scan(&a.ID, &a.PublicID, &a.OrgID, &a.CalendarID, &a.AssigneeType, &a.AssigneeID, &a.EffectiveDate, &a.CreatedBy, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("holidays: FindAssignment: %w", err)
	}
	return a, nil
}

func (r *repoImpl) UpsertAssignment(ctx context.Context, a *CalendarAssignment) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_calendar_assignments (org_id,calendar_id,assignee_type,assignee_id,effective_date,created_by)
		VALUES ($1,$2,$3,$4,$5::date,$6)
		ON CONFLICT (assignee_type,assignee_id) DO UPDATE SET calendar_id=EXCLUDED.calendar_id, effective_date=EXCLUDED.effective_date
		RETURNING id, public_id, created_at`,
		a.OrgID, a.CalendarID, a.AssigneeType, a.AssigneeID, a.EffectiveDate, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt)
}

func (r *repoImpl) DeleteAssignment(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx, `DELETE FROM hrm_calendar_assignments WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref)
	if err != nil {
		return fmt.Errorf("holidays: DeleteAssignment: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}
