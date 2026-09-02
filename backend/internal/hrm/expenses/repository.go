// backend/internal/hrm/expenses/repository.go
package expenses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

type Repository interface {
	// Travel
	ListTravel(ctx context.Context, orgID string, filter ListFilter) ([]*TravelRequest, int, error)
	FindTravelByRef(ctx context.Context, orgID, ref string) (*TravelRequest, error)
	CreateTravel(ctx context.Context, t *TravelRequest) error
	UpdateTravel(ctx context.Context, t *TravelRequest) error

	CreateItineraryItem(ctx context.Context, i *ItineraryItem) error
	ListItinerary(ctx context.Context, travelRequestID string) ([]*ItineraryItem, error)

	// Advances
	ListAdvances(ctx context.Context, orgID string, filter ListFilter) ([]*Advance, int, error)
	FindAdvanceByRef(ctx context.Context, orgID, ref string) (*Advance, error)
	CreateAdvance(ctx context.Context, a *Advance) error
	UpdateAdvance(ctx context.Context, a *Advance) error
	// OutstandingAdvancesForEmployee returns disbursed advances with money
	// still unsettled. Backs the F&F settlement, where an advance a leaver
	// never accounted for is recovered from their final pay.
	OutstandingAdvancesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Advance, error)

	// Claims + lines
	ListClaims(ctx context.Context, orgID string, filter ListFilter) ([]*Claim, int, error)
	FindClaimByRef(ctx context.Context, orgID, ref string) (*Claim, error)
	FindClaimByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Claim, error)
	CreateClaim(ctx context.Context, c *Claim) error
	UpdateClaim(ctx context.Context, c *Claim) error

	CreateLine(ctx context.Context, l *Line) error
	ListLines(ctx context.Context, claimID string) ([]*Line, error)
	FindLineByRef(ctx context.Context, ref string) (*Line, error)
	// SetLineApprovedAmount is the LINE-LEVEL approval write — the one place
	// approved_amount changes. Deliberately narrow: nothing else may touch it.
	SetLineApprovedAmount(ctx context.Context, lineID string, approved decimal.Decimal) error

	// Config — effective-dated lookups use MAX(effective_date) <= asOf, the
	// 7D statutory.SlabsAsOf shape.
	ListPolicies(ctx context.Context, orgID string) ([]*Policy, error)
	CreatePolicy(ctx context.Context, p *Policy) error
	FindPolicyAsOf(ctx context.Context, orgID string, category LineCategory, asOf time.Time) (*Policy, error)

	ListPerDiemRates(ctx context.Context, orgID string) ([]*PerDiemRate, error)
	CreatePerDiemRate(ctx context.Context, r *PerDiemRate) error
	FindPerDiemRateAsOf(ctx context.Context, orgID string, countryCode *string, asOf time.Time) (*PerDiemRate, error)

	ListMileageRates(ctx context.Context, orgID string) ([]*MileageRate, error)
	CreateMileageRate(ctx context.Context, r *MileageRate) error
	FindMileageRateAsOf(ctx context.Context, orgID string, asOf time.Time) (*MileageRate, error)

	CreateViolation(ctx context.Context, v *PolicyViolation) error
	ListViolationsByClaim(ctx context.Context, claimID string) (map[string][]*PolicyViolation, error)

	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// scopedWhere builds the shared org + employee-scope predicate every list
// query here needs.
func scopedWhere(orgID string, filter ListFilter, employeeCol string) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", employeeCol, len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, employeeCol, len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

// ── Travel ───────────────────────────────────────────────────────────────────

const travelSel = `id, public_id, org_id, employee_id, purpose, destination, destination_country,
	start_date, end_date, status, approval_instance_id, currency, created_by, created_at, updated_at`

func scanTravel(row pgx.Row) (*TravelRequest, error) {
	t := &TravelRequest{}
	err := row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.EmployeeID, &t.Purpose, &t.Destination,
		&t.DestinationCountry, &t.StartDate, &t.EndDate, &t.Status, &t.ApprovalInstanceID,
		&t.Currency, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *repoImpl) ListTravel(ctx context.Context, orgID string, filter ListFilter) ([]*TravelRequest, int, error) {
	where, args := scopedWhere(orgID, filter, "employee_id")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_travel_requests WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("expenses: ListTravel: count: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_travel_requests WHERE %s ORDER BY start_date DESC LIMIT $%d OFFSET $%d`,
		travelSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("expenses: ListTravel: %w", err)
	}
	defer rows.Close()
	list := make([]*TravelRequest, 0)
	for rows.Next() {
		t, err := scanTravel(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindTravelByRef(ctx context.Context, orgID, ref string) (*TravelRequest, error) {
	return scanTravel(r.db.QueryRow(ctx,
		`SELECT `+travelSel+` FROM hrm_travel_requests WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateTravel(ctx context.Context, t *TravelRequest) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_travel_requests (org_id, employee_id, purpose, destination, destination_country,
		     start_date, end_date, status, currency, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id, public_id, created_at, updated_at`,
		t.OrgID, t.EmployeeID, t.Purpose, t.Destination, t.DestinationCountry,
		t.StartDate, t.EndDate, t.Status, t.Currency, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) UpdateTravel(ctx context.Context, t *TravelRequest) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_travel_requests SET status=$3, approval_instance_id=$4, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		t.OrgID, t.ID, t.Status, t.ApprovalInstanceID)
	if err != nil {
		return fmt.Errorf("expenses: UpdateTravel: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrTravelNotFound
	}
	return nil
}

const itinerarySel = `id, public_id, travel_request_id, item_type, description, from_location, to_location,
	starts_at, ends_at, booking_reference, estimated_cost, currency, display_order, created_at, updated_at`

func scanItinerary(row pgx.Row) (*ItineraryItem, error) {
	i := &ItineraryItem{}
	err := row.Scan(&i.ID, &i.PublicID, &i.TravelRequestID, &i.ItemType, &i.Description,
		&i.FromLocation, &i.ToLocation, &i.StartsAt, &i.EndsAt, &i.BookingReference,
		&i.EstimatedCost, &i.Currency, &i.DisplayOrder, &i.CreatedAt, &i.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *repoImpl) CreateItineraryItem(ctx context.Context, i *ItineraryItem) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_travel_itinerary_items (travel_request_id, item_type, description,
		     from_location, to_location, starts_at, ends_at, booking_reference,
		     estimated_cost, currency, display_order)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id, public_id, created_at, updated_at`,
		i.TravelRequestID, i.ItemType, i.Description, i.FromLocation, i.ToLocation,
		i.StartsAt, i.EndsAt, i.BookingReference, i.EstimatedCost, i.Currency, i.DisplayOrder,
	).Scan(&i.ID, &i.PublicID, &i.CreatedAt, &i.UpdatedAt)
}

func (r *repoImpl) ListItinerary(ctx context.Context, travelRequestID string) ([]*ItineraryItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+itinerarySel+` FROM hrm_travel_itinerary_items
		  WHERE travel_request_id=$1::uuid ORDER BY display_order, starts_at NULLS LAST`,
		travelRequestID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListItinerary: %w", err)
	}
	defer rows.Close()
	list := make([]*ItineraryItem, 0)
	for rows.Next() {
		i, err := scanItinerary(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, rows.Err()
}

// ── Advances ─────────────────────────────────────────────────────────────────

const advanceSel = `id, public_id, org_id, employee_id, travel_request_id, amount, currency,
	settled_amount, status, disbursed_at, disbursed_by, created_by, created_at, updated_at`

func scanAdvance(row pgx.Row) (*Advance, error) {
	a := &Advance{}
	err := row.Scan(&a.ID, &a.PublicID, &a.OrgID, &a.EmployeeID, &a.TravelRequestID,
		&a.Amount, &a.Currency, &a.SettledAmount, &a.Status,
		&a.DisbursedAt, &a.DisbursedBy, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *repoImpl) ListAdvances(ctx context.Context, orgID string, filter ListFilter) ([]*Advance, int, error) {
	where, args := scopedWhere(orgID, filter, "employee_id")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_travel_advances WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("expenses: ListAdvances: count: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_travel_advances WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		advanceSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("expenses: ListAdvances: %w", err)
	}
	defer rows.Close()
	list := make([]*Advance, 0)
	for rows.Next() {
		a, err := scanAdvance(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindAdvanceByRef(ctx context.Context, orgID, ref string) (*Advance, error) {
	return scanAdvance(r.db.QueryRow(ctx,
		`SELECT `+advanceSel+` FROM hrm_travel_advances WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreateAdvance(ctx context.Context, a *Advance) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_travel_advances (org_id, employee_id, travel_request_id, amount, currency, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
		a.OrgID, a.EmployeeID, a.TravelRequestID, a.Amount, a.Currency, a.Status, a.CreatedBy,
	).Scan(&a.ID, &a.PublicID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *repoImpl) UpdateAdvance(ctx context.Context, a *Advance) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_travel_advances
		    SET status=$3, settled_amount=$4, disbursed_at=$5, disbursed_by=$6, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		a.OrgID, a.ID, a.Status, a.SettledAmount, a.DisbursedAt, a.DisbursedBy)
	if err != nil {
		return fmt.Errorf("expenses: UpdateAdvance: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrAdvanceNotFound
	}
	return nil
}

// ── Claims + lines ───────────────────────────────────────────────────────────

const claimSel = `id, public_id, org_id, employee_id, travel_request_id, advance_id, title, description,
	base_currency, status, approval_instance_id, reimbursement_id, submitted_at, decided_at,
	created_by, created_at, updated_at`

func scanClaim(row pgx.Row) (*Claim, error) {
	c := &Claim{}
	err := row.Scan(&c.ID, &c.PublicID, &c.OrgID, &c.EmployeeID, &c.TravelRequestID, &c.AdvanceID,
		&c.Title, &c.Description, &c.BaseCurrency, &c.Status, &c.ApprovalInstanceID,
		&c.ReimbursementID, &c.SubmittedAt, &c.DecidedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) ListClaims(ctx context.Context, orgID string, filter ListFilter) ([]*Claim, int, error) {
	where, args := scopedWhere(orgID, filter, "employee_id")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_expense_claims WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("expenses: ListClaims: count: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_expense_claims WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		claimSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("expenses: ListClaims: %w", err)
	}
	defer rows.Close()
	list := make([]*Claim, 0)
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindClaimByRef(ctx context.Context, orgID, ref string) (*Claim, error) {
	return scanClaim(r.db.QueryRow(ctx,
		`SELECT `+claimSel+` FROM hrm_expense_claims WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindClaimByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Claim, error) {
	return scanClaim(r.db.QueryRow(ctx,
		`SELECT `+claimSel+` FROM hrm_expense_claims WHERE org_id=$1 AND approval_instance_id=$2::uuid`,
		orgID, instanceID))
}

func (r *repoImpl) CreateClaim(ctx context.Context, c *Claim) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_expense_claims (org_id, employee_id, travel_request_id, advance_id,
		     title, description, base_currency, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id, public_id, created_at, updated_at`,
		c.OrgID, c.EmployeeID, c.TravelRequestID, c.AdvanceID,
		c.Title, c.Description, c.BaseCurrency, c.Status, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateClaim(ctx context.Context, c *Claim) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_expense_claims
		    SET status=$3, approval_instance_id=$4, reimbursement_id=$5,
		        submitted_at=$6, decided_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		c.OrgID, c.ID, c.Status, c.ApprovalInstanceID, c.ReimbursementID, c.SubmittedAt, c.DecidedAt)
	if err != nil {
		return fmt.Errorf("expenses: UpdateClaim: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrClaimNotFound
	}
	return nil
}

const lineSel = `id, public_id, claim_id, category, description, expense_date, amount, currency,
	exchange_rate, exchange_rate_date, base_amount, approved_amount, receipt_url, ocr_raw,
	mileage_distance, mileage_rate_id, display_order, created_at, updated_at`

func scanLine(row pgx.Row) (*Line, error) {
	l := &Line{}
	err := row.Scan(&l.ID, &l.PublicID, &l.ClaimID, &l.Category, &l.Description, &l.ExpenseDate,
		&l.Amount, &l.Currency, &l.ExchangeRate, &l.ExchangeRateDate, &l.BaseAmount, &l.ApprovedAmount,
		&l.ReceiptURL, &l.OCRRaw, &l.MileageDistance, &l.MileageRateID,
		&l.DisplayOrder, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *repoImpl) CreateLine(ctx context.Context, l *Line) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_expense_lines (claim_id, category, description, expense_date,
		     amount, currency, exchange_rate, exchange_rate_date, base_amount, receipt_url,
		     mileage_distance, mileage_rate_id, display_order)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id, public_id, created_at, updated_at`,
		l.ClaimID, l.Category, l.Description, l.ExpenseDate,
		l.Amount, l.Currency, l.ExchangeRate, l.ExchangeRateDate, l.BaseAmount, l.ReceiptURL,
		l.MileageDistance, l.MileageRateID, l.DisplayOrder,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *repoImpl) ListLines(ctx context.Context, claimID string) ([]*Line, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+lineSel+` FROM hrm_expense_lines WHERE claim_id=$1::uuid ORDER BY display_order, created_at`,
		claimID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListLines: %w", err)
	}
	defer rows.Close()
	list := make([]*Line, 0)
	for rows.Next() {
		l, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindLineByRef(ctx context.Context, ref string) (*Line, error) {
	return scanLine(r.db.QueryRow(ctx,
		`SELECT `+lineSel+` FROM hrm_expense_lines WHERE id::text=$1 OR public_id=$1`, ref))
}

func (r *repoImpl) SetLineApprovedAmount(ctx context.Context, lineID string, approved decimal.Decimal) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_expense_lines SET approved_amount=$2, updated_at=NOW() WHERE id=$1::uuid`,
		lineID, approved)
	if err != nil {
		// chk_hrm_expl_approved_le_amount fires when an approver tries to
		// inflate a line beyond what was spent.
		if strings.Contains(err.Error(), "chk_hrm_expl_approved_le_amount") {
			return ErrApprovedExceedsClaimed
		}
		return fmt.Errorf("expenses: SetLineApprovedAmount: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrLineNotFound
	}
	return nil
}

// ── Config ───────────────────────────────────────────────────────────────────

const policySel = `id, public_id, org_id, category, max_amount, currency, effective_date, created_by, created_at`

func scanPolicy(row pgx.Row) (*Policy, error) {
	p := &Policy{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.Category, &p.MaxAmount, &p.Currency,
		&p.EffectiveDate, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) ListPolicies(ctx context.Context, orgID string) ([]*Policy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+policySel+` FROM hrm_expense_policies WHERE org_id=$1 ORDER BY category, effective_date DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListPolicies: %w", err)
	}
	defer rows.Close()
	list := make([]*Policy, 0)
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreatePolicy(ctx context.Context, p *Policy) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_expense_policies (org_id, category, max_amount, currency, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at`,
		p.OrgID, p.Category, p.MaxAmount, p.Currency, p.EffectiveDate, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt)
}

// FindPolicyAsOf resolves the cap in force on asOf — the 7D SlabsAsOf shape.
// A cap raised next month must not retroactively excuse this month's breach.
func (r *repoImpl) FindPolicyAsOf(ctx context.Context, orgID string, category LineCategory, asOf time.Time) (*Policy, error) {
	return scanPolicy(r.db.QueryRow(ctx,
		`SELECT `+policySel+` FROM hrm_expense_policies
		  WHERE org_id=$1 AND category=$2 AND effective_date <= $3
		  ORDER BY effective_date DESC LIMIT 1`,
		orgID, category, asOf))
}

const perDiemSel = `id, public_id, org_id, country_code, daily_amount, currency, effective_date, created_by, created_at`

func scanPerDiem(row pgx.Row) (*PerDiemRate, error) {
	p := &PerDiemRate{}
	err := row.Scan(&p.ID, &p.PublicID, &p.OrgID, &p.CountryCode, &p.DailyAmount, &p.Currency,
		&p.EffectiveDate, &p.CreatedBy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) ListPerDiemRates(ctx context.Context, orgID string) ([]*PerDiemRate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+perDiemSel+` FROM hrm_per_diem_rates WHERE org_id=$1 ORDER BY country_code NULLS FIRST, effective_date DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListPerDiemRates: %w", err)
	}
	defer rows.Close()
	list := make([]*PerDiemRate, 0)
	for rows.Next() {
		p, err := scanPerDiem(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreatePerDiemRate(ctx context.Context, p *PerDiemRate) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_per_diem_rates (org_id, country_code, daily_amount, currency, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at`,
		p.OrgID, p.CountryCode, p.DailyAmount, p.Currency, p.EffectiveDate, p.CreatedBy,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt)
}

// FindPerDiemRateAsOf prefers a country-specific rate and falls back to the
// org-wide one (country_code IS NULL). ORDER BY puts the specific match first
// because FALSE < TRUE, so a NULL-country row only wins when nothing else does.
func (r *repoImpl) FindPerDiemRateAsOf(ctx context.Context, orgID string, countryCode *string, asOf time.Time) (*PerDiemRate, error) {
	return scanPerDiem(r.db.QueryRow(ctx,
		`SELECT `+perDiemSel+` FROM hrm_per_diem_rates
		  WHERE org_id=$1 AND effective_date <= $3
		    AND (country_code = $2 OR country_code IS NULL)
		  ORDER BY (country_code IS NULL), effective_date DESC LIMIT 1`,
		orgID, countryCode, asOf))
}

const mileageSel = `id, public_id, org_id, rate_per_unit, unit, currency, effective_date, created_by, created_at`

func scanMileage(row pgx.Row) (*MileageRate, error) {
	m := &MileageRate{}
	err := row.Scan(&m.ID, &m.PublicID, &m.OrgID, &m.RatePerUnit, &m.Unit, &m.Currency,
		&m.EffectiveDate, &m.CreatedBy, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *repoImpl) ListMileageRates(ctx context.Context, orgID string) ([]*MileageRate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+mileageSel+` FROM hrm_mileage_rates WHERE org_id=$1 ORDER BY effective_date DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListMileageRates: %w", err)
	}
	defer rows.Close()
	list := make([]*MileageRate, 0)
	for rows.Next() {
		m, err := scanMileage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreateMileageRate(ctx context.Context, m *MileageRate) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_mileage_rates (org_id, rate_per_unit, unit, currency, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, public_id, created_at`,
		m.OrgID, m.RatePerUnit, m.Unit, m.Currency, m.EffectiveDate, m.CreatedBy,
	).Scan(&m.ID, &m.PublicID, &m.CreatedAt)
}

func (r *repoImpl) FindMileageRateAsOf(ctx context.Context, orgID string, asOf time.Time) (*MileageRate, error) {
	return scanMileage(r.db.QueryRow(ctx,
		`SELECT `+mileageSel+` FROM hrm_mileage_rates
		  WHERE org_id=$1 AND effective_date <= $2 ORDER BY effective_date DESC LIMIT 1`,
		orgID, asOf))
}

func (r *repoImpl) CreateViolation(ctx context.Context, v *PolicyViolation) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_expense_policy_violations (line_id, policy_id, category, max_amount, actual_amount, message)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		v.LineID, v.PolicyID, v.Category, v.MaxAmount, v.ActualAmount, v.Message,
	).Scan(&v.ID, &v.CreatedAt)
}

func (r *repoImpl) ListViolationsByClaim(ctx context.Context, claimID string) (map[string][]*PolicyViolation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT v.id, v.line_id, v.policy_id, v.category, v.max_amount, v.actual_amount, v.message, v.created_at
		   FROM hrm_expense_policy_violations v
		   JOIN hrm_expense_lines l ON l.id = v.line_id
		  WHERE l.claim_id = $1::uuid
		  ORDER BY v.created_at`,
		claimID)
	if err != nil {
		return nil, fmt.Errorf("expenses: ListViolationsByClaim: %w", err)
	}
	defer rows.Close()
	out := make(map[string][]*PolicyViolation)
	for rows.Next() {
		v := &PolicyViolation{}
		if err := rows.Scan(&v.ID, &v.LineID, &v.PolicyID, &v.Category,
			&v.MaxAmount, &v.ActualAmount, &v.Message, &v.CreatedAt); err != nil {
			return nil, err
		}
		out[v.LineID] = append(out[v.LineID], v)
	}
	return out, rows.Err()
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
		return "", fmt.Errorf("expenses: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}

// OutstandingAdvancesForEmployee returns disbursed advances not yet fully
// settled.
//
// Only 'disbursed' counts: a 'pending' advance is money that never left, and
// recovering it would charge somebody for cash they never received.
func (r *repoImpl) OutstandingAdvancesForEmployee(ctx context.Context, orgID, employeeID string) ([]*Advance, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+advanceSel+` FROM hrm_travel_advances
		  WHERE org_id=$1 AND employee_id=$2::uuid AND status='disbursed'
		    AND amount > settled_amount
		  ORDER BY created_at`,
		orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("expenses: OutstandingAdvancesForEmployee: %w", err)
	}
	defer rows.Close()
	out := make([]*Advance, 0)
	for rows.Next() {
		a, err := scanAdvance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
