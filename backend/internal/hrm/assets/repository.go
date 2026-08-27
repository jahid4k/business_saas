// backend/internal/hrm/assets/repository.go
package assets

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
	// Categories + licences — hrm.asset_config, untiered catalog data.
	ListCategories(ctx context.Context, orgID string) ([]*Category, error)
	FindCategoryByRef(ctx context.Context, orgID, ref string) (*Category, error)
	CreateCategory(ctx context.Context, c *Category) error

	ListLicenses(ctx context.Context, orgID string) ([]*License, error)
	FindLicenseByRef(ctx context.Context, orgID, ref string) (*License, error)
	CreateLicense(ctx context.Context, l *License) error
	// CountActiveSeats is the DERIVED seats_used — COUNT(*) over unreleased
	// seat rows. There is no counter column; see migration 00106's header.
	CountActiveSeats(ctx context.Context, licenseID string) (int, error)
	AssignSeat(ctx context.Context, s *SeatAssignment) error
	ReleaseSeat(ctx context.Context, orgID, licenseID, employeeID string) error
	ListSeats(ctx context.Context, orgID, licenseID string) ([]*SeatAssignment, error)

	// Assets — hrm.assets.
	ListAssets(ctx context.Context, orgID string, filter ListFilter) ([]*Asset, int, error)
	FindAssetByRef(ctx context.Context, orgID, ref string) (*Asset, error)
	CreateAsset(ctx context.Context, a *Asset) error
	UpdateAssetStatus(ctx context.Context, orgID, assetID string, status AssetStatus) error

	// Assignments. FindCurrentAssignment IS the "who holds this" query — the
	// single place that definition lives.
	FindCurrentAssignment(ctx context.Context, assetID string) (*Assignment, error)
	CreateAssignment(ctx context.Context, a *Assignment) error
	ReturnAssignment(ctx context.Context, assignmentID, returnedBy string, conditionIn *Condition, notes *string) error
	ListAssignments(ctx context.Context, orgID string, filter ListFilter) ([]*Assignment, int, error)

	CreateMaintenance(ctx context.Context, m *MaintenanceLog) error
	ListMaintenance(ctx context.Context, orgID, assetID string) ([]*MaintenanceLog, error)

	ListRequests(ctx context.Context, orgID string, filter ListFilter) ([]*AssetRequest, int, error)
	FindRequestByRef(ctx context.Context, orgID, ref string) (*AssetRequest, error)
	CreateRequest(ctx context.Context, r *AssetRequest) error
	UpdateRequest(ctx context.Context, r *AssetRequest) error

	// FindEmployeeIDByUserID resolves a caller's OWN hrm_employees.id, for
	// the self-service .request path — the compensation.Repository /
	// benefits.Repository precedent, each package on its own repository
	// rather than a shared service method.
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Categories ───────────────────────────────────────────────────────────────

const categorySel = `id, public_id, org_id, name, description, requires_return,
	useful_life_months, is_active, created_by, created_at, updated_at`

func scanCategory(row pgx.Row) (*Category, error) {
	c := &Category{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Description, &c.RequiresReturn,
		&c.UsefulLifeMonths, &c.IsActive, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) ListCategories(ctx context.Context, orgID string) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `SELECT `+categorySel+` FROM hrm_asset_categories WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("assets: ListCategories: %w", err)
	}
	defer rows.Close()
	list := make([]*Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindCategoryByRef(ctx context.Context, orgID, ref string) (*Category, error) {
	return scanCategory(r.db.QueryRow(ctx,
		`SELECT `+categorySel+` FROM hrm_asset_categories WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateCategory(ctx context.Context, c *Category) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_asset_categories (org_id, name, description, requires_return, useful_life_months, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.Name, c.Description, c.RequiresReturn, c.UsefulLifeMonths, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

// ── Licences + seats ─────────────────────────────────────────────────────────

const licenseSel = `id, public_id, org_id, name, vendor, seats_total, cost_per_seat, currency,
	renewal_date, is_active, created_by, created_at, updated_at`

func scanLicense(row pgx.Row) (*License, error) {
	l := &License{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OrgID, &l.Name, &l.Vendor, &l.SeatsTotal,
		&l.CostPerSeat, &l.Currency, &l.RenewalDate, &l.IsActive,
		&l.CreatedBy, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *repoImpl) ListLicenses(ctx context.Context, orgID string) ([]*License, error) {
	rows, err := r.db.Query(ctx, `SELECT `+licenseSel+` FROM hrm_software_licenses WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("assets: ListLicenses: %w", err)
	}
	defer rows.Close()
	list := make([]*License, 0)
	for rows.Next() {
		l, err := scanLicense(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindLicenseByRef(ctx context.Context, orgID, ref string) (*License, error) {
	return scanLicense(r.db.QueryRow(ctx,
		`SELECT `+licenseSel+` FROM hrm_software_licenses WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateLicense(ctx context.Context, l *License) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_software_licenses (org_id, name, vendor, seats_total, cost_per_seat, currency, renewal_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
		l.OrgID, l.Name, l.Vendor, l.SeatsTotal, l.CostPerSeat, l.Currency, l.RenewalDate, l.CreatedBy,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *repoImpl) CountActiveSeats(ctx context.Context, licenseID string) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_license_seat_assignments WHERE license_id=$1::uuid AND released_at IS NULL`,
		licenseID).Scan(&n); err != nil {
		return 0, fmt.Errorf("assets: CountActiveSeats: %w", err)
	}
	return n, nil
}

func (r *repoImpl) AssignSeat(ctx context.Context, s *SeatAssignment) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_license_seat_assignments (org_id, license_id, employee_id, assigned_by)
		 VALUES ($1,$2,$3,$4) RETURNING id, public_id, assigned_at, created_at, updated_at`,
		s.OrgID, s.LicenseID, s.EmployeeID, s.AssignedBy,
	).Scan(&s.ID, &s.PublicID, &s.AssignedAt, &s.CreatedAt, &s.UpdatedAt)
	// uq_hrm_lseat_active surfaces as a friendly error rather than a raw 23505.
	if err != nil && strings.Contains(err.Error(), "uq_hrm_lseat_active") {
		return ErrSeatAlreadyHeld
	}
	return err
}

func (r *repoImpl) ReleaseSeat(ctx context.Context, orgID, licenseID, employeeID string) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_license_seat_assignments SET released_at=NOW(), updated_at=NOW()
		  WHERE org_id=$1 AND license_id=$2::uuid AND employee_id=$3::uuid AND released_at IS NULL`,
		orgID, licenseID, employeeID)
	if err != nil {
		return fmt.Errorf("assets: ReleaseSeat: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrSeatNotFound
	}
	return nil
}

const seatSel = `id, public_id, org_id, license_id, employee_id, assigned_at, assigned_by,
	released_at, created_at, updated_at`

func scanSeat(row pgx.Row) (*SeatAssignment, error) {
	s := &SeatAssignment{}
	err := row.Scan(&s.ID, &s.PublicID, &s.OrgID, &s.LicenseID, &s.EmployeeID,
		&s.AssignedAt, &s.AssignedBy, &s.ReleasedAt, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) ListSeats(ctx context.Context, orgID, licenseID string) ([]*SeatAssignment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+seatSel+` FROM hrm_license_seat_assignments
		  WHERE org_id=$1 AND license_id=$2::uuid ORDER BY assigned_at DESC`,
		orgID, licenseID)
	if err != nil {
		return nil, fmt.Errorf("assets: ListSeats: %w", err)
	}
	defer rows.Close()
	list := make([]*SeatAssignment, 0)
	for rows.Next() {
		s, err := scanSeat(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// ── Assets ───────────────────────────────────────────────────────────────────

const assetSel = `a.id, a.public_id, a.org_id, a.category_id, a.name, a.asset_tag, a.serial_number,
	a.purchase_date, a.purchase_cost, a.currency, a.status, a.notes,
	a.created_by, a.created_at, a.updated_at`

// assetSelWithHolder appends the DERIVED current holder. This LEFT JOIN is
// the whole "current holder" mechanism — there is no column to read, so
// every asset read path resolves it here, from the assignment row with
// returned_at IS NULL (single-valued by uq_hrm_asgn_active).
const assetSelWithHolder = assetSel + `, cur.employee_id::text`

const assetFromWithHolder = `FROM hrm_assets a
	LEFT JOIN hrm_asset_assignments cur
	       ON cur.asset_id = a.id AND cur.returned_at IS NULL`

func scanAssetWithHolder(row pgx.Row) (*Asset, error) {
	a := &Asset{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.CategoryID, &a.Name, &a.AssetTag, &a.SerialNumber,
		&a.PurchaseDate, &a.PurchaseCost, &a.Currency, &a.Status, &a.Notes,
		&a.CreatedBy, &a.CreatedAt, &a.UpdatedAt, &a.CurrentHolderEmployeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *repoImpl) ListAssets(ctx context.Context, orgID string, filter ListFilter) ([]*Asset, int, error) {
	clauses := []string{"a.org_id = $1"}
	args := []any{orgID}
	if filter.CategoryID != "" {
		args = append(args, filter.CategoryID)
		clauses = append(clauses, fmt.Sprintf("a.category_id = $%d::uuid", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("a.status = $%d", len(args)))
	}
	// Scope filters on the CURRENT HOLDER, not on the asset — hrm_assets has
	// no employee_id of its own. See migration 00107's header on the
	// asymmetry, which mirrors hrm.payroll's (00097).
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "cur.employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) `+assetFromWithHolder+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("assets: ListAssets: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s %s WHERE %s ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d`,
		assetSelWithHolder, assetFromWithHolder, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("assets: ListAssets: %w", err)
	}
	defer rows.Close()
	list := make([]*Asset, 0)
	for rows.Next() {
		a, err := scanAssetWithHolder(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindAssetByRef(ctx context.Context, orgID, ref string) (*Asset, error) {
	return scanAssetWithHolder(r.db.QueryRow(ctx,
		`SELECT `+assetSelWithHolder+` `+assetFromWithHolder+
			` WHERE a.org_id=$1 AND (a.id::text=$2 OR a.public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateAsset(ctx context.Context, a *Asset) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_assets (org_id, category_id, name, asset_tag, serial_number,
		     purchase_date, purchase_cost, currency, status, notes, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id, public_id, created_at, updated_at`,
		a.OrgID, a.CategoryID, a.Name, a.AssetTag, a.SerialNumber,
		a.PurchaseDate, a.PurchaseCost, a.Currency, a.Status, a.Notes, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) UpdateAssetStatus(ctx context.Context, orgID, assetID string, status AssetStatus) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_assets SET status=$3, updated_at=NOW() WHERE org_id=$1 AND id=$2::uuid`,
		orgID, assetID, status)
	if err != nil {
		return fmt.Errorf("assets: UpdateAssetStatus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// ── Assignments ──────────────────────────────────────────────────────────────

const assignmentSel = `id, public_id, org_id, asset_id, employee_id, assigned_at, assigned_by,
	condition_out, returned_at, returned_by, condition_in, notes, created_at, updated_at`

func scanAssignment(row pgx.Row) (*Assignment, error) {
	a := &Assignment{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.AssetID, &a.EmployeeID,
		&a.AssignedAt, &a.AssignedBy, &a.ConditionOut,
		&a.ReturnedAt, &a.ReturnedBy, &a.ConditionIn, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// FindCurrentAssignment is THE definition of "who holds this asset" — the one
// place it lives. Nothing else may compute it independently.
func (r *repoImpl) FindCurrentAssignment(ctx context.Context, assetID string) (*Assignment, error) {
	return scanAssignment(r.db.QueryRow(ctx,
		`SELECT `+assignmentSel+` FROM hrm_asset_assignments
		  WHERE asset_id=$1::uuid AND returned_at IS NULL`,
		assetID))
}

func (r *repoImpl) CreateAssignment(ctx context.Context, a *Assignment) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_asset_assignments (org_id, asset_id, employee_id, assigned_by, condition_out, notes)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, assigned_at, created_at, updated_at`,
		a.OrgID, a.AssetID, a.EmployeeID, a.AssignedBy, a.ConditionOut, a.Notes,
	).Scan(&a.ID, &a.PublicID, &a.AssignedAt, &a.CreatedAt, &a.UpdatedAt)
	// uq_hrm_asgn_active is the guarantee that makes the derived current
	// holder single-valued; surface it as a real error, not a raw 23505.
	if err != nil && strings.Contains(err.Error(), "uq_hrm_asgn_active") {
		return ErrAlreadyAssigned
	}
	return err
}

func (r *repoImpl) ReturnAssignment(ctx context.Context, assignmentID, returnedBy string, conditionIn *Condition, notes *string) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_asset_assignments
		    SET returned_at=NOW(), returned_by=$2::uuid, condition_in=$3,
		        notes=COALESCE($4, notes), updated_at=NOW()
		  WHERE id=$1::uuid AND returned_at IS NULL`,
		assignmentID, returnedBy, conditionIn, notes)
	if err != nil {
		return fmt.Errorf("assets: ReturnAssignment: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotAssigned
	}
	return nil
}

func (r *repoImpl) ListAssignments(ctx context.Context, orgID string, filter ListFilter) ([]*Assignment, int, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_asset_assignments WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("assets: ListAssignments: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_asset_assignments WHERE %s ORDER BY assigned_at DESC LIMIT $%d OFFSET $%d`,
		assignmentSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("assets: ListAssignments: %w", err)
	}
	defer rows.Close()
	list := make([]*Assignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

// ── Maintenance ──────────────────────────────────────────────────────────────

const maintenanceSel = `id, public_id, org_id, asset_id, maintenance_type, description,
	cost, currency, performed_at, vendor, created_by, created_at`

func scanMaintenance(row pgx.Row) (*MaintenanceLog, error) {
	m := &MaintenanceLog{}
	err := row.Scan(&m.ID, &m.PublicID, &m.OrgID, &m.AssetID, &m.MaintenanceType, &m.Description,
		&m.Cost, &m.Currency, &m.PerformedAt, &m.Vendor, &m.CreatedBy, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *repoImpl) CreateMaintenance(ctx context.Context, m *MaintenanceLog) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_asset_maintenance_logs (org_id, asset_id, maintenance_type, description,
		     cost, currency, performed_at, vendor, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, public_id, created_at`,
		m.OrgID, m.AssetID, m.MaintenanceType, m.Description,
		m.Cost, m.Currency, m.PerformedAt, m.Vendor, m.CreatedBy,
	).Scan(&m.ID, &m.PublicID, &m.CreatedAt)
}

func (r *repoImpl) ListMaintenance(ctx context.Context, orgID, assetID string) ([]*MaintenanceLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+maintenanceSel+` FROM hrm_asset_maintenance_logs
		  WHERE org_id=$1 AND asset_id=$2::uuid ORDER BY performed_at DESC`,
		orgID, assetID)
	if err != nil {
		return nil, fmt.Errorf("assets: ListMaintenance: %w", err)
	}
	defer rows.Close()
	list := make([]*MaintenanceLog, 0)
	for rows.Next() {
		m, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ── Requests ─────────────────────────────────────────────────────────────────

const requestSel = `id, public_id, org_id, employee_id, category_id, justification, status,
	approval_instance_id, fulfilled_asset_id, fulfilled_at, created_by, created_at, updated_at`

func scanRequest(row pgx.Row) (*AssetRequest, error) {
	rq := &AssetRequest{}
	err := row.Scan(&rq.ID, &rq.PublicID, &rq.OrgID, &rq.EmployeeID, &rq.CategoryID,
		&rq.Justification, &rq.Status, &rq.ApprovalInstanceID,
		&rq.FulfilledAssetID, &rq.FulfilledAt, &rq.CreatedBy, &rq.CreatedAt, &rq.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rq, nil
}

func (r *repoImpl) ListRequests(ctx context.Context, orgID string, filter ListFilter) ([]*AssetRequest, int, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_asset_requests WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("assets: ListRequests: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_asset_requests WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		requestSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("assets: ListRequests: %w", err)
	}
	defer rows.Close()
	list := make([]*AssetRequest, 0)
	for rows.Next() {
		rq, err := scanRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, rq)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindRequestByRef(ctx context.Context, orgID, ref string) (*AssetRequest, error) {
	return scanRequest(r.db.QueryRow(ctx,
		`SELECT `+requestSel+` FROM hrm_asset_requests WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateRequest(ctx context.Context, rq *AssetRequest) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_asset_requests (org_id, employee_id, category_id, justification, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at, updated_at`,
		rq.OrgID, rq.EmployeeID, rq.CategoryID, rq.Justification, rq.Status, rq.CreatedBy,
	).Scan(&rq.ID, &rq.PublicID, &rq.CreatedAt, &rq.UpdatedAt)
}

func (r *repoImpl) UpdateRequest(ctx context.Context, rq *AssetRequest) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_asset_requests
		    SET status=$3, approval_instance_id=$4, fulfilled_asset_id=$5, fulfilled_at=$6, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		rq.OrgID, rq.ID, rq.Status, rq.ApprovalInstanceID, rq.FulfilledAssetID, rq.FulfilledAt)
	if err != nil {
		return fmt.Errorf("assets: UpdateRequest: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrRequestNotFound
	}
	return nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM hrm_employees WHERE org_id=$1 AND user_id=$2 LIMIT 1`,
		orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("assets: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}
